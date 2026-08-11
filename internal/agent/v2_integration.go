package agent

import (
	"os"
	"strconv"
	"time"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/middleware"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/model"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/resilience"
	"github.com/sirupsen/logrus"
)

// WrapModelWithResilience wraps a ChatModel with v2's circuit breaker and rate
// limiter for LLM API calls. This supplements (does NOT replace) the existing
// K8s API throttle in internal/runtime which handles kubectl-specific rate limits.
func WrapModelWithResilience(cm model.ChatModel) model.ChatModel {
	cb := resilience.NewCircuitBreaker(5, 30*time.Second)
	rl := resilience.NewRateLimiter(10.0, 20)
	return resilience.Wrap(cm, resilience.WithCircuitBreaker(cb), resilience.WithRateLimit(rl))
}

// NewCostTracker creates a CostTrackerMiddleware with budget enforcement.
// Reads K8SOPS_MAX_COST_USD env (default: 0 = no limit).
// This is ADDITIVE — it does not change existing behavior when the env is unset.
func NewCostTracker() *middleware.CostTrackerMiddleware {
	maxCost := 0.0
	if v := os.Getenv("K8SOPS_MAX_COST_USD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			maxCost = f
		}
	}

	prices := map[string]middleware.ModelPrice{
		"gpt-4o":          {InputPerMillion: 2.50, OutputPerMillion: 10.00},
		"gpt-4o-mini":     {InputPerMillion: 0.15, OutputPerMillion: 0.60},
		"gpt-4.1":         {InputPerMillion: 2.00, OutputPerMillion: 8.00},
		"gpt-4.1-mini":    {InputPerMillion: 0.40, OutputPerMillion: 1.60},
		"claude-sonnet-4": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
		"qwen-plus":       {InputPerMillion: 0.80, OutputPerMillion: 2.00},
		"deepseek-chat":   {InputPerMillion: 0.27, OutputPerMillion: 1.10},
	}

	opts := []middleware.CostTrackerOption{
		middleware.WithExchangeRate("CNY", 7.2),
	}
	if maxCost > 0 {
		opts = append(opts, middleware.WithMaxCostUSD(maxCost))
		logrus.Infof("cost-tracker: budget cap set to $%.2f per session", maxCost)
	}

	return middleware.NewCostTrackerMiddleware(prices, opts...)
}
