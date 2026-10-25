// Package audit covers the R1-D/R1-E shared foreign-key fact seams through
// the public audit path: referenced-key landing premises, refer-action
// version boundaries, CREATE/ALTER supporting-index parity, unknown
// propagation, and every index-removal dependency entry point.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r1Child returns a proven child shape: every member collection loaded
// and empty, the authored columns vouched — the baseline an add extends.
func t11r1Child(columns ...string) *spec.TableSnapshot {
	cols := make([]spec.Column, 0, len(columns))
	for _, name := range columns {
		cols = append(cols, t10r2Col(name, "int", false, 0))
	}
	return t10r2Table("t", cols)
}

// t11r1KeylessParent returns a proven parent whose member set is vouched
// empty — referenced columns exist but no usable key covers them. The
// premise every anchor refuses natively: 1215 on 5.7, 1822 on 8.0/TiDB,
// 6125 on 8.4.
func t11r1KeylessParent(columns ...string) *spec.TableSnapshot {
	cols := make([]spec.Column, 0, len(columns))
	for _, name := range columns {
		cols = append(cols, t10r2Col(name, "int", true, 0))
	}
	shape := t10r2Table("p", cols)
	shape.Indexes = []spec.Index{}
	return shape
}

// t11r1Apply seeds several present tables under schema "golden" and runs a
// batch of statements through the ordered transitions, mirroring the
// production pre-state → publish order per statement.
func t11r1Apply(t *testing.T, dialect spec.Dialect, seeded map[string]*spec.TableSnapshot, provider *t10Provider, sql string) *batchState {
	t.Helper()
	if provider == nil {
		version := "8.0.46"
		if dialect == spec.DialectTiDB {
			version = "8.0.11-TiDB-v8.5.0"
		}
		provider = &t10Provider{version: version}
	}
	state := newBatchState(dialect, "golden", provider)
	facts, err := provider.LoadInstanceFacts(context.Background(), dialect, "golden")
	if err != nil {
		t.Fatalf("instance facts: %v", err)
	}
	state.instanceFacts = facts
	state.resolvedVersion = t10r4Version(dialect)
	for name, shape := range seeded {
		key := state.keyFor("golden", spec.Table{Name: name})
		state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displayTable: name}
	}
	parsed, err := parseSQL(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for i, statement := range statements {
		statement.Metadata = &spec.Metadata{Version: t10r4Version(dialect)}
		if statement.DDL != nil {
			for _, target := range statement.DDL.TableTargets() {
				snapshot, err := state.preState(context.Background(), "golden", target)
				if err != nil {
					t.Fatalf("preState #%d: %v", i, err)
				}
				statement.Metadata.TargetTable = snapshot
			}
		}
		if err := state.apply(context.Background(), statement); err != nil {
			t.Fatalf("apply #%d: %v", i, err)
		}
	}
	return state
}

func boolPtr(v bool) *bool { return &v }

// TestT11R1DNoUsableParentKeyRefusesAdd pins the shared red case: a proven
// parent with referenced columns but no usable key cannot land the foreign
// key — the add is a native refusal, so the successor DROP FOREIGN KEY
// must still answer proven-missing instead of finding a fabricated member.
func TestT11R1DNoUsableParentKeyRefusesAdd(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			version := "8.0.46"
			if dialect == spec.DialectTiDB {
				version = "8.0.11-TiDB-v8.5.0"
			}
			provider := &t10Provider{
				version: version,
				tables: map[string]*spec.TableSnapshot{
					"t": t11r1Child("c"),
					"p": t11r1KeylessParent("id"),
				},
			}
			result := t10Audit(t, dialect,
				"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"+
					"ALTER TABLE t DROP FOREIGN KEY fk_c;",
				provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
			if len(result.Statements) != 2 {
				t.Fatalf("expected 2 statements, got %d", len(result.Statements))
			}
			drop := result.Statements[1]
			if len(drop.Findings) != 1 {
				t.Fatalf("the add could never land — the drop must report proven-missing, got %+v", drop.Findings)
			}
			if drop.Coverage.Status != report.CoverageComplete {
				t.Fatalf("every premise was proven — no gap owed, got %+v", drop.EvidenceGaps)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("the missing blocker must reject, got %v", result.Verdict)
			}
		})
	}
}

// Usable-key variants: a unique member qualifies everywhere; a plain
// secondary member qualifies on pre-8.4 MySQL and TiDB but is refused by
// 8.4 under observed restrict_fk_on_non_standard_key=ON (6125).
func TestT11R1DReferencedKeyEligibilityByProduct(t *testing.T) {
	t.Parallel()
	parentWith := func(indexes ...spec.Index) *spec.TableSnapshot {
		shape := t11r1KeylessParent("id")
		shape.Indexes = indexes
		return shape
	}
	secondary := spec.Index{Name: "ix_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}}
	unique := spec.Index{Name: "uq_id", Kind: spec.IndexKindUnique, Columns: []string{"id"}}
	fulltext := spec.Index{Name: "ft_id", Kind: spec.IndexKindFulltext, Columns: []string{"id"}}
	addSQL := "ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);" +
		"ALTER TABLE t DROP FOREIGN KEY fk_c;"
	audit := func(provider *t10Provider, dialect spec.Dialect) report.Result {
		return t10Audit(t, dialect, addSQL, provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	}

	t.Run("secondary member accepted on mysql 8.0", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(secondary)}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 0 {
			t.Fatalf("the landed member drops cleanly — no finding, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("secondary member refused on mysql 8.4 restrict on", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(secondary)}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 1 {
			t.Fatalf("6125 refused the add — the drop answers missing, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("secondary member undecidable on mysql 8.4 restrict unread", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.4.10", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(secondary)}}, spec.DialectMySQL)
		if len(result.Statements[1].EvidenceGaps) == 0 {
			t.Fatalf("an unread restrict flag leaves the landing unprovable — gap owed, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("secondary member accepted on tidb", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(secondary)}}, spec.DialectTiDB)
		if len(result.Statements[1].Findings) != 0 {
			t.Fatalf("TiDB accepts non-unique referenced keys — drop lands cleanly, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("unique member accepted everywhere", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(unique)}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 0 {
			t.Fatalf("a unique referenced key qualifies under restrict too, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("fulltext member never qualifies", func(t *testing.T) {
		t.Parallel()
		result := audit(&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parentWith(fulltext)}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 1 {
			t.Fatalf("a fulltext member is never a usable referenced key — drop answers missing, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("non-leftmost member columns refuse", func(t *testing.T) {
		t.Parallel()
		wrongOrder := spec.Index{Name: "ix", Kind: spec.IndexKindSecondary, Columns: []string{"x", "id"}}
		parent := parentWith(wrongOrder)
		parent.Columns = append(parent.Columns, t10r2Col("x", "int", true, 0))
		result := audit(&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parent}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 1 {
			t.Fatalf("id is not the leftmost member of (x,id) — the add refuses, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("leftmost prefix of composite key qualifies", func(t *testing.T) {
		t.Parallel()
		composite := spec.Index{Name: "ix", Kind: spec.IndexKindSecondary, Columns: []string{"id", "x"}}
		parent := parentWith(composite)
		parent.Columns = append(parent.Columns, t10r2Col("x", "int", true, 0))
		result := audit(&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
			"t": t11r1Child("c"), "p": parent}}, spec.DialectMySQL)
		if len(result.Statements[1].Findings) != 0 {
			t.Fatalf("id is the leftmost prefix of (id,x) — the key is usable, got %+v", result.Statements[1].Findings)
		}
	})
}

// Parent member-set unknown: the usable-key answer cannot be read, so the
// add unvouches rather than publishing or refusing.
func TestT11R1DUnvouchedParentKeyPropagates(t *testing.T) {
	t.Parallel()
	parent := t11r1KeylessParent("id")
	parent.IndexesUnknown = true
	parent.Indexes = nil
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t11r1Child("c"), "p": parent}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the add could not be decided — no proven-missing finding, got %+v", drop.Findings)
	}
	if len(drop.EvidenceGaps) == 0 || drop.Coverage.Status == report.CoverageComplete {
		t.Fatalf("an undecidable add owes the constraint-set gap, got %+v / %+v", drop.EvidenceGaps, drop.Coverage)
	}
}

// Refer-action landing: SET NULL against a proven NOT NULL member refuses
// on every anchor; SET DEFAULT refuses pre-8.0 MySQL only.
func TestT11R1DReferActionBoundaries(t *testing.T) {
	t.Parallel()
	t.Run("set null on not-null member refuses", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			version := "8.0.46"
			if dialect == spec.DialectTiDB {
				version = "8.0.11-TiDB-v8.5.0"
			}
			child := t11r1Child("c")
			child.Columns[0].NotNull = true
			result := t10Audit(t, dialect,
				"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id) ON DELETE SET NULL;"+
					"ALTER TABLE t DROP FOREIGN KEY fk_c;",
				&t10Provider{version: version, tables: map[string]*spec.TableSnapshot{
					"t": child, "p": t11fkParent("id")}},
				t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
			if len(result.Statements[1].Findings) != 1 {
				t.Fatalf("%s: 1830-class refusal leaves no member — drop answers missing, got %+v", dialect, result.Statements[1].Findings)
			}
		}
	})
	t.Run("set default refuses on mysql 5.7", func(t *testing.T) {
		t.Parallel()
		result := t10Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id) ON DELETE SET DEFAULT;"+
				"ALTER TABLE t DROP FOREIGN KEY fk_c;",
			&t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{
				"t": t11r1Child("c"), "p": t11fkParent("id")}},
			t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
		if len(result.Statements[1].Findings) != 1 {
			t.Fatalf("5.7 refuses SET DEFAULT — drop answers missing, got %+v", result.Statements[1].Findings)
		}
	})
	t.Run("set default lands on mysql 8.0 and tidb", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			dialect spec.Dialect
			version string
		}{{spec.DialectMySQL, "8.0.46"}, {spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"}} {
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id) ON DELETE SET DEFAULT;"+
					"ALTER TABLE t DROP FOREIGN KEY fk_c;",
				&t10Provider{version: tc.version, tables: map[string]*spec.TableSnapshot{
					"t": t11r1Child("c"), "p": t11fkParent("id")}},
				t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
			if len(result.Statements[1].Findings) != 0 {
				t.Fatalf("%s %s accepts SET DEFAULT — drop lands cleanly, got %+v", tc.dialect, tc.version, result.Statements[1].Findings)
			}
		}
	})
}

// CREATE/ALTER parity on the refusal premise: a declared foreign key
// whose parent has no usable key refuses the CREATE — no success shape,
// no member, no supporting index may publish.
func TestT11R1DCreateTableRefusalParity(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"p": t11r1KeylessParent("id")}}
	shape := t10r4PostState(t, spec.DialectMySQL, provider,
		"CREATE TABLE t (c INT, CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id));")
	if shape != nil {
		t.Fatalf("a refused CREATE publishes no shape, got %+v", shape)
	}
}

// A landed CREATE declares the implicit supporting index the same way an
// ALTER add does — the successor drop binds it by name.
func TestT11R1ECreateDerivedSupportingIndex(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"p": t11fkParent("id")}}
	entry := t11Entry(t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{}, provider,
		"CREATE TABLE t (c INT, CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id));"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;"+
			"ALTER TABLE t DROP INDEX fk_c;"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatal("a landed create keeps a shape")
	}
	if len(entry.shape.Indexes) != 0 {
		t.Fatalf("the derived supporting index survived the key drop, then bound its name and dropped, got %+v", entry.shape.Indexes)
	}
}

// An undecidable add whose member columns are not provably covered may
// still have landed an implicit supporting index — the index set loses
// its vouch alongside the constraint set.
func TestT11R1EUndecidableAddPropagatesIndexUncertainty(t *testing.T) {
	t.Parallel()
	parent := t11r1KeylessParent("id")
	parent.Indexes = nil
	parent.IndexesUnknown = true
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"p": parent}}
	entry := t11Entry(t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": t11r1Child("c")}, provider,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept for assertion, got %+v", entry)
	}
	if !entry.shape.ConstraintsUnknown {
		t.Fatalf("undecidable add unvouches the constraint set, got %+v", entry.shape.Constraints)
	}
	if !entry.shape.IndexesUnknown {
		t.Fatalf("an uncovered undecidable add may have created the supporting index — set must unvouch, got %+v", entry.shape.Indexes)
	}
}

// A provably covered member set reuses its covering index — an
// undecidable add leaves the index facts vouched.
func TestT11R1EUndecidableAddKeepsCoveredIndexFacts(t *testing.T) {
	t.Parallel()
	parent := t11r1KeylessParent("id")
	parent.Indexes = nil
	parent.IndexesUnknown = true
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"p": parent}}
	child := t11r1Child("c")
	child.Indexes = []spec.Index{{Name: "ix_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	entry := t11Entry(t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": child}, provider,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept for assertion, got %+v", entry)
	}
	if !entry.shape.ConstraintsUnknown {
		t.Fatalf("undecidable add unvouches the constraint set, got %+v", entry.shape.Constraints)
	}
	if entry.shape.IndexesUnknown {
		t.Fatalf("a covered member cannot have created an index — set stays vouched, got %+v", entry.shape.Indexes)
	}
}

// The PK object as sole supporter refuses DROP PRIMARY KEY on both
// anchors — the second consecutive drop must still refuse, never report
// the object missing.
func TestT11R1EDropPrimaryKeyNeededByForeignKeyRefuses(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
			child.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"c"}}
			if dialect == spec.DialectTiDB {
				// A clustered PK refuses earlier at the 8200 class — a
				// proven nonclustered object exercises the 1553 arm the
				// probe verified on v8.5.0.
				child.PrimaryKeyClustered = false
				child.PrimaryKeyIsHandle = false
			}
			child.Constraints = []spec.Constraint{{
				Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
				ReferencedTable: "p", ReferencedColumns: []string{"id"},
			}}
			state := t11r1Apply(t, dialect,
				map[string]*spec.TableSnapshot{"t": child}, nil,
				"ALTER TABLE t DROP PRIMARY KEY; ALTER TABLE t DROP PRIMARY KEY;")
			entry := t11Entry(state, "t")
			if entry == nil || entry.shape == nil || !entry.shape.HasPrimaryKey() {
				t.Fatalf("the needed PK object survives both refusals, got %+v", entry)
			}
		})
	}
	// The same refusal binds through every spelling: MySQL DROP INDEX
	// `primary` routes to the PK object, and TiDB's member-first binding
	// reaches a member literally named primary — each sole supporter
	// survives.
	t.Run("mysql drop index primary binds the pk object", func(t *testing.T) {
		t.Parallel()
		child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
		child.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"c"}}
		child.Constraints = []spec.Constraint{{
			Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
			ReferencedTable: "p", ReferencedColumns: []string{"id"},
		}}
		state := t11r1Apply(t, spec.DialectMySQL,
			map[string]*spec.TableSnapshot{"t": child}, nil,
			"ALTER TABLE t DROP INDEX `primary`;")
		entry := t11Entry(state, "t")
		if entry == nil || entry.shape == nil || !entry.shape.HasPrimaryKey() {
			t.Fatalf("the needed PK object survives the member spelling too, got %+v", entry)
		}
	})
	t.Run("tidb member-first primary member refused when needed", func(t *testing.T) {
		t.Parallel()
		child := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
		child.Indexes = []spec.Index{{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
		child.Constraints = []spec.Constraint{{
			Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
			ReferencedTable: "p", ReferencedColumns: []string{"id"},
		}}
		state := t11r1Apply(t, spec.DialectTiDB,
			map[string]*spec.TableSnapshot{"t": child}, nil,
			"ALTER TABLE t DROP PRIMARY KEY;")
		entry := t11Entry(state, "t")
		if entry == nil || entry.shape == nil || len(entry.shape.Indexes) != 1 {
			t.Fatalf("the member-first primary stays while the FK needs it, got %+v", entry)
		}
	})
}

// A loaded child's foreign key binds the parent's sole covering member —
// dropping it on the parent refuses too; an alternate eligible coverer
// frees the drop exactly like the child side.
func TestT11R1EParentSideReferencedKeyDependency(t *testing.T) {
	t.Parallel()
	uq := spec.Index{Name: "uq_id", Kind: spec.IndexKindUnique, Columns: []string{"id"}}
	ix := spec.Index{Name: "ix_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}}
	parent := func(indexes ...spec.Index) *spec.TableSnapshot {
		shape := t11r1KeylessParent("id")
		shape.Indexes = indexes
		return shape
	}
	child := func() *spec.TableSnapshot {
		shape := t11r1Child("c")
		shape.Constraints = []spec.Constraint{{
			Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
			ReferencedTable: "p", ReferencedColumns: []string{"id"},
		}}
		return shape
	}
	t.Run("sole referenced member refuses on parent", func(t *testing.T) {
		t.Parallel()
		state := t11r1Apply(t, spec.DialectMySQL,
			map[string]*spec.TableSnapshot{"t": child(), "p": parent(uq)}, nil,
			"ALTER TABLE p DROP INDEX uq_id;")
		entry := t11Entry(state, "p")
		if entry == nil || entry.shape == nil || len(entry.shape.Indexes) != 1 {
			t.Fatalf("1553 refuses the last covering member, got %+v", entry)
		}
	})
	t.Run("alternate coverer frees the drop", func(t *testing.T) {
		t.Parallel()
		state := t11r1Apply(t, spec.DialectMySQL,
			map[string]*spec.TableSnapshot{"t": child(), "p": parent(uq, ix)}, nil,
			"ALTER TABLE p DROP INDEX uq_id;")
		entry := t11Entry(state, "p")
		if entry == nil || entry.shape == nil {
			t.Fatalf("the freed member drops, got %+v", entry)
		}
		if len(entry.shape.Indexes) != 1 || entry.shape.Indexes[0].Name != "ix_id" {
			t.Fatalf("only uq_id must drop, got %+v", entry.shape.Indexes)
		}
	})
}

// A specialized member kind is never a supporter: a fulltext member on
// the child columns does not free the FK from needing an implicit index.
func TestT11R1EFulltextMemberCannotSupport(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"p": t11fkParent("id")}}
	child := t11r1Child("c")
	child.Indexes = []spec.Index{{Name: "ft_c", Kind: spec.IndexKindFulltext, Columns: []string{"c"}}}
	entry := t11Entry(t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": child}, provider,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept, got %+v", entry)
	}
	if t11fkIndex(entry.shape, "fk_c") == nil {
		t.Fatalf("a fulltext member never supports the key — the implicit index must still land, got %+v", entry.shape.Indexes)
	}
}

// An unproven member kind cannot answer support coverage — the landed
// add unvouches the index set rather than deciding either way.
func TestT11R1EUnknownKindMemberUnvouchesIndexSet(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"p": t11fkParent("id")}}
	child := t11r1Child("c")
	child.Indexes = []spec.Index{{Name: "x", Kind: spec.IndexKindUnknown, Columns: []string{"c"}}}
	entry := t11Entry(t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": child}, provider,
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept, got %+v", entry)
	}
	if !entry.shape.IndexesUnknown {
		t.Fatalf("a maybe-covering member leaves the implicit index undecidable — set unvouches, got %+v", entry.shape.Indexes)
	}
	if t11fkIndex(entry.shape, "fk_c") != nil {
		t.Fatalf("no member may publish while coverage is unprovable, got %+v", entry.shape.Indexes)
	}
}
