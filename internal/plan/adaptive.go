package plan

import (
	"context"
	"strings"
	"time"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"

	"github.com/alanfokco/kube-ops-agent-go/internal/reasoning"
)

// AdaptiveExecutionResult is adaptive execution result with recursive sub-results.
type AdaptiveExecutionResult struct {
	Success        bool
	Output         any
	SubResults     []*AdaptiveExecutionResult
	Depth          int
	StepID         string
	NeedsReplan    bool
	ReplanReason   string
	DurationSeconds float64
	Error          string
}

// GetAllOutputs collects output from this and all sub-results.
func (r *AdaptiveExecutionResult) GetAllOutputs() []any {
	out := []any{}
	if r.Output != nil {
		out = append(out, r.Output)
	}
	for _, sub := range r.SubResults {
		out = append(out, sub.GetAllOutputs()...)
	}
	return out
}

// AgentExecutorFunc is function type for executing single agent.
type AgentExecutorFunc func(ctx context.Context, agentName string, input map[string]any) (*message.Msg, error)

// PlanCreatorFunc is function type for creating sub-plans.
type PlanCreatorFunc func(ctx context.Context, action string, context map[string]any) (*reasoning.ThinkingPlan, error)

// AdaptivePlanExecutor is plan executor with recursion and replanning.
type AdaptivePlanExecutor struct {
	AgentExecutor AgentExecutorFunc
	PlanCreator   PlanCreatorFunc
	DefaultTimeout int
	MaxDepth      int
	MaxParallel   int
}

// NewAdaptivePlanExecutor creates adaptive plan executor.
func NewAdaptivePlanExecutor(
	exec AgentExecutorFunc,
	creator PlanCreatorFunc,
	defaultTimeout, maxDepth, maxParallel int,
) *AdaptivePlanExecutor {
	if defaultTimeout <= 0 {
		defaultTimeout = 300
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxParallel <= 0 {
		maxParallel = 5
	}
	return &AdaptivePlanExecutor{
		AgentExecutor:  exec,
		PlanCreator:    creator,
		DefaultTimeout: defaultTimeout,
		MaxDepth:       maxDepth,
		MaxParallel:    maxParallel,
	}
}

// Execute recursively executes ThinkingPlan.
func (e *AdaptivePlanExecutor) Execute(
	ctx context.Context,
	plan *reasoning.ThinkingPlan,
	depth int,
	contextData map[string]any,
) *AdaptiveExecutionResult {
	start := time.Now()
	if depth > e.MaxDepth {
		return &AdaptiveExecutionResult{
			Success:         false,
			Error:           "max recursion depth reached",
			Depth:           depth,
			DurationSeconds: time.Since(start).Seconds(),
		}
	}
	if contextData == nil {
		contextData = make(map[string]any)
	}

	var subResults []*AdaptiveExecutionResult
	allSuccess := true

	for _, step := range plan.Steps {
		stepRes := e.executeStep(ctx, &step, depth, contextData)
		subResults = append(subResults, stepRes)

		if !stepRes.Success {
			allSuccess = false
		}

		// If replan needed and planCreator exists, create sub-plan and recurse
		if stepRes.NeedsReplan && depth < e.MaxDepth && e.PlanCreator != nil {
			if step.SubPlan == nil {
				if subPlan, err := e.PlanCreator(ctx, step.Action, map[string]any{
					"reason":  stepRes.ReplanReason,
					"context": contextData,
				}); err == nil && subPlan != nil {
					subRes := e.Execute(ctx, subPlan, depth+1, contextData)
					subResults = append(subResults, subRes)
				}
			} else {
				subRes := e.Execute(ctx, step.SubPlan, depth+1, contextData)
				subResults = append(subResults, subRes)
		}
	}
}

	return &AdaptiveExecutionResult{
		Success:         allSuccess,
		Output:          plan.Goal,
		SubResults:      subResults,
		Depth:           depth,
		DurationSeconds: time.Since(start).Seconds(),
	}
}

func (e *AdaptivePlanExecutor) executeStep(
	ctx context.Context,
	step *reasoning.PlanStep,
	depth int,
	contextData map[string]any,
) *AdaptiveExecutionResult {
	start := time.Now()
	if e.AgentExecutor == nil {
		return &AdaptiveExecutionResult{
			Success:         true,
			Output:          "step recorded: " + step.Action,
			Depth:           depth,
			StepID:          step.ID,
			DurationSeconds: time.Since(start).Seconds(),
		}
	}

	timeout := time.Duration(e.DefaultTimeout) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msg, err := e.AgentExecutor(runCtx, step.Action, contextData)
	if err != nil {
		return &AdaptiveExecutionResult{
			Success:         false,
			Error:           err.Error(),
			Depth:           depth,
			StepID:          step.ID,
			NeedsReplan:     true,
			ReplanReason:    "execution failed: " + err.Error(),
			DurationSeconds: time.Since(start).Seconds(),
		}
	}

	content := ""
	if msg != nil {
		if txt := msg.GetTextContent(""); txt != nil {
			content = *txt
		}
	}

	// Heuristic: detect complexity metrics, trigger replanning
	needsReplan := false
	replanReason := ""
	lower := strings.ToLower(content)
	for _, kw := range []string{"complex", "multiple issues", "further investigation", "unclear"} {
		if strings.Contains(lower, kw) {
			needsReplan = true
			replanReason = "complexity detected in results"
			break
		}
	}

	return &AdaptiveExecutionResult{
		Success:         true,
		Output:          msg,
		Depth:           depth,
		StepID:          step.ID,
		NeedsReplan:     needsReplan,
		ReplanReason:    replanReason,
		DurationSeconds: time.Since(start).Seconds(),
	}
}

