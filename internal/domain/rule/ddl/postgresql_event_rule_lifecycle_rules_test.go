// Package ddl verifies PostgreSQL event trigger and rewrite rule lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL event trigger and rule lifecycle signals and cross-dialect policy controls
// output: focused coverage for the seven PostgreSQL event trigger and rule lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG event trigger and rule lifecycle
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGEventRuleLifecycleRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
		operation spec.DDLOperation
		options   map[string]string
		level     rule.Level
	}{
		{
			name:      "create_event_trigger_notice",
			construct: newCreateEventTriggerNoticeRule,
			operation: spec.DDLOperationCreateEventTrigger,
			options:   map[string]string{"event": "ddl_command_end", "function": "log_ddl"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_event_trigger_notice_enable",
			construct: newAlterEventTriggerNoticeRule,
			operation: spec.DDLOperationAlterEventTrigger,
			options:   map[string]string{"action": "enable"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_event_trigger_notice_rename",
			construct: newAlterEventTriggerNoticeRule,
			operation: spec.DDLOperationAlterEventTrigger,
			options:   map[string]string{"action": "rename", "new_name": "trg_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_event_trigger_disable_warn",
			construct: newAlterEventTriggerDisableWarnRule,
			operation: spec.DDLOperationAlterEventTrigger,
			options:   map[string]string{"action": "disable"},
			level:     rule.LevelWarning,
		},
		{
			name:      "drop_event_trigger_warn",
			construct: newDropEventTriggerWarnRule,
			operation: spec.DDLOperationDropEventTrigger,
			options:   map[string]string{"if_exists": "false"},
			level:     rule.LevelWarning,
		},
		{
			name:      "create_rule_notice",
			construct: newCreateRuleNoticeRule,
			operation: spec.DDLOperationCreateRule,
			options:   map[string]string{"table": "users", "event": "insert"},
			level:     rule.LevelNotice,
		},
		{
			name:      "alter_rule_notice_rename",
			construct: newAlterRuleNoticeRule,
			operation: spec.DDLOperationAlterRule,
			options:   map[string]string{"action": "rename", "new_name": "rule_v2"},
			level:     rule.LevelNotice,
		},
		{
			name:      "drop_rule_warn",
			construct: newDropRuleWarnRule,
			operation: spec.DDLOperationDropRule,
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

			// Positive: correct PG statement fires the rule
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  tc.operation,
					ObjectType: "event_trigger",
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

func TestPGAlterEventTriggerDisableVsEnable(t *testing.T) {
	t.Parallel()

	// Verify disable rule fires only on disable action
	disableRule, err := newAlterEventTriggerDisableWarnRule(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	if err != nil {
		t.Fatalf("construct disable: %v", err)
	}

	enableStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterEventTrigger,
			ObjectType: "event_trigger",
			ObjectName: "trg",
			Options:    map[string]string{"action": "enable"},
		},
	}

	findings, _ := disableRule.Evaluate(context.Background(), enableStmt)
	if len(findings) != 0 {
		t.Fatalf("disable rule should not fire on enable action, got %d findings", len(findings))
	}

	disableStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterEventTrigger,
			ObjectType: "event_trigger",
			ObjectName: "trg",
			Options:    map[string]string{"action": "disable"},
		},
	}

	findings, _ = disableRule.Evaluate(context.Background(), disableStmt)
	if len(findings) != 1 {
		t.Fatalf("disable rule should fire on disable action, got %d findings", len(findings))
	}

	// Verify notice rule fires on enable and rename, not on disable
	noticeRule, err := newAlterEventTriggerNoticeRule(policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	if err != nil {
		t.Fatalf("construct notice: %v", err)
	}

	findings, _ = noticeRule.Evaluate(context.Background(), disableStmt)
	if len(findings) != 0 {
		t.Fatalf("notice rule should not fire on disable action, got %d findings", len(findings))
	}

	findings, _ = noticeRule.Evaluate(context.Background(), enableStmt)
	if len(findings) != 1 {
		t.Fatalf("notice rule should fire on enable action, got %d findings", len(findings))
	}

	renameStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterEventTrigger,
			ObjectType: "event_trigger",
			ObjectName: "trg",
			Options:    map[string]string{"action": "rename", "new_name": "trg_v2"},
		},
	}

	findings, _ = noticeRule.Evaluate(context.Background(), renameStmt)
	if len(findings) != 1 {
		t.Fatalf("notice rule should fire on rename action, got %d findings", len(findings))
	}
}

func TestRegistryIncludesPGEventRuleLifecycleRules(t *testing.T) {
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
		{ruleIDPGCreateEventTriggerNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateEventTrigger, ObjectName: "trg", ObjectType: "event_trigger", Options: map[string]string{"event": "ddl_command_end"}},
		}},
		{ruleIDPGAlterEventTriggerNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterEventTrigger, ObjectName: "trg", ObjectType: "event_trigger", Options: map[string]string{"action": "enable"}},
		}},
		{ruleIDPGAlterEventTriggerDisableWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterEventTrigger, ObjectName: "trg", ObjectType: "event_trigger", Options: map[string]string{"action": "disable"}},
		}},
		{ruleIDPGDropEventTriggerWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropEventTrigger, ObjectName: "trg", ObjectType: "event_trigger", Options: map[string]string{"if_exists": "false"}},
		}},
		{ruleIDPGCreateRuleNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateRule, ObjectName: "r", ObjectType: "rule", Options: map[string]string{"table": "users", "event": "insert"}},
		}},
		{ruleIDPGAlterRuleNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterRule, ObjectName: "r", ObjectType: "rule", Options: map[string]string{"action": "rename", "new_name": "r2"}},
		}},
		{ruleIDPGDropRuleWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropRule, ObjectName: "r", ObjectType: "rule", Options: map[string]string{"if_exists": "false"}},
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

func mustNewDropEventTriggerWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropEventTriggerWarnRule(cfg)
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	return r
}
