// Package ddl verifies the T23-R8 SPLIT/COMPACT rule surface: refused
// declarations emit findings, unproven premises emit named fact gaps,
// and accepted/no-op arms stay quiet or warn (issue #102).
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t23r8SplitStatement(decl *spec.SplitRegionDecl, snapshot *spec.TableSnapshot, instance *spec.InstanceFacts) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation:   spec.DDLOperationSplitRegion,
			Table:       &spec.Table{Name: "t"},
			SplitRegion: decl,
		},
	}
	if snapshot != nil || instance != nil {
		statement.Metadata = &spec.Metadata{TargetTable: snapshot, Instance: instance}
	}
	return statement
}

func t23r8CompactStatement(decl *spec.CompactTableDecl, snapshot *spec.TableSnapshot, instance *spec.InstanceFacts) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation:    spec.DDLOperationCompactTable,
			Table:        &spec.Table{Name: "t"},
			CompactTable: decl,
		},
	}
	if snapshot != nil || instance != nil {
		statement.Metadata = &spec.Metadata{TargetTable: snapshot, Instance: instance}
	}
	return statement
}

func t23r8Instance() *spec.InstanceFacts {
	return &spec.InstanceFacts{
		Version:                    "8.0.11-TiDB-v8.5.0",
		TiDBSplitRegionMaxNum:      1000,
		TiDBSplitRegionMaxNumKnown: true,
	}
}

func TestT23R8SplitFindings(t *testing.T) {
	ruleObj, err := newTableSplitRegionValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	lit := []spec.SplitBoundValue{{Class: spec.SplitValueLiteral}}
	between := func(lower, upper int, num int64, lc, uc []spec.SplitBoundValue) *spec.SplitRegionDecl {
		return &spec.SplitRegionDecl{
			Bound: spec.SplitBound{LowerArity: lower, UpperArity: upper, Lower: lc, Upper: uc},
			Num:   &num,
		}
	}
	intBetween := func(lo, hi, num int64) *spec.SplitRegionDecl {
		return between(1, 1, num,
			[]spec.SplitBoundValue{{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: lo}},
			[]spec.SplitBoundValue{{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: hi}})
	}
	snap := &spec.TableSnapshot{Options: map[string]string{}}
	cases := []struct {
		name       string
		decl       *spec.SplitRegionDecl
		snapshot   *spec.TableSnapshot
		wantReason string
		wantNone   bool
	}{
		{"temporary_refused", intBetween(0, 10000, 4), &spec.TableSnapshot{Temporary: true}, spec.LayoutReasonSplitTemporary, false},
		{"arity_refused", between(2, 2, 4, []spec.SplitBoundValue{lit[0], lit[0]}, []spec.SplitBoundValue{lit[0], lit[0]}), snap, spec.LayoutReasonSplitArityTable, false},
		{"null_refused", between(1, 1, 4, []spec.SplitBoundValue{{Class: spec.SplitValueNull}}, lit), snap, spec.LayoutReasonSplitNullValue, false},
		{"over_limit_refused", intBetween(0, 10000, 1001), snap, spec.LayoutReasonSplitNumOverLimit, false},
		{"bounds_reversed_refused", intBetween(100, 0, 4), snap, spec.LayoutReasonSplitBoundNotIncreasing, false},
		{"step_too_small_refused", intBetween(0, 100, 4), snap, spec.LayoutReasonSplitStepTooSmall, false},
		{"clustered_primary_refused", func() *spec.SplitRegionDecl {
			d := intBetween(0, 10000, 4)
			d.IndexName = "primary"
			return d
		}(), &spec.TableSnapshot{PrimaryKeyClustered: true, PrimaryKey: &spec.Index{Columns: []string{"a"}}, Columns: []spec.Column{{Name: "a", Type: "int"}}}, spec.LayoutReasonSplitPrimaryClustered, false},
		{"missing_index_refused", func() *spec.SplitRegionDecl {
			d := intBetween(0, 10000, 4)
			d.IndexName = "ghost"
			return d
		}(), snap, spec.LayoutReasonSplitIndexMissing, false},
		{"accepted_silent", intBetween(0, 10000, 4), snap, "", true},
		{"undecidable_silent", intBetween(0, 10000, 4), nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := ruleObj.Evaluate(context.Background(),
				t23r8SplitStatement(tc.decl, tc.snapshot, t23r8Instance()))
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantNone {
				if len(findings) != 0 {
					t.Fatalf("want no finding, got %+v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("want one refusal finding, got %+v", findings)
			}
			if findings[0].Metadata["reason"] != tc.wantReason {
				t.Fatalf("reason %v want %q", findings[0].Metadata["reason"], tc.wantReason)
			}
			if findings[0].Level != rule.LevelBlocker {
				t.Fatalf("level %v want blocker", findings[0].Level)
			}
		})
	}
}

func TestT23R8SplitGaps(t *testing.T) {
	ruleObj, err := newTableSplitRegionValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	reporter := ruleObj.(rule.EvidenceReporter)
	lit := []spec.SplitBoundValue{{Class: spec.SplitValueLiteral}}
	num := int64(4)
	intBound := func(v int64) []spec.SplitBoundValue {
		return []spec.SplitBoundValue{{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: v}}
	}
	betweenDecl := func(lc, uc []spec.SplitBoundValue) *spec.SplitRegionDecl {
		return &spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, Lower: lc, Upper: uc}, Num: &num}
	}
	cases := []struct {
		name       string
		decl       *spec.SplitRegionDecl
		snapshot   *spec.TableSnapshot
		instance   *spec.InstanceFacts
		wantReason string
		wantFact   string
	}{
		{"offline_owes_handle", betweenDecl(lit, lit), nil, nil, gapReasonMissingInstanceFact, "table.handle_columns"},
		{"index_owes_members", &spec.SplitRegionDecl{IndexName: "ix", Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, Lower: lit, Upper: lit}, Num: &num}, &spec.TableSnapshot{IndexesUnknown: true}, nil, gapReasonMissingInstanceFact, "table.index_members"},
		{"between_owes_limit", betweenDecl(lit, lit), &spec.TableSnapshot{}, nil, gapReasonMissingInstanceFact, "instance.split_region_max_num"},
		{"expr_owes_foldability", betweenDecl([]spec.SplitBoundValue{{Class: spec.SplitValueExpr}}, lit), &spec.TableSnapshot{}, t23r8Instance(), gapReasonUnprovenStatementFact, "statement.split_value_foldability"},
		{"index_compare_owes_relation", func() *spec.SplitRegionDecl {
			d := betweenDecl(intBound(0), intBound(10000))
			d.IndexName = "ix"
			return d
		}(), &spec.TableSnapshot{Indexes: []spec.Index{{Name: "ix", Columns: []string{"a"}}}, Columns: []spec.Column{{Name: "a", Type: "int"}}}, t23r8Instance(), gapReasonUnprovenStatementFact, "statement.split_bound_relation"},
		{"convert_owes_statement_fact", betweenDecl([]spec.SplitBoundValue{{Class: spec.SplitValueLiteral}}, intBound(10000)), &spec.TableSnapshot{}, t23r8Instance(), gapReasonUnprovenStatementFact, "statement.split_bound_conversion"},
		{"partition_clause_owes_partitioned", &spec.SplitRegionDecl{HasPartitionClause: true, Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, Lower: lit, Upper: lit}, Num: &num}, &spec.TableSnapshot{}, t23r8Instance(), gapReasonMissingInstanceFact, "table.partitioned"},
		{"view_owes_no_fact", betweenDecl(lit, lit), &spec.TableSnapshot{RelationKind: "view"}, t23r8Instance(), gapReasonMissingInstanceFact, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gaps := reporter.EvidenceGaps(t23r8SplitStatement(tc.decl, tc.snapshot, tc.instance))
			if len(gaps) != 1 {
				t.Fatalf("want one gap, got %+v", gaps)
			}
			if gaps[0].ReasonCode != tc.wantReason {
				t.Fatalf("reason_code %q want %q", gaps[0].ReasonCode, tc.wantReason)
			}
			if tc.wantFact == "" {
				if len(gaps[0].RequiredFacts) != 0 {
					t.Fatalf("bounded boundary must not name a fact: %+v", gaps[0])
				}
				return
			}
			if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.wantFact {
				t.Fatalf("required_facts %+v want [%q]", gaps[0].RequiredFacts, tc.wantFact)
			}
		})
	}
	// A proven refusal owes no gap.
	snap := &spec.TableSnapshot{Temporary: true}
	gaps := reporter.EvidenceGaps(t23r8SplitStatement(
		betweenDecl(intBound(0), intBound(10000)),
		snap, t23r8Instance()))
	if len(gaps) != 0 {
		t.Fatalf("proven refusal must not gap, got %+v", gaps)
	}
}

func TestT23R8CompactFindings(t *testing.T) {
	ruleObj, err := newTableCompactValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	cases := []struct {
		name       string
		decl       *spec.CompactTableDecl
		snapshot   *spec.TableSnapshot
		wantReason string
		wantLevel  rule.Level
		wantNone   bool
	}{
		{"accepted_silent", &spec.CompactTableDecl{ReplicaKind: "tiflash"}, &spec.TableSnapshot{Options: map[string]string{"tiflash_replica_count": "1"}}, "", rule.LevelBlocker, true},
		{"no_replica_warn_noop", &spec.CompactTableDecl{ReplicaKind: "tiflash"}, &spec.TableSnapshot{Options: map[string]string{"tiflash_replica_count": "0"}}, spec.LayoutReasonCompactNoReplicaWarn, rule.LevelWarning, false},
		{"undecidable_silent", &spec.CompactTableDecl{ReplicaKind: "tiflash"}, nil, "", rule.LevelBlocker, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := ruleObj.Evaluate(context.Background(),
				t23r8CompactStatement(tc.decl, tc.snapshot, t23r8Instance()))
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantNone {
				if len(findings) != 0 {
					t.Fatalf("want no finding, got %+v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("want one finding, got %+v", findings)
			}
			if findings[0].Metadata["reason"] != tc.wantReason {
				t.Fatalf("reason %v want %q", findings[0].Metadata["reason"], tc.wantReason)
			}
			if findings[0].Level != tc.wantLevel {
				t.Fatalf("level %v want %v", findings[0].Level, tc.wantLevel)
			}
		})
	}
	// Replica-kind refusals are static — no instance facts needed.
	for _, kind := range []string{"tikv", "rocksdb"} {
		findings, err := ruleObj.Evaluate(context.Background(),
			t23r8CompactStatement(&spec.CompactTableDecl{ReplicaKind: kind}, &spec.TableSnapshot{}, t23r8Instance()))
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason"] != spec.LayoutReasonCompactReplicaKind {
			t.Fatalf("kind %q: want replica-kind refusal, got %+v", kind, findings)
		}
	}
}

func TestT23R8CompactGaps(t *testing.T) {
	ruleObj, err := newTableCompactValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	reporter := ruleObj.(rule.EvidenceReporter)
	cases := []struct {
		name     string
		decl     *spec.CompactTableDecl
		snapshot *spec.TableSnapshot
		instance *spec.InstanceFacts
		wantFact string
	}{
		{"offline_owes_store", &spec.CompactTableDecl{ReplicaKind: "tiflash"}, nil, nil, "instance.tikv_compatible_storage"},
		{"names_owe_partitioned", &spec.CompactTableDecl{ReplicaKind: "tiflash", PartitionNames: []string{"p0"}}, &spec.TableSnapshot{}, t23r8Instance(), "table.partitioned"},
		{"names_owe_members", &spec.CompactTableDecl{ReplicaKind: "tiflash", PartitionNames: []string{"p0"}}, &spec.TableSnapshot{Partitioned: t13r1Bool(true)}, t23r8Instance(), "table.partition_names"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gaps := reporter.EvidenceGaps(t23r8CompactStatement(tc.decl, tc.snapshot, tc.instance))
			if len(gaps) != 1 {
				t.Fatalf("want one gap, got %+v", gaps)
			}
			if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.wantFact {
				t.Fatalf("required_facts %+v want [%q]", gaps[0].RequiredFacts, tc.wantFact)
			}
		})
	}
	// Replica-kind refusal is static — no gap.
	gaps := reporter.EvidenceGaps(t23r8CompactStatement(
		&spec.CompactTableDecl{ReplicaKind: "tikv"}, &spec.TableSnapshot{}, t23r8Instance()))
	if len(gaps) != 0 {
		t.Fatalf("static refusal must not gap, got %+v", gaps)
	}
}

func TestT23R8RulesSkipMySQL(t *testing.T) {
	splitRuleObj, _ := newTableSplitRegionValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	compactRuleObj, _ := newTableCompactValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	statement := t23r8SplitStatement(&spec.SplitRegionDecl{}, nil, nil)
	statement.Dialect = spec.DialectMySQL
	if splitRuleObj.AppliesTo(statement) {
		t.Fatal("split rule must not apply under mysql dialect")
	}
	compact := t23r8CompactStatement(&spec.CompactTableDecl{ReplicaKind: "tiflash"}, nil, nil)
	compact.Dialect = spec.DialectMySQL
	if compactRuleObj.AppliesTo(compact) {
		t.Fatal("compact rule must not apply under mysql dialect")
	}
}
