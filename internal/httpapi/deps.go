package httpapi

import (
	"time"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	mcp "github.com/alanfokco/kube-ops-agent-go/internal/mcp"
	"github.com/alanfokco/kube-ops-agent-go/internal/operations"
	"github.com/alanfokco/kube-ops-agent-go/internal/report"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
	"github.com/alanfokco/kube-ops-agent-go/internal/scheduler"
)

// Deps aggregates HTTP API dependencies.
type Deps struct {
	Env              *runtime.Environment
	Registry         agent.Registry
	Executor         *agent.Executor
	ReportMgr        *report.Manager
	OpsMgr           *operations.Manager
	MCPReg           *mcp.Registry
	Scheduler        *scheduler.Scheduler
	ReportDir        string
	ReportMaxReports int // 0=unlimited
	StartTime        time.Time // server start time for uptime
}
