// Package ddl verifies the T06-A14 three ordinary CREATE TABLE
// table-option allowlist policies on synthetic statements.
// input: the frozen allowed/denied/omitted option sets through each rule's
// registered constructor and through the real registry seam
// output: per-role findings pinned for the six-role matrix (allowed /
// denied / omitted-strict / omitted-optional / denied-optional / disabled),
// the require_explicit semantics, default level/params, invalid-parameter
// controls, the case-insensitive comparison, and the CREATE-only/PG
// applicability seams
// pos: domain DDL rule regression for issue #85 T06-A14
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06A14Families maps each frozen family key to its rule, options key,
// message label, and narrowed allowlist.
func t06A14Families() []struct {
	key     string
	ruleID  string
	option  string
	label   string
	allowed []string
} {
	return []struct {
		key     string
		ruleID  string
		option  string
		label   string
		allowed []string
	}{
		{"engine", ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}},
		{"charset", ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8mb4"}},
		{"row-format", ruleIDTableRowFormatAllowlist, "row_format", "row format", []string{"DYNAMIC"}},
	}
}

func t06A14Statement(dialect spec.Dialect, options map[string]string) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Options:   options,
		},
	}
}

func t06A14Params(allowed []string, requireExplicit bool) map[string]any {
	values := make([]any, 0, len(allowed))
	for _, v := range allowed {
		values = append(values, v)
	}
	return map[string]any{"values": values, "require_explicit": requireExplicit}
}

// t06A14Registry builds a real registry carrying exactly one family rule at
// blocker with the frozen isolated params.
func t06A14Registry(t *testing.T, ruleID string, params map[string]any) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
		ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: params},
	}}); err != nil {
		t.Fatalf("register %s: %v", ruleID, err)
	}
	if registry.LoadedStatementRuleCount() != 1 {
		t.Fatalf("loaded rules = %d, want 1", registry.LoadedStatementRuleCount())
	}
	return registry
}

// TestT06A14RoleMatrix pins the six-role contract per family: allowed and
// omitted-optional pass; denied (strict or optional) and omitted-strict
// produce exactly one blocker with the complete frozen identity.
func TestT06A14RoleMatrix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	denied := map[string]string{"engine": "MyISAM", "charset": "latin1", "row_format": "COMPACT"}
	allowedVal := map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": "DYNAMIC"}
	for _, family := range t06A14Families() {
		family := family
		strict := t06A14Registry(t, family.ruleID, t06A14Params(family.allowed, true))
		optional := t06A14Registry(t, family.ruleID, t06A14Params(family.allowed, false))

		checkFinding := func(t *testing.T, findings []rule.Finding, actual string) {
			t.Helper()
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly 1", findings)
			}
			finding := findings[0]
			wantMessage := "table " + family.label + " must be one of [" + family.allowed[0] + "]"
			if finding.RuleID != family.ruleID || finding.Level != rule.LevelBlocker ||
				finding.Message != wantMessage {
				t.Fatalf("finding = %+v, want %s blocker %q", finding, family.ruleID, wantMessage)
			}
			if finding.Metadata["table"] != "t" || finding.Metadata["option"] != family.option ||
				finding.Metadata["actual"] != actual {
				t.Fatalf("metadata = %+v, want table=t option=%s actual=%q",
					finding.Metadata, family.option, actual)
			}
			gotAllowed, ok := finding.Metadata["allowed"].([]string)
			if !ok || len(gotAllowed) != 1 || gotAllowed[0] != family.allowed[0] {
				t.Fatalf("metadata.allowed = %+v, want [%s]", finding.Metadata["allowed"], family.allowed[0])
			}
		}

		t.Run(family.key+"/allowed-strict", func(t *testing.T) {
			findings, err := strict.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL,
				map[string]string{family.option: allowedVal[family.option]}))
			if err != nil || len(findings) != 0 {
				t.Fatalf("findings = %+v err=%v, want none", findings, err)
			}
		})
		t.Run(family.key+"/denied-strict", func(t *testing.T) {
			findings, err := strict.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL,
				map[string]string{family.option: denied[family.option]}))
			if err != nil {
				t.Fatal(err)
			}
			checkFinding(t, findings, denied[family.option])
		})
		t.Run(family.key+"/omitted-strict", func(t *testing.T) {
			findings, err := strict.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL, map[string]string{}))
			if err != nil {
				t.Fatal(err)
			}
			checkFinding(t, findings, "")
		})
		t.Run(family.key+"/omitted-optional", func(t *testing.T) {
			findings, err := optional.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL, map[string]string{}))
			if err != nil || len(findings) != 0 {
				t.Fatalf("findings = %+v err=%v, want none — require_explicit=false exempts omission only",
					findings, err)
			}
		})
		t.Run(family.key+"/denied-optional", func(t *testing.T) {
			findings, err := optional.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL,
				map[string]string{family.option: denied[family.option]}))
			if err != nil {
				t.Fatal(err)
			}
			checkFinding(t, findings, denied[family.option])
		})
	}
}

// TestT06A14DisabledNeverLoads pins that enabled=false removes the rule
// from the registry entirely — the absent rule_summary role, distinct from
// any applicable-but-clean outcome.
func TestT06A14DisabledNeverLoads(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
		ruleIDTableEngineAllowlist: {Enabled: false},
	}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if registry.LoadedStatementRuleCount() != 0 {
		t.Fatalf("loaded rules = %d, want 0 for enabled=false", registry.LoadedStatementRuleCount())
	}
}

// TestT06A14DefaultsAndParams pins the shipped fallbacks and the real
// parameter boundaries: omitted values/require_explicit fall back per rule;
// a non-string values element and a non-bool require_explicit are
// constructor errors, never silent coercions.
func TestT06A14DefaultsAndParams(t *testing.T) {
	t.Parallel()
	fallbacks := map[string]struct {
		ruleID    string
		optionKey string
		label     string
		values    []string
	}{
		"engine":     {ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}},
		"charset":    {ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8", "utf8mb4"}},
		"row-format": {ruleIDTableRowFormatAllowlist, "row_format", "row format", []string{"DYNAMIC"}},
	}
	for name, family := range fallbacks {
		family := family
		t.Run(name+"/fallback", func(t *testing.T) {
			t.Parallel()
			constructed, err := newTableOptionAllowlistRule(family.ruleID, family.optionKey,
				family.label, family.values, rule.LevelBlocker, policy.RulePolicy{
					Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{},
				})
			if err != nil {
				t.Fatalf("construct: %v", err)
			}
			typed, ok := constructed.(tableOptionAllowlistRule)
			if !ok {
				t.Fatalf("rule type = %T", constructed)
			}
			if len(typed.allowed) != len(family.values) {
				t.Fatalf("allowed = %#v, want %#v", typed.allowed, family.values)
			}
			for i, v := range family.values {
				if typed.allowed[i] != v {
					t.Fatalf("allowed[%d] = %q, want %q", i, typed.allowed[i], v)
				}
			}
			if !typed.requireExplicit {
				t.Fatalf("requireExplicit = false, want fallback true")
			}
		})
	}
	t.Run("non-string values element rejected", func(t *testing.T) {
		t.Parallel()
		registry := rule.NewRegistry()
		err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
			ruleIDTableEngineAllowlist: {Enabled: true, Level: rule.LevelBlocker,
				Params: map[string]any{"values": []any{"InnoDB", 1}}},
		}})
		if err == nil {
			t.Fatal("expected constructor error for non-string values element")
		}
	})
	t.Run("scalar values rejected", func(t *testing.T) {
		t.Parallel()
		registry := rule.NewRegistry()
		err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
			ruleIDTableCharsetAllowlist: {Enabled: true, Level: rule.LevelBlocker,
				Params: map[string]any{"values": "utf8mb4"}},
		}})
		if err == nil {
			t.Fatal("expected constructor error for scalar values")
		}
	})
	t.Run("non-bool require_explicit rejected", func(t *testing.T) {
		t.Parallel()
		registry := rule.NewRegistry()
		err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
			ruleIDTableRowFormatAllowlist: {Enabled: true, Level: rule.LevelBlocker,
				Params: map[string]any{"values": []any{"DYNAMIC"}, "require_explicit": "yes"}},
		}})
		if err == nil {
			t.Fatal("expected constructor error for non-bool require_explicit")
		}
	})
}

// TestT06A14CaseInsensitiveMembership pins the real containsFold seam:
// declaration case stays verbatim in Options/actual while membership is
// case-insensitive — `innodb` still satisfies an `InnoDB` allowlist and a
// mismatched value is never laundered.
func TestT06A14CaseInsensitiveMembership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := t06A14Registry(t, ruleIDTableEngineAllowlist,
		t06A14Params([]string{"InnoDB"}, true))
	for _, actual := range []string{"innodb", "INNODB", " InnoDB "} {
		findings, err := engine.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL,
			map[string]string{"engine": actual}))
		if err != nil || len(findings) != 0 {
			t.Fatalf("actual=%q findings = %+v err=%v, want none — membership is fold-compared", actual, findings, err)
		}
	}
	findings, err := engine.EvaluateStatement(ctx, t06A14Statement(spec.DialectMySQL,
		map[string]string{"engine": "myisam"}))
	if err != nil || len(findings) != 1 {
		t.Fatalf("actual=myisam findings = %+v err=%v, want 1 blocker", findings, err)
	}
	if findings[0].Metadata["actual"] != "myisam" {
		t.Fatalf("actual recorded = %q, want the declared text verbatim", findings[0].Metadata["actual"])
	}
}

// TestT06A14ApplicabilityBoundaries pins the CREATE-only scope and the PG
// gate through the real rule instances: ALTER statements never apply, and
// all three rules skip the PostgreSQL dialect.
func TestT06A14ApplicabilityBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, family := range t06A14Families() {
		family := family
		registry := t06A14Registry(t, family.ruleID, t06A14Params(family.allowed, true))
		t.Run(family.key+"/alter", func(t *testing.T) {
			alter := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectMySQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "t"},
					Alter:     []spec.Alter{{Action: "add_column"}},
				},
			}
			findings, err := registry.EvaluateStatement(ctx, alter)
			if err != nil || len(findings) != 0 {
				t.Fatalf("evaluate on ALTER = %+v err=%v, want none", findings, err)
			}
		})
		t.Run(family.key+"/postgresql", func(t *testing.T) {
			findings, err := registry.EvaluateStatement(ctx, t06A14Statement(spec.DialectPostgreSQL,
				map[string]string{family.option: "BOGUS"}))
			if err != nil || len(findings) != 0 {
				t.Fatalf("evaluate on PG = %+v err=%v, want none — the four table allowlists skip PostgreSQL",
					findings, err)
			}
		})
	}
}
