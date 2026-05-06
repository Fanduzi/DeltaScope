// Package ddl verifies PostgreSQL extension lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL extension lifecycle signals and cross-dialect policy controls
// output: focused coverage for the six PostgreSQL extension lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG extension lifecycle
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Positive tests: rules fire for matching PG statements
// ---------------------------------------------------------------------------

func TestCreateExtensionNoticeRule(t *testing.T) {
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
	}

	findings, err := r.Evaluate(stmt)
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
	if f.Metadata["object_name"] != "pg_trgm" {
		t.Fatalf("expected metadata object_name=pg_trgm, got %v", f.Metadata["object_name"])
	}
}

func TestCreateExtensionWithIfNotExistsMetadata(t *testing.T) {
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"if_not_exists": "true"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["if_not_exists"] != "true" {
		t.Fatalf("expected metadata if_not_exists=true, got %v", findings[0].Metadata["if_not_exists"])
	}
}

func TestCreateExtensionWithSchemaMetadata(t *testing.T) {
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"schema": "public"},
		},
	}

	findings, err := r.Evaluate(stmt)
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

func TestCreateExtensionWithVersionMetadata(t *testing.T) {
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"version": "1.6"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["version"] != "1.6" {
		t.Fatalf("expected metadata version=1.6, got %v", findings[0].Metadata["version"])
	}
}

func TestCreateExtensionCascadeWarnRule(t *testing.T) {
	r := mustNewCreateExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	findings, err := r.Evaluate(stmt)
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

// Duplicate finding: CREATE EXTENSION CASCADE fires both notice and cascade.warn.
func TestCreateExtensionCascadeFiresBothNoticeAndCascadeWarn(t *testing.T) {
	notice := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	cascade := mustNewCreateExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	f1, _ := notice.Evaluate(stmt)
	f2, _ := cascade.Evaluate(stmt)
	if len(f1) != 1 {
		t.Fatalf("expected notice to fire for CREATE EXTENSION CASCADE, got %d findings", len(f1))
	}
	if len(f2) != 1 {
		t.Fatalf("expected cascade warn to fire, got %d findings", len(f2))
	}
	if notice.ID() == cascade.ID() {
		t.Fatalf("expected distinct rule IDs, got same: %s", notice.ID())
	}
}

func TestAlterExtensionUpdateNoticeRule(t *testing.T) {
	r := mustNewAlterExtensionUpdateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "update"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "update" {
		t.Fatalf("expected metadata action=update, got %v", findings[0].Metadata["action"])
	}
}

func TestAlterExtensionUpdateWithVersionMetadata(t *testing.T) {
	r := mustNewAlterExtensionUpdateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "update", "version": "1.6"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["version"] != "1.6" {
		t.Fatalf("expected metadata version=1.6, got %v", findings[0].Metadata["version"])
	}
}

func TestAlterExtensionSetSchemaNoticeRule(t *testing.T) {
	r := mustNewAlterExtensionSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "set_schema", "new_schema": "extensions"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["new_schema"] != "extensions" {
		t.Fatalf("expected metadata new_schema=extensions, got %v", findings[0].Metadata["new_schema"])
	}
}

func TestDropExtensionAdvisoryRule(t *testing.T) {
	r := mustNewDropExtensionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
	}

	findings, err := r.Evaluate(stmt)
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

func TestDropExtensionCascadeWarnRule(t *testing.T) {
	r := mustNewDropExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
		},
	}

	findings, err := r.Evaluate(stmt)
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

// Duplicate finding: DROP EXTENSION CASCADE fires both advisory and cascade.warn.
func TestDropExtensionCascadeFiresBothAdvisoryAndCascadeWarn(t *testing.T) {
	advisory := mustNewDropExtensionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	cascade := mustNewDropExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	f1, _ := advisory.Evaluate(stmt)
	f2, _ := cascade.Evaluate(stmt)
	if len(f1) != 1 {
		t.Fatalf("expected advisory to fire for DROP EXTENSION CASCADE, got %d findings", len(f1))
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

func TestExtensionLifecycleRulesSkipMySQL(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewCreateExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewAlterExtensionUpdateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterExtensionSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropExtensionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewDropExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	for _, r := range rules {
		findings, err := r.Evaluate(stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", r.ID(), err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected %s to skip MySQL, got %d findings", r.ID(), len(findings))
		}
	}
}

func TestCreateExtensionCascadeWarnSkipsNonCascade(t *testing.T) {
	r := mustNewCreateExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected cascade warn to skip CREATE EXTENSION without cascade, got %d findings", len(findings))
	}
}

func TestAlterExtensionUpdateSkipsNonUpdate(t *testing.T) {
	r := mustNewAlterExtensionUpdateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "set_schema", "new_schema": "extensions"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected update notice to skip set_schema, got %d findings", len(findings))
	}
}

func TestAlterExtensionSetSchemaSkipsNonSetSchema(t *testing.T) {
	r := mustNewAlterExtensionSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "update"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected set_schema notice to skip update, got %d findings", len(findings))
	}
}

func TestDropExtensionCascadeWarnSkipsNonCascade(t *testing.T) {
	r := mustNewDropExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected cascade warn to skip DROP EXTENSION without cascade, got %d findings", len(findings))
	}
}

// Deferred member mutation specs do not trigger extension rules.
func TestExtensionRulesSkipMemberMutation(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewCreateExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewAlterExtensionUpdateNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterExtensionSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropExtensionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewDropExtensionCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
	}

	addMember := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperation("alter_extension_add_member"),
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "add_member"},
		},
	}
	dropMember := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperation("alter_extension_drop_member"),
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"action": "drop_member"},
		},
	}

	for _, r := range rules {
		for _, stmt := range []spec.Statement{addMember, dropMember} {
			findings, err := r.Evaluate(stmt)
			if err != nil {
				t.Fatalf("evaluate %s: %v", r.ID(), err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected %s to skip member mutation, got %d findings", r.ID(), len(findings))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Registry and defaults tests
// ---------------------------------------------------------------------------

func TestRegistryIncludesPGExtensionLifecycleRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateExtensionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateExtension, ObjectName: "pg_trgm", ObjectType: "extension"},
		}},
		{ruleIDPGCreateExtensionCascadeWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateExtension, ObjectName: "pg_trgm", ObjectType: "extension", Options: map[string]string{"cascade": "true"}},
		}},
		{ruleIDPGAlterExtensionUpdateNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterExtension, ObjectName: "pg_trgm", ObjectType: "extension", Options: map[string]string{"action": "update"}},
		}},
		{ruleIDPGAlterExtensionSetSchemaNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterExtension, ObjectName: "pg_trgm", ObjectType: "extension", Options: map[string]string{"action": "set_schema", "new_schema": "extensions"}},
		}},
		{ruleIDPGDropExtensionAdvisory, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropExtension, ObjectName: "pg_trgm", ObjectType: "extension"},
		}},
		{ruleIDPGDropExtensionCascadeWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropExtension, ObjectName: "pg_trgm", ObjectType: "extension", Options: map[string]string{"cascade": "true"}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			findings, err := registry.EvaluateStatement(tc.stmt)
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

func TestDefaultPolicyIncludesPGExtensionLifecycleRules(t *testing.T) {
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGCreateExtensionNotice, rule.LevelNotice, true},
		{ruleIDPGCreateExtensionCascadeWarn, rule.LevelWarning, true},
		{ruleIDPGAlterExtensionUpdateNotice, rule.LevelNotice, true},
		{ruleIDPGAlterExtensionSetSchemaNotice, rule.LevelNotice, true},
		{ruleIDPGDropExtensionAdvisory, rule.LevelWarning, true},
		{ruleIDPGDropExtensionCascadeWarn, rule.LevelWarning, true},
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

func TestExtensionLifecycleRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
			Options:    map[string]string{"cascade": "true"},
		},
	}
	findings, err := registry.EvaluateStatement(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGCreateExtensionNotice:         true,
		ruleIDPGCreateExtensionCascadeWarn:    true,
		ruleIDPGAlterExtensionUpdateNotice:    true,
		ruleIDPGAlterExtensionSetSchemaNotice: true,
		ruleIDPGDropExtensionAdvisory:         true,
		ruleIDPGDropExtensionCascadeWarn:      true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG extension lifecycle rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewCreateExtensionNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateExtensionNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create extension notice rule: %v", err)
	}
	return r
}

func mustNewCreateExtensionCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateExtensionCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new create extension cascade warn rule: %v", err)
	}
	return r
}

func mustNewAlterExtensionUpdateNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterExtensionUpdateNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter extension update notice rule: %v", err)
	}
	return r
}

func mustNewAlterExtensionSetSchemaNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterExtensionSetSchemaNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter extension set schema notice rule: %v", err)
	}
	return r
}

func mustNewDropExtensionAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropExtensionAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop extension advisory rule: %v", err)
	}
	return r
}

func mustNewDropExtensionCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropExtensionCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop extension cascade warn rule: %v", err)
	}
	return r
}
