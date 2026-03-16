package agent

import (
	"context"
	"fmt"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"
)

// SelfDrivenOrchestrator is a self-driven Orchestrator based on ThinkingAgent.
// Preferred for global inspection planning in intelligent mode.
type SelfDrivenOrchestrator struct {
	*ThinkingAgent
	registry Registry
}

func NewSelfDrivenOrchestrator(m model.ChatModel, reg Registry) *SelfDrivenOrchestrator {
	cfg := DefaultThinkingConfig()
	ta := NewThinkingAgent("SelfDrivenOrchestrator", m, nil, cfg)
	return &SelfDrivenOrchestrator{
		ThinkingAgent: ta,
		registry:      reg,
	}
}

// Orchestrate runs one self-driven planning flow, outputs AgentResult with RawPlanJSON as the plan.
func (o *SelfDrivenOrchestrator) Orchestrate(
	ctx context.Context,
	focusAreas []string,
) (*AgentResult, error) {
	agentsMeta := make([]map[string]any, 0, len(o.registry.Specs()))
	for _, sp := range o.registry.Specs() {
		agentsMeta = append(agentsMeta, SpecToMetaMap(sp))
	}

	task := fmt.Sprintf(
		"Perform a Kubernetes cluster health inspection with dynamic planning. Focus areas: %v",
		focusAreas,
	)

	ctxData := map[string]any{
		"available_agents": agentsMeta,
		"focus_areas":      focusAreas,
	}

	return o.ThinkAndExecute(ctx, task, ctxData)
}

