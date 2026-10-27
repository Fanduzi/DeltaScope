// Package ddl verifies the T13-R3 remediation cells: the EXCHANGE
// comparison reads effective DECIMAL precision/scale rather than declared
// spelling differences (DECIMAL(M) = DECIMAL(M,0), bare DECIMAL =
// DECIMAL(10,0)), and the PK premise separates proven presence from a
// proven member sequence — an unattributed PK object proves presence
// without proving members, and a residual pointer under PrimaryKeyUnknown
// never answers presence at all.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t13r3ExchangeStmt(primaryColumns, counterpartColumns []spec.Column,
	primaryPK, counterpartPK *spec.Index, counterpartConstraints []spec.Constraint) spec.Statement {
	primary := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t"},
		Columns:        primaryColumns,
		PrimaryKey:     primaryPK,
		Partitioned:    t13r1Bool(true),
		PartitionType:  t13r1Str("range"),
		PartitionNames: []string{"p0", "p1"},
		Options:        map[string]string{"engine": "innodb"},
	}
	counterpart := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t2"},
		Columns:     counterpartColumns,
		PrimaryKey:  counterpartPK,
		Partitioned: t13r1Bool(false),
		Constraints: counterpartConstraints,
		Options:     map[string]string{"engine": "innodb"},
	}
	return t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, primary, counterpart)
}

func t13r3PK() *spec.Index {
	return &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
}

// B: DECIMAL effective parameters — declared spellings that normalize to
// the same catalog answer never fabricate 1736.
func TestT13R3CounterpartDecimalEffectiveParams(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	for _, tc := range []struct {
		name      string
		aType     string
		bType     string
		aUnsigned bool
		bUnsigned bool
		want1736  bool
	}{
		{"declared M vs declared M,0", "decimal(12)", "decimal(12,0)", false, false, false},
		{"bare vs explicit 10,0", "decimal", "decimal(10,0)", false, false, false},
		{"bare vs bare", "decimal", "decimal", false, false, false},
		{"explicit M,D equal", "decimal(10,4)", "decimal(10,4)", false, false, false},
		{"unsigned default params", "decimal(12) unsigned", "decimal(12,0) unsigned", true, true, false},
		{"declared vs catalog spelling", "decimal(12)", "decimal(12,0)", false, false, false},
		{"scale differs", "decimal(12,3)", "decimal(12,0)", false, false, true},
		{"precision differs", "decimal(12)", "decimal(11,0)", false, false, true},
		{"bare vs non-default M", "decimal", "decimal(12,0)", false, false, true},
		{"signedness differs", "decimal(12) unsigned", "decimal(12,0)", true, false, true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stmt := t13r3ExchangeStmt(
				[]spec.Column{{Name: "id", Type: "int"}, {Name: "v", Type: tc.aType, Unsigned: tc.aUnsigned}},
				[]spec.Column{{Name: "id", Type: "int"}, {Name: "v", Type: tc.bType, Unsigned: tc.bUnsigned}},
				t13r3PK(), t13r3PK(), []spec.Constraint{})
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if tc.want1736 {
				if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" {
					t.Fatalf("%s: real effective difference must block with 1736, got %#v", tc.name, findings)
				}
				return
			}
			if len(findings) != 0 {
				t.Fatalf("%s: equal effective type must not fabricate 1736, got %#v", tc.name, findings)
			}
		})
	}
}

// B boundary: a faithful-base spelling the bounded normalizer cannot
// equate stays undecidable — the comparison owes the columns facts and
// never answers with a fabricated refusal or a silent pass.
func TestT13R3CounterpartUnnormalizableTypeOwesGap(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	stmt := t13r3ExchangeStmt(
		[]spec.Column{{Name: "id", Type: "int"}, {Name: "v", Type: "decimal(12) zerofill"}},
		[]spec.Column{{Name: "id", Type: "int"}, {Name: "v", Type: "decimal(12,0) zerofill"}},
		t13r3PK(), t13r3PK(), []spec.Constraint{})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("unnormalizable faithful form must not fabricate a refusal, got %#v", findings)
	}
	gaps := t13r1Gaps(r, stmt)
	var columnsFact bool
	for _, gap := range gaps {
		for _, fact := range gap.RequiredFacts {
			columnsFact = columnsFact || fact == "target_table.columns" || fact == "exchange_table.columns"
		}
	}
	if !columnsFact {
		t.Fatalf("undecidable faithful compare owes the columns facts, got %+v", gaps)
	}
}

// C1: PrimaryKeyUnknown outranks the residual pointer — the ordered-state
// unvouch keeps the pointer while presence is unproven. No independent
// difference may escalate it to a 1736; a proven column difference still
// blocks independently.
func TestT13R3PKUnknownOutranksResidualPointer(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	for _, tc := range []struct {
		name     string
		primary  func() *spec.TableSnapshot
		wantFact string
	}{
		{"target side unvouched", func() *spec.TableSnapshot {
			p := t13r1Snapshot("t")
			p.PrimaryKeyUnknown = true // residual {primary,[id]} pointer + flag
			return p
		}, "target_table.primary_key"},
		{"counterpart side unvouched", func() *spec.TableSnapshot {
			p := t13r1Snapshot("t")
			return p
		}, "exchange_table.primary_key"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counterpart := &spec.TableSnapshot{
				Exists: true, Table: &spec.Table{Name: "t2"},
				Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
				Partitioned: t13r1Bool(false),
				Constraints: []spec.Constraint{}, // proven-absent PK
				Options:     map[string]string{"engine": "innodb"},
			}
			if tc.wantFact == "exchange_table.primary_key" {
				counterpart.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, NameUnknown: true, Columns: []string{"id"}}
				counterpart.PrimaryKeyUnknown = true
			}
			stmt := t13r1StatementExchange(spec.Alter{
				Action: "exchange_partition", Partitions: []string{"p0"},
				ExchangeTable: &spec.Table{Name: "t2"},
			}, tc.primary(), counterpart)
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("%s: unproven presence must not fabricate 1736, got %#v", tc.name, findings)
			}
			found := false
			for _, gap := range t13r1Gaps(r, stmt) {
				for _, fact := range gap.RequiredFacts {
					found = found || fact == tc.wantFact
				}
			}
			if !found {
				t.Fatalf("%s: owes %s, got %+v", tc.name, tc.wantFact, t13r1Gaps(r, stmt))
			}
		})
	}
	// An independent proven column-count difference still blocks while the
	// PK premise stays unproven.
	primary := t13r1Snapshot("t")
	primary.PrimaryKeyUnknown = true
	counterpart := &spec.TableSnapshot{
		Exists: true, Table: &spec.Table{Name: "t2"},
		Columns: []spec.Column{
			{Name: "id", Type: "int"}, {Name: "c", Type: "int"}, {Name: "x", Type: "int"},
		},
		Partitioned: t13r1Bool(false),
		Constraints: []spec.Constraint{},
		Options:     map[string]string{"engine": "innodb"},
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
		t.Fatalf("independent column difference must still block, got %#v", findings)
	}
}

// C2: the unattributed TiDB placeholder ({Kind:Primary, NameUnknown:true},
// Columns=nil, PrimaryKeyUnknown=false) proves presence without proving
// the member sequence — never a member difference finding, always the
// owed primary_key fact.
func TestT13R3PKPresenceSeparateFromMemberProof(t *testing.T) {
	t.Parallel()
	r := t13r2CounterpartRule(t)
	placeholder := func() *spec.Index {
		return &spec.Index{Kind: spec.IndexKindPrimary, NameUnknown: true}
	}
	t.Run("placeholder vs member-proven owes target fact", func(t *testing.T) {
		t.Parallel()
		stmt := t13r3ExchangeStmt(
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			placeholder(), t13r3PK(), []spec.Constraint{})
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("unproven member sequence must not fabricate 1736, got %#v", findings)
		}
		found := false
		for _, gap := range t13r1Gaps(r, stmt) {
			for _, fact := range gap.RequiredFacts {
				found = found || fact == "target_table.primary_key"
			}
		}
		if !found {
			t.Fatalf("presence-proven member-unproven owes target_table.primary_key, got %+v",
				t13r1Gaps(r, stmt))
		}
	})
	t.Run("member-proven vs placeholder owes counterpart fact", func(t *testing.T) {
		t.Parallel()
		stmt := t13r3ExchangeStmt(
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			t13r3PK(), placeholder(), []spec.Constraint{})
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("unproven counterpart members must not fabricate 1736, got %#v", findings)
		}
		found := false
		for _, gap := range t13r1Gaps(r, stmt) {
			for _, fact := range gap.RequiredFacts {
				found = found || fact == "exchange_table.primary_key"
			}
		}
		if !found {
			t.Fatalf("owes exchange_table.primary_key, got %+v", t13r1Gaps(r, stmt))
		}
	})
	t.Run("both placeholders owe both facts", func(t *testing.T) {
		t.Parallel()
		stmt := t13r3ExchangeStmt(
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			placeholder(), placeholder(), []spec.Constraint{})
		if findings, err := r.Evaluate(context.Background(), stmt); err != nil || len(findings) != 0 {
			t.Fatalf("two unproven member sequences compare nothing — err=%v findings=%#v", err, findings)
		}
		var tFact, xFact bool
		for _, gap := range t13r1Gaps(r, stmt) {
			for _, fact := range gap.RequiredFacts {
				tFact = tFact || fact == "target_table.primary_key"
				xFact = xFact || fact == "exchange_table.primary_key"
			}
		}
		if !tFact || !xFact {
			t.Fatalf("each side owes its own primary_key fact, got %+v", t13r1Gaps(r, stmt))
		}
	})
	t.Run("proven presence vs proven absence still blocks", func(t *testing.T) {
		t.Parallel()
		stmt := t13r3ExchangeStmt(
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			[]spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			placeholder(), nil, []spec.Constraint{})
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" {
			t.Fatalf("proven-present PK vs proven-absent PK is an independent 1736, got %#v", findings)
		}
	})
}
