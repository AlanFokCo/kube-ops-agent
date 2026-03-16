package runtime

import (
	"sync"
	"time"
)

// AgentMetrics is metrics for single Agent.
type AgentMetrics struct {
	TotalExecutions      int     `json:"total_executions"`
	SuccessfulExecutions int     `json:"successful_executions"`
	FailedExecutions     int     `json:"failed_executions"`
	TotalDurationSeconds float64 `json:"total_duration_seconds"`
	LastDurationSeconds  float64 `json:"last_duration_seconds"`
}

// SuccessRate computes success rate.
func (m *AgentMetrics) SuccessRate() float64 {
	if m.TotalExecutions == 0 {
		return 1.0
	}
	return float64(m.SuccessfulExecutions) / float64(m.TotalExecutions)
}

// AvgDurationSeconds computes average duration.
func (m *AgentMetrics) AvgDurationSeconds() float64 {
	if m.TotalExecutions == 0 {
		return 0
	}
	return m.TotalDurationSeconds / float64(m.TotalExecutions)
}

// MetricsCollector collects and exposes runtime metrics.
type MetricsCollector struct {
	mu           sync.RWMutex
	agentMetrics map[string]*AgentMetrics
	startTime    time.Time
}

// NewMetricsCollector creates metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		agentMetrics: make(map[string]*AgentMetrics),
		startTime:    time.Now(),
	}
}

// RecordExecution records one Agent execution.
func (m *MetricsCollector) RecordExecution(agentName string, success bool, durationSeconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.agentMetrics[agentName] == nil {
		m.agentMetrics[agentName] = &AgentMetrics{}
	}
	am := m.agentMetrics[agentName]
	am.TotalExecutions++
	am.TotalDurationSeconds += durationSeconds
	am.LastDurationSeconds = durationSeconds
	if success {
		am.SuccessfulExecutions++
	} else {
		am.FailedExecutions++
	}
}

// GetMetrics returns all metrics.
func (m *MetricsCollector) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make(map[string]any)
	for name, am := range m.agentMetrics {
		agents[name] = map[string]any{
			"total_executions":       am.TotalExecutions,
			"successful_executions":  am.SuccessfulExecutions,
			"failed_executions":      am.FailedExecutions,
			"success_rate":           am.SuccessRate(),
			"avg_duration_seconds":   am.AvgDurationSeconds(),
			"last_duration_seconds":  am.LastDurationSeconds,
		}
	}
	return map[string]any{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"agents":         agents,
	}
}

// UptimeSeconds returns uptime in seconds.
func (m *MetricsCollector) UptimeSeconds() float64 {
	return time.Since(m.startTime).Seconds()
}
