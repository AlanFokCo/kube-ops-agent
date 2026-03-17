package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	"github.com/alanfokco/kube-ops-agent-go/internal/logging"
	"github.com/alanfokco/kube-ops-agent-go/internal/plan"
	"github.com/alanfokco/kube-ops-agent-go/internal/report"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// Mode represents scheduler mode.
type Mode int

const (
	ModeSimple Mode = iota
	ModeIntelligent
)

// Scheduler periodically triggers Agents.
type Scheduler struct {
	mode             Mode
	registry         agent.Registry
	exec             *agent.Executor
	env              *runtime.Environment
	reportDir        string
	checkInterval    time.Duration
	intervalOverride int // if >0, overrides all Agents' interval_seconds
	workflowPath     string // optional: static workflow YAML, takes precedence over LLM

	orchestrator        *agent.OrchestratorAgent
	selfDrivenOrch      *agent.SelfDrivenOrchestrator
	summary             *agent.SummaryAgent
	selfDrivenSummary   *agent.SelfDrivenSummary

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

func New(
	mode Mode,
	reg agent.Registry,
	exec *agent.Executor,
	env *runtime.Environment,
	reportDir string,
	orch *agent.OrchestratorAgent,
	sum *agent.SummaryAgent,
	selfOrch *agent.SelfDrivenOrchestrator,
	selfSum *agent.SelfDrivenSummary,
) *Scheduler {
	return NewWithCheckInterval(mode, reg, exec, env, reportDir, 10*time.Second, orch, sum, selfOrch, selfSum)
}

// NewWithCheckInterval creates scheduler with custom check interval.
func NewWithCheckInterval(
	mode Mode,
	reg agent.Registry,
	exec *agent.Executor,
	env *runtime.Environment,
	reportDir string,
	checkInterval time.Duration,
	orch *agent.OrchestratorAgent,
	sum *agent.SummaryAgent,
	selfOrch *agent.SelfDrivenOrchestrator,
	selfSum *agent.SelfDrivenSummary,
	opts ...SchedulerOption,
) *Scheduler {
	return NewWithOptions(mode, reg, exec, env, reportDir, checkInterval, 0, orch, sum, selfOrch, selfSum, opts...)
}

// SchedulerOption configures Scheduler.
type SchedulerOption func(*Scheduler)

// WithWorkflowPath sets static workflow YAML path; when set, uses static plan instead of LLM self-planning.
func WithWorkflowPath(path string) SchedulerOption {
	return func(s *Scheduler) {
		s.workflowPath = path
	}
}

// NewWithOptions creates scheduler with full config.
func NewWithOptions(
	mode Mode,
	reg agent.Registry,
	exec *agent.Executor,
	env *runtime.Environment,
	reportDir string,
	checkInterval time.Duration,
	intervalOverride int,
	orch *agent.OrchestratorAgent,
	sum *agent.SummaryAgent,
	selfOrch *agent.SelfDrivenOrchestrator,
	selfSum *agent.SelfDrivenSummary,
	opts ...SchedulerOption,
) *Scheduler {
	if checkInterval <= 0 {
		checkInterval = 10 * time.Second
	}
	s := &Scheduler{
		mode:              mode,
		registry:          reg,
		exec:              exec,
		env:               env,
		reportDir:         reportDir,
		checkInterval:     checkInterval,
		intervalOverride:  intervalOverride,
		orchestrator:      orch,
		summary:           sum,
		selfDrivenOrch:    selfOrch,
		selfDrivenSummary: selfSum,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Start launches the scheduler loop in a separate goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	if s.env.State != nil {
		_, _ = s.env.State.Load()
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	go s.loop(ctx)
}

// Stop requests scheduler to stop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.cancel()
	s.running = false
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switch s.mode {
			case ModeSimple:
				s.runSimpleRound(ctx)
			case ModeIntelligent:
				s.runIntelligentRound(ctx)
			}
		}
	}
}

// runSimpleRound simple mode: determines due based on last_run_at + interval_seconds.
func (s *Scheduler) runSimpleRound(ctx context.Context) {
	now := time.Now()
	specs := s.registry.Specs()
	var due []agent.Spec
	intervalSec := s.intervalOverride
	for _, sp := range specs {
		iv := sp.IntervalSecond
		if intervalSec > 0 {
			iv = intervalSec
		}
		if iv <= 0 {
			continue
		}
		var lastRun *time.Time
		if s.env.State != nil {
			lastRun = s.env.State.GetAgentLastRun(sp.Name)
		}
		if lastRun == nil {
			due = append(due, sp)
			continue
		}
		elapsed := now.Sub(*lastRun).Seconds()
		if elapsed >= float64(iv) {
			due = append(due, sp)
		}
	}
	if len(due) == 0 {
		return
	}
	s.runOneRoundSync(ctx, due)
	if s.env.State != nil && s.env.State.IsDirty() {
		_ = s.env.State.Save()
	}
}

// RunOneRound runs one round asynchronously (for /trigger endpoint).
func (s *Scheduler) RunOneRound(specs []agent.Spec) {
	go s.runOneRoundSync(context.Background(), specs)
}

func (s *Scheduler) runOneRoundSync(ctx context.Context, specs []agent.Spec) {
	now := time.Now()
	var wg sync.WaitGroup
	for _, sp := range specs {
		wg.Add(1)
		go func(spec agent.Spec) {
			defer wg.Done()
			_, _ = s.exec.Execute(ctx, spec.Name, map[string]any{
				"trigger":   "manual_http",
				"timestamp": now.Unix(),
			})
		}(sp)
	}
	wg.Wait()
	if s.env.State != nil && s.env.State.IsDirty() {
		_ = s.env.State.Save()
	}
}

// GetStatus returns scheduler status for /health endpoint.
func (s *Scheduler) GetStatus() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"running":       s.running,
		"mode":          s.modeString(),
		"active_agents": s.env.Concurrency.ActiveAgentCount(),
	}
}

func (s *Scheduler) modeString() string {
	if s.mode == ModeIntelligent {
		return "intelligent"
	}
	return "simple"
}

// runIntelligentRound uses OrchestratorAgent to generate plan and execute step by step.
func (s *Scheduler) runIntelligentRound(ctx context.Context) {
	if (s.orchestrator == nil && s.selfDrivenOrch == nil) ||
		(s.summary == nil && s.selfDrivenSummary == nil) {
		// Fall back to simple mode
		s.runSimpleRound(ctx)
		return
	}

	specs := s.registry.Specs()
	var planObj *plan.InspectionPlan

	// 0) If workflow file is set, load static plan first
	if s.workflowPath != "" {
		if p, err := plan.LoadFromFile(s.workflowPath); err == nil {
			planObj = p
		}
	}

	// 1) Prefer SelfDrivenOrchestrator to generate plan (when no static workflow)
	if planObj == nil && s.selfDrivenOrch != nil {
		res, err := s.selfDrivenOrch.Orchestrate(ctx, nil)
		if err == nil && res != nil && res.RawPlanJSON != "" {
			if p, err := plan.FromJSON(res.RawPlanJSON); err == nil {
				planObj = p
			}
		}
	}

	// 2) Fall back to traditional OrchestratorAgent on failure
	if planObj == nil && s.orchestrator != nil {
		p, err := s.orchestrator.Plan(ctx, specs, "Periodic cluster health inspection")
		if err == nil {
			planObj = p
		}
	}
	if planObj == nil {
		// create_fallback_plan: use default plan when orchestrator fails
		planObj = createFallbackPlan(specs)
		if planObj == nil {
			return
		}
	}

	results := make(map[string]string)

	for stepIdx, step := range planObj.Steps {
		if len(step.Agents) == 0 {
			continue
		}
		// depends_on: skip this step if dependent agents have no results
		if len(step.DependsOn) > 0 {
			depMet := true
			for _, dep := range step.DependsOn {
				if _, ok := results[dep]; !ok {
					depMet = false
					break
				}
			}
			if !depMet {
				continue
			}
		}

		stepStart := time.Now()

		// Ignore agents in skip_agents.
		agents := make([]string, 0, len(step.Agents))
		for _, name := range step.Agents {
			skip := false
			for _, sk := range planObj.SkipAgents {
				if sk == name {
					skip = true
					break
				}
			}
			if !skip {
				agents = append(agents, name)
			}
		}
		if len(agents) == 0 {
			continue
		}

		// step timeout: create context with timeout if timeout_seconds is set
		stepCtx := ctx
		var stepCancel context.CancelFunc
		if step.TimeoutSecs > 0 {
			stepCtx, stepCancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSecs)*time.Second)
		}

		// Pass dependent agent results into input
		baseInput := map[string]any{
			"trigger":       "scheduler_intelligent",
			"focus_areas":   step.FocusAreas,
			"assessment":    planObj.Assessment,
			"step_reason":   step.Condition,
			"plan_priority": planObj.Priority,
		}
		for _, dep := range step.DependsOn {
			if r, ok := results[dep]; ok {
				baseInput[dep] = r
			}
		}

		maxParallel := s.env.Config.MaxConcurrentAgents
		if maxParallel <= 0 {
			maxParallel = 5
		}
		sem := make(chan struct{}, maxParallel)

		switch step.Mode {
		case plan.ModeSequential:
			for _, name := range agents {
				input := make(map[string]any)
				for k, v := range baseInput {
					input[k] = v
				}
				msg, err := s.exec.Execute(stepCtx, name, input)
				if err == nil && msg != nil {
					if txt := msg.GetTextContent(""); txt != nil {
						results[name] = *txt
					}
				}
			}
		default: // parallel, limit concurrency
			var wg sync.WaitGroup
			mu := sync.Mutex{}
			for _, name := range agents {
				wg.Add(1)
				go func(agentName string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					input := make(map[string]any)
					for k, v := range baseInput {
						input[k] = v
					}
					msg, err := s.exec.Execute(stepCtx, agentName, input)
					if err != nil || msg == nil {
						return
					}
					if txt := msg.GetTextContent(""); txt != nil {
						mu.Lock()
						results[agentName] = *txt
						mu.Unlock()
					}
				}(name)
			}
			wg.Wait()
		}

		duration := time.Since(stepStart).Seconds()
		success := true
		for _, name := range agents {
			if _, ok := results[name]; !ok {
				success = false
				break
			}
		}
		mode := "parallel"
		if step.Mode == plan.ModeSequential {
			mode = "sequential"
		}
		logging.LogPlanExecution(stepIdx+1, len(planObj.Steps), len(agents), mode, duration, success)
		if stepCancel != nil {
			stepCancel()
		}
	}

	// Aggregate results into report: prefer SelfDrivenSummary
	if s.selfDrivenSummary != nil {
		if res, err := s.selfDrivenSummary.SummarizeThinking(ctx, planObj, results); err == nil && res != nil {
			content := fmt.Sprintf("%v", res.Output)
			_ = s.writeReport(content)
			return
		}
	}

	if s.summary != nil {
		reportContent, err := s.summary.Summarize(ctx, planObj, results)
		if err != nil {
			return
		}
		_ = s.writeReport(reportContent)
	}
	if s.env.State != nil && s.env.State.IsDirty() {
		_ = s.env.State.Save()
	}
}

// createFallbackPlan creates default parallel execution plan when orchestrator cannot generate one.
func createFallbackPlan(specs []agent.Spec) *plan.InspectionPlan {
	if len(specs) == 0 {
		return nil
	}
	agents := make([]string, 0, len(specs))
	for _, sp := range specs {
		agents = append(agents, sp.Name)
	}
	return &plan.InspectionPlan{
		Assessment: "Fallback: full cluster inspection",
		Priority:   "medium",
		Steps: []plan.Step{{
			Agents: agents,
			Mode:   plan.ModeParallel,
		}},
		Reasoning: "Orchestrator failed; using fallback plan",
		AllowReplan: false,
	}
}

func (s *Scheduler) writeReport(content string) error {
	if s.reportDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.reportDir, 0o755); err != nil {
		return err
	}
	filename := report.ReportFilename(time.Now())
	path := filepath.Join(s.reportDir, filename)
	return os.WriteFile(path, []byte(content), 0o644)
}


