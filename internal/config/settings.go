package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// loadEnvFile loads .env into env vars (pydantic-settings equivalent).
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if key != "" && os.Getenv(key) == "" {
				val = strings.Trim(val, `"'`)
				_ = os.Setenv(key, val)
			}
		}
	}
}

// Settings is full app config (pydantic-settings equivalent).
type Settings struct {
	Server   ServerSettings   `yaml:"server"`
	Runtime  RuntimeSettings  `yaml:"runtime"`
	Agent    AgentSettings   `yaml:"agent"`
	Skills   SkillsSettings   `yaml:"skills"`
	Report   ReportSettings  `yaml:"report"`
	MCP      MCPSettings     `yaml:"mcp"`
	Security SecuritySettings `yaml:"security"`
}

// ServerSettings configures HTTP server.
type ServerSettings struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// RuntimeSettings configures runtime params.
type RuntimeSettings struct {
	MaxConcurrentAgents  int     `yaml:"max_concurrent_agents"`
	MaxConcurrentKubectl int     `yaml:"max_concurrent_kubectl"`
	KubectlRateLimit     float64 `yaml:"kubectl_rate_limit"`
	APIRateLimit         float64 `yaml:"api_rate_limit"`
	KubectlTimeout       string  `yaml:"kubectl_timeout"`
	AgentTimeout         string  `yaml:"agent_timeout"`
	CircuitBreakerThreshold int  `yaml:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   string `yaml:"circuit_breaker_timeout"`
	StateFile              string  `yaml:"state_file"`
	PersistState           bool    `yaml:"persist_state"`
	ShutdownTimeout        string  `yaml:"shutdown_timeout"`
	MinBackoff             string  `yaml:"min_backoff"`
	MaxBackoff             string  `yaml:"max_backoff"`
	BackoffMultiplier      float64 `yaml:"backoff_multiplier"`
}

// AgentSettings configures Agent params.
type AgentSettings struct {
	ModelProvider string `yaml:"model_provider"`
	ModelName     string `yaml:"model_name"`
	APIKey        string `yaml:"api_key"`
	MaxDepth      int    `yaml:"max_depth"`
	MaxIterations int   `yaml:"max_iterations"`
	PlanningThreshold float64 `yaml:"planning_threshold"`
}

// SkillsSettings configures skills directory.
type SkillsSettings struct {
	RootDir string `yaml:"root_dir"`
}

// ReportSettings configures report dir and count limit.
type ReportSettings struct {
	Dir       string `yaml:"dir"`
	MaxReports int   `yaml:"max_reports"` // max reports to retain, 0=unlimited
}

// SecuritySettings corresponds to Python: API auth and CORS.
type SecuritySettings struct {
	APIToken      string   `yaml:"api_token"`       // optional Bearer token
	EnableAuth    bool     `yaml:"enable_auth"`     // whether auth is enabled
	AllowedOrigins []string `yaml:"allowed_origins"` // CORS allowed origins
}

// MCPSettings configures MCP params.
type MCPSettings struct {
	ConfigPath string `yaml:"config_path"`
}

// LoadSettings loads config from YAML and env vars.
func LoadSettings(configPath string) (*Settings, error) {
	// Load .env first (pydantic-settings equivalent)
	loadEnvFile(".env")
	if configPath != "" {
		loadEnvFile(filepath.Join(filepath.Dir(configPath), ".env"))
	}

	s := &Settings{}

	// Set defaults first
	setDefaults(s)

	// If config file exists, load from file
	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, s); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	// Env vars override (higher priority)
	overrideFromEnv(s)

	return s, nil
}

// ToProductionConfig converts Settings to runtime.ProductionConfig.
func (s *Settings) ToProductionConfig() *runtime.ProductionConfig {
	cfg := runtime.DefaultConfig()

	if s.Runtime.MaxConcurrentAgents > 0 {
		cfg.MaxConcurrentAgents = s.Runtime.MaxConcurrentAgents
	}
	if s.Runtime.MaxConcurrentKubectl > 0 {
		cfg.MaxConcurrentKubectl = s.Runtime.MaxConcurrentKubectl
	}
	if s.Runtime.KubectlRateLimit > 0 {
		cfg.KubectlRateLimit = s.Runtime.KubectlRateLimit
	}
	if s.Runtime.APIRateLimit > 0 {
		cfg.APIRateLimit = s.Runtime.APIRateLimit
	}
	if s.Runtime.KubectlTimeout != "" {
		if d, err := parseDuration(s.Runtime.KubectlTimeout); err == nil {
			cfg.KubectlTimeout = d
		}
	}
	if s.Runtime.AgentTimeout != "" {
		if d, err := parseDuration(s.Runtime.AgentTimeout); err == nil {
			cfg.AgentTimeout = d
		}
	}
	if s.Runtime.CircuitBreakerThreshold > 0 {
		cfg.CircuitBreakerThreshold = s.Runtime.CircuitBreakerThreshold
	}
	if s.Runtime.CircuitBreakerTimeout != "" {
		if d, err := parseDuration(s.Runtime.CircuitBreakerTimeout); err == nil {
			cfg.CircuitBreakerTimeout = d
		}
	}
	if s.Runtime.StateFile != "" {
		cfg.StateFile = s.Runtime.StateFile
	}
	cfg.PersistState = s.Runtime.PersistState
	if s.Runtime.ShutdownTimeout != "" {
		if d, err := parseDuration(s.Runtime.ShutdownTimeout); err == nil {
			cfg.ShutdownTimeout = d
		}
	}
	if s.Runtime.MinBackoff != "" {
		if d, err := parseDuration(s.Runtime.MinBackoff); err == nil {
			cfg.MinBackoff = d
		}
	}
	if s.Runtime.MaxBackoff != "" {
		if d, err := parseDuration(s.Runtime.MaxBackoff); err == nil {
			cfg.MaxBackoff = d
		}
	}
	if s.Runtime.BackoffMultiplier > 0 {
		cfg.BackoffMultiplier = s.Runtime.BackoffMultiplier
	}

	return cfg
}

func setDefaults(s *Settings) {
	s.Server.Addr = ":8080"
	s.Server.ReadTimeout = "30s"
	s.Server.WriteTimeout = "30s"

	s.Runtime.MaxConcurrentAgents = 5
	s.Runtime.MaxConcurrentKubectl = 10
	s.Runtime.KubectlRateLimit = 5.0
	s.Runtime.APIRateLimit = 10.0
	s.Runtime.KubectlTimeout = "60s"
	s.Runtime.AgentTimeout = "300s"
	s.Runtime.CircuitBreakerThreshold = 3
	s.Runtime.CircuitBreakerTimeout = "300s"
	s.Runtime.StateFile = "/tmp/k8s-ops-agent-go-state.json"
	s.Runtime.PersistState = true
	s.Runtime.ShutdownTimeout = "30s"
	s.Runtime.MinBackoff = "1s"
	s.Runtime.MaxBackoff = "60s"
	s.Runtime.BackoffMultiplier = 2.0

	s.Agent.ModelProvider = "openai"
	s.Agent.ModelName = "gpt-4o-mini"
	s.Agent.MaxDepth = 3
	s.Agent.MaxIterations = 8
	s.Agent.PlanningThreshold = 0.5

	s.Skills.RootDir = "kubernetes-ops-agent/skills"
	s.Report.Dir = "kubernetes-ops-agent/report"
	s.Report.MaxReports = 0
	s.Security.EnableAuth = false
}

func overrideFromEnv(s *Settings) {
	if v := os.Getenv("K8SOPS_HTTP_ADDR"); v != "" {
		s.Server.Addr = v
	}
	if v := os.Getenv("K8SOPS_SKILLS_DIR"); v != "" {
		s.Skills.RootDir = v
	}
	if v := os.Getenv("K8SOPS_REPORT_DIR"); v != "" {
		s.Report.Dir = v
	}
	if v := os.Getenv("K8SOPS_MCP_CONFIG"); v != "" {
		s.MCP.ConfigPath = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		s.Agent.APIKey = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		s.Agent.ModelName = v
	}
	if v := os.Getenv("K8SOPS_MAX_CONCURRENT_AGENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.Runtime.MaxConcurrentAgents = n
		}
	}
	if v := os.Getenv("K8SOPS_AGENT_TIMEOUT"); v != "" {
		s.Runtime.AgentTimeout = v
	}
	if v := os.Getenv("K8SOPS_STATE_FILE"); v != "" {
		s.Runtime.StateFile = v
	}
	if v := os.Getenv("K8SOPS_API_TOKEN"); v != "" {
		s.Security.APIToken = v
		s.Security.EnableAuth = true
	}
	if v := os.Getenv("K8SOPS_MAX_REPORTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.Report.MaxReports = n
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	return time.ParseDuration(s)
}
