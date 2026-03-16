package app

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	"github.com/alanfokco/kube-ops-agent-go/internal/httpapi"
	mcp "github.com/alanfokco/kube-ops-agent-go/internal/mcp"
	"github.com/alanfokco/kube-ops-agent-go/internal/operations"
	"github.com/alanfokco/kube-ops-agent-go/internal/report"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
	"github.com/alanfokco/kube-ops-agent-go/internal/scheduler"
)

// AgentApp is app abstraction layer, aggregates runtime and HTTP routes.
type AgentApp struct {
	Env              *runtime.Environment
	Registry         agent.Registry
	Executor         *agent.Executor
	MCPReg           *mcp.Registry
	Scheduler        *scheduler.Scheduler
	ReportDir        string
	ReportMaxReports int // 0=unlimited
	engine           *gin.Engine
	server           *http.Server
}

// NewAgentApp creates a new app instance.
func NewAgentApp(
	env *runtime.Environment,
	reg agent.Registry,
	exec *agent.Executor,
	mcpReg *mcp.Registry,
	sched *scheduler.Scheduler,
	reportDir string,
	reportMaxReports int,
) *AgentApp {
	opsMgr := operations.NewManager()
	env.OpsRecorder = opsMgr
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	app := &AgentApp{
		Env:              env,
		Registry:         reg,
		Executor:         exec,
		MCPReg:           mcpReg,
		Scheduler:        sched,
		ReportDir:        reportDir,
		ReportMaxReports: reportMaxReports,
		engine:           engine,
	}
	app.registerRoutes()
	return app
}

// registerRoutes registers all HTTP routes.
func (a *AgentApp) registerRoutes() {
	var opsMgr *operations.Manager
	if r, ok := a.Env.OpsRecorder.(*operations.Manager); ok {
		opsMgr = r
	}
	deps := &httpapi.Deps{
		Env:              a.Env,
		Registry:         a.Registry,
		Executor:         a.Executor,
		ReportMgr:        report.NewManager(a.ReportDir, a.ReportMaxReports),
		OpsMgr:           opsMgr,
		MCPReg:           a.MCPReg,
		Scheduler:        a.Scheduler,
		ReportDir:        a.ReportDir,
		ReportMaxReports: a.ReportMaxReports,
	}
	httpapi.RegisterRoutesWithDeps(a.engine, deps)
}

// Start starts HTTP server.
func (a *AgentApp) Start(addr string) error {
	a.server = &http.Server{
		Addr:    addr,
		Handler: a.engine,
	}
	return a.server.ListenAndServe()
}

// Shutdown gracefully shuts down HTTP server.
func (a *AgentApp) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// GetEngine returns internal Gin Engine (for testing or extension).
func (a *AgentApp) GetEngine() *gin.Engine {
	return a.engine
}
