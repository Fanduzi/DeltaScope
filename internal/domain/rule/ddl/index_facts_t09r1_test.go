// Package ddl verifies the T09-R1 shared index-comparability contract.
// input: extractor/provider-verbatim spec.Index shapes — plain keys plus
// indexes marked with the recorded non-plain key-part facts (prefix lengths,
// descending parts, expression keys, predicates, include columns, unmodeled
// options, GLOBAL)
// output: duplicate, left-prefix and unique-overlap rules share one
// comparability premise — a pair involving a non-plain member never reports,
// while plain same-column pairs keep their accurate findings
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
