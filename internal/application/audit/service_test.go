// Package audit verifies the application audit service behavior.
// input: audit service requests with SQL, dialect, and optional config path
// output: end-to-end application audit coverage over policy loading, extraction, and rules
// pos: application audit service test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataProvider struct {
	instanceCalls int
	tableCalls    []string
	instance      *spec.InstanceFacts
	snapshot      *spec.TableSnapshot
	err           error
}

func (f *fakeMetadataProvider) LoadInstanceFacts(_ context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	f.instanceCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.instance, nil
}

func (f *fakeMetadataProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, _ string, table string) (*spec.TableSnapshot, error) {
	f.tableCalls = append(f.tableCalls, table)
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func TestAuditSQLUsesDefaultPolicy(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "delete from users",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) == 0 {
		t.Fatal("expected statement findings from default policy")
	}
}

func TestAuditSQLAppliesConfigOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	content := []byte(`
rules:
  dml.where.require:
    enabled: false
    level: notice
    params:
      required: false
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "delete from users",
		Dialect:    spec.DialectMySQL,
		ConfigPath: path,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if result.Verdict != report.VerdictPass {
		t.Fatalf("expected pass verdict after disabling WHERE rule, got %q", result.Verdict)
	}
}

func TestAuditSQLReturnsGroupedStatementResults(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "delete from users; update accounts set active = 0",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statement results, got %d", len(result.Statements))
	}
	if result.Summary.Statements != 2 {
		t.Fatalf("expected summary statements=2, got %d", result.Summary.Statements)
	}
}

func TestAuditSQLUsesTopLevelMetadataRequestFields(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{
			Version:                "8.0.36",
			DefaultCharset:         "utf8mb4",
			InnoDBDefaultRowFormat: "dynamic",
		},
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users add column email varchar(255)",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql with top-level metadata fields: %v", err)
	}

	if provider.instanceCalls != 1 {
		t.Fatalf("expected one instance-facts call, got %d", provider.instanceCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", result.Statements)
	}
}

func TestEnrichStatementsWithMetadataAddsInstanceAndTargetTableFacts(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{
			Version:                "8.0.36",
			DefaultCharset:         "utf8mb4",
			InnoDBDefaultRowFormat: "dynamic",
		},
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "id", Type: "bigint"},
			},
		},
	}

	statements := []spec.Statement{
		{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
			},
		},
	}

	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL, &MetadataRequest{
		Schema:   "app",
		Provider: provider,
	}, statements)
	if err != nil {
		t.Fatalf("enrich statements: %v", err)
	}

	if provider.instanceCalls != 1 {
		t.Fatalf("expected one instance-facts call, got %d", provider.instanceCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table call for users, got %#v", provider.tableCalls)
	}
	if enriched[0].Metadata == nil || enriched[0].Metadata.Instance == nil {
		t.Fatalf("expected instance metadata to be attached")
	}
	if enriched[0].Metadata.Schema != "app" {
		t.Fatalf("expected schema context app, got %#v", enriched[0].Metadata)
	}
	if enriched[0].Metadata.TargetTable == nil || !enriched[0].Metadata.TargetTable.Exists {
		t.Fatalf("expected table snapshot metadata to be attached")
	}
}

func TestEnrichStatementsWithMetadataPreservesSchemaContextWithoutProvider(t *testing.T) {
	statements := []spec.Statement{
		{Kind: spec.KindDML, Dialect: spec.DialectMySQL, DML: &spec.DML{Operation: spec.DMLOperationDelete, Tables: []spec.Table{{Name: "users"}}}},
	}

	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL, &MetadataRequest{Schema: "app"}, statements)
	if err != nil {
		t.Fatalf("enrich statements with schema only: %v", err)
	}
	if enriched[0].Metadata == nil || enriched[0].Metadata.Schema != "app" {
		t.Fatalf("expected schema context without provider, got %#v", enriched[0].Metadata)
	}
	if enriched[0].Metadata.Instance != nil || enriched[0].Metadata.TargetTable != nil {
		t.Fatalf("expected schema-only metadata, got %#v", enriched[0].Metadata)
	}
}

func TestEnrichStatementsWithMetadataKeepsOfflinePathWhenProviderIsAbsent(t *testing.T) {
	statements := []spec.Statement{
		{Kind: spec.KindDDL, Dialect: spec.DialectMySQL, DDL: &spec.DDL{Table: &spec.Table{Name: "users"}}},
	}

	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL, nil, statements)
	if err != nil {
		t.Fatalf("enrich statements without provider: %v", err)
	}
	if enriched[0].Metadata != nil {
		t.Fatalf("expected offline path to keep metadata nil, got %#v", enriched[0].Metadata)
	}
}
