// Package ddl verifies PostgreSQL alter table advisory rule behavior.
// input: synthetic DDL statements with PostgreSQL alter table signals and cross-dialect policy controls
// output: focused coverage for the three PG-only alter table gap rules with PG-only gating
// pos: domain DDL rule test coverage for PG alter table advisories
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Positive tests
// ---------------------------------------------------------------------------

func TestDropColumnAdvisoryFiresForPG(t *testing.T) {
	r := mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_column", Name: "email"},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "drop_column" {
		t.Fatalf("expected action=drop_column, got %v", findings[0].Metadata["action"])
	}
}

func TestValidateConstraintAdvisoryFiresForPG(t *testing.T) {
	r := mustNewValidateConstraintAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{
				{Action: "validate_constraint", Name: "chk_positive_amount"},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
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

func TestAddColumnNullableNoticeFiresForPG(t *testing.T) {
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "nickname",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:   "nickname",
							Type:   "varchar(100)",
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
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

// ---------------------------------------------------------------------------
// Negative tests: nullable add-column skips covered cases
// ---------------------------------------------------------------------------

func TestAddColumnNullableSkipsNotNullWithDefault(t *testing.T) {
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:       "status",
							Type:       "varchar(20)",
							NotNull:    true,
							HasDefault: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for NOT NULL DEFAULT, got %d", len(findings))
	}
}

func TestAddColumnNullableSkipsNotNullNoDefault(t *testing.T) {
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:    "status",
							Type:    "varchar(20)",
							NotNull: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for NOT NULL without default, got %d", len(findings))
	}
}

func TestAddColumnNullableSkipsHasDefault(t *testing.T) {
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:       "status",
							Type:       "varchar(20)",
							HasDefault: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for nullable with default, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Cross-dialect negative tests
// ---------------------------------------------------------------------------

func TestPGAlterTableRulesSkipNonPGDialects(t *testing.T) {
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	baseStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_column", Name: "email"},
			},
		},
	}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "drop_column_advisory",
			r:    mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: baseStmt,
		},
		{
			name: "validate_constraint_advisory",
			r:    mustNewValidateConstraintAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter: []spec.Alter{
						{Action: "validate_constraint", Name: "chk"},
					},
				},
			},
		},
		{
			name: "add_column_nullable_notice",
			r:    mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter: []spec.Alter{
						{
							Action: "add_column",
							Name:   "nick",
							Column: &spec.AlterColumn{
								Definition: &spec.Column{Name: "nick", Type: "text"},
							},
						},
					},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(stmt)
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

func TestPGAlterTableRulesSkipWrongAction(t *testing.T) {
	r := mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "add_column", Name: "email"},
			},
		},
	}

	if r.AppliesTo(stmt) {
		t.Fatalf("expected AppliesTo() == false for wrong action")
	}
	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for wrong action, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

func TestRegisterIncludesPGAlterTableRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	pgAlterRuleIDs := []string{
		ruleIDPGAlterDropColumnAdvisory,
		ruleIDPGAlterValidateConstraintAdvisory,
		ruleIDPGAlterAddColumnNullableNotice,
	}
	for _, ruleID := range pgAlterRuleIDs {
		cfg.Rules[ruleID] = policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelNotice,
		}
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("drop_column_advisory_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter:     []spec.Alter{{Action: "drop_column", Name: "email"}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterDropColumnAdvisory {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected drop_column_advisory rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("validate_constraint_advisory_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "orders"},
				Alter:     []spec.Alter{{Action: "validate_constraint", Name: "chk_amt"}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterValidateConstraintAdvisory {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected validate_constraint_advisory rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("pg_alter_table_rules_do_not_fire_for_mysql", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter:     []spec.Alter{{Action: "drop_column", Name: "email"}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		pgRuleIDs := map[string]bool{
			ruleIDPGAlterDropColumnAdvisory:         true,
			ruleIDPGAlterValidateConstraintAdvisory: true,
			ruleIDPGAlterAddColumnNullableNotice:    true,
		}
		for _, f := range findings {
			if pgRuleIDs[f.RuleID] {
				t.Fatalf("expected PG rule %q not to fire for MySQL", f.RuleID)
			}
		}
	})
}

func TestDefaultPolicyIncludesPGAlterTableRules(t *testing.T) {
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGAlterDropColumnAdvisory, rule.LevelWarning, true},
		{ruleIDPGAlterValidateConstraintAdvisory, rule.LevelNotice, true},
		{ruleIDPGAlterAddColumnNullableNotice, rule.LevelNotice, true},
	}

	for _, exp := range expected {
		t.Run(exp.id, func(t *testing.T) {
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
// Helpers
// ---------------------------------------------------------------------------

func mustNewDropColumnAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropColumnAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop column advisory rule: %v", err)
	}
	return r
}

func mustNewValidateConstraintAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newValidateConstraintAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new validate constraint advisory rule: %v", err)
	}
	return r
}

func mustNewAddColumnNullableNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAddColumnNullableNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new add column nullable notice rule: %v", err)
	}
	return r
}
