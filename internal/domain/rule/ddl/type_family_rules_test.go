// Package ddl verifies create-table type-family breadth behavior.
// input: synthetic create-table Statement specs with column type and charset metadata plus policy overrides
// output: focused coverage for forbidden type families, char-length, and charset/collation findings
// pos: domain DDL rule test coverage for create-table type-family governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestColumnTypeForbiddenRulesFindConfiguredFamilies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		ruleID     string
		label      string
		column     spec.Column
		suggestion string
		matches    func(spec.Column) bool
	}{
		{
			name:       "blob text",
			ruleID:     ruleIDColumnBlobTextForbid,
			label:      "blob/text",
			column:     spec.Column{Name: "body", Type: "text", Comment: "'body'"},
			suggestion: "switch to varchar or relax the blob/text policy intentionally",
			matches: func(column spec.Column) bool {
				return isBlobTextLike(column) && !strings.EqualFold(baseType(column), "json")
			},
		},
		{
			name:       "json",
			ruleID:     ruleIDColumnJSONForbid,
			label:      "json",
			column:     spec.Column{Name: "payload", Type: "json", Comment: "'payload'"},
			suggestion: "store structured data in relational columns or relax the json policy intentionally",
			matches: func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "json")
			},
		},
		{
			name:       "bit",
			ruleID:     ruleIDColumnBitForbid,
			label:      "bit",
			column:     spec.Column{Name: "flags", Type: "bit(8)", Comment: "'flags'"},
			suggestion: "use integer or boolean-friendly types instead of bit",
			matches: func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "bit")
			},
		},
		{
			name:       "timestamp",
			ruleID:     ruleIDColumnTimestampForbid,
			label:      "timestamp",
			column:     spec.Column{Name: "created_at", Type: "timestamp", Comment: "'created'"},
			suggestion: "prefer datetime unless the team intentionally allows timestamp columns",
			matches: func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "timestamp")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statementRule, err := newColumnTypeForbiddenRule(tt.ruleID, tt.label, rule.LevelWarning, tt.suggestion, tt.matches, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"forbid": true},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("users", tt.column))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
		})
	}
}

func TestColumnCharMaxLengthRuleFindsOversizedChar(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnCharMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"limit": 64},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("users", spec.Column{Name: "code", Type: "char(96)", Length: 96, Comment: "'code'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnCharsetAllowlistRuleFindsUnsupportedCharset(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnValueAllowlistRule(ruleIDColumnCharsetAllowlist, "charset", []string{"utf8", "utf8mb4"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []string{"utf8", "utf8mb4"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("users", spec.Column{Name: "nickname", Type: "varchar(32)", Length: 32, Charset: "latin1", Collation: "latin1_swedish_ci", Comment: "'nickname'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnCollationAllowlistRuleFindsUnsupportedCollation(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnValueAllowlistRule(ruleIDColumnCollationAllowlist, "collation", []string{"utf8mb4_general_ci", "utf8mb4_bin"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []string{"utf8mb4_general_ci", "utf8mb4_bin"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("users", spec.Column{Name: "nickname", Type: "varchar(32)", Length: 32, Charset: "utf8mb4", Collation: "utf8mb4_unicode_ci", Comment: "'nickname'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnCharsetAllowlistRuleSkipsPostgreSQL(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnValueAllowlistRule(ruleIDColumnCharsetAllowlist, "charset", []string{"utf8", "utf8mb4"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []string{"utf8", "utf8mb4"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := createTableWithColumns("users", spec.Column{Name: "nickname", Type: "varchar(32)", Length: 32, Charset: "latin1", Collation: "latin1_swedish_ci", Comment: "'nickname'"})
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected PostgreSQL charset rule to be skipped, got %d findings", len(findings))
	}
}

func TestColumnCollationAllowlistRuleSkipsPostgreSQL(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnValueAllowlistRule(ruleIDColumnCollationAllowlist, "collation", []string{"utf8mb4_general_ci", "utf8mb4_bin"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []string{"utf8mb4_general_ci", "utf8mb4_bin"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := createTableWithColumns("users", spec.Column{Name: "nickname", Type: "varchar(32)", Length: 32, Charset: "utf8mb4", Collation: "utf8mb4_unicode_ci", Comment: "'nickname'"})
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected PostgreSQL collation rule to be skipped, got %d findings", len(findings))
	}
}

func TestColumnCharsetCollationMatchRuleFindsPartialAndMismatchedPairs(t *testing.T) {
	t.Parallel()
	statementRule, err := newColumnCharsetCollationMatchRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := createTableWithColumns(
		"users",
		spec.Column{Name: "nickname", Type: "varchar(32)", Length: 32, Charset: "utf8", Comment: "'nickname'"},
		spec.Column{Name: "display_name", Type: "varchar(32)", Length: 32, Charset: "utf8mb4", Collation: "utf8_general_ci", Comment: "'display'"},
	)
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}
