package plan

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ExecutionMode corresponds to Python ExecutionMode.
type ExecutionMode string

const (
	ModeParallel   ExecutionMode = "parallel"
	ModeSequential ExecutionMode = "sequential"
)

// Step represents one execution step.
type Step struct {
	Agents      []string       `json:"agents"`
	Mode        ExecutionMode  `json:"mode"`
	Condition   string         `json:"condition,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty"`
	FocusAreas  []string       `json:"focus_areas,omitempty"`
	TimeoutSecs int            `json:"timeout_seconds,omitempty"`
}

// InspectionPlan aligns with Python InspectionPlan fields.
type InspectionPlan struct {
	Assessment   string       `json:"assessment"`
	Priority     string       `json:"priority"`
	Steps        []Step       `json:"steps"`
	Reasoning    string       `json:"reasoning"`
	AllowReplan  bool         `json:"allow_replan"`
	FocusAreas   []string     `json:"focus_areas,omitempty"`
	SkipAgents   []string     `json:"skip_agents,omitempty"`
	SkipReason   string       `json:"skip_reasoning,omitempty"`
	rawJSON      string
}

// FromJSON parses LLM JSON, compatible with legacy agents_to_invoke format.
func FromJSON(s string) (*InspectionPlan, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("parse plan json: %w", err)
	}

	p := &InspectionPlan{rawJSON: s}

	// Parse top-level fields
	if v, ok := raw["assessment"]; ok {
		_ = json.Unmarshal(v, &p.Assessment)
	}
	if v, ok := raw["overall_assessment"]; ok && p.Assessment == "" {
		_ = json.Unmarshal(v, &p.Assessment)
	}
	if v, ok := raw["priority"]; ok {
		_ = json.Unmarshal(v, &p.Priority)
	}
	if p.Priority == "" {
		p.Priority = "normal"
	}
	if v, ok := raw["reasoning"]; ok {
		_ = json.Unmarshal(v, &p.Reasoning)
	}
	if v, ok := raw["allow_replan"]; ok {
		_ = json.Unmarshal(v, &p.AllowReplan)
	}
	if v, ok := raw["focus_areas"]; ok {
		_ = json.Unmarshal(v, &p.FocusAreas)
	}
	if v, ok := raw["skip_agents"]; ok {
		_ = json.Unmarshal(v, &p.SkipAgents)
	}
	if v, ok := raw["skip_reasoning"]; ok {
		_ = json.Unmarshal(v, &p.SkipReason)
	}

	// New format: steps array
	if v, ok := raw["steps"]; ok {
		var steps []Step
		if err := json.Unmarshal(v, &steps); err == nil && len(steps) > 0 {
			p.Steps = steps
		}
	}

	// Legacy format: agents_to_invoke -> single-step parallel
	if len(p.Steps) == 0 {
		var agents []string
		if v, ok := raw["agents_to_invoke"]; ok {
			if err := json.Unmarshal(v, &agents); err == nil && len(agents) > 0 {
				var focusAreas []string
				if v, ok := raw["focus_areas"]; ok {
					_ = json.Unmarshal(v, &focusAreas)
				}
				p.Steps = []Step{{
					Agents:     agents,
					Mode:       ModeParallel,
					FocusAreas: focusAreas,
				}}
			}
		}
	}

	if len(p.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}
	return p, nil
}

// Raw returns raw JSON string for debugging and logging.
func (p *InspectionPlan) Raw() string {
	return p.rawJSON
}

// workflowYAML is the YAML struct for loading workflow config.
type workflowYAML struct {
	Assessment   string       `yaml:"assessment"`
	Priority     string       `yaml:"priority"`
	Steps        []Step       `yaml:"steps"`
	Reasoning    string       `yaml:"reasoning"`
	AllowReplan  bool         `yaml:"allow_replan"`
	FocusAreas   []string     `yaml:"focus_areas"`
	SkipAgents   []string     `yaml:"skip_agents"`
	SkipReason   string       `yaml:"skip_reasoning"`
}

// LoadFromFile loads InspectionPlan from YAML or JSON file.
func LoadFromFile(path string) (*InspectionPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workflow file: %w", err)
	}
	var w workflowYAML
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse workflow yaml: %w", err)
	}
	if len(w.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}
	p := &InspectionPlan{
		Assessment:  w.Assessment,
		Priority:    w.Priority,
		Steps:       w.Steps,
		Reasoning:   w.Reasoning,
		AllowReplan: w.AllowReplan,
		FocusAreas:  w.FocusAreas,
		SkipAgents:  w.SkipAgents,
		SkipReason:  w.SkipReason,
		rawJSON:     string(data),
	}
	if p.Priority == "" {
		p.Priority = "normal"
	}
	return p, nil
}

// GetAllAgents returns all agent names in plan (deduplicated).
func (p *InspectionPlan) GetAllAgents() []string {
	seen := make(map[string]bool)
	for _, s := range p.Steps {
		for _, a := range s.Agents {
			seen[a] = true
		}
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	return out
}

// IsEmpty checks if plan has no valid steps.
func (p *InspectionPlan) IsEmpty() bool {
	if len(p.Steps) == 0 {
		return true
	}
	for _, s := range p.Steps {
		if len(s.Agents) > 0 {
			return false
		}
	}
	return true
}

