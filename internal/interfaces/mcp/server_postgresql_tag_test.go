//go:build postgresql

// Package mcpapi verifies PostgreSQL-only MCP behavior under the PG-capable build.
// input: MCP audit_sql tool calls executed with dialect=postgresql against the PG-capable binary path
// output: focused coverage for PostgreSQL offline audit success and additive MCP context fields
// pos: tagged MCP adapter regression coverage for PostgreSQL surface support
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolAcceptsPostgreSQLOfflineRequests(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "drop index idx_name;", "dialect": "postgresql"},
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
	if body["verdict"] != "pass" {
		t.Fatalf("expected pass verdict, got %#v", body["verdict"])
	}

	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %T", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "postgresql" {
		t.Fatalf("expected postgresql dialect, got %#v", contextValue["dialect"])
	}
	if contextValue["dialect_source"] != "request" {
		t.Fatalf("expected request dialect source, got %#v", contextValue["dialect_source"])
	}
	if contextValue["metadata_source"] != "none" {
		t.Fatalf("expected none metadata source, got %#v", contextValue["metadata_source"])
	}
}

type mcpMetadataAuditTestClient struct {
	closed        bool
	detectDialect spec.Dialect
	planCalls     int
	tableCalls    []string
	indexCalls    []string
	indexSchemas  []string
	indexDialects []spec.Dialect
	indexTable    string
	snapshot      *spec.TableSnapshot
}

func (c *mcpMetadataAuditTestClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "PostgreSQL 16.3"}, nil
}

func (c *mcpMetadataAuditTestClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, _ string, table string) (*spec.TableSnapshot, error) {
	c.tableCalls = append(c.tableCalls, table)
	if c.snapshot != nil {
		return c.snapshot, nil
	}
	return &spec.TableSnapshot{Exists: true}, nil
}

func (c *mcpMetadataAuditTestClient) DetectDialect(context.Context) (spec.Dialect, error) {
	if c.detectDialect == "" {
		return spec.DialectPostgreSQL, nil
	}
	return c.detectDialect, nil
}

func (c *mcpMetadataAuditTestClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return []string{"public"}, nil
}

func (c *mcpMetadataAuditTestClient) ResolveTableForIndex(_ context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	c.indexCalls = append(c.indexCalls, index)
	c.indexDialects = append(c.indexDialects, dialect)
	c.indexSchemas = append(c.indexSchemas, schema)
	return c.indexTable, nil
}

func (c *mcpMetadataAuditTestClient) Close() error {
	c.closed = true
	return nil
}

func (c *mcpMetadataAuditTestClient) LoadPlanEstimate(context.Context, spec.Statement) (*spec.ImpactEstimate, error) {
	c.planCalls++
	rows := int64(7)
	ratio := 0.07
	return &spec.ImpactEstimate{
		EstimatedRows:  &rows,
		EstimatedRatio: &ratio,
		RiskLevel:      spec.ImpactRiskMedium,
		Confidence:     spec.ImpactConfidenceHigh,
		Source:         spec.ImpactSourcePlan,
		ReasonCodes:    []string{"planner_estimate"},
	}, nil
}

func TestAuditSQLToolSupportsPostgreSQLMetadataAwareMode(t *testing.T) {
	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}
	prepareMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Dialect != spec.DialectPostgreSQL {
			t.Fatalf("expected postgresql dialect hint to flow into shared prepare, got %#v", request.Connection)
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

func TestAuditSQLToolPostgreSQLAlterColumnActionsMapToSemanticRules(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "alter table users alter column created_at set default now(), alter column updated_at drop default, alter column email set not null, alter column phone drop not null;",
			"dialect": "postgresql",
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
	counts := map[string]int{}
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		counts[ruleID]++
	}
	if len(findings) != 8 {
		t.Fatalf("expected exactly 8 alter-column findings, got %#v", findings)
	}
	if counts["ddl.alter.set_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected set_default semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.drop_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected drop_default semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.set_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected set_not_null semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.drop_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected drop_not_null semantic finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLValidateConstraintReturnsNormalResult(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "alter table users validate constraint chk_amount;",
			"dialect": "postgresql",
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
	if statement["kind"] != "ddl" {
		t.Fatalf("expected ddl kind, got %#v", statement["kind"])
	}
	findings, _ := statement["findings"].([]any)
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("validate_constraint should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
}

func TestAuditSQLToolPostgreSQLAlterColumnSetDefaultReturnsNormalResult(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "alter table users alter column status set default 'active';",
			"dialect": "postgresql",
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
		if ok && finding["rule_id"] == "ddl.alter.set_default.explicit_default_change.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected set_default semantic finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLSetDataTypeMapsToForbidRule(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "alter table users alter column status type bigint;",
			"dialect": "postgresql",
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
	counts := make(map[string]int)
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected finding object, got %#v", item)
		}
		ruleID, _ := finding["rule_id"].(string)
		counts[ruleID]++
	}
	if counts["ddl.alter.set_data_type.forbid"] != 1 {
		t.Fatalf("expected set_data_type forbid finding, got %#v", findings)
	}
	if counts["ddl.pg.alter.set_data_type.rewrite.warn"] != 1 {
		t.Fatalf("expected pg set_data_type rewrite warning, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLRenameIndexMapsToForbidRule(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "alter index idx_old rename to idx_new;",
			"dialect": "postgresql",
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
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 rename_index finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.alter.rename_index.forbid" {
		t.Fatalf("expected rename_index forbid finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLCreateViewMapsToForbidRule(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "create view public.active_users as select id from public.users;",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %#v", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
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
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 create_view finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.view.create.forbid" {
		t.Fatalf("expected create_view forbid finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLDropViewMapsToForbidRule(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "drop view if exists public.active_users;",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %#v", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
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
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 drop_view finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.view.drop.forbid" {
		t.Fatalf("expected drop_view forbid finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLCreateTableConstraintsReturnNormalResult(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]string{
		"named table-level CHECK":       "create table orders (id bigint primary key, amount numeric, constraint chk_orders_amount check (amount > 0));",
		"column-level inline CHECK":     "create table orders (id bigint primary key, amount numeric check (amount > 0));",
		"named table-level UNIQUE":      "create table users (id bigint primary key, email text, constraint uq_users_email unique (email));",
		"column-level inline UNIQUE":    "create table users (id bigint primary key, email text unique);",
		"named table-level FOREIGN KEY": "create table orders (id bigint primary key, user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));",
		"column-level inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result, got tool error: %#v", result)
			}
			body := result.StructuredContent.(map[string]any)
			contextValue, ok := body["context"].(map[string]any)
			if !ok || contextValue["mode"] != "offline" {
				t.Fatalf("expected offline context, got %#v", body["context"])
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			if statement["kind"] != "ddl" {
				t.Fatalf("expected ddl kind, got %#v", statement["kind"])
			}
			unsupported, ok := body["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLCreateTableForeignKeyRendersForbidFinding(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]string{
		"named FOREIGN KEY":  "create table orders (id bigint primary key, user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		"inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     sql,
					"dialect": "postgresql",
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
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if ok && finding["rule_id"] == "ddl.table.foreign_key.forbid" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLCreateTableBoundaryReturnsUnsupportedError(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]struct {
		sql     string
		feature string
	}{
		"identity": {
			sql:     "CREATE TABLE users (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, email text);",
			feature: "generated_as_identity",
		},
		"generated_stored": {
			sql:     "CREATE TABLE users (first_name text, last_name text, full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED);",
			feature: "generated_column",
		},
		"exclusion": {
			sql:     "CREATE TABLE bookings (room_id int, during tsrange, EXCLUDE USING gist (room_id WITH =, during WITH &&));",
			feature: "exclusion_constraint",
		},
		"partitioned": {
			sql:     "CREATE TABLE events (id bigint, created_at timestamptz NOT NULL) PARTITION BY RANGE (created_at);",
			feature: "partitioning",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     tc.sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected tool error for unsupported %s, got success result", name)
			}
			body, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("expected structured content, got %#v", result.StructuredContent)
			}
			code, _ := body["code"].(string)
			if code == "" {
				t.Fatalf("expected error code in tool error, got %#v", body)
			}
			message, _ := body["message"].(string)
			if !strings.Contains(message, "unsupported") {
				t.Fatalf("expected unsupported in error message, got %q", message)
			}
		})
	}
}
