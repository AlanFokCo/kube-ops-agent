package runtime

import (
	"context"
	"sync"
	"time"
)

// GracefulShutdown manages graceful shutdown flow.
type GracefulShutdown struct {
	cfg          *ProductionConfig
	mu           sync.Mutex
	shutdownSet  bool
	shutdownDone chan struct{}
	tasks        map[interface{}]struct{}
}

// NewGracefulShutdown creates graceful shutdown manager.
func NewGracefulShutdown(cfg *ProductionConfig) *GracefulShutdown {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &GracefulShutdown{
		cfg:          cfg,
		shutdownDone: make(chan struct{}),
		tasks:        make(map[interface{}]struct{}),
	}
}

// IsShuttingDown returns whether shutdown is in progress.
func (g *GracefulShutdown) IsShuttingDown() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.shutdownSet
}

// RequestShutdown requests shutdown.
func (g *GracefulShutdown) RequestShutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.shutdownSet {
		g.shutdownSet = true
		close(g.shutdownDone)
	}
}

// WaitForShutdown waits for shutdown signal.
func (g *GracefulShutdown) WaitForShutdown(ctx context.Context) {
	select {
	case <-g.shutdownDone:
	case <-ctx.Done():
	}
}

// RegisterTask registers task to wait for (e.g. goroutine context).
func (g *GracefulShutdown) RegisterTask(task interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tasks[task] = struct{}{}
}

// UnregisterTask unregisters task.
func (g *GracefulShutdown) UnregisterTask(task interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.tasks, task)
}

// TaskCount returns registered task count.
func (g *GracefulShutdown) TaskCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.tasks)
}

// WaitForTasks waits for all tasks to complete or timeout.
func (g *GracefulShutdown) WaitForTasks(ctx context.Context, waitFn func(context.Context) error) error {
	timeout := g.cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if waitFn != nil {
		return waitFn(ctx)
	}
	return nil
}
