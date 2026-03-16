package runtime

import (
	"context"
	"sync"
	"time"
)

// ProductionConfig corresponds to Python core/production.ProductionConfig subset.
type ProductionConfig struct {
	// Concurrency control
	MaxConcurrentAgents  int
	MaxConcurrentKubectl int

	// Rate limiting
	KubectlRateLimit float64
	APIRateLimit     float64

	// Timeout
	KubectlTimeout time.Duration
	AgentTimeout   time.Duration

	// Circuit breaker
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration

	// State persistence (interface only, implement as needed)
	StateFile      string
	PersistState   bool
	ShutdownTimeout time.Duration

	// Backoff
	MinBackoff       time.Duration
	MaxBackoff       time.Duration
	BackoffMultiplier float64
}

func DefaultConfig() *ProductionConfig {
	return &ProductionConfig{
		MaxConcurrentAgents:     5,
		MaxConcurrentKubectl:    10,
		KubectlRateLimit:        5.0,
		APIRateLimit:            10.0,
		KubectlTimeout:          60 * time.Second,
		AgentTimeout:            300 * time.Second,
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   300 * time.Second,
		StateFile:               "/tmp/k8s-ops-agent-go-state.json",
		PersistState:            true,
		ShutdownTimeout:         30 * time.Second,
		MinBackoff:              1 * time.Second,
		MaxBackoff:              60 * time.Second,
		BackoffMultiplier:       2.0,
	}
}

// OpsRecorder records execution results to operations.Manager.
type OpsRecorder interface {
	Record(agentName string, success bool, startedAt, finishedAt time.Time, durationSeconds float64, errMsg string)
}

// Environment aggregates runtime deps for passing between layers.
type Environment struct {
	Config        *ProductionConfig
	Concurrency   *ConcurrencyController
	KubectlLimit  *RateLimiter
	APILimit      *RateLimiter
	Circuit       *CircuitBreaker
	State         *StatePersistence
	Metrics       *MetricsCollector
	Graceful      *GracefulShutdown
	OpsRecorder   OpsRecorder
}

func NewEnvironment(cfg *ProductionConfig) *Environment {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Environment{
		Config:       cfg,
		Concurrency:  NewConcurrencyController(cfg),
		KubectlLimit: NewRateLimiter(cfg.KubectlRateLimit, int(cfg.KubectlRateLimit*2)),
		APILimit:     NewRateLimiter(cfg.APIRateLimit, int(cfg.APIRateLimit*2)),
		Circuit:      NewCircuitBreaker(cfg),
		State:        NewStatePersistence(cfg),
		Metrics:      NewMetricsCollector(),
		Graceful:     NewGracefulShutdown(cfg),
	}
}

// ConcurrencyController controls Agent and kubectl concurrency.
type ConcurrencyController struct {
	cfg *ProductionConfig

	agentCh   chan struct{}
	kubectlCh chan struct{}

	mu           sync.Mutex
	activeAgents map[string]struct{}
}

func NewConcurrencyController(cfg *ProductionConfig) *ConcurrencyController {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &ConcurrencyController{
		cfg:          cfg,
		agentCh:      make(chan struct{}, cfg.MaxConcurrentAgents),
		kubectlCh:    make(chan struct{}, cfg.MaxConcurrentKubectl),
		activeAgents: make(map[string]struct{}),
	}
}

// WithAgentSlot acquires a concurrency slot for an Agent.
func (c *ConcurrencyController) WithAgentSlot(ctx context.Context, agentName string, fn func(ctx context.Context) error) error {
	select {
	case c.agentCh <- struct{}{}:
		// got slot
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.agentCh }()

	c.mu.Lock()
	c.activeAgents[agentName] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.activeAgents, agentName)
		c.mu.Unlock()
	}()

	return fn(ctx)
}

func (c *ConcurrencyController) ActiveAgentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.activeAgents)
}

// ActiveAgents returns currently active Agent names.
func (c *ConcurrencyController) ActiveAgents() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.activeAgents))
	for name := range c.activeAgents {
		out = append(out, name)
	}
	return out
}

// RateLimiter is simple token bucket for kubectl/API rate limiting.
type RateLimiter struct {
	rate   float64
	burst  int
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		rate:   rate,
		burst:  burst,
		tokens: float64(burst),
		last:   time.Now(),
	}
}

func (r *RateLimiter) Wait(ctx context.Context, tokens int) error {
	if tokens <= 0 {
		return nil
	}

	for {
		r.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(r.last).Seconds()
		if elapsed > 0 {
			r.tokens += elapsed * r.rate
			if r.tokens > float64(r.burst) {
				r.tokens = float64(r.burst)
			}
			r.last = now
		}

		if r.tokens >= float64(tokens) {
			r.tokens -= float64(tokens)
			r.mu.Unlock()
			return nil
		}

		need := float64(tokens) - r.tokens
		wait := need / r.rate
		r.mu.Unlock()

		t := time.NewTimer(time.Duration(wait*float64(time.Second)))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// CircuitBreaker is simple circuit breaker implementation.
type CircuitBreaker struct {
	threshold int
	timeout   time.Duration

	mu          sync.Mutex
	failures    map[string]int
	openUntil   map[string]time.Time
}

func NewCircuitBreaker(cfg *ProductionConfig) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &CircuitBreaker{
		threshold:  cfg.CircuitBreakerThreshold,
		timeout:    cfg.CircuitBreakerTimeout,
		failures:   make(map[string]int),
		openUntil:  make(map[string]time.Time),
	}
}

func (cb *CircuitBreaker) IsOpen(agent string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	until, ok := cb.openUntil[agent]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(cb.openUntil, agent)
		cb.failures[agent] = 0
		return false
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess(agent string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.failures, agent)
	delete(cb.openUntil, agent)
}

func (cb *CircuitBreaker) RecordFailure(agent string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[agent]++
	if cb.failures[agent] >= cb.threshold {
		cb.openUntil[agent] = time.Now().Add(cb.timeout)
	}
}

// GetStatus returns circuit breaker status for /health, /metrics.
func (cb *CircuitBreaker) GetStatus() map[string]any {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	open := make([]string, 0)
	for agent, until := range cb.openUntil {
		if time.Now().Before(until) {
			open = append(open, agent)
		}
	}
	failures := make(map[string]int)
	for k, v := range cb.failures {
		failures[k] = v
	}
	return map[string]any{
		"open_circuits":   open,
		"failure_counts": failures,
	}
}

