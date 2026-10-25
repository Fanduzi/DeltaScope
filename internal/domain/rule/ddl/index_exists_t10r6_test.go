// Package ddl verifies the T10-R6 drop-primary-key existence closure: the
// `primary` name-keyed answer binds the actual stored-name space, not mere
// PK presence — a renamed PK frees `primary` (1091 verified), a proven
// member `primary` answers member-first, the occupying PK object answers
// PK-side, and withheld bindings report the exact missing member/PK fact.
// input: hand-built table snapshots bound to drop_primary_key statements
// output: alterPrimaryKeyExistenceRule Evaluate findings and EvidenceGaps
// pos: rule-side T10-R6 actual-name routing coverage (issue #89)
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t10r6DropPKRule(t *testing.T) rule.StatementRule {
	t.Helper()
	ruleUnderTest, err := newAlterPrimaryKeyExistenceRule(
		ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ruleUnderTest.(rule.EvidenceReporter); !ok {
		t.Fatal("the rule must implement rule.EvidenceReporter")
	}
	return ruleUnderTest
}

func t10r6DropPKGaps(t *testing.T, ruleUnderTest rule.StatementRule, stmt spec.Statement) []rule.EvidenceGap {
	t.Helper()
	reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("the rule must implement rule.EvidenceReporter")
	}
	return reporter.EvidenceGaps(stmt)
}

func t10r6DropPKStatement(dialect spec.Dialect, snapshot *spec.TableSnapshot) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter:     []spec.Alter{{Action: "drop_primary_key"}},
		},
		Metadata: &spec.Metadata{TargetTable: snapshot},
	}
}

// C: the PK object renamed to pk2 frees `primary` — the statement hits
// nothing natively (1091, probe a2b), so mere PK presence must not answer
// target existence. The proven-missing finding must fire.
func TestT10R6DropPrimaryKeyFreedNameFiresMissing(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	ruleUnderTest := t10r6DropPKRule(t)
	stmt := t10r6DropPKStatement(spec.DialectTiDB, snapshot)
	findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("the freed `primary` name is a proven-missing target — the blocker must fire, got %+v", findings)
	}
	if got := findings[0].Metadata["exists"]; got != false {
		t.Fatalf("the finding records the proven-absent answer, got %+v", findings[0].Metadata)
	}
	if gaps := t10r6DropPKGaps(t, ruleUnderTest, stmt); len(gaps) != 0 {
		t.Fatalf("every consulted fact is proven — no gaps, got %+v", gaps)
	}
}

// C controls: the member-first and PK-side bindings keep answering
// presence — a proven member `primary` and a `primary`-stored PK object
// both suppress the blocker, while a proven-absent PK fires on either
// dialect.
func TestT10R6DropPrimaryKeyBindingControls(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t10r6DropPKRule(t)
	t.Run("member primary owns the name", func(t *testing.T) {
		t.Parallel()
		snapshot := &spec.TableSnapshot{
			Exists:              true,
			PrimaryKey:          &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary},
			PrimaryKeyIsHandle:  true,
			PrimaryKeyClustered: true,
			Indexes:             []spec.Index{{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}},
		}
		findings, err := ruleUnderTest.Evaluate(context.Background(), t10r6DropPKStatement(spec.DialectTiDB, snapshot))
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("a member `primary` owns the target — no fabricated absent blocker, got %+v", findings)
		}
	})
	t.Run("primary-stored PK object owns the name", func(t *testing.T) {
		t.Parallel()
		snapshot := &spec.TableSnapshot{
			Exists:     true,
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary},
			Indexes:    []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}},
		}
		findings, err := ruleUnderTest.Evaluate(context.Background(), t10r6DropPKStatement(spec.DialectTiDB, snapshot))
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("the occupying PK object answers presence, got %+v", findings)
		}
	})
	t.Run("proven-absent PK fires on both dialects", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			snapshot := &spec.TableSnapshot{
				Exists:  true,
				Indexes: []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}},
			}
			findings, err := ruleUnderTest.Evaluate(context.Background(), t10r6DropPKStatement(dialect, snapshot))
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("%s: a proven-absent PK is a proven-missing target, got %+v", dialect, findings)
			}
		}
	})
}

// C gaps: a withheld binding reports exactly the facts it could not prove
// — an unvouched member space owes the member collection, a withheld PK
// object name owes the primary-key identity, and both together owe both.
func TestT10R6DropPrimaryKeyAmbiguousGapFacts(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t10r6DropPKRule(t)
	requiresFacts := func(t *testing.T, stmt spec.Statement, want ...string) {
		t.Helper()
		gaps := t10r6DropPKGaps(t, ruleUnderTest, stmt)
		if len(gaps) == 0 {
			t.Fatalf("the suppressed binding owes its missing facts, got none")
		}
		got := map[string]bool{}
		for _, gap := range gaps {
			for _, fact := range gap.RequiredFacts {
				got[fact] = true
			}
		}
		for _, fact := range want {
			if !got[fact] {
				t.Fatalf("the suppressed binding owes %s, got %+v", fact, gaps)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("exactly the unproven facts are owed, got %+v", gaps)
		}
	}
	t.Run("pk object name withheld under vouched members", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r5RenamedPK()
		snapshot.PrimaryKey.NameUnknown = true
		snapshot.PrimaryKey.Name = ""
		stmt := t10r6DropPKStatement(spec.DialectTiDB, snapshot)
		findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("the withheld name suppresses the blocker, got %+v", findings)
		}
		requiresFacts(t, stmt, "target_table.primary_key")
	})
	t.Run("unvouched member space under proven non-primary pk name", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r5RenamedPK()
		snapshot.IndexNamesUnknown = true
		snapshot.Indexes[0].NameUnknown = true
		stmt := t10r6DropPKStatement(spec.DialectTiDB, snapshot)
		findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("the unvouched member space suppresses the blocker, got %+v", findings)
		}
		requiresFacts(t, stmt, "target_table.indexes")
	})
	t.Run("both sides unproven owe both facts", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r5RenamedPK()
		snapshot.PrimaryKey.NameUnknown = true
		snapshot.PrimaryKey.Name = ""
		snapshot.Indexes[0].NameUnknown = true
		stmt := t10r6DropPKStatement(spec.DialectTiDB, snapshot)
		requiresFacts(t, stmt, "target_table.indexes", "target_table.primary_key")
	})
}
