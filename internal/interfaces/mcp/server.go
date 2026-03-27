// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: MCP server construction inputs, DeltaScope version metadata, and tool registration definitions
// output: ready-to-run MCP server instances that expose the official DeltaScope tool surface
// pos: interface adapter between the Go MCP SDK and DeltaScope audit/rule capabilities
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config configures the DeltaScope MCP server bootstrap.
type Config struct {
	Version         string
	ConnectionsPath string
}

// NewServer returns a configured MCP server with the core DeltaScope tools registered.
func NewServer(config Config) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "deltascope-mcp",
		Version: config.Version,
	}, nil)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "audit_sql",
		Description:  "Audit SQL statements with DeltaScope.",
		OutputSchema: auditSQLResultSchema,
	}, newAuditSQLTool(config))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "describe_rule",
		Description:  "Describe one shipped DeltaScope rule by rule ID.",
		OutputSchema: describeRuleOutputSchema,
	}, describeRuleTool)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "list_rules",
		Description:  "List shipped DeltaScope rules with optional filters.",
		OutputSchema: listRulesOutputSchema,
	}, listRulesTool)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:         "get_capabilities",
		Description:  "Return a concise DeltaScope capability summary for MCP clients.",
		OutputSchema: capabilitiesOutputSchema,
	}, newGetCapabilitiesTool(config))

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
