// Package ddl verifies create-table identifier governance behavior.
// input: synthetic create-table Statement specs with table, column, and index names plus policy overrides
// output: focused coverage for identifier-pattern and reserved-keyword findings
// pos: domain DDL rule test coverage for identifier and keyword governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestIdentifierPatternRuleFindsIllegalTableName(t *testing.T) {
	statementRule, err := newIdentifierPatternRule(ruleIDTableNamePatternRequire, "table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}, selectTableName)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statementWithNamedObjects("user-profile", nil, nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestIdentifierPatternRuleFindsIllegalColumnName(t *testing.T) {
	statementRule, err := newIdentifierPatternRule(ruleIDColumnNamePatternRequire, "column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}, selectColumnNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statementWithNamedObjects("users", []spec.Column{{Name: "display-name", Type: "varchar(32)", Comment: "'display'"}}, nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestIdentifierPatternRuleFindsUnnamedIndex(t *testing.T) {
	statementRule, err := newIdentifierPatternRule(ruleIDIndexNamePatternRequire, "index", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}, selectIndexNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statementWithNamedObjects("users", nil, []spec.Index{{Name: "", Kind: spec.IndexKindSecondary, Columns: []string{"name"}}}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestIdentifierKeywordRuleFindsReservedKeywords(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		subject   string
		statement spec.Statement
		selects   func(spec.Statement) []identifierSubject
	}{
		{
			name:      "table keyword",
			ruleID:    ruleIDTableNameKeywordForbid,
			subject:   "table",
			statement: statementWithNamedObjects("select", nil, nil),
			selects:   selectTableName,
		},
		{
			name:      "column keyword",
			ruleID:    ruleIDColumnNameKeywordForbid,
			subject:   "column",
			statement: statementWithNamedObjects("users", []spec.Column{{Name: "from", Type: "varchar(16)", Comment: "'from'"}}, nil),
			selects:   selectColumnNames,
		},
		{
			name:      "index keyword",
			ruleID:    ruleIDIndexNameKeywordForbid,
			subject:   "index",
			statement: statementWithNamedObjects("users", nil, []spec.Index{{Name: "order", Kind: spec.IndexKindSecondary, Columns: []string{"name"}}}),
			selects:   selectIndexNames,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statementRule, err := newIdentifierKeywordRule(tt.ruleID, tt.subject, rule.LevelBlocker, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"forbid": true},
			}, tt.selects)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(tt.statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
		})
	}
}

func statementWithNamedObjects(tableName string, columns []spec.Column, indexes []spec.Index) spec.Statement {
	if len(columns) == 0 {
		columns = []spec.Column{{Name: "id", Type: "bigint", Comment: "'id'", NotNull: true, HasDefault: true, DefaultValue: "1"}}
	}

	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:   &spec.Table{Name: tableName, Comment: "'table'"},
			Columns: columns,
			Indexes: indexes,
			PrimaryKey: &spec.Index{
				Name:    "primary",
				Columns: []string{"id"},
			},
		},
	}
}
