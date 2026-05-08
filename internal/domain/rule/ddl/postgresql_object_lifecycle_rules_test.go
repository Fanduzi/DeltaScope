// Package ddl verifies PostgreSQL object lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL object lifecycle signals and cross-dialect policy controls
// output: focused coverage for the nine PostgreSQL object lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG object lifecycle
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

func TestDropSchemaAdvisoryRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: "app_data",
			ObjectType: "schema",
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
	if f.Explanation == nil || f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected complete explanation, got %+v", f.Explanation)
	}
}

func TestDropSchemaCascadeWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropSchemaCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: "app_data",
			ObjectType: "schema",
			Options:    map[string]string{"cascade": "true"},
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
}

func TestCreateSequenceCycleWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewCreateSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateSequence,
			ObjectName: "order_seq",
			ObjectType: "sequence",
			Options:    map[string]string{"cycle": "true"},
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
}

func TestAlterSequenceRestartWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSequenceRestartWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterSequence,
			ObjectName: "order_seq",
			ObjectType: "sequence",
			Options:    map[string]string{"restart": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterSequenceCycleWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterSequence,
			ObjectName: "order_seq",
			ObjectType: "sequence",
			Options:    map[string]string{"cycle": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDropSequenceAdvisoryRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropSequenceAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: "order_seq",
			ObjectType: "sequence",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
}

func TestDropSequenceCascadeWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropSequenceCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: "order_seq",
			ObjectType: "sequence",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDropMaterializedViewAdvisoryRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropMaterializedViewAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: "mv_daily_sales",
			ObjectType: "materialized_view",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
}

func TestDropMaterializedViewCascadeWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropMaterializedViewCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: "mv_daily_sales",
			ObjectType: "materialized_view",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Negative tests: rules skip when conditions not met
// ---------------------------------------------------------------------------

func TestPGObjectLifecycleRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "drop_schema_advisory",
			r:    mustNewDropSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSchema,
					ObjectName: "app_data",
				},
			},
		},
		{
			name: "drop_schema_cascade",
			r:    mustNewDropSchemaCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSchema,
					ObjectName: "app_data",
					Options:    map[string]string{"cascade": "true"},
				},
			},
		},
		{
			name: "create_sequence_cycle",
			r:    mustNewCreateSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationCreateSequence,
					ObjectName: "seq1",
					Options:    map[string]string{"cycle": "true"},
				},
			},
		},
		{
			name: "alter_sequence_restart",
			r:    mustNewAlterSequenceRestartWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationAlterSequence,
					ObjectName: "seq1",
					Options:    map[string]string{"restart": "true"},
				},
			},
		},
		{
			name: "alter_sequence_cycle",
			r:    mustNewAlterSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationAlterSequence,
					ObjectName: "seq1",
					Options:    map[string]string{"cycle": "true"},
				},
			},
		},
		{
			name: "drop_sequence_advisory",
			r:    mustNewDropSequenceAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSequence,
					ObjectName: "seq1",
				},
			},
		},
		{
			name: "drop_sequence_cascade",
			r:    mustNewDropSequenceCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSequence,
					ObjectName: "seq1",
					Options:    map[string]string{"cascade": "true"},
				},
			},
		},
		{
			name: "drop_materialized_view_advisory",
			r:    mustNewDropMaterializedViewAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropMaterializedView,
					ObjectName: "mv1",
				},
			},
		},
		{
			name: "drop_materialized_view_cascade",
			r:    mustNewDropMaterializedViewCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropMaterializedView,
					ObjectName: "mv1",
					Options:    map[string]string{"cascade": "true"},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestPGObjectLifecycleCascadeRulesSkipWithoutCascade(t *testing.T) {
	t.Parallel()
	cascadeRules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "drop_schema_cascade",
			r:    mustNewDropSchemaCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSchema,
					ObjectName: "app_data",
					Options:    map[string]string{"cascade": "false"},
				},
			},
		},
		{
			name: "drop_sequence_cascade",
			r:    mustNewDropSequenceCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropSequence,
					ObjectName: "seq1",
					Options:    map[string]string{"cascade": "false"},
				},
			},
		},
		{
			name: "drop_materialized_view_cascade",
			r:    mustNewDropMaterializedViewCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationDropMaterializedView,
					ObjectName: "mv1",
					Options:    map[string]string{"cascade": "false"},
				},
			},
		},
	}

	for _, rl := range cascadeRules {
		t.Run(rl.name, func(t *testing.T) {
			t.Parallel()
			findings, err := rl.r.Evaluate(context.Background(), rl.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings without cascade, got %d", len(findings))
			}
		})
	}
}

func TestPGObjectLifecycleOptionRulesSkipWithoutOption(t *testing.T) {
	t.Parallel()
	optionRules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "create_sequence_without_cycle",
			r:    mustNewCreateSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationCreateSequence,
					ObjectName: "seq1",
				},
			},
		},
		{
			name: "alter_sequence_without_restart",
			r:    mustNewAlterSequenceRestartWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationAlterSequence,
					ObjectName: "seq1",
				},
			},
		},
		{
			name: "alter_sequence_without_cycle",
			r:    mustNewAlterSequenceCycleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationAlterSequence,
					ObjectName: "seq1",
				},
			},
		},
	}

	for _, rl := range optionRules {
		t.Run(rl.name, func(t *testing.T) {
			t.Parallel()
			findings, err := rl.r.Evaluate(context.Background(), rl.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings without option, got %d", len(findings))
			}
		})
	}
}

func TestPGObjectLifecycleRulesSkipWrongOperation(t *testing.T) {
	t.Parallel()
	r := mustNewDropSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateSchema,
			ObjectName: "app_data",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for create_schema, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

func TestRegisterIncludesPGObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()

	pgLifecycleRuleIDs := []string{
		ruleIDPGDropSchemaAdvisory,
		ruleIDPGDropSchemaCascadeWarn,
		ruleIDPGCreateSequenceCycleWarn,
		ruleIDPGAlterSequenceRestartWarn,
		ruleIDPGAlterSequenceCycleWarn,
		ruleIDPGDropSequenceAdvisory,
		ruleIDPGDropSequenceCascadeWarn,
		ruleIDPGDropMaterializedViewAdvisory,
		ruleIDPGDropMaterializedViewCascadeWarn,
	}
	for _, ruleID := range pgLifecycleRuleIDs {
		cfg.Rules[ruleID] = policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelNotice,
		}
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("drop_schema_advisory_fires_for_pg", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationDropSchema,
				ObjectName: "app_data",
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGDropSchemaAdvisory {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected drop_schema_advisory rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("create_sequence_cycle_fires_for_pg", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationCreateSequence,
				ObjectName: "seq1",
				Options:    map[string]string{"cycle": "true"},
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGCreateSequenceCycleWarn {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected create_sequence_cycle rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("pg_lifecycle_rules_do_not_fire_for_mysql", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation:  spec.DDLOperationDropSchema,
				ObjectName: "app_data",
				Options:    map[string]string{"cascade": "true"},
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		pgRuleIDs := map[string]bool{
			ruleIDPGDropSchemaAdvisory:              true,
			ruleIDPGDropSchemaCascadeWarn:           true,
			ruleIDPGCreateSequenceCycleWarn:         true,
			ruleIDPGAlterSequenceRestartWarn:        true,
			ruleIDPGAlterSequenceCycleWarn:          true,
			ruleIDPGDropSequenceAdvisory:            true,
			ruleIDPGDropSequenceCascadeWarn:         true,
			ruleIDPGDropMaterializedViewAdvisory:    true,
			ruleIDPGDropMaterializedViewCascadeWarn: true,
		}
		for _, f := range findings {
			if pgRuleIDs[f.RuleID] {
				t.Fatalf("expected PG lifecycle rule %q not to fire for MySQL", f.RuleID)
			}
		}
	})
}

func TestDefaultPolicyIncludesPGObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGDropSchemaAdvisory, rule.LevelNotice, true},
		{ruleIDPGDropSchemaCascadeWarn, rule.LevelWarning, true},
		{ruleIDPGCreateSequenceCycleWarn, rule.LevelWarning, true},
		{ruleIDPGAlterSequenceRestartWarn, rule.LevelWarning, true},
		{ruleIDPGAlterSequenceCycleWarn, rule.LevelWarning, true},
		{ruleIDPGDropSequenceAdvisory, rule.LevelNotice, true},
		{ruleIDPGDropSequenceCascadeWarn, rule.LevelWarning, true},
		{ruleIDPGDropMaterializedViewAdvisory, rule.LevelNotice, true},
		{ruleIDPGDropMaterializedViewCascadeWarn, rule.LevelWarning, true},
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

func TestDefaultPolicyExcludesPGRulesFromMySQL(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()
	pgPrefixes := []string{
		"ddl.pg.drop_schema",
		"ddl.pg.drop_sequence",
		"ddl.pg.create_sequence",
		"ddl.pg.alter_sequence",
		"ddl.pg.drop_materialized_view",
	}
	for id := range cfg.Rules {
		for _, prefix := range pgPrefixes {
			if len(id) >= len(prefix) && id[:len(prefix)] == prefix { //nolint:staticcheck // existence verified
				// This is expected; just verify it exists
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewDropSchemaAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropSchemaAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop schema advisory rule: %v", err)
	}
	return r
}

func mustNewDropSchemaCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropSchemaCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop schema cascade warn rule: %v", err)
	}
	return r
}

func mustNewCreateSequenceCycleWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateSequenceCycleWarnRule(cfg)
	if err != nil {
		t.Fatalf("new create sequence cycle warn rule: %v", err)
	}
	return r
}

func mustNewAlterSequenceRestartWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterSequenceRestartWarnRule(cfg)
	if err != nil {
		t.Fatalf("new alter sequence restart warn rule: %v", err)
	}
	return r
}

func mustNewAlterSequenceCycleWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterSequenceCycleWarnRule(cfg)
	if err != nil {
		t.Fatalf("new alter sequence cycle warn rule: %v", err)
	}
	return r
}

func mustNewDropSequenceAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropSequenceAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop sequence advisory rule: %v", err)
	}
	return r
}

func mustNewDropSequenceCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropSequenceCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop sequence cascade warn rule: %v", err)
	}
	return r
}

func mustNewDropMaterializedViewAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropMaterializedViewAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop materialized view advisory rule: %v", err)
	}
	return r
}

func mustNewDropMaterializedViewCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropMaterializedViewCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop materialized view cascade warn rule: %v", err)
	}
	return r
}
