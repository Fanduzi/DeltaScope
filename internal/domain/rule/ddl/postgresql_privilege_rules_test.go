// Package ddl verifies PostgreSQL table privilege rule behavior.
// input: synthetic DDL statements with PostgreSQL table privilege DCL signals and cross-dialect policy controls
// output: focused coverage for the four PostgreSQL table privilege rules with PG-only gating
// pos: domain DDL rule test coverage for PG table privilege DCL
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

func TestGrantTablePrivilegeNoticeRule(t *testing.T) {
	r := mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
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
	if f.Metadata["object_name"] != "users" {
		t.Fatalf("expected metadata object_name=users, got %v", f.Metadata["object_name"])
	}
	if f.Metadata["privileges"] != "select" {
		t.Fatalf("expected metadata privileges=select, got %v", f.Metadata["privileges"])
	}
	if f.Metadata["grantees"] != "analyst" {
		t.Fatalf("expected metadata grantees=analyst, got %v", f.Metadata["grantees"])
	}
}

func TestGrantTablePrivilegeMultiplePrivilegesAndGrantees(t *testing.T) {
	r := mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select,insert", "grantees": "analyst,app_user"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["privileges"] != "select,insert" {
		t.Fatalf("expected metadata privileges=select,insert, got %v", findings[0].Metadata["privileges"])
	}
	if findings[0].Metadata["grantees"] != "analyst,app_user" {
		t.Fatalf("expected metadata grantees=analyst,app_user, got %v", findings[0].Metadata["grantees"])
	}
}

func TestGrantTablePrivilegeWithSchema(t *testing.T) {
	r := mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst", "schema": "public"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["schema"] != "public" {
		t.Fatalf("expected metadata schema=public, got %v", findings[0].Metadata["schema"])
	}
}

func TestGrantTablePrivilegeAllWarnRule(t *testing.T) {
	r := mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"all_privileges": "true", "grantees": "analyst"},
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
	if findings[0].Metadata["all_privileges"] != "true" {
		t.Fatalf("expected metadata all_privileges=true, got %v", findings[0].Metadata["all_privileges"])
	}
}

// Duplicate finding: GRANT ALL PRIVILEGES fires both notice and all.warn.
func TestGrantAllPrivilegesFiresBothNoticeAndAllWarn(t *testing.T) {
	notice := mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	allWarn := mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"all_privileges": "true", "grantees": "analyst"},
		},
	}

	f1, _ := notice.Evaluate(context.Background(), stmt)
	f2, _ := allWarn.Evaluate(context.Background(), stmt)
	if len(f1) != 1 {
		t.Fatalf("expected notice to fire for GRANT ALL PRIVILEGES, got %d findings", len(f1))
	}
	if len(f2) != 1 {
		t.Fatalf("expected all-privileges warn to fire, got %d findings", len(f2))
	}
	if notice.ID() == allWarn.ID() {
		t.Fatalf("expected distinct rule IDs, got same: %s", notice.ID())
	}
}

func TestRevokeTablePrivilegeNoticeRule(t *testing.T) {
	r := mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRevokeTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["object_name"] != "users" {
		t.Fatalf("expected metadata object_name=users, got %v", findings[0].Metadata["object_name"])
	}
}

func TestRevokeTablePrivilegeMultiplePrivilegesAndGrantees(t *testing.T) {
	r := mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRevokeTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select,insert", "grantees": "analyst,app_user"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["privileges"] != "select,insert" {
		t.Fatalf("expected metadata privileges=select,insert, got %v", findings[0].Metadata["privileges"])
	}
	if findings[0].Metadata["grantees"] != "analyst,app_user" {
		t.Fatalf("expected metadata grantees=analyst,app_user, got %v", findings[0].Metadata["grantees"])
	}
}

func TestRevokeTablePrivilegeCascadeWarnRule(t *testing.T) {
	r := mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRevokeTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"all_privileges": "true", "grantees": "analyst", "cascade": "true"},
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
	if findings[0].Metadata["cascade"] != "true" {
		t.Fatalf("expected metadata cascade=true, got %v", findings[0].Metadata["cascade"])
	}
}

// Duplicate finding: REVOKE CASCADE fires both notice and cascade.warn.
func TestRevokeCascadeFiresBothNoticeAndCascadeWarn(t *testing.T) {
	notice := mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	cascadeWarn := mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRevokeTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst", "cascade": "true"},
		},
	}

	f1, _ := notice.Evaluate(context.Background(), stmt)
	f2, _ := cascadeWarn.Evaluate(context.Background(), stmt)
	if len(f1) != 1 {
		t.Fatalf("expected notice to fire for REVOKE CASCADE, got %d findings", len(f1))
	}
	if len(f2) != 1 {
		t.Fatalf("expected cascade warn to fire, got %d findings", len(f2))
	}
	if notice.ID() == cascadeWarn.ID() {
		t.Fatalf("expected distinct rule IDs, got same: %s", notice.ID())
	}
}

// ---------------------------------------------------------------------------
// Negative tests: rules skip for non-matching conditions
// ---------------------------------------------------------------------------

func TestTablePrivilegeRulesSkipMySQL(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst", "all_privileges": "true", "cascade": "true"},
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

func TestTablePrivilegeRulesSkipTiDB(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
		},
	}

	for _, r := range rules {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", r.ID(), err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected %s to skip TiDB, got %d findings", r.ID(), len(findings))
		}
	}
}

func TestGrantAllPrivilegesWarnSkipsNonAll(t *testing.T) {
	r := mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected all-privileges warn to skip GRANT SELECT, got %d findings", len(findings))
	}
}

func TestRevokeCascadeWarnSkipsNonCascade(t *testing.T) {
	r := mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRevokeTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected cascade warn to skip REVOKE without CASCADE, got %d findings", len(findings))
	}
}

// Deferred forms do not trigger table privilege rules.
func TestTablePrivilegeRulesSkipDeferredForms(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewGrantTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewGrantTablePrivilegeAllWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewRevokeTablePrivilegeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewRevokeTablePrivilegeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	// All these use different operations that don't match grant_table/revoke_table.
	deferredStmts := []spec.Statement{
		{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  "grant_all_tables_in_schema",
				ObjectName: "users",
				ObjectType: "table",
				Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
			},
		},
		{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  "grant_role",
				ObjectName: "analyst",
				ObjectType: "role",
				Options:    map[string]string{"grantees": "app_user"},
			},
		},
		{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  "revoke_role",
				ObjectName: "analyst",
				ObjectType: "role",
				Options:    map[string]string{"grantees": "app_user"},
			},
		},
		{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation:  "alter_default_privileges",
				ObjectName: "users",
				ObjectType: "table",
				Options:    map[string]string{"privileges": "select", "grantees": "analyst"},
			},
		},
	}

	for _, r := range rules {
		for _, stmt := range deferredStmts {
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate %s: %v", r.ID(), err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected %s to skip deferred form %q, got %d findings", r.ID(), stmt.DDL.Operation, len(findings))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Registry and defaults tests
// ---------------------------------------------------------------------------

func TestRegistryIncludesPGTablePrivilegeRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGGrantTablePrivilegeNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationGrantTable, ObjectName: "users", ObjectType: "table", Options: map[string]string{"privileges": "select", "grantees": "analyst"}},
		}},
		{ruleIDPGGrantTablePrivilegeAllWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationGrantTable, ObjectName: "users", ObjectType: "table", Options: map[string]string{"all_privileges": "true", "grantees": "analyst"}},
		}},
		{ruleIDPGRevokeTablePrivilegeNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationRevokeTable, ObjectName: "users", ObjectType: "table", Options: map[string]string{"privileges": "select", "grantees": "analyst"}},
		}},
		{ruleIDPGRevokeTablePrivilegeCascadeWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationRevokeTable, ObjectName: "users", ObjectType: "table", Options: map[string]string{"privileges": "select", "grantees": "analyst", "cascade": "true"}},
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

func TestDefaultPolicyIncludesPGTablePrivilegeRules(t *testing.T) {
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGGrantTablePrivilegeNotice, rule.LevelNotice, true},
		{ruleIDPGGrantTablePrivilegeAllWarn, rule.LevelWarning, true},
		{ruleIDPGRevokeTablePrivilegeNotice, rule.LevelNotice, true},
		{ruleIDPGRevokeTablePrivilegeCascadeWarn, rule.LevelWarning, true},
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

func TestTablePrivilegeRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationGrantTable,
			ObjectName: "users",
			ObjectType: "table",
			Options:    map[string]string{"privileges": "select", "grantees": "analyst", "all_privileges": "true", "cascade": "true"},
		},
	}
	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGGrantTablePrivilegeNotice:       true,
		ruleIDPGGrantTablePrivilegeAllWarn:      true,
		ruleIDPGRevokeTablePrivilegeNotice:      true,
		ruleIDPGRevokeTablePrivilegeCascadeWarn: true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG table privilege rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewGrantTablePrivilegeNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newGrantTablePrivilegeNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new grant table privilege notice rule: %v", err)
	}
	return r
}

func mustNewGrantTablePrivilegeAllWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newGrantTablePrivilegeAllWarnRule(cfg)
	if err != nil {
		t.Fatalf("new grant table privilege all warn rule: %v", err)
	}
	return r
}

func mustNewRevokeTablePrivilegeNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newRevokeTablePrivilegeNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new revoke table privilege notice rule: %v", err)
	}
	return r
}

func mustNewRevokeTablePrivilegeCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newRevokeTablePrivilegeCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new revoke table privilege cascade warn rule: %v", err)
	}
	return r
}
