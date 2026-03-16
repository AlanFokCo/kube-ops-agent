package mcp

import (
	"context"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/tool"
)

// Tool represents an MCP tool.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ServerName  string                 `json:"server_name"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// Server represents an MCP server.
type Server struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

// Registry manages MCP servers and tools.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*Server
	tools   map[string]*Tool
	runtime *Runtime // runtime connection for actual calls
}

// NewRegistry creates new MCP registry.
func NewRegistry() *Registry {
	return &Registry{
		servers: make(map[string]*Server),
		tools:   make(map[string]*Tool),
	}
}

// ServerConfig aligned with Python MCPServerConfig, supports command/args/transport.
type ServerConfig struct {
	Name        string            `yaml:"name"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Transport   string            `yaml:"transport"`
	URL         string            `yaml:"url"`
	Env         map[string]string `yaml:"env"`
	Enabled     bool              `yaml:"enabled"`
	Description string            `yaml:"description"`
}

// InitFromConfig initializes MCP registry from YAML config.
// Two formats: 1) static tools list 2) Python format command/args/transport (register servers only, tools need runtime)
func InitFromConfig(path string) (*Registry, error) {
	r := NewRegistry()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mcp config: %w", err)
	}

	// Format A: static tools
	var configA struct {
		Servers []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			Tools       []struct {
				Name        string                 `yaml:"name"`
				Description string                 `yaml:"description"`
				InputSchema map[string]interface{} `yaml:"input_schema"`
			} `yaml:"tools"`
		} `yaml:"servers"`
	}

	if err := yaml.Unmarshal(data, &configA); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}

	for _, srv := range configA.Servers {
		server := &Server{
			Name:        srv.Name,
			Description: srv.Description,
			Tools:       make([]string, 0, len(srv.Tools)),
		}
		r.servers[srv.Name] = server

		for _, t := range srv.Tools {
			tool := &Tool{
				Name:        t.Name,
				Description: t.Description,
				ServerName:  srv.Name,
				InputSchema: t.InputSchema,
			}
			r.tools[t.Name] = tool
			server.Tools = append(server.Tools, t.Name)
		}
	}

	// If servers exist, done; else try Format B (Python format)
	if len(r.servers) > 0 {
		return r, nil
	}

	var configB struct {
		Enabled     bool          `yaml:"enabled"`
		AutoConnect bool          `yaml:"auto_connect"`
		Servers     []ServerConfig `yaml:"servers"`
	}
	if err := yaml.Unmarshal(data, &configB); err != nil {
		return r, nil
	}
	for _, sc := range configB.Servers {
		if !sc.Enabled {
			continue
		}
		if sc.Name == "" {
			continue
		}
		// Register servers only, tools need MCP runtime (to be implemented)
		r.servers[sc.Name] = &Server{
			Name:        sc.Name,
			Description: sc.Description,
			Tools:       []string{},
		}
	}

	return r, nil
}

// TryInitFromConfig tries to load config, returns nil,nil if file missing.
func TryInitFromConfig(path string) (*Registry, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return InitFromConfig(path)
}

// ListServers returns all registered servers.
func (r *Registry) ListServers() []Server {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Server, 0, len(r.servers))
	for _, s := range r.servers {
		out = append(out, *s)
	}
	return out
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, *t)
	}
	return out
}

// GetTool gets tool by name.
func (r *Registry) GetTool(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// IsInitialized checks if registry is initialized.
func (r *Registry) IsInitialized() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.servers) > 0 || len(r.tools) > 0
}

// ConnectedServers returns connected server names (aligned with Python).
func (r *Registry) ConnectedServers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// ToolCount returns total tool count.
func (r *Registry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// InitializeMCP initializes MCP, establishes stdio/HTTP runtime and discovers tools.
func InitializeMCP(configPath string) (*Registry, error) {
	r, err := InitFromConfig(configPath)
	if err != nil {
		return nil, err
	}
	// Try loading Python format config and establish runtime connection
	configs := loadServerConfigs(configPath)
	if len(configs) > 0 {
		rt := NewRuntime()
		if rt.Initialize(configs) == nil {
			r.mu.Lock()
			r.runtime = rt
			for _, t := range rt.GetTools() {
				tool := &Tool{
					Name:        t.Name,
					Description: t.Description,
					ServerName:  t.ServerName,
					InputSchema: t.InputSchema,
				}
				r.tools[t.Name] = tool
				if srv, ok := r.servers[t.ServerName]; ok {
					srv.Tools = append(srv.Tools, t.Name)
				}
			}
			r.mu.Unlock()
		}
	}
	return r, nil
}

func loadServerConfigs(path string) []ServerConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Servers []ServerConfig `yaml:"servers"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	var out []ServerConfig
	for _, s := range cfg.Servers {
		if !s.Enabled {
			continue
		}
		if s.Command != "" || s.URL != "" {
			out = append(out, s)
		}
	}
	return out
}

// ShutdownMCP closes MCP connections, releases resources.
func (r *Registry) ShutdownMCP() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runtime != nil {
		r.runtime.Shutdown()
		r.runtime = nil
	}
}

// CreateMCPToolFunction converts MCP tool to agentscope-go *tool.Tool for Agent Toolkit.
// If Registry has runtime via InitializeMCP, actually calls MCP server.
func (r *Registry) CreateMCPToolFunction(t *Tool) *tool.Tool {
	if t == nil {
		return nil
	}
	desc := t.Description
	if desc == "" {
		desc = "MCP tool: " + t.Name
	}
	reg := r
	return &tool.Tool{
		Name:        t.Name,
		Description: desc,
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			reg.mu.RLock()
			rt := reg.runtime
			reg.mu.RUnlock()
			if rt == nil {
				return map[string]any{"error": "MCP runtime not initialized"}, fmt.Errorf("MCP tool %s: runtime not initialized", t.Name)
			}
			res, err := rt.CallTool(ctx, t.Name, args)
			if err != nil {
				return map[string]any{"error": err.Error()}, err
			}
			return map[string]any{"content": res.Content, "text": res.GetText()}, nil
		},
	}
}

// CreateMCPToolFunctionStatic creates MCP tool statically (when no Registry).
func CreateMCPToolFunction(t *Tool) *tool.Tool {
	if t == nil {
		return nil
	}
	desc := t.Description
	if desc == "" {
		desc = "MCP tool: " + t.Name
	}
	return &tool.Tool{
		Name:        t.Name,
		Description: desc,
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"error": "MCP runtime not initialized"}, fmt.Errorf("MCP tool %s: runtime not initialized", t.Name)
		},
	}
}

// MCPToolsAsAgentTools converts all MCP tools in Registry to agentscope-go tool list.
func (r *Registry) MCPToolsAsAgentTools() []*tool.Tool {
	r.mu.RLock()
	tools := r.ListTools()
	r.mu.RUnlock()

	out := make([]*tool.Tool, 0, len(tools))
	for i := range tools {
		if t := r.CreateMCPToolFunction(&tools[i]); t != nil {
			out = append(out, t)
		}
	}
	return out
}
