package agent

import (
	"context"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
)

// SelfDrivenWorker is a self-driven Worker based on ThinkingAgent.
type SelfDrivenWorker struct {
	*ThinkingAgent
	spec Spec
}

func NewSelfDrivenWorker(m model.ChatModel, spec Spec) *SelfDrivenWorker {
	cfg := DefaultThinkingConfig()
	ta := NewThinkingAgent(spec.Name, m, nil, cfg)

	return &SelfDrivenWorker{
		ThinkingAgent: ta,
		spec:          spec,
	}
}

// Inspect runs one self-driven inspection.
func (w *SelfDrivenWorker) Inspect(
	ctx context.Context,
	focusAreas []string,
	orchestratorContext string,
) (*AgentResult, error) {
	task := "Execute Kubernetes worker inspection with intelligent planning."

	ctxData := map[string]any{
		"skill_name":          w.spec.SkillName,
		"skill_description":   w.spec.Description,
		"skill_dir":           w.spec.SkillDir,
		"report_section":      w.spec.ReportSection,
		"focus_areas":         focusAreas,
		"orchestrator_context": orchestratorContext,
	}

	return w.ThinkAndExecute(ctx, task, ctxData)
}

