// Package ddl verifies PostgreSQL foreign object lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL foreign object lifecycle signals and cross-dialect policy controls
// output: focused coverage for the twelve PostgreSQL foreign object lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG foreign object lifecycle
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Positive tests: rules fire for matching PG statements
// ---------------------------------------------------------------------------

func TestPGForeignObjectLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		object    string
		level     rule.Level
	}{
		{
			name:      "create_foreign_table",
			construct: newCreateForeignTableNoticeRule,
			operation: spec.DDLOperationCreateForeignTable,
			object:    "foreign_table",
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_foreign_table",
			construct: newAlterForeignTableNoticeRule,
			operation: spec.DDLOperationAlterForeignTable,
			object:    "foreign_table",
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_foreign_table",
			construct: newDropForeignTableWarnRule,
			operation: spec.DDLOperationDropForeignTable,
			object:    "foreign_table",
			level:     rule.LevelWarning,
		},
		{
			name:      "create_foreign_server",
			construct: newCreateForeignServerNoticeRule,
			operation: spec.DDLOperationCreateForeignServer,
			object:    "foreign_server",
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_foreign_server",
			construct: newAlterForeignServerNoticeRule,
			operation: spec.DDLOperationAlterForeignServer,
			object:    "foreign_server",
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_foreign_server",
			construct: newDropForeignServerWarnRule,
			operation: spec.DDLOperationDropForeignServer,
			object:    "foreign_server",
			level:     rule.LevelWarning,
		},
		{
			name:      "create_user_mapping",
			construct: newCreateUserMappingNoticeRule,
			operation: spec.DDLOperationCreateUserMapping,
			object:    "user_mapping",
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_user_mapping",
			construct: newAlterUserMappingNoticeRule,
			operation: spec.DDLOperationAlterUserMapping,
			object:    "user_mapping",
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_user_mapping",
			construct: newDropUserMappingWarnRule,
			operation: spec.DDLOperationDropUserMapping,
			object:    "user_mapping",
			level:     rule.LevelWarning,
		},
		{
			name:      "create_foreign_data_wrapper",
			construct: newCreateForeignDataWrapperNoticeRule,
			operation: spec.DDLOperationCreateForeignDataWrapper,
			object:    "foreign_data_wrapper",
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_foreign_data_wrapper",
			construct: newAlterForeignDataWrapperNoticeRule,
			operation: spec.DDLOperationAlterForeignDataWrapper,
			object:    "foreign_data_wrapper",
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_foreign_data_wrapper",
			construct: newDropForeignDataWrapperWarnRule,
			operation: spec.DDLOperationDropForeignDataWrapper,
			object:    "foreign_data_wrapper",
			level:     rule.LevelWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := tc.construct(policy.RulePolicy{Enabled: true, Level: tc.level})
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			// --- Positive: correct PG statement fires the rule ---
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: tc.object,
					ObjectName: "test_obj",
				},
			}

			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].Level != tc.level {
				t.Fatalf("expected level %q, got %q", tc.level, findings[0].Level)
			}
			if findings[0].Explanation == nil || findings[0].Explanation.Why == "" || findings[0].Explanation.Risk == "" || findings[0].Explanation.Suggestion == "" {
				t.Fatalf("expected complete explanation, got %+v", findings[0].Explanation)
			}
			if findings[0].Metadata["object_name"] != "test_obj" {
				t.Fatalf("expected metadata object_name=test_obj, got %v", findings[0].Metadata["object_name"])
			}

			// --- Wrong dialect: MySQL statements are skipped ---
			mysqlStmt := stmt
			mysqlStmt.Dialect = spec.DialectMySQL
			findings, err = r.Evaluate(context.Background(), mysqlStmt)
			if err != nil {
				t.Fatalf("evaluate mysql: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip MySQL, got %d findings", len(findings))
			}

			// --- Wrong operation: different DDL operation is skipped ---
			wrongStmt := stmt
			wrongStmt.DDL = &spec.DDL{
				Operation:  spec.DDLOperationCreateTable,
				ObjectType: "table",
				ObjectName: "other_obj",
			}
			findings, err = r.Evaluate(context.Background(), wrongStmt)
			if err != nil {
				t.Fatalf("evaluate wrong op: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip wrong operation, got %d findings", len(findings))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry integration tests
// ---------------------------------------------------------------------------

func TestRegistryIncludesPGForeignObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateForeignTableNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateForeignTable, ObjectName: "ft1", ObjectType: "foreign_table"},
		}},
		{ruleIDPGAlterForeignTableNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterForeignTable, ObjectName: "ft1", ObjectType: "foreign_table"},
		}},
		{ruleIDPGDropForeignTableWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropForeignTable, ObjectName: "ft1", ObjectType: "foreign_table"},
		}},
		{ruleIDPGCreateForeignServerNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateForeignServer, ObjectName: "fs1", ObjectType: "foreign_server"},
		}},
		{ruleIDPGAlterForeignServerNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterForeignServer, ObjectName: "fs1", ObjectType: "foreign_server"},
		}},
		{ruleIDPGDropForeignServerWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropForeignServer, ObjectName: "fs1", ObjectType: "foreign_server"},
		}},
		{ruleIDPGCreateUserMappingNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateUserMapping, ObjectName: "um1", ObjectType: "user_mapping"},
		}},
		{ruleIDPGAlterUserMappingNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterUserMapping, ObjectName: "um1", ObjectType: "user_mapping"},
		}},
		{ruleIDPGDropUserMappingWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropUserMapping, ObjectName: "um1", ObjectType: "user_mapping"},
		}},
		{ruleIDPGCreateForeignDataWrapperNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateForeignDataWrapper, ObjectName: "fdw1", ObjectType: "foreign_data_wrapper"},
		}},
		{ruleIDPGAlterForeignDataWrapperNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterForeignDataWrapper, ObjectName: "fdw1", ObjectType: "foreign_data_wrapper"},
		}},
		{ruleIDPGDropForeignDataWrapperWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropForeignDataWrapper, ObjectName: "fdw1", ObjectType: "foreign_data_wrapper"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			t.Parallel()
			findings, err := registry.EvaluateStatement(context.Background(), tc.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			found := false
			for _, f := range findings {
				if f.RuleID == tc.ruleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected registry to produce finding for %q, got %d findings", tc.ruleID, len(findings))
			}
		})
	}
}

func TestDefaultPolicyIncludesPGForeignObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGCreateForeignTableNotice, rule.LevelNotice, true},
		{ruleIDPGAlterForeignTableNotice, rule.LevelNotice, true},
		{ruleIDPGDropForeignTableWarn, rule.LevelWarning, true},
		{ruleIDPGCreateForeignServerNotice, rule.LevelNotice, true},
		{ruleIDPGAlterForeignServerNotice, rule.LevelNotice, true},
		{ruleIDPGDropForeignServerWarn, rule.LevelWarning, true},
		{ruleIDPGCreateUserMappingNotice, rule.LevelNotice, true},
		{ruleIDPGAlterUserMappingNotice, rule.LevelNotice, true},
		{ruleIDPGDropUserMappingWarn, rule.LevelWarning, true},
		{ruleIDPGCreateForeignDataWrapperNotice, rule.LevelNotice, true},
		{ruleIDPGAlterForeignDataWrapperNotice, rule.LevelNotice, true},
		{ruleIDPGDropForeignDataWrapperWarn, rule.LevelWarning, true},
	}

	for _, exp := range expected {
		t.Run(exp.id, func(t *testing.T) {
			t.Parallel()
			p, ok := cfg.Rules[exp.id]
			if !ok {
				t.Fatalf("expected default policy to include %q", exp.id)
			}
			if !p.Enabled {
				t.Fatalf("expected %q to be enabled", exp.id)
			}
			if p.Level != exp.level {
				t.Fatalf("expected %q level %q, got %q", exp.id, exp.level, p.Level)
			}
		})
	}
}

func TestForeignObjectLifecycleRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateForeignTable,
			ObjectName: "ft1",
			ObjectType: "foreign_table",
		},
	}
	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGCreateForeignTableNotice:       true,
		ruleIDPGAlterForeignTableNotice:        true,
		ruleIDPGDropForeignTableWarn:           true,
		ruleIDPGCreateForeignServerNotice:      true,
		ruleIDPGAlterForeignServerNotice:       true,
		ruleIDPGDropForeignServerWarn:          true,
		ruleIDPGCreateUserMappingNotice:        true,
		ruleIDPGAlterUserMappingNotice:         true,
		ruleIDPGDropUserMappingWarn:            true,
		ruleIDPGCreateForeignDataWrapperNotice: true,
		ruleIDPGAlterForeignDataWrapperNotice:  true,
		ruleIDPGDropForeignDataWrapperWarn:     true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG foreign object lifecycle rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Negative tests: all rules skip MySQL dialect
// ---------------------------------------------------------------------------

func TestForeignObjectLifecycleRulesSkipMySQL(t *testing.T) {
	t.Parallel()
	rules := []rule.StatementRule{
		mustNewCreateForeignTableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterForeignTableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropForeignTableWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewCreateForeignServerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterForeignServerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropForeignServerWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewCreateUserMappingNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterUserMappingNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropUserMappingWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewCreateForeignDataWrapperNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterForeignDataWrapperNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropForeignDataWrapperWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateForeignTable,
			ObjectName: "ft1",
			ObjectType: "foreign_table",
		},
	}

	for _, r := range rules {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", r.ID(), err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected %s to skip MySQL, got %d findings", r.ID(), len(findings))
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewCreateForeignTableNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateForeignTableNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create foreign table notice rule: %v", err)
	}
	return r
}

func mustNewAlterForeignTableNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterForeignTableNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter foreign table notice rule: %v", err)
	}
	return r
}

func mustNewDropForeignTableWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropForeignTableWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop foreign table warn rule: %v", err)
	}
	return r
}

func mustNewCreateForeignServerNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateForeignServerNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create foreign server notice rule: %v", err)
	}
	return r
}

func mustNewAlterForeignServerNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterForeignServerNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter foreign server notice rule: %v", err)
	}
	return r
}

func mustNewDropForeignServerWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropForeignServerWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop foreign server warn rule: %v", err)
	}
	return r
}

func mustNewCreateUserMappingNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateUserMappingNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create user mapping notice rule: %v", err)
	}
	return r
}

func mustNewAlterUserMappingNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterUserMappingNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter user mapping notice rule: %v", err)
	}
	return r
}

func mustNewDropUserMappingWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropUserMappingWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop user mapping warn rule: %v", err)
	}
	return r
}

func mustNewCreateForeignDataWrapperNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateForeignDataWrapperNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create foreign data wrapper notice rule: %v", err)
	}
	return r
}

func mustNewAlterForeignDataWrapperNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterForeignDataWrapperNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter foreign data wrapper notice rule: %v", err)
	}
	return r
}

func mustNewDropForeignDataWrapperWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropForeignDataWrapperWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop foreign data wrapper warn rule: %v", err)
	}
	return r
}
