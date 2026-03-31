// Package ddl verifies Tier-1 DDL column rule behavior.
// input: create-table Statement specs with synthetic column metadata and rule policy overrides
// output: deterministic findings for column-count, comment, naming, default, nullability, and type rules
// pos: domain DDL rule test coverage for column governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableColumnsMinCountRuleFindsEmptyTables(t *testing.T) {
	ruleUnderTest, err := newTableColumnsMinCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": 1},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users"))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnCommentRequiredRuleFindsMissingComments(t *testing.T) {
	ruleUnderTest, err := newColumnCommentRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "name", Type: "varchar(32)"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnNameMaxLengthRuleFindsLongNames(t *testing.T) {
	ruleUnderTest, err := newColumnNameMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": 8},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "display_name", Type: "varchar(32)", Comment: "'name'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnVarcharMaxLengthRuleFindsOversizedColumns(t *testing.T) {
	ruleUnderTest, err := newColumnVarcharMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": 64},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "bio", Type: "varchar(255)", Length: 255, Comment: "'bio'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnDefaultRequiredRuleFindsMissingDefaults(t *testing.T) {
	ruleUnderTest, err := newColumnDefaultRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "score", Type: "int", NotNull: true, Comment: "'score'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnDefaultRequiredRuleIgnoresBlobTextLikeColumns(t *testing.T) {
	ruleUnderTest, err := newColumnDefaultRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "payload", Type: "json", Comment: "'payload'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestColumnNotNullRequiredRuleFindsNullableBusinessColumns(t *testing.T) {
	ruleUnderTest, err := newColumnNotNullRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required":        true,
			"allow_time_null": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "status", Type: "varchar(16)", Length: 16, Comment: "'status'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnNotNullRequiredRuleAllowsTimeLikeNullableColumnsWhenConfigured(t *testing.T) {
	ruleUnderTest, err := newColumnNotNullRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required":        true,
			"allow_time_null": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "deleted_at", Type: "datetime", Comment: "'deleted'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestColumnFloatDoubleForbiddenRuleFindsFloatColumns(t *testing.T) {
	ruleUnderTest, err := newColumnFloatDoubleForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(createTableWithColumns("users", spec.Column{Name: "ratio", Type: "float", Comment: "'ratio'", NotNull: true, HasDefault: true, DefaultValue: "0"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestColumnNameGovernanceRules(t *testing.T) {
	tests := []struct {
		name      string
		columns   []spec.Column
		rules     map[string]policy.RulePolicy
		wantIDs   []string
		wantCount int
	}{
		{
			name: "column prefix suffix and contains checks run independently",
			columns: []spec.Column{
				{Name: "user", Type: "bigint", Comment: "'user'"},
			},
			rules: map[string]policy.RulePolicy{
				"ddl.column.name.prefix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "col_"},
				},
				"ddl.column.name.suffix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "_id"},
				},
				"ddl.column.name.contains.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"user", "account"}},
				},
			},
			wantIDs:   []string{"ddl.column.name.prefix.require", "ddl.column.name.suffix.require"},
			wantCount: 2,
		},
		{
			name: "inactive column naming constraints stay quiet",
			columns: []spec.Column{
				{Name: "user", Type: "bigint", Comment: "'user'"},
			},
			rules: map[string]policy.RulePolicy{
				"ddl.column.name.prefix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": " "},
				},
				"ddl.column.name.suffix.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": ""},
				},
				"ddl.column.name.contains.require": {
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"", " "}},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := evaluateDDLFindings(t, policy.Policy{Rules: tt.rules}, createTableWithColumns("users", tt.columns...))

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

func createTableWithColumns(name string, columns ...spec.Column) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:   &spec.Table{Name: name, Comment: "'table'"},
			Columns: columns,
			PrimaryKey: &spec.Index{
				Name:    "primary",
				Columns: []string{"id"},
			},
		},
	}
}
