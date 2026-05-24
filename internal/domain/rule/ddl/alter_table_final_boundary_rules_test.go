package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestFinalBoundarySetExpressionNoticeRule(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}
	r, err := newSetExpressionNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if r.ID() != ruleIDPGAlterSetExpressionNotice {
		t.Fatalf("expected ID %q, got %q", ruleIDPGAlterSetExpressionNotice, r.ID())
	}

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "set_expression",
			Name:   "full_name",
			Options: map[string]string{
				"column":         "full_name",
				"has_expression": "true",
			},
		},
	)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	m := findings[0].Metadata
	assertMetadata(t, m, "operation", "alter_table")
	assertMetadata(t, m, "action", "set_expression")
	assertMetadata(t, m, "table", "users")
	assertMetadata(t, m, "column", "full_name")
	assertMetadata(t, m, "has_expression", "true")
	assertNoLeak(t, m)
}

func TestFinalBoundaryAddIdentityNoticeRule(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}
	r, err := newAddIdentityNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if r.ID() != ruleIDPGAlterAddIdentityNotice {
		t.Fatalf("expected ID %q, got %q", ruleIDPGAlterAddIdentityNotice, r.ID())
	}

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_identity",
			Name:   "id",
			Options: map[string]string{
				"column":         "id",
				"identity":       "true",
				"generated_when": "by_default",
			},
		},
	)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	m := findings[0].Metadata
	assertMetadata(t, m, "operation", "alter_table")
	assertMetadata(t, m, "action", "add_identity")
	assertMetadata(t, m, "table", "users")
	assertMetadata(t, m, "column", "id")
	assertMetadata(t, m, "identity", "true")
	assertMetadata(t, m, "generated_when", "by_default")
	assertNoLeak(t, m)
}

func TestFinalBoundaryAddExclusionConstraintNoticeRule(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}
	r, err := newAddExclusionConstraintNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if r.ID() != ruleIDPGAlterAddExclusionConstraintNotice {
		t.Fatalf("expected ID %q, got %q", ruleIDPGAlterAddExclusionConstraintNotice, r.ID())
	}

	stmt := alterStatementWithDialect(spec.DialectPostgreSQL,
		spec.Alter{
			Action: "add_exclusion_constraint",
			Name:   "bookings_no_overlap",
			Options: map[string]string{
				"constraint":      "bookings_no_overlap",
				"constraint_type": "exclusion",
				"access_method":   "gist",
			},
		},
	)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	m := findings[0].Metadata
	assertMetadata(t, m, "operation", "alter_table")
	assertMetadata(t, m, "action", "add_exclusion_constraint")
	assertMetadata(t, m, "table", "users")
	assertMetadata(t, m, "constraint", "bookings_no_overlap")
	assertMetadata(t, m, "constraint_type", "exclusion")
	assertMetadata(t, m, "access_method", "gist")
	assertNoLeak(t, m)
}

func TestFinalBoundaryMoveAllTablespaceNoticeRule(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}
	r, err := newMoveAllTablespaceNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if r.ID() != ruleIDPGAlterMoveAllTablespaceNotice {
		t.Fatalf("expected ID %q, got %q", ruleIDPGAlterMoveAllTablespaceNotice, r.ID())
	}

	// move_all_tablespace has no table identity
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter: []spec.Alter{{
				Action: "move_all_tablespace",
				Options: map[string]string{
					"object_type":        "table",
					"source_tablespace":  "pg_default",
					"target_tablespace":  "fastspace",
					"has_table_identity": "false",
				},
			}},
		},
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	m := findings[0].Metadata
	assertMetadata(t, m, "operation", "alter_table")
	assertMetadata(t, m, "action", "move_all_tablespace")
	assertMetadata(t, m, "object_type", "table")
	assertMetadata(t, m, "source_tablespace", "pg_default")
	assertMetadata(t, m, "target_tablespace", "fastspace")
	assertMetadata(t, m, "has_table_identity", "false")
	if _, hasTable := m["table"]; hasTable {
		t.Errorf("move_all_tablespace metadata must not contain 'table' key, got: %v", m)
	}
	assertNoLeak(t, m)
}

func TestFinalBoundaryRulesDoNotApplyToMySQL(t *testing.T) {
	t.Parallel()

	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}}
	rules := []struct {
		name        string
		constructor func(policy.RulePolicy) (rule.StatementRule, error)
		action      string
	}{
		{"set_expression", newSetExpressionNoticeRule, "set_expression"},
		{"add_identity", newAddIdentityNoticeRule, "add_identity"},
		{"add_exclusion_constraint", newAddExclusionConstraintNoticeRule, "add_exclusion_constraint"},
		{"move_all_tablespace", newMoveAllTablespaceNoticeRule, "move_all_tablespace"},
	}

	for _, tc := range rules {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := tc.constructor(cfg)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			mysqlStmt := alterStatementWithDialect(spec.DialectMySQL,
				spec.Alter{Action: tc.action, Options: map[string]string{}},
			)
			if r.AppliesTo(mysqlStmt) {
				t.Errorf("rule %s must not apply to MySQL dialect", tc.name)
			}
		})
	}
}

func assertMetadata(t *testing.T, m map[string]any, key, value string) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Errorf("metadata missing key %q, have: %v", key, m)
		return
	}
	if got != value {
		t.Errorf("metadata[%q] = %v, want %q", key, got, value)
	}
}

func assertNoLeak(t *testing.T, m map[string]any) {
	t.Helper()
	forbidden := []string{
		"raw_sql", "expression", "first_name", "last_name", "||",
		"sequence_options", "start", "increment", "cache", "cycle",
		"exclusions", "operator", "operator_class", "room_id", "during", "&&",
		"predicate", "where_clause",
		"catalog_state", "validation_result", "dependency_graph", "rewrite_duration",
	}
	for _, f := range forbidden {
		for k, v := range m {
			ks := strings.ToLower(k)
			vs := strings.ToLower(v.(string))
			// has_expression is a bounded boolean flag; don't flag it as expression leak
			if f == "expression" && (ks == "has_expression" || ks == "action") {
				continue
			}
			if strings.Contains(ks, f) {
				t.Errorf("forbidden substring %q in metadata key %q", f, k)
			}
			if strings.Contains(vs, f) {
				t.Errorf("forbidden substring %q in metadata[%q]=%q", f, k, v)
			}
		}
	}
}
