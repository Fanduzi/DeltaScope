// Package ddl verifies MySQL/TiDB database lifecycle rule behavior.
// input: synthetic DDL statements with database/schema lifecycle signals and cross-dialect policy controls
// output: focused coverage for database create/drop rules with MySQL/TiDB-only gating
// pos: domain DDL rule test coverage for MySQL/TiDB database lifecycle
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
// Positive tests: rules fire for matching MySQL/TiDB statements
// ---------------------------------------------------------------------------

func TestDatabaseCreateNoticeRule(t *testing.T) {
	t.Parallel()
	r := mustNewDatabaseCreateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: dialect,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationCreateSchema,
					ObjectType: "database",
					ObjectName: "app",
				},
			}

			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			f := findings[0]
			if f.Level != rule.LevelNotice {
				t.Fatalf("expected notice level, got %q", f.Level)
			}
			if f.Metadata["object_type"] != "database" {
				t.Fatalf("expected object_type=database, got %v", f.Metadata["object_type"])
			}
			if f.Metadata["object_name"] != "app" {
				t.Fatalf("expected object_name=app, got %v", f.Metadata["object_name"])
			}
		})
	}
}

func TestDatabaseDropWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDatabaseDropWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: dialect,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSchema,
					ObjectType: "database",
					ObjectName: "app",
				},
			}

			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].Level != rule.LevelWarning {
				t.Fatalf("expected warning level, got %q", findings[0].Level)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Dialect isolation: database rules must not fire for PostgreSQL
// ---------------------------------------------------------------------------

func TestDatabaseLifecycleRulesDialectIsolation(t *testing.T) {
	t.Parallel()

	rules := []struct {
		name string
		r    rule.StatementRule
	}{
		{"create_notice", mustNewDatabaseCreateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})},
		{"drop_warn", mustNewDatabaseDropWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})},
	}

	for _, rl := range rules {
		t.Run(rl.name+"_skips_postgresql", func(t *testing.T) {
			t.Parallel()
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationCreateSchema,
					ObjectType: "schema",
					ObjectName: "app",
				},
			}
			if rl.r.AppliesTo(stmt) {
				t.Fatalf("expected database lifecycle rule %q not to apply to PostgreSQL", rl.name)
			}
			findings, err := rl.r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings for PostgreSQL, got %d", len(findings))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Negative: database rules ignore schema object type (PG create_schema)
// ---------------------------------------------------------------------------

func TestDatabaseLifecycleRulesIgnoreSchemaObjectType(t *testing.T) {
	t.Parallel()
	r := mustNewDatabaseCreateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateSchema,
			ObjectType: "schema",
			ObjectName: "app",
		},
	}

	if r.AppliesTo(stmt) {
		t.Fatalf("expected database create notice rule not to apply when ObjectType is schema, not database")
	}
}

// ---------------------------------------------------------------------------
// Negative: database rules skip wrong operation
// ---------------------------------------------------------------------------

func TestDatabaseLifecycleRulesSkipWrongOperation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		r         rule.StatementRule
		operation spec.DDLOperation
	}{
		{"create_notice_skips_drop", mustNewDatabaseCreateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}), spec.DDLOperationDropSchema},
		{"drop_warn_skips_create", mustNewDatabaseDropWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}), spec.DDLOperationCreateSchema},
		{"create_notice_skips_alter", mustNewDatabaseCreateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}), spec.DDLOperationAlterTable},
		{"drop_warn_skips_drop_table", mustNewDatabaseDropWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}), spec.DDLOperationDropTable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectMySQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "database",
					ObjectName: "app",
				},
			}
			if tc.r.AppliesTo(stmt) {
				t.Fatalf("expected rule not to apply for operation %q", tc.operation)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

func TestDatabaseLifecycleRuleDefaults(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()

	expected := []struct {
		id    string
		level rule.Level
	}{
		{ruleIDDatabaseCreateNotice, rule.LevelNotice},
		{ruleIDDatabaseDropWarn, rule.LevelWarning},
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

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRegisterIncludesDatabaseLifecycleRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("create_database_fires_for_mysql", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationCreateSchema,
				ObjectType: "database",
				ObjectName: "app",
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDDatabaseCreateNotice {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected database create notice rule to fire for MySQL, got %d findings", len(findings))
		}
	})

	t.Run("drop_database_fires_for_tidb", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationDropSchema,
				ObjectType: "database",
				ObjectName: "app",
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDDatabaseDropWarn {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected database drop warn rule to fire for TiDB, got %d findings", len(findings))
		}
	})

	t.Run("database_rules_do_not_fire_for_postgresql", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationCreateSchema,
				ObjectType: "schema",
				ObjectName: "app",
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		dbRuleIDs := map[string]bool{
			ruleIDDatabaseCreateNotice: true,
			ruleIDDatabaseDropWarn:     true,
		}
		for _, f := range findings {
			if dbRuleIDs[f.RuleID] {
				t.Fatalf("expected database rule %q not to fire for PostgreSQL", f.RuleID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewDatabaseCreateNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDatabaseCreateNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new database create notice rule: %v", err)
	}
	return r
}

func mustNewDatabaseDropWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDatabaseDropWarnRule(cfg)
	if err != nil {
		t.Fatalf("new database drop warn rule: %v", err)
	}
	return r
}
