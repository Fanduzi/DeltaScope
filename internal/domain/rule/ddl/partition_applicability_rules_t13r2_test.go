// Package ddl verifies the T13-R2 applicability remediations: the
// counterpart comparison consumes effective column types rather than raw
// declaration sentinels, every enabled target rule owns its own missing
// premises, Evaluate and EvidenceGaps share one comparison premise, and
// the bounded engine premise follows the probed per-dialect matrix.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t13r2CounterpartRule(t *testing.T) rule.StatementRule {
	t.Helper()
	r, err := newExchangeCounterpartCompatibilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

// B1: `CHAR` and `CHAR(1)` are the same effective type — the parser's -1
// declaration sentinel must not masquerade as a definition difference.
// The catalog side reports `char(1)` either way (probed: a bare CHAR
// materializes char(1) in information_schema.columns on every anchor).
func TestT13R2CounterpartComparesEffectiveTypes(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	primary := t13r1Snapshot("t")
	primary.Columns = []spec.Column{
		{Name: "id", Type: "int", Length: -1},
		{Name: "c", Type: "char", Length: -1},
	}
	counterpart := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t2"},
		Columns: []spec.Column{
			{Name: "id", Type: "int", Length: 11}, // catalog int(11)
			{Name: "c", Type: "char", Length: 1},
		},
		PrimaryKey:  primary.PrimaryKey,
		Partitioned: t13r1Bool(false),
	}
	stmt := t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, primary, counterpart)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("effective-equal definitions must not fabricate 1736, got %#v", findings)
	}
}

// B1 control: a real effective-type difference still blocks with 1736.
func TestT13R2CounterpartRealTypeMismatchBlocks(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	counterpart := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t2"},
		Columns: []spec.Column{
			{Name: "id", Type: "int", Length: 11},
			{Name: "c", Type: "char", Length: 4},
		},
		PrimaryKey:  t13r1Snapshot("t").PrimaryKey,
		Partitioned: t13r1Bool(false),
	}
	stmt := t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, t13r1Snapshot("t"), counterpart)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" ||
		findings[0].Metadata["native_error"] != "1736" {
		t.Fatalf("real type mismatch must block once with 1736, got %#v", findings)
	}
}

// B3a: a proven-absent primary key is not reopened by an unvouched
// CHECK/FK constraint set — absence vs presence is a proven 1736.
func TestT13R2CounterpartPKAbsenceHonored(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	primary := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t"},
		Columns:            []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
		PrimaryKey:         nil,
		PrimaryKeyUnknown:  false,
		ConstraintsUnknown: true, // unrelated CHECK/FK sets are unvouched
		Partitioned:        t13r1Bool(true),
		PartitionType:      t13r1Str("range"),
		PartitionNames:     []string{"p0", "p1"},
	}
	counterpart := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t2"},
		Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
		PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		Partitioned: t13r1Bool(false),
	}
	stmt := t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, primary, counterpart)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" {
		t.Fatalf("proven PK absence vs proven PK must block with 1736, got %#v", findings)
	}
}

// B3b: a genuinely unproven PK premise is owed as that side's gap — and a
// proven column-count difference still blocks independently.
func TestT13R2CounterpartSharedComparePremise(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	t.Run("unproven pk owes its own gap", func(t *testing.T) {
		t.Parallel()
		primary := t13r1Snapshot("t")
		primary.PrimaryKey = nil
		primary.PrimaryKeyUnknown = true
		counterpart := &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Name: "t2"},
			Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			Partitioned: t13r1Bool(false),
		}
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, primary, counterpart)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("unproven PK must not fabricate a refusal, got %#v", findings)
		}
		gaps := t13r1Gaps(r, stmt)
		found := false
		for _, gap := range gaps {
			for _, fact := range gap.RequiredFacts {
				found = found || fact == "target_table.primary_key"
			}
		}
		if !found {
			t.Fatalf("unproven PK owes target_table.primary_key, got %+v", gaps)
		}
	})
	t.Run("column count blocks while pk unproven", func(t *testing.T) {
		t.Parallel()
		primary := t13r1Snapshot("t")
		primary.PrimaryKey = nil
		primary.PrimaryKeyUnknown = true
		counterpart := &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Name: "t2"},
			Columns: []spec.Column{
				{Name: "id", Type: "int"}, {Name: "c", Type: "int"}, {Name: "x", Type: "int"},
			},
			Partitioned: t13r1Bool(false),
		}
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, primary, counterpart)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" {
			t.Fatalf("proven column-count difference must block independently, got %#v", findings)
		}
		gaps := t13r1Gaps(r, stmt)
		found := false
		for _, gap := range gaps {
			for _, fact := range gap.RequiredFacts {
				found = found || fact == "target_table.primary_key"
			}
		}
		if !found {
			t.Fatalf("the unproven PK premise is still owed alongside the finding, got %+v", gaps)
		}
	})
}

// B2: an enabled target rule owns its own missing premises — it never
// borrows the applicability rule's job. Only the facts the rule consumes
// are owed.
func TestT13R2TargetRuleOwnsItsPremises(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"drop_partition", "truncate_partition", "exchange_partition"} {
		action := action
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			r, err := newPartitionTargetExistsRule(action, t13r1Config())
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			for _, tc := range []struct {
				name     string
				shape    *spec.TableSnapshot
				wantFact string
			}{
				{name: "table unobserved", shape: nil, wantFact: "target_table.partitioned"},
				{name: "partitioned unobserved", shape: func() *spec.TableSnapshot {
					return &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t"}}
				}(), wantFact: "target_table.partitioned"},
				{name: "directory unobserved", shape: func() *spec.TableSnapshot {
					return &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t"},
						Partitioned: t13r1Bool(true)}
				}(), wantFact: "target_table.partitions"},
			} {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					stmt := t13r1Statement(action, spec.Alter{
						Action: action, Partitions: []string{"p0"},
					}, tc.shape)
					gaps := t13r1Gaps(r, stmt)
					if len(gaps) != 1 {
						t.Fatalf("%s/%s owes exactly one gap, got %+v", action, tc.name, gaps)
					}
					if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.wantFact {
						t.Fatalf("%s/%s must name %s alone, got %+v", action, tc.name, tc.wantFact, gaps[0].RequiredFacts)
					}
				})
			}
		})
	}
}

// B2 control: a proven-absent or proven-non-partitioned primary is a
// finding path for the family, not a gap path for the target rule; ALL
// consumes no directory at any state.
func TestT13R2TargetRuleProvenStatesOweNothing(t *testing.T) {
	t.Parallel()
	r, err := newPartitionTargetExistsRule("drop_partition", t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	for _, tc := range []struct {
		name  string
		shape *spec.TableSnapshot
	}{
		{name: "absent", shape: &spec.TableSnapshot{Exists: false, Table: &spec.Table{Name: "t"}}},
		{name: "non-partitioned", shape: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t"},
			Partitioned: t13r1Bool(false), PartitionNames: []string{}}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stmt := t13r1Statement("drop_partition", spec.Alter{
				Action: "drop_partition", Partitions: []string{"p0"},
			}, tc.shape)
			if gaps := t13r1Gaps(r, stmt); len(gaps) != 0 {
				t.Fatalf("%s owes no target-rule gap, got %+v", tc.name, gaps)
			}
		})
	}
	truncR, err := newPartitionTargetExistsRule("truncate_partition", t13r1Config())
	if err != nil {
		t.Fatalf("truncate rule: %v", err)
	}
	allStmt := t13r1Statement("truncate_partition", spec.Alter{
		Action: "truncate_partition", AllPartitions: true,
	}, nil)
	if gaps := t13r1Gaps(truncR, allStmt); len(gaps) != 0 {
		t.Fatalf("TRUNCATE ALL owes nothing even unobserved, got %+v", gaps)
	}
}

// B4: on MySQL an engine-mismatched counterpart is a proven refusal
// (1497 — probed 5.7/8.0/8.4); equal engines satisfy the premise; an
// unobserved side owes its own engine fact. TiDB normalizes the ENGINE
// label to InnoDB in the catalog (probed) — the premise is never required
// there and no gap is owed.
func TestT13R2CounterpartEngineApplicability(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	exchange := func(primary, counterpart *spec.TableSnapshot, dialect spec.Dialect) spec.Statement {
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, primary, counterpart)
		stmt.Dialect = dialect
		return stmt
	}
	compatibleCounterpart := func(engine string, present bool) *spec.TableSnapshot {
		s := &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Name: "t2"},
			Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			PrimaryKey:  t13r1Snapshot("t").PrimaryKey,
			Partitioned: t13r1Bool(false),
			Options:     map[string]string{},
		}
		if present {
			s.Options["engine"] = engine
		}
		return s
	}
	primaryWith := func(engine string, present bool) *spec.TableSnapshot {
		s := t13r1Snapshot("t")
		s.Options = map[string]string{}
		if present {
			s.Options["engine"] = engine
		}
		return s
	}

	t.Run("mysql mismatch blocks 1497", func(t *testing.T) {
		t.Parallel()
		stmt := exchange(primaryWith("InnoDB", true), compatibleCounterpart("MyISAM", true), spec.DialectMySQL)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_engine_mismatch" ||
			findings[0].Metadata["native_error"] != "1497" {
			t.Fatalf("engine mismatch must block once with 1497, got %#v", findings)
		}
	})
	t.Run("mysql equal engines satisfy", func(t *testing.T) {
		t.Parallel()
		stmt := exchange(primaryWith("InnoDB", true), compatibleCounterpart("innodb", true), spec.DialectMySQL)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("equal engines satisfy the premise, got %#v", findings)
		}
	})
	t.Run("mysql unobserved side owes engine fact", func(t *testing.T) {
		t.Parallel()
		stmt := exchange(primaryWith("InnoDB", true), compatibleCounterpart("", false), spec.DialectMySQL)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("unobserved engine must not fabricate a refusal, got %#v", findings)
		}
		gaps := t13r1Gaps(r, stmt)
		found := false
		for _, gap := range gaps {
			for _, fact := range gap.RequiredFacts {
				found = found || fact == "exchange_table.engine"
			}
		}
		if !found {
			t.Fatalf("unobserved counterpart engine owes exchange_table.engine, got %+v", gaps)
		}
	})
	t.Run("tidb never owes or fires the premise", func(t *testing.T) {
		t.Parallel()
		stmt := exchange(primaryWith("", false), compatibleCounterpart("", false), spec.DialectTiDB)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("TiDB normalizes engine labels — no engine finding, got %#v", findings)
		}
		for _, gap := range t13r1Gaps(r, stmt) {
			for _, fact := range gap.RequiredFacts {
				if fact == "target_table.engine" || fact == "exchange_table.engine" {
					t.Fatalf("TiDB never owes the engine premise, got %+v", gap.RequiredFacts)
				}
			}
		}
	})
}
