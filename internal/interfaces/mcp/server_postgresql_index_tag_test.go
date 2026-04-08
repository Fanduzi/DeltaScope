//go:build postgresql

// Package mcpapi verifies PostgreSQL standalone index metadata-aware MCP behavior.
// input: MCP audit_sql calls for standalone PostgreSQL index statements with fake metadata clients
// output: focused transport parity coverage for standalone index owner resolution paths
// pos: tagged MCP adapter regression coverage for PostgreSQL standalone index metadata support
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolPostgreSQLMetadataRenameIndexResolvesOwnerAndReportsExistence(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
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
			"sql":        "alter index missing_idx rename to idx_new;",
			"dialect":    "postgresql",
			"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "public"},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	if len(client.indexCalls) != 1 || client.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution, got %#v", client.indexCalls)
	}
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", client.indexSchemas)
	}
	if len(client.indexDialects) != 1 || client.indexDialects[0] != spec.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", client.indexDialects)
	}
	if len(client.tableCalls) != 1 || client.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", client.tableCalls)
	}
	body := result.StructuredContent.(map[string]any)
	contextBody, ok := body["context"].(map[string]any)
	if !ok || contextBody["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", body["context"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.rename_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename_index existence finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLMetadataDropIndexResolvesOwnerAndReportsExistence(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
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
			"sql":        "drop index missing_idx;",
			"dialect":    "postgresql",
			"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "public"},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	if len(client.indexCalls) != 1 || client.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution, got %#v", client.indexCalls)
	}
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", client.indexSchemas)
	}
	if len(client.indexDialects) != 1 || client.indexDialects[0] != spec.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", client.indexDialects)
	}
	if len(client.tableCalls) != 1 || client.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", client.tableCalls)
	}
	body := result.StructuredContent.(map[string]any)
	contextBody, ok := body["context"].(map[string]any)
	if !ok || contextBody["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", body["context"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.drop_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop_index existence finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLMetadataQualifiedRenameIndexUsesStatementSchema(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			DialectSource: "request",
			SchemaSource:  "none",
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
			"sql":        "alter index accounting.missing_idx rename to idx_new;",
			"dialect":    "postgresql",
			"connection": map[string]any{"host": "127.0.0.1", "user": "root"},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	if len(client.indexCalls) != 1 || client.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution, got %#v", client.indexCalls)
	}
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "accounting" {
		t.Fatalf("expected accounting schema for index-owner resolution, got %#v", client.indexSchemas)
	}
	if len(client.indexDialects) != 1 || client.indexDialects[0] != spec.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", client.indexDialects)
	}
	if len(client.tableCalls) != 1 || client.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", client.tableCalls)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.rename_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename_index existence finding, got %#v", findings)
	}
}
