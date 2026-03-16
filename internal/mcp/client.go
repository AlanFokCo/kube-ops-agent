package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Client is MCP client interface, supports stdio and HTTP transport.
type Client interface {
	Connect(ctx context.Context) error
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
	Close() error
	Tools() []Tool
}

// Ensure StdioClient and HTTPClient implement Client interface.
var _ Client = (*StdioClient)(nil)
var _ Client = (*HTTPClient)(nil)

// ToolResult represents MCP tool call result.
type ToolResult struct {
	Content []map[string]any `json:"content"`
	IsError bool             `json:"isError"`
}

// GetText extracts text from content.
func (r *ToolResult) GetText() string {
	var texts []string
	for _, item := range r.Content {
		if item["type"] == "text" {
			if t, ok := item["text"].(string); ok {
				texts = append(texts, t)
			}
		}
	}
	if len(texts) == 0 {
		return ""
	}
	result := texts[0]
	for i := 1; i < len(texts); i++ {
		result += "\n" + texts[i]
	}
	return result
}

// StdioClient communicates with MCP server via stdio.
type StdioClient struct {
	config ServerConfig
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
	mu     sync.Mutex
	id     int
	tools  []Tool
}

// NewStdioClient creates stdio MCP client.
func NewStdioClient(cfg ServerConfig) *StdioClient {
	return &StdioClient{config: cfg}
}

// Connect starts subprocess and completes initialize handshake.
func (c *StdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		return nil
	}
	args := c.config.Args
	if args == nil {
		args = []string{}
	}
	c.cmd = exec.CommandContext(ctx, c.config.Command, args...)
	c.cmd.Env = os.Environ()
	for k, v := range c.config.Env {
		c.cmd.Env = append(c.cmd.Env, k+"="+v)
	}
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := c.cmd.Start(); err != nil {
		return err
	}
	c.stdin = bufio.NewWriter(stdin)
	c.stdout = bufio.NewReader(stdout)

	// initialize
	initReq := c.mkRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"clientInfo":     map[string]any{"name": "KOA-MCP-Client", "version": "1.0.0"},
	})
	if _, err := c.send(initReq); err != nil {
		c.cmd.Process.Kill()
		return err
	}
	// notifications/initialized
	_ = c.sendNotification("notifications/initialized", nil)

	// tools/list
	listReq := c.mkRequest("tools/list", nil)
	res, err := c.send(listReq)
	if err != nil {
		return err
	}
	if m, ok := res.(map[string]any); ok {
		if arr, ok := m["tools"].([]any); ok {
			for _, a := range arr {
				if t, ok := a.(map[string]any); ok {
					tool := Tool{
						Name:        getStr(t, "name"),
						Description: getStr(t, "description"),
						ServerName:  c.config.Name,
						InputSchema: getMap(t, "inputSchema"),
					}
					c.tools = append(c.tools, tool)
				}
			}
		}
	}
	return nil
}

func getStr(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getMap(m map[string]any, k string) map[string]interface{} {
	if v, ok := m[k]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
	}
	return nil
}

func (c *StdioClient) mkRequest(method string, params any) map[string]any {
	c.id++
	req := map[string]any{"jsonrpc": "2.0", "method": method, "id": c.id}
	if params != nil {
		req["params"] = params
	}
	return req
}

func (c *StdioClient) sendNotification(method string, params any) error {
	req := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		req["params"] = params
	}
	data, _ := json.Marshal(req)
	c.stdin.Write(data)
	c.stdin.WriteByte('\n')
	return c.stdin.Flush()
}

func (c *StdioClient) send(req map[string]any) (any, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	c.stdin.Write(data)
	c.stdin.WriteByte('\n')
	if err := c.stdin.Flush(); err != nil {
		return nil, err
	}
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if errObj, ok := resp["error"]; ok {
		if em, ok := errObj.(map[string]any); ok {
			return nil, fmt.Errorf("MCP error: %v", em["message"])
		}
	}
	return resp["result"], nil
}

// CallTool calls MCP tool.
func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return nil, fmt.Errorf("not connected")
	}
	req := c.mkRequest("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	res, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if m, ok := res.(map[string]any); ok {
		tr := &ToolResult{
			IsError: m["isError"] == true,
		}
		if arr, ok := m["content"].([]any); ok {
			for _, a := range arr {
				if item, ok := a.(map[string]any); ok {
					tr.Content = append(tr.Content, item)
				}
			}
		}
		return tr, nil
	}
	return nil, fmt.Errorf("unexpected response")
}

// Close closes connection.
func (c *StdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd = nil
	}
	return nil
}

// Tools returns discovered tools.
func (c *StdioClient) Tools() []Tool {
	return c.tools
}

// HTTPClient communicates with MCP server via HTTP POST (JSON-RPC over HTTP).
type HTTPClient struct {
	config ServerConfig
	url    string
	client *http.Client
	mu     sync.Mutex
	id     int
	tools  []Tool
}

// NewHTTPClient creates HTTP MCP client.
func NewHTTPClient(cfg ServerConfig) *HTTPClient {
	if cfg.URL == "" {
		return nil
	}
	return &HTTPClient{
		config: cfg,
		url:    cfg.URL,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Connect completes initialize handshake via HTTP and fetches tools.
func (c *HTTPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.tools) > 0 {
		return nil
	}
	initReq := c.mkRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"clientInfo":     map[string]any{"name": "KOA-MCP-Client", "version": "1.0.0"},
	})
	if _, err := c.send(ctx, initReq); err != nil {
		return err
	}
	_ = c.sendNotification(ctx, "notifications/initialized", nil)
	listReq := c.mkRequest("tools/list", nil)
	res, err := c.send(ctx, listReq)
	if err != nil {
		return err
	}
	if m, ok := res.(map[string]any); ok {
		if arr, ok := m["tools"].([]any); ok {
			for _, a := range arr {
				if t, ok := a.(map[string]any); ok {
					c.tools = append(c.tools, Tool{
						Name:        getStr(t, "name"),
						Description: getStr(t, "description"),
						ServerName:  c.config.Name,
						InputSchema: getMap(t, "inputSchema"),
					})
				}
			}
		}
	}
	return nil
}

func (c *HTTPClient) mkRequest(method string, params any) map[string]any {
	c.id++
	req := map[string]any{"jsonrpc": "2.0", "method": method, "id": c.id}
	if params != nil {
		req["params"] = params
	}
	return req
}

func (c *HTTPClient) sendNotification(ctx context.Context, method string, params any) error {
	req := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		req["params"] = params
	}
	data, _ := json.Marshal(req)
	hr, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	hr.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(hr)
	if err != nil {
		return err
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil
}

func (c *HTTPClient) send(ctx context.Context, req map[string]any) (any, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hr, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(hr)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if errObj, ok := result["error"]; ok {
		if em, ok := errObj.(map[string]any); ok {
			return nil, fmt.Errorf("MCP error: %v", em["message"])
		}
	}
	return result["result"], nil
}

// CallTool calls MCP tool.
func (c *HTTPClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.mkRequest("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	res, err := c.send(ctx, req)
	if err != nil {
		return nil, err
	}
	if m, ok := res.(map[string]any); ok {
		tr := &ToolResult{IsError: m["isError"] == true}
		if arr, ok := m["content"].([]any); ok {
			for _, a := range arr {
				if item, ok := a.(map[string]any); ok {
					tr.Content = append(tr.Content, item)
				}
			}
		}
		return tr, nil
	}
	return nil, fmt.Errorf("unexpected response")
}

// Close closes connection (HTTP has no persistent conn, no-op).
func (c *HTTPClient) Close() error {
	return nil
}

// Tools returns discovered tools.
func (c *HTTPClient) Tools() []Tool {
	return c.tools
}

// Runtime manages multiple MCP client connections (stdio and HTTP).
type Runtime struct {
	mu      sync.RWMutex
	clients map[string]Client
	tools   map[string]Tool
	server  map[string]string // toolName -> serverName
}

// NewRuntime creates MCP runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		clients: make(map[string]Client),
		tools:   make(map[string]Tool),
		server:  make(map[string]string),
	}
}

// Initialize connects all enabled servers from config. Supports transport=stdio (default) and transport=http.
func (r *Runtime) Initialize(configs []ServerConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		var client Client
		if cfg.URL != "" {
			hc := NewHTTPClient(cfg)
			if hc != nil {
				client = hc
			} else {
				continue
			}
		} else if cfg.Command != "" {
			client = NewStdioClient(cfg)
		} else {
			continue
		}
		if err := client.Connect(ctx); err != nil {
			continue
		}
		r.mu.Lock()
		r.clients[cfg.Name] = client
		for _, t := range client.Tools() {
			r.tools[t.Name] = t
			r.server[t.Name] = cfg.Name
		}
		r.mu.Unlock()
	}
	return nil
}

// Shutdown closes all connections.
func (r *Runtime) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.clients {
		c.Close()
	}
	r.clients = nil
	r.tools = nil
	r.server = nil
}

// CallTool calls the specified tool.
func (r *Runtime) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	r.mu.RLock()
	serverName, ok := r.server[name]
	client, clientOk := r.clients[serverName]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	if !clientOk || client == nil {
		return nil, fmt.Errorf("server not connected: %s", serverName)
	}
	return client.CallTool(ctx, name, args)
}

// GetTools returns all tools.
func (r *Runtime) GetTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Tool
	seen := make(map[string]bool)
	for _, t := range r.tools {
		if !seen[t.Name] {
			seen[t.Name] = true
			out = append(out, t)
		}
	}
	return out
}
