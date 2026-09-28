// Package mcpapi verifies the T04-B/#83 target-version MCP contract.
// input: audit_sql tool calls carrying target_version under the isolated key-length policy
// output: isError=false results with canonical version/gap evidence and isError=true bad_request for malformed input
// pos: MCP transport contract tests for version evidence (issue #83 T04-B slice)
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

const t04bKeyLengthRuleID = "ddl.index.key_length.max_bytes.require"
const t04bGoldenSQL = "CREATE TABLE t (c VARCHAR(255) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

func writeT04BIsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04b-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04bKeyLengthRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

func callT04BAudit(t *testing.T, session *sdkmcp.ClientSession, configPath string, targetVersion string) *sdkmcp.CallToolResult {
	t.Helper()
	args := map[string]any{
		"sql":         t04bGoldenSQL,
		"dialect":     "mysql",
		"config_path": configPath,
	}
	if targetVersion != "" {
		args["target_version"] = targetVersion
	}
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("expected tool result, got protocol error: %v", err)
	}
	return result
}

// The MCP surface forwards target_version to the shared parser: absent input
// yields the bounded gap, valid input projects the canonical identity, and
// malformed input is a bad_request tool error.
func TestAuditSQLT04BTargetVersionContract(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	configPath := writeT04BIsolatedPolicy(t)

	t.Run("absent version returns gap", func(t *testing.T) {
		result := callT04BAudit(t, session, configPath, "")
		if result.IsError {
			t.Fatal("expected isError=false for gap-only audit")
		}
		body := requireAuditStructuredMap(t, result)
		if c, _ := body["coverage"].(map[string]any); c["status"] != "unverified" {
			t.Fatalf("expected unverified, got %#v", body["coverage"])
		}
		stmts, _ := body["statements"].([]any)
		first, _ := stmts[0].(map[string]any)
		gaps, _ := first["evidence_gaps"].([]any)
		if len(gaps) != 1 {
			t.Fatalf("expected one gap, got %#v", gaps)
		}
		gap, _ := gaps[0].(map[string]any)
		if gap["reason_code"] != "missing_target_version" || gap["rule_id"] != t04bKeyLengthRuleID {
			t.Fatalf("gap = %#v", gap)
		}
	})

	t.Run("valid version returns identity", func(t *testing.T) {
		result := callT04BAudit(t, session, configPath, "8.4.10")
		if result.IsError {
			t.Fatal("expected isError=false for valid version")
		}
		body := requireAuditStructuredMap(t, result)
		version, _ := body["version"].(map[string]any)
		if version["product"] != "mysql" || version["version"] != "8.4.10" || version["source"] != "target" {
			t.Fatalf("version = %#v", version)
		}
	})

	t.Run("malformed version is bad_request error", func(t *testing.T) {
		result := callT04BAudit(t, session, configPath, "8.4")
		if !result.IsError {
			t.Fatal("expected isError=true for malformed target_version")
		}
		body := requireAuditStructuredMap(t, result)
		if body["code"] != "bad_request" {
			t.Fatalf("expected code=bad_request, got %#v", body)
		}
	})

	t.Run("out-of-range version returns range gap", func(t *testing.T) {
		result := callT04BAudit(t, session, configPath, "9.0.0")
		if result.IsError {
			t.Fatal("expected isError=false for out-of-range version")
		}
		body := requireAuditStructuredMap(t, result)
		stmts, _ := body["statements"].([]any)
		first, _ := stmts[0].(map[string]any)
		gaps, _ := first["evidence_gaps"].([]any)
		if len(gaps) != 1 {
			t.Fatalf("expected one gap, got %#v", gaps)
		}
		if gap, _ := gaps[0].(map[string]any); gap["reason_code"] != "target_version_out_of_validated_range" {
			t.Fatalf("gap = %#v", gap)
		}
	})
}

// A malformed target_version is validated before any connection lookup or
// open: even a resolvable connection_ref must lose to the input error.
func TestAuditSQLT04BPreflightPrecedesConnection(t *testing.T) {
	dir := t.TempDir()
	connectionsPath := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(connectionsPath, []byte(`
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password: secret
`), 0o600); err != nil {
		t.Fatalf("write connections: %v", err)
	}

	previous := prepareMetadataAudit
	prepareCalled := false
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		prepareCalled = true
		return nil, errors.New("prepare must not run before target_version preflight")
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	session, err := connectClientSession(context.Background(), NewServer(Config{
		Version:         "test-version",
		ConnectionsPath: connectionsPath,
	}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":            t04bGoldenSQL,
			"dialect":        "mysql",
			"target_version": "8.4",
			"connection_ref": "prod_readonly",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true for malformed target_version")
	}
	body := requireAuditStructuredMap(t, result)
	if body["code"] != "bad_request" {
		t.Fatalf("expected code=bad_request before connection, got %#v", body)
	}
	if prepareCalled {
		t.Fatalf("metadata prepare must not run before target_version preflight")
	}
}
