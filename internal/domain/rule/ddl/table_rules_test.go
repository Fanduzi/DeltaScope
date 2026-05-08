// Package ddl verifies Tier-1 DDL table-level rule behavior.
// input: create-table Statement specs and rule policy overrides
// output: deterministic findings for table comment and table-name rules
// pos: domain DDL rule test coverage for table naming and comments
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableCommentRequiredRuleFindsMissingComment(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableCommentRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("users", "", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Message != "table comment is required" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestTableCommentRequiredRuleIgnoresAlterStatements(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableCommentRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "add_columns", Name: "age"}},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for alter table, got %d", len(findings))
	}
}

func TestTableNameMaxLengthRuleFindsLongNames(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableNameMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("orders_archive", "archive table", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelBlocker {
		t.Fatalf("expected blocker level, got %q", findings[0].Level)
	}
	if findings[0].Message != "table name must not exceed 5 characters" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
	if findings[0].Metadata["actual"] != len("orders_archive") {
		t.Fatalf("expected actual length metadata, got %#v", findings[0].Metadata["actual"])
	}
}

func TestTableNameMaxLengthRuleAcceptsBoundaryLength(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableNameMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("users", "user table", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings at boundary, got %d", len(findings))
	}
}

func TestTableNameGovernanceRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tableName  string
		rules      map[string]policy.RulePolicy
		wantIDs    []string
		wantCount  int
		wantAbsent bool
	}{
		{
			name:      "prefix suffix and contains each emit one finding",
			tableName: "users",
			rules: map[string]policy.RulePolicy{
				"ddl.table.name.prefix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "tbl_"},
				},
				"ddl.table.name.suffix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "_table"},
				},
				"ddl.table.name.contains.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"order", "account"}},
				},
			},
			wantIDs:   []string{"ddl.table.name.prefix.require", "ddl.table.name.suffix.require", "ddl.table.name.contains.require"},
			wantCount: 3,
		},
		{
			name:      "inactive constraints stay quiet",
			tableName: "users",
			rules: map[string]policy.RulePolicy{
				"ddl.table.name.prefix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": ""},
				},
				"ddl.table.name.suffix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "   "},
				},
				"ddl.table.name.contains.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{" ", ""}},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			findings := evaluateDDLFindings(t, policy.Policy{Rules: tt.rules}, createTableStatement(tt.tableName, "user table", []string{"id"}))

			if len(findings) != tt.wantCount {
				t.Fatalf("expected %d findings, got %d", tt.wantCount, len(findings))
			}
			for i, wantID := range tt.wantIDs {
				if findings[i].RuleID != wantID {
					t.Fatalf("expected finding %d to use rule %q, got %q", i, wantID, findings[i].RuleID)
				}
			}
		})
	}
}

func createTableStatement(name string, comment string, primaryKeyColumns []string) spec.Statement {
	var primaryKey *spec.Index
	if len(primaryKeyColumns) > 0 {
		primaryKey = &spec.Index{
			Name:    "primary",
			Columns: append([]string(nil), primaryKeyColumns...),
		}
	}

	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{
				Name:    name,
				Comment: comment,
			},
			PrimaryKey: primaryKey,
		},
	}
}

func evaluateDDLFindings(t *testing.T, cfg policy.Policy, statement spec.Statement) []rule.Finding {
	t.Helper()

	registry := rule.NewRegistry()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return findings
}
