package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettings_Defaults(t *testing.T) {
	s, err := LoadSettings("")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q", s.Server.Addr)
	}
	if s.Runtime.MaxConcurrentAgents != 5 {
		t.Errorf("MaxConcurrentAgents = %d", s.Runtime.MaxConcurrentAgents)
	}
	if s.Agent.ModelProvider != "openai" {
		t.Errorf("ModelProvider = %q", s.Agent.ModelProvider)
	}
	if s.Skills.RootDir == "" {
		t.Error("expected non-empty SkillsRootDir")
	}
}

func TestLoadSettings_FromFile(t *testing.T) {
	dir := t.TempDir()
	yaml := `
server:
  addr: ":9090"
runtime:
  max_concurrent_agents: 10
  persist_state: false
agent:
  model_name: "gpt-4"
skills:
  root_dir: "/tmp/skills"
report:
  dir: "/tmp/reports"
  max_reports: 5
`
	configPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(configPath, []byte(yaml), 0644)

	s, err := LoadSettings(configPath)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %q, want ':9090'", s.Server.Addr)
	}
	if s.Runtime.MaxConcurrentAgents != 10 {
		t.Errorf("MaxConcurrentAgents = %d, want 10", s.Runtime.MaxConcurrentAgents)
	}
	if s.Agent.ModelName != "gpt-4" {
		t.Errorf("ModelName = %q, want 'gpt-4'", s.Agent.ModelName)
	}
	if s.Report.MaxReports != 5 {
		t.Errorf("MaxReports = %d, want 5", s.Report.MaxReports)
	}
}

func TestLoadSettings_EnvOverride(t *testing.T) {
	os.Setenv("K8SOPS_HTTP_ADDR", ":7070")
	os.Setenv("K8SOPS_SKILLS_DIR", "/tmp/my-skills")
	os.Setenv("K8SOPS_API_TOKEN", "secret-token")
	os.Setenv("K8SOPS_MAX_REPORTS", "20")
	os.Setenv("K8SOPS_MAX_CONCURRENT_AGENTS", "8")
	defer func() {
		os.Unsetenv("K8SOPS_HTTP_ADDR")
		os.Unsetenv("K8SOPS_SKILLS_DIR")
		os.Unsetenv("K8SOPS_API_TOKEN")
		os.Unsetenv("K8SOPS_MAX_REPORTS")
		os.Unsetenv("K8SOPS_MAX_CONCURRENT_AGENTS")
	}()

	s, err := LoadSettings("")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Server.Addr != ":7070" {
		t.Errorf("Server.Addr = %q, want ':7070'", s.Server.Addr)
	}
	if s.Skills.RootDir != "/tmp/my-skills" {
		t.Errorf("SkillsRootDir = %q", s.Skills.RootDir)
	}
	if !s.Security.EnableAuth {
		t.Error("expected EnableAuth=true when API_TOKEN set")
	}
	if s.Security.APIToken != "secret-token" {
		t.Errorf("APIToken = %q", s.Security.APIToken)
	}
	if s.Report.MaxReports != 20 {
		t.Errorf("MaxReports = %d, want 20", s.Report.MaxReports)
	}
	if s.Runtime.MaxConcurrentAgents != 8 {
		t.Errorf("MaxConcurrentAgents = %d, want 8", s.Runtime.MaxConcurrentAgents)
	}
}

func TestLoadSettings_ToProductionConfig(t *testing.T) {
	s, err := LoadSettings("")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	cfg := s.ToProductionConfig()
	if cfg == nil {
		t.Fatal("expected non-nil ProductionConfig")
	}
	if cfg.MaxConcurrentAgents != s.Runtime.MaxConcurrentAgents {
		t.Errorf("MaxConcurrentAgents mismatch: %d vs %d", cfg.MaxConcurrentAgents, s.Runtime.MaxConcurrentAgents)
	}
}

func TestLoadSettings_ToProductionConfig_AllFields(t *testing.T) {
	s := &Settings{}
	setDefaults(s)
	s.Runtime.MaxConcurrentAgents = 3
	s.Runtime.MaxConcurrentKubectl = 7
	s.Runtime.KubectlRateLimit = 2.5
	s.Runtime.APIRateLimit = 15.0
	s.Runtime.KubectlTimeout = "45s"
	s.Runtime.AgentTimeout = "600s"
	s.Runtime.CircuitBreakerThreshold = 5
	s.Runtime.CircuitBreakerTimeout = "120s"
	s.Runtime.StateFile = "/tmp/test-state.json"
	s.Runtime.PersistState = false
	s.Runtime.ShutdownTimeout = "15s"
	s.Runtime.MinBackoff = "2s"
	s.Runtime.MaxBackoff = "30s"
	s.Runtime.BackoffMultiplier = 3.0

	cfg := s.ToProductionConfig()
	if cfg.MaxConcurrentAgents != 3 {
		t.Errorf("MaxConcurrentAgents = %d", cfg.MaxConcurrentAgents)
	}
	if cfg.MaxConcurrentKubectl != 7 {
		t.Errorf("MaxConcurrentKubectl = %d", cfg.MaxConcurrentKubectl)
	}
	if cfg.KubectlRateLimit != 2.5 {
		t.Errorf("KubectlRateLimit = %f", cfg.KubectlRateLimit)
	}
	if cfg.PersistState {
		t.Error("expected PersistState=false")
	}
	if cfg.StateFile != "/tmp/test-state.json" {
		t.Errorf("StateFile = %q", cfg.StateFile)
	}
}

func TestLoadSettings_EnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("K8SOPS_ENV_TEST_VAR=from-env-file\n# comment\n\nK8SOPS_REPORT_DIR=/tmp/env-reports\n"), 0644)

	configPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(configPath, []byte(""), 0644)
	// loadEnvFile is called from LoadSettings with dir of config
	s, err := LoadSettings(configPath)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	_ = s
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"30s", false},
		{"5m", false},
		{"1h30m", false},
		{"invalid", true},
		{"", true},
	}
	for _, tt := range tests {
		_, err := parseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDuration(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
	}
}

func TestLoadSettings_EnvOverride_OtherVars(t *testing.T) {
	os.Setenv("K8SOPS_REPORT_DIR", "/tmp/reports2")
	os.Setenv("K8SOPS_MCP_CONFIG", "/tmp/mcp.yaml")
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("OPENAI_MODEL", "gpt-4-turbo")
	os.Setenv("K8SOPS_AGENT_TIMEOUT", "120s")
	os.Setenv("K8SOPS_STATE_FILE", "/tmp/test-state.json")
	defer func() {
		os.Unsetenv("K8SOPS_REPORT_DIR")
		os.Unsetenv("K8SOPS_MCP_CONFIG")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_MODEL")
		os.Unsetenv("K8SOPS_AGENT_TIMEOUT")
		os.Unsetenv("K8SOPS_STATE_FILE")
	}()

	s, err := LoadSettings("")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Report.Dir != "/tmp/reports2" {
		t.Errorf("Report.Dir = %q", s.Report.Dir)
	}
	if s.MCP.ConfigPath != "/tmp/mcp.yaml" {
		t.Errorf("MCP.ConfigPath = %q", s.MCP.ConfigPath)
	}
	if s.Agent.APIKey != "test-key" {
		t.Errorf("APIKey = %q", s.Agent.APIKey)
	}
	if s.Agent.ModelName != "gpt-4-turbo" {
		t.Errorf("ModelName = %q", s.Agent.ModelName)
	}
	if s.Runtime.AgentTimeout != "120s" {
		t.Errorf("AgentTimeout = %q", s.Runtime.AgentTimeout)
	}
	if s.Runtime.StateFile != "/tmp/test-state.json" {
		t.Errorf("StateFile = %q", s.Runtime.StateFile)
	}
}

func TestLoadEnvFile_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte(`K8SOPS_QUOTED_TEST="quoted value"
K8SOPS_SINGLE_QUOTED='single quoted'
`), 0644)
	os.Unsetenv("K8SOPS_QUOTED_TEST")
	os.Unsetenv("K8SOPS_SINGLE_QUOTED")
	defer func() {
		os.Unsetenv("K8SOPS_QUOTED_TEST")
		os.Unsetenv("K8SOPS_SINGLE_QUOTED")
	}()
	loadEnvFile(envFile)
	if v := os.Getenv("K8SOPS_QUOTED_TEST"); v != "quoted value" {
		t.Errorf("expected 'quoted value', got %q", v)
	}
	if v := os.Getenv("K8SOPS_SINGLE_QUOTED"); v != "single quoted" {
		t.Errorf("expected 'single quoted', got %q", v)
	}
}

func TestLoadEnvFile_NotExist(t *testing.T) {
	// Should not panic or error
	loadEnvFile("/nonexistent/.env")
}

func TestLoadEnvFile_ExistingEnvNotOverridden(t *testing.T) {
	os.Setenv("K8SOPS_EXISTING_VAR", "original")
	defer os.Unsetenv("K8SOPS_EXISTING_VAR")

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("K8SOPS_EXISTING_VAR=new-value\n"), 0644)
	loadEnvFile(envFile)

	if v := os.Getenv("K8SOPS_EXISTING_VAR"); v != "original" {
		t.Errorf("expected original value preserved, got %q", v)
	}
}
