package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/pkg/agentscope/model"

	"github.com/alanfokco/kube-ops-agent-go/internal/plan"
)

// SummaryAgent aggregates worker results into the final report.
type SummaryAgent struct {
	Model model.ChatModel
}

func NewSummaryAgent(m model.ChatModel) *SummaryAgent {
	return &SummaryAgent{Model: m}
}

// Summarize generates Markdown report from plan and agent outputs.
func (s *SummaryAgent) Summarize(
	ctx context.Context,
	p *plan.InspectionPlan,
	results map[string]string,
) (string, error) {
	if s.Model == nil {
		return "", fmt.Errorf("summary chat model is nil")
	}

	var b strings.Builder
	for name, content := range results {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", name, content)
	}

	sys := `You are the Kubernetes Operation Summary Agent.

You will receive:
- An inspection plan in JSON form
- A set of section reports from worker agents

Your job:
- Produce a single, well-structured Markdown report for SREs and platform engineers
- Highlight critical issues, recommended actions, and overall cluster health

Return ONLY Markdown, no JSON.`

	user := fmt.Sprintf("Inspection plan JSON:\n```json\n%s\n```\n\nWorker reports:\n%s",
		p.Raw(), b.String(),
	)

	msgs := []*message.Msg{
		message.NewMsg("system", message.RoleSystem, sys),
		message.NewMsg("user", message.RoleUser, user),
	}

	resp, err := s.Model.Chat(ctx, msgs)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Msg == nil {
		return "", fmt.Errorf("summary agent returned nil response")
	}
	text := resp.Msg.GetTextContent("")
	if text == nil {
		return "", fmt.Errorf("summary agent returned empty content")
	}
	return *text, nil
}

