// Package ddl verifies PostgreSQL annotation lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL annotation lifecycle signals and cross-dialect policy controls
// output: focused coverage for the four PostgreSQL annotation lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG annotation lifecycle
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAnnotationLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		options   map[string]string
		level     rule.Level
	}{
		{
			name:      "comment_on_notice",
			construct: newCommentOnNoticeRule,
			operation: spec.DDLOperationCommentOn,
			options:   map[string]string{"is_null": "false"},
			level:     rule.LevelNotice,
		},
		{
			name:      "comment_on_remove_notice",
			construct: newCommentOnRemoveNoticeRule,
			operation: spec.DDLOperationCommentOn,
			options:   map[string]string{"is_null": "true"},
			level:     rule.LevelNotice,
		},
		{
			name:      "security_label_notice",
			construct: newSecurityLabelNoticeRule,
			operation: spec.DDLOperationSecurityLabel,
			options:   map[string]string{"is_null": "false"},
			level:     rule.LevelNotice,
		},
		{
			name:      "security_label_remove_notice",
			construct: newSecurityLabelRemoveNoticeRule,
			operation: spec.DDLOperationSecurityLabel,
			options:   map[string]string{"is_null": "true"},
			level:     rule.LevelNotice,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := tc.construct(policy.RulePolicy{Enabled: true, Level: tc.level})
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			// --- Positive: correct PG statement fires the rule ---
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "comment",
					ObjectName: "users",
					Options:    tc.options,
				},
			}

			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].Level != tc.level {
				t.Fatalf("expected level %q, got %q", tc.level, findings[0].Level)
			}
			if findings[0].Explanation == nil || findings[0].Explanation.Why == "" || findings[0].Explanation.Risk == "" || findings[0].Explanation.Suggestion == "" {
				t.Fatalf("expected complete explanation, got %+v", findings[0].Explanation)
			}
			if findings[0].Metadata["object_name"] != "users" {
				t.Fatalf("expected metadata object_name=users, got %v", findings[0].Metadata["object_name"])
			}

			// --- Wrong dialect: MySQL statements are skipped ---
			mysqlStmt := stmt
			mysqlStmt.Dialect = spec.DialectMySQL
			findings, err = r.Evaluate(context.Background(), mysqlStmt)
			if err != nil {
				t.Fatalf("evaluate mysql: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip MySQL, got %d findings", len(findings))
			}

			// --- Wrong operation: different DDL operation is skipped ---
			wrongStmt := stmt
			wrongStmt.DDL = &spec.DDL{
				Operation:  spec.DDLOperationCreateTable,
				ObjectType: "table",
				ObjectName: "other_obj",
			}
			findings, err = r.Evaluate(context.Background(), wrongStmt)
			if err != nil {
				t.Fatalf("evaluate wrong op: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip wrong operation, got %d findings", len(findings))
			}

			// --- Wrong is_null: comment_on.notice skips is_null=true ---
			if tc.options["is_null"] == "false" {
				nullStmt := stmt
				nullStmt.DDL = &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "comment",
					ObjectName: "users",
					Options:    map[string]string{"is_null": "true"},
				}
				findings, err = r.Evaluate(context.Background(), nullStmt)
				if err != nil {
					t.Fatalf("evaluate null stmt: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected rule to skip is_null=true, got %d findings", len(findings))
				}
			}
			// --- Wrong is_null: remove rules skip is_null=false ---
			if tc.options["is_null"] == "true" {
				nonNullStmt := stmt
				nonNullStmt.DDL = &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "comment",
					ObjectName: "users",
					Options:    map[string]string{"is_null": "false"},
				}
				findings, err = r.Evaluate(context.Background(), nonNullStmt)
				if err != nil {
					t.Fatalf("evaluate non-null stmt: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected rule to skip is_null=false, got %d findings", len(findings))
				}
			}
		})
	}
}

func TestRegistryIncludesPGAnnotationLifecycleRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCommentOnNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCommentOn, ObjectName: "users", ObjectType: "comment", Options: map[string]string{"is_null": "false"}},
		}},
		{ruleIDPGCommentOnRemoveNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCommentOn, ObjectName: "users", ObjectType: "comment", Options: map[string]string{"is_null": "true"}},
		}},
		{ruleIDPGSecurityLabelNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationSecurityLabel, ObjectName: "users", ObjectType: "security_label", Options: map[string]string{"is_null": "false", "provider": "selinux"}},
		}},
		{ruleIDPGSecurityLabelRemoveNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationSecurityLabel, ObjectName: "users", ObjectType: "security_label", Options: map[string]string{"is_null": "true", "provider": "selinux"}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			t.Parallel()
			findings, err := registry.EvaluateStatement(context.Background(), tc.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			found := false
			for _, f := range findings {
				if f.RuleID == tc.ruleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("rule %q did not fire", tc.ruleID)
			}
		})
	}
}
