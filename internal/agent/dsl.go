package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/tool"

	"gopkg.in/yaml.v3"

	runtimepkg "github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// ModelConfig corresponds to Python agent DSL model config.
type ModelConfig struct {
	Provider   string   `yaml:"provider" json:"provider"`
	Model      string   `yaml:"model" json:"model"`
	APIKey     string   `yaml:"api_key" json:"api_key,omitempty"`
	BaseURL    string   `yaml:"base_url" json:"base_url,omitempty"`
	Temperature float64 `yaml:"temperature" json:"temperature,omitempty"`
	MaxTokens  int      `yaml:"max_tokens" json:"max_tokens,omitempty"`
	TopP       float64  `yaml:"top_p" json:"top_p,omitempty"`
	Stream     bool     `yaml:"stream" json:"stream,omitempty"`
}

// ToolkitConfig corresponds to Python agent DSL toolkit config.
type ToolkitConfig struct {
	Tools      []string `yaml:"tools" json:"tools,omitempty"`           // e.g. execute_shell_command, view_text_file
	MainSkill  string   `yaml:"main_skill" json:"main_skill"`
	SubSkills  []string `yaml:"sub_skills" json:"sub_skills,omitempty"`
	MCPTools   []string `yaml:"mcp_tools" json:"mcp_tools,omitempty"`   // MCP tool names to include, empty=all
	UseMCP     bool     `yaml:"use_mcp" json:"use_mcp"`
	UseBuiltins bool    `yaml:"use_builtins" json:"use_builtins"`       // default true
}

// AgentDSL corresponds to full agent.yaml DSL structure.
type AgentDSL struct {
	Name           string         `yaml:"name"`
	Description    string         `yaml:"description"`
	SysPrompt      string         `yaml:"sys_prompt" json:"sys_prompt,omitempty"`
	IntervalSecond int            `yaml:"interval_seconds"`
	ReportSection  string         `yaml:"report_section"`
	SubSkills      []string       `yaml:"sub_skills"`
	ReadOnly       *bool          `yaml:"read_only"`
	MaxIters       int            `yaml:"max_iters" json:"max_iters,omitempty"`
	Model          *ModelConfig   `yaml:"model,omitempty"`
	Toolkit        *ToolkitConfig `yaml:"toolkit,omitempty"`
}

// PLANNING_SYS_PROMPT planning mode system prompt template, aligned with Python worker_agent.
const planningSysPromptTmpl = `You are an intelligent Kubernetes operations agent responsible for: %s

## Your Approach

You operate in **Intelligent Planning Mode**:

1. **Think First**: Before executing any commands, create a plan
2. **Be Adaptive**: Adjust your approach based on what you find
3. **Correlate Data**: Connect observations to identify root causes
4. **Provide Insights**: Not just data, but intelligent analysis

## Your Capabilities

- execute_shell_command: Run read-only kubectl commands
- view_text_file: Read your skill file and other references

## Security Constraints (CRITICAL)

**Allowed Operations:**
- kubectl get, describe, logs, top, api-resources, version
- Read-only system commands

**Prohibited Operations:**
- ANY modification: apply, delete, patch, edit, create, scale
- Interactive: exec, attach, port-forward, cp
- Dangerous: rollout restart, drain, cordon, taint

## Your Workflow

### Step 1: Read Your Skill
First, use view_text_file to read your skill at %s

### Step 2: Create Your Inspection Plan
Based on your skill domain and any focus areas, create YOUR OWN plan:
## My Inspection Plan
1. [What I'll check] - [What I'm looking for]
2. ...

### Step 3: Execute Intelligently
- Run commands from your plan
- If you find something unexpected, investigate further
- Adapt your approach based on findings

### Step 4: Plan Your Report
Before writing, plan your report structure:
## Report Structure Plan
1. [Section] - [Key points]
2. ...

### Step 5: Generate Report
Execute your report plan to produce comprehensive findings.

## Focus Areas
%s
%s
## Key Principles

1. **Don't follow blindly** - Create YOUR plan based on context
2. **Be thorough but efficient** - Focus on relevant areas
3. **Explain your reasoning** - Why you checked what you checked
4. **Provide actionable insights** - Not just data dumps
5. **Stay safe** - Only read-only operations, always
`

// AgentBuilder builds ReActAgent from AgentDSL and skillDir.
func AgentBuilder(dsl *AgentDSL, skillDir string, env *runtimepkg.Environment, mcpTools []*tool.Tool) (*agent.ReActAgent, error) {
	m := resolveModel(dsl)
	if m == nil {
		return nil, fmt.Errorf("no chat model available for agent %s", dsl.Name)
	}

	tk := buildToolkit(dsl, skillDir, env, mcpTools)
	sysPrompt := buildSysPrompt(dsl, skillDir)
	return agent.NewReActAgent(dsl.Name, sysPrompt, m, tk, nil), nil
}

func buildSysPrompt(dsl *AgentDSL, skillDir string) string {
	readOnly := true
	if dsl.ReadOnly != nil {
		readOnly = *dsl.ReadOnly
	}
	readOnlyConstraint := "Never modify cluster state. Allowed: get/describe/logs/top. Forbidden: apply/delete/patch/edit/create/exec."
	if !readOnly {
		readOnlyConstraint = "Read-write mode: kubectl exec/cp allowed when necessary."
	}

	if dsl.SysPrompt != "" {
		subSkillsInfo := ""
		if len(dsl.SubSkills) > 0 {
			subSkillsInfo = "\n\n## Reference Sub-Skills\nYou can use view_text_file to read:\n"
			for _, p := range dsl.SubSkills {
				subSkillsInfo += "  - " + filepath.Join(skillDir, p) + "\n"
			}
		}
		return dsl.SysPrompt + subSkillsInfo + "\n\n## Security Constraints\n" + readOnlyConstraint
	}

	mainSkill := "SKILL.md"
	if dsl.Toolkit != nil && dsl.Toolkit.MainSkill != "" {
		mainSkill = dsl.Toolkit.MainSkill
	}
	mainSkillPath := filepath.Join(skillDir, mainSkill)
	subSkillsInfo := ""
	if len(dsl.SubSkills) > 0 {
		subSkillsInfo = "\n## Reference Sub-Skills\nYou can also refer to these sub-skills for additional context:\n"
		for _, p := range dsl.SubSkills {
			subSkillsInfo += "  - " + filepath.Join(skillDir, p) + "\n"
		}
	}
	focusInfo := "No specific focus areas - perform comprehensive inspection."
	// Use PLANNING_SYS_PROMPT style (planning mode)
	return fmt.Sprintf(planningSysPromptTmpl,
		dsl.Description,
		mainSkillPath,
		focusInfo,
		subSkillsInfo,
	)
}

func resolveModel(dsl *AgentDSL) model.ChatModel {
	if dsl.Model != nil && dsl.Model.APIKey != "" && dsl.Model.Model != "" {
		cfg := model.OpenAIConfig{
			APIKey:  dsl.Model.APIKey,
			Model:   dsl.Model.Model,
			BaseURL: dsl.Model.BaseURL,
		}
		if m, err := model.NewOpenAIChatModel(cfg); err == nil {
			return m
		}
	}
	// Fall back to env vars
	if m, err := NewDefaultChatModel(); err == nil && m != nil {
		return m
	}
	return nil
}

func buildToolkit(dsl *AgentDSL, skillDir string, env *runtimepkg.Environment, mcpTools []*tool.Tool) *tool.Toolkit {
	var tools []*tool.Tool
	var limiter *runtimepkg.RateLimiter
	if env != nil {
		limiter = env.KubectlLimit
	}
	tools = append(tools, newKubectlTool(limiter))

	useBuiltins := true
	if dsl.Toolkit != nil && !dsl.Toolkit.UseBuiltins {
		useBuiltins = false
	}
	if useBuiltins {
		tools = append(tools, tool.ExecuteShellCommandTool(), tool.ViewTextFileTool())
	}

	if dsl.Toolkit != nil && dsl.Toolkit.UseMCP && len(mcpTools) > 0 {
		filter := make(map[string]bool)
		for _, n := range dsl.Toolkit.MCPTools {
			filter[n] = true
		}
		for _, t := range mcpTools {
			if len(filter) == 0 || filter[t.Name] {
				tools = append(tools, t)
			}
		}
	}

	return tool.NewToolkit(tools...)
}

// LoadAgentDSL loads agent.yaml or SKILL.md frontmatter from skillDir.
func LoadAgentDSL(skillDir string) (*AgentDSL, error) {
	agentPath := filepath.Join(skillDir, "agent.yaml")
	if data, err := os.ReadFile(agentPath); err == nil {
		var dsl AgentDSL
		if err := yaml.Unmarshal(data, &dsl); err != nil {
			return nil, err
		}
		if dsl.Name != "" {
			return &dsl, nil
		}
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}
	fm, err := parseSKILLMDFrontmatter(data)
	if err != nil || fm == nil {
		return nil, fmt.Errorf("no valid SKILL.md frontmatter in %s", skillDir)
	}
	dsl := &AgentDSL{}
	if v, ok := fm["agent_name"]; ok {
		if s, ok := v.(string); ok && s != "" {
			dsl.Name = s
		}
	}
	if dsl.Name == "" {
		if v, ok := fm["name"]; ok {
			if s, ok := v.(string); ok {
				dsl.Name = s
			}
		}
	}
	if v, ok := fm["description"]; ok {
		if s, ok := v.(string); ok {
			dsl.Description = s
		}
	}
	if v, ok := fm["interval_seconds"]; ok {
		switch n := v.(type) {
		case int:
			dsl.IntervalSecond = n
		case float64:
			dsl.IntervalSecond = int(n)
		}
	}
	if v, ok := fm["report_section"]; ok {
		if s, ok := v.(string); ok {
			dsl.ReportSection = s
		}
	}
	if v, ok := fm["sub_skills"]; ok {
		if arr, ok := v.([]any); ok {
			for _, a := range arr {
				if s, ok := a.(string); ok {
					dsl.SubSkills = append(dsl.SubSkills, s)
				}
			}
		}
	}
	if v, ok := fm["read_only"]; ok {
		if b, ok := v.(bool); ok {
			dsl.ReadOnly = &b
		}
	}
	if dsl.Name == "" {
		return nil, fmt.Errorf("agent name required in %s", skillDir)
	}
	return dsl, nil
}

// RegisterAgentSkill loads skill content from skillDir and registers to Agent (for Summary etc).
func RegisterAgentSkill(skillDir string) (string, error) {
	mainPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(mainPath); err != nil {
		mainPath = filepath.Join(skillDir, "skill.md")
	}
	data, err := os.ReadFile(mainPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
