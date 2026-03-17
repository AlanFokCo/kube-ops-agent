package reasoning

import (
	"testing"
	"time"
)

// ---- ThinkingContext ----

func TestNewThinkingContext(t *testing.T) {
	ctx := NewThinkingContext("inspect cluster health")
	if ctx.Goal != "inspect cluster health" {
		t.Errorf("Goal = %q", ctx.Goal)
	}
	if ctx.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ctx.Depth != 0 {
		t.Errorf("expected Depth=0, got %d", ctx.Depth)
	}
	if ctx.MaxDepth != 3 {
		t.Errorf("expected MaxDepth=3, got %d", ctx.MaxDepth)
	}
	if ctx.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestThinkingContext_CanRecurse(t *testing.T) {
	ctx := NewThinkingContext("task")
	ctx.MaxDepth = 3
	ctx.Depth = 0
	if !ctx.CanRecurse() {
		t.Error("expected CanRecurse=true when depth < maxDepth")
	}
	ctx.Depth = 3
	if ctx.CanRecurse() {
		t.Error("expected CanRecurse=false when depth == maxDepth")
	}
	ctx.Depth = 5
	if ctx.CanRecurse() {
		t.Error("expected CanRecurse=false when depth > maxDepth")
	}
}

func TestThinkingContext_CreateChildContext(t *testing.T) {
	parent := NewThinkingContext("parent task")
	parent.AvailableTools = []string{"kubectl", "shell"}
	parent.Constraints = []string{"read-only"}
	parent.Depth = 1
	parent.MaxDepth = 3

	child := parent.CreateChildContext("sub task", map[string]any{"key": "val"})
	if child.Task != "sub task" {
		t.Errorf("Task = %q", child.Task)
	}
	if child.Depth != 2 {
		t.Errorf("expected Depth=2, got %d", child.Depth)
	}
	if child.MaxDepth != parent.MaxDepth {
		t.Errorf("MaxDepth should be inherited")
	}
	if child.ParentContext != parent {
		t.Error("expected ParentContext to be parent")
	}
	if len(child.AvailableTools) != 2 {
		t.Error("expected AvailableTools inherited")
	}
	if len(child.Constraints) != 1 {
		t.Error("expected Constraints inherited")
	}
}

func TestThinkingContext_CreateChildContext_NilSubContext(t *testing.T) {
	parent := NewThinkingContext("task")
	child := parent.CreateChildContext("sub", nil)
	if child.Context == nil {
		t.Error("expected non-nil Context even when nil passed")
	}
}

// ---- Analysis ----

func TestNewAnalysis(t *testing.T) {
	a := NewAnalysis("kubernetes", "complex", "Cluster has issues",
		[]string{"nodes", "pods"}, []string{"disk"}, []string{"restart pods"})
	if a.Domain != "kubernetes" {
		t.Errorf("Domain = %q", a.Domain)
	}
	if a.Complexity != "complex" {
		t.Errorf("Complexity = %q", a.Complexity)
	}
	if a.Summary != "Cluster has issues" {
		t.Errorf("Summary = %q", a.Summary)
	}
	if len(a.KeyAreas) != 2 {
		t.Errorf("expected 2 key areas, got %d", len(a.KeyAreas))
	}
	if len(a.PotentialRisks) != 1 {
		t.Errorf("expected 1 risk, got %d", len(a.PotentialRisks))
	}
	if len(a.Recommendations) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(a.Recommendations))
	}
}

func TestNewAnalysisFromDict(t *testing.T) {
	data := map[string]any{
		"complexity":           "medium",
		"needs_planning":       true,
		"reasoning":            "some reasoning",
		"confidence":           0.8,
		"missing_information":  []any{"resource limits"},
		"suggested_approach":   "check nodes first",
		"detected_issues":      []any{"high cpu"},
	}
	a := NewAnalysisFromDict(data)
	if a.Complexity != "medium" {
		t.Errorf("Complexity = %q", a.Complexity)
	}
	if !a.NeedsPlanning {
		t.Error("expected NeedsPlanning=true")
	}
	if a.Reasoning != "some reasoning" {
		t.Errorf("Reasoning = %q", a.Reasoning)
	}
	if a.Confidence != 0.8 {
		t.Errorf("Confidence = %f", a.Confidence)
	}
	if len(a.MissingInformation) != 1 {
		t.Errorf("expected 1 missing info, got %d", len(a.MissingInformation))
	}
	if a.SuggestedApproach != "check nodes first" {
		t.Errorf("SuggestedApproach = %q", a.SuggestedApproach)
	}
	if len(a.DetectedIssues) != 1 {
		t.Errorf("expected 1 detected issue, got %d", len(a.DetectedIssues))
	}
}

func TestNewAnalysisFromDict_Empty(t *testing.T) {
	a := NewAnalysisFromDict(map[string]any{})
	if a.Complexity != "simple" {
		t.Errorf("expected default complexity 'simple', got %q", a.Complexity)
	}
	if a.Confidence != 0.5 {
		t.Errorf("expected default confidence 0.5, got %f", a.Confidence)
	}
}

// ---- ThinkingResult ----

func TestThinkingResult_GetAllOutputs_Nil(t *testing.T) {
	var r *ThinkingResult
	outputs := r.GetAllOutputs()
	if outputs != nil {
		t.Error("expected nil for nil result")
	}
}

func TestThinkingResult_GetAllOutputs_Recursive(t *testing.T) {
	r := &ThinkingResult{
		Output: "root",
		SubResults: []*ThinkingResult{
			{Output: "child1"},
			{Output: "child2", SubResults: []*ThinkingResult{
				{Output: "grandchild"},
			}},
		},
	}
	outputs := r.GetAllOutputs()
	if len(outputs) != 4 {
		t.Errorf("expected 4 outputs, got %d", len(outputs))
	}
}

func TestThinkingResult_GetAllOutputs_NilOutput(t *testing.T) {
	r := &ThinkingResult{Output: nil}
	outputs := r.GetAllOutputs()
	if len(outputs) != 0 {
		t.Errorf("expected 0 outputs for nil Output, got %d", len(outputs))
	}
}

// ---- ThinkingPlan ----

func TestNewThinkingPlan(t *testing.T) {
	p := NewThinkingPlan("inspect nodes")
	if p.Goal != "inspect nodes" {
		t.Errorf("Goal = %q", p.Goal)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.Status != PlanStatusPlanning {
		t.Errorf("Status = %q", p.Status)
	}
	if p.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d", p.CurrentStep)
	}
}

func TestThinkingPlan_AddStep(t *testing.T) {
	p := NewThinkingPlan("goal")
	p.AddStep("check nodes", "NodeHealth", nil)
	p.AddStep("check pods", "PodHealth", map[string]any{"ns": "default"})

	if len(p.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(p.Steps))
	}
	if p.Steps[0].Description != "check nodes" {
		t.Errorf("Step[0].Description = %q", p.Steps[0].Description)
	}
	if p.Steps[0].Status != StepStatusPending {
		t.Errorf("Step[0].Status = %q", p.Steps[0].Status)
	}
	if p.Steps[0].ID == "" {
		t.Error("expected non-empty step ID")
	}
}

func TestThinkingPlan_MarkStepRunning(t *testing.T) {
	p := NewThinkingPlan("goal")
	p.AddStep("step1", "action1", nil)

	p.MarkStepRunning(0)
	if p.Steps[0].Status != StepStatusRunning {
		t.Errorf("expected Running, got %q", p.Steps[0].Status)
	}
	if p.Steps[0].StartedAt == nil {
		t.Error("expected non-nil StartedAt")
	}
	// Out of range - should not panic
	p.MarkStepRunning(-1)
	p.MarkStepRunning(100)
}

func TestThinkingPlan_MarkStepCompleted(t *testing.T) {
	p := NewThinkingPlan("goal")
	p.AddStep("step1", "action1", nil)
	p.MarkStepRunning(0)
	p.MarkStepCompleted(0, "done")

	if p.Steps[0].Status != StepStatusCompleted {
		t.Errorf("expected Completed, got %q", p.Steps[0].Status)
	}
	if p.Steps[0].Result != "done" {
		t.Errorf("Result = %q", p.Steps[0].Result)
	}
	if p.Steps[0].FinishedAt == nil {
		t.Error("expected non-nil FinishedAt")
	}
	if p.CurrentStep != 1 {
		t.Errorf("expected CurrentStep=1, got %d", p.CurrentStep)
	}
	// Out of range - should not panic
	p.MarkStepCompleted(-1, "x")
	p.MarkStepCompleted(100, "x")
}

func TestThinkingPlan_MarkStepFailed(t *testing.T) {
	p := NewThinkingPlan("goal")
	p.AddStep("step1", "action1", nil)
	p.MarkStepFailed(0, "timeout")

	if p.Steps[0].Status != StepStatusFailed {
		t.Errorf("expected Failed, got %q", p.Steps[0].Status)
	}
	if p.Steps[0].Error != "timeout" {
		t.Errorf("Error = %q", p.Steps[0].Error)
	}
	if p.Status != PlanStatusFailed {
		t.Errorf("Plan status = %q", p.Status)
	}
	// Out of range - should not panic
	p.MarkStepFailed(-1, "x")
	p.MarkStepFailed(100, "x")
}

// ---- ReasoningLoop ----

func TestNewReasoningLoop(t *testing.T) {
	loop := NewReasoningLoop(5)
	if loop.MaxIterations != 5 {
		t.Errorf("MaxIterations = %d", loop.MaxIterations)
	}
	if loop.ID == "" {
		t.Error("expected non-empty ID")
	}
	if loop.Iteration != 0 {
		t.Errorf("Iteration = %d", loop.Iteration)
	}
}

func TestReasoningLoop_AddReflection(t *testing.T) {
	loop := NewReasoningLoop(3)
	newPlan := NewThinkingPlan("revised goal")

	loop.AddReflection("found issues", "revise plan", newPlan)
	if loop.Iteration != 1 {
		t.Errorf("expected Iteration=1, got %d", loop.Iteration)
	}
	if len(loop.Reflections) != 1 {
		t.Fatalf("expected 1 reflection, got %d", len(loop.Reflections))
	}
	if loop.Reflections[0].Observation != "found issues" {
		t.Errorf("Observation = %q", loop.Reflections[0].Observation)
	}
	if loop.Plan != newPlan {
		t.Error("expected loop.Plan updated to newPlan")
	}

	// Add reflection without new plan
	loop.AddReflection("check again", "continue", nil)
	if loop.Iteration != 2 {
		t.Errorf("expected Iteration=2, got %d", loop.Iteration)
	}
	if loop.Plan != newPlan {
		t.Error("plan should not change when nil newPlan")
	}
}

// ---- PlanRevision ----

func TestPlanRevision_Apply_NilRevision(t *testing.T) {
	var r *PlanRevision
	result := r.Apply()
	if result != nil {
		t.Error("expected nil for nil revision")
	}
}

func TestPlanRevision_Apply_NilOriginalPlan(t *testing.T) {
	r := &PlanRevision{OriginalPlan: nil}
	result := r.Apply()
	if result != nil {
		t.Error("expected nil when OriginalPlan is nil")
	}
}

func TestPlanRevision_Apply(t *testing.T) {
	original := NewThinkingPlan("original goal")
	original.AddStep("step1", "action1", nil)
	original.AddStep("step2", "action2", nil)
	// Mark step1 completed so it gets kept
	original.MarkStepCompleted(0, "done")

	now := time.Now()
	newStep := PlanStep{
		ID:          "new-step",
		Description: "new step",
		Action:      "action3",
		Status:      StepStatusPending,
		StartedAt:   &now,
	}

	r := &PlanRevision{
		OriginalPlan:   original,
		Reason:         "replan needed",
		NewSteps:       []PlanStep{newStep},
		RemovedStepIDs: []string{"nonexistent"},
	}

	revised := r.Apply()
	if revised == nil {
		t.Fatal("expected non-nil revised plan")
	}
	if revised.Goal != "original goal" {
		t.Errorf("Goal = %q", revised.Goal)
	}
	if revised.Status != PlanStatusExecuting {
		t.Errorf("Status = %q", revised.Status)
	}
	// Should have: 1 kept completed step + 1 new step = 2
	if len(revised.Steps) != 2 {
		t.Errorf("expected 2 steps (1 kept + 1 new), got %d", len(revised.Steps))
	}
	if revised.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d", revised.CurrentStep)
	}
}

func TestPlanRevision_Apply_RemoveStep(t *testing.T) {
	original := NewThinkingPlan("goal")
	original.AddStep("step1", "action1", nil)
	// Mark step1 completed
	original.MarkStepCompleted(0, "done")
	stepID := original.Steps[0].ID

	r := &PlanRevision{
		OriginalPlan:   original,
		Reason:         "remove old step",
		RemovedStepIDs: []string{stepID},
	}

	revised := r.Apply()
	if revised == nil {
		t.Fatal("expected non-nil plan")
	}
	// Step was completed but removed, so it should not be in revised
	if len(revised.Steps) != 0 {
		t.Errorf("expected 0 steps after removal, got %d", len(revised.Steps))
	}
}

// ---- generateID / randomString ----

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty IDs")
	}
	// IDs should have reasonable length
	if len(id1) < 14 {
		t.Errorf("ID too short: %q", id1)
	}
}

// ---- Constants ----

func TestReplanReasonConstants(t *testing.T) {
	consts := []string{
		ReplanReasonUnexpected,
		ReplanReasonFailed,
		ReplanReasonIncomplete,
		ReplanReasonNewContext,
		ReplanReasonUserRequest,
	}
	for _, c := range consts {
		if c == "" {
			t.Error("replan reason constant should not be empty")
		}
	}
}
