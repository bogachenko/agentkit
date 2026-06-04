package port

import (
	"context"

	"github.com/bogachenko/agentkit/core/tool"
)

// ToolExecutor executes explicit tool calls after runtime validation, without selecting tools.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error)
}
