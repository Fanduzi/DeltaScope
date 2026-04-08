// Package mcpapi verifies the MCP interface bootstrap and tool surface.
// input: MCP server construction, in-memory transports, and tool discovery requests
// output: regression coverage for the official DeltaScope MCP server bootstrap
// pos: interface-level tests for the MCP adapter module
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewServerExposesCoreTools(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	got, err := collectToolNames(context.Background(), session.Tools(context.Background(), nil))
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	want := []string{"audit_sql", "describe_rule", "get_capabilities", "list_rules"}
	if !slices.Equal(got, want) {
		t.Fatalf("tool names mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestNewServerPublishesOutputSchemasForCoreTools(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tools, err := collectTools(context.Background(), session.Tools(context.Background(), nil))
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	required := map[string][]string{
		"audit_sql":        {"verdict", "context"},
		"describe_rule":    {"rule_id", "summary"},
		"list_rules":       {"count", "rules"},
		"get_capabilities": {"dialects", "audit_modes"},
	}

	for name, props := range required {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("missing tool %q", name)
		}
		if tool.OutputSchema == nil {
			t.Fatalf("tool %q missing output schema", name)
		}
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Fatalf("tool %q output schema type = %T", name, tool.OutputSchema)
		}
		propertyMap, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("tool %q schema missing properties: %#v", name, tool.OutputSchema)
		}
		for _, prop := range props {
			if _, ok := propertyMap[prop]; !ok {
				t.Fatalf("tool %q schema missing property %q: %#v", name, prop, propertyMap)
			}
		}
	}

	auditTool := tools["audit_sql"]
	auditSchema, ok := auditTool.OutputSchema.(map[string]any)
	if !ok {
		t.Fatalf("audit_sql output schema type = %T", auditTool.OutputSchema)
	}
	properties, ok := auditSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema missing properties: %#v", auditTool.OutputSchema)
	}
	statements, ok := properties["statements"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema missing statements property: %#v", properties)
	}
	items, ok := statements["items"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema statements missing items: %#v", statements)
	}
	statementProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema statement items missing properties: %#v", items)
	}
	impactSchema, ok := statementProps["impact"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema missing impact property: %#v", statementProps)
	}
	impactProps, ok := impactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("audit_sql schema impact object missing properties: %#v", impactSchema)
	}
	for _, prop := range []string{"estimated_rows", "estimated_ratio", "risk_level", "confidence", "source", "reason_codes"} {
		if _, ok := impactProps[prop]; !ok {
			t.Fatalf("audit_sql schema impact missing property %q: %#v", prop, impactProps)
		}
	}
}

func TestAuditSQLToolReturnsOfflineAuditResultWithContext(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	if body["verdict"] != "reject" {
		t.Fatalf("expected reject verdict, got %#v", body["verdict"])
	}

	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %T", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "mysql" {
		t.Fatalf("expected mysql dialect, got %#v", contextValue["dialect"])
	}
	if contextValue["dialect_source"] != "default" {
		t.Fatalf("expected default dialect source, got %#v", contextValue["dialect_source"])
	}
	if contextValue["metadata_source"] != "none" {
		t.Fatalf("expected none metadata source, got %#v", contextValue["metadata_source"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements array, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if impact["risk_level"] != "high" {
		t.Fatalf("expected high risk level, got %#v", impact["risk_level"])
	}
	if impact["confidence"] != "high" {
		t.Fatalf("expected high confidence, got %#v", impact["confidence"])
	}
	if impact["source"] != "shape" {
		t.Fatalf("expected shape source, got %#v", impact["source"])
	}
	if ratio, ok := impact["estimated_ratio"].(float64); !ok || ratio != 1 {
		t.Fatalf("expected estimated_ratio 1, got %#v", impact["estimated_ratio"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "missing_where" {
		t.Fatalf("expected missing_where reason code, got %#v", impact["reason_codes"])
	}

	for _, key := range []string{"summary", "statements", "explanation"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("expected key %q in body: %#v", key, maps.Keys(body))
		}
	}
}

func TestAuditSQLToolReturnsStatementImpact(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements array, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if impact["risk_level"] != "high" {
		t.Fatalf("expected high risk level, got %#v", impact["risk_level"])
	}
	if impact["confidence"] != "high" {
		t.Fatalf("expected high confidence, got %#v", impact["confidence"])
	}
	if impact["source"] != "shape" {
		t.Fatalf("expected shape source, got %#v", impact["source"])
	}
	if ratio, ok := impact["estimated_ratio"].(float64); !ok || ratio != 1 {
		t.Fatalf("expected estimated_ratio 1, got %#v", impact["estimated_ratio"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "missing_where" {
		t.Fatalf("expected missing_where reason code, got %#v", impact["reason_codes"])
	}
}

func TestAuditSQLToolReturnsMetadataAwareContextForDirectConnection(t *testing.T) {
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        metadataOnlyClient{},
			Dialect:       spec.DialectTiDB,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"host":     "127.0.0.1",
				"port":     3306,
				"user":     "root",
				"password": "secret",
				"schema":   "app",
			},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body := result.StructuredContent.(map[string]any)
	contextValue := body["context"].(map[string]any)
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "tidb" {
		t.Fatalf("expected tidb dialect, got %#v", contextValue["dialect"])
	}
	if contextValue["schema"] != "app" {
		t.Fatalf("expected app schema, got %#v", contextValue["schema"])
	}
	if contextValue["metadata_source"] != "direct" {
		t.Fatalf("expected direct metadata source, got %#v", contextValue["metadata_source"])
	}
}

func TestAuditSQLToolResolvesConnectionRefFromServerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(path, []byte(`
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password: secret
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Host != "10.0.0.12" || request.Connection.User != "audit_bot" || request.Connection.Password != "secret" {
			t.Fatalf("unexpected prepared connection: %#v", request.Connection)
		}
		return &auditmeta.PreparedAudit{
			Client:        metadataOnlyClient{},
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "config",
		}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version", ConnectionsPath: path})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users", "connection_ref": "prod_readonly"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body := result.StructuredContent.(map[string]any)
	contextValue := body["context"].(map[string]any)
	if contextValue["metadata_source"] != "connection_ref" {
		t.Fatalf("expected connection_ref metadata source, got %#v", contextValue["metadata_source"])
	}
	if contextValue["schema_source"] != "config" {
		t.Fatalf("expected config schema source, got %#v", contextValue["schema_source"])
	}
}

func TestAuditSQLToolUsesConnectionRefSchemaHint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(path, []byte(`
connections:
  prod_readonly:
    host: 10.0.0.12
    user: audit_bot
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previous := prepareMetadataAudit
	var gotSchemaHint string
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		gotSchemaHint = request.SchemaHint
		return nil, &auditmeta.Error{
			Kind:    auditmeta.ErrorSchemaHintRequired,
			Message: `schema inference for table "users" is ambiguous; set connections.prod_readonly.schema in ` + path,
		}
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version", ConnectionsPath: path})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users", "connection_ref": "prod_readonly"},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
	if !strings.Contains(gotSchemaHint, `connections.prod_readonly.schema`) {
		t.Fatalf("unexpected schema hint: %q", gotSchemaHint)
	}
	if !strings.Contains(body["message"].(string), `connections.prod_readonly.schema`) {
		t.Fatalf("unexpected message: %#v", body["message"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForEmptySQL(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": ""},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "bad_request" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolRejectsUnsupportedDialectBeforeMetadataSetup(t *testing.T) {
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatal("expected unsupported dialect to fail before metadata setup")
		return nil, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "delete from users",
			"dialect": "postgres",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "bad_request" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForConflictingConnectionInputs(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":            "delete from users",
			"connection_ref": "prod",
			"connection": map[string]any{
				"host":     "127.0.0.1",
				"user":     "root",
				"password": "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForSocketTCPConflict(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"port":   3306,
				"socket": "/tmp/mysql.sock",
				"user":   "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForPasswordFileReadFailure(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"host":          "127.0.0.1",
				"user":          "root",
				"password_file": "/definitely/missing/secret.txt",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForEmptyDirectConnection(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":        "delete from users",
			"connection": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForMalformedConnectionRefConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(path, []byte("connections: ["), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	server := NewServer(Config{Version: "test-version", ConnectionsPath: path})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users", "connection_ref": "prod"},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "config_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsConnectionInvalidForMetadataSchemaHintError(t *testing.T) {
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return nil, &auditmeta.Error{
			Kind:    auditmeta.ErrorSchemaHintRequired,
			Message: `schema inference for table "users" is ambiguous; set connection.schema`,
		}
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsConnectionInvalidForMetadataParseError(t *testing.T) {
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return nil, &auditmeta.Error{
			Kind:    auditmeta.ErrorInvalidSQL,
			Message: `resolve schema targets: parse sql`,
		}
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "bad_request" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestAuditSQLToolReturnsStructuredErrorForIncompleteDirectConnection(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from users",
			"connection": map[string]any{
				"schema": "app",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := result.StructuredContent.(map[string]any)
	if body["code"] != "connection_invalid" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
}

func TestNewServerPublishesImplementationMetadata(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "v9.9.9"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	info := session.InitializeResult().ServerInfo
	if info == nil {
		t.Fatal("expected initialize result server info")
	}
	if info.Name != "deltascope-mcp" {
		t.Fatalf("expected implementation name deltascope-mcp, got %q", info.Name)
	}
	if info.Version != "v9.9.9" {
		t.Fatalf("expected implementation version v9.9.9, got %q", info.Version)
	}
}

func connectClientSession(ctx context.Context, server *sdkmcp.Server) (*sdkmcp.ClientSession, error) {
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		return nil, err
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	return client.Connect(ctx, clientTransport, nil)
}

func collectToolNames(_ context.Context, sequence iter.Seq2[*sdkmcp.Tool, error]) ([]string, error) {
	names := make([]string, 0)
	for tool, err := range sequence {
		if err != nil {
			return nil, err
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names, nil
}

func collectTools(_ context.Context, sequence iter.Seq2[*sdkmcp.Tool, error]) (map[string]*sdkmcp.Tool, error) {
	tools := make(map[string]*sdkmcp.Tool)
	for tool, err := range sequence {
		if err != nil {
			return nil, err
		}
		tools[tool.Name] = tool
	}
	return tools, nil
}

type metadataOnlyClient struct{}

func (metadataOnlyClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (metadataOnlyClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return &spec.TableSnapshot{}, nil
}

func (metadataOnlyClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectTiDB, nil
}

func (metadataOnlyClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return nil, nil
}

func (metadataOnlyClient) ResolveTableForIndex(context.Context, spec.Dialect, string, string) (string, error) {
	return "", nil
}

func (metadataOnlyClient) Close() error { return nil }
