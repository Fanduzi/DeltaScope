// Package audit pins the T11 foreign-key ordered-state transitions:
// a proven add merges the constraint member and the implicit supporting
// index only when nothing already covers its local columns (bare TiDB
// index name stays unproven; bare MySQL takes the first column); a bare
// add keeps the member name unproven — on CREATE-derived shapes too; a
// proven duplicate name is the native 1826 refusal; a missing member
// column or absent table invalidates; a proven drop removes only the
// constraint member while its supporting index survives; and a
// still-needed index drop is the native 1553 refusal.
// input: seeded batch entries plus parsed ALTER statements
// output: post-state entries whose constraint/index membership and
// name-space provability mirror native semantics
// pos: T11 FK ordered-state unit coverage under batch state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11fkShape returns a vouched table carrying columns a,c and a proven
// empty member space — the baseline an add must extend honestly.
func t11fkShape(indexes ...spec.Index) *spec.TableSnapshot {
	return t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	}, indexes...)
}

// t11fkParent returns a proven parent table carrying the referenced
// columns — the premise an add needs before it may publish a member.
func t11fkParent(columns ...string) *spec.TableSnapshot {
	cols := make([]spec.Column, 0, len(columns))
	for _, name := range columns {
		cols = append(cols, t10r2Col(name, "int", true, 0))
	}
	return t10r2Table("p", cols)
}

// t11Apply mirrors t10r4Apply but lets the batch resolve referenced
// parents through the provider — the referenced-side premise is a real
// provider lookup, not a seeded entry.
func t11Apply(t *testing.T, dialect spec.Dialect, shape *spec.TableSnapshot, parents map[string]*spec.TableSnapshot, sql string) *batchState {
	t.Helper()
	version := "8.0.46"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	provider := &t10Provider{version: version, tables: parents}
	state := newBatchState(dialect, "golden", provider)
	facts, err := provider.LoadInstanceFacts(context.Background(), dialect, "golden")
	if err != nil {
		t.Fatalf("instance facts: %v", err)
	}
	state.instanceFacts = facts
	key := state.keyFor("golden", spec.Table{Name: "t"})
	state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape}
	parsed, err := parseSQL(context.Background(), sql, dialect)
	if err != nil || len(parsed.Statements) != 1 {
		t.Fatalf("parse %q: %v", sql, err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	statements[0].Metadata = &spec.Metadata{Version: t10r4Version(dialect)}
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply %q: %v", sql, err)
	}
	return state
}

func t11Entry(state *batchState, table string) *batchTableEntry {
	return state.entries[state.keyFor("golden", spec.Table{Name: table})]
}

// t11fkConstraint is the proven stored member the provider load or an
// earlier add would publish.
func t11fkConstraint(name string, columns, refColumns []string) spec.Constraint {
	return spec.Constraint{
		Type:              "foreign_key",
		Name:              name,
		Columns:           columns,
		ReferencedTable:   "p",
		ReferencedColumns: refColumns,
	}
}

func t11fkIndex(shape *spec.TableSnapshot, name string) *spec.Index {
	if shape == nil {
		return nil
	}
	return shape.FindProvenIndex(name)
}

func t11fkMember(shape *spec.TableSnapshot, name string) *spec.Constraint {
	if shape == nil {
		return nil
	}
	return shape.FindForeignKeyConstraint(name)
}

// An authored add on a vouched space lands natively: the constraint
// member keeps its declared facts and, because no existing member
// covers (c), the server creates the implicit supporting index named
// after the authored constraint name.
func TestT11AddForeignKeyPublishesMemberAndImplicitIndex(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		state := t11Apply(t, dialect, t11fkShape(),
			map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);")
		entry := t11Entry(state, "t")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("%s: expected present entry, got %#v", dialect, entry)
		}
		member := t11fkMember(entry.shape, "fk_c")
		if member == nil {
			t.Fatalf("%s: fk_c member missing from post-state", dialect)
		}
		if len(member.Columns) != 1 || member.Columns[0] != "c" ||
			member.ReferencedTable != "p" || len(member.ReferencedColumns) != 1 ||
			member.ReferencedColumns[0] != "id" {
			t.Fatalf("%s: fk_c member facts wrong: %+v", dialect, member)
		}
		idx := t11fkIndex(entry.shape, "fk_c")
		if idx == nil || len(idx.Columns) != 1 || idx.Columns[0] != "c" {
			t.Fatalf("%s: implicit supporting index fk_x(c) missing: %+v", dialect, entry.shape.Indexes)
		}
	}
}

// When a member already covers the local columns, the server reuses it —
// no implicit index member may be invented.
func TestT11AddForeignKeyCoveredPublishesNoIndex(t *testing.T) {
	t.Parallel()
	shape := t11fkShape(spec.Index{Name: "c_idx", Kind: spec.IndexKindSecondary, Columns: []string{"c"}})
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, shape,
		map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if t11fkMember(entry.shape, "fk_c") == nil {
		t.Fatal("fk_c member missing")
	}
	if len(entry.shape.Indexes) != 1 || entry.shape.Indexes[0].Name != "c_idx" {
		t.Fatalf("implicit index fabricated on covered columns: %+v", entry.shape.Indexes)
	}
}

// A bare ADD FOREIGN KEY lands natively under a server-derived catalog
// name the statement never authored — the member stays in the collection
// but its name is unproven, and the implicit index derives its name from
// the first local column.
func TestT11AddBareForeignKeyKeepsMemberNameUnproven(t *testing.T) {
	t.Parallel()
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(),
		map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
		"ALTER TABLE t ADD FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 1 {
		t.Fatalf("expected one constraint member, got %+v", entry.shape.Constraints)
	}
	member := entry.shape.Constraints[0]
	if member.Type != "foreign_key" || !member.NameUnknown || member.Name != "" {
		t.Fatalf("bare member must keep unproven empty name: %+v", member)
	}
	idx := t11fkIndex(entry.shape, "c")
	if idx == nil || len(idx.Columns) != 1 || idx.Columns[0] != "c" {
		t.Fatalf("derived index must take the first local column name: %+v", entry.shape.Indexes)
	}
}

// A proven member already holding the authored name is the native 1826
// refusal — a provable no-op, not a merge.
func TestT11AddForeignKeyDuplicateNameIsNoOp(t *testing.T) {
	t.Parallel()
	shape := t11fkShape()
	shape.Constraints = []spec.Constraint{t11fkConstraint("fk_c", []string{"a"}, []string{"id"})}
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, shape,
		map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 1 || len(entry.shape.Constraints[0].Columns) != 1 ||
		entry.shape.Constraints[0].Columns[0] != "a" {
		t.Fatalf("duplicate add mutated proven member: %+v", entry.shape.Constraints)
	}
	if len(entry.shape.Indexes) != 0 {
		t.Fatalf("refused add published an index: %+v", entry.shape.Indexes)
	}
}

// An authored name under an unvouched member-name space cannot rule out a
// hidden duplicate — the constraint collection unvouches instead of
// appending a member the statement may never have landed.
func TestT11AddForeignKeyUnvouchedNameSpaceUnvouchesSet(t *testing.T) {
	t.Parallel()
	shape := t11fkShape()
	hidden := t11fkConstraint("", []string{"a"}, []string{"id"})
	hidden.NameUnknown = true
	shape.Constraints = []spec.Constraint{hidden}
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, shape,
		map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
		"ALTER TABLE t ADD CONSTRAINT fk_new FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if !entry.shape.ConstraintsUnknown {
		t.Fatal("unvouched name space must unvouch the constraint set")
	}
	if t11fkMember(entry.shape, "fk_new") != nil {
		t.Fatal("unprovable add must not append a member")
	}
}

// An add whose member column is not on the vouched column set cannot have
// landed — the entry tombstones rather than publishing a half-state.
func TestT11AddForeignKeyMissingColumnInvalidates(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectMySQL, t11fkShape(),
		"ALTER TABLE t ADD CONSTRAINT fk_x FOREIGN KEY (nope) REFERENCES p(id);")
	if entry == nil || entry.state != tableUnknown {
		t.Fatalf("missing-column add must tombstone, got %#v", entry)
	}
}

// A drop on a proven name removes only the constraint member — the
// supporting index, explicit or implicit, survives natively.
func TestT11DropForeignKeyKeepsSupportingIndex(t *testing.T) {
	t.Parallel()
	shape := t11fkShape(spec.Index{Name: "fk_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}})
	shape.Constraints = []spec.Constraint{t11fkConstraint("fk_c", []string{"c"}, []string{"id"})}
	entry := t10r4Apply(t, spec.DialectMySQL, shape,
		"ALTER TABLE t DROP FOREIGN KEY fk_c;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 0 {
		t.Fatalf("fk_c member not removed: %+v", entry.shape.Constraints)
	}
	if t11fkIndex(entry.shape, "fk_c") == nil {
		t.Fatal("supporting index must survive the constraint drop")
	}
}

// A drop name that provably misses under a vouched space is the native
// 1091 refusal — a provable no-op, never a fabricated removal.
func TestT11DropForeignKeyMissingNameIsNoOp(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectMySQL, t11fkShape(),
		"ALTER TABLE t DROP FOREIGN KEY fk_x;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 0 || entry.shape.ConstraintsUnknown {
		t.Fatalf("proven-missing drop mutated the set: %+v", entry.shape.Constraints)
	}
}

// A drop name under an unvouched member-name space may have bound a
// hidden member — the surviving set is unprovable.
func TestT11DropForeignKeyUnvouchedNameSpaceUnvouchesSet(t *testing.T) {
	t.Parallel()
	shape := t11fkShape()
	hidden := t11fkConstraint("", []string{"c"}, []string{"id"})
	hidden.NameUnknown = true
	shape.Constraints = []spec.Constraint{hidden}
	entry := t10r4Apply(t, spec.DialectMySQL, shape,
		"ALTER TABLE t DROP FOREIGN KEY fk_x;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if !entry.shape.ConstraintsUnknown {
		t.Fatal("unvouched-name drop must unvouch the constraint set")
	}
}

// Dropping the sole covering index of a proven foreign key is the native
// 1553 refusal — a provable no-op; the member survives.
func TestT11DropNeededIndexIsProvableRefusal(t *testing.T) {
	t.Parallel()
	shape := t11fkShape(spec.Index{Name: "fk_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}})
	shape.Constraints = []spec.Constraint{t11fkConstraint("fk_c", []string{"c"}, []string{"id"})}
	entry := t10r4Apply(t, spec.DialectMySQL, shape,
		"ALTER TABLE t DROP INDEX fk_c;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if t11fkIndex(entry.shape, "fk_c") == nil {
		t.Fatal("needed supporting index must survive the refused drop")
	}
}

// When another member still covers the foreign key's columns, the drop
// lands and only that member leaves.
func TestT11DropCoveredIndexRemovesMember(t *testing.T) {
	t.Parallel()
	shape := t11fkShape(
		spec.Index{Name: "fk_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		spec.Index{Name: "c_extra", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	)
	shape.Constraints = []spec.Constraint{t11fkConstraint("fk_c", []string{"c"}, []string{"id"})}
	entry := t10r4Apply(t, spec.DialectMySQL, shape,
		"ALTER TABLE t DROP INDEX c_extra;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if t11fkIndex(entry.shape, "c_extra") != nil {
		t.Fatal("unneeded index member must leave")
	}
	if t11fkIndex(entry.shape, "fk_c") == nil {
		t.Fatal("covering member must stay")
	}
}

// Ordered add-then-drop net effect: the constraint member leaves but the
// implicit supporting index it created stays — natively DROP FOREIGN KEY
// never removes the index.
func TestT11OrderedAddThenDropKeepsDerivedIndex(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t11fkShape(),
		"p": t11fkParent("id"),
	}}
	shape := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id); "+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;")
	if shape == nil {
		t.Fatal("expected present post-state")
	}
	if len(shape.Constraints) != 0 {
		t.Fatalf("constraint member must be gone: %+v", shape.Constraints)
	}
	if t11fkIndex(shape, "fk_c") == nil {
		t.Fatal("implicit index must survive the ordered add-then-drop")
	}
}

// A composite add preserves the authored column and referenced-column
// pairing in order — the second target is as much a fact as the first.
func TestT11CompositeForeignKeyKeepsOrderedPairs(t *testing.T) {
	t.Parallel()
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(),
		map[string]*spec.TableSnapshot{"p": t11fkParent("y", "z")},
		"ALTER TABLE t ADD CONSTRAINT fk_bx FOREIGN KEY (a, c) REFERENCES p (y, z) ON DELETE CASCADE ON UPDATE SET NULL;"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	member := t11fkMember(entry.shape, "fk_bx")
	if member == nil {
		t.Fatal("composite member missing")
	}
	if len(member.Columns) != 2 || member.Columns[0] != "a" || member.Columns[1] != "c" ||
		len(member.ReferencedColumns) != 2 || member.ReferencedColumns[0] != "y" ||
		member.ReferencedColumns[1] != "z" {
		t.Fatalf("ordered column pairs not preserved: %+v", member)
	}
	if member.OnDelete != "cascade" || member.OnUpdate != "set_null" {
		t.Fatalf("referential actions not modeled: %+v", member)
	}
	idx := t11fkIndex(entry.shape, "fk_bx")
	if idx == nil || len(idx.Columns) != 2 || idx.Columns[0] != "a" || idx.Columns[1] != "c" {
		t.Fatalf("implicit index must cover the composite columns in order: %+v", entry.shape.Indexes)
	}
}

// An add whose parent is proven absent can never have landed — the child
// is provably unchanged, and no member may be published.
func TestT11AddForeignKeyAbsentParentIsProvableRefusal(t *testing.T) {
	t.Parallel()
	// The provider answers Exists=false for a table absent from the map —
	// the same fact a live load produces for a missing parent.
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(), nil,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 0 || entry.shape.ConstraintsUnknown {
		t.Fatalf("refused add must keep the proven empty set: %+v", entry.shape.Constraints)
	}
	if len(entry.shape.Indexes) != 0 || entry.shape.IndexesUnknown {
		t.Fatalf("refused add must not publish a supporting index: %+v", entry.shape.Indexes)
	}
}

// An add whose referenced column is proven missing on a present parent is
// equally refused — the child stays exactly as proven.
func TestT11AddForeignKeyMissingReferencedColumnIsProvableRefusal(t *testing.T) {
	t.Parallel()
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(),
		map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(nope);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if len(entry.shape.Constraints) != 0 {
		t.Fatalf("refused add appended a member: %+v", entry.shape.Constraints)
	}
}

// A parent the batch cannot prove leaves the landing unprovable — the
// constraint set unvouches rather than claiming a member that may not
// exist.
func TestT11AddForeignKeyUnknownParentUnvouchesSet(t *testing.T) {
	t.Parallel()
	state := t11Apply(t, spec.DialectMySQL, t11fkShape(), nil,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);")
	// Seed the parent as unknown and re-apply the add directly: a never-
	// answered parent leaves the outcome unprovable.
	parentKey := state.keyFor("golden", spec.Table{Name: "p"})
	state.entries[parentKey] = &batchTableEntry{state: tableUnknown}
	shape := t11fkShape()
	key := state.keyFor("golden", spec.Table{Name: "t"})
	state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape}
	parsed, err := parseSQL(context.Background(),
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	statements[0].Metadata = &spec.Metadata{Version: t10r4Version(spec.DialectMySQL)}
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}
	entry := state.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if !entry.shape.ConstraintsUnknown {
		t.Fatal("unknown parent must unvouch the constraint set")
	}
	if len(entry.shape.Constraints) != 0 {
		t.Fatalf("unprovable add must not append a member: %+v", entry.shape.Constraints)
	}
}

// A self-referencing foreign key answers the referenced premise against
// the child's own pre-transition shape.
func TestT11AddForeignKeySelfReferenceUsesOwnShape(t *testing.T) {
	t.Parallel()
	entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(), nil,
		"ALTER TABLE t ADD CONSTRAINT fk_self FOREIGN KEY (c) REFERENCES t(a);"), "t")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("expected present entry, got %#v", entry)
	}
	if t11fkMember(entry.shape, "fk_self") == nil {
		t.Fatal("self-referencing member missing — own shape must answer the premise")
	}
}

// A CREATE-declared bare foreign key authors no constraint name — the
// server derives ibfk_N / fk_N — so the derived member keeps the same
// unproven-name marker an ALTER-side bare add does. Dropping the derived
// name afterwards cannot bind a hidden member and unvouches the set
// rather than firing a proven-missing answer the server would land.
func TestT11CreateDerivedBareForeignKeyKeepsNameUnproven(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	shape := t10r4PostState(t, spec.DialectMySQL, provider,
		"CREATE TABLE t (a INT, c INT, FOREIGN KEY (c) REFERENCES p(id)); "+
			"ALTER TABLE t DROP FOREIGN KEY t_ibfk_1;")
	if shape == nil {
		t.Fatal("expected a present post-state shape")
	}
	if len(shape.Constraints) != 1 || shape.Constraints[0].Type != "foreign_key" {
		t.Fatalf("bare create must keep one fk member, got %+v", shape.Constraints)
	}
	if !shape.Constraints[0].NameUnknown {
		t.Fatalf("bare create-derived fk member must stay name-unproven: %+v", shape.Constraints[0])
	}
	if !shape.ConstraintsUnknown {
		t.Fatal("dropping an unproven-derived name must unvouch the constraint set, not fabricate absence")
	}
}

// TiDB pairs a bare foreign key's server-derived fk_<n> constraint name
// with an equally derived index name — the implicit member publishes
// with its name unproven rather than a guessed numeral. MySQL takes the
// first local column name instead, which stays provable when free.
func TestT11AddBareForeignKeyImplicitIndexNameByDialect(t *testing.T) {
	t.Parallel()
	t.Run("tidb keeps index name unproven", func(t *testing.T) {
		t.Parallel()
		entry := t11Entry(t11Apply(t, spec.DialectTiDB, t11fkShape(),
			map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
			"ALTER TABLE t ADD FOREIGN KEY (c) REFERENCES p(id);"), "t")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("expected present entry, got %#v", entry)
		}
		var implicit *spec.Index
		for i := range entry.shape.Indexes {
			if len(entry.shape.Indexes[i].Columns) == 1 && entry.shape.Indexes[i].Columns[0] == "c" {
				implicit = &entry.shape.Indexes[i]
			}
		}
		if implicit == nil {
			t.Fatalf("implicit supporting index member missing: %+v", entry.shape.Indexes)
		}
		if !implicit.NameUnknown {
			t.Fatalf("TiDB bare-add index name is server-derived — must stay unproven: %+v", implicit)
		}
	})
	t.Run("mysql takes first column name", func(t *testing.T) {
		t.Parallel()
		entry := t11Entry(t11Apply(t, spec.DialectMySQL, t11fkShape(),
			map[string]*spec.TableSnapshot{"p": t11fkParent("id")},
			"ALTER TABLE t ADD FOREIGN KEY (c) REFERENCES p(id);"), "t")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("expected present entry, got %#v", entry)
		}
		idx := t11fkIndex(entry.shape, "c")
		if idx == nil || idx.NameUnknown || len(idx.Columns) != 1 || idx.Columns[0] != "c" {
			t.Fatalf("MySQL bare-add index must take first-column name: %+v", entry.shape.Indexes)
		}
	})
}
