// Package ddl verifies Tier-1 DDL table-level rule behavior.
// input: create-table Statement specs and rule policy overrides
// output: deterministic findings for table comment and table-name rules
// pos: domain DDL rule test coverage for table naming and comments
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableCommentRequiredRuleFindsMissingComment(t *testing.T) {
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

	findings, err := ruleUnderTest.Evaluate(createTableStatement("users", "", []string{"id"}))
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

	findings, err := ruleUnderTest.Evaluate(spec.Statement{
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

	findings, err := ruleUnderTest.Evaluate(createTableStatement("orders_archive", "archive table", []string{"id"}))
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

	findings, err := ruleUnderTest.Evaluate(createTableStatement("users", "user table", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings at boundary, got %d", len(findings))
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
