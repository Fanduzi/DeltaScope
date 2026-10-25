// Package ddl verifies the T10-R5 actual-name binding at the rule layer:
// after a TiDB PK object is renamed off `primary`, name-keyed existence
// answers bind the object's real stored name (pk2 → exists), the freed
// `primary` name no longer asserts presence merely because a PK exists,
// and an unproven PK-object name suppresses instead of guessing.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r5RenamedPK models the provider-reloaded post-rename catalog: the PK
// object carries its real stored name pk2 beside an ordinary member s.
func t10r5RenamedPK() *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		PrimaryKey: &spec.Index{
			Name: "pk2", Kind: spec.IndexKindPrimary, Columns: []string{"id"},
		},
		Indexes: []spec.Index{
			{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		},
	}
}

// The renamed PK object's actual stored name answers existence — pk2 must
// satisfy every name-keyed target predicate even though no member owns it.
func TestT10R5RenamedPKActualNameBindsExists(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	stmt := t10r4Statement(spec.DialectTiDB)
	if !snapshotHasIndex(stmt, snapshot, "pk2") {
		t.Fatal("the PK object's actual stored name must answer existence")
	}
	if !dropIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("DROP INDEX pk2 targets the renamed PK object — it exists")
	}
	if !renameIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("RENAME INDEX pk2 targets the renamed PK object — it exists")
	}
	if !alterIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("ALTER INDEX pk2 targets the renamed PK object — it exists")
	}
}

// The freed `primary` name binds nothing once the object is renamed away:
// the name answers provably absent — verified natively (1176/1091), and
// member `primary` claims remain possible only while the member space is
// unvouched.
func TestT10R5FreedPrimaryNameAnswersAbsent(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	stmt := t10r4Statement(spec.DialectTiDB)
	if snapshotHasIndex(stmt, snapshot, "primary") {
		t.Fatal("a renamed PK object frees the `primary` name — presence must not follow the PK identity")
	}
	if dropIndexTargetExists(stmt, snapshot, "primary") {
		t.Fatal("DROP INDEX `primary` after rename hits nothing (1091) — suppress existence")
	}
}

// An unproven PK-object name (attribution ambiguous on reload) must not
// assert either answer — the candidate could own the target name, so every
// name-keyed predicate suppresses rather than guessing presence or absence.
func TestT10R5UnprovenPKObjectNameSuppresses(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	snapshot.PrimaryKey.NameUnknown = true
	snapshot.PrimaryKey.Name = ""
	stmt := t10r4Statement(spec.DialectTiDB)
	if snapshotHasIndex(stmt, snapshot, "pk2") {
		t.Fatal("an unproven PK-object name must not assert existence")
	}
	if !dropIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("an unproven PK-object name must not assert absence either")
	}
	if !alterIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("ambiguous binding suppresses the alter-index answer")
	}
	if !renameIndexTargetExists(stmt, snapshot, "pk2") {
		t.Fatal("ambiguous binding suppresses the rename-source answer")
	}
	// The `primary` target stays ambiguous as well — a suppressed answer.
	if !dropIndexTargetExists(stmt, snapshot, "primary") {
		t.Fatal("the withheld PK-object name keeps `primary` ambiguous, never a fabricated absence")
	}
}

// A withheld PK-object name leaves even a non-`primary` target ambiguous:
// the member name space may be vouched (no member gap is owed), yet the PK
// side cannot clear the name — the suppressed answer must surface as
// target_table.primary_key rather than pass clean.
func TestT10R5WithheldPKNameGapOnNonPrimaryTarget(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	snapshot.PrimaryKey.Name = ""
	snapshot.PrimaryKey.NameUnknown = true
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter:     []spec.Alter{{Action: "drop_index", Name: "pkX"}},
		},
		Metadata: &spec.Metadata{TargetTable: snapshot},
	}
	ruleUnderTest, err := newIndexExistenceRule(ruleIDAlterDropIndexExistsRequire,
		[]string{"drop_index"}, policy.RulePolicy{}, dropIndexTargetExists)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("ambiguous binding must suppress the finding, got %+v", findings)
	}
	reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("the index existence rule must implement rule.EvidenceReporter")
	}
	found := false
	for _, gap := range reporter.EvidenceGaps(stmt) {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.primary_key" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("a withheld PK-object name owes target_table.primary_key on the ambiguous target")
	}
}

// A bare IndexNamesUnknown marks the member name space unvouched — the PK
// object's stored name has its own marker (PrimaryKey.NameUnknown), so a
// `primary` target still resolves the proven PK object: exists, no
// suppression, and no primary_key debt. The member fact is owed only while
// a member may literally own `primary` — which a proven PK object rules
// out (1061).
func TestT10R5IndexNamesUnknownPrimaryTargetKeepsPKBinding(t *testing.T) {
	t.Parallel()
	snapshot := t10r5RenamedPK()
	snapshot.IndexNamesUnknown = true
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter:     []spec.Alter{{Action: "drop_index", Name: "primary"}},
		},
		Metadata: &spec.Metadata{TargetTable: snapshot},
	}
	ruleUnderTest, err := newIndexExistenceRule(ruleIDAlterDropIndexExistsRequire,
		[]string{"drop_index"}, policy.RulePolicy{}, dropIndexTargetExists)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("a proven PK object still owns `primary` under a bare name-space flag, got %+v", findings)
	}
	reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("the index existence rule must implement rule.EvidenceReporter")
	}
	for _, gap := range reporter.EvidenceGaps(stmt) {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.primary_key" {
				t.Fatalf("the bare member-name flag never unvouches the PK object's own stored name")
			}
		}
	}
}

// TiDB routes DROP PRIMARY KEY through dropIndex(…, "PRIMARY") member-first:
// a PK-less table with a proven member literally named `primary` drops that
// member — the exists rule must not fabricate a "primary key does not
// exist" blocker. An unvouched member space suppresses and owes the member
// collection fact instead.
func TestT10R5DropPrimaryKeyMemberFirstBinding(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newAlterPrimaryKeyExistenceRule(
		ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	stmtFor := func(snapshot *spec.TableSnapshot) spec.Statement {
		return spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "t"},
				Alter:     []spec.Alter{{Action: "drop_primary_key"}},
			},
			Metadata: &spec.Metadata{TargetTable: snapshot},
		}
	}
	t.Run("member primary exists answers the target", func(t *testing.T) {
		t.Parallel()
		snapshot := &spec.TableSnapshot{Exists: true, Indexes: []spec.Index{
			{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		}}
		findings, err := ruleUnderTest.Evaluate(context.Background(), stmtFor(snapshot))
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("a member `primary` owns the DROP PRIMARY KEY target — no fabricated PK-absent blocker, got %+v", findings)
		}
	})
	t.Run("unvouched member space suppresses and gaps", func(t *testing.T) {
		t.Parallel()
		snapshot := &spec.TableSnapshot{Exists: true, IndexesUnknown: true}
		stmt := stmtFor(snapshot)
		findings, err := ruleUnderTest.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("an unvouched member space suppresses the blocker, got %+v", findings)
		}
		reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
		if !ok {
			t.Fatal("the rule must implement rule.EvidenceReporter")
		}
		found := false
		for _, gap := range reporter.EvidenceGaps(stmt) {
			for _, fact := range gap.RequiredFacts {
				if fact == "target_table.indexes" {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("a member `primary` may hide under the unvouched space — owes target_table.indexes")
		}
	})
}
