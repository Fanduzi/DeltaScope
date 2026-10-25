// Package ddl verifies the T10-R4 spec-review corrections to the shared
// name-keyed index existence predicates: an unproven projected member
// name can never assert existence, so a `primary` target resolves through
// the primary-key identity facts and the binding tri-state instead of
// binding the stale projection.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t10r4Statement(dialect spec.Dialect) spec.Statement {
	return spec.Statement{Dialect: dialect}
}

func t10r4Shape(indexes ...spec.Index) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:  true,
		Indexes: indexes,
	}
}

// Pinned defect (spec review): a member whose projected name reads
// `primary` but whose stored name is unproven must never assert
// existence. On MySQL the reserved name can only be the PK identity — a
// provably absent PK plus an unprovable member name means the target is
// confirmed absent. On TiDB the member may really own the name, so the
// binding stays ambiguous and the predicate suppresses.
func TestT10R4UnprovenProjectedPrimaryCannotBindExists(t *testing.T) {
	t.Parallel()
	member := spec.Index{Name: "primary", Kind: spec.IndexKindSecondary,
		Columns: []string{"c"}, NameUnknown: true}

	t.Run("mysql proven absent", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r4Shape(member)
		snapshot.IndexNamesUnknown = true
		stmt := t10r4Statement(spec.DialectMySQL)
		if dropIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("MySQL: an unproven projected `primary` must not assert existence — the PK is provably absent")
		}
		if alterIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("MySQL: unproven projected name must not answer the alter-index target")
		}
		if renameIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("MySQL: unproven projected name must not answer the rename source")
		}
	})

	t.Run("tidb ambiguous suppresses", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r4Shape(member)
		snapshot.IndexNamesUnknown = true
		stmt := t10r4Statement(spec.DialectTiDB)
		if !dropIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("TiDB: a member `primary` may own the name — absence is unprovable, suppress")
		}
		if !alterIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("TiDB: ambiguous binding must suppress the absence answer")
		}
	})
}

// The proven-name binding works in the positive direction too: a member
// whose stored name is provable binds a name-keyed target on TiDB, and a
// non-`primary` target under a fully vouched name space binds normally.
func TestT10R4ProvenNameBindsExists(t *testing.T) {
	t.Parallel()
	t.Run("tidb member primary owns the name", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r4Shape(spec.Index{Name: "primary",
			Kind: spec.IndexKindSecondary, Columns: []string{"c"}})
		stmt := t10r4Statement(spec.DialectTiDB)
		if !dropIndexTargetExists(stmt, snapshot, "primary") {
			t.Fatalf("a proven member `primary` owns the name — the target exists")
		}
	})
	t.Run("ordinary member under vouched names", func(t *testing.T) {
		t.Parallel()
		snapshot := t10r4Shape(
			spec.Index{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
			spec.Index{Name: "hidden", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, NameUnknown: true},
		)
		snapshot.IndexNamesUnknown = true
		stmt := t10r4Statement(spec.DialectMySQL)
		if !snapshotHasIndex(stmt, snapshot, "s") {
			t.Fatalf("a proven member name binds under an unvouched name space")
		}
		if snapshotHasIndex(stmt, snapshot, "hidden") {
			t.Fatalf("an unproven projected name must never bind")
		}
	})
}
