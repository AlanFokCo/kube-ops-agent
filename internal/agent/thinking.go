package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
)

// ThinkingConfig controls ThinkingAgent recursion depth and iteration count.
type ThinkingConfig struct {
	MaxDepth        int      `json:"max_depth"`
	MaxIterations   int      `json:"max_iterations"`
	PlanningThreshold float64 `json:"planning_threshold"`
	Constraints     []string `json:"constraints"`
	ReadOnly        bool     `json:"read_only"`
}

func DefaultThinkingConfig() ThinkingConfig {
	return ThinkingConfig{
		MaxDepth:        3,
		MaxIterations:   8,
		PlanningThreshold: 0.5,
		Constraints: []string{
			"READ-ONLY inspection only",
			"Allowed: kubectl get, describe, logs, top",
			"Prohibited: apply, delete, patch, edit, create, exec",
		},
		ReadOnly: true,
	}
}

// AgentResult aligns with Python AgentResult core fields.
type AgentResult struct {
	Success         bool          `json:"success"`
	Output          any           `json:"output"`
	DurationSeconds float64       `json:"duration_seconds"`
	AgentName       string        `json:"agent_name"`
	RawPlanJSON     string        `json:"raw_plan_json,omitempty"`
}

type ThinkingAgent struct {
	Name   string
	Model  model.ChatModel
	Config ThinkingConfig
}

// NewThinkingAgent creates a generic ThinkingAgent.
func NewThinkingAgent(name string, m model.ChatModel, _ any, cfg ThinkingConfig) *ThinkingAgent {
	if cfg.MaxIterations == 0 {
		cfg = DefaultThinkingConfig()
	}
	return &ThinkingAgent{
		Name:   name,
		Model:  m,
		Config: cfg,
	}
}

// ThinkAndExecute is the Go version of think_and_execute: analyze first, then either execute directly or create a plan and execute recursively.
func (t *ThinkingAgent) ThinkAndExecute(
	ctx context.Context,
	task string,
	contextData map[string]any,
) (*AgentResult, error) {
	start := time.Now()

	analysis, err := t.analyze(ctx, task, contextData)
	if err != nil {
		return nil, err
	}

	// If analysis deems the problem simple, execute ReAct once directly.
	if !analysis.Complex {
		out, err := t.executeDirect(ctx, task, contextData)
		return &AgentResult{
			Success:         err == nil,
			Output:          out,
			DurationSeconds: time.Since(start).Seconds(),
			AgentName:       t.Name,
		}, err
	}

	// Otherwise create a plan and execute step by step.
	planJSON, err := t.createPlan(ctx, task, contextData)
	if err != nil {
		return nil, err
	}

	// Sub-plan here is a logical concept; actual multi-agent planning is done by the orchestrator/scheduler above.
	out, err := t.executeDirect(ctx, task, map[string]any{
		"analysis": analysis.Summary,
		"plan":     planJSON,
	})
	return &AgentResult{
		Success:         err == nil,
		Output:          out,
		DurationSeconds: time.Since(start).Seconds(),
		AgentName:       t.Name,
		RawPlanJSON:     planJSON,
	}, err
}

// ThinkAndExecuteWithRetry implements the full Analyze-Plan-Execute loop with reflection and retry on failure.
func (t *ThinkingAgent) ThinkAndExecuteWithRetry(
	ctx context.Context,
	task string,
	contextData map[string]any,
) (*AgentResult, error) {
	maxIter := t.Config.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}
	var lastErr error
	var lastPlan string
	for i := 0; i < maxIter; i++ {
		res, err := t.ThinkAndExecute(ctx, task, contextData)
		if err == nil && res != nil && res.Success {
			return res, nil
		}
		lastErr = err
		if res != nil {
			lastPlan = res.RawPlanJSON
		}
		// Reflection: add failure info to context for next round
		if contextData == nil {
			contextData = make(map[string]any)
		}
		contextData["_retry_iteration"] = i + 1
		contextData["_last_error"] = ""
		if err != nil {
			contextData["_last_error"] = err.Error()
		}
		contextData["_last_plan"] = lastPlan
	}
	res, err := t.ThinkAndExecute(ctx, task, contextData)
	if err != nil {
		lastErr = err
	}
	if res != nil {
		return res, lastErr
	}
	return &AgentResult{Success: false, AgentName: t.Name}, lastErr
}

// analysisResult is the simple structure for the analyze phase.
type analysisResult struct {
	Complex bool   `json:"complex"`
	Summary string `json:"summary"`
}

// analyze calls the model to determine task complexity and produce a summary.
func (t *ThinkingAgent) analyze(
	ctx context.Context,
	task string,
	contextData map[string]any,
) (*analysisResult, error) {
	sys := `You are an analysis module for a self-driven Kubernetes agent.

Given a task and context JSON, decide whether the problem is simple or complex.
Return ONLY a compact JSON object:
{
  "complex": true|false,
  "summary": "short natural language summary"
}`

	payload := map[string]any{
		"task":    task,
		"context": contextData,
	}
	data, _ := json.Marshal(payload)

	msgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sys),
		message.NewMsg(t.Name, message.RoleUser, string(data)),
	}

	resp, err := t.Model.Chat(ctx, msgs)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("analysis returned nil response")
	}
	text := resp.Msg.GetTextContent("")
	if text == nil {
		return nil, fmt.Errorf("analysis returned empty content")
	}
	var ar analysisResult
	if err := json.Unmarshal([]byte(*text), &ar); err != nil {
		return nil, fmt.Errorf("parse analysis json: %w", err)
	}
	return &ar, nil
}

// createPlan asks the model to output ThinkingPlan-style JSON (returned as string) based on task and context.
func (t *ThinkingAgent) createPlan(
	ctx context.Context,
	task string,
	contextData map[string]any,
) (string, error) {
	sys := `You are a planning module for a self-driven Kubernetes agent.

Given a task and context JSON, create a multi-step plan.
Return ONLY a valid JSON object with this shape:
{
  "goal": "string",
  "steps": [
    {
      "id": "step-1",
      "description": "what to do",
      "depends_on": [],
      "tool_hint": "kubectl or shell",
      "expected_outcome": "string"
    }
  ]
}`

	payload := map[string]any{
		"task":    task,
		"context": contextData,
	}
	data, _ := json.Marshal(payload)

	msgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sys),
		message.NewMsg(t.Name, message.RoleUser, string(data)),
	}

	resp, err := t.Model.Chat(ctx, msgs)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Msg == nil {
		return "", fmt.Errorf("plan returned nil response")
	}
	text := resp.Msg.GetTextContent("")
	if text == nil {
		return "", fmt.Errorf("plan returned empty content")
	}
	// No strict JSON structure validation; any valid JSON is accepted.
	var tmp any
	if err := json.Unmarshal([]byte(*text), &tmp); err != nil {
		return "", fmt.Errorf("plan is not valid json: %w", err)
	}
	return *text, nil
}

// executeDirect uses ReActAgent + tools to execute one concrete action.
// commandSpec describes the command output by the model.
type commandSpec struct {
	Kind    string   `json:"kind"`              // fixed as "command"
	Engine  string   `json:"engine"`            // "kubectl" or "shell"
	Args    []string `json:"args,omitempty"`    // kubectl args
	Cmd     string   `json:"cmd,omitempty"`     // shell command
	Comment string   `json:"comment,omitempty"` // optional note
}

// executeDirect executes one concrete action using pure JSON protocol + exec.
func (t *ThinkingAgent) executeDirect(
	ctx context.Context,
	task string,
	contextData map[string]any,
) (any, error) {
	sysPrompt := fmt.Sprintf(`You are a self-driven Kubernetes worker agent: %s.

Task: %s

Context JSON:
%s

First, design a SMALL set of read-only commands to inspect the cluster.
You MUST respond with ONLY a JSON array, no extra text, with this shape:
[
  {
    "kind": "command",
    "engine": "kubectl",
    "args": ["get", "pods", "-A"],
    "comment": "optional note"
  }
]

Follow the constraints:
%v

Return ONLY the JSON array.`, t.Name, task, mustJSON(contextData), t.Config.Constraints)

	// 1) Ask model to output command JSON first.
	msgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sysPrompt),
		message.NewMsg(t.Name, message.RoleUser, "PLAN_COMMANDS"),
	}
	resp, err := t.Model.Chat(ctx, msgs)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("model returned nil response")
	}
	text := resp.Msg.GetTextContent("")
	if text == nil {
		return nil, fmt.Errorf("empty command plan from model")
	}

	planText := strings.TrimSpace(*text)
	var cmds []commandSpec
	if err := json.Unmarshal([]byte(planText), &cmds); err != nil {
		return nil, fmt.Errorf("invalid command JSON: %w", err)
	}

	// 2) Execute commands and collect results.
	type cmdResult struct {
		Command commandSpec `json:"command"`
		Output  string      `json:"output"`
		Error   string      `json:"error,omitempty"`
	}
	var results []cmdResult
	for _, c := range cmds {
		if strings.ToLower(c.Kind) != "command" {
			continue
		}
		r := cmdResult{Command: c}
		switch strings.ToLower(c.Engine) {
		case "kubectl":
			if len(c.Args) == 0 {
				r.Error = "kubectl: empty args"
				results = append(results, r)
				continue
			}
			joined := " " + strings.Join(c.Args, " ") + " "
			if !(strings.Contains(joined, " get ") ||
				strings.Contains(joined, " describe ") ||
				strings.Contains(joined, " logs") ||
				strings.Contains(joined, " top ")) {
				r.Error = "kubectl: only get/describe/logs/top allowed"
				results = append(results, r)
				continue
			}
			cmd := exec.CommandContext(ctx, "kubectl", c.Args...)
			out, err := cmd.CombinedOutput()
			r.Output = string(out)
			if err != nil {
				r.Error = err.Error()
			}
		case "shell":
			if strings.TrimSpace(c.Cmd) == "" {
				r.Error = "shell: empty cmd"
				results = append(results, r)
				continue
			}
			if !strings.Contains(c.Cmd, "kubectl") ||
				strings.Contains(c.Cmd, " apply ") ||
				strings.Contains(c.Cmd, " delete ") ||
				strings.Contains(c.Cmd, " patch ") ||
				strings.Contains(c.Cmd, " edit ") ||
				strings.Contains(c.Cmd, " create ") ||
				strings.Contains(c.Cmd, " exec ") {
				r.Error = "shell: only read-only kubectl commands allowed"
				results = append(results, r)
				continue
			}
			cmd := exec.CommandContext(ctx, "bash", "-lc", c.Cmd)
			out, err := cmd.CombinedOutput()
			r.Output = string(out)
			if err != nil {
				r.Error = err.Error()
			}
		default:
			r.Error = "unsupported engine: " + c.Engine
		}
		results = append(results, r)
	}

	// 3) With command execution results, ask model to generate final Markdown report.
	reportSys := `You are a Kubernetes inspection report generator.

You receive:
- The original task and context JSON
- A list of executed commands and their outputs

Produce a concise, well-structured Markdown report for SREs.`

	reportPayload := map[string]any{
		"task":            task,
		"context":         contextData,
		"command_plan":    cmds,
		"command_results": results,
	}
	reportJSON, _ := json.MarshalIndent(reportPayload, "", "  ")

	reportMsgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, reportSys),
		message.NewMsg(t.Name, message.RoleUser, string(reportJSON)),
	}
	reportResp, err := t.Model.Chat(ctx, reportMsgs)
	if err != nil {
		return nil, err
	}
	if reportResp == nil || reportResp.Msg == nil {
		return nil, fmt.Errorf("empty report response from model")
	}
	if txt := reportResp.Msg.GetTextContent(""); txt != nil {
		return *txt, nil
	}
	return reportResp.Msg.Content, nil
}

func mustJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

