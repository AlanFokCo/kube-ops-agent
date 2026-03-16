package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"

	"github.com/alanfokco/kube-ops-agent-go/internal/plan"
)

// OrchestratorAgent generates inspection plans from available Worker agents.
type OrchestratorAgent struct {
	Model model.ChatModel
}

func NewOrchestratorAgent(m model.ChatModel) *OrchestratorAgent {
	return &OrchestratorAgent{Model: m}
}

// Plan generates InspectionPlan from current available agent list and simple context.
func (o *OrchestratorAgent) Plan(ctx context.Context, specs []Spec, highLevelTask string) (*plan.InspectionPlan, error) {
	if o.Model == nil {
		return nil, fmt.Errorf("orchestrator chat model is nil")
	}

	available := make([]map[string]any, 0, len(specs))
	for _, sp := range specs {
		available = append(available, SpecToMetaMap(sp))
	}
	agentsJSON, _ := json.MarshalIndent(available, "", "  ")

	sys := fmt.Sprintf(`You are the Kubernetes Cluster Inspection Orchestrator Agent.

Your job:
- Analyze the task and available worker agents
- Create a dynamic inspection plan in JSON

Available worker agents (name, description, focus_area):
%s

You MUST output ONLY a single valid JSON object, no markdown fences, no extra text.
The JSON schema:
{
  "assessment": "string",
  "priority": "critical|high|normal|low",
  "steps": [
    {
      "agents": ["AgentName1", "AgentName2"],
      "mode": "parallel|sequential",
      "focus_areas": ["..."],
      "depends_on": ["AgentName1"],
      "condition": "optional text"
    }
  ],
  "reasoning": "why you chose this",
  "allow_replan": false,
  "skip_agents": ["AgentNameX"],
  "skip_reasoning": "why skipped"
}`, string(agentsJSON))

	user := fmt.Sprintf("Plan a cluster inspection for task: %s", highLevelTask)

	msgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sys),
		message.NewMsg("user", message.RoleUser, user),
	}

	resp, err := o.Model.Chat(ctx, msgs)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("orchestrator returned nil response")
	}
	text := resp.Msg.GetTextContent("")
	if text == nil {
		return nil, fmt.Errorf("orchestrator returned empty content")
	}
	planText := strings.TrimSpace(*text)
	// Strip possible ```json wrapper
	if strings.HasPrefix(planText, "```") {
		planText = strings.TrimPrefix(planText, "```json")
		planText = strings.TrimPrefix(planText, "```")
		if idx := strings.LastIndex(planText, "```"); idx >= 0 {
			planText = planText[:idx]
		}
		planText = strings.TrimSpace(planText)
	}
	return plan.FromJSON(planText)
}

