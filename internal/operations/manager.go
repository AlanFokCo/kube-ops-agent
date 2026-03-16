package operations

import (
	"sync"
	"time"

	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// EnrichmentSources merges data sources for summary.
type EnrichmentSources struct {
	Metrics *runtime.MetricsCollector
	State   *runtime.StatePersistence
	Circuit *runtime.CircuitBreaker
}

// Ensure Manager implements runtime.OpsRecorder.
var _ runtime.OpsRecorder = (*Manager)(nil)

// ExecutionRecord records one Agent execution, aligned with Python AgentExecution.
type ExecutionRecord struct {
	AgentName       string    `json:"agent_name"`
	Success         bool      `json:"success"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	ExecutedAt      string    `json:"executed_at,omitempty"` // ISO format
	Timestamp       float64   `json:"timestamp,omitempty"`  // Unix timestamp
	DurationSeconds float64   `json:"duration_seconds"`
	Error           string    `json:"error_message,omitempty"`
}

// Summary is Agent aggregate stats.
type Summary struct {
	AgentName            string  `json:"agent_name"`
	TotalExecutions      int     `json:"total_executions"`
	SuccessfulExecutions int     `json:"successful_executions"`
	FailedExecutions     int     `json:"failed_executions"`
	SuccessRate          float64 `json:"success_rate"`
	AvgDurationSeconds   float64 `json:"avg_duration_seconds"`
	LastDurationSeconds  float64 `json:"last_duration_seconds"`
	LastRunAt            int64   `json:"last_run_at,omitempty"`
	LastRunAtISO         string  `json:"last_run_at_iso,omitempty"`
	LastSuccessAtISO     string  `json:"last_success_at_iso,omitempty"`
	ConsecutiveFailures  int     `json:"consecutive_failures,omitempty"`
	CircuitBreakerOpen   bool    `json:"circuit_breaker_open,omitempty"`
}

// Manager manages execution history.
type Manager struct {
	mu      sync.RWMutex
	records []ExecutionRecord
}

// NewManager creates new execution history manager.
func NewManager() *Manager {
	return &Manager{
		records: make([]ExecutionRecord, 0, 128),
	}
}

// RecordExecutionRecord records one execution (for external ExecutionRecord).
func (m *Manager) RecordExecutionRecord(r ExecutionRecord) {
	m.record(r)
}

// Record implements runtime.OpsRecorder, called by Executor.
func (m *Manager) Record(agentName string, success bool, startedAt, finishedAt time.Time, durationSeconds float64, errMsg string) {
	m.record(ExecutionRecord{
		AgentName:       agentName,
		Success:         success,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		DurationSeconds: durationSeconds,
		Error:           errMsg,
	})
}

func (m *Manager) record(r ExecutionRecord) {
	// Fill executed_at and timestamp
	if r.ExecutedAt == "" && !r.FinishedAt.IsZero() {
		r.ExecutedAt = r.FinishedAt.Format(time.RFC3339)
	}
	if r.Timestamp == 0 && !r.FinishedAt.IsZero() {
		r.Timestamp = float64(r.FinishedAt.Unix())
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, r)
	if len(m.records) > 1000 {
		m.records = m.records[len(m.records)-1000:]
	}
}

// GetHistory gets execution history with filter and pagination.
// No time range when startTime/endTime is nil.
func (m *Manager) GetHistory(
	agentName string,
	limit, offset int,
	successOnly *bool,
	startTime, endTime *time.Time,
) ([]ExecutionRecord, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []ExecutionRecord
	for i := len(m.records) - 1; i >= 0; i-- {
		r := m.records[i]
		if agentName != "" && r.AgentName != agentName {
			continue
		}
		if successOnly != nil && r.Success != *successOnly {
			continue
		}
		if startTime != nil && r.StartedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && r.FinishedAt.After(*endTime) {
			continue
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)
	if offset > total {
		return []ExecutionRecord{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total
}

// GetSummary gets aggregate stats for all Agents.
func (m *Manager) GetSummary() []Summary {
	return m.GetSummaryWithEnrichment(nil)
}

// GetSummaryWithEnrichment merges Metrics/State/Circuit, aligned with Python get_agent_summary.
func (m *Manager) GetSummaryWithEnrichment(src *EnrichmentSources) []Summary {
	m.mu.RLock()
	agg := make(map[string]*Summary)
	for _, r := range m.records {
		s, ok := agg[r.AgentName]
		if !ok {
			s = &Summary{AgentName: r.AgentName}
			agg[r.AgentName] = s
		}
		s.TotalExecutions++
		if r.Success {
			s.SuccessfulExecutions++
		} else {
			s.FailedExecutions++
		}
		s.LastDurationSeconds = r.DurationSeconds
		if r.FinishedAt.Unix() != 0 {
			s.LastRunAt = r.FinishedAt.Unix()
		}
		n := float64(s.TotalExecutions)
		if s.TotalExecutions == 1 {
			s.AvgDurationSeconds = r.DurationSeconds
		} else {
			s.AvgDurationSeconds = (s.AvgDurationSeconds*(n-1) + r.DurationSeconds) / n
		}
	}
	m.mu.RUnlock()

	if src != nil {
		if src.Metrics != nil {
			metrics := src.Metrics.GetMetrics()
			if agents, ok := metrics["agents"].(map[string]any); ok {
				for name, am := range agents {
					amap, _ := am.(map[string]any)
					if amap == nil {
						continue
					}
					if agg[name] == nil {
						agg[name] = &Summary{AgentName: name}
					}
					s := agg[name]
					s.TotalExecutions = toInt(amap["total_executions"], s.TotalExecutions)
					s.SuccessfulExecutions = toInt(amap["successful_executions"], s.SuccessfulExecutions)
					s.FailedExecutions = toInt(amap["failed_executions"], s.FailedExecutions)
					s.SuccessRate = toFloat64(amap["success_rate"], s.SuccessRate)
					s.AvgDurationSeconds = toFloat64(amap["avg_duration_seconds"], s.AvgDurationSeconds)
					s.LastDurationSeconds = toFloat64(amap["last_duration_seconds"], s.LastDurationSeconds)
				}
			}
		}
		if src.State != nil {
			state := src.State.GetStateSnapshot()
			for name, ast := range state {
				if agg[name] == nil {
					agg[name] = &Summary{AgentName: name}
				}
				s := agg[name]
				if ast.LastRunAt > 0 {
					s.LastRunAt = int64(ast.LastRunAt)
					s.LastRunAtISO = time.Unix(int64(ast.LastRunAt), 0).Format(time.RFC3339)
				}
				if ast.LastSuccessAt > 0 {
					s.LastSuccessAtISO = time.Unix(int64(ast.LastSuccessAt), 0).Format(time.RFC3339)
				}
				s.ConsecutiveFailures = ast.ConsecutiveFailures
			}
		}
		if src.Circuit != nil {
			status := src.Circuit.GetStatus()
			if open, ok := status["open_circuits"].([]string); ok {
				for _, name := range open {
					if agg[name] != nil {
						agg[name].CircuitBreakerOpen = true
					}
				}
			}
		}
	}

	out := make([]Summary, 0, len(agg))
	for _, s := range agg {
		if s.TotalExecutions > 0 && s.SuccessRate == 0 {
			s.SuccessRate = float64(s.SuccessfulExecutions) / float64(s.TotalExecutions)
		}
		out = append(out, *s)
	}
	return out
}

func toInt(v any, def int) int {
	if v == nil {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return def
	}
}

func toFloat64(v any, def float64) float64 {
	if v == nil {
		return def
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return def
	}
}
