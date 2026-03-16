package mcp

import (
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
