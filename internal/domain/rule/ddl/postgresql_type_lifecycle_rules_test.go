// Package ddl verifies PostgreSQL type lifecycle rule behavior.
// input: synthetic DDL statements with PostgreSQL type lifecycle signals and cross-dialect policy controls
// output: focused coverage for the five PostgreSQL type lifecycle rules with PG-only gating
// pos: domain DDL rule test coverage for PG type lifecycle
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

func TestCreateTypeEnumNoticeRule(t *testing.T) {
	r := mustNewCreateTypeEnumNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum"},
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
	if f.Metadata["type_kind"] != "enum" {
		t.Fatalf("expected metadata type_kind=enum, got %v", f.Metadata["type_kind"])
	}
}

func TestAlterTypeAddValueAdvisoryRule(t *testing.T) {
	r := mustNewAlterTypeAddValueAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "add_value", "value": "yellow"},
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
	if findings[0].Metadata["value"] != "yellow" {
		t.Fatalf("expected metadata value=yellow, got %v", findings[0].Metadata["value"])
	}
}

func TestAlterTypeAddValuePositionNoticeRule(t *testing.T) {
	r := mustNewAlterTypeAddValuePositionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "add_value", "value": "yellow", "placement": "after", "neighbor": "green"},
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
	if f.Metadata["placement"] != "after" {
		t.Fatalf("expected metadata placement=after, got %v", f.Metadata["placement"])
	}
	if f.Metadata["neighbor"] != "green" {
		t.Fatalf("expected metadata neighbor=green, got %v", f.Metadata["neighbor"])
	}
}

func TestAlterTypeAddValueWithPositionFiresBothRules(t *testing.T) {
	advisory := mustNewAlterTypeAddValueAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	position := mustNewAlterTypeAddValuePositionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "add_value", "value": "yellow", "placement": "before", "neighbor": "green"},
		},
	}

	f1, _ := advisory.Evaluate(stmt)
	f2, _ := position.Evaluate(stmt)
	if len(f1) != 1 {
		t.Fatalf("expected advisory to fire, got %d findings", len(f1))
	}
	if len(f2) != 1 {
		t.Fatalf("expected position notice to fire, got %d findings", len(f2))
	}
}

func TestDropTypeAdvisoryRule(t *testing.T) {
	r := mustNewDropTypeAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
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

func TestDropTypeCascadeWarnRule(t *testing.T) {
	r := mustNewDropTypeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
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

// Duplicate finding: DROP TYPE CASCADE fires both drop_type.advisory and drop_type.cascade.warn.
func TestDropTypeCascadeFiresBothAdvisoryAndCascadeWarn(t *testing.T) {
	advisory := mustNewDropTypeAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	cascade := mustNewDropTypeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"cascade": "true"},
		},
	}

	f1, _ := advisory.Evaluate(stmt)
	f2, _ := cascade.Evaluate(stmt)
	if len(f1) != 1 {
		t.Fatalf("expected advisory to fire for DROP TYPE CASCADE, got %d findings", len(f1))
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

func TestTypeLifecycleRulesSkipMySQL(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateTypeEnumNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeAddValueAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewAlterTypeAddValuePositionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewDropTypeAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewDropTypeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewCreateTypeCompositeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "add_value", "cascade": "true"},
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

func TestCreateTypeEnumNoticeSkipsNonEnum(t *testing.T) {
	r := mustNewCreateTypeEnumNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "composite"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected enum notice to skip composite type, got %d findings", len(findings))
	}
}

func TestAlterTypeAddValueAdvisorySkipsNonAddValue(t *testing.T) {
	r := mustNewAlterTypeAddValueAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "rename"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected advisory to skip non-add_value, got %d findings", len(findings))
	}
}

func TestPositionNoticeSkipsNoPlacement(t *testing.T) {
	r := mustNewAlterTypeAddValuePositionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum", "action": "add_value", "value": "yellow"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected position notice to skip add_value without placement, got %d findings", len(findings))
	}
}

func TestDropTypeCascadeWarnSkipsNonCascade(t *testing.T) {
	r := mustNewDropTypeCascadeWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected cascade warn to skip DROP TYPE without cascade, got %d findings", len(findings))
	}
}

func TestCreateTypeCompositeNoticeRule(t *testing.T) {
	r := mustNewCreateTypeCompositeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "composite", "attributes": "2", "attribute_names": "street,city"},
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
	if f.Metadata["type_kind"] != "composite" {
		t.Fatalf("expected metadata type_kind=composite, got %v", f.Metadata["type_kind"])
	}
	if f.Metadata["attributes"] != "2" {
		t.Fatalf("expected metadata attributes=2, got %v", f.Metadata["attributes"])
	}
	if f.Metadata["attribute_names"] != "street,city" {
		t.Fatalf("expected metadata attribute_names=street,city, got %v", f.Metadata["attribute_names"])
	}
}

func TestAlterTypeCompositeRenameNoticeRule(t *testing.T) {
	r := mustNewAlterTypeCompositeRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"action": "rename", "new_name": "mailing_address"},
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
	if f.Metadata["new_name"] != "mailing_address" {
		t.Fatalf("expected metadata new_name=mailing_address, got %v", f.Metadata["new_name"])
	}
}

func TestAlterTypeCompositeSetSchemaNoticeRule(t *testing.T) {
	r := mustNewAlterTypeCompositeSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"action": "set_schema", "new_schema": "archive"},
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
	if f.Metadata["new_schema"] != "archive" {
		t.Fatalf("expected metadata new_schema=archive, got %v", f.Metadata["new_schema"])
	}
}

// ---------------------------------------------------------------------------
// Negative tests: composite type rules skip non-matching conditions
// ---------------------------------------------------------------------------

func TestCreateTypeCompositeNoticeSkipsEnum(t *testing.T) {
	r := mustNewCreateTypeCompositeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "enum"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected composite notice to skip enum type, got %d findings", len(findings))
	}
}

func TestAlterTypeCompositeRenameSkipsAddValue(t *testing.T) {
	r := mustNewAlterTypeCompositeRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"action": "add_value", "value": "yellow"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected rename notice to skip add_value, got %d findings", len(findings))
	}
}

func TestAlterTypeCompositeSetSchemaSkipsRename(t *testing.T) {
	r := mustNewAlterTypeCompositeSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"action": "rename", "new_name": "mailing_address"},
		},
	}

	findings, err := r.Evaluate(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected set_schema notice to skip rename, got %d findings", len(findings))
	}
}

func TestCompositeTypeRulesSkipDropType(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateTypeCompositeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "address",
			ObjectType: "type",
		},
	}

	for _, r := range rules {
		findings, err := r.Evaluate(stmt)
		if err != nil {
			t.Fatalf("evaluate %s: %v", r.ID(), err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected %s to skip DROP TYPE, got %d findings", r.ID(), len(findings))
		}
	}
}

func TestCompositeTypeRulesSkipMySQL(t *testing.T) {
	rules := []rule.StatementRule{
		mustNewCreateTypeCompositeNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeRenameNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
		mustNewAlterTypeCompositeSetSchemaNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateType,
			ObjectName: "address",
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "composite"},
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

func TestRegistryIncludesPGTypeLifecycleRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Verify each rule fires through the registry for a matching PG statement.
	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateTypeEnumNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateType, ObjectName: "color", ObjectType: "type", Options: map[string]string{"type_kind": "enum"}},
		}},
		{ruleIDPGAlterTypeAddValueAdvisory, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterType, ObjectName: "color", ObjectType: "type", Options: map[string]string{"action": "add_value"}},
		}},
		{ruleIDPGAlterTypeAddValuePositionNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterType, ObjectName: "color", ObjectType: "type", Options: map[string]string{"action": "add_value", "placement": "before", "neighbor": "green"}},
		}},
		{ruleIDPGDropTypeAdvisory, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropType, ObjectName: "color", ObjectType: "type"},
		}},
		{ruleIDPGDropTypeCascadeWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropType, ObjectName: "color", ObjectType: "type", Options: map[string]string{"cascade": "true"}},
		}},
		{ruleIDPGCreateTypeCompositeNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateType, ObjectName: "address", ObjectType: "type", Options: map[string]string{"type_kind": "composite"}},
		}},
		{ruleIDPGAlterTypeCompositeRenameNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterType, ObjectName: "address", ObjectType: "type", Options: map[string]string{"action": "rename", "new_name": "mailing_address"}},
		}},
		{ruleIDPGAlterTypeCompositeSetSchemaNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterType, ObjectName: "address", ObjectType: "type", Options: map[string]string{"action": "set_schema", "new_schema": "archive"}},
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

func TestDefaultPolicyIncludesPGTypeLifecycleRules(t *testing.T) {
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGCreateTypeEnumNotice, rule.LevelNotice, true},
		{ruleIDPGAlterTypeAddValueAdvisory, rule.LevelWarning, true},
		{ruleIDPGAlterTypeAddValuePositionNotice, rule.LevelNotice, true},
		{ruleIDPGDropTypeAdvisory, rule.LevelWarning, true},
		{ruleIDPGDropTypeCascadeWarn, rule.LevelWarning, true},
		{ruleIDPGCreateTypeCompositeNotice, rule.LevelNotice, true},
		{ruleIDPGAlterTypeCompositeRenameNotice, rule.LevelNotice, true},
		{ruleIDPGAlterTypeCompositeSetSchemaNotice, rule.LevelNotice, true},
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

func TestTypeLifecycleRulesDoNotFireForMySQLViaRegistry(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "color",
			ObjectType: "type",
			Options:    map[string]string{"cascade": "true"},
		},
	}
	findings, err := registry.EvaluateStatement(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	pgRuleIDs := map[string]bool{
		ruleIDPGCreateTypeEnumNotice:              true,
		ruleIDPGAlterTypeAddValueAdvisory:         true,
		ruleIDPGAlterTypeAddValuePositionNotice:   true,
		ruleIDPGDropTypeAdvisory:                  true,
		ruleIDPGDropTypeCascadeWarn:               true,
		ruleIDPGCreateTypeCompositeNotice:         true,
		ruleIDPGAlterTypeCompositeRenameNotice:    true,
		ruleIDPGAlterTypeCompositeSetSchemaNotice: true,
	}
	for _, f := range findings {
		if pgRuleIDs[f.RuleID] {
			t.Fatalf("expected PG type lifecycle rule %q not to fire for MySQL", f.RuleID)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewCreateTypeEnumNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateTypeEnumNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create type enum notice rule: %v", err)
	}
	return r
}

func mustNewAlterTypeAddValueAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterTypeAddValueAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new alter type add value advisory rule: %v", err)
	}
	return r
}

func mustNewAlterTypeAddValuePositionNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterTypeAddValuePositionNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter type add value position notice rule: %v", err)
	}
	return r
}

func mustNewDropTypeAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropTypeAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop type advisory rule: %v", err)
	}
	return r
}

func mustNewDropTypeCascadeWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropTypeCascadeWarnRule(cfg)
	if err != nil {
		t.Fatalf("new drop type cascade warn rule: %v", err)
	}
	return r
}

func mustNewCreateTypeCompositeNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newCreateTypeCompositeNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new create type composite notice rule: %v", err)
	}
	return r
}

func mustNewAlterTypeCompositeRenameNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterTypeCompositeRenameNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter type composite rename notice rule: %v", err)
	}
	return r
}

func mustNewAlterTypeCompositeSetSchemaNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAlterTypeCompositeSetSchemaNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new alter type composite set schema notice rule: %v", err)
	}
	return r
}
