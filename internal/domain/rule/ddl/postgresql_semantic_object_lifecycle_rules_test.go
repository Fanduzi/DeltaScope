package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGSemanticObjectLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		options   map[string]string
		level     rule.Level
	}{
		{
			name:      "create_aggregate_notice",
			construct: newCreateAggregateNoticeRule,
			operation: spec.DDLOperationCreateAggregate,
			options:   map[string]string{},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_aggregate_notice_rename",
			construct: newAlterAggregateNoticeRule,
			operation: spec.DDLOperationAlterAggregate,
			options:   map[string]string{"action": "rename", "new_name": "sum2_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_aggregate_notice_set_schema",
			construct: newAlterAggregateNoticeRule,
			operation: spec.DDLOperationAlterAggregate,
			options:   map[string]string{"action": "set_schema", "new_schema": "app"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_aggregate_notice_owner",
			construct: newAlterAggregateNoticeRule,
			operation: spec.DDLOperationAlterAggregate,
			options:   map[string]string{"action": "set_owner", "owner": "app_owner"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_aggregate_warn",
			construct: newDropAggregateWarnRule,
			operation: spec.DDLOperationDropAggregate,
			options:   map[string]string{"if_exists": "false"},
			level:     rule.LevelWarning,
		},
		{
			name:      "create_operator_notice",
			construct: newCreateOperatorNoticeRule,
			operation: spec.DDLOperationCreateOperator,
			options:   map[string]string{},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_operator_notice_set_schema",
			construct: newAlterOperatorNoticeRule,
			operation: spec.DDLOperationAlterOperator,
			options:   map[string]string{"action": "set_schema", "new_schema": "app"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_operator_notice_owner",
			construct: newAlterOperatorNoticeRule,
			operation: spec.DDLOperationAlterOperator,
			options:   map[string]string{"action": "set_owner", "owner": "app_owner"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_operator_warn",
			construct: newDropOperatorWarnRule,
			operation: spec.DDLOperationDropOperator,
			options:   map[string]string{"if_exists": "false"},
			level:     rule.LevelWarning,
		},
		{
			name:      "create_conversion_notice",
			construct: newCreateConversionNoticeRule,
			operation: spec.DDLOperationCreateConversion,
			options:   map[string]string{},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_conversion_notice_rename",
			construct: newAlterConversionNoticeRule,
			operation: spec.DDLOperationAlterConversion,
			options:   map[string]string{"action": "rename", "new_name": "conv_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_conversion_notice_set_schema",
			construct: newAlterConversionNoticeRule,
			operation: spec.DDLOperationAlterConversion,
			options:   map[string]string{"action": "set_schema", "new_schema": "app"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_conversion_notice_owner",
			construct: newAlterConversionNoticeRule,
			operation: spec.DDLOperationAlterConversion,
			options:   map[string]string{"action": "set_owner", "owner": "app_owner"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_conversion_warn",
			construct: newDropConversionWarnRule,
			operation: spec.DDLOperationDropConversion,
			options:   map[string]string{"if_exists": "false"},
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

			objectType := "aggregate"
			if strings.Contains(tc.name, "operator") {
				objectType = "operator"
			} else if strings.Contains(tc.name, "conversion") {
				objectType = "conversion"
			}

			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: objectType,
					ObjectName: "test_obj",
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
			if findings[0].Metadata["object_name"] != "test_obj" {
				t.Fatalf("expected object_name metadata, got %v", findings[0].Metadata["object_name"])
			}

			// Dialect isolation: MySQL must be skipped
			mysqlStmt := stmt
			mysqlStmt.Dialect = spec.DialectMySQL
			findings, err = r.Evaluate(context.Background(), mysqlStmt)
			if err != nil {
				t.Fatalf("evaluate mysql: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected rule to skip MySQL, got %d findings", len(findings))
			}

			// Wrong operation must be skipped
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

func TestRegistryIncludesPGSemanticObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	ruleCases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateAggregateNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateAggregate, ObjectName: "sum2", ObjectType: "aggregate", Options: map[string]string{}},
		}},
		{ruleIDPGAlterAggregateNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterAggregate, ObjectName: "sum2", ObjectType: "aggregate", Options: map[string]string{"action": "rename", "new_name": "sum2_v2"}},
		}},
		{ruleIDPGDropAggregateWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropAggregate, ObjectName: "sum2", ObjectType: "aggregate", Options: map[string]string{"if_exists": "false"}},
		}},
		{ruleIDPGCreateOperatorNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateOperator, ObjectName: "===", ObjectType: "operator", Options: map[string]string{}},
		}},
		{ruleIDPGAlterOperatorNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterOperator, ObjectName: "===", ObjectType: "operator", Options: map[string]string{"action": "set_owner", "owner": "app_owner"}},
		}},
		{ruleIDPGDropOperatorWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropOperator, ObjectName: "===", ObjectType: "operator", Options: map[string]string{"if_exists": "false"}},
		}},
		{ruleIDPGCreateConversionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateConversion, ObjectName: "conv", ObjectType: "conversion", Options: map[string]string{}},
		}},
		{ruleIDPGAlterConversionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterConversion, ObjectName: "conv", ObjectType: "conversion", Options: map[string]string{"action": "rename", "new_name": "conv_v2"}},
		}},
		{ruleIDPGDropConversionWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropConversion, ObjectName: "conv", ObjectType: "conversion", Options: map[string]string{"if_exists": "false"}},
		}},
	}

	for _, tc := range ruleCases {
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
