// Package ddl verifies PG-native alter-action restriction behavior.
// input: synthetic alter-table statements with PG-native actions and dialect-specific policy overrides
// output: focused coverage for PG-native alter action forbid rules with dialect gating
// pos: domain DDL rule test coverage for PG alter governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// pgAlterActionCase is a test case descriptor for PG-native alter action forbid rules.
type pgAlterActionCase struct {
	name    string
	action  string
	dialect spec.Dialect
	forbid  bool
	// expected
	wantFindings int
	wantApplies  bool
}

func TestPGAlterActionForbidRules(t *testing.T) {
	pgForbidConfig := policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	}

	// The five PG-native actions that get their own forbid rules.
	pgActions := []struct {
		ruleID string
		action string
		label  string
	}{
		{ruleIDAlterSetDataTypeForbid, "set_data_type", "set data type"},
		{ruleIDAlterSetDefaultForbid, "set_default", "set default"},
		{ruleIDAlterDropDefaultForbid, "drop_default", "drop default"},
		{ruleIDAlterSetNotNullForbid, "set_not_null", "set not null"},
		{ruleIDAlterDropNotNullForbid, "drop_not_null", "drop not null"},
	}

	for _, pg := range pgActions {
		t.Run(pg.ruleID, func(t *testing.T) {
			r, err := newForbiddenAlterActionRule(
				pg.ruleID, pg.action, pg.label,
				rule.LevelWarning, pgForbidConfig,
				withDialectAllowlist(spec.DialectPostgreSQL),
			)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			// --- Positive: PG + matching action -> 1 finding ---
			t.Run("positive_pg_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 1 {
					t.Fatalf("expected 1 finding, got %d", len(findings))
				}
			})

			// --- Negative: MySQL + matching action -> 0 findings ---
			t.Run("negative_mysql_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectMySQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for MySQL, got %d", len(findings))
				}
			})

			// --- Negative: TiDB + matching action -> 0 findings ---
			t.Run("negative_tidb_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectTiDB,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for TiDB, got %d", len(findings))
				}
			})

			// --- Negative: PG + wrong action -> 0 findings ---
			t.Run("negative_pg_wrong_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: "modify_column", Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for wrong action, got %d", len(findings))
				}
			})

			// --- Negative: PG + add_column -> 0 findings ---
			t.Run("negative_pg_add_column", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: "add_column", Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for add_column, got %d", len(findings))
				}
			})

			// --- AppliesTo boundary: PG + matching action -> AppliesTo() == true ---
			t.Run("applies_to_pg_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if !r.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == true for PG + matching action")
				}
			})

			// --- AppliesTo boundary: MySQL + matching action -> AppliesTo() == false ---
			t.Run("applies_to_mysql_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectMySQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if r.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == false for MySQL + matching action")
				}
			})

			// --- AppliesTo boundary: forbid:false -> AppliesTo() == false ---
			t.Run("applies_to_forbid_false", func(t *testing.T) {
				noForbidConfig := policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"forbid": false},
				}
				rNoForbid, err := newForbiddenAlterActionRule(
					pg.ruleID, pg.action, pg.label,
					rule.LevelWarning, noForbidConfig,
					withDialectAllowlist(spec.DialectPostgreSQL),
				)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if rNoForbid.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == false when forbid:false")
				}
			})
		})
	}
}

func TestPGAlterActionForbidRulesForbidFalse(t *testing.T) {
	// forbid:false -> Evaluate returns 0 findings even for PG + matching action
	noForbidConfig := policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": false},
	}
	r, err := newForbiddenAlterActionRule(
		ruleIDAlterSetDataTypeForbid, "set_data_type", "set data type",
		rule.LevelWarning, noForbidConfig,
		withDialectAllowlist(spec.DialectPostgreSQL),
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{Action: "set_data_type", Name: "col1"},
	)
	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when forbid:false, got %d", len(findings))
	}
}

func TestExistingAlterActionRulesZeroValueCompatibility(t *testing.T) {
	// Verify that existing rules (e.g. drop_column.forbid) without dialectAllowlist
	// retain their old behavior: they fire for ANY dialect.
	cfg := policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	}

	r, err := newForbiddenAlterActionRule(
		ruleIDAlterDropColumnForbid, "drop_column", "drop column",
		rule.LevelWarning, cfg,
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	for _, tc := range []struct {
		dialect spec.Dialect
	}{
		{spec.DialectMySQL},
		{spec.DialectTiDB},
		{spec.DialectPostgreSQL},
		{spec.DialectUnknown},
	} {
		t.Run("dialect_"+string(tc.dialect), func(t *testing.T) {
			stmt := alterStatementWithDialect(tc.dialect,
				spec.Alter{Action: "drop_column", Name: "old_col"},
			)
			// AppliesTo must return true for all dialects
			if !r.AppliesTo(stmt) {
				t.Fatalf("expected AppliesTo() == true for dialect %s", tc.dialect)
			}
			findings, err := r.Evaluate(stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for dialect %s, got %d", tc.dialect, len(findings))
			}
		})
	}

	// Also verify forbid:false still works as before
	t.Run("forbid_false", func(t *testing.T) {
		noForbidCfg := policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{"forbid": false},
		}
		rNoForbid, err := newForbiddenAlterActionRule(
			ruleIDAlterDropColumnForbid, "drop_column", "drop column",
			rule.LevelWarning, noForbidCfg,
		)
		if err != nil {
			t.Fatalf("new rule: %v", err)
		}
		stmt := alterStatementWithDialect(spec.DialectMySQL,
			spec.Alter{Action: "drop_column", Name: "old_col"},
		)
		if rNoForbid.AppliesTo(stmt) {
			t.Fatal("expected AppliesTo() == false when forbid:false for existing rule")
		}
		findings, err := rNoForbid.Evaluate(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings when forbid:false, got %d", len(findings))
		}
	})
}

// TestPGGeneratedIdentityForbidRules covers the three PG-only generated/identity
// state-transition forbid rules: drop_expression, set_generated, drop_identity.
// These rules are registered in Register() and only fire for PostgreSQL dialect.
func TestPGGeneratedIdentityForbidRules(t *testing.T) {
	pgForbidConfig := policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	}

	// The three PG-generated/identity actions that get their own forbid rules.
	pgGenIdentActions := []struct {
		ruleID string
		action string
		label  string
	}{
		{ruleIDAlterDropExpressionForbid, "drop_expression", "drop expression"},
		{ruleIDAlterSetGeneratedForbid, "set_generated", "set generated"},
		{ruleIDAlterDropIdentityForbid, "drop_identity", "drop identity"},
	}

	for _, pg := range pgGenIdentActions {
		t.Run(pg.ruleID, func(t *testing.T) {
			r, err := newForbiddenAlterActionRule(
				pg.ruleID, pg.action, pg.label,
				rule.LevelWarning, pgForbidConfig,
				withDialectAllowlist(spec.DialectPostgreSQL),
			)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			// --- Positive: PG + matching action -> 1 finding ---
			t.Run("positive_pg_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 1 {
					t.Fatalf("expected 1 finding, got %d", len(findings))
				}
			})

			// --- Negative: MySQL + matching action -> 0 findings ---
			t.Run("negative_mysql_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectMySQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for MySQL, got %d", len(findings))
				}
			})

			// --- Negative: TiDB + matching action -> 0 findings ---
			t.Run("negative_tidb_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectTiDB,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for TiDB, got %d", len(findings))
				}
			})

			// --- Negative: PG + wrong action -> 0 findings ---
			t.Run("negative_pg_wrong_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: "modify_column", Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for wrong action, got %d", len(findings))
				}
			})

			// --- Negative: PG + add_column -> not caught by these rules ---
			t.Run("negative_pg_add_column", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: "add_column", Name: "col1"},
				)
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for add_column, got %d", len(findings))
				}
			})

			// --- AppliesTo boundary: PG + matching action -> true ---
			t.Run("applies_to_pg_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if !r.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == true for PG + matching action")
				}
			})

			// --- AppliesTo boundary: MySQL + matching action -> false ---
			t.Run("applies_to_mysql_matching_action", func(t *testing.T) {
				stmt := alterStatementWithDialect(spec.DialectMySQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if r.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == false for MySQL + matching action")
				}
			})

			// --- AppliesTo boundary: forbid:false -> false ---
			t.Run("applies_to_forbid_false", func(t *testing.T) {
				noForbidConfig := policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelWarning,
					Params:  map[string]any{"forbid": false},
				}
				rNoForbid, err := newForbiddenAlterActionRule(
					pg.ruleID, pg.action, pg.label,
					rule.LevelWarning, noForbidConfig,
					withDialectAllowlist(spec.DialectPostgreSQL),
				)
				if err != nil {
					t.Fatalf("new rule: %v", err)
				}
				stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
					spec.Alter{Action: pg.action, Name: "col1"},
				)
				if rNoForbid.AppliesTo(stmt) {
					t.Fatal("expected AppliesTo() == false when forbid:false")
				}
			})
		})
	}
}

// TestPGGeneratedIdentityForbidRulesNoCrossFire proves the three new
// generated/identity forbid rules do NOT match any of the five existing
// PG-native alter action values (set_data_type, set_default, etc.).
func TestPGGeneratedIdentityForbidRulesNoCrossFire(t *testing.T) {
	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}}

	newActions := []struct {
		ruleID string
		action string
	}{
		{ruleIDAlterDropExpressionForbid, "drop_expression"},
		{ruleIDAlterSetGeneratedForbid, "set_generated"},
		{ruleIDAlterDropIdentityForbid, "drop_identity"},
	}

	existingPGActions := []string{"set_data_type", "set_default", "drop_default", "set_not_null", "drop_not_null"}

	for _, na := range newActions {
		r, err := newForbiddenAlterActionRule(
			na.ruleID, na.action, na.action,
			rule.LevelWarning, cfg,
			withDialectAllowlist(spec.DialectPostgreSQL),
		)
		if err != nil {
			t.Fatalf("new rule %s: %v", na.ruleID, err)
		}

		for _, existing := range existingPGActions {
			t.Run(na.action+"_does_not_match_"+existing, func(t *testing.T) {
				stmt := spec.Statement{
					Kind:    spec.KindDDL,
					Dialect: spec.DialectPostgreSQL,
					DDL: &spec.DDL{
						Table: &spec.Table{Name: "users"},
						Alter: []spec.Alter{{Action: existing, Name: "col1"}},
					},
				}
				findings, err := r.Evaluate(stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for action %q against rule %q, got %d",
						existing, na.ruleID, len(findings))
				}
			})
		}
	}
}

// alterStatementWithDialect builds a synthetic ALTER TABLE statement with a specific dialect.
func alterStatementWithDialect(dialect spec.Dialect, alters ...spec.Alter) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: alters,
		},
	}
}
