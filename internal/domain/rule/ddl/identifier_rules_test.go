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

func TestIdentifierKeywordRuleFindsPostgreSQLReservedKeywords(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		subject   string
		statement spec.Statement
		selects   func(spec.Statement) []identifierSubject
	}{
		{
			name:   "postgresql table keyword",
			ruleID: ruleIDTableNameKeywordForbid,
			subject: "table",
			statement: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationCreateTable,
					Table:     &spec.Table{Name: "user"},
				},
			},
			selects: selectTableName,
		},
		{
			name:   "postgresql column keyword",
			ruleID: ruleIDColumnNameKeywordForbid,
			subject: "column",
			statement: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationCreateTable,
					Table:     &spec.Table{Name: "accounts"},
					Columns:   []spec.Column{{Name: "select", Type: "text"}},
				},
			},
			selects: selectColumnNames,
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

func TestIdentifierKeywordRuleFindsSharedReservedKeywordsForPostgreSQL(t *testing.T) {
	statementRule, err := newIdentifierKeywordRule(ruleIDTableNameKeywordForbid, "table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	}, selectTableName)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "from"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for shared reserved keyword, got %d", len(findings))
	}
}

func TestIdentifierKeywordRuleFindsPostgreSQLIndexReservedKeywords(t *testing.T) {
	statementRule, err := newIdentifierKeywordRule(ruleIDIndexNameKeywordForbid, "index", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	}, selectIndexNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "accounts"},
			Columns:   []spec.Column{{Name: "id", Type: "bigint"}},
			Indexes:   []spec.Index{{Name: "user", Kind: spec.IndexKindSecondary, Columns: []string{"id"}}},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for PostgreSQL index keyword, got %d", len(findings))
	}
}

func TestIdentifierKeywordRuleDoesNotFlagSafePostgreSQLNames(t *testing.T) {
	statementRule, err := newIdentifierKeywordRule(ruleIDTableNameKeywordForbid, "table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	}, selectTableName)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "accounts"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe PostgreSQL table name, got %d", len(findings))
	}
}

func TestNamingRulesValidatePrefixSuffixAndContains(t *testing.T) {
	tests := []struct {
		name         string
		build        func(t *testing.T) rule.StatementRule
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

func TestConstraintNamingRulesEvaluateExplicitNamesOnly(t *testing.T) {
	tests := []struct {
		name       string
		build      func(t *testing.T) rule.StatementRule
		statement  spec.Statement
		wantCount  int
		wantRuleID string
	}{
		{
			name: "primary key prefix checks explicit names only",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingPrefixRule("ddl.constraint.primary_key.name.prefix.require", "primary key constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "pk_"},
				}, selectPrimaryKeyConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithConstraints(
				&spec.Index{Name: "orders_pk", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
				nil,
			),
			wantCount:  1,
			wantRuleID: "ddl.constraint.primary_key.name.prefix.require",
		},
		{
			name: "unique key contains passes on positive match",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingContainsRule("ddl.constraint.unique_key.name.contains.require", "unique key constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"user", "account"}},
				}, selectUniqueConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithConstraints(
				nil,
				[]spec.Index{{Name: "uk_user_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}}},
			),
			wantCount: 0,
		},
		{
			name: "foreign key suffix checks explicit names only",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingSuffixRule("ddl.constraint.foreign_key.name.suffix.require", "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "_fk"},
				}, selectForeignKeyConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement:  statementWithConstraints(nil, nil, spec.Constraint{Type: "foreign_key", Name: "fk_orders_user"}),
			wantCount:  1,
			wantRuleID: "ddl.constraint.foreign_key.name.suffix.require",
		},
		{
			name: "check constraint skips unnamed objects",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingContainsRule("ddl.constraint.check.name.contains.require", "check constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"amount", "price"}},
				}, selectCheckConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement: statementWithConstraints(nil, nil, spec.Constraint{Type: "check", Name: ""}),
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
			if findings[0].RuleID != tt.wantRuleID {
				t.Fatalf("expected rule id %q, got %q", tt.wantRuleID, findings[0].RuleID)
			}
		})
	}
}

func TestPostgreSQLConstraintNamingRulesReuseSharedSelectors(t *testing.T) {
	tests := []struct {
		name         string
		build        func(t *testing.T) rule.StatementRule
		statement    spec.Statement
		wantCount    int
		wantRuleID   string
		wantSubject  string
		wantMetadata string
	}{
		{
			name: "named check uses shared constraint naming rule",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"prefix": "ck_"},
				}, selectCheckConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement:    postgresStatementWithConstraints(nil, nil, spec.Constraint{Type: "check", Name: "amount_positive"}),
			wantCount:    1,
			wantRuleID:   ruleIDConstraintCheckNamePrefixRequire,
			wantSubject:  "constraint.check",
			wantMetadata: "ck_",
		},
		{
			name: "named foreign key uses shared constraint naming rule",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingSuffixRule(ruleIDConstraintForeignKeyNameSuffixRequire, "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"suffix": "_fk"},
				}, selectForeignKeyConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement:    postgresStatementWithConstraints(nil, nil, spec.Constraint{Type: "foreign_key", Name: "orders_user_ref"}),
			wantCount:    1,
			wantRuleID:   ruleIDConstraintForeignKeyNameSuffixRequire,
			wantSubject:  "constraint.foreign_key",
			wantMetadata: "_fk",
		},
		{
			name: "named unique uses shared constraint naming rule",
			build: func(t *testing.T) rule.StatementRule {
				t.Helper()
				statementRule, err := newNamingContainsRule(ruleIDConstraintUniqueKeyNameContainsRequire, "unique key constraint", rule.LevelWarning, policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"contains": []string{"account", "tenant"}},
				}, selectUniqueConstraintNames)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				return statementRule
			},
			statement:    postgresStatementWithConstraints(nil, []spec.Index{{Name: "uniq_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}}}),
			wantCount:    1,
			wantRuleID:   ruleIDConstraintUniqueKeyNameContainsRequire,
			wantSubject:  "constraint.unique_key",
			wantMetadata: "account,tenant",
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
			if findings[0].RuleID != tt.wantRuleID {
				t.Fatalf("expected rule id %q, got %q", tt.wantRuleID, findings[0].RuleID)
			}
			if got := findings[0].Metadata["subject"]; got != tt.wantSubject {
				t.Fatalf("expected subject metadata %q, got %#v", tt.wantSubject, got)
			}
			switch tt.name {
			case "named check uses shared constraint naming rule":
				if got := findings[0].Metadata["prefix"]; got != tt.wantMetadata {
					t.Fatalf("expected prefix metadata %q, got %#v", tt.wantMetadata, got)
				}
			case "named foreign key uses shared constraint naming rule":
				if got := findings[0].Metadata["suffix"]; got != tt.wantMetadata {
					t.Fatalf("expected suffix metadata %q, got %#v", tt.wantMetadata, got)
				}
			case "named unique uses shared constraint naming rule":
				values, ok := findings[0].Metadata["contains"].([]string)
				if !ok {
					t.Fatalf("expected contains metadata slice, got %#v", findings[0].Metadata["contains"])
				}
				if strings.Join(values, ",") != tt.wantMetadata {
					t.Fatalf("expected contains metadata %q, got %v", tt.wantMetadata, values)
				}
			}
		})
	}
}

func TestRegisterSkipsForeignKeyConstraintNamingWhenForeignKeysAreForbidden(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Policy{
		Rules: map[string]policy.RulePolicy{
			ruleIDTableForeignKeyForbid: {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"forbid": true},
			},
			"ddl.constraint.foreign_key.name.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"prefix": "fk_"},
			},
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(statementWithConstraints(nil, nil, spec.Constraint{
		Type:    "foreign_key",
		Name:    "orders_user_ref",
		Columns: []string{"user_id"},
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDTableForeignKeyForbid {
		t.Fatalf("expected foreign key forbid finding only, got %+v", findings)
	}
}

func TestImplicitPrimaryKeyConstraintNamingIsSkipped(t *testing.T) {
	statementRule, err := newNamingPrefixRule("ddl.constraint.primary_key.name.prefix.require", "primary key constraint", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"prefix": "pk_"},
	}, selectPrimaryKeyConstraintNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statementWithConstraints(nil, nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected implicit primary key name to be skipped, got %+v", findings)
	}
}

func TestForeignKeyConstraintNamingRulesAreSuppressedWhenForeignKeysAreForbidden(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Policy{
		Rules: map[string]policy.RulePolicy{
			ruleIDTableForeignKeyForbid: {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"forbid": true},
			},
			"ddl.constraint.foreign_key.name.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"prefix": "fk_"},
			},
			"ddl.constraint.foreign_key.name.suffix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"suffix": "_fk"},
			},
			"ddl.constraint.foreign_key.name.contains.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"contains": []string{"tenant", "account"}},
			},
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(statementWithConstraints(nil, nil, spec.Constraint{
		Type:    "foreign_key",
		Name:    "orders_user_ref",
		Columns: []string{"user_id"},
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected only the forbid finding, got %+v", findings)
	}
	if findings[0].RuleID != ruleIDTableForeignKeyForbid {
		t.Fatalf("expected foreign key forbid finding only, got %+v", findings)
	}
}

func TestRicherPostgreSQLForeignKeyConstraintUsesSharedConstraintNamingRule(t *testing.T) {
	statementRule, err := newNamingPrefixRule(ruleIDConstraintForeignKeyNamePrefixRequire, "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"prefix": "fk_"},
	}, selectForeignKeyConstraintNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := postgresStatementWithConstraints(nil, nil, spec.Constraint{
		Type:              "foreign_key",
		Name:              "bad_orders_user",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	})

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for richer FK constraint with wrong prefix, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDConstraintForeignKeyNamePrefixRequire {
		t.Fatalf("expected rule id %q, got %q", ruleIDConstraintForeignKeyNamePrefixRequire, findings[0].RuleID)
	}
	if findings[0].Metadata["subject"] != "constraint.foreign_key" {
		t.Fatalf("expected subject constraint.foreign_key, got %#v", findings[0].Metadata["subject"])
	}
	if findings[0].Metadata["prefix"] != "fk_" {
		t.Fatalf("expected prefix metadata fk_, got %#v", findings[0].Metadata["prefix"])
	}
	if findings[0].Metadata["name"] != "bad_orders_user" {
		t.Fatalf("expected name metadata bad_orders_user, got %#v", findings[0].Metadata["name"])
	}
}

func TestRicherPostgreSQLForeignKeyConstraintPassesSharedNamingRule(t *testing.T) {
	statementRule, err := newNamingPrefixRule(ruleIDConstraintForeignKeyNamePrefixRequire, "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"prefix": "fk_"},
	}, selectForeignKeyConstraintNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := postgresStatementWithConstraints(nil, nil, spec.Constraint{
		Type:              "foreign_key",
		Name:              "fk_orders_user",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	})

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for richer FK constraint with correct prefix, got %d", len(findings))
	}
}

func TestRicherPostgreSQLInlineReferencesConstraintUsesSharedForeignKeyForbidRule(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Policy{
		Rules: map[string]policy.RulePolicy{
			ruleIDTableForeignKeyForbid: {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"forbid": true},
			},
		},
	}
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders"},
			Columns:   []spec.Column{{Name: "user_id", Type: "bigint"}},
			Constraints: []spec.Constraint{{
				Type:              "foreign_key",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 foreign_key forbid finding for richer inline references, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDTableForeignKeyForbid {
		t.Fatalf("expected foreign key forbid finding, got %+v", findings)
	}
}

// ---------------------------------------------------------------------------
// v0.29.0 Task 2: ddl.pg.table.foreign_key.cross_schema.advisory
// ---------------------------------------------------------------------------

func TestTableForeignKeyCrossSchemaAdvisoryFiresForExplicitCrossSchemaFK(t *testing.T) {
	statementRule, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders", Schema: "public"},
			Columns:   []spec.Column{{Name: "id", Type: "bigint"}, {Name: "approver_id", Type: "bigint"}},
			Constraints: []spec.Constraint{
				{
					Type:              "foreign_key",
					Name:              "fk_orders_approver",
					Columns:           []string{"approver_id"},
					ReferencedSchema:  "auth",
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 cross-schema advisory finding, got %d", len(findings))
	}
	if statementRule.ID() != "ddl.pg.table.foreign_key.cross_schema.advisory" {
		t.Fatalf("expected rule ID ddl.pg.table.foreign_key.cross_schema.advisory, got %q", statementRule.ID())
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected level notice, got %q", findings[0].Level)
	}
	if findings[0].Metadata["table_schema"] != "public" {
		t.Fatalf("expected table_schema public, got %v", findings[0].Metadata["table_schema"])
	}
	if findings[0].Metadata["referenced_schema"] != "auth" {
		t.Fatalf("expected referenced_schema auth, got %v", findings[0].Metadata["referenced_schema"])
	}
	if findings[0].Metadata["referenced_table"] != "users" {
		t.Fatalf("expected referenced_table users, got %v", findings[0].Metadata["referenced_table"])
	}
}

func TestTableForeignKeyCrossSchemaAdvisoryDoesNotFireForSameSchemaFK(t *testing.T) {
	statementRule, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders", Schema: "public"},
			Columns:   []spec.Column{{Name: "id", Type: "bigint"}, {Name: "user_id", Type: "bigint"}},
			Constraints: []spec.Constraint{
				{
					Type:              "foreign_key",
					Name:              "fk_orders_user",
					Columns:           []string{"user_id"},
					ReferencedSchema:  "public",
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 advisory findings for same-schema FK, got %d", len(findings))
	}
}

func TestTableForeignKeyCrossSchemaAdvisoryDoesNotFireForBareReference(t *testing.T) {
	statementRule, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	// Bare REFERENCES users(id) — no schema qualifier, no owning schema.
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders"},
			Columns:   []spec.Column{{Name: "id", Type: "bigint"}, {Name: "approver_id", Type: "bigint"}},
			Constraints: []spec.Constraint{
				{
					Type:              "foreign_key",
					Name:              "",
					Columns:           []string{"approver_id"},
					ReferencedSchema:  "",
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 advisory findings for bare FK reference, got %d", len(findings))
	}
}

func TestTableForeignKeyCrossSchemaAdvisoryDoesNotFireForMySQL(t *testing.T) {
	statementRule, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders", Schema: "public"},
			Columns:   []spec.Column{{Name: "id", Type: "bigint"}, {Name: "approver_id", Type: "bigint"}},
			Constraints: []spec.Constraint{
				{
					Type:              "foreign_key",
					Name:              "fk_orders_approver",
					Columns:           []string{"approver_id"},
					ReferencedSchema:  "auth",
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 advisory findings for MySQL, got %d", len(findings))
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

func statementWithConstraints(primaryKey *spec.Index, indexes []spec.Index, constraints ...spec.Constraint) spec.Statement {
	if primaryKey == nil {
		primaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	}
	if len(indexes) == 0 {
		indexes = nil
	}
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:       &spec.Table{Name: "orders", Comment: "'orders'"},
			Columns:     []spec.Column{{Name: "id", Type: "bigint", Comment: "'id'", NotNull: true, HasDefault: true, DefaultValue: "1"}},
			PrimaryKey:  primaryKey,
			Indexes:     indexes,
			Constraints: constraints,
		},
	}
}

func postgresStatementWithConstraints(primaryKey *spec.Index, indexes []spec.Index, constraints ...spec.Constraint) spec.Statement {
	statement := statementWithConstraints(primaryKey, indexes, constraints...)
	statement.Dialect = spec.DialectPostgreSQL
	statement.DDL.Operation = spec.DDLOperationCreateTable
	return statement
}

// ---------------------------------------------------------------------------
// v0.40.0 Task 1: Rule applicability gap tests — ALTER TABLE ADD CONSTRAINT FK
// These tests prove that ALTER TABLE ADD CONSTRAINT FOREIGN KEY does NOT trigger
// existing FK naming rules today, because DDL.Constraints is empty.
// Task 2 must project FK facts from Alter.Options into DDL.Constraints.
// ---------------------------------------------------------------------------

// postgresAlterTableAddFKConstraint builds a spec.Statement that mirrors what
// the PostgreSQL extractor currently produces for
//   ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);
// Today the extractor puts FK facts only in Alter.Options — DDL.Constraints is empty.
func postgresAlterTableAddFKConstraint(constraintName string, columns []string, referencedTable string, referencedColumns []string) spec.Statement {
	options := map[string]string{
		"constraint_type": "foreign_key",
		"not_valid":       "false",
	}
	if len(columns) > 0 {
		options["columns"] = strings.Join(columns, ",")
	}
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action:  "add_constraint",
				Name:    constraintName,
				Options: options,
			}},
			// DDL.Constraints is intentionally empty — this is the current production gap.
		},
	}
}

func TestForeignKeyNamingRuleGapAlterTableAddConstraintFKNotFound(t *testing.T) {
	// Build a statement that represents ALTER TABLE ADD CONSTRAINT FK.
	// The constraint name "bad_name" does not follow any naming convention.
	statement := postgresAlterTableAddFKConstraint("bad_name", []string{"user_id"}, "users", []string{"id"})

	rule, err := newNamingPrefixRule(ruleIDConstraintForeignKeyNamePrefixRequire, "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
		Params: map[string]any{"prefix": "fk_"},
		Level:  rule.LevelWarning,
	}, selectForeignKeyConstraintNames)
	if err != nil {
		t.Fatalf("newNamingPrefixRule: %v", err)
	}

	// Task 2: DDL.Constraints is still empty in the test helper (projection
	// happens in the extractor, not in hand-built specs), so the naming rule
	// still cannot fire.  The rule fires on Constraints, not Alter.Options.
	// Full end-to-end coverage lives in service/corpus tests (Task 3).
	findings, err := rule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (DDL.Constraints empty), got %d", len(findings))
	}
	t.Log("Naming rule reads DDL.Constraints — helper does not populate it; extractor projection is tested separately")
}

func TestForeignKeyForbidRuleGapAlterTableAddConstraintFKNotSeen(t *testing.T) {
	statement := postgresAlterTableAddFKConstraint("fk_orders_user", []string{"user_id"}, "users", []string{"id"})

	rule, err := newTableForeignKeyForbidRule(policy.RulePolicy{
		Params: map[string]any{"forbid": true},
		Level:  rule.LevelBlocker,
	})
	if err != nil {
		t.Fatalf("newTableForeignKeyForbidRule: %v", err)
	}

	// Task 2: AppliesTo now accepts ALTER TABLE with FK constraints.
	if !rule.AppliesTo(statement) {
		t.Fatal("expected AppliesTo=true for ALTER TABLE ADD CONSTRAINT FK")
	}
	t.Log("Task 2: tableForeignKeyForbidRule now AppliesTo ALTER TABLE ADD CONSTRAINT FK")
}

func TestForeignKeyCrossSchemaAdvisoryGapAlterTableAddConstraintFKNotSeen(t *testing.T) {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "fk_orders_user",
				Options: map[string]string{
					"constraint_type":   "foreign_key",
					"columns":           "user_id",
					"referenced_table":  "public.users",
				},
			}},
		},
	}

	rule, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{
		Level: rule.LevelNotice,
	})
	if err != nil {
		t.Fatalf("newTableForeignKeyCrossSchemaAdvisoryRule: %v", err)
	}

	// Task 2: AppliesTo now accepts ALTER TABLE with FK constraints.
	if !rule.AppliesTo(statement) {
		t.Fatal("expected AppliesTo=true for ALTER TABLE ADD CONSTRAINT FK")
	}
	t.Log("Task 2: crossSchemaAdvisory now AppliesTo ALTER TABLE ADD CONSTRAINT FK")
}

// ---------------------------------------------------------------------------
// v0.41.0 Task 1: Rule applicability gap tests — ALTER TABLE ADD CONSTRAINT CHECK
// These tests prove that ALTER TABLE ADD CONSTRAINT CHECK does NOT trigger
// existing check naming rules today, because:
//   1) namingRule.AppliesTo calls appliesToCreateTable which returns false for ALTER TABLE
//   2) DDL.Constraints is empty on the ALTER CHECK path
// Task 2 must: (a) project check facts into DDL.Constraints, (b) widen AppliesTo.
// ---------------------------------------------------------------------------

// postgresAlterTableAddCheckConstraint builds a spec.Statement that mirrors what
// the PostgreSQL extractor currently produces for
//   ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);
// Today the extractor puts check facts only in Alter.Options — DDL.Constraints is empty.
func postgresAlterTableAddCheckConstraint(constraintName string, columns []string) spec.Statement {
	options := map[string]string{
		"constraint_type": "check",
		"not_valid":       "false",
	}
	if len(columns) > 0 {
		options["columns"] = strings.Join(columns, ",")
	}
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action:  "add_constraint",
				Name:    constraintName,
				Options: options,
			}},
			// DDL.Constraints is intentionally empty — this is the current production gap.
		},
	}
}

// postgresAlterTableAddCheckConstraintWithProjection builds the same ALTER CHECK
// statement but WITH projected DDL.Constraints — simulating post-Task-2 extractor output.
func postgresAlterTableAddCheckConstraintWithProjection(constraintName string, columns []string) spec.Statement {
	stmt := postgresAlterTableAddCheckConstraint(constraintName, columns)
	stmt.DDL.Constraints = []spec.Constraint{
		{Type: "check", Name: constraintName, Columns: columns},
	}
	return stmt
}

func TestCheckNamingRule_AlterTableProjectedCheckNowFires(t *testing.T) {
	// With extractor projecting CHECK into DDL.Constraints and the appliesTo
	// gate widened via appliesToCreateTableOrAlterCheckConstraint, the naming
	// rule now fires on ALTER TABLE ADD CONSTRAINT CHECK.
	statement := postgresAlterTableAddCheckConstraintWithProjection("bad_name", []string{"amount"})

	r, err := newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, policy.RulePolicy{
		Params: map[string]any{"prefix": "ck_"},
		Level:  rule.LevelWarning,
	}, selectCheckConstraintNames, appliesToCreateTableOrAlterCheckConstraint)
	if err != nil {
		t.Fatalf("newNamingPrefixRule: %v", err)
	}

	if !r.AppliesTo(statement) {
		t.Fatal("expected AppliesTo=true for ALTER TABLE ADD CONSTRAINT CHECK with widened gate")
	}

	findings, err := r.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for projected ALTER CHECK with bad name, got %d", len(findings))
	}
	t.Log("Projected ALTER TABLE CHECK constraint now fires naming rule")
}

func TestCheckNamingRule_AlterTableProjectedCheckAppliesToNowWidened(t *testing.T) {
	// With the widened appliesTo gate, ALTER TABLE ADD CONSTRAINT CHECK with
	// projected constraints is now accepted by AppliesTo.
	statement := postgresAlterTableAddCheckConstraintWithProjection("bad_name", []string{"amount"})

	r, err := newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, policy.RulePolicy{
		Params: map[string]any{"prefix": "ck_"},
		Level:  rule.LevelWarning,
	}, selectCheckConstraintNames, appliesToCreateTableOrAlterCheckConstraint)
	if err != nil {
		t.Fatalf("newNamingPrefixRule: %v", err)
	}

	if !r.AppliesTo(statement) {
		t.Fatal("expected AppliesTo=true for ALTER TABLE with projected CHECK constraint")
	}
	t.Log("Widened appliesTo gate accepts ALTER TABLE ADD CONSTRAINT CHECK")
}

func TestCheckNamingRuleGap_ProjectedConstraintWouldFireWithFixedAppliesTo(t *testing.T) {
	// Simulate a CREATE TABLE statement with the same projected constraint.
	// This proves the naming rule WOULD fire if AppliesTo accepted ALTER TABLE.
	statement := postgresStatementWithConstraints(nil, nil,
		spec.Constraint{Type: "check", Name: "bad_name", Columns: []string{"amount"}})

	r, err := newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, policy.RulePolicy{
		Params: map[string]any{"prefix": "ck_"},
		Level:  rule.LevelWarning,
	}, selectCheckConstraintNames)
	if err != nil {
		t.Fatalf("newNamingPrefixRule: %v", err)
	}

	if !r.AppliesTo(statement) {
		t.Fatal("expected AppliesTo=true for CREATE TABLE with check constraint")
	}
	findings, err := r.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for bad_name on CREATE TABLE, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDConstraintCheckNamePrefixRequire {
		t.Fatalf("expected rule ID %s, got %s", ruleIDConstraintCheckNamePrefixRequire, findings[0].RuleID)
	}
	t.Log("Naming rule fires correctly on CREATE TABLE with projected check constraint")
}

func TestCheckNamingRule_UnnamedProjectedCheckStillSkipped(t *testing.T) {
	// Unnamed checks (empty Name) should be skipped by naming rules even with widened gate.
	statement := postgresStatementWithConstraints(nil, nil,
		spec.Constraint{Type: "check", Name: "", Columns: []string{"amount"}})

	r, err := newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, policy.RulePolicy{
		Params: map[string]any{"prefix": "ck_"},
		Level:  rule.LevelWarning,
	}, selectCheckConstraintNames)
	if err != nil {
		t.Fatalf("newNamingPrefixRule: %v", err)
	}

	findings, err := r.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for unnamed check, got %d", len(findings))
	}
	t.Log("Unnamed check constraints correctly skipped by naming rules")
}

func TestCheckNamingRule_AlterTableAppliesToGatePrecedent(t *testing.T) {
	// Both FK and CHECK naming rules now use analogous appliesTo functions
	// that accept CREATE TABLE and their respective ALTER TABLE ADD CONSTRAINT forms.

	fkAlterStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action:  "add_constraint",
				Name:    "fk_orders_user",
				Options: map[string]string{"constraint_type": "foreign_key"},
			}},
		},
	}
	if !appliesToCreateTableOrAlterFKConstraint(fkAlterStmt) {
		t.Fatal("FK precedent: appliesToCreateTableOrAlterFKConstraint should accept ALTER TABLE ADD CONSTRAINT FK")
	}

	checkAlterStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action:  "add_constraint",
				Name:    "amount_positive",
				Options: map[string]string{"constraint_type": "check"},
			}},
		},
	}
	// appliesToCreateTableOrAlterCheckConstraint now accepts ALTER TABLE ADD CONSTRAINT CHECK.
	if !appliesToCreateTableOrAlterCheckConstraint(checkAlterStmt) {
		t.Fatal("expected appliesToCreateTableOrAlterCheckConstraint to accept ALTER TABLE ADD CONSTRAINT CHECK")
	}
	t.Log("Both FK and CHECK appliesTo gates accept their respective ALTER TABLE ADD CONSTRAINT forms")
}
