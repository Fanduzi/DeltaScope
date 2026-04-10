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
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const postgreSQLSyntaxNoticeRuleID = "dialect.postgresql.syntax.detected.notice"

type fakeMetadataProvider struct {
	instanceCalls int
	tableCalls    []string
	plannerCalls  int
	indexCalls    []string
	indexSchemas  []string
	indexDialects []spec.Dialect
	indexTable    string
	instance      *spec.InstanceFacts
	snapshot      *spec.TableSnapshot
	planner       *spec.ImpactEstimate
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

func (f *fakeMetadataProvider) LoadPlanEstimate(_ context.Context, _ spec.Statement) (*spec.ImpactEstimate, error) {
	f.plannerCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.planner, nil
}

func (f *fakeMetadataProvider) ResolveTableForIndex(_ context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	f.indexCalls = append(f.indexCalls, index)
	f.indexDialects = append(f.indexDialects, dialect)
	f.indexSchemas = append(f.indexSchemas, schema)
	if f.err != nil {
		return "", f.err
	}
	return f.indexTable, nil
}

func assertHasPostgreSQLSyntaxNotice(t *testing.T, result report.Result, token string) {
	t.Helper()
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %#v", result.GlobalFindings)
	}
	finding := result.GlobalFindings[0]
	if finding.RuleID != postgreSQLSyntaxNoticeRuleID {
		t.Fatalf("expected rule id %q, got %#v", postgreSQLSyntaxNoticeRuleID, finding)
	}
	if finding.Level != rule.LevelNotice {
		t.Fatalf("expected notice-level finding, got %#v", finding)
	}
	if !strings.Contains(strings.ToLower(finding.Message), "sql looks like postgresql") {
		t.Fatalf("expected PostgreSQL syntax notice message, got %#v", finding)
	}
	if !strings.Contains(finding.Message, "--dialect postgresql") {
		t.Fatalf("expected explicit dialect guidance, got %#v", finding)
	}
	if finding.Explanation == nil {
		t.Fatalf("expected explanation, got %#v", finding)
	}
	if !strings.Contains(strings.ToLower(finding.Explanation.Why), "postgresql") {
		t.Fatalf("expected why explanation, got %#v", finding.Explanation)
	}
	if !strings.Contains(strings.ToLower(finding.Explanation.Risk), "misleading") {
		t.Fatalf("expected misleading risk explanation, got %#v", finding.Explanation)
	}
	if !strings.Contains(finding.Explanation.Suggestion, "--dialect postgresql") {
		t.Fatalf("expected suggestion explanation, got %#v", finding.Explanation)
	}
	if finding.Metadata["token"] != token {
		t.Fatalf("expected token metadata %q, got %#v", token, finding.Metadata)
	}
}

func TestAuditPostgreSQLSyntaxNoticeProvidesExplicitTrustGuidance(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "insert into users(id) values (1) returning id;",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(result.GlobalFindings))
	}
	finding := result.GlobalFindings[0]
	if finding.RuleID != postgreSQLSyntaxNoticeRuleID {
		t.Fatalf("expected rule id %q, got %q", postgreSQLSyntaxNoticeRuleID, finding.RuleID)
	}
	if finding.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", finding.Level)
	}
	if finding.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}

	// Trust: DeltaScope does not auto-switch dialect
	if !strings.Contains(strings.ToLower(finding.Explanation.Risk), "does not auto-switch") {
		t.Fatalf("expected risk to mention no auto-switch, got %q", finding.Explanation.Risk)
	}

	// Suggestion must say: if PG, use --dialect postgresql; if not, ignore
	sug := finding.Explanation.Suggestion
	if !strings.Contains(sug, "--dialect postgresql") {
		t.Fatalf("expected suggestion to mention --dialect postgresql, got %q", sug)
	}
	if !strings.Contains(strings.ToLower(sug), "if this sql targets postgresql") {
		t.Fatalf("expected suggestion to mention targeting postgresql, got %q", sug)
	}
	if !strings.Contains(strings.ToLower(sug), "ignore") {
		t.Fatalf("expected suggestion to mention ignoring the notice, got %q", sug)
	}

	// Message must carry --dialect postgresql
	if !strings.Contains(finding.Message, "--dialect postgresql") {
		t.Fatalf("expected message to carry --dialect postgresql, got %q", finding.Message)
	}
}

func assertHasNoPostgreSQLSyntaxNotice(t *testing.T, result report.Result) {
	t.Helper()
	for _, finding := range result.GlobalFindings {
		if finding.RuleID == postgreSQLSyntaxNoticeRuleID {
			t.Fatalf("did not expect PostgreSQL syntax notice, got %#v", finding)
		}
	}
}

func TestAuditSQLAcceptsPostgreSQLAtValidationBoundary(t *testing.T) {
	_, err := AuditSQL(context.Background(), Request{
		SQL:     "select 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected postgresql parse stub error")
	}
	if err == ErrUnknownDialect {
		t.Fatal("expected postgresql dialect to pass validation")
	}
}

func TestAuditAddsPostgreSQLSyntaxNoticeForReturning(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "insert into users(id) values (1) returning id;",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	assertHasPostgreSQLSyntaxNotice(t, result, "RETURNING")
	if result.Verdict == report.VerdictPass {
		t.Fatalf("expected parse-error partial result to avoid pass verdict, got %#v", result)
	}
}

func TestAuditAddsPostgreSQLSyntaxNoticeForOnConflict(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "insert into users(id) values (1) on conflict (id) do nothing;",
		Dialect: spec.DialectTiDB,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	assertHasPostgreSQLSyntaxNotice(t, result, "ON CONFLICT")
}

func TestAuditAddsPostgreSQLSyntaxNoticeForTypeCast(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "select id::bigint from users;",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	assertHasPostgreSQLSyntaxNotice(t, result, "::")
}

func TestAuditAddsPostgreSQLSyntaxNoticeForAlterColumnTypeUsing(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users alter column score type bigint using abs(score);",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	assertHasPostgreSQLSyntaxNotice(t, result, "ALTER COLUMN TYPE USING")
}

func TestAuditSQLMySQLSerialDoesNotAddGlobalNotice(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table users (id serial primary key);",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
	if result.RuleSummary == nil || result.RuleSummary.Loaded == 0 {
		t.Fatalf("expected rule summary to be preserved, got %#v", result.RuleSummary)
	}
}

func TestAuditSQLTiDBSerialDoesNotAddGlobalNotice(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table users (id serial primary key);",
		Dialect: spec.DialectTiDB,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeWhenDialectExplicitlyPostgreSQL(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "insert into users(id) values (1) returning id;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "dialect mismatch") {
		t.Fatalf("did not expect dialect mismatch hint on postgresql path, got %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeForMySQLSQL(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table users (id bigint primary key) comment='users';",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeForTokenInsideStringLiteral(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "select 'returning' as note;",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeForTokenInsideBlockComment(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "select 1 /* returning */;",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeForTokenInsideLineComment(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "select 1 -- returning\n;",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
}

func TestAuditDoesNotAddPostgreSQLSyntaxNoticeForTokenInsideBacktickIdentifier(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "select `returning` from users;",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	assertHasNoPostgreSQLSyntaxNotice(t, result)
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

func TestAuditSQLTriggersNamingGovernanceFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	content := []byte(`
rules:
  ddl.table.name.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: tbl_
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "create table users (id bigint, primary key (id)) comment='users';",
		Dialect:    spec.DialectMySQL,
		ConfigPath: path,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if len(result.Statements) != 1 || len(result.Statements[0].Findings) == 0 {
		t.Fatalf("expected naming findings, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.name.prefix.require" && finding.Message == `table name "users" must start with "tbl_"` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected naming rule finding, got %#v", result.Statements[0].Findings)
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

func TestMetadataRequestForMergesTopLevelAndLegacyFields(t *testing.T) {
	legacyProvider := &fakeMetadataProvider{}
	topLevelProvider := &fakeMetadataProvider{}

	tests := []struct {
		name         string
		request      Request
		wantNil      bool
		wantSchema   string
		wantProvider MetadataProvider
	}{
		{
			name:    "returns nil when metadata is absent",
			request: Request{},
			wantNil: true,
		},
		{
			name: "returns legacy metadata when top-level fields are absent",
			request: Request{
				Metadata: &MetadataRequest{Schema: "legacy", Provider: legacyProvider},
			},
			wantSchema:   "legacy",
			wantProvider: legacyProvider,
		},
		{
			name: "merges top-level schema with legacy provider",
			request: Request{
				Schema:   "app",
				Metadata: &MetadataRequest{Schema: "legacy", Provider: legacyProvider},
			},
			wantSchema:   "app",
			wantProvider: legacyProvider,
		},
		{
			name: "merges top-level provider with legacy schema",
			request: Request{
				MetadataProvider: topLevelProvider,
				Metadata:         &MetadataRequest{Schema: "legacy", Provider: legacyProvider},
			},
			wantSchema:   "legacy",
			wantProvider: topLevelProvider,
		},
		{
			name: "top-level fields override legacy per field",
			request: Request{
				Schema:           "app",
				MetadataProvider: topLevelProvider,
				Metadata:         &MetadataRequest{Schema: "legacy", Provider: legacyProvider},
			},
			wantSchema:   "app",
			wantProvider: topLevelProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadataRequestFor(tt.request)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil metadata request, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected metadata request, got nil")
			}
			if got.Schema != tt.wantSchema {
				t.Fatalf("expected schema %q, got %#v", tt.wantSchema, got)
			}
			if got.Provider != tt.wantProvider {
				t.Fatalf("expected provider %#v, got %#v", tt.wantProvider, got.Provider)
			}
		})
	}
}

func TestAuditSQLMergesTopLevelSchemaWithLegacyMetadataProvider(t *testing.T) {
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
		SQL:     "alter table users add column email varchar(255)",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		Metadata: &MetadataRequest{
			Provider: provider,
		},
	})
	if err != nil {
		t.Fatalf("audit sql with mixed metadata request fields: %v", err)
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

func TestAuditSQLMetadataRefinesStatementImpact(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			PrimaryKey: &spec.Index{
				Name:    "PRIMARY",
				Kind:    spec.IndexKindPrimary,
				Columns: []string{"id"},
			},
			Options: map[string]string{
				"table_rows": "100",
			},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "update users set active = 0 where id = 42",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql with metadata refinement: %v", err)
	}

	if len(result.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", result.Statements)
	}

	impact := result.Statements[0].Impact
	if impact == nil {
		t.Fatalf("expected statement impact, got %#v", result.Statements[0])
	}
	if impact.Source != report.ImpactSourceMetadata {
		t.Fatalf("expected metadata-backed impact source, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 1 {
		t.Fatalf("expected one estimated row, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 0.01 {
		t.Fatalf("expected estimated ratio 0.01, got %#v", impact)
	}
}

func TestAuditSQLMetadataRequestProviderAppliesPlannerEstimate(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			PrimaryKey: &spec.Index{
				Name:    "PRIMARY",
				Kind:    spec.IndexKindPrimary,
				Columns: []string{"id"},
			},
			Options: map[string]string{
				"table_rows": "100",
			},
		},
		planner: &spec.ImpactEstimate{
			EstimatedRows: ptrInt64(7),
			RiskLevel:     spec.ImpactRiskMedium,
			Confidence:    spec.ImpactConfidenceMedium,
			Source:        spec.ImpactSourcePlan,
			ReasonCodes:   []string{"explain_rows"},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "update users set active = 0 where id = 42",
		Dialect: spec.DialectMySQL,
		Metadata: &MetadataRequest{
			Schema:   "app",
			Provider: provider,
		},
	})
	if err != nil {
		t.Fatalf("audit sql with metadata request planner: %v", err)
	}
	if provider.plannerCalls != 1 {
		t.Fatalf("expected one planner call, got %d", provider.plannerCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", result.Statements)
	}
	impact := result.Statements[0].Impact
	if impact == nil || impact.Source != report.ImpactSourcePlan {
		t.Fatalf("expected plan-backed impact, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 7 {
		t.Fatalf("expected planner estimated rows 7, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 0.07 {
		t.Fatalf("expected metadata-refined ratio 0.07, got %#v", impact)
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
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
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

func TestEnrichStatementsWithMetadataResolvesStandaloneIndexOwnerWhenAvailable(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance:   &spec.InstanceFacts{Version: "16.2"},
		indexTable: "users",
		snapshot:   &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "users"}},
	}

	statements := []spec.Statement{{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter:     []spec.Alter{{Action: "rename_index", Name: "missing_idx", Options: map[string]string{"new_name": "idx_new"}}},
		},
	}}

	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: provider,
	}, statements)
	if err != nil {
		t.Fatalf("enrich standalone index statements: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner lookup, got %#v", provider.indexCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one resolved table snapshot load, got %#v", provider.tableCalls)
	}
	if enriched[0].Metadata == nil || enriched[0].Metadata.TargetTable == nil || enriched[0].Metadata.TargetTable.Table == nil || enriched[0].Metadata.TargetTable.Table.Name != "users" {
		t.Fatalf("expected resolved target table metadata, got %#v", enriched[0].Metadata)
	}
}

func TestEnrichStatementsWithMetadataSkipsStandaloneIndexOwnerWithoutResolver(t *testing.T) {
	provider := metadataOnlyProvider{
		instance: &spec.InstanceFacts{Version: "16.2"},
	}

	statements := []spec.Statement{{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter:     []spec.Alter{{Action: "rename_index", Name: "missing_idx", Options: map[string]string{"new_name": "idx_new"}}},
		},
	}}

	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: provider,
	}, statements)
	if err != nil {
		t.Fatalf("enrich standalone index statements without resolver: %v", err)
	}
	if enriched[0].Metadata == nil || enriched[0].Metadata.Schema != "public" || enriched[0].Metadata.Instance == nil {
		t.Fatalf("expected schema and instance metadata, got %#v", enriched[0].Metadata)
	}
	if enriched[0].Metadata.TargetTable != nil {
		t.Fatalf("expected no target table without resolver, got %#v", enriched[0].Metadata)
	}
}

type metadataOnlyProvider struct {
	instance *spec.InstanceFacts
	err      error
}

func (p metadataOnlyProvider) LoadInstanceFacts(_ context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.instance, nil
}

func (p metadataOnlyProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, _ string, _ string) (*spec.TableSnapshot, error) {
	if p.err != nil {
		return nil, p.err
	}
	return nil, nil
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

func TestAuditIncludesRuleSummary(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table users (id bigint primary key) comment='users';",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if result.RuleSummary == nil {
		t.Fatal("expected rule summary on result")
	}
	if result.RuleSummary.Loaded == 0 {
		t.Fatal("expected loaded rules count > 0")
	}
	if result.RuleSummary.Applicable == 0 {
		t.Fatal("expected applicable rules count > 0")
	}
	if len(result.RuleSummary.Skipped) == 0 {
		t.Fatal("expected skipped PG rules for MySQL dialect")
	}
	for _, s := range result.RuleSummary.Skipped {
		if s.Reason != "dialect_mismatch" {
			t.Fatalf("expected dialect_mismatch reason, got %s", s.Reason)
		}
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
