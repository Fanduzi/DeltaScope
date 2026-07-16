// Package mcpapi verifies that Query Access remains outside the MCP tool surface.
// input: MCP server tool discovery
// output: no query-access tool registration
// pos: cross-surface contract coverage
package mcpapi

import (
	"context"
	"testing"
)

func TestQueryAccessPureEffectSurfaceContract(t *testing.T) {
	// Given: the current official MCP server.
	server := NewServer(Config{Version: "test-version"})

	// When: a client discovers the registered tools.
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	tools, err := collectToolNames(context.Background(), session.Tools(context.Background(), nil))
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	// Then: Query Access has no MCP tool in this milestone.
	for _, tool := range tools {
		if tool == "query_access" || tool == "query-access" {
			t.Fatalf("unexpected Query Access MCP tool %q", tool)
		}
	}
}
