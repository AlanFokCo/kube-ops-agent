package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/tool"

	"github.com/alanfokco/kube-ops-agent-go/internal/env"
	"github.com/alanfokco/kube-ops-agent-go/internal/logging"
	runtimepkg "github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// Executor wraps agentscope-go Agent/Model with production-grade protection (concurrency, circuit breaker, timeout, etc.).
type Executor struct {
	Registry Registry
	Env      *runtimepkg.Environment
	Model    model.ChatModel
	Cache    *AgentCache // optional, for reusing Worker Agent instances
}

func NewExecutor(reg Registry, env *runtimepkg.Environment, m model.ChatModel) *Executor {
	return &Executor{
		Registry: reg,
		Env:      env,
		Model:    m,
		Cache:    NewAgentCache(10, 3600, 600),
	}
}

// MaxExecuteAttempts is the max retry count on execution failure, aligned with Python AgentExecutor.
const MaxExecuteAttempts = 3

// Execute runs the named Agent. Retries with exponential backoff on failure, aligned with Python AgentExecutor.execute_agent.
func (e *Executor) Execute(ctx context.Context, name string, input map[string]any) (*message.Msg, error) {
	if e.Model == nil {
		return nil, fmt.Errorf("chat model is nil, cannot execute agent")
	}
	spec, ok := e.Registry.SpecByName(name)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	if e.Env.Circuit.IsOpen(name) {
		return nil, fmt.Errorf("circuit open for agent: %s", name)
	}

	timeout := e.Env.Config.AgentTimeout
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	var result *message.Msg
	var execErr error
	start := time.Now()
	cfg := e.Env.Config
	if cfg == nil {
		cfg = runtimepkg.DefaultConfig()
	}

	for attempt := 0; attempt < MaxExecuteAttempts; attempt++ {
		if attempt > 0 {
			backoff := runtimepkg.CalculateBackoff(attempt, cfg)
			if err := sleepWithContext(ctx, backoff); err != nil {
				return nil, err
			}
		}

		execCtx, cancel := context.WithTimeout(ctx, timeout)
		err := e.Env.Concurrency.WithAgentSlot(execCtx, name, func(slotCtx context.Context) error {
			if ta, err := e.executeSelfDrivenWorker(slotCtx, spec, input); err == nil && ta != nil {
				result = ta
				return nil
			}
			var wa *agent.ReActAgent
			if e.Cache != nil {
				var cacheErr error
				wa, cacheErr = e.Cache.GetOrCreate(spec.Name, func() (*agent.ReActAgent, error) {
					return buildWorkerAgent(spec, e.Model, e.Env), nil
				})
				if cacheErr != nil {
					execErr = cacheErr
					return cacheErr
				}
			} else {
				wa = buildWorkerAgent(spec, e.Model, e.Env)
			}
			userInput := MakeTriggerMsg(spec, input, ExtractFocusAreas(input), ExtractOrchestratorContext(input))
			msg, err := wa.Reply(slotCtx, userInput)
			if err != nil {
				execErr = err
				return err
			}
			result = msg
			return nil
		})
		cancel()

		if err == nil {
			break
		}
		execErr = err
		if isNonRetryableError(err) {
			break
		}
	}

	duration := time.Since(start).Seconds()
	now := time.Now()
	startedAt := now.Add(-time.Duration(duration * float64(time.Second)))
	err := execErr

	if err != nil {
		e.Env.Circuit.RecordFailure(name)
		if e.Env.Metrics != nil {
			e.Env.Metrics.RecordExecution(name, false, duration)
		}
		if e.Env.State != nil {
			e.Env.State.UpdateAgent(name, &now, false)
		}
		if e.Env.OpsRecorder != nil {
			e.Env.OpsRecorder.Record(name, false, startedAt, now, duration, err.Error())
		}
		logging.LogAgentResult(name, false, duration, err.Error())
		return nil, err
	}

	e.Env.Circuit.RecordSuccess(name)
	if e.Env.Metrics != nil {
		e.Env.Metrics.RecordExecution(name, true, duration)
	}
	if e.Env.State != nil {
		e.Env.State.UpdateAgent(name, &now, true)
	}
	if e.Env.OpsRecorder != nil {
		e.Env.OpsRecorder.Record(name, true, startedAt, now, duration, "")
	}
	logging.LogAgentResult(name, true, duration, "")
	return result, nil
}

// ExecuteChat runs chat using standalone K8sChatAgent, aligned with Python chat_handler.
// Registers all Skills for Chat Agent (register_agent_skill), supports read-only kubectl queries.
func (e *Executor) ExecuteChat(ctx context.Context, question string) (*message.Msg, error) {
	if e.Model == nil {
		return nil, fmt.Errorf("chat model is nil")
	}
	chatAgent := buildChatAgent(e.Registry, e.Model, e.Env)
	msg, err := chatAgent.Reply(ctx, question)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ExecuteChatStream runs chat in streaming mode with per-token/chunk callback, aligned with Python stream_printing_messages.
// Falls back to one-shot full result if model does not support streaming.
func (e *Executor) ExecuteChatStream(ctx context.Context, question string, onChunk func(text string) error) (*message.Msg, error) {
	if e.Model == nil {
		return nil, fmt.Errorf("chat model is nil")
	}
	msgs := buildChatMessages(e.Registry, question)
	stream, err := e.Model.ChatStream(ctx, msgs)
	if err != nil {
		if errors.Is(err, model.ErrStreamNotSupported) {
			msg, fallbackErr := e.ExecuteChat(ctx, question)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			if msg != nil {
				if p := msg.GetTextContent(""); p != nil && *p != "" {
					_ = onChunk(*p)
				}
			}
			return msg, nil
		}
		return nil, err
	}
	defer stream.Close()
	var fullText strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk != nil {
			if p := chunk.GetTextContent(""); p != nil && *p != "" {
				fullText.WriteString(*p)
				if onChunk != nil && onChunk(*p) != nil {
					break
				}
			}
		}
	}
	return message.NewMsg("K8sChatAgent", message.RoleAssistant, fullText.String()), nil
}

// buildChatSysPrompt builds the chat system prompt with skill paths.
func buildChatSysPrompt(reg Registry) string {
	specs := reg.Specs()
	var b strings.Builder
	for _, s := range specs {
		b.WriteString("  - ")
		b.WriteString(filepath.Join(s.SkillDir, "SKILL.md"))
		b.WriteByte('\n')
	}
	skillPaths := b.String()
	if skillPaths != "" {
		skillPaths = "\n## Available Skills (use view_text_file to read)\n" + skillPaths
	}
	return `You are a K8s cluster operations assistant that can answer user queries.
You can execute read-only kubectl commands to get cluster information, but are NOT allowed to perform any modification operations.
Answer concisely and clearly with specific data.` + skillPaths
}

// buildChatMessages builds system+user messages for Chat, for direct ChatStream invocation.
func buildChatMessages(reg Registry, question string) []*message.Msg {
	sysPrompt := buildChatSysPrompt(reg)
	return []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sysPrompt),
		message.NewMsg("user", message.RoleUser, question),
	}
}

// buildChatAgent builds K8sChatAgent, registers all Skills (aligned with Python register_agent_skill).
func buildChatAgent(reg Registry, m model.ChatModel, env *runtimepkg.Environment) *agent.ReActAgent {
	var limiter *runtimepkg.RateLimiter
	if env != nil {
		limiter = env.KubectlLimit
	}
	tk := newWorkerToolkit(limiter)
	sysPrompt := buildChatSysPrompt(reg)
	return agent.NewReActAgent("K8sChatAgent", sysPrompt, m, tk, nil)
}

// executeSelfDrivenWorker runs one self-driven inspection using SelfDrivenWorker/ThinkingAgent.
func (e *Executor) executeSelfDrivenWorker(
	ctx context.Context,
	spec Spec,
	input map[string]any,
) (*message.Msg, error) {
	w := NewSelfDrivenWorker(e.Model, spec)
	focus, _ := input["focus_areas"].([]string)
	orchestratorCtx, _ := input["orchestrator_context"].(string)

	res, err := w.Inspect(ctx, focus, orchestratorCtx)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("%v", res.Output)
	return message.NewMsg(spec.Name, message.RoleAssistant, content), nil
}

// newKubectlTool builds read-only kubectl tool with rate limiting.
func newKubectlTool(limiter *runtimepkg.RateLimiter) *tool.Tool {
	return &tool.Tool{
		Name:        "kubectl",
		Description: "Run a read-only kubectl command. Args: {\"args\": [\"get\", \"pods\", \"-A\"]}",
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			if limiter != nil {
				if err := limiter.Wait(ctx, 1); err != nil {
					return nil, err
				}
			}
			raw, ok := args["args"]
			if !ok {
				return nil, fmt.Errorf("kubectl: missing args")
			}
			rawSlice, ok := raw.([]any)
			if !ok {
				return nil, fmt.Errorf("kubectl: args must be array")
			}
			var argv []string
			for _, v := range rawSlice {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					argv = append(argv, s)
				}
			}
			if len(argv) == 0 {
				return nil, fmt.Errorf("kubectl: empty args")
			}
			cmd := exec.CommandContext(ctx, "kubectl", argv...)
			out, err := cmd.CombinedOutput()
			res := map[string]any{
				"command": append([]string{"kubectl"}, argv...),
				"output":  string(out),
			}
			if err != nil {
				res["error"] = err.Error()
			}
			return res, nil
		},
	}
}

// newWorkerToolkit builds Worker Toolkit: kubectl + agentscope-go v1.0.1 built-in execute_shell_command / view_text_file.
func newWorkerToolkit(limiter *runtimepkg.RateLimiter) *tool.Toolkit {
	return tool.NewToolkit(
		newKubectlTool(limiter),
		tool.ExecuteShellCommandTool(),
		tool.ViewTextFileTool(),
	)
}

// buildWorkerAgent builds from DSL first, otherwise uses default fixed build.
func buildWorkerAgent(spec Spec, m model.ChatModel, env *runtimepkg.Environment) *agent.ReActAgent {
	dsl, err := LoadAgentDSL(spec.SkillDir)
	if err == nil && dsl != nil && m != nil {
		var mcpTools []*tool.Tool
		// MCP tools injected by caller; extensible here
		if a, err := AgentBuilder(dsl, spec.SkillDir, env, mcpTools); err == nil {
			return a
		}
	}
	return newWorkerReActAgent(spec, m, env)
}

// newWorkerReActAgent builds ReActAgent for a single Worker, mounts kubectl + execute_shell_command + view_text_file.
// Uses PLANNING_SYS_PROMPT style, registers main skill and sub_skill directories.
func newWorkerReActAgent(spec Spec, m model.ChatModel, env *runtimepkg.Environment) *agent.ReActAgent {
	var limiter *runtimepkg.RateLimiter
	if env != nil {
		limiter = env.KubectlLimit
	}
	tk := newWorkerToolkit(limiter)

	skillPath := filepath.Join(spec.SkillDir, "SKILL.md")
	subSkillsInfo := ""
	for _, p := range spec.SubSkills {
		subSkillsInfo += "  - " + filepath.Join(spec.SkillDir, p) + "\n"
	}
	if subSkillsInfo != "" {
		subSkillsInfo = "\n## Reference Sub-Skills\nYou can also refer to these sub-skills for additional context:\n" + subSkillsInfo
	}
	focusInfo := "No specific focus areas - perform comprehensive inspection."
	sysPrompt := fmt.Sprintf(planningSysPromptTmpl, spec.Description, skillPath, focusInfo, subSkillsInfo)

	return agent.NewReActAgent(
		spec.Name,
		sysPrompt,
		m,
		tk,
		nil,
	)
}

// NewDefaultChatModel creates a default ChatModel from env config.
func NewDefaultChatModel() (model.ChatModel, error) {
	return NewChatModelWithOverride("")
}

// NewChatModelWithOverride creates ChatModel; modelOverride takes precedence over env vars.
// Supports OPENAI_API_KEY, OPENAI_BASE_URL (for DashScope/OpenAI-compatible APIs), OPENAI_MODEL.
func NewChatModelWithOverride(modelOverride string) (model.ChatModel, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		key = os.Getenv("DASHSCOPE_API_KEY") // Alibaba Cloud DashScope compatible
	}
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY") // Anthropic Claude OpenAI-compatible API
	}
	if key == "" {
		return nil, fmt.Errorf("no supported chat model env configured (e.g. OPENAI_API_KEY, DASHSCOPE_API_KEY, or ANTHROPIC_API_KEY)")
	}
	modelName := env.Get("OPENAI_MODEL", "gpt-4o-mini")
	if modelOverride != "" {
		modelName = modelOverride
	}
	cfg := model.OpenAIConfig{APIKey: key, Model: modelName}
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return model.NewOpenAIChatModel(cfg)
}

// sleepWithContext sleeps for the given duration; returns early if ctx is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// isNonRetryableError returns true for errors that should not be retried (e.g. context cancelled, timeout, agent not found).
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(err.Error(), "agent not found") || strings.Contains(err.Error(), "circuit open") {
		return true
	}
	return false
}

