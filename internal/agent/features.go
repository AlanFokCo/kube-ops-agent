package agent

import (
	"os"
	"path/filepath"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/audit"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/middleware"
	"github.com/sirupsen/logrus"
)

// DefaultGuardrailMiddleware creates a guardrail that redacts common secret
// patterns from model output. Critical for K8s inspection where configmaps
// and env vars can leak credentials into LLM responses.
func DefaultGuardrailMiddleware() *middleware.GuardrailMiddleware {
	return middleware.NewGuardrailMiddleware(
		middleware.KeywordRedactRule("aws-keys", "[REDACTED]", "AKIA"),
		middleware.KeywordRedactRule("github-tokens", "[REDACTED]", "ghp_", "gho_", "ghs_"),
		middleware.KeywordRedactRule("bearer-tokens", "[REDACTED]", "Bearer ey"),
		middleware.MaxLengthRule("max-output", 50000, middleware.GuardrailWarn),
	)
}

// NewAuditLogger creates an audit file logger at the given directory.
// Returns a NopLogger if the directory cannot be created.
func NewAuditLogger(dir string) audit.Logger {
	if dir == "" {
		dir = "/var/log/kube-ops-agent/audit"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logrus.Warnf("audit: cannot create dir %s: %v; auditing disabled", dir, err)
		return audit.NopLogger{}
	}
	path := filepath.Join(dir, "audit.jsonl")
	logger, err := audit.NewFileLogger(path)
	if err != nil {
		logrus.Warnf("audit: cannot open %s: %v; auditing disabled", path, err)
		return audit.NopLogger{}
	}
	return logger
}
