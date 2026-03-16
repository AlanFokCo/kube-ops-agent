package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	"github.com/alanfokco/kube-ops-agent-go/internal/operations"
	"github.com/alanfokco/kube-ops-agent-go/internal/report"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
	"github.com/alanfokco/kube-ops-agent-go/version"
)

// RegisterRoutes registers HTTP routes in Python main.py interface style.
func RegisterRoutes(engine *gin.Engine, env *runtime.Environment, reg agent.Registry, exec *agent.Executor, reportDir string, maxReports int) {
	d := &Deps{
		Env:              env,
		Registry:         reg,
		Executor:         exec,
		ReportMgr:        report.NewManager(reportDir, maxReports),
		OpsMgr:           nil,
		MCPReg:           nil,
		Scheduler:        nil,
		ReportDir:        reportDir,
		ReportMaxReports: maxReports,
	}
	RegisterRoutesWithDeps(engine, d)
}

// RegisterRoutesWithDeps registers routes with full deps.
func RegisterRoutesWithDeps(engine *gin.Engine, d *Deps) {
	// Health & Readiness
	engine.GET("/health", func(c *gin.Context) { handleHealth(c, d) })
	engine.GET("/ready", func(c *gin.Context) { handleReady(c, d) })
	engine.GET("/metrics", func(c *gin.Context) { handleMetrics(c, d) })

	// Trigger & Chat
	engine.POST("/trigger", func(c *gin.Context) { handleTrigger(c, d) })
	engine.POST("/chat", func(c *gin.Context) {
		if c.GetHeader("Accept") == "text/event-stream" {
			handleChatStream(c, d.Executor)
		} else {
			handleChat(c, d.Executor)
		}
	})

	// MCP
	engine.GET("/mcp-tools", func(c *gin.Context) { handleMCPTools(c, d) })
	engine.POST("/mcp-tools", func(c *gin.Context) { handleMCPTools(c, d) })

	// Reports
	rm := d.ReportMgr
	if rm == nil {
		rm = report.NewManager(d.ReportDir, d.ReportMaxReports)
	}
	engine.GET("/api/reports", func(c *gin.Context) { handleReportsList(c, rm) })
	engine.POST("/api/reports", func(c *gin.Context) { handleReportsList(c, rm) })
	engine.GET("/api/report", func(c *gin.Context) { handleReportDetail(c, rm) })
	engine.POST("/api/report", func(c *gin.Context) { handleReportDetail(c, rm) })

	// Operations
	engine.GET("/api/operations", func(c *gin.Context) { handleOperations(c, d) })
	engine.POST("/api/operations", func(c *gin.Context) { handleOperations(c, d) })

	// System status
	engine.GET("/api/system", func(c *gin.Context) { handleSystem(c, d) })
}

func handleHealth(c *gin.Context, d *Deps) {
	specs := d.Registry.Specs()
	circuitStatus := d.Env.Circuit.GetStatus()
	openList, _ := circuitStatus["open_circuits"].([]string)
	openCount := len(openList)

	status := "ok"
	if d.Env.Graceful != nil && d.Env.Graceful.IsShuttingDown() {
		status = "shutting_down"
	} else if openCount > len(specs)/2 {
		status = "degraded"
	}

	now := time.Now()
	agentStatus := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		var lastRun *time.Time
		if d.Env.State != nil {
			lastRun = d.Env.State.GetAgentLastRun(s.Name)
		}
		nextIn := 0.0
		if lastRun != nil && s.IntervalSecond > 0 {
			elapsed := now.Sub(*lastRun).Seconds()
			if remaining := float64(s.IntervalSecond) - elapsed; remaining > 0 {
				nextIn = remaining
			}
		}
		lastRunAt := 0.0
		if lastRun != nil {
			lastRunAt = float64(lastRun.Unix())
		}
		agentStatus = append(agentStatus, map[string]any{
			"agent":            s.Name,
			"interval_seconds": s.IntervalSecond,
			"last_run_at":      lastRunAt,
			"next_run_in":      nextIn,
		})
	}

	resp := map[string]any{
		"status":         status,
		"service":        "K8sOpsMultiAgent",
		"version":        version.Version,
		"report_dir":     d.ReportDir,
		"active_agents":  d.Env.Concurrency.ActiveAgentCount(),
		"circuit_breaker": circuitStatus,
		"agents":         agentStatus,
	}
	if d.Scheduler != nil {
		resp["scheduler"] = d.Scheduler.GetStatus()
	}
	c.JSON(http.StatusOK, resp)
}

func handleReady(c *gin.Context, d *Deps) {
	if d.Env.Graceful != nil && d.Env.Graceful.IsShuttingDown() {
		c.JSON(http.StatusOK, map[string]any{"status": "not_ready", "reason": "shutting down"})
		return
	}
	if d.Scheduler == nil {
		c.JSON(http.StatusOK, map[string]any{"status": "not_ready", "reason": "scheduler not initialized"})
		return
	}
	c.JSON(http.StatusOK, map[string]any{"status": "ready"})
}

func handleMetrics(c *gin.Context, d *Deps) {
	metrics := map[string]any{"active_agents": d.Env.Concurrency.ActiveAgentCount()}
	if d.Env.Metrics != nil {
		metrics = d.Env.Metrics.GetMetrics()
	}
	resp := map[string]any{
		"metrics":        metrics,
		"circuit_breaker": d.Env.Circuit.GetStatus(),
		"concurrency": map[string]any{
			"active_agents":       d.Env.Concurrency.ActiveAgentCount(),
			"active_agent_names":  d.Env.Concurrency.ActiveAgents(),
		},
		"timestamp": time.Now().Unix(),
	}
	c.JSON(http.StatusOK, resp)
}

func handleTrigger(c *gin.Context, d *Deps) {
	specs := d.Registry.Specs()
	if len(specs) == 0 {
		c.JSON(http.StatusOK, map[string]any{"status": "error", "message": "No agents in registry"})
		return
	}

	var agentNamesFilter []string
	if c.GetHeader("Content-Type") == "application/json" {
		var body struct {
			Input []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			for _, msg := range body.Input {
				for _, block := range msg.Content {
					if block.Type == "text" && strings.Contains(block.Text, "agent_names:") {
						parts := strings.SplitN(block.Text, "agent_names:", 2)
						if len(parts) == 2 {
							names := strings.Split(parts[1], ",")
							for _, n := range names {
								n = strings.TrimSpace(n)
								if n != "" {
									agentNamesFilter = append(agentNamesFilter, n)
								}
							}
						}
					}
				}
			}
		}
	}

	filtered := specs
	if len(agentNamesFilter) > 0 {
		nameSet := make(map[string]bool)
		for _, n := range agentNamesFilter {
			nameSet[n] = true
		}
		filtered = nil
		for _, s := range specs {
			if nameSet[s.Name] {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			c.JSON(http.StatusOK, map[string]any{"status": "error", "message": "No matching agents found"})
			return
		}
	}

	if d.Scheduler != nil {
		go d.Scheduler.RunOneRound(filtered)
		c.JSON(http.StatusOK, map[string]any{
			"status":  "triggered",
			"message": "Triggered " + strconv.Itoa(len(filtered)) + " agents for concurrent execution",
			"agents":  agentNames(filtered),
		})
		return
	}

	ctx := c.Request.Context()
	results := make([]map[string]any, 0, len(filtered))
	for _, s := range filtered {
		_, err := d.Executor.Execute(ctx, s.Name, map[string]any{"trigger": "manual_http"})
		ar := map[string]any{"name": s.Name}
		if err != nil {
			ar["error"] = err.Error()
		}
		results = append(results, ar)
	}
	c.JSON(http.StatusOK, map[string]any{"success": true, "agents": results})
}

func agentNames(specs []agent.Spec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

func handleChat(c *gin.Context, exec *agent.Executor) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	defer c.Request.Body.Close()

	question := string(data)
	if text, ok := ParseAgentRequest(io.NopCloser(strings.NewReader(question))); ok && text != "" {
		question = text
	}

	ctx := c.Request.Context()
	msg, err := exec.ExecuteChat(ctx, question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "error": err.Error()})
		return
	}
	txt := ""
	if msg != nil {
		if p := msg.GetTextContent(""); p != nil {
			txt = *p
		}
	}
	c.JSON(http.StatusOK, map[string]any{"success": true, "answer": txt})
}

func handleChatStream(c *gin.Context, exec *agent.Executor) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	defer c.Request.Body.Close()

	question := string(data)
	if text, ok := ParseAgentRequest(io.NopCloser(strings.NewReader(question))); ok && text != "" {
		question = text
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	ctx := c.Request.Context()
	_, err = exec.ExecuteChatStream(ctx, question, func(chunk string) error {
		if chunk == "" {
			return nil
		}
		payload, _ := json.Marshal(map[string]string{"answer": chunk})
		_, _ = c.Writer.Write(append(append([]byte("data: "), payload...), '\n', '\n'))
		c.Writer.Flush()
		return nil
	})
	if err != nil {
		payload, _ := json.Marshal(map[string]string{"error": err.Error()})
		_, _ = c.Writer.Write(append(append([]byte("data: "), payload...), '\n', '\n'))
		c.Writer.Flush()
	}
}

func handleMCPTools(c *gin.Context, d *Deps) {
	if d.MCPReg == nil || !d.MCPReg.IsInitialized() {
		c.JSON(http.StatusOK, map[string]any{
			"status":  "not_initialized",
			"message": "MCP registry not initialized",
			"servers": []any{},
			"tools":   []any{},
		})
		return
	}

	tools := d.MCPReg.ListTools()
	toolList := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		toolList = append(toolList, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"server":      t.ServerName,
			"input_schema": t.InputSchema,
		})
	}
	c.JSON(http.StatusOK, map[string]any{
		"status":     "ok",
		"servers":    d.MCPReg.ConnectedServers(),
		"tool_count": d.MCPReg.ToolCount(),
		"tools":      toolList,
	})
}

func handleReportsList(c *gin.Context, rm *report.Manager) {
	limit, offset := 50, 0
	var startPtr, endPtr *time.Time

	q := c.Request.URL.Query()
	limit, _ = strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ = strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	if v := q.Get("start_time"); v != "" {
		if ts, err := strconv.ParseFloat(v, 64); err == nil {
			t := time.Unix(int64(ts), 0)
			startPtr = &t
		}
	}
	if v := q.Get("end_time"); v != "" {
		if ts, err := strconv.ParseFloat(v, 64); err == nil {
			t := time.Unix(int64(ts), 0)
			endPtr = &t
		}
	}
	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") == "application/json" {
		if text, ok := ParseAgentRequest(c.Request.Body); ok {
			params := ParseParamsFromText(text)
			if n := GetInt(params, "limit", limit); n > 0 {
				limit = n
			}
			if n := GetInt(params, "offset", offset); n >= 0 {
				offset = n
			}
			if f := GetFloat(params, "start_time", 0); f > 0 {
				t := time.Unix(int64(f), 0)
				startPtr = &t
			}
			if f := GetFloat(params, "end_time", 0); f > 0 {
				t := time.Unix(int64(f), 0)
				endPtr = &t
			}
		}
	}

	items, total, err := rm.List(limit, offset, startPtr, endPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"reports": items,
	})
}

func handleReportDetail(c *gin.Context, rm *report.Manager) {
	if c.Request.Method == http.MethodPost {
		var bodyBytes []byte
		if c.GetHeader("Content-Type") == "application/json" {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			var probe struct {
				Content string `json:"content"`
			}
			_ = json.Unmarshal(bodyBytes, &probe)
			if probe.Content != "" {
				c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
				handleReportPOST(c, rm)
				return
			}
			if text, ok := ParseAgentRequest(io.NopCloser(strings.NewReader(string(bodyBytes)))); ok {
				params := ParseParamsFromText(text)
				id := GetString(params, "id")
				latest := strings.ToLower(GetString(params, "latest")) == "true"
				if latest || id != "" {
					var item *report.Item
					var body string
					var err error
					if latest {
						item, body, err = rm.Latest()
					} else {
						item, body, err = rm.Get(id)
					}
					if err == nil && item != nil {
						c.JSON(http.StatusOK, map[string]any{
							"success": true,
							"report":  map[string]any{"meta": item, "content": body, "format": "markdown"},
						})
						return
					}
					c.JSON(http.StatusNotFound, map[string]any{"success": false, "error": err.Error()})
					return
				}
			}
		}
		if len(bodyBytes) > 0 {
			c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		}
		handleReportPOST(c, rm)
		return
	}
	if c.Request.Method != http.MethodGet {
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return
	}
	q := c.Request.URL.Query()
	id := q.Get("id")
	latest := q.Get("latest") == "true"
	reportFormat := q.Get("format")

	var item *report.Item
	var body string
	var err error

	switch {
	case latest:
		item, body, err = rm.Latest()
	case id != "":
		item, body, err = rm.Get(id)
	default:
		c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "missing id or latest=true"})
		return
	}
	if err != nil {
		c.JSON(http.StatusNotFound, map[string]any{"success": false, "error": err.Error()})
		return
	}
	switch reportFormat {
	case "markdown":
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.String(http.StatusOK, body)
	default:
		c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"report":  map[string]any{"meta": item, "content": body, "format": "markdown"},
		})
	}
}

func handleReportPOST(c *gin.Context, rm *report.Manager) {
	if c.GetHeader("Content-Type") != "application/json" {
		c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "Content-Type must be application/json"})
		return
	}
	var body struct {
		Input []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "invalid json: " + err.Error()})
		return
	}
	content := body.Content
	if content == "" && len(body.Input) > 0 {
		for _, msg := range body.Input {
			for _, block := range msg.Content {
				if block.Type == "text" && block.Text != "" {
					content = block.Text
					break
				}
			}
			if content != "" {
				break
			}
		}
	}
	if content == "" {
		c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "missing content or input"})
		return
	}
	item, err := rm.Save(content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"report":  map[string]any{"meta": item, "content": content},
	})
}

func handleOperations(c *gin.Context, d *Deps) {
	if d.OpsMgr == nil {
		c.JSON(http.StatusOK, map[string]any{"success": false, "error": "Operation manager not initialized"})
		return
	}

	queryType := "summary"
	agentName := ""
	limit := 100
	offset := 0
	var successOnly *bool

	q := c.Request.URL.Query()
	queryType = q.Get("type")
	if queryType == "" {
		queryType = "summary"
	}
	agentName = q.Get("agent_name")
	if l, _ := strconv.Atoi(q.Get("limit")); l > 0 {
		limit = l
	}
	if o, _ := strconv.Atoi(q.Get("offset")); o >= 0 {
		offset = o
	}
	if v := q.Get("success_only"); v == "true" {
		t := true
		successOnly = &t
	} else if v == "false" {
		t := false
		successOnly = &t
	}
	var startTime, endTime *time.Time
	if v := q.Get("start_time"); v != "" {
		if ts, err := strconv.ParseFloat(v, 64); err == nil {
			t := time.Unix(int64(ts), 0)
			startTime = &t
		}
	}
	if v := q.Get("end_time"); v != "" {
		if ts, err := strconv.ParseFloat(v, 64); err == nil {
			t := time.Unix(int64(ts), 0)
			endTime = &t
		}
	}

	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") == "application/json" {
		var body struct {
			Input []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			for _, msg := range body.Input {
				for _, block := range msg.Content {
					if block.Type != "text" {
						continue
					}
					for _, part := range strings.Split(block.Text, ",") {
						part = strings.TrimSpace(part)
						if strings.HasPrefix(part, "type:") {
							queryType = strings.TrimSpace(strings.TrimPrefix(part, "type:"))
						} else if strings.HasPrefix(part, "agent_name:") {
							agentName = strings.TrimSpace(strings.TrimPrefix(part, "agent_name:"))
						} else if strings.HasPrefix(part, "limit:") {
							if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "limit:"))); err == nil && n > 0 {
								limit = n
							}
						} else if strings.HasPrefix(part, "offset:") {
							if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "offset:"))); err == nil && n >= 0 {
								offset = n
							}
						} else if strings.HasPrefix(part, "success_only:") {
							v := strings.TrimSpace(strings.TrimPrefix(part, "success_only:"))
							if v == "true" {
								t := true
								successOnly = &t
							} else if v == "false" {
								t := false
								successOnly = &t
							}
						} else if strings.HasPrefix(part, "start_time:") {
							if ts, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(part, "start_time:")), 64); err == nil {
								t := time.Unix(int64(ts), 0)
								startTime = &t
							}
						} else if strings.HasPrefix(part, "end_time:") {
							if ts, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(part, "end_time:")), 64); err == nil {
								t := time.Unix(int64(ts), 0)
								endTime = &t
							}
						}
					}
				}
			}
		}
	}

	if queryType == "history" {
		records, total := d.OpsMgr.GetHistory(agentName, limit, offset, successOnly, startTime, endTime)
		c.JSON(http.StatusOK, map[string]any{
			"success":    true,
			"type":       "history",
			"total":      total,
			"limit":      limit,
			"offset":     offset,
			"executions": records,
		})
		return
	}

	enrich := &operations.EnrichmentSources{
		Metrics: d.Env.Metrics,
		State:   d.Env.State,
		Circuit: d.Env.Circuit,
	}
	summaries := d.OpsMgr.GetSummaryWithEnrichment(enrich)
	c.JSON(http.StatusOK, map[string]any{
		"success":      true,
		"type":         "summary",
		"agents":       summaries,
		"total_agents": len(summaries),
	})
}

func handleSystem(c *gin.Context, d *Deps) {
	resp := map[string]any{
		"success":   true,
		"version":   version.Version,
		"timestamp": time.Now().Unix(),
	}
	if !d.StartTime.IsZero() {
		resp["uptime"] = time.Since(d.StartTime).Seconds()
	}
	if d.Env.Metrics != nil {
		resp["metrics"] = d.Env.Metrics.GetMetrics()
	}
	specs := d.Registry.Specs()
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}
	resp["agents"] = map[string]any{"total": len(specs), "names": names}
	if d.MCPReg != nil {
		resp["mcp"] = map[string]any{
			"initialized": d.MCPReg.IsInitialized(),
			"servers":     d.MCPReg.ConnectedServers(),
			"tool_count":  d.MCPReg.ToolCount(),
		}
	}
	if d.ReportMgr != nil {
		items, total, _ := d.ReportMgr.List(5, 0, nil, nil)
		recent := make([]map[string]any, 0, len(items))
		for _, it := range items {
			recent = append(recent, map[string]any{"id": it.ID, "created_at": it.CreatedAt, "level": it.Level})
		}
		resp["reports"] = map[string]any{"total": total, "recent": recent}
	}
	if d.Env.Circuit != nil {
		resp["circuit_breaker"] = d.Env.Circuit.GetStatus()
	}
	c.JSON(http.StatusOK, resp)
}
