// Package ddl verifies PostgreSQL domain lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL domain lifecycle signals and cross-dialect policy controls
// output: focused coverage for the seven PostgreSQL domain lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG domain lifecycle
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

func TestCreateDomainNoticeRule(t *testing.T) {
	r := mustNewCreateDomainNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"base_type": "text", "has_check": "true"},
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
	if f.Metadata["base_type"] != "text" {
		t.Fatalf("expected metadata base_type=text, got %v", f.Metadata["base_type"])
	}
	if f.Metadata["has_check"] != "true" {
		t.Fatalf("expected metadata has_check=true, got %v", f.Metadata["has_check"])
	}
}

func TestAlterDomainConstraintAddNoticeRule(t *testing.T) {
	r := mustNewAlterDomainConstraintNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "add_constraint", "constraint": "email_not_empty", "has_check": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["constraint"] != "email_not_empty" {
		t.Fatalf("expected metadata constraint=email_not_empty, got %v", findings[0].Metadata["constraint"])
	}
}

func TestAlterDomainConstraintDropNoticeRule(t *testing.T) {
	r := mustNewAlterDomainConstraintNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "drop_constraint", "constraint": "email_not_empty"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainConstraintValidateNoticeRule(t *testing.T) {
	r := mustNewAlterDomainConstraintNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "validate_constraint", "constraint": "email_not_empty"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainDefaultSetNoticeRule(t *testing.T) {
	r := mustNewAlterDomainDefaultNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "set_default", "has_default": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainDefaultDropNoticeRule(t *testing.T) {
	r := mustNewAlterDomainDefaultNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "drop_default"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainNotNullSetNoticeRule(t *testing.T) {
	r := mustNewAlterDomainNotNullNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "set_not_null", "not_null": "true"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainNotNullDropNoticeRule(t *testing.T) {
	r := mustNewAlterDomainNotNullNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "drop_not_null"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAlterDomainRenameNoticeRule(t *testing.T) {
	r := mustNewAlterDomainRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "rename", "new_name": "contact_email"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["new_name"] != "contact_email" {
		t.Fatalf("expected metadata new_name=contact_email, got %v", findings[0].Metadata["new_name"])
	}
}

func TestDropDomainAdvisoryRule(t *testing.T) {
	r := mustNewDropDomainAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
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

func TestDropDomainCascadeWarnRule(t *testing.T) {
	r := mustNewDropDomainCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
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

// Duplicate finding: DROP DOMAIN CASCADE fires both drop_domain.advisory and drop_domain.cascade.warn.
func TestDropDomainCascadeFiresBothAdvisoryAndCascadeWarn(t *testing.T) {
	advisory := mustNewDropDomainAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	cascade := mustNewDropDomainCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	f1, _ := advisory.Evaluate(context.Background(), stmt)
	f2, _ := cascade.Evaluate(context.Background(), stmt)
	if len(f1) != 1 {
		t.Fatalf("expected advisory to fire for DROP DOMAIN CASCADE, got %d findings", len(f1))
	}
	if len(f2) != 1 {
		t.Fatalf("expected cascade warn to fire, got %d findings", len(f2))
	}
	if advisory.ID() == cascade.ID() {
		t.Fatalf("expected distinct rule IDs, got same: %s", advisory.ID())
	}
}

// ---------------------------------------------------------------------------
// Negative tests: rules skip for non-matching conditions
// ---------------------------------------------------------------------------

func TestDomainLifecycleRulesSkipMySQL(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateDomainNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterDomainConstraintNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterDomainDefaultNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterDomainNotNullNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterDomainRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropDomainAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewDropDomainCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "add_constraint", "cascade": "true"},
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

func TestAlterDomainConstraintNoticeSkipsNonConstraintAction(t *testing.T) {
	r := mustNewAlterDomainConstraintNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "set_default"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected constraint notice to skip set_default, got %d findings", len(findings))
	}
}

func TestAlterDomainDefaultNoticeSkipsNonDefaultAction(t *testing.T) {
	r := mustNewAlterDomainDefaultNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "set_not_null"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected default notice to skip set_not_null, got %d findings", len(findings))
	}
}

func TestAlterDomainNotNullNoticeSkipsNonNullAction(t *testing.T) {
	r := mustNewAlterDomainNotNullNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"action": "rename"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected not_null notice to skip rename, got %d findings", len(findings))
	}
}

func TestDropDomainCascadeWarnSkipsNonCascade(t *testing.T) {
	r := mustNewDropDomainCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected cascade warn to skip DROP DOMAIN without cascade, got %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Registry / default policy tests
// ---------------------------------------------------------------------------

func TestRegistryIncludesPGDomainLifecycleRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateDomainNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"base_type": "text"}},
		}},
		{ruleIDPGAlterDomainConstraintNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"action": "add_constraint"}},
		}},
		{ruleIDPGAlterDomainDefaultNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"action": "set_default"}},
		}},
		{ruleIDPGAlterDomainNotNullNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"action": "set_not_null"}},
		}},
		{ruleIDPGAlterDomainRenameNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"action": "rename"}},
		}},
		{ruleIDPGDropDomainAdvisory, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropDomain, ObjectName: "email", ObjectType: "domain"},
		}},
		{ruleIDPGDropDomainCascadeWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropDomain, ObjectName: "email", ObjectType: "domain", Options: map[string]string{"cascade": "true"}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
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

func TestDefaultPolicyIncludesPGDomainLifecycleRules(t *testing.T) {
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGCreateDomainNotice, rule.LevelNotice, true},
		{ruleIDPGAlterDomainConstraintNotice, rule.LevelNotice, true},
		{ruleIDPGAlterDomainDefaultNotice, rule.LevelNotice, true},
		{ruleIDPGAlterDomainNotNullNotice, rule.LevelNotice, true},
		{ruleIDPGAlterDomainRenameNotice, rule.LevelNotice, true},
		{ruleIDPGDropDomainAdvisory, rule.LevelWarning, true},
		{ruleIDPGDropDomainCascadeWarn, rule.LevelWarning, true},
	}

	for _, exp := range expected {
		t.Run(exp.id, func(t *testing.T) {
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

func TestDomainLifecycleRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "email",
			ObjectType: "domain",
			Options:    map[string]string{"cascade": "true"},
		},
	}
	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGCreateDomainNotice:          true,
		ruleIDPGAlterDomainConstraintNotice: true,
		ruleIDPGAlterDomainDefaultNotice:    true,
		ruleIDPGAlterDomainNotNullNotice:    true,
		ruleIDPGAlterDomainRenameNotice:     true,
		ruleIDPGDropDomainAdvisory:          true,
		ruleIDPGDropDomainCascadeWarn:       true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG domain lifecycle rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewCreateDomainNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateDomainNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create domain notice rule: %v", err)
	}
	return r
}

func mustNewAlterDomainConstraintNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterDomainConstraintNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter domain constraint notice rule: %v", err)
	}
	return r
}

func mustNewAlterDomainDefaultNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterDomainDefaultNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter domain default notice rule: %v", err)
	}
	return r
}

func mustNewAlterDomainNotNullNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterDomainNotNullNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter domain not_null notice rule: %v", err)
	}
	return r
}

func mustNewAlterDomainRenameNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterDomainRenameNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter domain rename notice rule: %v", err)
	}
	return r
}

func mustNewDropDomainAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropDomainAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop domain advisory rule: %v", err)
	}
	return r
}

func mustNewDropDomainCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropDomainCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop domain cascade warn rule: %v", err)
	}
	return r
}
