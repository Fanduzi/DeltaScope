// Package ddl verifies the T06-A11 single-member primary-key BIGINT/UNSIGNED
// rule boundaries on synthetic statements.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t06A11BigintRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	statementRule, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyBigintRequire,
		rule.LevelBlocker,
		"must use bigint",
		"change the primary key column type to bigint",
		func(column spec.Column) bool { return baseType(column) == "bigint" },
		cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

func t06A11UnsignedRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	statementRule, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyUnsignedRequire,
		rule.LevelBlocker,
		"must be unsigned",
		"mark the primary key column as UNSIGNED",
		func(column spec.Column) bool { return column.Unsigned },
		cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

var t06A11RequiredTrue = policy.RulePolicy{
	Enabled: true, Level: rule.LevelBlocker,
	Params: map[string]any{"required": true}}

// TestT06A11RequiredFalseSilencesBothRules pins required=false as a distinct
// control from disabled: each rule still constructs but applies to nothing.
func TestT06A11RequiredFalseSilencesBothRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker,
		Params: map[string]any{"required": false}}
	for name, statementRule := range map[string]rule.StatementRule{
		"bigint":   t06A11BigintRule(t, cfg),
		"unsigned": t06A11UnsignedRule(t, cfg),
	} {
		violating := primaryKeyStatement(spec.Column{Name: "id", Type: "int", NotNull: true})
		if statementRule.AppliesTo(violating) {
			t.Fatalf("%s required=false rule must not apply", name)
		}
		findings, err := statementRule.Evaluate(ctx, violating)
		if err != nil || len(findings) != 0 {
			t.Fatalf("%s required=false findings = %v, err = %v; want none", name, findings, err)
		}
	}
}

// TestT06A11DefaultLevelAndInvalidParameter pins the blocker fallback level
// and boolParam's strict typing: a non-bool `required` fails construction.
func TestT06A11DefaultLevelAndInvalidParameter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	noLevel := policy.RulePolicy{Enabled: true, Params: map[string]any{"required": true}}
	for name, statementRule := range map[string]rule.StatementRule{
		"bigint":   t06A11BigintRule(t, noLevel),
		"unsigned": t06A11UnsignedRule(t, noLevel),
	} {
		findings, err := statementRule.Evaluate(ctx,
			primaryKeyStatement(spec.Column{Name: "id", Type: "int", NotNull: true}))
		if err != nil {
			t.Fatalf("%s evaluate: %v", name, err)
		}
		if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
			t.Fatalf("%s findings = %+v, want one blocker under the default level", name, findings)
		}
	}

	badParams := policy.RulePolicy{Enabled: true, Params: map[string]any{"required": "yes"}}
	if _, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyBigintRequire, rule.LevelBlocker, "must use bigint",
		"change the primary key column type to bigint",
		func(column spec.Column) bool { return baseType(column) == "bigint" },
		badParams); err == nil {
		t.Fatal("non-bool required must fail bigint rule construction")
	}
	if _, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyUnsignedRequire, rule.LevelBlocker, "must be unsigned",
		"mark the primary key column as UNSIGNED",
		func(column spec.Column) bool { return column.Unsigned },
		badParams); err == nil {
		t.Fatal("non-bool required must fail unsigned rule construction")
	}
}

// TestT06A11RuleIndependence pins the shared-layer controls: a signed BIGINT
// satisfies the BIGINT rule while staying a violation of the UNSIGNED rule,
// and an INT UNSIGNED satisfies the UNSIGNED rule while staying a violation
// of the BIGINT rule — neither rule borrows the other's fact.
func TestT06A11RuleIndependence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bigint := t06A11BigintRule(t, t06A11RequiredTrue)
	unsigned := t06A11UnsignedRule(t, t06A11RequiredTrue)

	signedBigint := primaryKeyStatement(spec.Column{Name: "id", Type: "bigint(20)", NotNull: true})
	if findings, err := bigint.Evaluate(ctx, signedBigint); err != nil || len(findings) != 0 {
		t.Fatalf("bigint rule on signed BIGINT = %v, err = %v; want none — UNSIGNED is not required", findings, err)
	}
	if findings, err := unsigned.Evaluate(ctx, signedBigint); err != nil || len(findings) != 1 {
		t.Fatalf("unsigned rule on signed BIGINT = %v, err = %v; want one finding", findings, err)
	}

	unsignedInt := primaryKeyStatement(spec.Column{Name: "id", Type: "int(11)", NotNull: true, Unsigned: true})
	if findings, err := unsigned.Evaluate(ctx, unsignedInt); err != nil || len(findings) != 0 {
		t.Fatalf("unsigned rule on INT UNSIGNED = %v, err = %v; want none — BIGINT is not required", findings, err)
	}
	if findings, err := bigint.Evaluate(ctx, unsignedInt); err != nil || len(findings) != 1 {
		t.Fatalf("bigint rule on INT UNSIGNED = %v, err = %v; want one finding", findings, err)
	}
}

// TestT06A11SingleMemberGateAndCompositeSkip pins that each rule inspects
// exactly one bound PK member: a two-member composite primary key yields zero
// findings — a member-count skip, never "all members satisfy".
func TestT06A11SingleMemberGateAndCompositeSkip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	composite := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "t"},
			Columns: []spec.Column{
				{Name: "id", Type: "int(11)", NotNull: true},
				{Name: "tenant_id", Type: "int(11)", NotNull: true},
			},
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary,
				Columns: []string{"id", "tenant_id"}},
		},
	}
	for name, statementRule := range map[string]rule.StatementRule{
		"bigint":   t06A11BigintRule(t, t06A11RequiredTrue),
		"unsigned": t06A11UnsignedRule(t, t06A11RequiredTrue),
	} {
		if !statementRule.AppliesTo(composite) {
			t.Fatalf("%s rule must still apply to a create-table with a primary key", name)
		}
		findings, err := statementRule.Evaluate(ctx, composite)
		if err != nil || len(findings) != 0 {
			t.Fatalf("%s composite findings = %v, err = %v; want none — skipped, not satisfied",
				name, findings, err)
		}
	}
}
