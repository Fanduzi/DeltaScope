//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
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
		t.Fatalf("expected pass verdict (ddl.pg.drop_index.advisory fires at notice level, which does not elevate verdict), got %#v", body["verdict"])
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
	version       string
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
	version := c.version
	if version == "" {
		version = "PostgreSQL 16.3"
	}
	return &spec.InstanceFacts{Version: version}, nil
}

func (c *mcpMetadataAuditTestClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, _ string, table string) (*spec.TableSnapshot, error) {
	c.tableCalls = append(c.tableCalls, table)
	if c.snapshot != nil {
		return c.snapshot, nil
	}
	return &spec.TableSnapshot{Options: map[string]string{"cache_status": "disable"}, Exists: true}, nil
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

func TestMySQLDropIndexDependencyThroughMCP(t *testing.T) {
	client := &mcpMetadataAuditTestClient{version: "8.0.46", detectDialect: spec.DialectMySQL, snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Schema: "app", Name: "child"},
		Columns:     []spec.Column{{Name: "column_id", Type: "int"}},
		Indexes:     []spec.Index{{Name: "idx_child", Kind: spec.IndexKindSecondary, Columns: []string{"column_id"}}},
		Constraints: []spec.Constraint{{Type: "foreign_key", Name: "private_fk", Columns: []string{"column_id"}, ReferencedTable: "parent", ReferencedColumns: []string{"id"}}},
	}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectMySQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "DROP INDEX idx_child ON app.child", "dialect": "mysql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body := requireAuditStructuredMap(t, result)
	statements := body["statements"].([]any)
	for _, item := range statements[0].(map[string]any)["findings"].([]any) {
		if item.(map[string]any)["rule_id"] == "ddl.drop_index.foreign_key.warn" {
			return
		}
	}
	t.Fatal("missing FK index warning")
}
