package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAlterConstraintDeferrabilityRules(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
		Params:  map[string]any{},
	}

	cases := []struct {
		name         string
		ruleID       string
		constructor  func(policy.RulePolicy) (rule.StatementRule, error)
		action       string
		alterOptions map[string]string
		wantFindings int
	}{
		{
			name:         "alter_constraint_deferrable",
			ruleID:       ruleIDPGAlterConstraintDeferrableNotice,
			constructor:  newAlterConstraintDeferrableNoticeRule,
			action:       "alter_constraint_deferrable",
			alterOptions: map[string]string{"constraint_type": "foreign_key", "deferrable": "true", "initially_deferred": "false"},
			wantFindings: 1,
		},
		{
			name:         "alter_constraint_initially_deferred",
			ruleID:       ruleIDPGAlterConstraintInitiallyDeferredNotice,
			constructor:  newAlterConstraintInitiallyDeferredNoticeRule,
			action:       "alter_constraint_initially_deferred",
			alterOptions: map[string]string{"constraint_type": "foreign_key", "deferrable": "true", "initially_deferred": "true"},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := tc.constructor(cfg)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			if r.ID() != tc.ruleID {
				t.Fatalf("expected ID %q, got %q", tc.ruleID, r.ID())
			}

			// PG + matching action -> finding
			stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
				spec.Alter{Action: tc.action, Name: "orders_user_id_fkey", Options: tc.alterOptions},
			)
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != tc.wantFindings {
				t.Fatalf("expected %d finding, got %d", tc.wantFindings, len(findings))
			}
			if len(findings) > 0 {
				f := findings[0]
				meta := f.Metadata
				if meta["operation"] != "alter_table" {
					t.Fatalf("metadata operation = %v, want alter_table", meta["operation"])
				}
				if meta["action"] != tc.action {
					t.Fatalf("metadata action = %v, want %s", meta["action"], tc.action)
				}
				if meta["table"] != "users" {
					t.Fatalf("metadata table = %v, want users", meta["table"])
				}
				if meta["constraint"] != "orders_user_id_fkey" {
					t.Fatalf("metadata constraint = %v, want orders_user_id_fkey", meta["constraint"])
				}
				if meta["constraint_type"] != "foreign_key" {
					t.Fatalf("metadata constraint_type = %v, want foreign_key", meta["constraint_type"])
				}
				if meta["deferrable"] == nil {
					t.Fatal("metadata missing deferrable key")
				}
				if meta["initially_deferred"] == nil {
					t.Fatal("metadata missing initially_deferred key")
				}
			}

			// MySQL -> zero findings
			mysqlStmt := alterStatementWithDialect(spec.DialectMySQL,
				spec.Alter{Action: tc.action, Name: "orders_user_id_fkey", Options: tc.alterOptions},
			)
			mysqlFindings, _ := r.Evaluate(context.Background(), mysqlStmt)
			if len(mysqlFindings) != 0 {
				t.Fatalf("MySQL: expected 0 findings, got %d", len(mysqlFindings))
			}

			// TiDB -> zero findings
			tidbStmt := alterStatementWithDialect(spec.DialectTiDB,
				spec.Alter{Action: tc.action, Name: "orders_user_id_fkey", Options: tc.alterOptions},
			)
			tidbFindings, _ := r.Evaluate(context.Background(), tidbStmt)
			if len(tidbFindings) != 0 {
				t.Fatalf("TiDB: expected 0 findings, got %d", len(tidbFindings))
			}

			// Wrong action -> zero findings
			wrongStmt := alterStatementWithDialect(spec.DialectPostgreSQL,
				spec.Alter{Action: "add_column", Options: map[string]string{}},
			)
			wrongFindings, _ := r.Evaluate(context.Background(), wrongStmt)
			if len(wrongFindings) != 0 {
				t.Fatalf("wrong action: expected 0 findings, got %d", len(wrongFindings))
			}
		})
	}
}

func TestPGAlterConstraintDeferrabilityNoLeak(t *testing.T) {
	t.Parallel()
	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}

	forbiddenKeys := []string{
		"raw_sql", "expression", "predicate",
		"operator_class", "exclusions", "sequence_options",
		"catalog_state", "validation_result", "dependency_graph",
	}

	r, err := newAlterConstraintDeferrableNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action:  "alter_constraint_deferrable",
			Name:    "orders_user_id_fkey",
			Options: map[string]string{"constraint_type": "foreign_key", "deferrable": "true", "initially_deferred": "false"},
		},
	)
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	meta := findings[0].Metadata
	for _, key := range forbiddenKeys {
		if _, ok := meta[key]; ok {
			t.Fatalf("forbidden key %q present in finding metadata", key)
		}
	}
}
