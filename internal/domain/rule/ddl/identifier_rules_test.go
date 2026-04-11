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
