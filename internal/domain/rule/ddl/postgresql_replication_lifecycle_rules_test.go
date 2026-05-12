//go:build postgresql

// Package ddl verifies PostgreSQL publication/subscription lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL replication lifecycle signals and cross-dialect policy controls
// output: focused coverage for the seven PostgreSQL publication/subscription lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG replication lifecycle
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
// Positive tests: rules fire for matching PG statements
// ---------------------------------------------------------------------------

func TestCreatePublicationNoticeRule(t *testing.T) {
	t.Parallel()
	r := mustNewCreatePublicationNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreatePublication,
			ObjectName: "my_pub",
			ObjectType: "publication",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", f.Level)
	}
	if f.Explanation == nil || f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected complete explanation, got %+v", f.Explanation)
	}
	if f.Metadata["object_name"] != "my_pub" {
		t.Fatalf("expected metadata object_name=my_pub, got %v", f.Metadata["object_name"])
	}
}

func TestAlterPublicationNoticeRule(t *testing.T) {
	t.Parallel()
	r := mustNewAlterPublicationNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterPublication,
			ObjectName: "my_pub",
			ObjectType: "publication",
			Options:    map[string]string{"action": "add_table"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", f.Level)
	}
	if f.Explanation == nil || f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected complete explanation, got %+v", f.Explanation)
	}
}

func TestDropPublicationWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropPublicationWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropPublication,
			ObjectName: "my_pub",
			ObjectType: "publication",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
}

func TestCreateSubscriptionNoticeRule(t *testing.T) {
	t.Parallel()
	r := mustNewCreateSubscriptionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateSubscription,
			ObjectName: "my_sub",
			ObjectType: "subscription",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", f.Level)
	}
	if f.Explanation == nil || f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected complete explanation, got %+v", f.Explanation)
	}
	if f.Metadata["object_name"] != "my_sub" {
		t.Fatalf("expected metadata object_name=my_sub, got %v", f.Metadata["object_name"])
	}
}

func TestAlterSubscriptionNoticeRule(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSubscriptionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterSubscription,
			ObjectName: "my_sub",
			ObjectType: "subscription",
			Options:    map[string]string{"action": "connection"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", f.Level)
	}
	if f.Explanation == nil || f.Explanation.Why == "" || f.Explanation.Risk == "" || f.Explanation.Suggestion == "" {
		t.Fatalf("expected complete explanation, got %+v", f.Explanation)
	}
}

func TestAlterSubscriptionDisableWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSubscriptionDisableWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterSubscription,
			ObjectName: "my_sub",
			ObjectType: "subscription",
			Options:    map[string]string{"action": "disable"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "disable" {
		t.Fatalf("expected metadata action=disable, got %v", findings[0].Metadata["action"])
	}
}

func TestDropSubscriptionWarnRule(t *testing.T) {
	t.Parallel()
	r := mustNewDropSubscriptionWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSubscription,
			ObjectName: "my_sub",
			ObjectType: "subscription",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
}

// ---------------------------------------------------------------------------
// Negative tests: rules skip for non-matching conditions
// ---------------------------------------------------------------------------

func TestReplicationRulesSkipMySQL(t *testing.T) {
	t.Parallel()
	rules := []rule.StatementRule{
		mustNewCreatePublicationNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterPublicationNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropPublicationWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewCreateSubscriptionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterSubscriptionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterSubscriptionDisableWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewDropSubscriptionWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreatePublication,
			ObjectName: "my_pub",
			ObjectType: "publication",
			Options:    map[string]string{"action": "disable"},
		},
	}

	for _, r := range rules {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", r.ID(), err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected %s to skip MySQL, got %d findings", r.ID(), len(findings))
		}
	}
}

func TestAlterSubscriptionDisableWarnSkipsEnable(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSubscriptionDisableWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterSubscription,
			ObjectName: "my_sub",
			ObjectType: "subscription",
			Options:    map[string]string{"action": "enable"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected disable warn to skip action=enable, got %d findings", len(findings))
	}
}

func TestAlterSubscriptionDisableWarnSkipsOtherActions(t *testing.T) {
	t.Parallel()
	r := mustNewAlterSubscriptionDisableWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	otherActions := []string{"connection", "refresh", "set_publication", "add_refresh"}
	for _, action := range otherActions {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			stmt := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation:  spec.DDLOperationAlterSubscription,
					ObjectName: "my_sub",
					ObjectType: "subscription",
					Options:    map[string]string{"action": action},
				},
			}

			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected disable warn to skip action=%s, got %d findings", action, len(findings))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry and defaults tests
// ---------------------------------------------------------------------------

func TestRegistryIncludesPGReplicationLifecycleRules(t *testing.T) {
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
		{ruleIDPGCreatePublicationNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreatePublication, ObjectName: "my_pub", ObjectType: "publication"},
		}},
		{ruleIDPGAlterPublicationNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterPublication, ObjectName: "my_pub", ObjectType: "publication", Options: map[string]string{"action": "add_table"}},
		}},
		{ruleIDPGDropPublicationWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropPublication, ObjectName: "my_pub", ObjectType: "publication"},
		}},
		{ruleIDPGCreateSubscriptionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateSubscription, ObjectName: "my_sub", ObjectType: "subscription"},
		}},
		{ruleIDPGAlterSubscriptionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterSubscription, ObjectName: "my_sub", ObjectType: "subscription", Options: map[string]string{"action": "connection"}},
		}},
		{ruleIDPGAlterSubscriptionDisableWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterSubscription, ObjectName: "my_sub", ObjectType: "subscription", Options: map[string]string{"action": "disable"}},
		}},
		{ruleIDPGDropSubscriptionWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropSubscription, ObjectName: "my_sub", ObjectType: "subscription"},
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
				t.Fatalf("expected registry to produce finding for %q, got %d findings", tc.ruleID, len(findings))
			}
		})
	}
}

func TestDefaultPolicyIncludesPGReplicationLifecycleRules(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGCreatePublicationNotice, rule.LevelNotice, true},
		{ruleIDPGAlterPublicationNotice, rule.LevelNotice, true},
		{ruleIDPGDropPublicationWarn, rule.LevelWarning, true},
		{ruleIDPGCreateSubscriptionNotice, rule.LevelNotice, true},
		{ruleIDPGAlterSubscriptionNotice, rule.LevelNotice, true},
		{ruleIDPGAlterSubscriptionDisableWarn, rule.LevelWarning, true},
		{ruleIDPGDropSubscriptionWarn, rule.LevelWarning, true},
	}

	for _, exp := range expected {
		t.Run(exp.id, func(t *testing.T) {
			t.Parallel()
			p, ok := cfg.Rules[exp.id]
			if !ok {
				t.Fatalf("expected default policy to include %q", exp.id)
			}
			if !p.Enabled {
				t.Fatalf("expected %q to be enabled", exp.id)
			}
			if p.Level != exp.level {
				t.Fatalf("expected %q level %q, got %q", exp.id, exp.level, p.Level)
			}
		})
	}
}

func TestReplicationLifecycleRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreatePublication,
			ObjectName: "my_pub",
			ObjectType: "publication",
			Options:    map[string]string{"action": "disable"},
		},
	}
	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGCreatePublicationNotice:      true,
		ruleIDPGAlterPublicationNotice:       true,
		ruleIDPGDropPublicationWarn:          true,
		ruleIDPGCreateSubscriptionNotice:     true,
		ruleIDPGAlterSubscriptionNotice:      true,
		ruleIDPGAlterSubscriptionDisableWarn: true,
		ruleIDPGDropSubscriptionWarn:         true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG replication lifecycle rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewCreatePublicationNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreatePublicationNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create publication notice rule: %v", err)
	}
	return r
}

func mustNewAlterPublicationNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterPublicationNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter publication notice rule: %v", err)
	}
	return r
}

func mustNewDropPublicationWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropPublicationWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop publication warn rule: %v", err)
	}
	return r
}

func mustNewCreateSubscriptionNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateSubscriptionNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create subscription notice rule: %v", err)
	}
	return r
}

func mustNewAlterSubscriptionNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterSubscriptionNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter subscription notice rule: %v", err)
	}
	return r
}

func mustNewAlterSubscriptionDisableWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterSubscriptionDisableWarnRule(cfg)
	if err != nil {
		t.Fatalf("new alter subscription disable warn rule: %v", err)
	}
	return r
}

func mustNewDropSubscriptionWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropSubscriptionWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop subscription warn rule: %v", err)
	}
	return r
}
