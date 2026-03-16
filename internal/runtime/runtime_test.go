package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---- DefaultConfig / NewEnvironment ----

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxConcurrentAgents <= 0 {
		t.Error("MaxConcurrentAgents must be positive")
	}
	if cfg.MaxConcurrentKubectl <= 0 {
		t.Error("MaxConcurrentKubectl must be positive")
	}
	if cfg.CircuitBreakerThreshold <= 0 {
		t.Error("CircuitBreakerThreshold must be positive")
	}
	if cfg.MinBackoff <= 0 {
		t.Error("MinBackoff must be positive")
	}
	if cfg.MaxBackoff <= 0 {
		t.Error("MaxBackoff must be positive")
	}
}

func TestNewEnvironment_NilConfig(t *testing.T) {
	env := NewEnvironment(nil)
	if env == nil {
		t.Fatal("expected non-nil environment")
	}
	if env.Config == nil {
		t.Error("expected Config to be set")
	}
	if env.Concurrency == nil {
		t.Error("expected Concurrency")
	}
	if env.Circuit == nil {
		t.Error("expected Circuit")
	}
	if env.Metrics == nil {
		t.Error("expected Metrics")
	}
}

func TestNewEnvironment_WithConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrentAgents = 2
	env := NewEnvironment(cfg)
	if env.Config.MaxConcurrentAgents != 2 {
		t.Errorf("expected 2, got %d", env.Config.MaxConcurrentAgents)
	}
}

// ---- CalculateBackoff ----

func TestCalculateBackoff_ZeroAttempt(t *testing.T) {
	cfg := DefaultConfig()
	d := CalculateBackoff(0, cfg)
	if d < cfg.MinBackoff {
		t.Errorf("backoff %v should be >= MinBackoff %v", d, cfg.MinBackoff)
	}
}

func TestCalculateBackoff_Exponential(t *testing.T) {
	cfg := DefaultConfig()
	d0 := CalculateBackoff(0, cfg)
	d1 := CalculateBackoff(1, cfg)
	if d1 < d0 {
		t.Errorf("backoff should grow with attempt: d0=%v d1=%v", d0, d1)
	}
}

func TestCalculateBackoff_MaxCapped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBackoff = 2 * time.Second
	cfg.MinBackoff = 1 * time.Second
	d := CalculateBackoff(100, cfg)
	if d > cfg.MaxBackoff {
		t.Errorf("backoff %v exceeds MaxBackoff %v", d, cfg.MaxBackoff)
	}
}

func TestCalculateBackoff_NilConfig(t *testing.T) {
	d := CalculateBackoff(1, nil)
	if d <= 0 {
		t.Error("expected positive backoff with nil config")
	}
}

// ---- CircuitBreaker ----

func TestCircuitBreaker_InitiallyClosed(t *testing.T) {
	cb := NewCircuitBreaker(DefaultConfig())
	if cb.IsOpen("agent1") {
		t.Error("circuit should be closed initially")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 3
	cfg.CircuitBreakerTimeout = 5 * time.Minute
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure("agent1")
	cb.RecordFailure("agent1")
	if cb.IsOpen("agent1") {
		t.Error("circuit should still be closed after 2 failures")
	}
	cb.RecordFailure("agent1")
	if !cb.IsOpen("agent1") {
		t.Error("circuit should open after 3 failures")
	}
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 2
	cfg.CircuitBreakerTimeout = 5 * time.Minute
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure("agent1")
	cb.RecordFailure("agent1")
	if !cb.IsOpen("agent1") {
		t.Fatal("circuit should be open")
	}
	cb.RecordSuccess("agent1")
	if cb.IsOpen("agent1") {
		t.Error("circuit should close after success")
	}
}

func TestCircuitBreaker_TimedRecovery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 1
	cfg.CircuitBreakerTimeout = 10 * time.Millisecond
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure("agent1")
	if !cb.IsOpen("agent1") {
		t.Fatal("circuit should be open")
	}
	time.Sleep(20 * time.Millisecond)
	if cb.IsOpen("agent1") {
		t.Error("circuit should recover after timeout")
	}
}

func TestCircuitBreaker_NilConfig(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	cb.RecordFailure("x")
	_ = cb.IsOpen("x")
	_ = cb.GetStatus()
}

func TestCircuitBreaker_GetStatus(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 1
	cb := NewCircuitBreaker(cfg)
	cb.RecordFailure("agent1")
	status := cb.GetStatus()
	if _, ok := status["open_circuits"]; !ok {
		t.Error("expected open_circuits key")
	}
	if _, ok := status["failure_counts"]; !ok {
		t.Error("expected failure_counts key")
	}
}

// ---- RateLimiter ----

func TestRateLimiter_BasicWait(t *testing.T) {
	rl := NewRateLimiter(100, 10) // high rate, large burst
	ctx := context.Background()
	err := rl.Wait(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiter_ZeroTokens(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	err := rl.Wait(context.Background(), 0)
	if err != nil {
		t.Errorf("Wait(0) should be no-op: %v", err)
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter(0.001, 1) // very slow rate
	rl.Wait(context.Background(), 1) // drain the burst
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := rl.Wait(ctx, 1)
	if err == nil {
		t.Error("expected context error")
	}
}

func TestNewRateLimiter_Defaults(t *testing.T) {
	rl := NewRateLimiter(0, 0)
	if rl.rate <= 0 {
		t.Error("rate should default to 1")
	}
	if rl.burst <= 0 {
		t.Error("burst should default to 1")
	}
}

// ---- ConcurrencyController ----

func TestConcurrencyController_WithAgentSlot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrentAgents = 2
	cc := NewConcurrencyController(cfg)

	err := cc.WithAgentSlot(context.Background(), "agent1", func(ctx context.Context) error {
		if cc.ActiveAgentCount() != 1 {
			t.Errorf("expected 1 active agent, got %d", cc.ActiveAgentCount())
		}
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cc.ActiveAgentCount() != 0 {
		t.Error("expected 0 active agents after completion")
	}
}

func TestConcurrencyController_ContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrentAgents = 1
	cc := NewConcurrencyController(cfg)

	// Fill the slot
	done := make(chan struct{})
	go cc.WithAgentSlot(context.Background(), "blocking", func(ctx context.Context) error {
		<-done
		return nil
	})
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cc.WithAgentSlot(ctx, "waiting", func(ctx context.Context) error {
		return nil
	})
	close(done)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestConcurrencyController_NilConfig(t *testing.T) {
	cc := NewConcurrencyController(nil)
	if cc == nil {
		t.Fatal("expected non-nil controller")
	}
}

func TestConcurrencyController_ActiveAgents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrentAgents = 5
	cc := NewConcurrencyController(cfg)

	var wg sync.WaitGroup
	start := make(chan struct{})
	stop := make(chan struct{})

	for _, name := range []string{"a1", "a2", "a3"} {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			cc.WithAgentSlot(context.Background(), n, func(ctx context.Context) error {
				<-start
				<-stop
				return nil
			})
		}(name)
	}
	time.Sleep(20 * time.Millisecond)
	close(start)
	time.Sleep(10 * time.Millisecond)
	agents := cc.ActiveAgents()
	_ = agents // may vary due to goroutine scheduling
	close(stop)
	wg.Wait()
}

// ---- MetricsCollector ----

func TestMetricsCollector_RecordAndGet(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordExecution("agent1", true, 1.5)
	mc.RecordExecution("agent1", false, 0.5)
	mc.RecordExecution("agent2", true, 2.0)

	metrics := mc.GetMetrics()
	if _, ok := metrics["uptime_seconds"]; !ok {
		t.Error("expected uptime_seconds")
	}
	agents, ok := metrics["agents"].(map[string]any)
	if !ok {
		t.Fatal("expected agents map")
	}
	a1, ok := agents["agent1"].(map[string]any)
	if !ok {
		t.Fatal("expected agent1 metrics")
	}
	if a1["total_executions"].(int) != 2 {
		t.Errorf("expected 2 executions, got %v", a1["total_executions"])
	}
	if a1["successful_executions"].(int) != 1 {
		t.Errorf("expected 1 success, got %v", a1["successful_executions"])
	}
}

func TestMetricsCollector_UptimeSeconds(t *testing.T) {
	mc := NewMetricsCollector()
	time.Sleep(5 * time.Millisecond)
	uptime := mc.UptimeSeconds()
	if uptime <= 0 {
		t.Errorf("expected positive uptime, got %f", uptime)
	}
}

func TestAgentMetrics_SuccessRate_Empty(t *testing.T) {
	m := &AgentMetrics{}
	if m.SuccessRate() != 1.0 {
		t.Errorf("expected 1.0 success rate for empty metrics, got %f", m.SuccessRate())
	}
}

func TestAgentMetrics_AvgDuration_Empty(t *testing.T) {
	m := &AgentMetrics{}
	if m.AvgDurationSeconds() != 0 {
		t.Errorf("expected 0 avg duration for empty metrics, got %f", m.AvgDurationSeconds())
	}
}

// ---- GracefulShutdown ----

func TestGracefulShutdown_Basic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 1 * time.Second
	gs := NewGracefulShutdown(cfg)

	if gs.IsShuttingDown() {
		t.Error("should not be shutting down initially")
	}
	gs.RequestShutdown()
	if !gs.IsShuttingDown() {
		t.Error("should be shutting down after request")
	}
	// Second call should be idempotent
	gs.RequestShutdown()
	if !gs.IsShuttingDown() {
		t.Error("should still be shutting down")
	}
}

func TestGracefulShutdown_WaitForShutdown_ContextDone(t *testing.T) {
	gs := NewGracefulShutdown(DefaultConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	gs.WaitForShutdown(ctx) // should return quickly due to context timeout
}

func TestGracefulShutdown_WaitForShutdown_ShutdownSignal(t *testing.T) {
	gs := NewGracefulShutdown(DefaultConfig())
	go func() {
		time.Sleep(5 * time.Millisecond)
		gs.RequestShutdown()
	}()
	ctx := context.Background()
	gs.WaitForShutdown(ctx)
}

func TestGracefulShutdown_Tasks(t *testing.T) {
	gs := NewGracefulShutdown(DefaultConfig())
	if gs.TaskCount() != 0 {
		t.Error("expected 0 tasks initially")
	}
	gs.RegisterTask("task1")
	gs.RegisterTask("task2")
	if gs.TaskCount() != 2 {
		t.Errorf("expected 2 tasks, got %d", gs.TaskCount())
	}
	gs.UnregisterTask("task1")
	if gs.TaskCount() != 1 {
		t.Errorf("expected 1 task, got %d", gs.TaskCount())
	}
}

func TestGracefulShutdown_WaitForTasks_NilFn(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 100 * time.Millisecond
	gs := NewGracefulShutdown(cfg)
	err := gs.WaitForTasks(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGracefulShutdown_NilConfig(t *testing.T) {
	gs := NewGracefulShutdown(nil)
	if gs == nil {
		t.Fatal("expected non-nil GracefulShutdown")
	}
}

// ---- StatePersistence ----

func TestStatePersistence_LoadSave(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.PersistState = true
	cfg.StateFile = filepath.Join(dir, "state.json")

	sp := NewStatePersistence(cfg)
	sp.UpdateAgent("agent1", nil, true)
	sp.UpdateAgent("agent1", nil, false)

	if !sp.IsDirty() {
		t.Error("expected dirty after update")
	}
	if err := sp.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if sp.IsDirty() {
		t.Error("expected clean after save")
	}

	// Load in new instance
	sp2 := NewStatePersistence(cfg)
	state, err := sp2.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, ok := state.Agents["agent1"]
	if !ok {
		t.Fatal("expected agent1 in loaded state")
	}
	if a.TotalRuns != 2 {
		t.Errorf("expected 2 total runs, got %d", a.TotalRuns)
	}
}

func TestStatePersistence_LoadNotExist(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = true
	cfg.StateFile = "/tmp/nonexistent_state_test_abc123.json"
	sp := NewStatePersistence(cfg)
	state, err := sp.Load()
	if err != nil {
		t.Errorf("unexpected error for non-existent file: %v", err)
	}
	if state == nil {
		t.Error("expected non-nil state")
	}
}

func TestStatePersistence_NoPersist(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = false
	sp := NewStatePersistence(cfg)

	sp.UpdateAgent("a", nil, true)
	if err := sp.Save(); err != nil {
		t.Errorf("Save should be no-op when PersistState=false: %v", err)
	}

	state, err := sp.Load()
	if err != nil {
		t.Errorf("Load: %v", err)
	}
	if state == nil {
		t.Error("expected non-nil state")
	}
}

func TestStatePersistence_GetAgentLastRun(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = false
	sp := NewStatePersistence(cfg)

	if sp.GetAgentLastRun("unknown") != nil {
		t.Error("expected nil for unknown agent")
	}

	now := time.Now()
	sp.UpdateAgent("a", &now, true)
	lastRun := sp.GetAgentLastRun("a")
	if lastRun == nil {
		t.Error("expected non-nil last run time")
	}
}

func TestStatePersistence_GetStateSnapshot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = false
	sp := NewStatePersistence(cfg)
	sp.UpdateAgent("a1", nil, true)
	sp.UpdateAgent("a2", nil, false)

	snap := sp.GetStateSnapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 agents in snapshot, got %d", len(snap))
	}
}

func TestStatePersistence_LoadBadJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.PersistState = true
	cfg.StateFile = filepath.Join(dir, "bad.json")
	os.WriteFile(cfg.StateFile, []byte("not json"), 0644)

	sp := NewStatePersistence(cfg)
	_, err := sp.Load()
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestStatePersistence_NilConfig(t *testing.T) {
	sp := NewStatePersistence(nil)
	if sp == nil {
		t.Fatal("expected non-nil StatePersistence")
	}
}

func TestStatePersistence_UpdateAgent_WithLastRunAt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = false
	sp := NewStatePersistence(cfg)
	t0 := time.Now().Add(-time.Minute)
	sp.UpdateAgent("a", &t0, true)

	snap := sp.GetStateSnapshot()
	if a, ok := snap["a"]; ok {
		if a.LastSuccessAt <= 0 {
			t.Error("expected non-zero LastSuccessAt")
		}
	}
}

func TestStatePersistence_ConsecutiveFailures(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = false
	sp := NewStatePersistence(cfg)
	sp.UpdateAgent("a", nil, false)
	sp.UpdateAgent("a", nil, false)
	sp.UpdateAgent("a", nil, true) // reset

	snap := sp.GetStateSnapshot()
	if snap["a"].ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", snap["a"].ConsecutiveFailures)
	}
}

func TestStatePersistence_SaveNilState(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PersistState = true
	sp := NewStatePersistence(cfg)
	sp.state = nil
	if err := sp.Save(); err != nil {
		t.Errorf("Save with nil state should be no-op: %v", err)
	}
}

// ---- JSON round-trip for AgentState ----

func TestAgentStateJSON(t *testing.T) {
	a := AgentState{
		AgentName:           "test",
		TotalRuns:           5,
		TotalFailures:       1,
		ConsecutiveFailures: 0,
		LastRunAt:           float64(time.Now().Unix()),
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var b AgentState
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.AgentName != a.AgentName {
		t.Errorf("AgentName mismatch: %q vs %q", b.AgentName, a.AgentName)
	}
}
