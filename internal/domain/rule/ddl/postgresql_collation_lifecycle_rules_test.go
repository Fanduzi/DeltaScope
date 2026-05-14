package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCollationLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		options   map[string]string
		level     rule.Level
	}{
		{
			name:      "create_collation_notice",
			construct: newCreateCollationNoticeRule,
			operation: spec.DDLOperationCreateCollation,
			options:   map[string]string{},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_collation_notice_rename",
			construct: newAlterCollationNoticeRule,
			operation: spec.DDLOperationAlterCollation,
			options:   map[string]string{"action": "rename", "new_name": "col_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_collation_notice_set_schema",
			construct: newAlterCollationNoticeRule,
			operation: spec.DDLOperationAlterCollation,
			options:   map[string]string{"action": "set_schema", "new_schema": "public"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_collation_notice_owner",
			construct: newAlterCollationNoticeRule,
			operation: spec.DDLOperationAlterCollation,
			options:   map[string]string{"action": "owner"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_collation_warn",
			construct: newDropCollationWarnRule,
			operation: spec.DDLOperationDropCollation,
			options:   map[string]string{"if_exists": "false"},
			level:     rule.LevelWarning,
		},
		{
			name:      "drop_collation_warn_cascade",
			construct: newDropCollationWarnRule,
			operation: spec.DDLOperationDropCollation,
			options:   map[string]string{"if_exists": "true", "cascade": "true"},
			level:     rule.LevelWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := tc.construct(policy.RulePolicy{Enabled: true, Level: tc.level})
			if err != nil {
				t.Fatalf("construct: %v", err)
			}

			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "collation",
					ObjectName: "test_collation",
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
			if findings[0].Explanation == nil || findings[0].Explanation.Why == "" {
				t.Fatalf("expected complete explanation, got %+v", findings[0].Explanation)
			}
			if findings[0].Metadata["object_name"] != "test_collation" {
				t.Fatalf("expected object_name metadata, got %v", findings[0].Metadata["object_name"])
			}

			// Wrong dialect: MySQL statements are skipped
			mysqlStmt := stmt
			mysqlStmt.Dialect = spec.DialectMySQL
			findings, err = r.Evaluate(context.Background(), mysqlStmt)
			if err != nil {
				t.Fatalf("evaluate mysql: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip MySQL, got %d findings", len(findings))
			}

			// Wrong operation: different DDL operation is skipped
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
		})
	}
}

func TestRegistryIncludesPGCollationLifecycleRules(t *testing.T) {
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
		{ruleIDPGCreateCollationNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateCollation, ObjectName: "col1", ObjectType: "collation", Options: map[string]string{}},
		}},
		{ruleIDPGAlterCollationNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterCollation, ObjectName: "col1", ObjectType: "collation", Options: map[string]string{"action": "rename", "new_name": "col2"}},
		}},
		{ruleIDPGDropCollationWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropCollation, ObjectName: "col1", ObjectType: "collation", Options: map[string]string{"if_exists": "false"}},
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
