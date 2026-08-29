//go:build postgresql

// Package mcpapi verifies PostgreSQL metadata-aware MCP audit behavior.
// input: in-process MCP audit_sql calls, separate PostgreSQL database/schema inputs, and fake metadata clients
// output: PostgreSQL metadata-aware result context, rule findings, planner behavior, and connection propagation
// pos: PostgreSQL-tagged interface tests for the MCP adapter
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolSupportsPostgreSQLMetadataAwareMode(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Dialect != spec.DialectPostgreSQL {
			t.Fatalf("expected postgresql dialect hint to flow into shared prepare, got %#v", request.Connection)
		}
		if request.Connection.Database != "analytics" {
			t.Fatalf("expected postgresql database to flow into shared prepare, got %#v", request.Connection)
		}
		if request.ExplicitSchema != "public" {
			t.Fatalf("expected postgresql schema to remain separate, got %q", request.ExplicitSchema)
		}
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
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
			"sql":     "delete from public.users where id = 1",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":     "127.0.0.1",
				"user":     "root",
				"database": "analytics",
				"schema":   "public",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected successful result, got protocol error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got %#v", result.StructuredContent)
	}
	body := result.StructuredContent.(map[string]any)
	contextBody := body["context"].(map[string]any)
	if contextBody["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", contextBody)
	}
	if contextBody["dialect"] != "postgresql" {
		t.Fatalf("expected postgresql dialect context, got %#v", contextBody)
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact, got %#v", statement["impact"])
	}
	if impact["source"] != "plan" {
		t.Fatalf("expected planner impact source, got %#v", impact)
	}
	if rows, ok := impact["estimated_rows"].(float64); !ok || rows != 7 {
		t.Fatalf("expected estimated_rows 7, got %#v", impact["estimated_rows"])
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one planner call, got %d", client.planCalls)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
}

func TestAuditSQLToolPostgreSQLMetadataAwareUPDATETriggersPlanEstimation(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
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
			"sql":     "update public.users set name = 'x' where id = 1",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected successful result, got protocol error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got %#v", result.StructuredContent)
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one planner call, got %d", client.planCalls)
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
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact, got %#v", statement["impact"])
	}
	if impact["source"] != "plan" {
		t.Fatalf("expected planner impact source, got %#v", impact)
	}
}

func TestAuditSQLToolPostgreSQLMetadataAwareINSERTDoesNotTriggerPlanEstimation(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
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
			"sql":     "insert into public.users (id, name) values (1, 'alice')",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected successful result, got protocol error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got %#v", result.StructuredContent)
	}
	if client.planCalls != 0 {
		t.Fatalf("expected no planner calls for INSERT, got %d", client.planCalls)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
}

func TestAuditSQLToolPostgreSQLMetadataMapsDropConstraintToPrimaryKeyRule(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "users"},
			PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
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
			"sql":     "alter table users drop constraint users_pkey;",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"user":   "root",
				"schema": "public",
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
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.alter.drop_primary_key.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop primary key finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLMetadataRequiresExistingColumnForRenameColumn(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
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
			"sql":     "alter table users rename column missing_email to email;",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"user":   "root",
				"schema": "public",
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
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.alter.rename_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename-column existence finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLMetadataRequiresExistingColumnForDropColumn(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
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
			"sql":     "alter table users drop column missing_email;",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"user":   "root",
				"schema": "public",
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
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.alter.drop_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-column existence finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLMetadataRequiresExistingTableForRenameTable(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: false,
			Table:  &spec.Table{Name: "users"},
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
			"sql":     "alter table users rename to users_archive;",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"user":   "root",
				"schema": "public",
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
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.table.exists.alter.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected alter-table existence finding, got %#v", findings)
	}
}
