package agent

import (
	"context"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"

	"github.com/alanfokco/kube-ops-agent-go/internal/plan"
)

// SelfDrivenSummary is a self-driven summary Agent based on ThinkingAgent.
type SelfDrivenSummary struct {
	*ThinkingAgent
}

func NewSelfDrivenSummary(m model.ChatModel) *SelfDrivenSummary {
	cfg := DefaultThinkingConfig()
	ta := NewThinkingAgent("SelfDrivenSummary", m, nil, cfg)
	return &SelfDrivenSummary{
		ThinkingAgent: ta,
	}
}

// SummarizeThinking performs self-driven summary via ThinkingAgent.
func (s *SelfDrivenSummary) SummarizeThinking(
	ctx context.Context,
	p *plan.InspectionPlan,
	results map[string]string,
) (*AgentResult, error) {
	task := "Summarize Kubernetes inspection results into a single, well-structured Markdown report."

	ctxData := map[string]any{
		"inspection_plan": p,
		"worker_reports":  results,
	}

	return s.ThinkAndExecute(ctx, task, ctxData)
}

