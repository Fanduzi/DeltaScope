package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAlterRelationshipRules(t *testing.T) {
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
			name:         "add_inherit",
			ruleID:       ruleIDPGAlterAddInheritNotice,
			constructor:  newAddInheritNoticeRule,
			action:       "add_inherit",
			alterOptions: map[string]string{"parent_table": "users", "relationship": "inheritance"},
			wantFindings: 1,
		},
		{
			name:         "drop_inherit",
			ruleID:       ruleIDPGAlterDropInheritNotice,
			constructor:  newDropInheritNoticeRule,
			action:       "drop_inherit",
			alterOptions: map[string]string{"parent_table": "users", "relationship": "inheritance"},
			wantFindings: 1,
		},
		{
			name:         "add_of_type",
			ruleID:       ruleIDPGAlterAddOfTypeNotice,
			constructor:  newAddOfTypeNoticeRule,
			action:       "add_of_type",
			alterOptions: map[string]string{"type": "user_type", "relationship": "typed_table"},
			wantFindings: 1,
		},
		{
			name:         "drop_of_type",
			ruleID:       ruleIDPGAlterDropOfTypeNotice,
			constructor:  newDropOfTypeNoticeRule,
			action:       "drop_of_type",
			alterOptions: map[string]string{"relationship": "typed_table"},
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
				spec.Alter{Action: tc.action, Options: tc.alterOptions},
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
				if meta["relationship"] == nil {
					t.Fatal("metadata missing relationship key")
				}
			}

			// MySQL -> zero findings
			mysqlStmt := alterStatementWithDialect(spec.DialectMySQL,
				spec.Alter{Action: tc.action, Options: tc.alterOptions},
			)
			mysqlFindings, _ := r.Evaluate(context.Background(), mysqlStmt)
			if len(mysqlFindings) != 0 {
				t.Fatalf("MySQL: expected 0 findings, got %d", len(mysqlFindings))
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

func TestPGAlterRelationshipNoLeak(t *testing.T) {
	t.Parallel()
	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}

	forbiddenKeys := []string{
		"raw_sql", "column_definition", "type_attributes",
		"catalog_state", "validation_result", "dependency_graph",
	}

	r, err := newAddInheritNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action:  "add_inherit",
			Options: map[string]string{"parent_table": "users", "relationship": "inheritance"},
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
