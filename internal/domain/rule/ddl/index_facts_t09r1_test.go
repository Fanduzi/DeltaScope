// Package ddl verifies the T09-R1 shared index-comparability contract.
// input: extractor/provider-verbatim spec.Index shapes — plain keys plus
// indexes marked with the recorded non-plain key-part facts (prefix lengths,
// descending parts, expression keys, predicates, include columns, unmodeled
// options, GLOBAL)
// output: duplicate, left-prefix and unique-overlap rules share one
// comparability premise — a pair involving a non-plain member never reports,
// while plain same-column pairs keep their accurate findings; R3 adds the
// truncated-projection pairing: an expression-bearing member's Columns omits
// hidden parts, so equal length cannot disprove a left-prefix pairing
// pos: domain rule coverage for issue #88 T09-R1; the audit-level matrix in
// internal/application/audit/batch_state_t09r1_test.go covers the real
// parser/provider chain
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t09r1CreateTableStatement builds a create-table statement carrying the
// given embedded index declarations — the same shape the extractor writes.
func t09r1CreateTableStatement(indexes ...spec.Index) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
			Indexes:   indexes,
		},
	}
}

// t09r1NonPlainMarks enumerates the recorded non-plain key facts a provider
// or parser may stamp on an otherwise plain-column index.
func t09r1NonPlainMarks() map[string]func(*spec.Index) {
	return map[string]func(*spec.Index){
		"prefix_part":      func(i *spec.Index) { i.PrefixParts = 1 },
		"desc_part":        func(i *spec.Index) { i.DescParts = 1 },
		"expression_keys":  func(i *spec.Index) { i.HasExpressionKeys = true; i.ExpressionCount = 1 },
		"predicate":        func(i *spec.Index) { i.HasPredicate = true },
		"included_columns": func(i *spec.Index) { i.IncludedColumns = []string{"extra"} },
		"global":           func(i *spec.Index) { i.Global = true },
		"unmodeled_option": func(i *spec.Index) { i.UnmodeledOptions = []string{"comment"} },
	}
}

func t09r1DuplicateRule(t *testing.T) rule.StatementRule {
	t.Helper()
	r, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

func t09r1LeftPrefixRule(t *testing.T) rule.StatementRule {
	t.Helper()
	r, err := newRedundantLeftPrefixIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

func t09r1UniqueOverlapRule(t *testing.T) rule.StatementRule {
	t.Helper()
	r, err := newRedundantUniqueOverlapIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return r
}

// Every recorded non-plain mark suppresses the duplicate verdict on both the
// first-declared and second-declared position of the pair.
func TestT09R1DuplicateSuppressesNonPlainPairs(t *testing.T) {
	t.Parallel()
	for mark, apply := range t09r1NonPlainMarks() {
		mark, apply := mark, apply
		t.Run(mark, func(t *testing.T) {
			t.Parallel()
			for _, side := range []string{"first", "second"} {
				side := side
				t.Run(side, func(t *testing.T) {
					t.Parallel()
					first := spec.Index{Name: "i_first", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}
					second := spec.Index{Name: "i_second", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}
					if side == "first" {
						apply(&first)
					} else {
						apply(&second)
					}
					statement := t09r1CreateTableStatement(first, second)
					r := t09r1DuplicateRule(t)
					findings, err := r.Evaluate(context.Background(), statement)
					if err != nil {
						t.Fatalf("evaluate: %v", err)
					}
					if len(findings) != 0 {
						t.Fatalf("%s on %s side must suppress the duplicate pair, got %+v", mark, side, findings)
					}
				})
			}
		})
	}
}

// Control: the same pair without any mark keeps the accurate finding.
func TestT09R1DuplicatePlainControl(t *testing.T) {
	t.Parallel()
	statement := t09r1CreateTableStatement(
		spec.Index{Name: "i_first", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		spec.Index{Name: "i_second", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	)
	findings, err := t09r1DuplicateRule(t).Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("plain same-column pair must keep the duplicate finding, got %+v", findings)
	}
}

// Left-prefix suppression on either candidate side: a marked shorter is
// never a redundant subject and a marked longer never proves coverage.
func TestT09R1LeftPrefixSuppressesNonPlainPairs(t *testing.T) {
	t.Parallel()
	for mark, apply := range t09r1NonPlainMarks() {
		mark, apply := mark, apply
		t.Run(mark, func(t *testing.T) {
			t.Parallel()
			for _, side := range []string{"shorter", "longer"} {
				side := side
				t.Run(side, func(t *testing.T) {
					t.Parallel()
					shorter := spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}
					longer := spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}}
					if side == "shorter" {
						apply(&shorter)
					} else {
						apply(&longer)
					}
					statement := t09r1CreateTableStatement(shorter, longer)
					r := t09r1LeftPrefixRule(t)
					findings, err := r.Evaluate(context.Background(), statement)
					if err != nil {
						t.Fatalf("evaluate: %v", err)
					}
					if len(findings) != 0 {
						t.Fatalf("%s on %s side must suppress the left-prefix pair, got %+v", mark, side, findings)
					}
				})
			}
		})
	}
}

// Left-prefix control: plain (a) under plain (a,b) keeps the finding.
func TestT09R1LeftPrefixPlainControl(t *testing.T) {
	t.Parallel()
	statement := t09r1CreateTableStatement(
		spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
		spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}},
	)
	findings, err := t09r1LeftPrefixRule(t).Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("plain left-prefix pair must keep the finding, got %+v", findings)
	}
}

// Unique-overlap suppression on either candidate side: a marked unique never
// stands in as a plain unique and a marked secondary is never convicted.
func TestT09R1UniqueOverlapSuppressesNonPlainPairs(t *testing.T) {
	t.Parallel()
	for mark, apply := range t09r1NonPlainMarks() {
		mark, apply := mark, apply
		t.Run(mark, func(t *testing.T) {
			t.Parallel()
			for _, side := range []string{"unique", "secondary"} {
				side := side
				t.Run(side, func(t *testing.T) {
					t.Parallel()
					uniq := spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}}
					sec := spec.Index{Name: "i_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}
					if side == "unique" {
						apply(&uniq)
					} else {
						apply(&sec)
					}
					statement := t09r1CreateTableStatement(uniq, sec)
					r := t09r1UniqueOverlapRule(t)
					findings, err := r.Evaluate(context.Background(), statement)
					if err != nil {
						t.Fatalf("evaluate: %v", err)
					}
					if len(findings) != 0 {
						t.Fatalf("%s on %s side must suppress the unique-overlap pair, got %+v", mark, side, findings)
					}
				})
			}
		})
	}
}

// Unique-overlap control: plain unique + plain secondary keeps the finding.
func TestT09R1UniqueOverlapPlainControl(t *testing.T) {
	t.Parallel()
	statement := t09r1CreateTableStatement(
		spec.Index{Name: "uq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}},
		spec.Index{Name: "i_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	)
	findings, err := t09r1UniqueOverlapRule(t).Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("plain unique overlap must keep the finding, got %+v", findings)
	}
}

// The ALTER projection path runs the same inner rules over
// alterLifecycleIndexes — a provider-loaded non-plain existing index is not
// a comparable partner either.
func TestT09R1AlterExistingNonPlainSuppresses(t *testing.T) {
	t.Parallel()
	for mark, apply := range t09r1NonPlainMarks() {
		mark, apply := mark, apply
		t.Run(mark, func(t *testing.T) {
			t.Parallel()
			existing := spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}
			apply(&existing)
			statement := alterAddIndexStatement(alterAddIndex("i_new", spec.IndexKindSecondary, "a"))
			statement = withTargetSnapshot(statement, snapshotWithIndexes(existing))
			inner, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
				Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
			})
			if err != nil {
				t.Fatalf("new inner rule: %v", err)
			}
			r := alterAddedIndexRule{
				ruleID:               ruleIDAlterAddIndexDuplicateForbid,
				inner:                inner,
				indexes:              alterLifecycleIndexes,
				addedOnlyAttribution: true,
				lifecycle:            true,
			}
			findings, err := r.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("existing %s index must not convict the added plain index, got %+v", mark, findings)
			}
		})
	}
}

// Opposite control on the ALTER path: a plain existing index still convicts.
func TestT09R1AlterExistingPlainStillFlags(t *testing.T) {
	t.Parallel()
	existing := spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}
	statement := alterAddIndexStatement(alterAddIndex("i_new", spec.IndexKindSecondary, "a"))
	statement = withTargetSnapshot(statement, snapshotWithIndexes(existing))
	inner, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new inner rule: %v", err)
	}
	r := alterAddedIndexRule{
		ruleID:               ruleIDAlterAddIndexDuplicateForbid,
		inner:                inner,
		indexes:              alterLifecycleIndexes,
		addedOnlyAttribution: true,
		lifecycle:            true,
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("plain existing index must keep the duplicate finding, got %+v", findings)
	}
}

// --- Member-comparability evidence gaps -----------------------------------
// A vouched index set proves WHICH indexes exist; a non-plain member inside
// it still leaves the pair comparison incomplete. When an existing member
// could have paired with a declared index under a rule's plain-key shape,
// the rule must report an attributable gap — never pass as if redundancy
// were disproved. Gaps must stay scoped to actually-affected comparisons:
// a non-plain member no declared index could pair with emits nothing.

func t09r1GapHasKeyPartsFact(gaps []rule.EvidenceGap) bool {
	for _, gap := range gaps {
		for _, fact := range gap.RequiredFacts {
			if fact == indexKeyPartsGapFact {
				return true
			}
		}
	}
	return false
}

// create_index entry: each pair rule reports the member-comparability gap
// when a skipped existing member could have convicted a declared index.
func TestT09R1CreateIndexGapOnSkippedExistingMember(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		rule     func(t *testing.T) rule.StatementRule
		declared spec.Index
		existing spec.Index
	}{
		{
			name:     "duplicate_same_signature",
			rule:     t09r1DuplicateRule,
			declared: spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
			existing: spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}, PrefixParts: 1},
		},
		{
			name:     "left_prefix_covering",
			rule:     t09r1LeftPrefixRule,
			declared: spec.Index{Name: "i_narrow", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
			existing: spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}, PrefixParts: 1},
		},
		{
			name:     "unique_overlap_same_columns",
			rule:     t09r1UniqueOverlapRule,
			declared: spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
			existing: spec.Index{Name: "uq_pre", Kind: spec.IndexKindUnique, Columns: []string{"a"}, DescParts: 1},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statement := withTargetSnapshot(
				mysqlCreateIndexStatement(tc.declared),
				snapshotWithIndexes(tc.existing),
			)
			reporter, ok := tc.rule(t).(rule.EvidenceReporter)
			if !ok {
				t.Fatal("lifecycle rule must implement EvidenceReporter")
			}
			gaps := reporter.EvidenceGaps(statement)
			if !t09r1GapHasKeyPartsFact(gaps) {
				t.Fatalf("skipped member comparison must report the key-parts gap, got %+v", gaps)
			}
		})
	}
}

// Controls: the gap stays scoped to actually-affected comparisons — a
// non-plain member no declared index can pair with, a plain member that was
// compared, and a known-empty set all stay gap-free; an unknown set keeps the
// original collection fact vocabulary.
func TestT09R1CreateIndexGapScopeControls(t *testing.T) {
	t.Parallel()
	t.Run("non_pairing_non_plain_member", func(t *testing.T) {
		t.Parallel()
		statement := withTargetSnapshot(
			mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			snapshotWithIndexes(spec.Index{Name: "i_other", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, PrefixParts: 1}),
		)
		for _, build := range []func(t *testing.T) rule.StatementRule{t09r1DuplicateRule, t09r1LeftPrefixRule, t09r1UniqueOverlapRule} {
			gaps := build(t).(rule.EvidenceReporter).EvidenceGaps(statement)
			if len(gaps) != 0 {
				t.Fatalf("unrelated non-plain member must not emit a comparison gap, got %+v", gaps)
			}
		}
	})
	t.Run("plain_member_compared", func(t *testing.T) {
		t.Parallel()
		statement := withTargetSnapshot(
			mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			snapshotWithIndexes(spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
		)
		gaps := t09r1DuplicateRule(t).(rule.EvidenceReporter).EvidenceGaps(statement)
		if len(gaps) != 0 {
			t.Fatalf("a compared plain member emits no gap, got %+v", gaps)
		}
	})
	t.Run("known_empty_set", func(t *testing.T) {
		t.Parallel()
		statement := withTargetSnapshot(
			mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			snapshotWithIndexes(),
		)
		gaps := t09r1DuplicateRule(t).(rule.EvidenceReporter).EvidenceGaps(statement)
		if len(gaps) != 0 {
			t.Fatalf("known-empty set emits no gap, got %+v", gaps)
		}
	})
	t.Run("unknown_set_keeps_collection_fact", func(t *testing.T) {
		t.Parallel()
		snap := snapshotWithIndexes()
		snap.Indexes = nil
		snap.IndexesUnknown = true
		statement := withTargetSnapshot(
			mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			snap,
		)
		gaps := t09r1DuplicateRule(t).(rule.EvidenceReporter).EvidenceGaps(statement)
		if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
			t.Fatalf("unknown set must keep the collection fact, got %+v", gaps)
		}
	})
}

// --- R3: expression members hide key parts outside Columns ----------------
// The provider records an expression key part as a mark only — the member
// never enters Columns, so Columns is an incomplete projection of the real
// key sequence. A strict-length check on the truncated projection cannot
// prove "no pairing": a declared index matching every named column can
// still be a true left prefix once a hidden expression part follows them.

// t09r3ExpressionMember is the provider-verbatim shape of
// CREATE INDEX i ON t(a, (LOWER(b))): the expression part marks the index
// without contributing a column name to Columns.
func t09r3ExpressionMember(name string, kind spec.IndexKind, columns ...string) spec.Index {
	return spec.Index{
		Name:              name,
		Kind:              kind,
		Columns:           columns,
		HasExpressionKeys: true,
		ExpressionCount:   1,
	}
}

// The left-prefix pairing must treat an expression-bearing existing member
// as pairable whenever the declared columns are a prefix of the recorded
// named columns — hidden expression parts mean the real key sequence is
// longer than Columns, so equal length is not a non-pairing proof. Both
// lifecycle entries report through the same pairing.
func TestT09R3LeftPrefixGapOnTruncatedColumns(t *testing.T) {
	t.Parallel()
	existing := t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "a")

	t.Run("create_index", func(t *testing.T) {
		t.Parallel()
		statement := withTargetSnapshot(
			mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			snapshotWithIndexes(existing),
		)
		gaps := t09r1LeftPrefixRule(t).(rule.EvidenceReporter).EvidenceGaps(statement)
		if !t09r1GapHasKeyPartsFact(gaps) {
			t.Fatalf("equal-length truncated Columns must not disprove pairing, got %+v", gaps)
		}
	})
	t.Run("alter_add_index", func(t *testing.T) {
		t.Parallel()
		r, err := newAlterAddedRedundantLeftPrefixRule(rule.LevelWarning, policy.RulePolicy{
			Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
		})
		if err != nil {
			t.Fatalf("new rule: %v", err)
		}
		statement := withTargetSnapshot(
			alterAddIndexStatement(alterAddIndex("i_new", spec.IndexKindSecondary, "a")),
			snapshotWithIndexes(existing),
		)
		gaps := r.(rule.EvidenceReporter).EvidenceGaps(statement)
		if !t09r1GapHasKeyPartsFact(gaps) {
			t.Fatalf("alter wrapper must report the same truncated-Columns gap, got %+v", gaps)
		}
	})
}

// The relaxed pairing stays scoped: members provably unrelated under every
// interleaving of the hidden parts — a declared column absent from the
// named sequence, a named sequence exhausted before the declared prefix
// ends, or a complete Columns projection — still emit no gap.
func TestT09R3LeftPrefixExpressionScopeControls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		declared []string
		existing spec.Index
		wantGap  bool
	}{
		// Real key parts are {a}+expressions; declared b can never head the
		// sequence — provably unrelated.
		{"declared_column_absent", []string{"b"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "a"), false},
		// Declared extends past the named projection: real[2] can only be an
		// expression, never column b — provably unrelated.
		{"declared_longer_than_named", []string{"a", "b"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "a"), false},
		// Named order puts a after b; no interleaving of hidden parts can
		// place a first — provably unrelated.
		{"named_order_disproves", []string{"a"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "b", "a"), false},
		// A Columns-complete non-plain member keeps the strict proof: one
		// named part means no shorter strict prefix can exist.
		{"complete_projection_still_strict", []string{"a"}, spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}, DescParts: 1}, false},
		// An all-expression member holds no named column — declared a can
		// never appear.
		{"all_expression_member", []string{"a"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary), false},
		// Proper prefix of the named projection still pairs, as before.
		{"proper_prefix_still_pairs", []string{"a"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "a", "b"), true},
		// The R3 defect cell: equal length on the truncated projection.
		{"equal_length_truncated", []string{"a"}, t09r3ExpressionMember("i_pre", spec.IndexKindSecondary, "a"), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statement := withTargetSnapshot(
				mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: tc.declared}),
				snapshotWithIndexes(tc.existing),
			)
			gaps := t09r1LeftPrefixRule(t).(rule.EvidenceReporter).EvidenceGaps(statement)
			if tc.wantGap && !t09r1GapHasKeyPartsFact(gaps) {
				t.Fatalf("pairing must report the key-parts gap, got %+v", gaps)
			}
			if !tc.wantGap && len(gaps) != 0 {
				t.Fatalf("provably unrelated member must not emit a gap, got %+v", gaps)
			}
		})
	}
}

// ALTER entry: the add_index/add_constraint lifecycle wrappers report the
// same member-comparability gap through their EvidenceGaps.
func TestT09R1AlterAddIndexGapOnSkippedExistingMember(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		ruleID   string
		newRule  func(level rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error)
		declared spec.Alter
		existing spec.Index
	}{
		{
			name:     "duplicate_same_signature",
			ruleID:   ruleIDAlterAddIndexDuplicateForbid,
			newRule:  newAlterAddedDuplicateIndexForbiddenRule,
			declared: alterAddIndex("i_new", spec.IndexKindSecondary, "a"),
			existing: spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}, PrefixParts: 1},
		},
		{
			name:     "left_prefix_covering",
			ruleID:   ruleIDAlterAddIndexRedundantLeftPrefixForbid,
			newRule:  newAlterAddedRedundantLeftPrefixRule,
			declared: alterAddIndex("i_narrow", spec.IndexKindSecondary, "a"),
			existing: spec.Index{Name: "i_wide", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}, DescParts: 1},
		},
		{
			name:     "unique_overlap_same_columns",
			ruleID:   ruleIDAlterAddIndexRedundantUniqueOverlapForbid,
			newRule:  newAlterAddedRedundantUniqueOverlapRule,
			declared: alterAddIndex("i_new", spec.IndexKindSecondary, "a"),
			existing: spec.Index{Name: "uq_pre", Kind: spec.IndexKindUnique, Columns: []string{"a"}, PrefixParts: 1},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := tc.newRule(rule.LevelWarning, policy.RulePolicy{
				Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			statement := withTargetSnapshot(
				alterAddIndexStatement(tc.declared),
				snapshotWithIndexes(tc.existing),
			)
			gaps := r.(rule.EvidenceReporter).EvidenceGaps(statement)
			if !t09r1GapHasKeyPartsFact(gaps) {
				t.Fatalf("%s skipped member comparison must report the key-parts gap, got %+v", tc.ruleID, gaps)
			}
		})
	}
}
