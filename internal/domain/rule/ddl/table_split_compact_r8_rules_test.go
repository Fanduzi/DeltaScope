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
	lit := []spec.SplitValueClass{spec.SplitValueLiteral}
	between := func(lower, upper int, num int64, lc, uc []spec.SplitValueClass) *spec.SplitRegionDecl {
		return &spec.SplitRegionDecl{
			Bound: spec.SplitBound{LowerArity: lower, UpperArity: upper, LowerClasses: lc, UpperClasses: uc},
			Num:   &num,
		}
	}
	snap := &spec.TableSnapshot{Options: map[string]string{}}
	cases := []struct {
		name       string
		decl       *spec.SplitRegionDecl
		snapshot   *spec.TableSnapshot
		wantReason string
		wantNone   bool
	}{
		{"temporary_refused", between(1, 1, 4, lit, lit), &spec.TableSnapshot{Temporary: true}, spec.LayoutReasonSplitTemporary, false},
		{"arity_refused", between(2, 2, 4, []spec.SplitValueClass{lit[0], lit[0]}, []spec.SplitValueClass{lit[0], lit[0]}), snap, spec.LayoutReasonSplitArityTable, false},
		{"null_refused", between(1, 1, 4, []spec.SplitValueClass{spec.SplitValueNull}, lit), snap, spec.LayoutReasonSplitNullValue, false},
		{"over_limit_refused", between(1, 1, 1001, lit, lit), snap, spec.LayoutReasonSplitNumOverLimit, false},
		{"clustered_primary_refused", func() *spec.SplitRegionDecl {
			d := between(1, 1, 4, lit, lit)
			d.IndexName = "primary"
			return d
		}(), &spec.TableSnapshot{PrimaryKeyClustered: true, PrimaryKey: &spec.Index{Columns: []string{"a"}}, Columns: []spec.Column{{Name: "a", Type: "int"}}}, spec.LayoutReasonSplitPrimaryClustered, false},
		{"missing_index_refused", func() *spec.SplitRegionDecl {
			d := between(1, 1, 4, lit, lit)
			d.IndexName = "ghost"
			return d
		}(), snap, spec.LayoutReasonSplitIndexMissing, false},
		{"accepted_silent", between(1, 1, 4, lit, lit), snap, "", true},
		{"undecidable_silent", between(1, 1, 4, lit, lit), nil, "", true},
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
	lit := []spec.SplitValueClass{spec.SplitValueLiteral}
	num := int64(4)
	cases := []struct {
		name     string
		decl     *spec.SplitRegionDecl
		snapshot *spec.TableSnapshot
		instance *spec.InstanceFacts
		wantFact string
	}{
		{"offline_owes_handle", &spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num}, nil, nil, "table.handle_columns"},
		{"index_owes_members", &spec.SplitRegionDecl{IndexName: "ix", Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num}, &spec.TableSnapshot{IndexesUnknown: true}, nil, "table.index_members"},
		{"between_owes_limit", &spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num}, &spec.TableSnapshot{}, nil, "instance.split_region_max_num"},
		{"expr_owes_foldability", &spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: []spec.SplitValueClass{spec.SplitValueExpr}, UpperClasses: lit}, Num: &num}, &spec.TableSnapshot{}, t23r8Instance(), "statement.split_value_foldability"},
		{"partition_clause_owes_partitioned", &spec.SplitRegionDecl{HasPartitionClause: true, Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num}, &spec.TableSnapshot{}, t23r8Instance(), "table.partitioned"},
		{"view_owes_no_fact", &spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num}, &spec.TableSnapshot{RelationKind: "view"}, t23r8Instance(), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gaps := reporter.EvidenceGaps(t23r8SplitStatement(tc.decl, tc.snapshot, tc.instance))
			if len(gaps) != 1 {
				t.Fatalf("want one gap, got %+v", gaps)
			}
			if gaps[0].ReasonCode != gapReasonMissingInstanceFact {
				t.Fatalf("reason_code %q", gaps[0].ReasonCode)
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
		&spec.SplitRegionDecl{Bound: spec.SplitBound{LowerArity: 1, UpperArity: 1, LowerClasses: lit, UpperClasses: lit}, Num: &num},
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
