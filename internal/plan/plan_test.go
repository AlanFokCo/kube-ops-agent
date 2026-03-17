package plan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/message"

	"github.com/alanfokco/kube-ops-agent-go/internal/reasoning"
)

// ---- LoadFromFile ----

func TestLoadFromFile(t *testing.T) {
	path := filepath.Join("..", "..", "kubernetes-ops-agent", "workflow.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("workflow.yaml not found")
	}
	p, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(p.Steps) == 0 {
		t.Fatal("expected steps")
	}
	if p.Priority == "" {
		t.Error("expected priority")
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFromFile_NoSteps(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "workflow*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("assessment: test\npriority: normal\n")
	f.Close()
	_, err = LoadFromFile(f.Name())
	if err == nil || !strings.Contains(err.Error(), "no steps") {
		t.Fatalf("expected 'no steps' error, got: %v", err)
	}
}

func TestLoadFromFile_DefaultPriority(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "workflow*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("assessment: test\nsteps:\n  - agents: [A]\n    mode: parallel\n")
	f.Close()
	p, err := LoadFromFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Priority != "normal" {
		t.Errorf("expected default priority 'normal', got %q", p.Priority)
	}
}

// ---- FromJSON ----

func TestFromJSON_Basic(t *testing.T) {
	json := `{
		"assessment": "test assessment",
		"priority": "high",
		"reasoning": "test reason",
		"allow_replan": true,
		"focus_areas": ["nodes"],
		"skip_agents": ["AgentX"],
		"skip_reasoning": "not needed",
		"steps": [
			{"agents": ["A", "B"], "mode": "parallel", "focus_areas": ["nodes"], "depends_on": [], "timeout_seconds": 120},
			{"agents": ["C"], "mode": "sequential", "depends_on": ["A"]}
		]
	}`
	p, err := FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}
	if p.Assessment != "test assessment" {
		t.Errorf("Assessment = %q", p.Assessment)
	}
	if p.Priority != "high" {
		t.Errorf("Priority = %q", p.Priority)
	}
	if !p.AllowReplan {
		t.Error("expected AllowReplan true")
	}
	if len(p.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(p.Steps))
	}
	if p.Steps[0].Mode != ModeParallel {
		t.Errorf("step[0] mode = %q", p.Steps[0].Mode)
	}
	if p.Steps[1].Mode != ModeSequential {
		t.Errorf("step[1] mode = %q", p.Steps[1].Mode)
	}
	if len(p.SkipAgents) != 1 || p.SkipAgents[0] != "AgentX" {
		t.Errorf("SkipAgents = %v", p.SkipAgents)
	}
}

func TestFromJSON_LegacyAgentsToInvoke(t *testing.T) {
	json := `{"agents_to_invoke": ["A", "B"], "focus_areas": ["pods"]}`
	p, err := FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Steps))
	}
	if p.Steps[0].Mode != ModeParallel {
		t.Errorf("expected parallel, got %q", p.Steps[0].Mode)
	}
	if len(p.Steps[0].Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(p.Steps[0].Agents))
	}
}

func TestFromJSON_OverallAssessmentFallback(t *testing.T) {
	json := `{"overall_assessment": "fallback", "steps": [{"agents": ["A"], "mode": "parallel"}]}`
	p, err := FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}
	if p.Assessment != "fallback" {
		t.Errorf("Assessment = %q", p.Assessment)
	}
}

func TestFromJSON_DefaultPriority(t *testing.T) {
	json := `{"steps": [{"agents": ["A"], "mode": "parallel"}]}`
	p, err := FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}
	if p.Priority != "normal" {
		t.Errorf("expected 'normal', got %q", p.Priority)
	}
}

func TestFromJSON_NoSteps(t *testing.T) {
	json := `{"assessment": "x"}`
	_, err := FromJSON(json)
	if err == nil {
		t.Fatal("expected error for no steps")
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	_, err := FromJSON("not-json")
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

// ---- Raw ----

func TestRaw(t *testing.T) {
	raw := `{"steps": [{"agents": ["A"], "mode": "parallel"}]}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if p.Raw() != raw {
		t.Errorf("Raw() = %q, want %q", p.Raw(), raw)
	}
}

// ---- GetAllAgents ----

func TestGetAllAgents(t *testing.T) {
	p := &InspectionPlan{
		Steps: []Step{
			{Agents: []string{"A", "B"}},
			{Agents: []string{"B", "C"}},
		},
	}
	agents := p.GetAllAgents()
	if len(agents) != 3 {
		t.Errorf("expected 3 unique agents, got %d: %v", len(agents), agents)
	}
}

// ---- IsEmpty ----

func TestIsEmpty_TrueNoSteps(t *testing.T) {
	p := &InspectionPlan{}
	if !p.IsEmpty() {
		t.Error("expected IsEmpty true for no steps")
	}
}

func TestIsEmpty_TrueEmptyAgents(t *testing.T) {
	p := &InspectionPlan{Steps: []Step{{Agents: []string{}}}}
	if !p.IsEmpty() {
		t.Error("expected IsEmpty true for step with no agents")
	}
}

func TestIsEmpty_False(t *testing.T) {
	p := &InspectionPlan{Steps: []Step{{Agents: []string{"A"}}}}
	if p.IsEmpty() {
		t.Error("expected IsEmpty false")
	}
}

// ---- AdaptivePlanExecutor ----

func TestNewAdaptivePlanExecutor_Defaults(t *testing.T) {
	exec := NewAdaptivePlanExecutor(nil, nil, 0, 0, 0)
	if exec.DefaultTimeout != 300 {
		t.Errorf("DefaultTimeout = %d", exec.DefaultTimeout)
	}
	if exec.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d", exec.MaxDepth)
	}
	if exec.MaxParallel != 5 {
		t.Errorf("MaxParallel = %d", exec.MaxParallel)
	}
}

func TestAdaptivePlanExecutor_Execute_NoAgentExecutor(t *testing.T) {
	exec := NewAdaptivePlanExecutor(nil, nil, 10, 2, 3)
	plan := &reasoning.ThinkingPlan{
		Goal: "test goal",
		Steps: []reasoning.PlanStep{
			{ID: "s1", Action: "check-nodes"},
		},
	}
	res := exec.Execute(context.Background(), plan, 0, nil)
	if !res.Success {
		t.Errorf("expected success when no AgentExecutor, got: %s", res.Error)
	}
	if len(res.SubResults) != 1 {
		t.Errorf("expected 1 sub-result, got %d", len(res.SubResults))
	}
}

func TestAdaptivePlanExecutor_Execute_MaxDepth(t *testing.T) {
	exec := NewAdaptivePlanExecutor(nil, nil, 10, 2, 3)
	plan := &reasoning.ThinkingPlan{Goal: "test"}
	res := exec.Execute(context.Background(), plan, 5, nil) // depth > maxDepth
	if res.Success {
		t.Error("expected failure when max depth exceeded")
	}
	if !strings.Contains(res.Error, "max recursion depth") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

func TestAdaptivePlanExecutor_Execute_WithExecutor(t *testing.T) {
	agentExec := func(ctx context.Context, name string, input map[string]any) (*message.Msg, error) {
		return message.NewMsg(name, message.RoleAssistant, "ok"), nil
	}
	exec := NewAdaptivePlanExecutor(agentExec, nil, 10, 2, 3)
	plan := &reasoning.ThinkingPlan{
		Goal: "test",
		Steps: []reasoning.PlanStep{
			{ID: "s1", Action: "NodeHealth"},
		},
	}
	res := exec.Execute(context.Background(), plan, 0, map[string]any{"key": "val"})
	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}
}

func TestAdaptivePlanExecutor_Execute_AgentError(t *testing.T) {
	agentExec := func(ctx context.Context, name string, input map[string]any) (*message.Msg, error) {
		return nil, context.DeadlineExceeded
	}
	exec := NewAdaptivePlanExecutor(agentExec, nil, 10, 2, 3)
	plan := &reasoning.ThinkingPlan{
		Goal: "test",
		Steps: []reasoning.PlanStep{
			{ID: "s1", Action: "NodeHealth"},
		},
	}
	res := exec.Execute(context.Background(), plan, 0, nil)
	if res.Success {
		t.Error("expected failure when agent returns error")
	}
	if len(res.SubResults) == 0 || res.SubResults[0].NeedsReplan != true {
		t.Error("expected sub-result with NeedsReplan=true")
	}
}

func TestAdaptiveExecutionResult_GetAllOutputs(t *testing.T) {
	r := &AdaptiveExecutionResult{
		Output: "root",
		SubResults: []*AdaptiveExecutionResult{
			{Output: "child1"},
			{Output: "child2", SubResults: []*AdaptiveExecutionResult{
				{Output: "grandchild"},
			}},
		},
	}
	outputs := r.GetAllOutputs()
	if len(outputs) != 4 {
		t.Errorf("expected 4 outputs, got %d", len(outputs))
	}
}
