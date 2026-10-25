// Package audit pins the T11-R2 existing-binding dependency semantics: a
// self-referencing foreign key's referenced side binds the same table's
// covering member (native 1553 on 8.0/8.4 for both DROP PRIMARY KEY and
// the child supporting index), and a foreign key the provider already
// proves exists keeps binding its covering member even where the current
// configuration would refuse a NEW equivalent reference (8.4 restrict ON
// still refused the drop of an OFF-built non-unique parent index with
// 1553, probed). Binding eligibility for a loaded relationship is a
// physical-coverage question — never the creatability question.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r2SelfRef returns the mandate's self-referencing fixture: a proven
// table whose own foreign key REFERENCES its primary key.
func t11r2SelfRef() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("parent_id", "int", false, 0),
	})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	shape.Indexes = []spec.Index{{Name: "ix_parent", Kind: spec.IndexKindSecondary, Columns: []string{"parent_id"}}}
	shape.Constraints = []spec.Constraint{{
		Type: "foreign_key", Name: "fk_parent", Columns: []string{"parent_id"},
		ReferencedTable: "t", ReferencedColumns: []string{"id"},
	}}
	return shape
}

// A self-referencing foreign key needs its own table's primary key as the
// sole referenced coverer — DROP PRIMARY KEY refuses natively (1553), so
// the object survives and a repeated drop still answers present rather
// than reporting a fabricated missing primary key.
func TestT11R2SelfReferenceSoleCovererRefuses(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			fixture := t11r2SelfRef()
			if dialect == spec.DialectTiDB {
				// A clustered PK refuses earlier at the 8200 class; a
				// nonclustered object exercises the 1553 binding arm.
				fixture.PrimaryKeyClustered = false
				fixture.PrimaryKeyIsHandle = false
			}
			version := "8.0.46"
			if dialect == spec.DialectTiDB {
				version = "8.0.11-TiDB-v8.5.0"
			}
			result := t10Audit(t, dialect,
				"ALTER TABLE t DROP PRIMARY KEY; ALTER TABLE t DROP PRIMARY KEY;",
				&t10Provider{version: version, tables: map[string]*spec.TableSnapshot{"t": fixture}},
				t10Policy(t, t10Rule{id: "ddl.alter.drop_primary_key.exists.require"}))
			for i, stmt := range result.Statements {
				if len(stmt.Findings) != 0 {
					t.Fatalf("stmt %d: the self-bound PK must refuse the drop and stay present — no missing finding, got %#v",
						i, stmt.Findings)
				}
				if stmt.Coverage.Status != report.CoverageComplete {
					t.Fatalf("stmt %d: every premise proven — complete coverage owed, got gaps=%#v", i, stmt.EvidenceGaps)
				}
			}
		})
	}
}

// The same table's child-side supporting member is bound by the self-FK
// too — dropping ix_parent refuses and the member survives.
func TestT11R2SelfReferenceChildSideSoleCoverer(t *testing.T) {
	t.Parallel()
	state := t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": t11r2SelfRef()}, nil,
		"ALTER TABLE t DROP INDEX ix_parent; ALTER TABLE t DROP INDEX ix_parent;")
	entry := t11Entry(state, "t")
	if entry == nil || entry.shape == nil {
		t.Fatal("entry kept for assertion")
	}
	if entry.shape.FindProvenIndex("ix_parent") == nil {
		t.Fatalf("the self-FK's sole supporting member survives both refused drops, got indexes=%+v", entry.shape.Indexes)
	}
}

// An existing foreign key bound to a non-unique parent member keeps
// binding under restrict ON — the creatability answer changed, the
// loaded relationship did not, so the sole-coverer drop still refuses.
func TestT11R2ExistingNonStandardBindingSurvivesRestrict(t *testing.T) {
	t.Parallel()
	parent := t10r2Table("p", []spec.Column{t10r2Col("id", "int", true, 0)})
	parent.Indexes = []spec.Index{{Name: "ix_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}}}
	child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
	child.Indexes = []spec.Index{{Name: "ix_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	child.Constraints = []spec.Constraint{{
		Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
		ReferencedTable: "p", ReferencedColumns: []string{"id"},
	}}
	// The leading ALTER on t loads the child into the ordered state — the
	// parent-side dependency only scans already-loaded entries (the lazy
	// boundary), so the binding must be visible through the public path.
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN d INT; ALTER TABLE p DROP INDEX ix_id; ALTER TABLE p DROP INDEX ix_id;",
		&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
			"p": parent, "t": child,
		}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 0 {
			t.Fatalf("stmt %d: the existing binding refuses the drop natively — the member stays present, got %#v",
				i, stmt.Findings)
		}
		if stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("stmt %d: all facts proven — complete owed, got gaps=%#v", i, stmt.EvidenceGaps)
		}
	}
}

// A surviving alternative coverer frees the member — when the parent
// carries a second member that physically backs the loaded key, the drop
// lands and the member goes.
func TestT11R2ExistingBindingAlternativeCovererFrees(t *testing.T) {
	t.Parallel()
	parent := t10r2Table("p", []spec.Column{t10r2Col("id", "int", true, 0)})
	parent.Indexes = []spec.Index{
		{Name: "ix_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}},
		{Name: "uq_id", Kind: spec.IndexKindUnique, Columns: []string{"id"}},
	}
	child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
	child.Indexes = []spec.Index{{Name: "ix_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	child.Constraints = []spec.Constraint{{
		Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
		ReferencedTable: "p", ReferencedColumns: []string{"id"},
	}}
	state := t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"p": parent, "t": child},
		&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true)},
		"ALTER TABLE p DROP INDEX ix_id;")
	entry := t11Entry(state, "p")
	if entry == nil || entry.shape == nil {
		t.Fatal("entry kept for assertion")
	}
	if entry.shape.FindProvenIndex("ix_id") != nil {
		t.Fatalf("uq_id remains to back the loaded key — ix_id drops, got %+v", entry.shape.Indexes)
	}
	if entry.shape.FindProvenIndex("uq_id") == nil {
		t.Fatalf("the surviving coverer is untouched, got %+v", entry.shape.Indexes)
	}
}

// A loaded child whose constraint space cannot be vouched leaves the
// dependency unanswerable — the parent member's drop unvouches the index
// set rather than claiming the key does not bind.
func TestT11R2UnvouchedChildKeepsDependencyUnprovable(t *testing.T) {
	t.Parallel()
	parent := t10r2Table("p", []spec.Column{t10r2Col("id", "int", true, 0)})
	parent.Indexes = []spec.Index{{Name: "ix_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}}}
	child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
	child.Constraints = nil
	child.ConstraintsUnknown = true
	state := t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"p": parent, "t": child},
		&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true)},
		"ALTER TABLE p DROP INDEX ix_id;")
	entry := t11Entry(state, "p")
	if entry != nil && entry.state == tablePresent && entry.shape != nil && !entry.shape.IndexesUnknown {
		t.Fatalf("a hidden child binding cannot be ruled out — the member must not stay present-vouched, got %+v", entry.shape)
	}
}
