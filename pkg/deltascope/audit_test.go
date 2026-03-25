// Package deltascope verifies the public library audit API.
// input: inline SQL text, optional YAML policy overrides, and the public request contract
// output: regression coverage for public audit orchestration and stable result mapping
// pos: public API test coverage for pkg/deltascope
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataProvider struct {
	instanceCalls int
	tableCalls    []string
	instance      *InstanceFacts
	snapshot      *TableSnapshot
	err           error
}

func (f *fakeMetadataProvider) LoadInstanceFacts(_ context.Context, _ Dialect, _ string) (*InstanceFacts, error) {
	f.instanceCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.instance, nil
}

func (f *fakeMetadataProvider) LoadTableSnapshot(_ context.Context, _ Dialect, _ string, table string) (*TableSnapshot, error) {
	f.tableCalls = append(f.tableCalls, table)
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func TestAuditUsesDefaultPolicy(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "update users set name = 'delta';",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 1 {
		t.Fatalf("expected 1 blocker, got %#v", result.Summary)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("expected 1 statement finding, got %d", len(result.Statements[0].Findings))
	}
	if result.Statements[0].Findings[0].RuleID != "dml.where.require" {
		t.Fatalf("expected where rule finding, got %q", result.Statements[0].Findings[0].RuleID)
	}
}

func TestAuditSupportsConfigOverridePath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	config := "rules:\n  dml.where.require:\n    enabled: false\n    level: blocker\n    params:\n      required: true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Audit(context.Background(), Request{
		SQL:        "update users set name = 'delta';",
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictPass {
		t.Fatalf("expected pass verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("expected empty findings after disabling rule, got %#v", result.Summary)
	}
}

func TestAuditReturnsGroupedMultiStatementResults(t *testing.T) {
	sql := "create table users (id bigint); update users set name = 'delta';"

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected first statement kind ddl, got %q", result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != "dml" {
		t.Fatalf("expected second statement kind dml, got %q", result.Statements[1].Kind)
	}
	if len(result.Statements[0].Findings) == 0 {
		t.Fatalf("expected first statement findings to be grouped")
	}
	if len(result.Statements[1].Findings) == 0 {
		t.Fatalf("expected second statement findings to be grouped")
	}
	if result.Statements[0].Findings[0].StatementIndex != 0 {
		t.Fatalf("expected first statement finding index 0, got %d", result.Statements[0].Findings[0].StatementIndex)
	}
	if result.Statements[1].Findings[0].StatementIndex != 1 {
		t.Fatalf("expected second statement finding index 1, got %d", result.Statements[1].Findings[0].StatementIndex)
	}
}

func TestAuditRejectsUnsupportedDialect(t *testing.T) {
	_, err := Audit(context.Background(), Request{
		SQL:     "select 1;",
		Dialect: Dialect("postgres"),
	})
	if err == nil {
		t.Fatal("expected unsupported dialect error")
	}
}

func TestAuditSupportsTopLevelMetadataRequestFields(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance: &InstanceFacts{
			Version:                "8.0.36",
			DefaultCharset:         "utf8mb4",
			InnoDBDefaultRowFormat: "dynamic",
		},
		snapshot: &TableSnapshot{
			Exists: true,
			Table:  &Table{Name: "users"},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users add column email varchar(255);",
		Dialect:          DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit with metadata fields: %v", err)
	}

	if provider.instanceCalls != 1 {
		t.Fatalf("expected one instance-facts call, got %d", provider.instanceCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 || result.Statements[0].Findings == nil {
		t.Fatalf("expected statement result, got %#v", result.Statements)
	}
}

func TestFromDomainResultMapsTopLevelExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Verdict: report.VerdictPass,
		Explanation: &report.Explanation{
			Summary: "result explanation",
			Reasons: []string{"shared context"},
		},
	})

	if public.Explanation == nil {
		t.Fatal("expected top-level explanation to be mapped")
	}
	if public.Explanation.Summary != "result explanation" {
		t.Fatalf("expected top-level explanation summary to be mapped, got %#v", public.Explanation)
	}
	if len(public.Explanation.Reasons) != 1 || public.Explanation.Reasons[0] != "shared context" {
		t.Fatalf("expected top-level explanation reasons to be mapped, got %#v", public.Explanation)
	}
}

func TestFromDomainResultMapsStatementExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Explanation: &report.Explanation{
				Summary: "statement explanation",
				Reasons: []string{"missing predicate"},
			},
		}},
	})

	if len(public.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(public.Statements))
	}
	if public.Statements[0].Explanation == nil {
		t.Fatal("expected statement explanation to be mapped")
	}
	if public.Statements[0].Explanation.Summary != "statement explanation" {
		t.Fatalf("expected statement explanation summary to be mapped, got %#v", public.Statements[0].Explanation)
	}
	if len(public.Statements[0].Explanation.Reasons) != 1 || public.Statements[0].Explanation.Reasons[0] != "missing predicate" {
		t.Fatalf("expected statement explanation reasons to be mapped, got %#v", public.Statements[0].Explanation)
	}
}

func TestAuditBuildsAggregateExplanations(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "update users set name = 'delta';",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Explanation == nil {
		t.Fatal("expected result explanation to be populated")
	}
	if result.Explanation.Summary == "" || len(result.Explanation.Reasons) == 0 {
		t.Fatalf("expected result explanation content, got %#v", result.Explanation)
	}
	if len(result.Statements) != 1 || result.Statements[0].Explanation == nil {
		t.Fatalf("expected statement explanation to be populated, got %#v", result.Statements)
	}
	if result.Statements[0].Explanation.Summary == "" || len(result.Statements[0].Explanation.Reasons) == 0 {
		t.Fatalf("expected statement explanation content, got %#v", result.Statements[0].Explanation)
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("expected verdict semantics unchanged, got %q", result.Verdict)
	}
}

func TestFromDomainResultMapsFindingExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
				Explanation: &rule.FindingExplanation{
					Summary:    "Require DML where",
					Why:        "The statement is missing a clause that the shipped policy requires.",
					Risk:       "Ignoring this rule can allow high-impact data changes.",
					Suggestion: "Add a WHERE clause.",
					Metadata: &rule.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	})

	finding := public.Statements[0].Findings[0]
	if finding.Explanation == nil {
		t.Fatal("expected finding explanation to be mapped")
	}
	if finding.Explanation.Why == "" || finding.Explanation.Risk == "" || finding.Explanation.Suggestion == "" {
		t.Fatalf("expected mapped finding explanation fields, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata == nil || finding.Explanation.Metadata.Status != "limited" {
		t.Fatalf("expected mapped finding metadata note, got %#v", finding.Explanation)
	}
}

func TestFromDomainResultDeepCopiesFindingMetadata(t *testing.T) {
	domainMetadata := map[string]any{
		"operation": "delete",
	}
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
				Metadata: domainMetadata,
			}},
		}},
	})

	public.Statements[0].Findings[0].Metadata["operation"] = "mutated"
	if domainMetadata["operation"] != "delete" {
		t.Fatalf("expected public finding metadata to be defensive copied, got %#v", domainMetadata)
	}
}

func TestFromDomainResultDeepCopiesNestedFindingMetadata(t *testing.T) {
	domainMetadata := map[string]any{
		"labels":        []string{"delete", "high-risk"},
		"details":       map[string]any{"table": "users"},
		"label_groups":  [][]string{{"delete", "high-risk"}},
		"detail_groups": []map[string]any{{"table": "users"}},
	}
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause required",
				Metadata: domainMetadata,
			}},
		}},
	})

	publicLabels := public.Statements[0].Findings[0].Metadata["labels"].([]string)
	publicLabels[0] = "mutated"
	publicDetails := public.Statements[0].Findings[0].Metadata["details"].(map[string]any)
	publicDetails["table"] = "mutated_users"
	publicLabelGroups := public.Statements[0].Findings[0].Metadata["label_groups"].([][]string)
	publicLabelGroups[0][0] = "mutated_group"
	publicDetailGroups := public.Statements[0].Findings[0].Metadata["detail_groups"].([]map[string]any)
	publicDetailGroups[0]["table"] = "mutated_group_users"

	if domainMetadata["labels"].([]string)[0] != "delete" {
		t.Fatalf("expected nested metadata slice to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["details"].(map[string]any)["table"] != "users" {
		t.Fatalf("expected nested metadata map to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["label_groups"].([][]string)[0][0] != "delete" {
		t.Fatalf("expected nested string matrix to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["detail_groups"].([]map[string]any)[0]["table"] != "users" {
		t.Fatalf("expected nested map slice to be defensive copied, got %#v", domainMetadata)
	}
}

func TestPublicMetadataProviderClonesProviderResults(t *testing.T) {
	instance := &InstanceFacts{
		Version:                "8.0.36",
		DefaultCharset:         "utf8mb4",
		InnoDBDefaultRowFormat: "dynamic",
	}
	snapshot := &TableSnapshot{
		Schema:     "app",
		Exists:     true,
		Table:      &Table{Name: "users"},
		PrimaryKey: &Index{Name: "PRIMARY", Columns: []string{"id"}},
		Columns: []Column{
			{Name: "id", Type: "bigint"},
		},
		Indexes: []Index{{Name: "idx_email", Columns: []string{"email"}}},
		Constraints: []Constraint{{Name: "fk_team", Columns: []string{"team_id"}}},
		Options: map[string]string{"engine": "InnoDB"},
	}
	provider := publicMetadataProvider{provider: &fakeMetadataProvider{instance: instance, snapshot: snapshot}}

	mappedInstance, err := provider.LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
	if err != nil {
		t.Fatalf("load instance facts: %v", err)
	}
	mappedSnapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "users")
	if err != nil {
		t.Fatalf("load table snapshot: %v", err)
	}

	mappedInstance.DefaultCharset = "latin1"
	mappedSnapshot.Table.Name = "mutated_users"
	mappedSnapshot.Columns[0].Name = "mutated_id"
	mappedSnapshot.PrimaryKey.Columns[0] = "mutated_pk"
	mappedSnapshot.Indexes[0].Columns[0] = "mutated_email"
	mappedSnapshot.Constraints[0].Columns[0] = "mutated_team"
	mappedSnapshot.Options["engine"] = "MyISAM"

	if instance.DefaultCharset != "utf8mb4" {
		t.Fatalf("expected cloned instance facts, got %#v", instance)
	}
	if snapshot.Table == nil || snapshot.Table.Name != "users" {
		t.Fatalf("expected cloned table pointer, got %#v", snapshot.Table)
	}
	if len(snapshot.Columns) != 1 || snapshot.Columns[0].Name != "id" {
		t.Fatalf("expected cloned columns slice, got %#v", snapshot.Columns)
	}
	if snapshot.PrimaryKey == nil || len(snapshot.PrimaryKey.Columns) != 1 || snapshot.PrimaryKey.Columns[0] != "id" {
		t.Fatalf("expected cloned primary key columns, got %#v", snapshot.PrimaryKey)
	}
	if len(snapshot.Indexes) != 1 || len(snapshot.Indexes[0].Columns) != 1 || snapshot.Indexes[0].Columns[0] != "email" {
		t.Fatalf("expected cloned index columns, got %#v", snapshot.Indexes)
	}
	if len(snapshot.Constraints) != 1 || len(snapshot.Constraints[0].Columns) != 1 || snapshot.Constraints[0].Columns[0] != "team_id" {
		t.Fatalf("expected cloned constraint columns, got %#v", snapshot.Constraints)
	}
	if snapshot.Options["engine"] != "InnoDB" {
		t.Fatalf("expected cloned options map, got %#v", snapshot.Options)
	}
}
