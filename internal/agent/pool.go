package agent

import (
	"context"
	"sync"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/tool"

	runtimepkg "github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// AgentPool manages multiple Agent instances with concurrent execution.
type AgentPool struct {
	agents map[string]*agent.ReActAgent
	mu     sync.RWMutex
}

// NewAgentPool creates empty AgentPool.
func NewAgentPool() *AgentPool {
	return &AgentPool{
		agents: make(map[string]*agent.ReActAgent),
	}
}

// Register registers an Agent.
func (p *AgentPool) Register(name string, a *agent.ReActAgent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.agents[name] = a
}

// Get gets Agent by name.
func (p *AgentPool) Get(name string) (*agent.ReActAgent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	a, ok := p.agents[name]
	return a, ok
}

// Execute runs multiple Agents concurrently, returns result map.
func (p *AgentPool) Execute(ctx context.Context, names []string, input map[string]any) map[string]*message.Msg {
	results := make(map[string]*message.Msg)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, name := range names {
		a, ok := p.Get(name)
		if !ok {
			continue
		}
		wg.Add(1)
		go func(n string, ag *agent.ReActAgent) {
			defer wg.Done()
			msg, err := ag.Reply(ctx, input)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && msg != nil {
				results[n] = msg
			}
		}(name, a)
	}
	wg.Wait()
	return results
}

// BuildChatAgent builds Chat Agent standalone (ReActAgent + Toolkit + Skill).
func BuildChatAgent(name, sysPrompt string, m model.ChatModel, tk *tool.Toolkit) *agent.ReActAgent {
	if tk == nil {
		tk = tool.NewToolkit()
	}
	return agent.NewReActAgent(name, sysPrompt, m, tk, nil)
}

// BuildChatAgentFromSkillDir builds Chat Agent from skill dir.
func BuildChatAgentFromSkillDir(name, skillDir string, m model.ChatModel, extraTools []*tool.Tool) (*agent.ReActAgent, error) {
	skillContent, err := RegisterAgentSkill(skillDir)
	if err != nil {
		return nil, err
	}
	tools := []*tool.Tool{tool.ExecuteShellCommandTool(), tool.ViewTextFileTool()}
	tools = append(tools, extraTools...)
	tk := tool.NewToolkit(tools...)
	sysPrompt := skillContent
	return agent.NewReActAgent(name, sysPrompt, m, tk, nil), nil
}

// NewAgentPoolFromRegistry builds Agent for each Spec from Registry and fills Pool.
func NewAgentPoolFromRegistry(reg Registry, m model.ChatModel, env *runtimepkg.Environment) *AgentPool {
	pool := NewAgentPool()
	for _, spec := range reg.Specs() {
		a := buildWorkerAgent(spec, m, env)
		pool.Register(spec.Name, a)
	}
	return pool
}
