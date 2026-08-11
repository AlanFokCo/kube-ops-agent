package agent

import (
	"context"
	"encoding/json"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
)

// rawTool wraps a function into the tool.Tool interface, returning errors directly
// (not wrapped into ToolResponse). This preserves v1 error-return behavior for
// custom tools in this codebase.
type rawTool struct {
	tool.BaseTool
	fn func(ctx context.Context, input map[string]any) (any, error)
}

func (t *rawTool) Execute(ctx context.Context, input map[string]any) (*tool.ToolResponse, error) {
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

// newRawTool creates a tool.Tool from a name, description, and function.
// Errors from fn are returned directly (not wrapped into ToolResponse).
func newRawTool(name, description string, fn func(ctx context.Context, input map[string]any) (any, error)) tool.Tool {
	return &rawTool{
		BaseTool: tool.BaseTool{
			ToolName:        name,
			ToolDescription: description,
		},
		fn: fn,
	}
}
