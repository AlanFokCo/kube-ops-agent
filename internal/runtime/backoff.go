package runtime

import (
	"math"
	"math/rand"
	"time"
)

// CalculateBackoff computes exponential backoff duration with jitter.
func CalculateBackoff(attempt int, cfg *ProductionConfig) time.Duration {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	backoff := float64(cfg.MinBackoff)
	if attempt > 0 {
		backoff = backoff * math.Pow(cfg.BackoffMultiplier, float64(attempt))
	}
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}
	jitter := backoff * (0.25 * rand.Float64())
	result := backoff + jitter
	if result > float64(cfg.MaxBackoff) {
		result = float64(cfg.MaxBackoff)
	}
	return time.Duration(result)
}
