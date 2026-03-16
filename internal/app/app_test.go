package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// testRegistry implements agent.Registry for testing.
type testRegistry struct {
	specs []agent.Spec
}

func (r *testRegistry) Specs() []agent.Spec                        { return r.specs }
func (r *testRegistry) SpecByName(name string) (agent.Spec, bool) { return agent.Spec{}, false }

func TestNewAgentApp(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &testRegistry{
		specs: []agent.Spec{{Name: "TestAgent"}},
	}
	dir := t.TempDir()
	app := NewAgentApp(env, reg, nil, nil, nil, dir, 0)
	if app == nil {
		t.Fatal("expected non-nil AgentApp")
	}
	if app.GetEngine() == nil {
		t.Error("expected non-nil engine")
	}
}

func TestAgentApp_GetEngine(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &testRegistry{}
	app := NewAgentApp(env, reg, nil, nil, nil, t.TempDir(), 0)
	engine := app.GetEngine()
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	// Test the engine handles requests
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from health endpoint, got %d", w.Code)
	}
}

func TestAgentApp_Shutdown_NilServer(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &testRegistry{}
	app := NewAgentApp(env, reg, nil, nil, nil, t.TempDir(), 0)
	// Shutdown without Start should not error
	err := app.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentApp_OpsRecorderIntegration(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &testRegistry{}
	app := NewAgentApp(env, reg, nil, nil, nil, t.TempDir(), 0)
	// OpsRecorder should be set
	if app.Env.OpsRecorder == nil {
		t.Error("expected OpsRecorder to be set")
	}
}

func TestNewAgentApp_WithMCPReg(t *testing.T) {
	env := runtime.NewEnvironment(nil)
	reg := &testRegistry{}
	app := NewAgentApp(env, reg, nil, nil, nil, t.TempDir(), 5)
	if app.ReportMaxReports != 5 {
		t.Errorf("expected ReportMaxReports=5, got %d", app.ReportMaxReports)
	}
}
