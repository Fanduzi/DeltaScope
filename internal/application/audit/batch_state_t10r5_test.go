// Package audit verifies the T10-R5 actual-stored-name binding: a TiDB
// primary key renamed off `primary` keeps answering every name-keyed
// successor under its real catalog name (visibility, drop, re-rename), the
// freed `primary` name stops binding the PK identity, DROP PRIMARY KEY
// inherits the same member-first name routing DROP INDEX already used
// (native: dropIndex("PRIMARY") — verified on v8.5.0), and a proven 1061
// collision against the PK object's stored name preserves the shape.
// input: seeded ordered-state entries carrying renamed-PK and member-
// `primary` fixture shapes, applied through the same parse→extract→apply
// chain production uses
// output: post-state shape assertions per successor statement
// pos: application-layer regression for issue #89 T10-R5; native pins live
// in docs/verification/t10 native-probe records
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r5RenamedPKShape models the provider-reloaded post-rename state: the
// logical primary key is a real index object whose stored catalog name is
// `pk2` (RENAME INDEX `PRIMARY` TO pk2 verified on TiDB v8.5.0 — the
// constraint identity stays PRIMARY, the object name does not). The member
// set is fully vouched.
func t10r5RenamedPKShape() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "varchar(32)", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Name: "pk2", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "nonclustered"}
	shape.Indexes = []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	return shape
}

// t10r5HandleMemberPrimaryShape models a PKIsHandle table carrying a member
// literally named `primary` beside the synthesized PRIMARY catalog row —
// legal on TiDB (RENAME INDEX s TO `primary` verified rc=0 on v8.5.0).
func t10r5HandleMemberPrimaryShape() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	shape.PrimaryKeyIsHandle = true
	shape.PrimaryKeyClustered = true
	shape.Indexes = []spec.Index{{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	return shape
}

// A TiDB PK renamed to pk2 keeps binding name-keyed successors by its
// actual stored name: VISIBLE is a proven no-op on the real object,
// INVISIBLE is refused (3522 — the statement fails so the shape survives),
// DROP INDEX pk2 publishes the drop (the clustered refusal class applies
// only to the `primary` name — verified rc=0 on a common-handle shape),
// and RENAME moves the object's stored name again.
func TestT10R5TiDBRenamedPKSuccessors(t *testing.T) {
	t.Parallel()
	t.Run("alter visible on the actual name is a proven no-op", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
			"ALTER TABLE t ALTER INDEX pk2 VISIBLE;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("VISIBLE on the renamed PK object must keep the shape, got %+v", entry)
		}
		if entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk2" {
			t.Fatalf("the PK object must keep its actual stored name pk2, got %+v", entry.shape.PrimaryKey)
		}
	})
	t.Run("alter invisible is refused and unvouches", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
			"ALTER TABLE t ALTER INDEX pk2 INVISIBLE;")
		if entry != nil && entry.state == tablePresent {
			t.Fatalf("a proven 3522 refusal follows the name-keyed unvouch convention, got %+v", entry.shape)
		}
	})
	t.Run("drop index on the actual name clears the PK", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
			"DROP INDEX pk2 ON t;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("DROP INDEX pk2 is a proven PK drop, got %+v", entry)
		}
		if entry.shape.HasPrimaryKey() {
			t.Fatalf("the renamed PK object was dropped, got %+v", entry.shape.PrimaryKey)
		}
		if !entry.shape.HasIndex("s") {
			t.Fatalf("the ordinary member must survive the PK drop, got %+v", entry.shape.Indexes)
		}
	})
	t.Run("rename on the actual name moves the PK object's stored name", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
			"ALTER TABLE t RENAME INDEX pk2 TO pk3;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil ||
			entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk3" {
			t.Fatalf("the PK object must carry its new stored name pk3, got %+v", entry)
		}
	})
	t.Run("drop index on a clustered renamed name still clears the PK", func(t *testing.T) {
		t.Parallel()
		// Common-handle PK renamed off `primary`: the 8200 clustered
		// refusal locks only the `primary` name — a name-keyed drop on the
		// object's actual stored name removes even the clustered object
		// (verified natively: DROP INDEX pk2 rc=0, pk_left=0).
		shape := t10r5RenamedPKShape()
		shape.PrimaryKey.PrimaryKeyType = "clustered"
		shape.PrimaryKeyClustered = true
		entry := t10r4Apply(t, spec.DialectTiDB, shape,
			"DROP INDEX pk2 ON t;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("DROP INDEX pk2 on a common-handle object is a proven drop, got %+v", entry)
		}
		if entry.shape.HasPrimaryKey() {
			t.Fatalf("clustered storage does not protect the actual-name drop, got %+v", entry.shape.PrimaryKey)
		}
	})
}

// Once the PK object is renamed off `primary`, the freed name binds
// nothing — every `primary`-keyed statement fails natively (1176 ALTER /
// 1091 DROP, verified), so the ordered state treats the broken premise
// the same as a missing member name. A member can then legally take the
// freed name even while the PK object survives under pk3.
func TestT10R5TiDBPrimaryNameFreedAfterRename(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{name: "alter primary visible", sql: "ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;"},
		{name: "drop index primary", sql: "DROP INDEX `PRIMARY` ON t;"},
		{name: "drop primary key", sql: "ALTER TABLE t DROP PRIMARY KEY;"},
		{name: "rename primary source", sql: "ALTER TABLE t RENAME INDEX `PRIMARY` TO x;"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(), tc.sql)
			if entry != nil && entry.state == tablePresent {
				t.Fatalf("%s on the freed name fails natively — the entry must unvouch, got %+v", tc.sql, entry.shape)
			}
		})
	}
	t.Run("member takes the freed primary name", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
			"ALTER TABLE t RENAME INDEX s TO `primary`;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("a member legally takes the freed primary name, got %+v", entry)
		}
		if member := entry.shape.FindProvenIndex("primary"); member == nil {
			t.Fatalf("the renamed member must own the primary name, got %+v", entry.shape.Indexes)
		}
		if entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk2" {
			t.Fatalf("the PK object stays pk2 while a member owns primary, got %+v", entry.shape.PrimaryKey)
		}
	})
}

// DROP PRIMARY KEY routes through the same dropIndex("PRIMARY") name
// lookup DROP INDEX uses (upstream anchor: pkg/ddl/executor.go L1763-1766
// on TiDB v8.5.0): a member literally named `primary` provably owns the
// name, so the statement drops the member and the logical PK survives.
func TestT10R5TiDBDropPrimaryKeyHitsMemberPrimary(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectTiDB, t10r5HandleMemberPrimaryShape(),
		"ALTER TABLE t DROP PRIMARY KEY;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the member drop must keep the entry present, got %+v", entry)
	}
	if !entry.shape.HasPrimaryKey() {
		t.Fatalf("the handle PK survives the member-first drop, got %+v", entry.shape.PrimaryKey)
	}
	if entry.shape.FindProvenIndex("primary") != nil || len(entry.shape.Indexes) != 0 {
		t.Fatalf("the member named primary was dropped, got %+v", entry.shape.Indexes)
	}
}

// A pk-less table's member `primary` owns the name outright — the same
// name-keyed DROP PRIMARY KEY removes it (verified rc=0 on v8.5.0).
func TestT10R5TiDBDropPrimaryKeyMemberWithoutPK(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{t10r2Col("c", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t DROP PRIMARY KEY;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the member drop must keep the entry present, got %+v", entry)
	}
	if entry.shape.FindProvenIndex("primary") != nil || len(entry.shape.Indexes) != 0 {
		t.Fatalf("the member named primary was dropped, got %+v", entry.shape.Indexes)
	}
}

// An authored member add colliding with the PK object's actual stored name
// refuses 1061 — the statement fails before touching the table, so the
// shape survives instead of appending a name-twin member.
func TestT10R5TiDBAddIndexCollidesWithPKObjectName(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectTiDB, t10r5RenamedPKShape(),
		"ALTER TABLE t ADD INDEX pk2 (id);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the proven 1061 must keep the shape, got %+v", entry)
	}
	if len(entry.shape.Indexes) != 1 || entry.shape.Indexes[0].Name != "s" {
		t.Fatalf("no member may be appended on a refused add, got %+v", entry.shape.Indexes)
	}
}

// t10r5FailingProvider answers every table probe with a transport-class
// failure — the stand-in for the catalog connection a clean read never
// produced.
type t10r5FailingProvider struct{ err error }

func (p *t10r5FailingProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}, nil
}

func (p *t10r5FailingProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, p.err
}

// R5-B audit seam: an actual provider failure must surface as an audit
// error even with every rule disabled — issue #79 forbids degrading it
// into an ordinary evidence gap.
func TestT10R5ProviderErrorSurvivesRulesOff(t *testing.T) {
	t.Parallel()
	_, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE t ALTER INDEX s VISIBLE;",
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		ConfigPath:       t10Policy(t),
		MetadataProvider: &t10r5FailingProvider{err: errors.New("connection reset by peer")},
	})
	if err == nil {
		t.Fatal("a provider failure must surface as an audit error, never a withheld shape")
	}
}

// t10r5AmbiguousAttributionShape models the provider reload of a table whose
// PK constraint is vouched by TABLE_CONSTRAINTS while its STATISTICS object
// attribution stays unproven (a renamed PK object beside a second unique
// fold): a name-unknown primary placeholder coexists with kind-demoted
// candidate members.
func t10r5AmbiguousAttributionShape() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "varchar(32)", true, 0),
		t10r2Col("b", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, NameUnknown: true}
	shape.PrimaryKeyHandleUnknown = true
	shape.PrimaryKeyClusteredUnknown = true
	shape.Indexes = []spec.Index{
		{Name: "pk2", Kind: spec.IndexKindUnknown, Columns: []string{"id"}},
		{Name: "uq1", Kind: spec.IndexKindUnknown, Columns: []string{"b"}},
		{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	}
	return shape
}

// A2 public seam: under a withheld attribution the exists rule must still
// answer name-keyed member targets, while a drop on a unique-capable
// candidate fold unvouches PK presence — the constraint may have owned the
// dropped object. Demoted candidates must never feed member comparisons as
// proven UNIQUEs.
func TestT10R5TiDBAmbiguousAttributionThroughAudit(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r5AmbiguousAttributionShape()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP INDEX pk2; ALTER TABLE t ALTER INDEX s VISIBLE;",
		provider, t10ExistsPolicy(t))
	if len(result.Statements) != 2 {
		t.Fatalf("both statements must audit, got %+v", result.Statements)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("pk2 is a proven member name — no fabricated absent finding, got %+v", result.Statements[0].Findings)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("s stays a proven member after the drop, got %+v", result.Statements[1].Findings)
	}
	enriched := enrichA3(t,
		"ALTER TABLE t DROP INDEX pk2; ALTER TABLE t ALTER INDEX s VISIBLE;",
		spec.DialectTiDB, provider)
	post := enriched[1].Metadata.TargetTable
	if post == nil {
		t.Fatal("the second statement must observe the post-drop ordered state")
	}
	if !post.PrimaryKeyUnknown || !post.ConstraintsUnknown {
		t.Fatalf("dropping an attribution-candidate fold must unvouch PK presence, got %+v", post)
	}
	if post.PrimaryKey == nil {
		t.Fatal("the vouched-but-unattributed PK placeholder survives the drop")
	}
	if post.FindProvenIndex("pk2") != nil || post.FindProvenIndex("s") == nil || post.FindProvenIndex("uq1") == nil {
		t.Fatalf("member delta must drop pk2 only, got %+v", post.Indexes)
	}
}

// A2 ordered-state seam: the same shape through the batch-state applier keeps
// the PK placeholder but unvouches presence/constraint truth when a candidate
// fold is dropped; a proven secondary member drop leaves attribution intact.
func TestT10R5TiDBAmbiguousAttributionMemberDrop(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectTiDB, t10r5AmbiguousAttributionShape(),
		"ALTER TABLE t DROP INDEX pk2;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("a proven member fold drop publishes, got %+v", entry)
	}
	if !entry.shape.PrimaryKeyUnknown || !entry.shape.ConstraintsUnknown {
		t.Fatalf("pk2 may have been the PK object, got %+v", entry.shape)
	}
	entry2 := t10r4Apply(t, spec.DialectTiDB, t10r5AmbiguousAttributionShape(),
		"ALTER TABLE t DROP INDEX s;")
	if entry2 == nil || entry2.state != tablePresent || entry2.shape == nil {
		t.Fatalf("a proven secondary member drop publishes, got %+v", entry2)
	}
	if entry2.shape.PrimaryKeyUnknown || entry2.shape.ConstraintsUnknown {
		t.Fatalf("a proven secondary fold can never own the PK constraint, got %+v", entry2.shape)
	}
}

// t10r5DropIndexExistsPolicy isolates the drop-index existence rule so the
// `primary` binding's gap answer is not masked by sibling rules.
func t10r5DropIndexExistsPolicy(t *testing.T) string {
	return t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"})
}

// A4 violation (spec review): a member literally named `primary` answers the
// target by itself — unrelated PK-side unknowns must not mint a
// target_table.primary_key gap alongside the resolved member answer.
func TestT10R5TiDBMemberPrimaryOwesNoPrimaryKeyGap(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKeyUnknown = true
	shape.ConstraintsUnknown = true
	shape.Indexes = []spec.Index{
		{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	}
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": shape},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP INDEX `primary`;", provider, t10r5DropIndexExistsPolicy(t))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("a proven member `primary` answers the target — no finding, got %+v", stmt.Findings)
	}
	for _, gap := range stmt.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.primary_key" {
				t.Fatalf("the member binding owed no primary-key fact, got gap %+v", gap)
			}
		}
	}
}

// A4 violation (spec review): a withheld PK-object name under a vouched
// presence suppresses the `primary` answer — the suppression must surface as
// a primary_key gap, never as a clean "checked" result.
func TestT10R5TiDBWithheldPKNameEmitsPrimaryKeyGap(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "varchar(32)", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, Columns: []string{"id"}, NameUnknown: true}
	shape.Indexes = []spec.Index{
		{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	}
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": shape},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP INDEX `primary`;", provider, t10r5DropIndexExistsPolicy(t))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("an unproven PK-object name suppresses the finding, got %+v", stmt.Findings)
	}
	found := false
	for _, gap := range stmt.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.primary_key" {
				found = true
			}
			if fact == "target_table.indexes" {
				t.Fatalf("the member space is vouched — no member-collection fact owed, got %+v", gap)
			}
		}
	}
	if !found {
		t.Fatalf("the suppressed `primary` answer must emit target_table.primary_key, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("a suppressed answer reads as unverified review, got coverage=%s verdict=%s",
			stmt.Coverage.Status, result.Verdict)
	}
}

// R5-A(3) spec-review: an authored CONSTRAINT symbol names the constraint —
// the PK index object is still stored as PRIMARY on both anchors, so a
// `primary` target binds the object rather than a fabricated absence.
func TestT10R5ConstraintSymbolDoesNotRenamePKObject(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
		pkDecl  string
		wantPK  bool // post-drop PK presence (TiDB clustered keeps via 8200)
	}{
		{spec.DialectMySQL, "8.0.46", "CONSTRAINT sym PRIMARY KEY(id)", false},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", "CONSTRAINT sym PRIMARY KEY NONCLUSTERED(id)", false},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			sql := "CREATE TABLE t(id INT, " + tc.pkDecl + ", c INT, KEY s(c)); " +
				"ALTER TABLE t DROP INDEX `PRIMARY`;"
			enriched := enrichA3(t, sql, tc.dialect,
				&t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{}})
			pre := enriched[1].Metadata.TargetTable
			if pre == nil || pre.PrimaryKey == nil {
				t.Fatalf("the declared PK must reach the ordered state, got %+v", pre)
			}
			if pre.PrimaryKey.Name != "primary" {
				t.Fatalf("the symbol must not become the object name, got %q", pre.PrimaryKey.Name)
			}
			result := t10Audit(t, tc.dialect, sql,
				&t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{}},
				t10ExistsPolicy(t))
			stmt := result.Statements[1]
			if len(stmt.Findings) != 0 {
				t.Fatalf("`primary` binds the PK object — a fabricated absence would find, got %+v", stmt.Findings)
			}
		})
	}
}

// An ambiguous `primary` rename unvouches the member name space AND the
// PK object's stored name separately — IndexNamesUnknown covers the
// member names while PrimaryKey.NameUnknown carries the object's own
// fact. A successor DROP INDEX `PRIMARY` must not publish a fabricated
// PK drop on a possibly-freed name (natively 1091); the PK facts stay
// vouched-or-unknown, never a clean fabricated absence.
func TestT10R5AmbiguousPrimaryRenameKeepsPKNameUnproven(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "nonclustered"}
	// An unnamed member keeps the member name space unvouched — the
	// `primary` rename source is then ambiguous (member may own it, or
	// the PK object does).
	shape.Indexes = []spec.Index{
		{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		{Kind: spec.IndexKindSecondary, Columns: []string{"c"}, NameUnknown: true},
	}
	entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t RENAME INDEX `PRIMARY` TO pk2;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil || !entry.shape.IndexNamesUnknown {
		t.Fatalf("the ambiguous `primary` rename must unvouch the stored-name space, got %+v", entry)
	}
	// The successor DROP INDEX `PRIMARY` binds through the unvouched name
	// — it must not read the stale proven `primary` as a PK object.
	next := t10r4Apply(t, spec.DialectTiDB, entry.shape, "ALTER TABLE t DROP INDEX `PRIMARY`;")
	if next != nil && next.state == tablePresent && next.shape != nil &&
		!next.shape.HasPrimaryKey() && !next.shape.PrimaryKeyUnknown {
		t.Fatalf("a possibly-freed `primary` must never fabricate a PK drop, got %+v", next.shape)
	}
}

// A demoted unique-candidate fold (IndexKindUnknown under a withheld PK
// attribution) must not receive a plain-member INVISIBLE publish: if the
// fold is the PK object the flip is natively refused (3522), so the member
// outcome is unprovable and the member set unvouches. VISIBLE keeps both
// native outcomes consistent (member flips, object no-ops visible).
func TestT10R5DemotedCandidateVisibilityWithholds(t *testing.T) {
	t.Parallel()
	shape := func() *spec.TableSnapshot {
		s := t10r2Table("t", []spec.Column{
			t10r2Col("id", "int", true, 0),
			t10r2Col("c", "int", true, 0),
		})
		s.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, Columns: []string{"id"}, NameUnknown: true}
		s.Indexes = []spec.Index{
			{Name: "x", Kind: spec.IndexKindUnknown, Columns: []string{"id"}},
			{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		}
		return s
	}
	t.Run("invisible unvouches the member set", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, shape(), "ALTER TABLE t ALTER INDEX x INVISIBLE;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil || !entry.shape.IndexesUnknown {
			t.Fatalf("a demoted-candidate INVISIBLE outcome is unprovable — the member set must unvouch, got %+v", entry)
		}
	})
	t.Run("visible stays publishable", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, shape(), "ALTER TABLE t ALTER INDEX x VISIBLE;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("VISIBLE on a maybe-PK fold stays consistent, got %+v", entry)
		}
	})
}

// DROP PRIMARY KEY routes member-first on TiDB. When the `primary` member
// is itself a demoted attribution candidate, dropping it may drop the PK
// object — the drop mirrors applyDropIndex: PK presence and constraint
// truth unvouch together.
func TestT10R5DropPrimaryKeyDemotedMemberUnvouchesPK(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, Columns: []string{"id"}, NameUnknown: true}
	shape.Indexes = []spec.Index{
		{Name: "primary", Kind: spec.IndexKindUnknown, Columns: []string{"id"}},
		{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	}
	entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t DROP PRIMARY KEY;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the member-first drop keeps the table, got %+v", entry)
	}
	if entry.shape.HasIndex("primary") {
		t.Fatalf("the named member fold must drop, got %+v", entry.shape.Indexes)
	}
	if !entry.shape.PrimaryKeyUnknown || !entry.shape.ConstraintsUnknown {
		t.Fatalf("dropping a maybe-PK fold must unvouch PK presence and constraint truth, got %+v", entry.shape)
	}
}

// An authored index name collides with a hidden PK object whenever PK
// presence or its stored name is unproven — the add may have refused 1061,
// so the member set unvouches instead of appending a member the statement
// may never have created. This extends the NameUnknown arm to
// PrimaryKeyUnknown.
func TestT10R5AddIndexWithheldPKUnvouchesMembers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"alter add index", "ALTER TABLE t ADD INDEX ix (c);"},
		{"standalone create", "CREATE INDEX ix ON t (c);"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := t10r2Table("t", []spec.Column{
				t10r2Col("id", "int", true, 0),
				t10r2Col("c", "int", true, 0),
			})
			shape.PrimaryKeyUnknown = true
			shape.Indexes = []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
			entry := t10r4Apply(t, spec.DialectMySQL, shape, tc.sql)
			if entry == nil || entry.state != tablePresent || entry.shape == nil || !entry.shape.IndexesUnknown {
				t.Fatalf("an authored add under unproven PK presence must unvouch the member set, got %+v", entry)
			}
		})
	}
}

// A member RENAME INDEX destination must respect the PK object's actual
// stored name — `RENAME INDEX s TO pk2` where the PK object is pk2
// refuses natively (1061) and the member keeps its old name. Under a
// withheld PK stored name the collision is unprovable, so the member's
// name unvouches instead of publishing a rename that may not have landed.
func TestT10R5MemberRenameOntoPKObjectNameRefused(t *testing.T) {
	t.Parallel()
	base := func() *spec.TableSnapshot {
		shape := t10r2Table("t", []spec.Column{
			t10r2Col("id", "int", true, 0),
			t10r2Col("c", "int", true, 0),
		})
		shape.PrimaryKey = &spec.Index{Name: "pk2", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "nonclustered"}
		shape.Indexes = []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
		return shape
	}
	t.Run("proven pk2 destination refuses", func(t *testing.T) {
		t.Parallel()
		entry := t10r4Apply(t, spec.DialectTiDB, base(), "ALTER TABLE t RENAME INDEX s TO pk2;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("the refused rename keeps a present shape, got %+v", entry)
		}
		if got := entry.shape.Indexes[0].Name; got != "s" {
			t.Fatalf("a member renamed onto the PK object's stored name is refused 1061 — the member must stay s, got %q", got)
		}
	})
	t.Run("withheld PK name unvouches the member name", func(t *testing.T) {
		t.Parallel()
		shape := base()
		shape.PrimaryKey.NameUnknown = true
		entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t RENAME INDEX s TO x;")
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("the unprovable rename keeps a present shape, got %+v", entry)
		}
		if len(entry.shape.Indexes) != 1 || !entry.shape.Indexes[0].NameUnknown {
			t.Fatalf("a possibly-colliding destination unvouches the member's name, got %+v", entry.shape.Indexes)
		}
	})
}
