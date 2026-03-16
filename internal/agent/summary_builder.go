package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/tool"

	"github.com/alanfokco/kube-ops-agent-go/internal/report"
)

// BuildSummaryAgentFromSkillDir builds Summary ReActAgent from skill dir, mounts view_text_file, save_report, register_agent_skill.
func BuildSummaryAgentFromSkillDir(skillDir, reportDir string, m model.ChatModel) (*agent.ReActAgent, error) {
	skillContent, err := RegisterAgentSkill(skillDir)
	if err != nil {
		return nil, fmt.Errorf("load summary skill: %w", err)
	}

	saveReportTool := newSaveReportTool(reportDir)
	tk := tool.NewToolkit(
		tool.ViewTextFileTool(),
		saveReportTool,
		newRegisterAgentSkillTool(skillDir),
	)

	sysPrompt := fmt.Sprintf(`You are the Kubernetes Operation Summary Agent.

%s

Your tools:
- view_text_file: read file contents. Args: path or file_path (string)
- save_report: save the final Markdown report. Args: content (string, required)
- register_agent_skill: get the summary skill instructions from the skill directory

When you are done summarizing, call save_report with the complete Markdown report.
Return ONLY valid JSON for tool calls, then the final report.`, skillContent)

	return agent.NewReActAgent("SummaryAgent", sysPrompt, m, tk, nil), nil
}

func newSaveReportTool(reportDir string) *tool.Tool {
	return &tool.Tool{
		Name:        "save_report",
		Description: "Save the final Markdown report to disk. Args: content (string, required) - the full report Markdown.",
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			raw, ok := args["content"]
			if !ok {
				return nil, fmt.Errorf("content is required")
			}
			content, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("content must be a string")
			}
			if reportDir == "" {
				return nil, fmt.Errorf("report directory not configured")
			}
			if err := os.MkdirAll(reportDir, 0o755); err != nil {
				return nil, err
			}
			filename := report.ReportFilename(time.Now())
			path := filepath.Join(reportDir, filename)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return nil, err
			}
			return map[string]any{"path": path, "saved": true}, nil
		},
	}
}

func newRegisterAgentSkillTool(skillDir string) *tool.Tool {
	return &tool.Tool{
		Name:        "register_agent_skill",
		Description: "Get the summary agent skill instructions from the skill directory. Args: (none)",
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			content, err := RegisterAgentSkill(skillDir)
			if err != nil {
				return nil, err
			}
			return map[string]any{"skill_content": content, "skill_dir": skillDir}, nil
		},
	}
}
