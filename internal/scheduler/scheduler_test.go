package scheduler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	"github.com/alanfokco/kube-ops-agent-go/internal/plan"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// mockRegistry implements agent.Registry for testing.
type mockRegistry struct {
	specs []agent.Spec
}

func (r *mockRegistry) Specs() []agent.Spec { return r.specs }
func (r *mockRegistry) SpecByName(name string) (agent.Spec, bool) {
	for _, s := range r.specs {
		if s.Name == name {
			return s, true
		}
	}
	return agent.Spec{}, false
}

func newTestScheduler(mode Mode) *Scheduler {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{
			{Name: "NodeHealth", IntervalSecond: 300},
			{Name: "PodHealth", IntervalSecond: 600},
		},
	}
	return NewWithOptions(mode, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
}

func TestNew_Simple(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

func TestNew_Intelligent(t *testing.T) {
	s := newTestScheduler(ModeIntelligent)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

func TestNewWithCheckInterval_Default(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	s := NewWithCheckInterval(ModeSimple, reg, nil, env, "", 0, nil, nil, nil, nil)
	if s.checkInterval != 10*time.Second {
		t.Errorf("expected default 10s interval, got %v", s.checkInterval)
	}
}

func TestNewWithOptions_CustomInterval(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 30*time.Second, 0, nil, nil, nil, nil)
	if s.checkInterval != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", s.checkInterval)
	}
}

func TestWithWorkflowPath(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 10*time.Second, 0, nil, nil, nil, nil, WithWorkflowPath("/tmp/workflow.yaml"))
	if s.workflowPath != "/tmp/workflow.yaml" {
		t.Errorf("workflowPath = %q", s.workflowPath)
	}
}

func TestGetStatus_NotRunning(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	status := s.GetStatus()
	if status["running"] != false {
		t.Errorf("expected running=false, got %v", status["running"])
	}
	if status["mode"] != "simple" {
		t.Errorf("expected mode='simple', got %v", status["mode"])
	}
}

func TestGetStatus_Intelligent(t *testing.T) {
	s := newTestScheduler(ModeIntelligent)
	status := s.GetStatus()
	if status["mode"] != "intelligent" {
		t.Errorf("expected mode='intelligent', got %v", status["mode"])
	}
}

func TestModeString_Simple(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	if s.modeString() != "simple" {
		t.Errorf("expected 'simple', got %q", s.modeString())
	}
}

func TestModeString_Intelligent(t *testing.T) {
	s := newTestScheduler(ModeIntelligent)
	if s.modeString() != "intelligent" {
		t.Errorf("expected 'intelligent', got %q", s.modeString())
	}
}

func TestCreateFallbackPlan_Empty(t *testing.T) {
	result := createFallbackPlan(nil)
	if result != nil {
		t.Error("expected nil for empty specs")
	}
}

func TestCreateFallbackPlan_WithSpecs(t *testing.T) {
	specs := []agent.Spec{
		{Name: "NodeHealth"},
		{Name: "PodHealth"},
	}
	p := createFallbackPlan(specs)
	if p == nil {
		t.Fatal("expected non-nil fallback plan")
	}
	if p.Priority != "medium" {
		t.Errorf("expected 'medium' priority, got %q", p.Priority)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Steps))
	}
	if len(p.Steps[0].Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(p.Steps[0].Agents))
	}
	if p.Steps[0].Mode != plan.ModeParallel {
		t.Errorf("expected parallel mode, got %q", p.Steps[0].Mode)
	}
}

func TestScheduler_Start_Stop(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	// Double start should be idempotent
	s.Start(ctx)
	s.Stop()
}

func TestScheduler_Stop_NotRunning(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	// Stop without Start should not panic
	s.Stop()
}

func TestScheduler_WriteReport_EmptyDir(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	s.reportDir = ""
	err := s.writeReport("# Test Report")
	if err != nil {
		t.Errorf("writeReport with empty reportDir should be no-op: %v", err)
	}
}

func TestScheduler_WriteReport_ValidDir(t *testing.T) {
	s := newTestScheduler(ModeSimple)
	s.reportDir = t.TempDir()
	err := s.writeReport("# Test Report\n\nContent here.")
	if err != nil {
		t.Fatalf("writeReport: %v", err)
	}
}

// ---- Test New directly ----
func TestNew_Direct(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	s := New(ModeSimple, reg, nil, env, "/tmp", nil, nil, nil, nil)
	if s == nil {
		t.Fatal("expected non-nil scheduler from New")
	}
}

// ---- runSimpleRound with empty registry (no-op) ----
func TestRunSimpleRound_EmptyRegistry(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{specs: nil}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	// runSimpleRound with empty registry should be a no-op
	ctx := context.Background()
	s.runSimpleRound(ctx) // should not panic
}

func TestRunSimpleRound_NoIntervalSpecs(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{
			{Name: "NoInterval", IntervalSecond: 0}, // skip because interval=0
		},
	}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	ctx := context.Background()
	s.runSimpleRound(ctx) // should not panic because iv<=0 leads to continue
}

func TestRunSimpleRound_WithIntervalOverride(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{
			{Name: "Agent1", IntervalSecond: 300},
		},
	}
	// Mark Agent1 as recently run so it is NOT due during this round
	now := time.Now()
	env.State.UpdateAgent("Agent1", &now, true)
	// Use interval override of 999999 seconds so no agents are due
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 5*time.Second, 999999, nil, nil, nil, nil)
	ctx := context.Background()
	s.runSimpleRound(ctx) // no agents should be due with huge interval + recent lastRun
}

// ---- loop with short interval (runs once then cancel) ----
func TestScheduler_Loop_CancelImmediate(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{specs: nil}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 100*time.Millisecond, 0, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	// Let loop run at least one tick
	time.Sleep(150 * time.Millisecond)
	cancel() // This cancels the context, loop should exit
	s.Stop()
}

func TestScheduler_Loop_Intelligent_NoOrch(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{specs: nil}
	s := NewWithOptions(ModeIntelligent, reg, nil, env, "", 100*time.Millisecond, 0, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	cancel()
	s.Stop()
}

// ---- runIntelligentRound falls back to simple when no orchestrator ----
func TestRunIntelligentRound_FallsBackToSimple(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{specs: nil}
	// No orchestrator set - should fall back to runSimpleRound
	s := NewWithOptions(ModeIntelligent, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	ctx := context.Background()
	s.runIntelligentRound(ctx) // should not panic - falls back to simple with empty registry
}

func TestRunIntelligentRound_WithWorkflowPath(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{specs: nil}
	s := NewWithOptions(ModeIntelligent, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	// Set a workflow path that doesn't exist - should not panic
	s.workflowPath = "/nonexistent/workflow.yaml"
	ctx := context.Background()
	s.runIntelligentRound(ctx) // file not found, falls back to simple
}

// ---- RunOneRound and runOneRoundSync with a non-nil executor ----

func TestRunOneRound_WithNilModelExecutor(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{{Name: "Agent1", IntervalSecond: 300}},
	}
	// NewExecutor with nil model - Execute returns error but doesn't panic
	exec := agent.NewExecutor(reg, env, nil)
	s := NewWithOptions(ModeSimple, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)

	specs := []agent.Spec{{Name: "Agent1"}}
	done := make(chan struct{})
	go func() {
		s.runOneRoundSync(context.Background(), specs)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("runOneRoundSync timed out")
	}
}

func TestRunOneRound_Async(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{{Name: "Agent1"}},
	}
	exec := agent.NewExecutor(reg, env, nil)
	s := NewWithOptions(ModeSimple, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)

	// RunOneRound is asynchronous - just ensure it doesn't panic
	s.RunOneRound([]agent.Spec{{Name: "Agent1"}})
	// Give the goroutine a moment
	time.Sleep(100 * time.Millisecond)
}

func TestRunOneRound_EmptySpecs(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	exec := agent.NewExecutor(reg, env, nil)
	s := NewWithOptions(ModeSimple, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	// Empty specs - runOneRoundSync should be a no-op
	s.runOneRoundSync(context.Background(), nil)
}

// ---- runSimpleRound with agents that ARE due ----

func TestRunSimpleRound_WithDueAgent(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{
			{Name: "Agent1", IntervalSecond: 1},
		},
	}
	// Mark agent as last-run 2 seconds ago so it IS due (interval=1s)
	lastRun := time.Now().Add(-2 * time.Second)
	env.State.UpdateAgent("Agent1", &lastRun, true)
	exec := agent.NewExecutor(reg, env, nil)
	s := NewWithOptions(ModeSimple, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	ctx := context.Background()
	s.runSimpleRound(ctx) // Agent1 should be due and execute (nil model returns error - ignored)
}

func TestRunSimpleRound_NeverRun_WithExec(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{
			{Name: "Agent1", IntervalSecond: 300},
		},
	}
	// Agent never run - so it IS due; exec has nil model so returns error (not panic)
	exec := agent.NewExecutor(reg, env, nil)
	s := NewWithOptions(ModeSimple, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	ctx := context.Background()
	s.runSimpleRound(ctx)
}

// ---- runIntelligentRound with a static workflow file ----

func writeTestWorkflow(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/workflow.yaml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTestWorkflow: %v", err)
	}
	return path
}

func TestRunIntelligentRound_WithStaticWorkflow(t *testing.T) {
	workflowYAML := `priority: high
steps:
  - mode: parallel
    agents:
      - name: Agent1
        focus_areas: []
`
	path := writeTestWorkflow(t, workflowYAML)

	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{
		specs: []agent.Spec{{Name: "Agent1", IntervalSecond: 300}},
	}
	// Use non-nil exec with nil model - Execute returns error, does not panic
	exec := agent.NewExecutor(reg, env, nil)
	// orchestrator and summary are nil -> falls back to runSimpleRound
	s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	s.workflowPath = path
	s.runIntelligentRound(context.Background())
}

// ---- GetStatus with running scheduler ----

func TestGetStatus_Running(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &mockRegistry{}
	s := NewWithOptions(ModeSimple, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Stop()

	status := s.GetStatus()
	if status["running"] != true {
		t.Errorf("expected running=true after Start, got %v", status["running"])
	}
}

// ---- runIntelligentRound with real plan execution ----

func TestRunIntelligentRound_WithPlan_Parallel(t *testing.T) {
workflowYAML := `priority: high
steps:
  - mode: parallel
    agents:
      - name: Agent1
      - name: Agent2
`
path := writeTestWorkflow(t, workflowYAML)

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{
{Name: "Agent1"},
{Name: "Agent2"},
},
}
exec := agent.NewExecutor(reg, env, nil)
// Non-nil orchestrator and summary (nil model) to bypass early return
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_WithPlan_Sequential(t *testing.T) {
workflowYAML := `priority: medium
steps:
  - mode: sequential
    agents:
      - name: Agent1
`
path := writeTestWorkflow(t, workflowYAML)

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{{Name: "Agent1"}},
}
exec := agent.NewExecutor(reg, env, nil)
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_WithPlan_SkipAgents(t *testing.T) {
workflowYAML := `priority: low
skip_agents:
  - Agent2
steps:
  - mode: parallel
    agents:
      - name: Agent1
      - name: Agent2
`
path := writeTestWorkflow(t, workflowYAML)

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{
{Name: "Agent1"},
{Name: "Agent2"},
},
}
exec := agent.NewExecutor(reg, env, nil)
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_FallbackPlan(t *testing.T) {
env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{{Name: "Agent1"}},
}
exec := agent.NewExecutor(reg, env, nil)
// Non-nil orchestrator and summary (nil model) - orchestrator.Plan will fail, falls to createFallbackPlan
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
// No workflowPath, orchestrator with nil model will fail
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_WithReport(t *testing.T) {
workflowYAML := `priority: high
steps:
  - mode: parallel
    agents:
      - name: Agent1
`
path := writeTestWorkflow(t, workflowYAML)
reportDir := t.TempDir()

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{{Name: "Agent1"}},
}
exec := agent.NewExecutor(reg, env, nil)
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, reportDir, 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_WithDependsOn(t *testing.T) {
workflowYAML := `priority: high
steps:
  - mode: parallel
    agents:
      - name: Agent1
  - mode: sequential
    depends_on:
      - Agent1
    agents:
      - name: Agent2
`
path := writeTestWorkflow(t, workflowYAML)

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{
specs: []agent.Spec{
{Name: "Agent1"},
{Name: "Agent2"},
},
}
exec := agent.NewExecutor(reg, env, nil)
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
// Agent1 won't produce result (nil model), so Agent2 step won't run (depends_on not met)
s.runIntelligentRound(context.Background())
}

func TestRunIntelligentRound_EmptyAgentsStep(t *testing.T) {
workflowYAML := `priority: high
steps:
  - mode: parallel
    agents: []
`
path := writeTestWorkflow(t, workflowYAML)

env := runtime.NewEnvironment(nil)
reg := &mockRegistry{}
exec := agent.NewExecutor(reg, env, nil)
orch := agent.NewOrchestratorAgent(nil)
sum := agent.NewSummaryAgent(nil)
s := NewWithOptions(ModeIntelligent, reg, exec, env, "", 5*time.Second, 0, orch, sum, nil, nil)
s.workflowPath = path
s.runIntelligentRound(context.Background())
}
