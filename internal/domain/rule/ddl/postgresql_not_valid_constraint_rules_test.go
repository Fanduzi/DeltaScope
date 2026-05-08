package ddl

import (
	"context"
	"strconv"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestNotValidConstraintValidateRequiredRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		stmts    []spec.Statement
		wantFind int
	}{
		{
			name: "check_without_validate_fires",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", true),
			},
			wantFind: 1,
		},
		{
			name: "foreign_key_without_validate_fires",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "fk_orders_user", "foreign_key", true),
			},
			wantFind: 1,
		},
		{
			name: "later_validate_suppresses",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", true),
				pgValidateConstraint("orders", "", "chk_orders_amount"),
			},
			wantFind: 0,
		},
		{
			name: "earlier_validate_does_not_suppress",
			stmts: []spec.Statement{
				pgValidateConstraint("orders", "", "chk_orders_amount"),
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", true),
			},
			wantFind: 1,
		},
		{
			name: "different_table_does_not_suppress",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", true),
				pgValidateConstraint("archived_orders", "", "chk_orders_amount"),
			},
			wantFind: 1,
		},
		{
			name: "different_schema_does_not_suppress",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "public", "chk_orders_amount", "check", true),
				pgValidateConstraint("orders", "archive", "chk_orders_amount"),
			},
			wantFind: 1,
		},
		{
			name: "unnamed_constraint_skipped",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "", "check", true),
			},
			wantFind: 0,
		},
		{
			name: "not_not_valid_skipped",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", false),
			},
			wantFind: 0,
		},
		{
			name: "mysql_skipped",
			stmts: []spec.Statement{
				mysqlAlterConstraint("orders", "chk_orders_amount", "check", true),
			},
			wantFind: 0,
		},
		{
			name: "tidb_skipped",
			stmts: []spec.Statement{
				tidbAlterConstraint("orders", "chk_orders_amount", "check", true),
			},
			wantFind: 0,
		},
		{
			name: "required_false_disables_rule",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_orders_amount", "check", true),
			},
			wantFind: -1,
		},
		{
			name: "two_pending_only_validated_one_fires",
			stmts: []spec.Statement{
				pgAlterConstraint("orders", "", "chk_a", "check", true),
				pgAlterConstraint("orders", "", "chk_b", "check", true),
				pgValidateConstraint("orders", "", "chk_a"),
			},
			wantFind: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			var r rule.GlobalRule
			var err error
			cfg := policy.RulePolicy{
				Enabled: true,
				Level:   "warning",
				Params:  map[string]any{"required": true},
			}
			if tt.wantFind == -1 {
				cfg.Params = map[string]any{"required": false}
			}
			r, err = newNotValidConstraintValidateRequiredRule(cfg)
			if err != nil {
				t.Fatalf("construct rule: %v", err)
			}
			findings, err := r.EvaluateAll(context.Background(), tt.stmts)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if tt.wantFind == -1 {
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings with required=false, got %d", len(findings))
				}
				return
			}
			if len(findings) != tt.wantFind {
				t.Fatalf("expected %d findings, got %d", tt.wantFind, len(findings))
			}
		})
	}
}

func TestNotValidConstraintValidateRequiredRule_FindingMetadata(t *testing.T) {
	t.Parallel()
	r, err := newNotValidConstraintValidateRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   "warning",
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	stmts := []spec.Statement{
		pgAlterConstraint("orders", "public", "chk_amount", "check", true),
	}
	findings, err := r.EvaluateAll(context.Background(), stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]

	// RuleID is set by the registry during normalization, not by the rule itself.
	// Verify it through the registry-level test instead.
	if f.RuleID != "" && f.RuleID != ruleIDPGAlterNotValidConstraintValidateRequire {
		t.Fatalf("unexpected mismatched rule ID %q", f.RuleID)
	}
	if f.Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", f.Level)
	}
	if f.Explanation == nil {
		t.Fatal("expected non-nil explanation")
	}
	if f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected explanation with why/risk/suggestion, got %+v", f.Explanation)
	}

	checks := map[string]any{
		"dialect":           "postgresql",
		"action":            "add_constraint",
		"required_followup": "validate_constraint",
		"schema":            "public",
		"table":             "orders",
		"constraint":        "chk_amount",
		"constraint_type":   "check",
		"not_valid":         true,
		"statement_index":   0,
	}
	for key, expected := range checks {
		got, ok := f.Metadata[key]
		if !ok {
			t.Fatalf("missing metadata key %q", key)
		}
		if got != expected {
			t.Fatalf("metadata[%q]: expected %v, got %v", key, expected, got)
		}
	}
}

func TestNotValidConstraintValidateRequiredRule_SchemaQualifiedTable(t *testing.T) {
	t.Parallel()
	r, err := newNotValidConstraintValidateRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   "warning",
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	stmts := []spec.Statement{
		pgAlterConstraint("orders", "public", "chk_amount", "check", true),
		pgValidateConstraint("orders", "public", "chk_amount"),
	}
	findings, err := r.EvaluateAll(context.Background(), stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when schema-qualified validate matches, got %d", len(findings))
	}
}

func TestNotValidConstraintValidateRequiredRule_RegisteredInRegistry(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	statements := []spec.Statement{{
		Dialect: spec.DialectPostgreSQL,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "chk_orders_amount",
				Options: map[string]string{
					"constraint_type": "check",
					"not_valid":       "true",
				},
			}},
		},
	}}

	globalFindings, err := registry.EvaluateGlobal(context.Background(), statements)
	if err != nil {
		t.Fatalf("evaluate global: %v", err)
	}
	found := false
	for _, f := range globalFindings {
		if f.RuleID == ruleIDPGAlterNotValidConstraintValidateRequire {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected global finding with rule ID %q, got %d findings", ruleIDPGAlterNotValidConstraintValidateRequire, len(globalFindings))
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func pgAlterConstraint(table, schema, name, constraintType string, notValid bool) spec.Statement {
	return alterConstraintStatement(spec.DialectPostgreSQL, table, schema, name, constraintType, notValid)
}

func mysqlAlterConstraint(table, name, constraintType string, notValid bool) spec.Statement {
	return alterConstraintStatement(spec.DialectMySQL, table, "", name, constraintType, notValid)
}

func tidbAlterConstraint(table, name, constraintType string, notValid bool) spec.Statement {
	return alterConstraintStatement(spec.DialectTiDB, table, "", name, constraintType, notValid)
}

func alterConstraintStatement(dialect spec.Dialect, table, schema, name, constraintType string, notValid bool) spec.Statement {
	return spec.Statement{
		Dialect: dialect,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: table, Schema: schema},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   name,
				Options: map[string]string{
					"constraint_type": constraintType,
					"not_valid":       strconv.FormatBool(notValid),
				},
			}},
		},
	}
}

func pgValidateConstraint(table, schema, name string) spec.Statement {
	return spec.Statement{
		Dialect: spec.DialectPostgreSQL,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: table, Schema: schema},
			Alter: []spec.Alter{{
				Action: "validate_constraint",
				Name:   name,
			}},
		},
	}
}
