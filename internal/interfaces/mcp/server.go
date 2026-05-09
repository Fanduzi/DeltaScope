// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: MCP server construction inputs, DeltaScope version metadata, and tool registration definitions
// output: ready-to-run MCP server instances that expose the official DeltaScope tool surface
// pos: interface adapter between the Go MCP SDK and DeltaScope audit/rule capabilities
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config configures the DeltaScope MCP server bootstrap.
type Config struct {
	Version         string
	ConnectionsPath string
	Logger          *slog.Logger // Optional structured logger. Defaults to stderr JSON if nil.
}

// NewServer returns a configured MCP server with the core DeltaScope tools registered.
func NewServer(config Config) *sdkmcp.Server {
	panicLog := newPanicLogger(config.Logger)

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "deltascope-mcp",
		Version: config.Version,
	}, nil)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "audit_sql",
		Description:  "Audit SQL statements with DeltaScope.",
		OutputSchema: auditSQLResultSchema,
	}, recoverTool(newAuditSQLTool(config), panicLog))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "describe_rule",
		Description:  "Describe one shipped DeltaScope rule by rule ID.",
		OutputSchema: describeRuleOutputSchema,
	}, recoverTool(describeRuleTool, panicLog))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "list_rules",
		Description:  "List shipped DeltaScope rules with optional filters.",
		OutputSchema: listRulesOutputSchema,
	}, recoverTool(listRulesTool, panicLog))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "get_capabilities",
		Description:  "Return a concise DeltaScope capability summary for MCP clients.",
		OutputSchema: capabilitiesOutputSchema,
	}, recoverTool(newGetCapabilitiesTool(config), panicLog))

	return server
}

type describeRuleInput struct {
	RuleID string `json:"rule_id" jsonschema:"Shipped DeltaScope rule identifier"`
}

type listRulesInput struct {
	Query string `json:"query,omitempty" jsonschema:"Optional keyword query for shipped rules"`
}

type getCapabilitiesInput struct{}

func describeRuleTool(_ context.Context, _ *sdkmcp.CallToolRequest, input describeRuleInput) (*sdkmcp.CallToolResult, any, error) {
	result, err := describeRulePayload(input.RuleID)
	if err != nil {
		toolResult, toolErr := toolError("bad_request", err.Error())
		return toolResult, nil, toolErr
	}
	return nil, result, nil
}

func listRulesTool(_ context.Context, _ *sdkmcp.CallToolRequest, input listRulesInput) (*sdkmcp.CallToolResult, any, error) {
	return nil, listRulesPayload(input.Query), nil
}

func newGetCapabilitiesTool(config Config) func(context.Context, *sdkmcp.CallToolRequest, getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
		return nil, capabilitiesPayload(config.ConnectionsPath), nil
	}
}

// recoverTool wraps a tool handler with panic recovery. If the handler panics,
// the panic is caught, logged with a stack trace, and a structured error is
// returned to the client instead of crashing the entire MCP service.
func recoverTool[T any](handler func(context.Context, *sdkmcp.CallToolRequest, T) (*sdkmcp.CallToolResult, any, error), panicLog *log.Logger) func(context.Context, *sdkmcp.CallToolRequest, T) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input T) (result *sdkmcp.CallToolResult, structured any, err error) {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				panicLog.Printf("MCP panic recovered: %v\nStack trace:\n%s", r, string(buf[:n]))
				errorResult, _ := toolError("internal_error", "internal server error")
				result, structured, err = errorResult, nil, nil
			}
		}()
		return handler(ctx, req, input)
	}
}

func newPanicLogger(sl *slog.Logger) *log.Logger {
	if sl != nil {
		return logger.NewStdLogger(sl)
	}
	return log.New(os.Stderr, "mcp: ", log.LstdFlags)
}
