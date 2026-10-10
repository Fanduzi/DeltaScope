// Package ddl verifies the T06-A10 AUTO_INCREMENT rule boundaries.
// input: synthetic create-table statements exercising the pk auto_increment
// require rule and the table init-value equality rule under boundary policies
// output: required/disabled/level/parameter and composite-skip semantics pinned
// pos: domain DDL rule regression for issue #85 T06-A10
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t06A10PKRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	statementRule, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyAutoIncrementRequire,
		rule.LevelBlocker,
		"must use auto_increment",
		"add AUTO_INCREMENT to the primary key column",
		func(column spec.Column) bool { return column.AutoIncrement },
		cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

// TestT06A10PrimaryKeyAutoIncrementBoundaries pins the required parameter,
// the enabled gate, and the composite-PK skip on the single-member rule.
func TestT06A10PrimaryKeyAutoIncrementBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	missingAuto := primaryKeyStatement(spec.Column{Name: "id", Type: "bigint", NotNull: true})

	// required=false silences the missing AUTO_INCREMENT finding entirely.
	silent := t06A10PKRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": false}})
	if silent.AppliesTo(missingAuto) {
		t.Fatal("required=false rule must not apply")
	}
	findings, err := silent.Evaluate(ctx, missingAuto)
	if err != nil || len(findings) != 0 {
		t.Fatalf("required=false findings = %v, err = %v; want none", findings, err)
	}

	// The composite primary key keeps two members and is skipped by Evaluate;
	// the skip is a member-count gate, not proof the shape satisfies the policy.
	composite := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "t"},
			Columns: []spec.Column{
				{Name: "id", Type: "bigint", NotNull: true},
				{Name: "tenant_id", Type: "int", NotNull: true},
			},
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id", "tenant_id"}},
		},
	}
	active := t06A10PKRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if !active.AppliesTo(composite) {
		t.Fatal("rule must still apply to a create-table with a primary key")
	}
	findings, err = active.Evaluate(ctx, composite)
	if err != nil || len(findings) != 0 {
		t.Fatalf("composite findings = %v, err = %v; want none — skipped, not satisfied", findings, err)
	}
}

// TestT06A10InitValueEqualityBoundaries pins value construction bounds and
// the exact-equality policy: only the declared value equal to `value` passes.
func TestT06A10InitValueEqualityBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Construction bounds: value below 1 must fail even when non-positive.
	for _, bad := range []int{0, -1} {
		if _, err := newTableAutoIncrementInitValueRule(policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"value": bad},
		}); err == nil {
			t.Fatalf("value=%d must fail construction", bad)
		}
	}

	// Default level is blocker when the policy carries no level.
	defaulted, err := newTableAutoIncrementInitValueRule(policy.RulePolicy{
		Enabled: true, Params: map[string]any{"value": 8},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := defaulted.Evaluate(ctx, tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["auto_increment"] = "9"
	}))
	if err != nil || len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
		t.Fatalf("findings = %v, err = %v; want one default-blocker finding", findings, err)
	}

	// value=8 exact equality: 7 and 9 both reject, 8 passes, omission stays silent.
	rule8, err := newTableAutoIncrementInitValueRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"value": 8},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	for _, tc := range []struct {
		declared string
		want     int
	}{
		{declared: "7", want: 1},
		{declared: "8", want: 0},
		{declared: "9", want: 1},
		{declared: "", want: 0}, // key absent: silent — never defaulted to 8
	} {
		declared := tc.declared
		findings, err = rule8.Evaluate(ctx, tableOptionStatement(func(ddl *spec.DDL) {
			if declared != "" {
				ddl.Options["auto_increment"] = declared
			}
		}))
		if err != nil {
			t.Fatalf("evaluate declared=%q: %v", declared, err)
		}
		if len(findings) != tc.want {
			t.Fatalf("declared=%q findings = %v, want %d", declared, findings, tc.want)
		}
		if tc.want == 1 && findings[0].Metadata["required_value"] != 8 {
			t.Fatalf("declared=%q metadata = %v, want required_value=8", declared, findings[0].Metadata)
		}
	}
}
