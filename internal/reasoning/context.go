package reasoning

import (
	"strings"
	"time"
)

// ThinkingContext is full thinking context, aligned with Python reasoning.ThinkingContext.
// Supports task, context, available_tools, constraints, previous_results, depth, parent_context.
type ThinkingContext struct {
	ID               string            `json:"id"`
	Goal             string            `json:"goal"`
	Task             string            `json:"task,omitempty"`
	Context          map[string]any    `json:"context,omitempty"`
	AvailableTools   []string          `json:"available_tools,omitempty"`
	Constraints      []string          `json:"constraints,omitempty"`
	PreviousResults  []any             `json:"previous_results,omitempty"`
	Depth            int               `json:"depth"`
	MaxDepth         int               `json:"max_depth"`
	ParentContext    *ThinkingContext  `json:"parent_context,omitempty"`
	Analysis         *Analysis         `json:"analysis,omitempty"`
	Plan             *ThinkingPlan     `json:"plan,omitempty"`
	Loop             *ReasoningLoop    `json:"loop,omitempty"`
	ReplanReason     string            `json:"replan_reason,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// CreateChildContext creates child context for sub-problem decomposition (aligned with Python create_child_context).
func (t *ThinkingContext) CreateChildContext(subTask string, subContext map[string]any) *ThinkingContext {
	if subContext == nil {
		subContext = make(map[string]any)
	}
	return &ThinkingContext{
		Task:            subTask,
		Context:         subContext,
		AvailableTools:  t.AvailableTools,
		Constraints:     t.Constraints,
		PreviousResults: nil,
		Depth:           t.Depth + 1,
		MaxDepth:        t.MaxDepth,
		ParentContext:   t,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// CanRecurse returns whether recursion can continue.
func (t *ThinkingContext) CanRecurse() bool {
	return t.Depth < t.MaxDepth
}

// Analysis is problem/domain analysis result, aligned with Python reasoning.Analysis.
type Analysis struct {
	Domain            string   `json:"domain,omitempty"`
	Complexity        string   `json:"complexity,omitempty"`        // simple | medium | complex | uncertain
	NeedsPlanning     bool     `json:"needs_planning"`
	Reasoning         string   `json:"reasoning,omitempty"`
	MissingInformation []string `json:"missing_information,omitempty"`
	SuggestedApproach string   `json:"suggested_approach,omitempty"`
	Confidence        float64  `json:"confidence,omitempty"` // 0.0-1.0
	DetectedIssues    []string `json:"detected_issues,omitempty"`
	KeyAreas          []string `json:"key_areas,omitempty"`
	PotentialRisks    []string `json:"potential_risks,omitempty"`
	Summary           string   `json:"summary,omitempty"`
	Recommendations   []string `json:"recommendations,omitempty"`
	RawOutput         string   `json:"raw_output,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// ThinkingResult is the result of thinking/reasoning process, aligned with Python core/reasoning.ThinkingResult.
type ThinkingResult struct {
	Success          bool             `json:"success"`
	Output           any              `json:"output"`
	Analysis         *Analysis        `json:"analysis,omitempty"`
	NeedsReplan      bool             `json:"needs_replan"`
	ReplanReason     string           `json:"replan_reason,omitempty"`
	NewContext       *ThinkingContext  `json:"new_context,omitempty"`
	SubResults       []*ThinkingResult `json:"sub_results,omitempty"`
	DurationSeconds  float64          `json:"duration_seconds"`
	Depth            int              `json:"depth"`
}

// GetAllOutputs recursively collects output from this result and all sub-results (aligned with Python get_all_outputs).
func (t *ThinkingResult) GetAllOutputs() []any {
	if t == nil {
		return nil
	}
	var out []any
	if t.Output != nil {
		out = append(out, t.Output)
	}
	for _, sub := range t.SubResults {
		out = append(out, sub.GetAllOutputs()...)
	}
	return out
}

// ReplanReason constants for replan reasons.
const (
	ReplanReasonUnexpected   = "unexpected_finding"
	ReplanReasonFailed       = "step_failed"
	ReplanReasonIncomplete   = "incomplete"
	ReplanReasonNewContext   = "new_context"
	ReplanReasonUserRequest  = "user_request"
)

// NewThinkingContext creates new thinking context.
func NewThinkingContext(goal string) *ThinkingContext {
	now := time.Now()
	return &ThinkingContext{
		ID:        generateID(),
		Goal:      goal,
		Task:      goal,
		Depth:     0,
		MaxDepth:  3,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewAnalysis creates new analysis result.
func NewAnalysis(domain, complexity, summary string, keyAreas, risks, recommendations []string) *Analysis {
	return &Analysis{
		Domain:         domain,
		Complexity:     complexity,
		Summary:        summary,
		KeyAreas:       keyAreas,
		PotentialRisks: risks,
		Recommendations: recommendations,
		CreatedAt:      time.Now(),
	}
}

// NewAnalysisFromDict creates Analysis from LLM response dict (aligned with Python Analysis.from_dict).
func NewAnalysisFromDict(data map[string]any) *Analysis {
	complexity := "simple"
	if c, ok := data["complexity"].(string); ok {
		complexity = strings.ToLower(c)
	}
	needsPlanning := false
	if np, ok := data["needs_planning"].(bool); ok {
		needsPlanning = np
	}
	reasoning := ""
	if r, ok := data["reasoning"].(string); ok {
		reasoning = r
	}
	confidence := 0.5
	if cf, ok := data["confidence"].(float64); ok {
		confidence = cf
	}
	var missingInfo []string
	if mi, ok := data["missing_information"].([]any); ok {
		for _, v := range mi {
			if s, ok := v.(string); ok {
				missingInfo = append(missingInfo, s)
			}
		}
	}
	suggested := ""
	if s, ok := data["suggested_approach"].(string); ok {
		suggested = s
	}
	var issues []string
	if di, ok := data["detected_issues"].([]any); ok {
		for _, v := range di {
			if s, ok := v.(string); ok {
				issues = append(issues, s)
			}
		}
	}
	return &Analysis{
		Complexity:         complexity,
		NeedsPlanning:      needsPlanning,
		Reasoning:          reasoning,
		Confidence:         confidence,
		MissingInformation: missingInfo,
		SuggestedApproach:  suggested,
		DetectedIssues:     issues,
		CreatedAt:          time.Now(),
	}
}
