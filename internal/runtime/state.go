package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentState persists single Agent state.
type AgentState struct {
	AgentName           string  `json:"agent_name"`
	LastRunAt           float64 `json:"last_run_at,omitempty"`
	LastSuccessAt       float64 `json:"last_success_at,omitempty"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	TotalRuns           int     `json:"total_runs"`
	TotalFailures       int     `json:"total_failures"`
}

// SchedulerState persists scheduler overall state.
type SchedulerState struct {
	Agents    map[string]*AgentState `json:"agents"`
	Pipelines map[string]float64      `json:"pipelines"`
	LastSaved float64                `json:"last_saved,omitempty"`
	Version   string                 `json:"version"`
}

// StatePersistence handles state persistence, supports restart recovery.
type StatePersistence struct {
	cfg    *ProductionConfig
	mu     sync.RWMutex
	state  *SchedulerState
	dirty  bool
}

// NewStatePersistence creates state persistence instance.
func NewStatePersistence(cfg *ProductionConfig) *StatePersistence {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &StatePersistence{
		cfg:   cfg,
		state: &SchedulerState{Agents: make(map[string]*AgentState), Pipelines: make(map[string]float64), Version: "1.0"},
	}
}

// Load loads state from file.
func (s *StatePersistence) Load() (*SchedulerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.PersistState {
		s.state = &SchedulerState{Agents: make(map[string]*AgentState), Pipelines: make(map[string]float64), Version: "1.0"}
		return s.state, nil
	}

	path := s.cfg.StateFile
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.state = &SchedulerState{Agents: make(map[string]*AgentState), Pipelines: make(map[string]float64), Version: "1.0"}
			return s.state, nil
		}
		return s.state, err
	}

	var loaded SchedulerState
	if err := json.Unmarshal(data, &loaded); err != nil {
		return s.state, err
	}
	if loaded.Agents == nil {
		loaded.Agents = make(map[string]*AgentState)
	}
	if loaded.Pipelines == nil {
		loaded.Pipelines = make(map[string]float64)
	}
	s.state = &loaded
	return s.state, nil
}

// Save saves state to file.
func (s *StatePersistence) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.PersistState || s.state == nil {
		return nil
	}

	s.state.LastSaved = float64(time.Now().Unix())
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}

	path := s.cfg.StateFile
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	s.dirty = false
	return nil
}

// UpdateAgent updates Agent state after execution.
func (s *StatePersistence) UpdateAgent(agentName string, lastRunAt *time.Time, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == nil {
		s.state = &SchedulerState{Agents: make(map[string]*AgentState), Pipelines: make(map[string]float64), Version: "1.0"}
	}
	if s.state.Agents[agentName] == nil {
		s.state.Agents[agentName] = &AgentState{AgentName: agentName}
	}
	a := s.state.Agents[agentName]
	now := time.Now().Unix()
	if lastRunAt != nil {
		a.LastRunAt = float64(lastRunAt.Unix())
	} else {
		a.LastRunAt = float64(now)
	}
	a.TotalRuns++
	if success {
		a.LastSuccessAt = a.LastRunAt
		a.ConsecutiveFailures = 0
	} else {
		a.ConsecutiveFailures++
		a.TotalFailures++
	}
	s.dirty = true
}

// GetAgentLastRun gets Agent last run time.
func (s *StatePersistence) GetAgentLastRun(agentName string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state == nil || s.state.Agents[agentName] == nil {
		return nil
	}
	t := s.state.Agents[agentName].LastRunAt
	if t <= 0 {
		return nil
	}
	tt := time.Unix(int64(t), 0)
	return &tt
}

// IsDirty returns whether there are unsaved changes.
func (s *StatePersistence) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

// GetStateSnapshot returns snapshot of all Agent states for operations merge.
func (s *StatePersistence) GetStateSnapshot() map[string]AgentState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state == nil {
		return nil
	}
	out := make(map[string]AgentState, len(s.state.Agents))
	for k, v := range s.state.Agents {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}
