package mcp

import (
	"context"
	"encoding/json"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
)

// newMCPRawTool creates a tool.Tool from a name, description, and function.
// Errors from fn are returned directly (not wrapped into ToolResponse).
type mcpRawTool struct {
	tool.BaseTool
	fn func(ctx context.Context, input map[string]any) (any, error)
}

func (t *mcpRawTool) Execute(ctx context.Context, input map[string]any) (*tool.ToolResponse, error) {
	result, err := t.fn(ctx, input)
	if err != nil {
		return nil, err
	}
	switch v := result.(type) {
	case string:
		return tool.NewTextResponse(v), nil
	case *tool.ToolResponse:
		return v, nil
	default:
		b, _ := json.Marshal(v)
		return tool.NewTextResponse(string(b)), nil
	}
}

func newMCPRawTool(name, description string, fn func(ctx context.Context, input map[string]any) (any, error)) tool.Tool {
	return &mcpRawTool{
		BaseTool: tool.BaseTool{
			ToolName:        name,
			ToolDescription: description,
		},
		fn: fn,
	}
}
