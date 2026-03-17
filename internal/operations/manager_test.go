package operations

import (
	"testing"
	"time"

	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestManager_Record(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now.Add(-time.Second), now, 1.0, "")
	m.Record("agent1", false, now.Add(-2*time.Second), now, 2.0, "some error")

	records, total := m.GetHistory("agent1", 100, 0, nil, nil, nil)
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

func TestManager_RecordExecutionRecord_FillsTimestamps(t *testing.T) {
	m := NewManager()
	now := time.Now()
	r := ExecutionRecord{
		AgentName:       "a",
		Success:         true,
		StartedAt:       now.Add(-time.Second),
		FinishedAt:      now,
		DurationSeconds: 1.0,
	}
	m.RecordExecutionRecord(r)
	records, _ := m.GetHistory("a", 10, 0, nil, nil, nil)
	if len(records) == 0 {
		t.Fatal("expected 1 record")
	}
	if records[0].ExecutedAt == "" {
		t.Error("expected ExecutedAt to be set")
	}
	if records[0].Timestamp == 0 {
		t.Error("expected Timestamp to be set")
	}
}

func TestManager_GetHistory_Filter(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now.Add(-2*time.Second), now, 1.0, "")
	m.Record("agent1", false, now.Add(-1*time.Second), now, 0.5, "err")
	m.Record("agent2", true, now.Add(-3*time.Second), now, 2.0, "")

	// Filter by successOnly=true
	successOnly := true
	records, total := m.GetHistory("", 100, 0, &successOnly, nil, nil)
	if total != 2 {
		t.Errorf("expected 2 successful records, got %d", total)
	}
	for _, r := range records {
		if !r.Success {
			t.Error("expected only successful records")
		}
	}
}

func TestManager_GetHistory_FilterByAgent(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")
	m.Record("agent2", true, now, now, 1.0, "")

	records, total := m.GetHistory("agent1", 100, 0, nil, nil, nil)
	if total != 1 {
		t.Errorf("expected 1 record for agent1, got %d", total)
	}
	_ = records
}

func TestManager_GetHistory_TimeRange(t *testing.T) {
	m := NewManager()
	past := time.Now().Add(-10 * time.Minute)
	recent := time.Now()

	m.Record("a", true, past.Add(-time.Minute), past, 1.0, "")
	m.Record("a", true, recent.Add(-time.Second), recent, 1.0, "")

	start := past.Add(-2 * time.Minute)
	end := past.Add(time.Minute)
	records, total := m.GetHistory("", 100, 0, nil, &start, &end)
	if total != 1 {
		t.Errorf("expected 1 record in time range, got %d", total)
	}
	_ = records
}

func TestManager_GetHistory_Pagination(t *testing.T) {
	m := NewManager()
	now := time.Now()
	for i := 0; i < 5; i++ {
		m.Record("a", true, now, now, float64(i), "")
	}

	records, total := m.GetHistory("", 2, 0, nil, nil, nil)
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records (page 1), got %d", len(records))
	}

	records2, _ := m.GetHistory("", 2, 2, nil, nil, nil)
	if len(records2) != 2 {
		t.Errorf("expected 2 records (page 2), got %d", len(records2))
	}
}

func TestManager_GetHistory_OffsetBeyondTotal(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("a", true, now, now, 1.0, "")

	records, total := m.GetHistory("", 10, 100, nil, nil, nil)
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records past end, got %d", len(records))
	}
}

func TestManager_GetSummary(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")
	m.Record("agent1", true, now, now, 2.0, "")
	m.Record("agent1", false, now, now, 0.5, "err")

	summaries := m.GetSummary()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	s := summaries[0]
	if s.TotalExecutions != 3 {
		t.Errorf("expected 3 total, got %d", s.TotalExecutions)
	}
	if s.SuccessfulExecutions != 2 {
		t.Errorf("expected 2 successes, got %d", s.SuccessfulExecutions)
	}
	if s.FailedExecutions != 1 {
		t.Errorf("expected 1 failure, got %d", s.FailedExecutions)
	}
}

func TestManager_GetSummaryWithEnrichment_Nil(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("a", true, now, now, 1.0, "")
	summaries := m.GetSummaryWithEnrichment(nil)
	if len(summaries) == 0 {
		t.Error("expected at least 1 summary")
	}
}

func TestManager_GetSummaryWithEnrichment_WithMetrics(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")

	mc := runtime.NewMetricsCollector()
	mc.RecordExecution("agent1", true, 2.0)
	mc.RecordExecution("agent1", false, 0.5)

	src := &EnrichmentSources{Metrics: mc}
	summaries := m.GetSummaryWithEnrichment(src)
	found := false
	for _, s := range summaries {
		if s.AgentName == "agent1" {
			found = true
			if s.TotalExecutions < 2 {
				t.Errorf("expected enriched total_executions >= 2, got %d", s.TotalExecutions)
			}
		}
	}
	if !found {
		t.Error("agent1 not found in summaries")
	}
}

func TestManager_GetSummaryWithEnrichment_WithState(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")

	cfg := runtime.DefaultConfig()
	cfg.PersistState = false
	sp := runtime.NewStatePersistence(cfg)
	sp.UpdateAgent("agent1", &now, true)

	src := &EnrichmentSources{State: sp}
	summaries := m.GetSummaryWithEnrichment(src)
	for _, s := range summaries {
		if s.AgentName == "agent1" {
			if s.LastRunAtISO == "" {
				t.Error("expected LastRunAtISO to be set")
			}
		}
	}
}

func TestManager_GetSummaryWithEnrichment_WithCircuit(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")

	cfg := runtime.DefaultConfig()
	cfg.CircuitBreakerThreshold = 1
	cfg.CircuitBreakerTimeout = time.Minute
	cb := runtime.NewCircuitBreaker(cfg)
	cb.RecordFailure("agent1") // open circuit

	src := &EnrichmentSources{Circuit: cb}
	summaries := m.GetSummaryWithEnrichment(src)
	for _, s := range summaries {
		if s.AgentName == "agent1" {
			if !s.CircuitBreakerOpen {
				t.Error("expected CircuitBreakerOpen=true")
			}
		}
	}
}

func TestManager_CapAt1000(t *testing.T) {
	m := NewManager()
	now := time.Now()
	for i := 0; i < 1100; i++ {
		m.Record("a", true, now, now, 1.0, "")
	}
	_, total := m.GetHistory("", 2000, 0, nil, nil, nil)
	if total > 1000 {
		t.Errorf("expected at most 1000 records, got %d", total)
	}
}

func TestHelperFunctions_toInt(t *testing.T) {
	tests := []struct {
		input any
		def   int
		want  int
	}{
		{42, 0, 42},
		{int64(7), 0, 7},
		{float64(3.9), 0, 3},
		{nil, 5, 5},
		{"str", 5, 5},
	}
	for _, tt := range tests {
		got := toInt(tt.input, tt.def)
		if got != tt.want {
			t.Errorf("toInt(%v, %v) = %d, want %d", tt.input, tt.def, got, tt.want)
		}
	}
}

func TestHelperFunctions_toFloat64(t *testing.T) {
	tests := []struct {
		input any
		def   float64
		want  float64
	}{
		{1.5, 0, 1.5},
		{int(3), 0, 3.0},
		{int64(4), 0, 4.0},
		{nil, 9.9, 9.9},
		{"x", 9.9, 9.9},
	}
	for _, tt := range tests {
		got := toFloat64(tt.input, tt.def)
		if got != tt.want {
			t.Errorf("toFloat64(%v, %v) = %f, want %f", tt.input, tt.def, got, tt.want)
		}
	}
}

func TestManager_SuccessRateCalculation(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Record("agent1", true, now, now, 1.0, "")
	m.Record("agent1", true, now, now, 1.0, "")

	summaries := m.GetSummary()
	if len(summaries) != 1 {
		t.Fatal("expected 1 summary")
	}
	s := summaries[0]
	if s.SuccessRate != 1.0 && s.SuccessRate != 0 {
		// SuccessRate is set to 1.0 when computed from successful/total
		t.Errorf("unexpected success rate: %f", s.SuccessRate)
	}
}
