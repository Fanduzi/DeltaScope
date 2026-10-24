// Package ddl verifies the T10-R3 CREATE TABLE member-comparison evidence
// path: a lifecycle rule whose pairing matches two different index
// declarations inside one CREATE TABLE, but cannot complete the plain-key
// comparison because either member carries a non-plain key part, must report
// an attributable key-parts gap — "no redundancy found" is unverifiable,
// never a silent pass. Controls pin that self-pairs, non-pairing members,
// plain pairs, and disabled rules still emit nothing.
// input: create-table spec statements whose declared indexes mix plain and
// non-plain member forms
// output: rule-level EvidenceGaps attribution for duplicate, left-prefix,
// and unique-overlap pairings — plus the preserved plain-pair findings
// pos: domain DDL rule test coverage for issue #89 T10-R3 repair A; the
// shared lifecycle gap path lives in internal/domain/rule/ddl/index_rules.go
// and common.go
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r3CreateTable builds the spec shape the MySQL extractor produces for
// CREATE TABLE … (index declarations live in DDL.Indexes, never in Alter).
func t10r3CreateTable(indexes ...spec.Index) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Columns: []spec.Column{
				{Name: "c", Type: "varchar(32)"},
				{Name: "b", Type: "int"},
				{Name: "a", Type: "int"},
			},
			Indexes: indexes,
		},
	}
}

func t10r3DuplicateRule(t *testing.T, forbid bool) rule.StatementRule {
	t.Helper()
	r, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": forbid},
	})
	if err != nil {
		t.Fatalf("new duplicate rule: %v", err)
	}
	return r
}

func t10r3LeftPrefixRule(t *testing.T, forbid bool) rule.StatementRule {
	t.Helper()
	r, err := newRedundantLeftPrefixIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": forbid},
	})
	if err != nil {
		t.Fatalf("new left-prefix rule: %v", err)
	}
	return r
}

func t10r3UniqueOverlapRule(t *testing.T, forbid bool) rule.StatementRule {
	t.Helper()
	r, err := newRedundantUniqueOverlapIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": forbid},
	})
	if err != nil {
		t.Fatalf("new unique-overlap rule: %v", err)
	}
	return r
}

// t10r3AssertKeyPartsGap pins the member-comparison gap contract: exactly one
// gap carrying the index key-parts fact, and no fabricated finding.
func t10r3AssertKeyPartsGap(t *testing.T, r rule.StatementRule, statement spec.Statement) {
	t.Helper()
	gr, ok := r.(gapAwareRule)
	if !ok {
		t.Fatalf("rule %s must implement EvidenceGaps", r.ID())
	}
	gaps := gr.EvidenceGaps(statement)
	if len(gaps) != 1 {
		t.Fatalf("%s must report exactly one gap for the skipped comparison, got %+v", r.ID(), gaps)
	}
	gap := gaps[0]
	if gap.ReasonCode != "unknown_table_state" ||
		len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "target_table.indexes.key_parts" {
		t.Fatalf("gap must attribute the index key-parts fact, got %+v", gap)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("an uncompletable comparison must not fabricate findings, got %+v", findings)
	}
}

// t10r3AssertNoGap pins the negative control: the rule must not emit a
// member-comparison gap on this statement.
func t10r3AssertNoGap(t *testing.T, r rule.StatementRule, statement spec.Statement) {
	t.Helper()
	gr, ok := r.(gapAwareRule)
	if !ok {
		t.Fatalf("rule %s must implement EvidenceGaps", r.ID())
	}
	if gaps := gr.EvidenceGaps(statement); len(gaps) != 0 {
		t.Fatalf("%s must emit no gap here, got %+v", r.ID(), gaps)
	}
}

// --- duplicate ------------------------------------------------------------

// Pinned counterexample (MySQL 8.0.46): CREATE TABLE t (c VARCHAR(32),
// KEY i1(c(3)), KEY i2(c(3))) — same kind, same named columns, non-plain
// members on both sides. The plain-key comparison cannot complete, so the
// rule owes a key-parts gap, not silence.
func TestT10R3CreateTableDuplicateNonPlainPairReportsGap(t *testing.T) {
	t.Parallel()
	statement := t10r3CreateTable(
		spec.Index{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
		spec.Index{Name: "i2", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
	)
	t10r3AssertKeyPartsGap(t, t10r3DuplicateRule(t, true), statement)
}

// The same pairing fires on unique-kind members too — the duplicate rule
// compares within one kind.
func TestT10R3CreateTableDuplicateNonPlainUniquePairReportsGap(t *testing.T) {
	t.Parallel()
	statement := t10r3CreateTable(
		spec.Index{Name: "u1", Kind: spec.IndexKindUnique, Columns: []string{"c"}, DescParts: 1},
		spec.Index{Name: "u2", Kind: spec.IndexKindUnique, Columns: []string{"c"}, DescParts: 1},
	)
	t10r3AssertKeyPartsGap(t, t10r3DuplicateRule(t, true), statement)
}

// A plain declared member pairing against a non-plain declared member is
// equally uncompletable — the declared side is not exempt because the gap
// concerns the comparison, not only "provider" members.
func TestT10R3CreateTableDuplicateMixedPlainNonPlainReportsGap(t *testing.T) {
	t.Parallel()
	statement := t10r3CreateTable(
		spec.Index{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		spec.Index{Name: "i2", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
	)
	t10r3AssertKeyPartsGap(t, t10r3DuplicateRule(t, true), statement)
}

// Controls: pairing semantics still gate the gap — different kinds, different
// columns, single members, plain pairs, and a disabled rule emit nothing.
func TestT10R3CreateTableDuplicateGapControls(t *testing.T) {
	t.Parallel()
	prefix := func(name, col string) spec.Index {
		return spec.Index{Name: name, Kind: spec.IndexKindSecondary, Columns: []string{col}, PrefixParts: 1}
	}
	for _, tc := range []struct {
		name     string
		indexes  []spec.Index
		ruleID   string
		wantNote string
	}{
		{
			name: "kind mismatch does not pair",
			indexes: []spec.Index{
				{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
				{Name: "u1", Kind: spec.IndexKindUnique, Columns: []string{"c"}, PrefixParts: 1},
			},
			wantNote: "duplicate pairs only same-kind members",
		},
		{
			name: "different columns do not pair",
			indexes: []spec.Index{
				prefix("i1", "c"), prefix("i2", "b"),
			},
			wantNote: "columns differ — no applicable comparison",
		},
		{
			name:     "single non-plain member has no pair",
			indexes:  []spec.Index{prefix("i1", "c")},
			wantNote: "a list must not self-pair",
		},
		{
			name: "plain duplicate pair is provable — no gap",
			indexes: []spec.Index{
				{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
				{Name: "i2", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
			},
			wantNote: "the comparison completed as a finding",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t10r3AssertNoGap(t, t10r3DuplicateRule(t, true), t10r3CreateTable(tc.indexes...))
		})
	}
	t.Run("disabled rule emits nothing", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(prefix("i1", "c"), prefix("i2", "c"))
		gr, ok := t10r3DuplicateRule(t, false).(gapAwareRule)
		if !ok {
			t.Fatal("duplicate rule must implement EvidenceGaps")
		}
		if gaps := gr.EvidenceGaps(statement); len(gaps) != 0 {
			t.Fatalf("disabled rule must emit no gap, got %+v", gaps)
		}
	})
}

// --- left-prefix ----------------------------------------------------------

// Shorter declared member versus a longer member carrying a prefix part:
// (c) pairs against (c(3), b) under the column shape but the real sequence
// is unproven — the rule owes a gap.
func TestT10R3CreateTableLeftPrefixNonPlainPairReportsGap(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"shorter_first", "longer_first"} {
		order := order
		t.Run(order, func(t *testing.T) {
			t.Parallel()
			shorter := spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}
			longer := spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"c", "b"}, PrefixParts: 1}
			indexes := []spec.Index{shorter, longer}
			if order == "longer_first" {
				indexes = []spec.Index{longer, shorter}
			}
			t10r3AssertKeyPartsGap(t, t10r3LeftPrefixRule(t, true), t10r3CreateTable(indexes...))
		})
	}
}

// An expression-bearing declared member pairing as the longer side must also
// emit the gap — the hidden key parts make the truncation unverifiable.
func TestT10R3CreateTableLeftPrefixExpressionComparatorReportsGap(t *testing.T) {
	t.Parallel()
	statement := t10r3CreateTable(
		spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"c", "b"},
			HasExpressionKeys: true, ExpressionCount: 1},
	)
	t10r3AssertKeyPartsGap(t, t10r3LeftPrefixRule(t, true), statement)
}

// Controls: equal-length non-plain secondary members cannot pair as
// shorter-versus-longer, and plain/standalone/disabled stay silent.
func TestT10R3CreateTableLeftPrefixGapControls(t *testing.T) {
	t.Parallel()
	t.Run("equal-length members do not pair", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
			spec.Index{Name: "i2", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
		)
		t10r3AssertNoGap(t, t10r3LeftPrefixRule(t, true), statement)
	})
	t.Run("plain left-prefix pair is provable — no gap", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
			spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"c", "b"}},
		)
		t10r3AssertNoGap(t, t10r3LeftPrefixRule(t, true), statement)
		findings, err := t10r3LeftPrefixRule(t, true).Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 {
			t.Fatalf("plain left-prefix pair must keep its finding, got %+v", findings)
		}
	})
	t.Run("unique kind never enters left-prefix pairing", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "u_narrow", Kind: spec.IndexKindUnique, Columns: []string{"c"}, PrefixParts: 1},
			spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"c", "b"}, PrefixParts: 1},
		)
		t10r3AssertNoGap(t, t10r3LeftPrefixRule(t, true), statement)
	})
	t.Run("disabled rule emits nothing", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
			spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"c", "b"}, PrefixParts: 1},
		)
		gr, ok := t10r3LeftPrefixRule(t, false).(gapAwareRule)
		if !ok {
			t.Fatal("left-prefix rule must implement EvidenceGaps")
		}
		if gaps := gr.EvidenceGaps(statement); len(gaps) != 0 {
			t.Fatalf("disabled rule must emit no gap, got %+v", gaps)
		}
	})
}

// --- unique-overlap -------------------------------------------------------

// A declared secondary member whose named columns equal a declared unique
// member pairs under the rule's shape; a non-plain member on the unique side
// leaves the overlap unverifiable — gap.
func TestT10R3CreateTableUniqueOverlapNonPlainPairReportsGap(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"unique_first", "secondary_first"} {
		order := order
		t.Run(order, func(t *testing.T) {
			t.Parallel()
			unique := spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}, DescParts: 1}
			secondary := spec.Index{Name: "s_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}
			indexes := []spec.Index{unique, secondary}
			if order == "secondary_first" {
				indexes = []spec.Index{secondary, unique}
			}
			t10r3AssertKeyPartsGap(t, t10r3UniqueOverlapRule(t, true), t10r3CreateTable(indexes...))
		})
	}
}

// The secondary side carrying the non-plain member is equally unverifiable.
func TestT10R3CreateTableUniqueOverlapNonPlainSecondaryReportsGap(t *testing.T) {
	t.Parallel()
	statement := t10r3CreateTable(
		spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}},
		spec.Index{Name: "s_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1},
	)
	t10r3AssertKeyPartsGap(t, t10r3UniqueOverlapRule(t, true), statement)
}

// Controls: two uniques never pair (the subject must be secondary),
// column-mismatched members never pair, and plain/disabled stay silent.
func TestT10R3CreateTableUniqueOverlapGapControls(t *testing.T) {
	t.Parallel()
	t.Run("two uniques do not pair", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "uq1", Kind: spec.IndexKindUnique, Columns: []string{"c"}, PrefixParts: 1},
			spec.Index{Name: "uq2", Kind: spec.IndexKindUnique, Columns: []string{"c"}, PrefixParts: 1},
		)
		t10r3AssertNoGap(t, t10r3UniqueOverlapRule(t, true), statement)
	})
	t.Run("different columns do not pair", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}, PrefixParts: 1},
			spec.Index{Name: "s_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}},
		)
		t10r3AssertNoGap(t, t10r3UniqueOverlapRule(t, true), statement)
	})
	t.Run("plain overlap pair is provable — no gap", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}},
			spec.Index{Name: "s_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		)
		t10r3AssertNoGap(t, t10r3UniqueOverlapRule(t, true), statement)
		findings, err := t10r3UniqueOverlapRule(t, true).Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 {
			t.Fatalf("plain unique overlap must keep its finding, got %+v", findings)
		}
	})
	t.Run("disabled rule emits nothing", func(t *testing.T) {
		t.Parallel()
		statement := t10r3CreateTable(
			spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}, DescParts: 1},
			spec.Index{Name: "s_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		)
		gr, ok := t10r3UniqueOverlapRule(t, false).(gapAwareRule)
		if !ok {
			t.Fatal("unique-overlap rule must implement EvidenceGaps")
		}
		if gaps := gr.EvidenceGaps(statement); len(gaps) != 0 {
			t.Fatalf("disabled rule must emit no gap, got %+v", gaps)
		}
	})
}
