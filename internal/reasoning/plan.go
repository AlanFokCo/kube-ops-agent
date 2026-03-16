package reasoning

import "time"

// PlanStep represents a step in a plan (expected_outcome, complexity, verification, sub_plan).
type PlanStep struct {
	ID              string                 `json:"id"`
	Description     string                 `json:"description"`
	Action          string                 `json:"action"`
	Inputs          map[string]interface{} `json:"inputs,omitempty"`
	DependsOn       []string               `json:"depends_on,omitempty"`
	ExpectedOutcome string                 `json:"expected_outcome,omitempty"`
	Verification    string                 `json:"verification,omitempty"` // criteria for step completion
	Complexity      StepComplexity         `json:"complexity,omitempty"`
	ToolHint        string                 `json:"tool_hint,omitempty"`
	SubPlan         *ThinkingPlan          `json:"sub_plan,omitempty"`
	Status          StepStatus             `json:"status"`
	Result          string                 `json:"result,omitempty"`
	Error           string                 `json:"error,omitempty"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	FinishedAt      *time.Time             `json:"finished_at,omitempty"`
}

// StepStatus represents step execution status (full enum).
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusTimeout   StepStatus = "timeout"
	StepStatusCancelled StepStatus = "cancelled"
)

// StepComplexity represents step complexity.
type StepComplexity string

const (
	StepComplexitySimple   StepComplexity = "simple"
	StepComplexityMedium  StepComplexity = "medium"
	StepComplexityComplex StepComplexity = "complex"
	StepComplexityUncertain StepComplexity = "uncertain" // unknown before execution
)

// ThinkingPlan represents a complete thinking plan.
type ThinkingPlan struct {
	ID          string     `json:"id"`
	Goal        string     `json:"goal"`
	Steps       []PlanStep `json:"steps"`
	CurrentStep int        `json:"current_step"`
	Status      PlanStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PlanStatus represents overall plan status.
type PlanStatus string

const (
	PlanStatusPlanning   PlanStatus = "planning"
	PlanStatusExecuting  PlanStatus = "executing"
	PlanStatusCompleted  PlanStatus = "completed"
	PlanStatusFailed     PlanStatus = "failed"
	PlanStatusCancelled  PlanStatus = "cancelled"
)

// ReasoningLoop represents a complete reasoning loop.
type ReasoningLoop struct {
	ID           string        `json:"id"`
	Plan         *ThinkingPlan `json:"plan"`
	Iteration    int           `json:"iteration"`
	MaxIterations int          `json:"max_iterations"`
	Reflections  []Reflection  `json:"reflections"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Reflection represents one reflection/replanning.
type Reflection struct {
	Iteration   int       `json:"iteration"`
	Observation string    `json:"observation"`
	Decision    string    `json:"decision"`
	NewPlan     *ThinkingPlan `json:"new_plan,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewThinkingPlan creates a new thinking plan.
func NewThinkingPlan(goal string) *ThinkingPlan {
	now := time.Now()
	return &ThinkingPlan{
		ID:          generateID(),
		Goal:        goal,
		Steps:       make([]PlanStep, 0),
		CurrentStep: 0,
		Status:      PlanStatusPlanning,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewReasoningLoop creates a new reasoning loop.
func NewReasoningLoop(maxIterations int) *ReasoningLoop {
	now := time.Now()
	return &ReasoningLoop{
		ID:            generateID(),
		Plan:          nil,
		Iteration:     0,
		MaxIterations: maxIterations,
		Reflections:    make([]Reflection, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// AddStep adds a step to the plan.
func (p *ThinkingPlan) AddStep(description, action string, inputs map[string]interface{}) {
	step := PlanStep{
		ID:          generateID(),
		Description: description,
		Action:      action,
		Inputs:      inputs,
		Status:      StepStatusPending,
	}
	p.Steps = append(p.Steps, step)
	p.UpdatedAt = time.Now()
}

// MarkStepRunning marks step as running.
func (p *ThinkingPlan) MarkStepRunning(stepIndex int) {
	if stepIndex >= 0 && stepIndex < len(p.Steps) {
		now := time.Now()
		p.Steps[stepIndex].Status = StepStatusRunning
		p.Steps[stepIndex].StartedAt = &now
		p.UpdatedAt = now
	}
}

// MarkStepCompleted marks step as completed.
func (p *ThinkingPlan) MarkStepCompleted(stepIndex int, result string) {
	if stepIndex >= 0 && stepIndex < len(p.Steps) {
		now := time.Now()
		p.Steps[stepIndex].Status = StepStatusCompleted
		p.Steps[stepIndex].Result = result
		p.Steps[stepIndex].FinishedAt = &now
		p.CurrentStep = stepIndex + 1
		p.UpdatedAt = now
	}
}

// MarkStepFailed marks step as failed.
func (p *ThinkingPlan) MarkStepFailed(stepIndex int, errMsg string) {
	if stepIndex >= 0 && stepIndex < len(p.Steps) {
		now := time.Now()
		p.Steps[stepIndex].Status = StepStatusFailed
		p.Steps[stepIndex].Error = errMsg
		p.Steps[stepIndex].FinishedAt = &now
		p.Status = PlanStatusFailed
		p.UpdatedAt = now
	}
}

// AddReflection adds one reflection.
func (r *ReasoningLoop) AddReflection(observation, decision string, newPlan *ThinkingPlan) {
	r.Iteration++
	reflection := Reflection{
		Iteration:   r.Iteration,
		Observation: observation,
		Decision:    decision,
		NewPlan:     newPlan,
		Timestamp:   time.Now(),
	}
	r.Reflections = append(r.Reflections, reflection)
	r.UpdatedAt = time.Now()
	if newPlan != nil {
		r.Plan = newPlan
	}
}

// PlanRevision represents revision to existing plan for replanning, aligned with Python core/planning.PlanRevision.
type PlanRevision struct {
	OriginalPlan   *ThinkingPlan `json:"original_plan"`
	Reason         string        `json:"reason"`
	Changes        []map[string]any `json:"changes,omitempty"`
	NewSteps       []PlanStep    `json:"new_steps,omitempty"`
	RemovedStepIDs []string      `json:"removed_step_ids,omitempty"`
}

// Apply applies revision to produce new ThinkingPlan.
func (r *PlanRevision) Apply() *ThinkingPlan {
	if r == nil || r.OriginalPlan == nil {
		return nil
	}
	removed := make(map[string]bool)
	for _, id := range r.RemovedStepIDs {
		removed[id] = true
	}
	var kept []PlanStep
	for _, s := range r.OriginalPlan.Steps {
		if s.Status == StepStatusCompleted && !removed[s.ID] {
			kept = append(kept, s)
		}
	}
	allSteps := append(kept, r.NewSteps...)
	return &ThinkingPlan{
		ID:          generateID(),
		Goal:        r.OriginalPlan.Goal,
		Steps:       allSteps,
		CurrentStep: len(kept),
		Status:      PlanStatusExecuting,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
