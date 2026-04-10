// Package ddl verifies PostgreSQL migration-safety rule behavior.
// input: synthetic DDL statements with PostgreSQL migration-safety signals and cross-dialect policy controls
// output: focused coverage for the four PostgreSQL migration-safety rules with PG-only gating
// pos: domain DDL rule test coverage for PG migration safety
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Rule 1: ddl.pg.create_index.concurrently.require
// ---------------------------------------------------------------------------

func TestCreateIndexConcurrentlyRequiredRule(t *testing.T) {
	r := mustNewCreateIndexConcurrentlyRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateIndex,
			Table:     &spec.Table{Schema: "public", Name: "users"},
			Indexes:   []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
			Options:   map[string]string{"concurrently": "false"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", f.Level)
	}
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	if f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected explanation with why/risk/suggestion, got %+v", f.Explanation)
	}
}

func TestCreateIndexConcurrentlyRequiredRuleSkipsWhenConcurrent(t *testing.T) {
	r := mustNewCreateIndexConcurrentlyRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateIndex,
			Table:     &spec.Table{Schema: "public", Name: "users"},
			Indexes:   []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
			Options:   map[string]string{"concurrently": "true"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for concurrent index, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Rule 2: ddl.pg.alter.add_column.non_null_default.rewrite.warn
// ---------------------------------------------------------------------------

func TestAddColumnNonNullDefaultRewriteWarnRule(t *testing.T) {
	r := mustNewAddColumnNonNullDefaultRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_column",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name:       "status",
					Type:       "varchar(32)",
					NotNull:    true,
					HasDefault: true,
				},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
}

func TestAddColumnNonNullDefaultRewriteWarnRuleSkipsWhenNullable(t *testing.T) {
	r := mustNewAddColumnNonNullDefaultRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_column",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name:       "status",
					Type:       "varchar(32)",
					NotNull:    false,
					HasDefault: true,
				},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for nullable column, got %d", len(findings))
	}
}

func TestAddColumnNonNullDefaultRewriteWarnRuleSkipsWhenNoDefault(t *testing.T) {
	r := mustNewAddColumnNonNullDefaultRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_column",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name:       "status",
					Type:       "varchar(32)",
					NotNull:    true,
					HasDefault: false,
				},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for column without default, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Rule 3: ddl.pg.alter.add_check.not_valid.require
// ---------------------------------------------------------------------------

func TestAddCheckNotValidRule(t *testing.T) {
	r := mustNewAddCheckNotValidRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_constraint",
			Name:   "chk_amount",
			Options: map[string]string{
				"constraint_type": "check",
				"not_valid":       "false",
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
}

func TestAddCheckNotValidRuleSkipsWhenNotValidPresent(t *testing.T) {
	r := mustNewAddCheckNotValidRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_constraint",
			Name:   "chk_amount",
			Options: map[string]string{
				"constraint_type": "check",
				"not_valid":       "true",
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when NOT VALID present, got %d", len(findings))
	}
}

func TestAddCheckNotValidRuleSkipsWhenNotCheckConstraint(t *testing.T) {
	r := mustNewAddCheckNotValidRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_constraint",
			Name:   "uniq_email",
			Options: map[string]string{
				"constraint_type": "unique",
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-check constraint, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Rule 4: ddl.pg.alter.set_data_type.rewrite.warn
// ---------------------------------------------------------------------------

func TestSetDataTypeRewriteWarnRule(t *testing.T) {
	r := mustNewSetDataTypeRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "set_data_type",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{Name: "status", Type: "text"},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
}

func TestSetDataTypeRewriteWarnRuleSkipsWhenNotSetDataType(t *testing.T) {
	r := mustNewSetDataTypeRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "set_default",
			Name:   "status",
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-set_data_type action, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Cross-dialect: PG-only enforcement
// ---------------------------------------------------------------------------

func TestPostgreSQLMigrationRulesArePGOnly(t *testing.T) {
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "create_index_concurrently",
			r:    mustNewCreateIndexConcurrentlyRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationCreateIndex,
					Table:     &spec.Table{Name: "users"},
					Indexes:   []spec.Index{{Name: "idx_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
					Options:   map[string]string{"concurrently": "false"},
				},
			},
		},
		{
			name: "add_column_non_null_default",
			r:    mustNewAddColumnNonNullDefaultRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: alterStatement(
				spec.Alter{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{Definition: &spec.Column{Name: "status", NotNull: true, HasDefault: true}},
				},
			),
		},
		{
			name: "add_check_not_valid",
			r:    mustNewAddCheckNotValidRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: alterStatement(
				spec.Alter{
					Action:  "add_constraint",
					Name:    "chk_amount",
					Options: map[string]string{"constraint_type": "check", "not_valid": "false"},
				},
			),
		},
		{
			name: "set_data_type_rewrite",
			r:    mustNewSetDataTypeRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: alterStatement(
				spec.Alter{Action: "set_data_type", Name: "status"},
			),
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

// ---------------------------------------------------------------------------
// Registry-level: prove rules are registered
// ---------------------------------------------------------------------------

func TestRegisterIncludesPostgreSQLMigrationRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	pgMigrationRuleIDs := []string{
		ruleIDPGCreateIndexConcurrentlyRequire,
		ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn,
		ruleIDPGAlterAddCheckNotValidRequire,
		ruleIDPGAlterSetDataTypeRewriteWarn,
	}
	for _, ruleID := range pgMigrationRuleIDs {
		cfg.Rules[ruleID] = policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelWarning,
		}
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("create_index_concurrently_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationCreateIndex,
				Table:     &spec.Table{Name: "users"},
				Indexes:   []spec.Index{{Name: "idx_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
				Options:   map[string]string{"concurrently": "false"},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGCreateIndexConcurrentlyRequire {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected create_index_concurrently rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("add_column_non_null_default_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{Definition: &spec.Column{Name: "status", NotNull: true, HasDefault: true}},
				}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected add_column_non_null_default rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("add_check_not_valid_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "orders"},
				Alter: []spec.Alter{{
					Action: "add_constraint",
					Name:   "chk_amount",
					Options: map[string]string{
						"constraint_type": "check",
						"not_valid":       "false",
					},
				}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterAddCheckNotValidRequire {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected add_check_not_valid rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("set_data_type_rewrite_fires_for_pg", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "set_data_type",
					Name:   "status",
					Column: &spec.AlterColumn{Definition: &spec.Column{Name: "status", Type: "text"}},
				}},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterSetDataTypeRewriteWarn {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected set_data_type_rewrite rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("pg_migration_rules_do_not_fire_for_mysql", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{Action: "set_data_type", Name: "status"},
					{Action: "add_column", Name: "score", Column: &spec.AlterColumn{Definition: &spec.Column{Name: "score", NotNull: true, HasDefault: true}}},
					{Action: "add_constraint", Name: "chk_amount", Options: map[string]string{"constraint_type": "check", "not_valid": "false"}},
				},
			},
		}
		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		pgRuleIDs := map[string]bool{
			ruleIDPGCreateIndexConcurrentlyRequire:        true,
			ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn: true,
			ruleIDPGAlterAddCheckNotValidRequire:          true,
			ruleIDPGAlterSetDataTypeRewriteWarn:           true,
		}
		for _, f := range findings {
			if pgRuleIDs[f.RuleID] {
				t.Fatalf("expected PG migration rule %q not to fire for MySQL", f.RuleID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Task 4: Suggestion quality pass — actionable phrase assertions
// ---------------------------------------------------------------------------

func TestCreateIndexConcurrentlyRequiredRuleProvidesActionableSuggestion(t *testing.T) {
	r := mustNewCreateIndexConcurrentlyRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateIndex,
			Table:     &spec.Table{Schema: "public", Name: "users"},
			Indexes:   []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
			Options:   map[string]string{"concurrently": "false"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	if !strings.Contains(f.Explanation.Suggestion, "CONCURRENTLY") {
		t.Fatalf("expected suggestion to mention CONCURRENTLY, got %q", f.Explanation.Suggestion)
	}
	if !strings.Contains(strings.ToLower(f.Explanation.Suggestion), "transaction") {
		t.Fatalf("expected suggestion to mention transaction limitations, got %q", f.Explanation.Suggestion)
	}
}

func TestAddColumnNonNullDefaultRewriteWarnRuleProvidesSaferMigrationSuggestion(t *testing.T) {
	r := mustNewAddColumnNonNullDefaultRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_column",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name:       "status",
					Type:       "varchar(32)",
					NotNull:    true,
					HasDefault: true,
				},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	s := strings.ToLower(f.Explanation.Suggestion)
	if !strings.Contains(s, "nullable") {
		t.Fatalf("expected suggestion to mention nullable, got %q", f.Explanation.Suggestion)
	}
	if !strings.Contains(s, "backfill") {
		t.Fatalf("expected suggestion to mention backfill, got %q", f.Explanation.Suggestion)
	}
}

func TestAddCheckNotValidRuleProvidesValidationFlowSuggestion(t *testing.T) {
	r := mustNewAddCheckNotValidRequiredRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_constraint",
			Name:   "chk_amount",
			Options: map[string]string{
				"constraint_type": "check",
				"not_valid":       "false",
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	s := strings.ToLower(f.Explanation.Suggestion)
	if !strings.Contains(s, "not valid") {
		t.Fatalf("expected suggestion to mention NOT VALID, got %q", f.Explanation.Suggestion)
	}
	if !strings.Contains(s, "validate constraint") {
		t.Fatalf("expected suggestion to mention VALIDATE CONSTRAINT, got %q", f.Explanation.Suggestion)
	}
}

func TestSetDataTypeRewriteWarnRuleProvidesPhasedMigrationSuggestion(t *testing.T) {
	r := mustNewSetDataTypeRewriteWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "set_data_type",
			Name:   "status",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{Name: "status", Type: "text"},
			},
		},
	)

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	s := strings.ToLower(f.Explanation.Suggestion)
	if !strings.Contains(s, "shadow") {
		t.Fatalf("expected suggestion to mention shadow column approach, got %q", f.Explanation.Suggestion)
	}
	if !strings.Contains(s, "phased") {
		t.Fatalf("expected suggestion to mention phased migration, got %q", f.Explanation.Suggestion)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func mustNewCreateIndexConcurrentlyRequiredRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateIndexConcurrentlyRequiredRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

func mustNewAddColumnNonNullDefaultRewriteWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAddColumnNonNullDefaultRewriteWarnRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

func mustNewAddCheckNotValidRequiredRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAddCheckNotValidRequiredRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

func mustNewSetDataTypeRewriteWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetDataTypeRewriteWarnRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}
