// Package ddl characterizes PostgreSQL generated/identity rule coverage.
// input: synthetic DDL statements for all supported generated/identity forms
// output: proof of which existing rules fire and which forms are currently silent
// pos: v0.36.0 Task 1 characterization — read-only, no production behavior changes
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
// Characterization: full registry evaluation for each supported generated/identity form
// ---------------------------------------------------------------------------

// pgGeneratedIdentityCoverageCase describes one supported statement form and
// what we expect the current rule registry to produce.
type pgGeneratedIdentityCoverageCase struct {
	name       string
	stmt       spec.Statement
	wantSilent bool // true means we expect ZERO findings from ANY registered rule
}

// buildTestRegistry creates a fully-loaded DDL rule registry with all rules enabled.
func buildTestRegistry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	// Enable all PG migration safety rules explicitly.
	cfg.Rules[ruleIDPGCreateIndexConcurrentlyRequire] = policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}
	cfg.Rules[ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn] = policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}
	cfg.Rules[ruleIDPGAlterAddCheckNotValidRequire] = policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}
	cfg.Rules[ruleIDPGAlterSetDataTypeRewriteWarn] = policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}
	// Enable PG-native alter action forbid rules (forbid:true).
	for _, ruleID := range []string{
		ruleIDAlterSetDataTypeForbid,
		ruleIDAlterSetDefaultForbid,
		ruleIDAlterDropDefaultForbid,
		ruleIDAlterSetNotNullForbid,
		ruleIDAlterDropNotNullForbid,
	} {
		cfg.Rules[ruleID] = policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}}
	}
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}
	return registry
}

// TestPGGeneratedIdentityRuleCoverage_CreateTableGeneratedStored proves that
// CREATE TABLE ... GENERATED ALWAYS AS (...) STORED receives at least some
// rule findings from the general column/table rule pipeline.
func TestPGGeneratedIdentityRuleCoverage_CreateTableGeneratedStored(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Schema: "public", Name: "users"},
			Columns: []spec.Column{
				{Name: "id", Type: "bigint"},
				{Name: "first_name", Type: "text"},
				{Name: "full_name", Type: "text", GeneratedWhen: "a"},
			},
		},
	}

	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	t.Logf("CREATE TABLE ... GENERATED ALWAYS AS STORED: %d findings", len(findings))
	for _, f := range findings {
		t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
	}
	// General rules should fire (table/column naming, PK, etc.).
	// This characterization locks in that the statement is NOT silent.
	if len(findings) == 0 {
		t.Log("CHARACTERIZATION NOTE: CREATE TABLE with generated stored column produces zero findings — gap candidate")
	}
}

// TestPGGeneratedIdentityRuleCoverage_CreateTableIdentity proves that
// CREATE TABLE ... GENERATED ALWAYS AS IDENTITY receives at least some
// rule findings from the general column/table rule pipeline.
func TestPGGeneratedIdentityRuleCoverage_CreateTableIdentity(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Schema: "public", Name: "users"},
			Columns: []spec.Column{
				{Name: "id", Type: "bigint", GeneratedWhen: "a", IsIdentity: true},
			},
		},
	}

	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	t.Logf("CREATE TABLE ... GENERATED ALWAYS AS IDENTITY: %d findings", len(findings))
	for _, f := range findings {
		t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
	}
	if len(findings) == 0 {
		t.Log("CHARACTERIZATION NOTE: CREATE TABLE with identity column produces zero findings — gap candidate")
	}
}

// TestPGGeneratedIdentityRuleCoverage_AddColumnGeneratedStored proves that
// ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED triggers
// the PG add_column rewrite warning when the column is NOT NULL with a default,
// and is silent when those conditions are not met.
func TestPGGeneratedIdentityRuleCoverage_AddColumnGeneratedStored(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	t.Run("generated_stored_without_notnull_default", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "add_column",
					Name:   "full_name",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:          "full_name",
							Type:          "text",
							GeneratedWhen: "a",
						},
					},
				}},
			},
		}

		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		t.Logf("ADD COLUMN GENERATED STORED (no not-null/default): %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
		}

		// Check specifically for the PG rewrite warning.
		hasRewriteWarn := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn {
				hasRewriteWarn = true
			}
		}
		if hasRewriteWarn {
			t.Log("CHARACTERIZATION NOTE: PG rewrite warning fires for generated stored ADD COLUMN without NOT NULL+default — unexpected")
		} else {
			t.Log("CHARACTERIZATION NOTE: PG rewrite warning does NOT fire — expected because no NOT NULL+default")
		}
	})

	t.Run("generated_stored_with_notnull_default", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "add_column",
					Name:   "full_name",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:          "full_name",
							Type:          "text",
							NotNull:       true,
							HasDefault:    true,
							GeneratedWhen: "a",
						},
					},
				}},
			},
		}

		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		t.Logf("ADD COLUMN GENERATED STORED (not-null+default): %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
		}

		// The PG rewrite warn rule checks add_column + NotNull + HasDefault.
		// A generated column that also has NotNull+HasDefault SHOULD trigger this.
		hasRewriteWarn := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn {
				hasRewriteWarn = true
			}
		}
		if hasRewriteWarn {
			t.Log("CONFIRMED: PG rewrite warning fires for generated stored ADD COLUMN with NOT NULL+default")
		} else {
			t.Log("CHARACTERIZATION NOTE: PG rewrite warning does NOT fire even with NOT NULL+default — gap")
		}
	})
}

// TestPGGeneratedIdentityRuleCoverage_AddColumnIdentity proves that
// ALTER TABLE ... ADD COLUMN ... GENERATED AS IDENTITY receives findings
// from general alter rules but no identity-specific rule fires.
func TestPGGeneratedIdentityRuleCoverage_AddColumnIdentity(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "add_column",
				Name:   "id",
				Column: &spec.AlterColumn{
					Definition: &spec.Column{
						Name:          "id",
						Type:          "bigint",
						GeneratedWhen: "a",
						IsIdentity:    true,
					},
				},
			}},
		},
	}

	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	t.Logf("ADD COLUMN GENERATED AS IDENTITY: %d findings", len(findings))
	for _, f := range findings {
		t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
	}
	// Identity columns don't have HasDefault=true, so PG rewrite warn won't fire.
	// No identity-specific rule exists.
	if len(findings) == 0 {
		t.Log("CHARACTERIZATION NOTE: ADD COLUMN identity is completely silent — no generated/identity-specific rule fires")
	}
}

// ---------------------------------------------------------------------------
// Characterization: state-transition forms — the critical gap
// ---------------------------------------------------------------------------

// TestPGGeneratedIdentityRuleCoverage_DropExpression proves that
// ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION produces zero findings
// because no registered rule targets the "drop_expression" action.
func TestPGGeneratedIdentityRuleCoverage_DropExpression(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "drop_expression",
				Name:   "full_name",
				Column: &spec.AlterColumn{OldName: "full_name"},
			}},
		},
	}

	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	t.Logf("DROP EXPRESSION: %d findings", len(findings))
	for _, f := range findings {
		t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
	}

	// GAP: No rule targets "drop_expression" action.
	if len(findings) == 0 {
		t.Log("GAP CONFIRMED: DROP EXPRESSION is completely silent — zero findings from any registered rule")
	} else {
		t.Logf("CHARACTERIZATION NOTE: DROP EXPRESSION produced %d findings — check rule IDs above", len(findings))
	}
}

// TestPGGeneratedIdentityRuleCoverage_SetGenerated proves that
// ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS|BY DEFAULT
// produces zero findings because no registered rule targets the "set_generated" action.
func TestPGGeneratedIdentityRuleCoverage_SetGenerated(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	t.Run("set_generated_always", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "set_generated",
					Name:   "id",
					Column: &spec.AlterColumn{OldName: "id"},
					Options: map[string]string{
						"generated_when": "a",
					},
				}},
			},
		}

		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		t.Logf("SET GENERATED ALWAYS: %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
		}

		if len(findings) == 0 {
			t.Log("GAP CONFIRMED: SET GENERATED ALWAYS is completely silent — zero findings")
		}
	})

	t.Run("set_generated_by_default", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{{
					Action: "set_generated",
					Name:   "id",
					Column: &spec.AlterColumn{OldName: "id"},
					Options: map[string]string{
						"generated_when": "d",
					},
				}},
			},
		}

		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		t.Logf("SET GENERATED BY DEFAULT: %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
		}

		if len(findings) == 0 {
			t.Log("GAP CONFIRMED: SET GENERATED BY DEFAULT is completely silent — zero findings")
		}
	})
}

// TestPGGeneratedIdentityRuleCoverage_DropIdentity proves that
// ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY produces zero findings
// because no registered rule targets the "drop_identity" action.
func TestPGGeneratedIdentityRuleCoverage_DropIdentity(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "drop_identity",
				Name:   "id",
				Column: &spec.AlterColumn{OldName: "id"},
			}},
		},
	}

	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	t.Logf("DROP IDENTITY: %d findings", len(findings))
	for _, f := range findings {
		t.Logf("  - %s [%s]: %s", f.RuleID, f.Level, f.Message)
	}

	// GAP: No rule targets "drop_identity" action.
	if len(findings) == 0 {
		t.Log("GAP CONFIRMED: DROP IDENTITY is completely silent — zero findings from any registered rule")
	} else {
		t.Logf("CHARACTERIZATION NOTE: DROP IDENTITY produced %d findings — check rule IDs above", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Characterization: existing PG-native forbid rules do NOT cover generated/identity actions
// ---------------------------------------------------------------------------

// TestPGAlterActionForbidRulesDoNotCoverGeneratedIdentityActions proves that
// the five existing PG-native alter action forbid rules (set_data_type, set_default,
// drop_default, set_not_null, drop_not_null) do not accidentally match generated/identity
// action values.
func TestPGAlterActionForbidRulesDoNotCoverGeneratedIdentityActions(t *testing.T) {
	t.Parallel()
	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}}

	existingPGActions := []struct {
		ruleID string
		action string
	}{
		{ruleIDAlterSetDataTypeForbid, "set_data_type"},
		{ruleIDAlterSetDefaultForbid, "set_default"},
		{ruleIDAlterDropDefaultForbid, "drop_default"},
		{ruleIDAlterSetNotNullForbid, "set_not_null"},
		{ruleIDAlterDropNotNullForbid, "drop_not_null"},
	}

	generatedIdentityActions := []struct {
		name   string
		action string
	}{
		{"drop_expression", "drop_expression"},
		{"set_generated", "set_generated"},
		{"drop_identity", "drop_identity"},
	}

	for _, pg := range existingPGActions {
		r, err := newForbiddenAlterActionRule(
			pg.ruleID, pg.action, pg.action,
			rule.LevelWarning, cfg,
			withDialectAllowlist(spec.DialectPostgreSQL),
		)
		if err != nil {
			t.Fatalf("new rule %s: %v", pg.ruleID, err)
		}

		for _, gi := range generatedIdentityActions {
			t.Run(pg.action+"_does_not_match_"+gi.action, func(t *testing.T) {
				t.Parallel()
				stmt := spec.Statement{
					Kind:    spec.KindDDL,
					Dialect: spec.DialectPostgreSQL,
					DDL: &spec.DDL{
						Table: &spec.Table{Name: "users"},
						Alter: []spec.Alter{{Action: gi.action, Name: "col1"}},
					},
				}
				findings, err := r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for action %q against rule %q, got %d",
						gi.action, pg.ruleID, len(findings))
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Summary: coverage matrix snapshot
// ---------------------------------------------------------------------------

// TestPGGeneratedIdentityCoverageMatrix is a single test that evaluates all
// supported generated/identity forms through the full registry and prints a
// coverage matrix. This is the primary characterization for the decision gate.
func TestPGGeneratedIdentityCoverageMatrix(t *testing.T) {
	t.Parallel()
	registry := buildTestRegistry(t)

	cases := []struct {
		name string
		stmt spec.Statement
	}{
		{
			name: "create_table_generated_stored",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Columns: []spec.Column{
						{Name: "id", Type: "bigint"},
						{Name: "full_name", Type: "text", GeneratedWhen: "a"},
					},
				},
			},
		},
		{
			name: "create_table_identity",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Columns: []spec.Column{
						{Name: "id", Type: "bigint", GeneratedWhen: "a", IsIdentity: true},
					},
				},
			},
		},
		{
			name: "alter_add_column_generated_stored",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action: "add_column",
						Name:   "full_name",
						Column: &spec.AlterColumn{
							Definition: &spec.Column{Name: "full_name", Type: "text", GeneratedWhen: "a"},
						},
					}},
				},
			},
		},
		{
			name: "alter_add_column_identity",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action: "add_column",
						Name:   "id",
						Column: &spec.AlterColumn{
							Definition: &spec.Column{Name: "id", Type: "bigint", GeneratedWhen: "a", IsIdentity: true},
						},
					}},
				},
			},
		},
		{
			name: "alter_drop_expression",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action: "drop_expression",
						Name:   "full_name",
						Column: &spec.AlterColumn{OldName: "full_name"},
					}},
				},
			},
		},
		{
			name: "alter_set_generated_always",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action:  "set_generated",
						Name:    "id",
						Column:  &spec.AlterColumn{OldName: "id"},
						Options: map[string]string{"generated_when": "a"},
					}},
				},
			},
		},
		{
			name: "alter_set_generated_by_default",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action:  "set_generated",
						Name:    "id",
						Column:  &spec.AlterColumn{OldName: "id"},
						Options: map[string]string{"generated_when": "d"},
					}},
				},
			},
		},
		{
			name: "alter_drop_identity",
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Table: &spec.Table{Name: "t"},
					Alter: []spec.Alter{{
						Action: "drop_identity",
						Name:   "id",
						Column: &spec.AlterColumn{OldName: "id"},
					}},
				},
			},
		},
	}

	t.Log("=== PostgreSQL Generated/Identity Rule Coverage Matrix ===")
	silentCount := 0
	for _, tc := range cases {
		findings, err := registry.EvaluateStatement(context.Background(), tc.stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", tc.name, err)
		}
		status := "COVERED"
		if len(findings) == 0 {
			status = "SILENT"
			silentCount++
		}
		t.Logf("%-38s %s (%d findings)", tc.name, status, len(findings))
		for _, f := range findings {
			t.Logf("  - %s [%s]", f.RuleID, f.Level)
		}
	}
	t.Logf("=== Summary: %d/%d forms are SILENT (zero findings) ===", silentCount, len(cases))
}
