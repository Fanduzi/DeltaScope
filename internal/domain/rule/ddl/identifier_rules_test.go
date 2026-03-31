// Package ddl verifies create-table identifier governance behavior.
// input: synthetic create-table Statement specs with table, column, and index names plus policy overrides
// output: focused coverage for identifier-pattern and reserved-keyword findings
// pos: domain DDL rule test coverage for identifier and keyword governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"strings"
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

func TestNamingRulesValidatePrefixSuffixAndContains(t *testing.T) {
	tests := []struct {
		name        string
		build       func(t *testing.T) rule.StatementRule
		statement    spec.Statement
		wantCount    int
		wantMessage  string
		wantSubject  string
		wantMetadata string
	}{
		{
			name: "prefix passes when name matches",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingPrefixRule("ddl.table.name.prefix.require", "table", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "tbl_"},
				}, selectTableName)
				if err != nil {
					t.Fatalf("new prefix rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithNamedObjects("tbl_users", nil, nil),
			wantCount: 0,
		},
		{
			name: "prefix fails when name mismatches",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingPrefixRule("ddl.table.name.prefix.require", "table", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "tbl_"},
				}, selectTableName)
				if err != nil {
					t.Fatalf("new prefix rule: %v", err)
				}
				return statementRule
			},
			statement:    statementWithNamedObjects("users", nil, nil),
			wantCount:    1,
			wantMessage:  `table name "users" must start with "tbl_"`,
			wantSubject:  "table",
			wantMetadata: "tbl_",
		},
		{
			name: "suffix fails when name mismatches",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingSuffixRule("ddl.column.name.suffix.require", "column", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "_id"},
				}, selectColumnNames)
				if err != nil {
					t.Fatalf("new suffix rule: %v", err)
				}
				return statementRule
			},
			statement:    statementWithNamedObjects("users", []spec.Column{{Name: "user", Type: "bigint", Comment: "'user'"}}, nil),
			wantCount:    1,
			wantMessage:  `column name "user" must end with "_id"`,
			wantSubject:  "column",
			wantMetadata: "_id",
		},
		{
			name: "contains uses OR semantics",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingContainsRule("ddl.column.name.contains.require", "column", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"order", "user"}},
				}, selectColumnNames)
				if err != nil {
					t.Fatalf("new contains rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithNamedObjects("users", []spec.Column{
				{Name: "order_code", Type: "varchar(32)", Comment: "'order'"},
				{Name: "tenant_slug", Type: "varchar(32)", Comment: "'tenant'"},
			}, nil),
			wantCount:    1,
			wantMessage:  `column name "tenant_slug" must contain one of: order, user`,
			wantSubject:  "column",
			wantMetadata: "order,user",
		},
		{
			name: "empty requirement disables the check",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingContainsRule("ddl.column.name.contains.require", "column", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{" ", ""}},
				}, selectColumnNames)
				if err != nil {
					t.Fatalf("new contains rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithNamedObjects("users", []spec.Column{
				{Name: "tenant_slug", Type: "varchar(32)", Comment: "'tenant'"},
			}, nil),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statementRule := tt.build(t)

			findings, err := statementRule.Evaluate(tt.statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != tt.wantCount {
				t.Fatalf("expected %d findings, got %d", tt.wantCount, len(findings))
			}
			if tt.wantCount == 0 {
				return
			}
			if findings[0].Message != tt.wantMessage {
				t.Fatalf("expected message %q, got %q", tt.wantMessage, findings[0].Message)
			}
			if got := findings[0].Metadata["subject"]; got != tt.wantSubject {
				t.Fatalf("expected subject metadata %q, got %#v", tt.wantSubject, got)
			}
			switch tt.name {
			case "contains uses OR semantics":
				values, ok := findings[0].Metadata["contains"].([]string)
				if !ok {
					t.Fatalf("expected contains metadata slice, got %#v", findings[0].Metadata["contains"])
				}
				if strings.Join(values, ",") != tt.wantMetadata {
					t.Fatalf("expected contains metadata %q, got %v", tt.wantMetadata, values)
				}
			case "prefix fails when name mismatches":
				if got := findings[0].Metadata["prefix"]; got != tt.wantMetadata {
					t.Fatalf("expected prefix metadata %q, got %#v", tt.wantMetadata, got)
				}
			case "suffix fails when name mismatches":
				if got := findings[0].Metadata["suffix"]; got != tt.wantMetadata {
					t.Fatalf("expected suffix metadata %q, got %#v", tt.wantMetadata, got)
				}
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
