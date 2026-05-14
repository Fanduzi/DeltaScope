package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGStatisticsLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		options   map[string]string
		level     rule.Level
	}{
		{
			name:      "create_statistics_notice",
			construct: newCreateStatisticsNoticeRule,
			operation: spec.DDLOperationCreateStatistics,
			options:   map[string]string{"target_table": "users"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_statistics_notice_rename",
			construct: newAlterStatisticsNoticeRule,
			operation: spec.DDLOperationAlterStatistics,
			options:   map[string]string{"action": "rename", "new_name": "stats_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_statistics_notice_set_schema",
			construct: newAlterStatisticsNoticeRule,
			operation: spec.DDLOperationAlterStatistics,
			options:   map[string]string{"action": "set_schema", "new_schema": "public"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_statistics_notice_owner",
			construct: newAlterStatisticsNoticeRule,
			operation: spec.DDLOperationAlterStatistics,
			options:   map[string]string{"action": "set_owner", "owner": "app_owner"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_statistics_warn",
			construct: newDropStatisticsWarnRule,
			operation: spec.DDLOperationDropStatistics,
			options:   map[string]string{"if_exists": "false"},
			level:     rule.LevelWarning,
		},
		{
			name:      "drop_statistics_warn_cascade",
			construct: newDropStatisticsWarnRule,
			operation: spec.DDLOperationDropStatistics,
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
					ObjectType: "statistics",
					ObjectName: "test_stats",
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
			if findings[0].Metadata["object_name"] != "test_stats" {
				t.Fatalf("expected object_name metadata, got %v", findings[0].Metadata["object_name"])
			}

			mysqlStmt := stmt
			mysqlStmt.Dialect = spec.DialectMySQL
			findings, err = r.Evaluate(context.Background(), mysqlStmt)
			if err != nil {
				t.Fatalf("evaluate mysql: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip MySQL, got %d findings", len(findings))
			}

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

func TestRegistryIncludesPGStatisticsLifecycleRules(t *testing.T) {
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
		{ruleIDPGCreateStatisticsNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateStatistics, ObjectName: "st1", ObjectType: "statistics", Options: map[string]string{}},
		}},
		{ruleIDPGAlterStatisticsNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterStatistics, ObjectName: "st1", ObjectType: "statistics", Options: map[string]string{"action": "rename", "new_name": "st2"}},
		}},
		{ruleIDPGDropStatisticsWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropStatistics, ObjectName: "st1", ObjectType: "statistics", Options: map[string]string{"if_exists": "false"}},
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
