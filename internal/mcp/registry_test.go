package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.servers == nil {
		t.Error("expected non-nil servers map")
	}
	if r.tools == nil {
		t.Error("expected non-nil tools map")
	}
}

func TestRegistry_IsInitialized_Empty(t *testing.T) {
	r := NewRegistry()
	if r.IsInitialized() {
		t.Error("expected IsInitialized=false for empty registry")
	}
}

func TestRegistry_ListServers_Empty(t *testing.T) {
	r := NewRegistry()
	servers := r.ListServers()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestRegistry_ListTools_Empty(t *testing.T) {
	r := NewRegistry()
	tools := r.ListTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestRegistry_ToolCount_Empty(t *testing.T) {
	r := NewRegistry()
	if r.ToolCount() != 0 {
		t.Errorf("expected 0 tool count, got %d", r.ToolCount())
	}
}

func TestRegistry_GetTool_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.GetTool("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent tool")
	}
}

func TestRegistry_ConnectedServers_Empty(t *testing.T) {
	r := NewRegistry()
	connected := r.ConnectedServers()
	if len(connected) != 0 {
		t.Errorf("expected 0 connected servers, got %d", len(connected))
	}
}

func TestInitFromConfig_NotExist(t *testing.T) {
	_, err := InitFromConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent config file")
	}
}

func TestInitFromConfig_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mcp*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("")
	f.Close()

	r, err := InitFromConfig(f.Name())
	if err != nil {
		t.Fatalf("InitFromConfig: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil registry for empty config")
	}
}

func TestInitFromConfig_StaticTools(t *testing.T) {
	yaml := `
servers:
  - name: k8s-server
    description: Kubernetes MCP server
    tools:
      - name: get_pods
        description: Get pods in namespace
        input_schema:
          namespace:
            type: string
      - name: get_nodes
        description: Get cluster nodes
`
	f, err := os.CreateTemp(t.TempDir(), "mcp*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(yaml)
	f.Close()

	r, err := InitFromConfig(f.Name())
	if err != nil {
		t.Fatalf("InitFromConfig: %v", err)
	}

	if !r.IsInitialized() {
		t.Error("expected IsInitialized=true after loading")
	}

	servers := r.ListServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "k8s-server" {
		t.Errorf("server name = %q", servers[0].Name)
	}

	tools := r.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	if r.ToolCount() != 2 {
		t.Errorf("ToolCount = %d, want 2", r.ToolCount())
	}

	tool, ok := r.GetTool("get_pods")
	if !ok {
		t.Fatal("expected get_pods tool")
	}
	if tool.Name != "get_pods" {
		t.Errorf("tool.Name = %q", tool.Name)
	}
	if tool.ServerName != "k8s-server" {
		t.Errorf("tool.ServerName = %q", tool.ServerName)
	}
}

func TestTryInitFromConfig_NotExist(t *testing.T) {
	r, err := TryInitFromConfig("/nonexistent/mcp.yaml")
	if err != nil {
		t.Errorf("expected nil error for non-existent file, got %v", err)
	}
	if r != nil {
		t.Error("expected nil registry for non-existent file")
	}
}

func TestTryInitFromConfig_Exists(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "mcp*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("")
	f.Close()

	r, err := TryInitFromConfig(f.Name())
	if err != nil {
		t.Fatalf("TryInitFromConfig: %v", err)
	}
	if r == nil {
		t.Error("expected non-nil registry for existing file")
	}
}

func TestInitFromConfig_PythonFormat(t *testing.T) {
	// Python format with command/args
	yaml := `
servers:
  - name: filesystem
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - /tmp
    transport: stdio
    enabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	r, err := InitFromConfig(path)
	if err != nil {
		t.Fatalf("InitFromConfig (Python format): %v", err)
	}
	// Tools might be 0 since Python format needs runtime, but should parse without error
	_ = r
}

// ---- mcp/client.go testable functions ----

func TestToolResult_GetText_Empty(t *testing.T) {
r := &ToolResult{}
text := r.GetText()
if text != "" {
t.Errorf("expected empty text, got %q", text)
}
}

func TestToolResult_GetText_TextOnly(t *testing.T) {
r := &ToolResult{
Content: []map[string]any{
{"type": "text", "text": "hello world"},
},
}
text := r.GetText()
if text != "hello world" {
t.Errorf("expected 'hello world', got %q", text)
}
}

func TestToolResult_GetText_MultipleItems(t *testing.T) {
r := &ToolResult{
Content: []map[string]any{
{"type": "text", "text": "line1"},
{"type": "image", "data": "base64data"}, // non-text
{"type": "text", "text": "line2"},
},
}
text := r.GetText()
if !strings.Contains(text, "line1") || !strings.Contains(text, "line2") {
t.Errorf("expected both lines, got %q", text)
}
if strings.Contains(text, "base64data") {
t.Error("image content should be excluded")
}
}

func TestToolResult_GetText_NonStringText(t *testing.T) {
r := &ToolResult{
Content: []map[string]any{
{"type": "text", "text": 42}, // non-string text
},
}
text := r.GetText()
if text != "" {
t.Errorf("non-string text should be ignored, got %q", text)
}
}

func TestToolResult_IsError(t *testing.T) {
r := &ToolResult{IsError: true}
if !r.IsError {
t.Error("expected IsError=true")
}
}

// Test helper functions
func TestGetStr(t *testing.T) {
m := map[string]any{"key": "value", "num": 42}
if got := getStr(m, "key"); got != "value" {
t.Errorf("expected 'value', got %q", got)
}
if got := getStr(m, "num"); got != "" {
t.Errorf("expected empty for non-string, got %q", got)
}
if got := getStr(m, "missing"); got != "" {
t.Errorf("expected empty for missing key, got %q", got)
}
}

func TestGetMap(t *testing.T) {
inner := map[string]any{"k": "v"}
m := map[string]any{"sub": inner, "str": "hello"}
result := getMap(m, "sub")
if result == nil {
t.Error("expected non-nil map")
}
if result["k"] != "v" {
t.Errorf("expected 'v', got %v", result["k"])
}
if got := getMap(m, "str"); got != nil {
t.Errorf("expected nil for non-map, got %v", got)
}
if got := getMap(m, "missing"); got != nil {
t.Errorf("expected nil for missing key, got %v", got)
}
}

func TestNewRuntime(t *testing.T) {
r := NewRuntime()
if r == nil {
t.Fatal("expected non-nil Runtime")
}
}

func TestNewStdioClient(t *testing.T) {
cfg := ServerConfig{
Name:    "test",
Command: "echo",
}
c := NewStdioClient(cfg)
if c == nil {
t.Fatal("expected non-nil StdioClient")
}
if len(c.Tools()) != 0 {
t.Error("expected empty tools initially")
}
}

func TestNewHTTPClient(t *testing.T) {
cfg := ServerConfig{
Name: "test",
URL:  "http://localhost:9999",
}
c := NewHTTPClient(cfg)
if c == nil {
t.Fatal("expected non-nil HTTPClient")
}
if len(c.Tools()) != 0 {
t.Error("expected empty tools initially")
}
}

// ---- Runtime tests ----

func TestRuntime_Initialize_EmptyConfig(t *testing.T) {
r := NewRuntime()
err := r.Initialize(nil)
if err != nil {
t.Errorf("Initialize with nil config should not error: %v", err)
}
}

func TestRuntime_Initialize_DisabledServers(t *testing.T) {
r := NewRuntime()
configs := []ServerConfig{
{Name: "disabled", Enabled: false, Command: "echo"},
{Name: "no-cmd", Enabled: true},
}
err := r.Initialize(configs)
if err != nil {
t.Errorf("Initialize should not error: %v", err)
}
if len(r.clients) != 0 {
t.Errorf("expected no connected clients, got %d", len(r.clients))
}
}

func TestRuntime_Initialize_ConnectFail(t *testing.T) {
r := NewRuntime()
// Command that will fail to connect (non-existent program)
configs := []ServerConfig{
{Name: "fail", Enabled: true, Command: "/nonexistent/binary/that/does/not/exist"},
}
err := r.Initialize(configs)
// Should not return error - failed servers are skipped
if err != nil {
t.Errorf("Initialize should skip failed servers without error: %v", err)
}
}

func TestRuntime_Shutdown_Empty(t *testing.T) {
r := NewRuntime()
// Shutdown on empty runtime should not panic
r.Shutdown()
}

func TestRuntime_CallTool_NotFound(t *testing.T) {
r := NewRuntime()
_, err := r.CallTool(context.Background(), "nonexistent-tool", nil)
if err == nil {
t.Error("expected error for unknown tool")
}
if !strings.Contains(err.Error(), "tool not found") {
t.Errorf("expected 'tool not found' error, got: %v", err)
}
}

func TestRuntime_GetTools_Empty(t *testing.T) {
r := NewRuntime()
tools := r.GetTools()
if tools == nil {
// Can be nil or empty slice
}
if len(tools) != 0 {
t.Errorf("expected no tools, got %d", len(tools))
}
}

// ---- StdioClient and HTTPClient close tests ----

func TestStdioClient_Close_Unconnected(t *testing.T) {
c := NewStdioClient(ServerConfig{Name: "test", Command: "echo"})
// Close without connecting should be a no-op
if err := c.Close(); err != nil {
t.Errorf("Close on unconnected client: %v", err)
}
}

func TestHTTPClient_Close(t *testing.T) {
c := NewHTTPClient(ServerConfig{Name: "test", URL: "http://localhost:9999"})
if err := c.Close(); err != nil {
t.Errorf("Close should not error: %v", err)
}
}

// ---- Registry function tests ----

func TestRegistry_ShutdownMCP_NoRuntime(t *testing.T) {
r := NewRegistry()
// ShutdownMCP with no runtime should be a no-op
r.ShutdownMCP()
}

func TestRegistry_CreateMCPToolFunction_Nil(t *testing.T) {
r := NewRegistry()
result := r.CreateMCPToolFunction(nil)
if result != nil {
t.Error("expected nil for nil tool")
}
}

func TestRegistry_CreateMCPToolFunction_NoRuntime(t *testing.T) {
r := NewRegistry()
tool := &Tool{Name: "test", Description: "test tool"}
result := r.CreateMCPToolFunction(tool)
if result == nil {
t.Fatal("expected non-nil result")
}
if result.Name != "test" {
t.Errorf("expected name 'test', got %q", result.Name)
}
// Execute should return error since runtime is not initialized
_, err := result.Execute(context.Background(), nil)
if err == nil {
t.Error("expected error when runtime not initialized")
}
}

func TestCreateMCPToolFunctionStatic_Nil(t *testing.T) {
result := CreateMCPToolFunction(nil)
if result != nil {
t.Error("expected nil for nil tool")
}
}

func TestCreateMCPToolFunctionStatic(t *testing.T) {
tool := &Tool{Name: "static-tool", Description: "static"}
result := CreateMCPToolFunction(tool)
if result == nil {
t.Fatal("expected non-nil result")
}
if result.Name != "static-tool" {
t.Errorf("expected 'static-tool', got %q", result.Name)
}
// Execute should return error since runtime is static (no runtime)
_, err := result.Execute(context.Background(), nil)
if err == nil {
t.Error("expected error for static tool")
}
}

func TestRegistry_MCPToolsAsAgentTools_Empty(t *testing.T) {
r := NewRegistry()
tools := r.MCPToolsAsAgentTools()
if len(tools) != 0 {
t.Errorf("expected no tools, got %d", len(tools))
}
}

func TestRegistry_MCPToolsAsAgentTools_WithTools(t *testing.T) {
r := NewRegistry()
// Add a tool manually
r.tools["t1"] = &Tool{Name: "t1", Description: "tool1", ServerName: "s1"}
tools := r.MCPToolsAsAgentTools()
if len(tools) != 1 {
t.Fatalf("expected 1 tool, got %d", len(tools))
}
}

func TestRegistry_ListServers_EmptyNew(t *testing.T) {
r := NewRegistry()
servers := r.ListServers()
if len(servers) != 0 {
t.Errorf("expected no servers, got %d", len(servers))
}
}

func TestLoadServerConfigs_NotExist(t *testing.T) {
configs := loadServerConfigs("/nonexistent/path.yaml")
if len(configs) != 0 {
t.Errorf("expected empty configs for missing file, got %d", len(configs))
}
}

func TestLoadServerConfigs_ValidFile(t *testing.T) {
content := `
servers:
  - name: test-server
    enabled: true
    command: echo
    args: ["hello"]
  - name: disabled-server
    enabled: false
    command: cat
  - name: no-cmd-server
    enabled: true
`
dir := t.TempDir()
path := filepath.Join(dir, "mcp.yaml")
os.WriteFile(path, []byte(content), 0644)
configs := loadServerConfigs(path)
if len(configs) != 1 {
t.Errorf("expected 1 enabled+cmd server, got %d", len(configs))
}
}

func TestInitializeMCP_NotExist(t *testing.T) {
_, err := InitializeMCP("/nonexistent/path.yaml")
if err == nil {
t.Error("expected error for non-existent config file")
}
}

// ---- StdioClient mkRequest (private method, accessible in same package) ----

func TestStdioClient_mkRequest(t *testing.T) {
c := NewStdioClient(ServerConfig{Name: "test", Command: "echo"})
req := c.mkRequest("test-method", map[string]any{"key": "value"})
if req["jsonrpc"] != "2.0" {
t.Errorf("expected jsonrpc=2.0, got %v", req["jsonrpc"])
}
if req["method"] != "test-method" {
t.Errorf("expected method=test-method, got %v", req["method"])
}
if req["id"] != 1 {
t.Errorf("expected id=1, got %v", req["id"])
}
if req["params"] == nil {
t.Error("expected params to be set")
}
}

func TestStdioClient_mkRequest_NilParams(t *testing.T) {
c := NewStdioClient(ServerConfig{Name: "test", Command: "echo"})
req := c.mkRequest("initialize", nil)
if _, ok := req["params"]; ok {
t.Error("expected no params when params is nil")
}
}

func TestStdioClient_mkRequest_IncrementID(t *testing.T) {
c := NewStdioClient(ServerConfig{Name: "test", Command: "echo"})
r1 := c.mkRequest("method1", nil)
r2 := c.mkRequest("method2", nil)
id1, _ := r1["id"].(int)
id2, _ := r2["id"].(int)
if id2 != id1+1 {
t.Errorf("expected id2=%d, got id1=%d id2=%d", id1+1, id1, id2)
}
}

// ---- HTTPClient mkRequest ----

func TestHTTPClient_mkRequest(t *testing.T) {
c := NewHTTPClient(ServerConfig{Name: "test", URL: "http://localhost:9999"})
req := c.mkRequest("tools/list", nil)
if req["jsonrpc"] != "2.0" {
t.Errorf("expected jsonrpc=2.0, got %v", req["jsonrpc"])
}
if req["method"] != "tools/list" {
t.Errorf("expected tools/list method, got %v", req["method"])
}
}

func TestHTTPClient_mkRequest_WithParams(t *testing.T) {
c := NewHTTPClient(ServerConfig{Name: "test", URL: "http://localhost:9999"})
params := map[string]any{"name": "test-tool", "arguments": map[string]any{}}
req := c.mkRequest("tools/call", params)
if req["params"] == nil {
t.Error("expected params to be set")
}
}

// ---- Runtime Shutdown with clients ----

func TestRuntime_Shutdown_WithHTTPClient(t *testing.T) {
r := NewRuntime()
// Add an HTTP client manually (not connected)
r.mu.Lock()
r.clients["test"] = NewHTTPClient(ServerConfig{Name: "test", URL: "http://localhost:9999"})
r.mu.Unlock()
// Shutdown should call Close on all clients
r.Shutdown()
if r.clients != nil {
t.Error("expected clients to be nil after shutdown")
}
}

// ---- Registry.ConnectedServers ----

func TestRegistry_ConnectedServers_WithServers(t *testing.T) {
r := NewRegistry()
r.servers["s1"] = &Server{Name: "s1"}
r.servers["s2"] = &Server{Name: "s2"}
connected := r.ConnectedServers()
if len(connected) != 2 {
t.Errorf("expected 2 servers, got %d", len(connected))
}
}

// ---- Registry.ShutdownMCP with runtime ----

func TestRegistry_ShutdownMCP_WithRuntime(t *testing.T) {
r := NewRegistry()
r.runtime = NewRuntime()
// ShutdownMCP should close the runtime
r.ShutdownMCP()
if r.runtime != nil {
t.Error("expected runtime to be nil after ShutdownMCP")
}
}

// ---- Registry.CreateMCPToolFunction execute with no runtime ----
func TestRegistry_CreateMCPToolFunction_EmptyDescription(t *testing.T) {
r := NewRegistry()
tool := &Tool{Name: "no-desc"}
result := r.CreateMCPToolFunction(tool)
if result == nil {
t.Fatal("expected non-nil result")
}
// Description should default to "MCP tool: no-desc"
if !strings.Contains(result.Description, "no-desc") {
t.Errorf("expected tool name in description, got %q", result.Description)
}
}

// ---- InitFromConfig with static tools config ----

func TestInitFromConfig_StaticTools_v2(t *testing.T) {
content := `
servers:
  - name: k8s-server
    description: K8s MCP server
    tools:
      - name: kubectl-get
        description: Get k8s resources
        input_schema:
          type: object
          properties:
            resource:
              type: string
      - name: kubectl-describe
        description: Describe k8s resources
`
dir := t.TempDir()
path := filepath.Join(dir, "mcp.yaml")
os.WriteFile(path, []byte(content), 0644)

r, err := InitFromConfig(path)
if err != nil {
t.Fatalf("expected no error, got: %v", err)
}
if r == nil {
t.Fatal("expected non-nil registry")
}
if len(r.servers) != 1 {
t.Errorf("expected 1 server, got %d", len(r.servers))
}
if len(r.tools) != 2 {
t.Errorf("expected 2 tools, got %d", len(r.tools))
}
}

func TestInitFromConfig_FormatB(t *testing.T) {
content := `
enabled: true
auto_connect: false
servers:
  - name: kubectl-server
    enabled: true
    command: kubectl-mcp-server
  - name: disabled-server
    enabled: false
    command: disabled-cmd
  - name: no-name-server
    enabled: true
    command: some-cmd
`
dir := t.TempDir()
path := filepath.Join(dir, "mcp.yaml")
os.WriteFile(path, []byte(content), 0644)

r, err := InitFromConfig(path)
if err != nil {
t.Fatalf("expected no error, got: %v", err)
}
if r == nil {
t.Fatal("expected non-nil registry")
}
}

func TestTryInitFromConfig_MissingFile(t *testing.T) {
r, err := TryInitFromConfig("/nonexistent/mcp.yaml")
if err != nil {
t.Errorf("expected nil error for missing file, got: %v", err)
}
if r != nil {
t.Error("expected nil registry for missing file")
}
}

func TestTryInitFromConfig_ValidFile(t *testing.T) {
content := `
servers:
  - name: test-server
    description: Test server
    tools:
      - name: test-tool
        description: A test tool
`
dir := t.TempDir()
path := filepath.Join(dir, "mcp.yaml")
os.WriteFile(path, []byte(content), 0644)

r, err := TryInitFromConfig(path)
if err != nil {
t.Errorf("expected no error, got: %v", err)
}
if r == nil {
t.Fatal("expected non-nil registry")
}
}

// ---- InitializeMCP with a valid static config (no actual MCP runtime needed) ----

func TestInitializeMCP_StaticConfig(t *testing.T) {
content := `
servers:
  - name: test-server
    description: Test server
    tools:
      - name: test-tool
        description: A test tool
`
dir := t.TempDir()
path := filepath.Join(dir, "mcp.yaml")
os.WriteFile(path, []byte(content), 0644)

r, err := InitializeMCP(path)
if err != nil {
t.Fatalf("expected no error, got: %v", err)
}
if r == nil {
t.Fatal("expected non-nil registry")
}
}

// ---- Runtime.GetTools with populated tools ----

func TestRuntime_GetTools_WithTools(t *testing.T) {
r := NewRuntime()
// Manually add tools
r.mu.Lock()
r.tools["tool1"] = Tool{Name: "tool1", Description: "test", ServerName: "s1"}
r.tools["tool2"] = Tool{Name: "tool2", Description: "test2", ServerName: "s1"}
r.mu.Unlock()
tools := r.GetTools()
if len(tools) != 2 {
t.Errorf("expected 2 tools, got %d", len(tools))
}
}

// ---- Runtime.CallTool with server but no client ----

func TestRuntime_CallTool_ServerNotConnected(t *testing.T) {
r := NewRuntime()
r.mu.Lock()
r.server["tool1"] = "server1"
// No client for server1
r.mu.Unlock()
_, err := r.CallTool(context.Background(), "tool1", nil)
if err == nil {
t.Error("expected error for unconnected server")
}
}

// ---- Registry.ToolCount ----

func TestRegistry_ToolCount(t *testing.T) {
r := NewRegistry()
if r.ToolCount() != 0 {
t.Errorf("expected 0 tools, got %d", r.ToolCount())
}
r.tools["t1"] = &Tool{Name: "t1"}
if r.ToolCount() != 1 {
t.Errorf("expected 1 tool, got %d", r.ToolCount())
}
}

// ---- Registry.GetTool ----

func TestRegistry_GetTool_NotFound_v2(t *testing.T) {
r := NewRegistry()
_, ok := r.GetTool("nonexistent")
if ok {
t.Error("expected not found")
}
}

func TestRegistry_GetTool_Found(t *testing.T) {
r := NewRegistry()
r.tools["tool1"] = &Tool{Name: "tool1", Description: "desc"}
tool, ok := r.GetTool("tool1")
if !ok {
t.Error("expected to find tool1")
}
if tool.Name != "tool1" {
t.Errorf("expected name 'tool1', got %q", tool.Name)
}
}
