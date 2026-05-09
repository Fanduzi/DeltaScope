//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

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
