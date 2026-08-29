// Package audit verifies metadata-aware DML table-existence behavior.
// input: MySQL/TiDB DML SQL and metadata providers returning target-table snapshots
// output: regression coverage for the shared missing-target blocker contract
// pos: application audit seam for metadata-enriched DML rule evaluation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditMetadataDMLMissingTargetTableBlocksMySQLAndTiDB(t *testing.T) {
	t.Parallel()

	cases := []struct {
		dialect spec.Dialect
		sql     string
	}{
		{dialect: spec.DialectMySQL, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: spec.DialectMySQL, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: spec.DialectMySQL, sql: "DELETE FROM missing_users WHERE id = 1"},
		{dialect: spec.DialectTiDB, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: spec.DialectTiDB, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: spec.DialectTiDB, sql: "DELETE FROM missing_users WHERE id = 1"},
	}

	for _, tc := range cases {
		t.Run(string(tc.dialect)+"/"+tc.sql[:6], func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:              tc.sql,
				Dialect:          tc.dialect,
				Schema:           "app",
				MetadataProvider: &fakeMetadataProvider{snapshot: &spec.TableSnapshot{Schema: "app", Exists: false}},
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}

			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "dml.table.exists.require" {
					return
				}
			}
			t.Fatalf("expected dml.table.exists.require finding, got %#v", result.Statements[0].Findings)
		})
	}
}

func TestAuditMetadataDMLUsesQualifiedTargetSchema(t *testing.T) {
	t.Parallel()

	provider := &dmlTableMetadataProvider{
		snapshots: map[string]*spec.TableSnapshot{
			"tenant_a.missing_users": {Schema: "tenant_a", Exists: false},
			"app.missing_users":      {Schema: "app", Exists: true},
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "UPDATE tenant_a.missing_users SET name = 'x' WHERE id = 1",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.snapshotCalls) != 1 || provider.snapshotCalls[0] != "tenant_a.missing_users" {
		t.Fatalf("snapshot calls = %#v, want tenant_a.missing_users", provider.snapshotCalls)
	}
	if !hasDMLTableExistenceFinding(result.Statements[0].Findings) {
		t.Fatalf("expected missing qualified target finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditMetadataDMLUsesResolvedSchemaForUnqualifiedTarget(t *testing.T) {
	t.Parallel()

	provider := &dmlTableMetadataProvider{
		snapshots: map[string]*spec.TableSnapshot{
			"app.missing_users": {Schema: "app", Exists: false},
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "DELETE FROM missing_users WHERE id = 1",
		Dialect:          spec.DialectTiDB,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.snapshotCalls) != 1 || provider.snapshotCalls[0] != "app.missing_users" {
		t.Fatalf("snapshot calls = %#v, want app.missing_users", provider.snapshotCalls)
	}
	if !hasDMLTableExistenceFinding(result.Statements[0].Findings) {
		t.Fatalf("expected missing unqualified target finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditMetadataDMLDoesNotCheckInsertSelectSourceTables(t *testing.T) {
	t.Parallel()

	provider := &dmlTableMetadataProvider{
		snapshots: map[string]*spec.TableSnapshot{
			"app.archive_users": {Schema: "app", Exists: true},
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "INSERT INTO archive_users SELECT id FROM missing_source_users",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.snapshotCalls) != 1 || provider.snapshotCalls[0] != "app.archive_users" {
		t.Fatalf("snapshot calls = %#v, want only app.archive_users", provider.snapshotCalls)
	}
	if hasDMLTableExistenceFinding(result.Statements[0].Findings) {
		t.Fatalf("source-table absence must not produce a target existence finding: %#v", result.Statements[0].Findings)
	}
}

func TestAuditMetadataDMLLookupFailureRemainsAnError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("metadata lookup timed out")
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "DELETE FROM users WHERE id = 1",
		Dialect:          spec.DialectTiDB,
		Schema:           "app",
		MetadataProvider: &dmlTableMetadataProvider{snapshotErr: wantErr},
	})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected metadata lookup error %v, got result=%#v err=%v", wantErr, result, err)
	}
}

type dmlTableMetadataProvider struct {
	snapshots     map[string]*spec.TableSnapshot
	snapshotCalls []string
	snapshotErr   error
}

func (p *dmlTableMetadataProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *dmlTableMetadataProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	key := schema + "." + table
	p.snapshotCalls = append(p.snapshotCalls, key)
	if p.snapshotErr != nil {
		return nil, p.snapshotErr
	}
	return p.snapshots[key], nil
}

func hasDMLTableExistenceFinding(findings []rule.Finding) bool {
	for _, finding := range findings {
		if finding.RuleID == "dml.table.exists.require" {
			return true
		}
	}
	return false
}
