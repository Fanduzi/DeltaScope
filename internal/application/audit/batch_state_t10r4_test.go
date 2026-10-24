// Package audit verifies the T10-R4 ordered-state and fact-propagation
// corrections end to end: name-keyed mutations under an unproven name space
// keep member truth (a matched authored name applies, an unmatched name
// marks the member set unknown instead of retaining stale members);
// PRIMARY resolves through primary-key identity facts without dragging in
// the index name space; product-provably stored names resolve successors;
// a clustered TiDB primary key survives DROP PRIMARY KEY (8200) while
// staying addressable; and a direction-unknown expression member keeps its
// kind, expression identity, multi-valued marks, and aggregate counts.
// input: real MySQL/TiDB batches audited through AuditSQL with stub
// providers carrying vouched member facts and TiDB handle modes, plus
// direct batch-state applications for the mutation mechanics
// output: statement-bound findings, evidence gaps, coverage statuses,
// verdicts, and post-state member/key-part assertions
// pos: application-layer regression for issue #89 T10-R4; gaps live in
// internal/domain/rule/ddl, publication in batch_state*.go
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r4NamesUnknownShape is the fixture shape an unprovable name space
// produces: the derived member's projected name is unproven (NameUnknown),
// while a member merged by an authored ADD/CREATE keeps a provable name —
// the per-member marker under the collection-level IndexNamesUnknown flag.
func t10r4NamesUnknownShape() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.Indexes = append(shape.Indexes,
		// The unnamed member models a create-table derivation whose
		// stored name is unprovable — projected name kept as fact but
		// unusable for name-keyed binding.
		spec.Index{Name: "", Kind: spec.IndexKindUnique, Columns: []string{"a"}, NameUnknown: true},
		// `s` models a member merged by an authored CREATE/ADD INDEX —
		// its name is authored and therefore provable even while the
		// collection-level flag stays unvouched.
		spec.Index{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
	)
	shape.IndexNamesUnknown = true
	return shape
}

// t10r4Version is the validated product identity the production enrichment
// attaches before apply — without it every version-gated advanced-index
// feature (index visibility) marks the transition unresolved and the
// publication tombstones the entry instead of reaching the mutation.
func t10r4Version(dialect spec.Dialect) *spec.VersionIdentity {
	if dialect == spec.DialectTiDB {
		return &spec.VersionIdentity{Product: "tidb", Version: "v8.5.0", ValidatedRange: true, Major: 8, Minor: 5, Patch: 0}
	}
	return &spec.VersionIdentity{Product: "mysql", Version: "8.0.46", ValidatedRange: true, Major: 8, Minor: 0, Patch: 46}
}

// t10r4PostState runs a whole batch through the ordered state machine and
// returns the final snapshot the next statement's Metadata.TargetTable
// would carry — the consumer-visible post-state of the batch.
func t10r4PostState(t *testing.T, dialect spec.Dialect, provider MetadataProvider, sql string) *spec.TableSnapshot {
	t.Helper()
	state := newBatchState(dialect, "golden", provider)
	facts, err := provider.LoadInstanceFacts(context.Background(), dialect, "golden")
	if err != nil {
		t.Fatalf("instance facts: %v", err)
	}
	// Production sets the observed instance facts on the batch state in
	// enrichOrderedStatements — the clustered-mode and alter-primary-key
	// derivations read them there.
	state.instanceFacts = facts
	parsed, err := parseSQL(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for i, statement := range statements {
		// Mirror the production enrichment order: the per-statement
		// pre-state resolves (and caches) the ordered snapshot, the bound
		// TargetTable the statement's rules and advanced assessment read,
		// and only then does the transition publish.
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
	key := state.keyFor("golden", spec.Table{Name: "t"})
	entry := state.entries[key]
	if entry == nil || entry.state != tablePresent {
		return nil
	}
	return entry.shape
}

// t10r4Apply parses, extracts, and applies one statement against a seeded
// batch entry, returning the entry the transition left behind.
func t10r4Apply(t *testing.T, dialect spec.Dialect, shape *spec.TableSnapshot, sql string) *batchTableEntry {
	t.Helper()
	// The provider banner only feeds unobserved-fact answers, so a
	// dialect-matched banner keeps the fixture honest.
	version := "8.0.46"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	provider := &t10Provider{version: version}
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
	return state.entries[key]
}

// --- A: name-keyed mutations keep post-state truth under names-unknown ---

// A CREATE TABLE declaring two members under one authored name natively
// fails (1061) — the name space is unprovable on both dialects while the
// member collection stays known, and the distinct member columns keep the
// statement itself free of ordinary-key findings.
const t10r4CollidingCreate = "CREATE TABLE t (a INT, b INT, c INT, KEY a(a), KEY a(b));"

// Pinned defect (R4-A): a DROP resolving an authored member name must remove
// the member even while other members' stored names are unproven — the
// stale member must never answer a later duplicate check. The provider
// snapshot models a proven authored member beside an unproven derived one;
// the follow-up add cannot rule out a hidden 1061 on the unproven name
// space, so it honestly unvouches the set for the statement after it.
func TestT10R4StandaloneDropResolvesAuthoredMember(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t10r4NamesUnknownShape(),
	}}
	result := t09Audit(t, spec.DialectMySQL,
		"DROP INDEX s ON t;\n"+
			"CREATE INDEX s2 ON t(c);\n"+
			"CREATE INDEX s3 ON t(a);",
		provider, map[string]string{"ddl.index.duplicate.forbid": ""})
	// stmt2 sees the still-vouched post-drop set — s2(c) duplicates no
	// remaining member, so no finding and no gap.
	stmt2 := result.Statements[1]
	if got := t05FindingsByRule(result, 1, "ddl.index.duplicate.forbid"); len(got) != 0 {
		t.Fatalf("no duplicate stands against the surviving set, got %+v", stmt2.Findings)
	}
	if len(stmt2.EvidenceGaps) != 0 || stmt2.Coverage.Status != report.CoverageComplete {
		t.Fatalf("the vouched post-drop set answers stmt2 cleanly, got %+v/%s",
			stmt2.EvidenceGaps, stmt2.Coverage.Status)
	}
	// stmt3 sees the set stmt2's apply unvouched — the maybe-hidden s2
	// collision means membership is unprovable.
	stmt3 := result.Statements[2]
	gaps := t05GapsByRule(result, 2, "ddl.index.duplicate.forbid")
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
		t.Fatalf("unvouched member set must report the indexes gap, got %+v", stmt3.EvidenceGaps)
	}
	if stmt3.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("unvouched member set must hold unverified/review, got %s/%s",
			stmt3.Coverage.Status, result.Verdict)
	}
	next := t10r4PostState(t, spec.DialectMySQL, provider,
		"DROP INDEX s ON t;\nCREATE INDEX s2 ON t(c);")
	if next == nil || !next.IndexesUnknown || !next.IndexNamesUnknown {
		t.Fatalf("the maybe-refused add unvouches the member set, got %+v", next)
	}
	if next.HasIndex("s") || next.HasIndex("s2") {
		t.Fatalf("dropped s must not linger and maybe-absent s2 must not fabricate, got %+v", next.Indexes)
	}
	// The drop itself resolves the proven member while the unproven one
	// survives — the member set stays vouched for that transition.
	mid := t10r4PostState(t, spec.DialectMySQL, provider, "DROP INDEX s ON t;")
	if mid == nil || mid.IndexesUnknown || !mid.IndexNamesUnknown || mid.HasIndex("s") {
		t.Fatalf("a resolved authored drop removes only the member, got %+v", mid)
	}
}

// An unmatched drop under an unproven name space cannot prove which member
// it hit — the member set is unvouched afterwards and the next comparison
// must report the collection gap instead of reading stale members.
func TestT10R4UnmatchedDropMarksSetUnknown(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t09Audit(t, spec.DialectMySQL,
		t10r4CollidingCreate+"\n"+
			"DROP INDEX phantom ON t;\n"+
			"CREATE INDEX s2 ON t(c);",
		provider, map[string]string{"ddl.index.duplicate.forbid": ""})
	stmt3 := result.Statements[2]
	next := t10r4PostState(t, spec.DialectMySQL, provider,
		t10r4CollidingCreate+"\nDROP INDEX phantom ON t;\nCREATE INDEX s2 ON t(c);")
	if next == nil || !next.IndexesUnknown {
		t.Fatalf("an unprovable drop must unvouch the member set, got %+v", next)
	}
	gaps := t05GapsByRule(result, 2, "ddl.index.duplicate.forbid")
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
		t.Fatalf("unvouched member set must report the indexes gap, got %+v", stmt3.EvidenceGaps)
	}
	if stmt3.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("unvouched member set must hold unverified/review, got %s/%s",
			stmt3.Coverage.Status, result.Verdict)
	}
}

// The ALTER-side drop follows the same premise: a matched authored name
// resolves; the post-state stops feeding stale members, and the next add
// under the surviving unproven name space honestly unvouches the set.
func TestT10R4AlterDropIndexResolvesAuthoredMember(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t10r4NamesUnknownShape(),
	}}
	result := t09Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP INDEX s;\n"+
			"ALTER TABLE t ADD INDEX s2 (c);\n"+
			"CREATE INDEX s3 ON t(a);",
		provider, map[string]string{"ddl.index.duplicate.forbid": ""})
	// stmt2 audits against the still-vouched post-drop set — s2(c)
	// duplicates nothing and the add reports no gap.
	stmt2 := result.Statements[1]
	if got := t05FindingsByRule(result, 1, "ddl.index.duplicate.forbid"); len(got) != 0 {
		t.Fatalf("no duplicate stands against the surviving set, got %+v", stmt2.Findings)
	}
	if len(stmt2.EvidenceGaps) != 0 || stmt2.Coverage.Status != report.CoverageComplete {
		t.Fatalf("the vouched post-drop set answers stmt2 cleanly, got %+v/%s",
			stmt2.EvidenceGaps, stmt2.Coverage.Status)
	}
	// stmt3 audits against the set stmt2's apply unvouched.
	stmt3 := result.Statements[2]
	gaps := t05GapsByRule(result, 2, "ddl.index.duplicate.forbid")
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
		t.Fatalf("unvouched member set must report the indexes gap, got %+v", stmt3.EvidenceGaps)
	}
	if stmt3.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("unvouched member set must hold unverified/review, got %s/%s",
			stmt3.Coverage.Status, result.Verdict)
	}
	mid := t10r4PostState(t, spec.DialectMySQL, provider, "ALTER TABLE t DROP INDEX s;")
	if mid == nil || mid.IndexesUnknown || !mid.IndexNamesUnknown || mid.HasIndex("s") {
		t.Fatalf("a resolved authored drop removes only the member, got %+v", mid)
	}
}

// A rename resolving an authored member can only publish when the
// destination freedom is also provable — the colliding members' real
// stored names are unproven, so s3 may already be taken and the rename
// outcome is unprovable. The member's name unvouches; the successor drop
// then cannot prove which member s3 is, and the member set itself
// unvouches rather than guessing — the final add reports the bounded
// collection gap instead of a fabricated clean state.
func TestT10R4RenameResolvesThenDropApplies(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t09Audit(t, spec.DialectMySQL,
		t10r4CollidingCreate+"\n"+
			"CREATE INDEX s ON t(c);\n"+
			"ALTER TABLE t RENAME INDEX s TO s3;\n"+
			"DROP INDEX s3 ON t;\n"+
			"CREATE INDEX s2 ON t(c);",
		provider, map[string]string{"ddl.index.duplicate.forbid": ""})
	stmt5 := result.Statements[4]
	if got := t05FindingsByRule(result, 4, "ddl.index.duplicate.forbid"); len(got) != 0 {
		t.Fatalf("unvouched members must never assert a duplicate, got %+v", stmt5.Findings)
	}
	gaps := t05GapsByRule(result, 4, "ddl.index.duplicate.forbid")
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
		t.Fatalf("the unprovable rename chain must report the indexes gap, got %+v", stmt5.EvidenceGaps)
	}
	if stmt5.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("unprovable chain holds unverified/review, got %s/%s",
			stmt5.Coverage.Status, result.Verdict)
	}
}

// Direct-state mechanics: matched members apply, unmatched names unvouch
// the member set — each mutation family.
func TestT10R4MutationMechanicsUnderUnknownNames(t *testing.T) {
	t.Parallel()
	t.Run("standalone drop matched removes member", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "DROP INDEX s ON t;")
		if got == nil || got.state != tablePresent || got.shape == nil {
			t.Fatalf("matched drop must keep a present entry, got %+v", got)
		}
		if got.shape.HasIndex("s") || len(got.shape.Indexes) != 1 {
			t.Fatalf("member s must be removed, got %+v", got.shape.Indexes)
		}
		if got.shape.IndexesUnknown || !got.shape.IndexNamesUnknown {
			t.Fatalf("member truth preserved, name space still unproven: %+v", got.shape)
		}
	})
	t.Run("standalone drop unmatched marks set unknown", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "DROP INDEX phantom ON t;")
		if got == nil || got.state != tablePresent || got.shape == nil || !got.shape.IndexesUnknown {
			t.Fatalf("unmatched drop must mark IndexesUnknown, got %+v", got)
		}
		if len(got.shape.Indexes) != 2 {
			t.Fatalf("members stay attached — the set is unvouched, not lost: %+v", got.shape.Indexes)
		}
	})
	t.Run("alter drop matched removes member", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t DROP INDEX s;")
		if got == nil || got.state != tablePresent || got.shape.HasIndex("s") {
			t.Fatalf("matched alter drop must remove s, got %+v", got)
		}
	})
	t.Run("alter drop unmatched marks set unknown", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t DROP INDEX phantom;")
		if got == nil || got.state != tablePresent || !got.shape.IndexesUnknown {
			t.Fatalf("unmatched alter drop must mark IndexesUnknown, got %+v", got)
		}
	})
	t.Run("rename matched unvouches the member name", func(t *testing.T) {
		t.Parallel()
		// s is a proven target, but an unproven member may already hold
		// s3 — destination freedom cannot be proven, so the rename either
		// landed (s's name stale) or refused (s kept). The member's name
		// unvouches; the member itself stays.
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t RENAME INDEX s TO s3;")
		if got == nil || got.state != tablePresent || len(got.shape.Indexes) != 2 {
			t.Fatalf("rename keeps every member, got %+v", got)
		}
		member := got.shape.FindIndex("s")
		if member == nil || !member.NameUnknown {
			t.Fatalf("the matched member's stored name must unvouch, got %+v", got.shape.Indexes)
		}
		if !got.shape.IndexNamesUnknown || got.shape.IndexesUnknown {
			t.Fatalf("names unvouch, membership stays: %+v", got.shape)
		}
	})
	t.Run("rename unmatched unvouches names only", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t RENAME INDEX phantom TO x;")
		if got == nil || got.state != tablePresent || !got.shape.IndexNamesUnknown {
			t.Fatalf("unmatched rename must keep names unvouched, got %+v", got)
		}
		if got.shape.IndexesUnknown {
			t.Fatalf("a rename can never change membership: %+v", got.shape)
		}
		if len(got.shape.Indexes) != 2 {
			t.Fatalf("members stay attached: %+v", got.shape.Indexes)
		}
	})
	t.Run("alter index matched flips visibility", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t ALTER INDEX s VISIBLE;")
		if got == nil || got.state != tablePresent || got.shape == nil {
			t.Fatalf("matched alter index keeps a present entry, got %+v", got)
		}
		member := got.shape.FindIndex("s")
		if member == nil || member.Visibility != spec.ColumnVisibilityVisible {
			t.Fatalf("matched visibility flip must publish on s, got %+v", got.shape.Indexes)
		}
	})
	t.Run("alter index unmatched marks set unknown", func(t *testing.T) {
		t.Parallel()
		got := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(), "ALTER TABLE t ALTER INDEX phantom VISIBLE;")
		if got == nil || got.state != tablePresent || !got.shape.IndexesUnknown {
			t.Fatalf("unmatched alter index must mark IndexesUnknown, got %+v", got)
		}
	})
}

// --- A mirror: PRIMARY resolves on primary-key facts, not the name space ---

// t10r4PrimaryShape is a TiDB table with a provably real PRIMARY object
// (nonclustered declaration shape) inside an unproven index name space.
func t10r4PrimaryShape(handleUnknown bool) *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{t10r2Col("id", "int", true, 0)})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "nonclustered"}
	// An unnamed member's stored name is unprovable — per-member
	// NameUnknown carries what the flag alone used to imply.
	shape.Indexes = []spec.Index{{Name: "", Kind: spec.IndexKindSecondary, Columns: []string{"id"}, NameUnknown: true}}
	shape.IndexNamesUnknown = true
	shape.PrimaryKeyHandleUnknown = handleUnknown
	return shape
}

// The pinned mirror: every member collection except the index name space is
// vouched — a PRIMARY-only ALTER INDEX must answer from the primary-key
// facts with no findings and no index-collection gap.
func TestT10R4PrimaryResolvesWithoutIndexesGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r4PrimaryShape(false)},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("a known real PRIMARY must not fabricate a finding, got %+v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("PRIMARY resolution needs no index-collection gap, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("fully resolvable PRIMARY must keep complete/pass, got %s/%s",
			stmt.Coverage.Status, result.Verdict)
	}
}

// A genuinely unproven PRIMARY keeps its primary_key gap — and on TiDB the
// member-collection gap stays too: while the PK object identity is
// unresolvable and the member name space is unvouched, a member literally
// named `primary` may own the name, so the collection fact is owed. The
// suppression removes only a provably-unrelated indexes gap.
func TestT10R4PrimaryUnknownKeepsPrimaryKeyGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r4PrimaryShape(true)},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	stmt := result.Statements[0]
	gaps := t05GapsByRule(result, 0, t10AlterIndexExist)
	if len(gaps) != 2 {
		t.Fatalf("unproven PRIMARY on TiDB owes member + primary_key facts, got %+v", stmt.EvidenceGaps)
	}
	var sawIndexes, sawPrimaryKey bool
	for _, gap := range gaps {
		for _, fact := range gap.RequiredFacts {
			sawIndexes = sawIndexes || fact == "target_table.indexes"
			sawPrimaryKey = sawPrimaryKey || fact == "target_table.primary_key"
		}
	}
	if !sawIndexes || !sawPrimaryKey {
		t.Fatalf("unproven PRIMARY on TiDB owes member + primary_key facts, got %+v", stmt.EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 0, t10AlterIndexExist)) != 0 {
		t.Fatalf("unproven PRIMARY must not fabricate absence, got %+v", stmt.Findings)
	}
}

// Mixed targets: PRIMARY resolves independently while an ordinary index
// target under the same unproven name space keeps its own indexes gap.
func TestT10R4MixedPrimaryAndOrdinaryTargets(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r4PrimaryShape(false)},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;\n"+
			"ALTER TABLE t ALTER INDEX i_x VISIBLE;",
		provider, t10ExistsPolicy(t))
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("PRIMARY-only statement must emit no gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	gaps := t05GapsByRule(result, 1, t10AlterIndexExist)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
		t.Fatalf("the ordinary target keeps its own indexes gap, got %+v", result.Statements[1].EvidenceGaps)
	}
}

// --- B: only natively provable names resolve ----------------------------

// MySQL stores an unnamed index on a column named `primary` as `primary_2`
// — the reserved name is skipped (P1 probe). The successor resolves cleanly.
func TestT10R4MySQLReservedPrimarySuccessorResolves(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (`primary` INT, KEY(`primary`));\n"+
			"ALTER TABLE t ALTER INDEX primary_2 VISIBLE;",
		provider, t10ExistsPolicy(t))
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("stored name primary_2 is provable — no finding, no gap, got %+v / %+v",
			stmt1.Findings, stmt1.EvidenceGaps)
	}
	if stmt1.Coverage.Status != report.CoverageComplete {
		t.Fatalf("resolved member must keep complete coverage, got %s", stmt1.Coverage.Status)
	}
}

// TiDB stores an unnamed index with any expression member as
// `expression_index` (P3 probe) — the successor resolves cleanly.
func TestT10R4TiDBExpressionIndexSuccessorResolves(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (a INT, b INT, KEY(a, ((b + 1))));\n"+
			"ALTER TABLE t ALTER INDEX expression_index VISIBLE;",
		provider, t10ExistsPolicy(t))
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("stored name expression_index is provable — no finding, no gap, got %+v / %+v",
			stmt1.Findings, stmt1.EvidenceGaps)
	}
}

// MySQL stores an unnamed functional index as `functional_index` (P12
// probe): the stored name resolves and a wrong guess reports a vouched
// absence instead of a suppressed one.
func TestT10R4MySQLFunctionalIndexSuccessorResolves(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, KEY((a + 1)));\n"+
			"ALTER TABLE t ALTER INDEX functional_index VISIBLE;\n"+
			"ALTER TABLE t ALTER INDEX functional_idx VISIBLE;",
		provider, t10ExistsPolicy(t))
	if len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("stored name functional_index is provable — got %+v / %+v",
			result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 1 {
		t.Fatalf("a vouched name space reports a real absence, got %+v", result.Statements[2].Findings)
	}
}

// --- C: clustered PK stays — addressable but not droppable ---------------

// A clustered non-single-int primary key is a real, addressable index
// object that DROP PRIMARY KEY still refuses natively (8200). The post-state
// must keep the PK and the successor ALTER INDEX PRIMARY must resolve.
func TestT10R4TiDBCommonHandleDropKeepsPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	stmt3 := result.Statements[2]
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("a refused drop leaves the PRIMARY object addressable, got %+v", stmt3.Findings)
	}
	if len(stmt3.EvidenceGaps) != 0 {
		t.Fatalf("kept PRIMARY resolves cleanly, got %+v", stmt3.EvidenceGaps)
	}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;")
	if next == nil || next.PrimaryKey == nil {
		t.Fatalf("a refused drop must keep the primary key, got %+v", next)
	}
}

// Composite clustered PKs are common-handle too — the drop is refused even
// though the PK is not the row handle.
func TestT10R4TiDBCompositeClusteredDropKeepsPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (a INT, b INT, PRIMARY KEY(a, b) CLUSTERED);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("a refused drop leaves the PRIMARY object addressable, got %+v", result.Statements[2].Findings)
	}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (a INT, b INT, PRIMARY KEY(a, b) CLUSTERED);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;")
	if next == nil || next.PrimaryKey == nil {
		t.Fatalf("a refused drop must keep the primary key, got %+v", next)
	}
}

// An unstated-type declaration under an observed ON session clusters a
// varchar PK into a common handle — the drop is refused there as well (P7).
func TestT10R4TiDBUnstatedModeOnClusteredDropKeepsPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("a refused drop leaves the PRIMARY object addressable, got %+v", result.Statements[2].Findings)
	}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;")
	if next == nil || next.PrimaryKey == nil {
		t.Fatalf("a refused drop must keep the primary key, got %+v", next)
	}
}

// A NONCLUSTERED primary key drops cleanly — the successor then reports the
// PRIMARY object absent, proving the clustered refusal is shape-scoped.
func TestT10R4TiDBNonclusteredDropPublishes(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY NONCLUSTERED);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t))
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 1 {
		t.Fatalf("a provable drop clears the PK — successor reports absence, got %+v",
			result.Statements[2].Findings)
	}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY NONCLUSTERED);\n"+
			"ALTER TABLE t DROP PRIMARY KEY;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;")
	if next != nil && next.PrimaryKey != nil {
		t.Fatalf("a provable drop must clear the primary key, got %+v", next)
	}
}

// Direct-state: a provider-loaded common-handle table (clustered, real
// object) keeps its PK on drop; an unprovable clustered answer invalidates.
func TestT10R4ClusteredDropMechanics(t *testing.T) {
	t.Parallel()
	t.Run("clustered real object keeps PK on drop", func(t *testing.T) {
		t.Parallel()
		shape := t10r2Table("t", []spec.Column{t10r2Col("id", "varchar(32)", true, 0)})
		shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "clustered"}
		shape.PrimaryKeyClustered = true
		got := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t DROP PRIMARY KEY;")
		if got == nil || got.state != tablePresent || got.shape == nil || !got.shape.HasPrimaryKey() {
			t.Fatalf("a refused clustered drop keeps the PK, got %+v", got)
		}
	})
	t.Run("clustered unknown invalidates", func(t *testing.T) {
		t.Parallel()
		shape := t10r2Table("t", []spec.Column{t10r2Col("id", "varchar(32)", true, 0)})
		shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
		shape.PrimaryKeyClusteredUnknown = true
		got := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t DROP PRIMARY KEY;")
		if got != nil && got.state == tablePresent {
			t.Fatalf("an unprovable clustered answer must not publish a drop, got %+v", got)
		}
	})
}

// --- D: direction-unknown expression members keep identity ---------------

// t10r4DirUnknownIndex builds the provider-shape an expression member with
// an unproven catalog direction produces: the member is still an expression
// (identity, dependencies, MV marks) with direction withheld.
func t10r4DirUnknownIndex(name string) spec.Index {
	index := spec.Index{
		Name: name, Kind: spec.IndexKindSecondary, Columns: []string{"a"},
		KeyParts: []spec.IndexKeyPart{
			{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "a"},
			{Ordinal: 2, Kind: spec.IndexKeyPartExpression, DirectionUnknown: true,
				Expression: &spec.ExpressionFacts{Known: true, Fingerprint: "plus(b,1)", References: []spec.ColumnReference{{Name: "b"}}}},
		},
	}
	index.SetKeyParts(index.KeyParts)
	return index
}

// A left-prefix check against an index whose second member is a direction-
// unknown expression cannot complete: the pairing reaches through the
// truncated column projection and reports the member-comparison gap —
// the truncation must not read as a complete member list. The "truncated"
// variant is the shape a pre-fix provider emitted (unmodeled kind carrying
// the expression facts); the "direction-unknown" variant is the corrected
// shape — both must pair through to the same key-parts gap.
func TestT10R4DirUnknownExpressionLeftPrefixGap(t *testing.T) {
	t.Parallel()
	existing := t10r4DirUnknownIndex("i_ab")
	if existing.ExpressionCount != 1 || existing.UnmodeledKeyParts != 0 || existing.DirectionUnknownParts != 1 {
		t.Fatalf("fixture must carry the corrected member shape, got %+v", existing)
	}
	truncated := spec.Index{
		Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a"},
		KeyParts: []spec.IndexKeyPart{
			{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "a"},
			{Ordinal: 2, Kind: spec.IndexKeyPartUnmodeled,
				Expression: &spec.ExpressionFacts{Known: true, Fingerprint: "plus(b,1)", References: []spec.ColumnReference{{Name: "b"}}}},
		},
	}
	truncated.SetKeyParts(truncated.KeyParts)
	if truncated.ExpressionCount != 0 || truncated.UnmodeledKeyParts != 1 || len(truncated.Columns) != 1 {
		t.Fatalf("truncated fixture must carry the legacy member shape, got %+v", truncated)
	}
	table := func(index spec.Index) *spec.TableSnapshot {
		return t10r2Table("t", []spec.Column{
			t10r2Col("a", "int", true, 0),
			t10r2Col("b", "int", true, 0),
		}, index)
	}

	for _, tc := range []struct {
		name    string
		sql     string
		ruleID  string
		indexes map[string]spec.Index
	}{
		{name: "create index", sql: "CREATE INDEX i_a ON t(a);", ruleID: "ddl.index.redundant_left_prefix.forbid",
			indexes: map[string]spec.Index{"corrected": existing, "truncated": truncated}},
		{name: "alter add index", sql: "ALTER TABLE t ADD INDEX i_a (a);", ruleID: "ddl.alter.add_index.redundant_left_prefix.forbid",
			indexes: map[string]spec.Index{"corrected": existing, "truncated": truncated}},
	} {
		tc := tc
		for shapeName, index := range tc.indexes {
			index := index
			t.Run(tc.name+"/"+shapeName, func(t *testing.T) {
				t.Parallel()
				provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table(index)}}
				result := t09Audit(t, spec.DialectMySQL, tc.sql, provider, map[string]string{tc.ruleID: ""})
				stmt := result.Statements[0]
				if len(stmt.Findings) != 0 {
					t.Fatalf("an uncompletable comparison must not fabricate a finding, got %+v", stmt.Findings)
				}
				gaps := t05GapsByRule(result, 0, tc.ruleID)
				if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes.key_parts" {
					t.Fatalf("the truncated projection must report the key-parts gap, got %+v", stmt.EvidenceGaps)
				}
				if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
					t.Fatalf("skipped comparison must hold unverified/review, got %s/%s",
						stmt.Coverage.Status, result.Verdict)
				}
			})
		}
	}
}

// The named-unmodeled control stays a complete projection: a named member
// with unclassifiable direction keeps its column in Columns, so a declared
// prefix pairs against the full visible sequence.
func TestT10R4NamedUnmodeledControlKeepsCompleteProjection(t *testing.T) {
	t.Parallel()
	existing := spec.Index{
		Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"},
		KeyParts: []spec.IndexKeyPart{
			{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "a"},
			{Ordinal: 2, Kind: spec.IndexKeyPartUnmodeled, Column: "b"},
		},
	}
	existing.SetKeyParts(existing.KeyParts)
	if existing.UnmodeledKeyParts != 1 || existing.ExpressionCount != 0 {
		t.Fatalf("named unmodeled fixture = %+v", existing)
	}
	table := t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("b", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	}, existing)
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}

	result := t09Audit(t, spec.DialectMySQL, "CREATE INDEX i_a ON t(a);",
		provider, map[string]string{"ddl.index.redundant_left_prefix.forbid": ""})
	gaps := t05GapsByRule(result, 0, "ddl.index.redundant_left_prefix.forbid")
	if len(gaps) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes.key_parts" {
		t.Fatalf("a pairable non-plain member still reports the gap, got %+v", result.Statements[0].EvidenceGaps)
	}

	result = t09Audit(t, spec.DialectMySQL, "CREATE INDEX i_c ON t(c);",
		provider, map[string]string{"ddl.index.redundant_left_prefix.forbid": ""})
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("a non-pairing declaration stays complete, got %+v / %+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
}

// The member facts survive publication: ALTER INDEX VISIBLE flips the
// effective visibility while the expression kind, direction-unknown mark,
// and aggregate counts carry into the successor's snapshot.
func TestT10R4DirUnknownMemberFactsSurvivePostState(t *testing.T) {
	t.Parallel()
	table := t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("b", "int", true, 0),
	}, t10r4DirUnknownIndex("i_ab"))
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	next := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER INDEX i_ab VISIBLE;\nCREATE INDEX i_a ON t(a);")
	if next == nil || len(next.Indexes) != 2 {
		t.Fatalf("post-state must carry both members, got %+v", next)
	}
	member := next.FindIndex("i_ab")
	if member == nil || len(member.KeyParts) != 2 {
		t.Fatalf("member i_ab must survive with its full member list, got %+v", next.Indexes)
	}
	part := member.KeyParts[1]
	if part.Kind != spec.IndexKeyPartExpression || !part.DirectionUnknown || part.Expression == nil {
		t.Fatalf("expression identity and direction mark must survive publication, got %+v", part)
	}
	if member.ExpressionCount != 1 || member.DirectionUnknownParts != 1 || member.UnmodeledKeyParts != 0 {
		t.Fatalf("aggregates must stay consistent with the member facts, got %+v", member)
	}
}

// --- review fixes: `primary` targets resolve member-first, then the PK ---
//
// The M10 native matrix pins the name-binding order these tests encode:
//   - MySQL 8.0.46: `primary` is reserved — DROP INDEX `PRIMARY` drops the
//     PK (standalone and ALTER alike), RENAME INDEX touching `primary` is
//     refused 1280, ADD/CREATE INDEX named `primary` is refused 1280, and
//     ALTER INDEX `PRIMARY` is refused 3522 — every refused form leaves
//     the table provably unchanged.
//   - TiDB v8.5.0: `primary` binds member-first — a member literally named
//     `primary` (legal via RENAME … TO `primary` on PK-less or PKIsHandle
//     tables, never coexisting with a real PRIMARY object) owns the name;
//     otherwise it resolves to the PK index object — nonclustered drops/
//     renames/visible-pins succeed, clustered refuses 8200, PKIsHandle has
//     no object (1176). A member named `primary` is never reachable with
//     an authored name (1280).

// MySQL DROP INDEX `PRIMARY` is DROP PRIMARY KEY — the PK must be cleared
// on both the standalone and the ALTER form (verified: pk_left=0).
func TestT10R4MySQLDropIndexPrimaryClearsPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	for _, sql := range []string{
		"DROP INDEX `PRIMARY` ON t;",
		"ALTER TABLE t DROP INDEX `PRIMARY`;",
	} {
		next := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+sql)
		if next == nil {
			t.Fatalf("%q: the drop must keep the table entry, got nil", sql)
		}
		if next.PrimaryKey != nil {
			t.Fatalf("%q: DROP INDEX `PRIMARY` clears the PK on MySQL, got %+v", sql, next.PrimaryKey)
		}
		if !next.HasIndex("s") {
			t.Fatalf("%q: unrelated members must survive, got %+v", sql, next.Indexes)
		}
	}
}

// TiDB DROP INDEX `PRIMARY` inherits the clustered refusal: clustered and
// handle shapes keep the PK (8200), nonclustered drops publish.
func TestT10R4TiDBDropIndexPrimaryFollowsDropPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	for _, tc := range []struct {
		name   string
		create string
		wantPK bool
	}{
		{name: "common-handle kept", create: "CREATE TABLE t (id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT);", wantPK: true},
		{name: "composite clustered kept", create: "CREATE TABLE t (a INT, b VARCHAR(8), c INT, PRIMARY KEY(a, b) CLUSTERED);", wantPK: true},
		{name: "int handle kept", create: "CREATE TABLE t (id INT PRIMARY KEY, c INT);", wantPK: true},
		{name: "nonclustered cleared", create: "CREATE TABLE t (id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT);", wantPK: false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			next := t10r4PostState(t, spec.DialectTiDB, provider, tc.create+"\nDROP INDEX `PRIMARY` ON t;")
			if next == nil {
				t.Fatalf("the table entry must survive a refused or proved drop, got nil")
			}
			if got := next.PrimaryKey != nil; got != tc.wantPK {
				t.Fatalf("PrimaryKey present = %v, want %v (shape %+v)", got, tc.wantPK, next)
			}
		})
	}
}

// A TiDB member literally named `primary` owns the name — DROP INDEX
// `primary` removes the member and leaves the PKIsHandle PK untouched
// (verified: member-first precedence, case-insensitive).
func TestT10R4TiDBMemberNamedPrimaryWinsNameResolution(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+
			"ALTER TABLE t RENAME INDEX s TO `primary`;\n"+
			"DROP INDEX `primary` ON t;")
	if next == nil {
		t.Fatalf("the table entry must survive, got nil")
	}
	if next.PrimaryKey == nil {
		t.Fatalf("the member-first drop must leave the handle PK in place, got %+v", next)
	}
	if next.HasIndex("primary") {
		t.Fatalf("the dropped member must be gone, got %+v", next.Indexes)
	}
}

// MySQL refuses RENAME INDEX whenever `primary` is involved (1280): the
// shape provably stays — nothing may be renamed and nothing invalidated.
func TestT10R4MySQLRenameInvolvingPrimaryRefusedKeepsShape(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	for _, sql := range []string{
		"ALTER TABLE t RENAME INDEX `PRIMARY` TO pk2;",
		"ALTER TABLE t RENAME INDEX s TO `primary`;",
	} {
		next := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+sql)
		if next == nil || next.PrimaryKey == nil {
			t.Fatalf("%q: a refused rename keeps every fact, got %+v", sql, next)
		}
		if !next.HasIndex("s") || next.HasIndex("pk2") {
			t.Fatalf("%q: the member set must be untouched, got %+v", sql, next.Indexes)
		}
	}
}

// TiDB RENAME INDEX `PRIMARY` publishes on a real object (nonclustered or
// common-handle — verified rc=0), refusing on PKIsHandle (1176). The
// constraint identity stays PRIMARY; only the index name changes.
func TestT10R4TiDBRenamePrimaryIndexObject(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT);\n"+
			"ALTER TABLE t RENAME INDEX `PRIMARY` TO pk2;")
	if next == nil || next.PrimaryKey == nil {
		t.Fatalf("a successful rename keeps the PK, got %+v", next)
	}
	if next.PrimaryKey.Name != "pk2" {
		t.Fatalf("the stored PK index name must update, got %q", next.PrimaryKey.Name)
	}

	next = t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT);\n"+
			"ALTER TABLE t RENAME INDEX `PRIMARY` TO pk2;")
	if next == nil || next.PrimaryKey == nil || next.PrimaryKey.Name != "pk2" {
		t.Fatalf("the common-handle object renames too (verified rc=0), got %+v", next)
	}

	next = t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id INT PRIMARY KEY, c INT);\n"+
			"ALTER TABLE t RENAME INDEX `PRIMARY` TO pk2;")
	if next != nil && next.PrimaryKey != nil && next.PrimaryKey.Name == "pk2" {
		t.Fatalf("a PKIsHandle rename fails 1176 — the name must not publish, got %+v", next.PrimaryKey)
	}
}

// TiDB renaming a member TO `primary` is refused only while a real PRIMARY
// object occupies the name (1061 on nonclustered/clustered objects); on a
// PKIsHandle or PK-less table the name is free and the member takes it.
func TestT10R4TiDBRenameMemberToPrimaryOccupancy(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	// real object present → refused → member keeps its name
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT, KEY s(c));\n"+
			"ALTER TABLE t RENAME INDEX s TO `primary`;")
	if next == nil {
		t.Fatalf("the table entry must survive a refused rename, got nil")
	}
	if !next.HasIndex("s") || next.HasIndex("primary") {
		t.Fatalf("the refused rename must keep the member name, got %+v", next.Indexes)
	}
	// handle PK → the `primary` name is free → member takes it
	next = t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+
			"ALTER TABLE t RENAME INDEX s TO `primary`;")
	if next == nil {
		t.Fatalf("the table entry must survive, got nil")
	}
	if !next.HasIndex("primary") || next.HasIndex("s") {
		t.Fatalf("the member must publish under `primary`, got %+v", next.Indexes)
	}
}

// An authored `primary` member name is refused on both anchors (1280) —
// ADD/CREATE INDEX must publish nothing.
func TestT10R4AuthoredPrimaryMemberNameRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		version string
		sql     string
	}{
		{spec.DialectMySQL, "8.0.46", "ALTER TABLE t ADD INDEX `primary`(c);"},
		{spec.DialectMySQL, "8.0.46", "CREATE INDEX `primary` ON t(c);"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", "ALTER TABLE t ADD INDEX `primary`(c);"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", "CREATE INDEX `primary` ON t(c);"},
	} {
		provider := &t10Provider{version: tc.version}
		next := t10r4PostState(t, tc.dialect, provider,
			"CREATE TABLE t (id INT PRIMARY KEY, c INT);\n"+tc.sql)
		if next == nil || next.PrimaryKey == nil {
			t.Fatalf("%s %q: a refused add keeps every fact, got %+v", tc.dialect, tc.sql, next)
		}
		if next.HasIndex("primary") {
			t.Fatalf("%s %q: the refused member must not publish, got %+v", tc.dialect, tc.sql, next.Indexes)
		}
	}
}

// An ambiguous named drop under an unproven name space unvouches the member
// set rather than deleting every equal-fold match: two members share the
// authored name `a` and the statement can only name one of them.
func TestT10R4AmbiguousNamedDropUnvouchesSet(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	next := t10r4PostState(t, spec.DialectMySQL, provider,
		t10r4CollidingCreate+"\nDROP INDEX a ON t;")
	if next == nil {
		t.Fatalf("the table entry must survive, got nil")
	}
	if !next.IndexesUnknown {
		t.Fatalf("an ambiguous drop must unvouch the member set, got %+v", next.Indexes)
	}
}

// The same ambiguity holds for rename and visibility — the colliding
// authored names are unproven, so no member binds the target. A rename
// unvouches the name space only (membership cannot change); a visibility
// flip unvouches the member set itself (a member attribute may have
// changed).
func TestT10R4AmbiguousNamedMutationUnvouchesSet(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	next := t10r4PostState(t, spec.DialectMySQL, provider,
		t10r4CollidingCreate+"\nALTER TABLE t RENAME INDEX a TO a2;")
	if next == nil || !next.IndexNamesUnknown || next.IndexesUnknown {
		t.Fatalf("an ambiguous rename unvouches the name space only, got %+v", next)
	}
	next = t10r4PostState(t, spec.DialectMySQL, provider,
		t10r4CollidingCreate+"\nALTER TABLE t ALTER INDEX a INVISIBLE;")
	if next == nil || !next.IndexesUnknown {
		t.Fatalf("an ambiguous visibility flip unvouches the member set, got %+v", next)
	}
}

// t10r4NilTableProvider returns no snapshot at all — the table state is
// completely unprovable, so member-collection and primary-key facts are
// both missing.
type t10r4NilTableProvider struct{ version string }

func (p *t10r4NilTableProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t10r4NilTableProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

// A PRIMARY-only target on an unprovable table owes existence + the
// primary-key identity fact — never the index collection the target
// cannot consult. MySQL's reserved name binds only the PK; TiDB keeps the
// member-collection fact too because a member `primary` could own the
// name while the whole table state is unproven.
func TestT10R4PrimaryOnlyGapOnUnprovableTable(t *testing.T) {
	t.Parallel()
	t.Run("mysql", func(t *testing.T) {
		provider := &t10r4NilTableProvider{version: "8.0.46"}
		result := t10Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;", provider, t10ExistsPolicy(t))
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 {
			t.Fatalf("one bounded gap expected, got %+v", gaps)
		}
		facts := gaps[0].RequiredFacts
		if len(facts) != 2 || facts[0] != "target_table.existence" || facts[1] != "target_table.primary_key" {
			t.Fatalf("the PRIMARY-only gap names existence + primary_key, got %+v", gaps)
		}
	})
	t.Run("tidb", func(t *testing.T) {
		provider := &t10r4NilTableProvider{version: "8.0.11-TiDB-v8.5.0"}
		result := t10Audit(t, spec.DialectTiDB,
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;", provider, t10ExistsPolicy(t))
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 {
			t.Fatalf("one bounded gap expected, got %+v", gaps)
		}
		facts := gaps[0].RequiredFacts
		if len(facts) != 3 || facts[0] != "target_table.existence" ||
			facts[1] != "target_table.indexes" || facts[2] != "target_table.primary_key" {
			t.Fatalf("TiDB keeps the member fact too — a member `primary` may own the name, got %+v", gaps)
		}
	})
}

// A mixed target list keeps the ordinary member gap for its non-PRIMARY
// targets and adds the primary-key fact for the PRIMARY one.
func TestT10R4MixedTargetGapOnUnprovableTable(t *testing.T) {
	t.Parallel()
	provider := &t10r4NilTableProvider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE, ALTER INDEX s VISIBLE;",
		provider, t10ExistsPolicy(t))
	gaps := result.Statements[0].EvidenceGaps
	var sawIndexes, sawPrimaryKey bool
	for _, gap := range gaps {
		for _, fact := range gap.RequiredFacts {
			sawIndexes = sawIndexes || fact == "target_table.indexes"
			sawPrimaryKey = sawPrimaryKey || fact == "target_table.primary_key"
		}
	}
	if !sawIndexes || !sawPrimaryKey {
		t.Fatalf("the mixed target list owes both member and primary facts, got %+v", gaps)
	}
}

// MySQL ALTER INDEX `PRIMARY` is refused outright (3522 on both VISIBLE
// and INVISIBLE) — the refusal surfaces through the advanced-index
// issue path and the invalid-transition gate tombstones the entry like
// every provably-rejected transition.
func TestT10R4MySQLAlterIndexPrimaryRefusedTombstones(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	for _, sql := range []string{
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		"ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
	} {
		next := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+sql)
		if next != nil {
			t.Fatalf("%q: a provably-refused alter tombstones, got %+v", sql, next)
		}
	}
}

// TiDB `primary` under an unvouched member name space with no real
// PRIMARY object is ambiguous — a stored member may own the name. The
// rename outcome is name-level uncertainty the entry already marks, so
// the shape keeps without asserting which side the name bound to. A
// vouched name space on a handle table instead resolves `primary` to the
// absent object and invalidates like a missing member (1176).
func TestT10R4TiDBRenamePrimaryAmbiguousKeepsShape(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: "on"}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY a(c), KEY a(id));\n"+
			"ALTER TABLE t RENAME INDEX `primary` TO pk2;")
	if next == nil {
		t.Fatalf("the ambiguous rename keeps the entry, got nil")
	}
	if next.PrimaryKey != nil && next.PrimaryKey.Name == "pk2" {
		t.Fatalf("an ambiguous rename must not publish a PK rename, got %+v", next.PrimaryKey)
	}
	if !next.IndexNamesUnknown {
		t.Fatalf("the member name space stays unvouched, got %+v", next.Indexes)
	}
}

// Renaming a member TO `primary` while the PK object answer is unprovable
// cannot prove the stored name landed — the member name space unvouches
// rather than publishing the member under `primary`.
func TestT10R4TiDBRenameToPrimaryUnprovenOccupancyUnvouchesNames(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
	next := t10r4PostState(t, spec.DialectTiDB, provider,
		"CREATE TABLE t (id INT PRIMARY KEY, c INT, KEY s(c));\n"+
			"ALTER TABLE t RENAME INDEX s TO `primary`;")
	if next == nil {
		t.Fatalf("the table entry must survive, got nil")
	}
	if !next.IndexNamesUnknown {
		t.Fatalf("an unproven occupancy answer must unvouch the name space, got %+v", next)
	}
	if next.HasIndex("primary") {
		t.Fatalf("the landed name is unprovable — the member must not publish as `primary`, got %+v", next.Indexes)
	}
	member := next.FindIndex("s")
	if member == nil || !member.NameUnknown {
		t.Fatalf("the matched member's stored name is {s, primary} — its name must unvouch, got %+v", next.Indexes)
	}
	if next.FindProvenIndex("s") != nil {
		t.Fatalf("a possibly-renamed member must not stay proven-bindable on the old name, got %+v", next.Indexes)
	}
}

// --- spec-review pass: hidden-collision adds and ambiguous PK-add binding ---

// Pinned defect (spec review): ADD/CREATE INDEX under an unvouched member
// name space can collide on a hidden stored name — the add either landed
// or refused on the 1061, so membership is unprovable and the member set
// unvouches rather than appending a member the statement may never have
// created.
func TestT10R4AddIndexHiddenCollisionUnvouchesSet(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(),
		"ALTER TABLE t ADD INDEX n(a);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the table entry survives — the add may have refused, got %+v", entry)
	}
	if !entry.shape.IndexesUnknown {
		t.Fatalf("a possibly-hidden collision must unvouch the member set, got %+v", entry.shape)
	}
	if len(entry.shape.Indexes) != 2 || entry.shape.HasIndex("n") {
		t.Fatalf("the unproven add must not append a maybe-absent member, got %+v", entry.shape.Indexes)
	}
}

// Standalone CREATE INDEX follows the same collision honesty — a member
// whose real stored name is unproven may hold the authored name.
func TestT10R4CreateIndexHiddenCollisionUnvouchesSet(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectMySQL, t10r4NamesUnknownShape(),
		"CREATE INDEX n ON t(a);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the table entry survives, got %+v", entry)
	}
	if !entry.shape.IndexesUnknown || len(entry.shape.Indexes) != 2 || entry.shape.HasIndex("n") {
		t.Fatalf("a hidden-collision create unvouches without appending, got %+v", entry.shape.Indexes)
	}
}

// A member provably holding the authored name proves the refusal class:
// the add failed on 1061 before touching the table (or IF NOT EXISTS
// no-ops), so the entry keeps every fact instead of tombstoning on a
// statement that provably did nothing.
func TestT10R4AddIndexProvenCollisionKeepsShape(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}}
	entry := t10r4Apply(t, spec.DialectMySQL, shape, "ALTER TABLE t ADD INDEX s(a);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("a proven 1061 refusal keeps the entry, got %+v", entry)
	}
	if entry.shape.IndexesUnknown || len(entry.shape.Indexes) != 1 || entry.shape.Indexes[0].Name != "s" {
		t.Fatalf("the provably-refused add keeps the member set verbatim, got %+v", entry.shape.Indexes)
	}
}

// Standalone CREATE INDEX keeps the same shape on a proven name collision.
func TestT10R4CreateIndexProvenCollisionKeepsShape(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}}
	entry := t10r4Apply(t, spec.DialectMySQL, shape, "CREATE INDEX s ON t(a);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil || entry.shape.IndexesUnknown {
		t.Fatalf("a proven 1061 refusal keeps the shape verbatim, got %+v", entry)
	}
	if len(entry.shape.Indexes) != 1 || entry.shape.Indexes[0].Name != "s" {
		t.Fatalf("the member set is provably unchanged, got %+v", entry.shape.Indexes)
	}
}

// An authored `primary` member name is refused (1280) before the table is
// touched — even on an unknown table the statement provably did nothing,
// so the entry must not publish present-incomplete.
func TestT10R4CreateIndexPrimaryKeepsUnknownEntry(t *testing.T) {
	t.Parallel()
	state := newBatchState(spec.DialectMySQL, "golden", &t10Provider{version: "8.0.46"})
	key := state.keyFor("golden", spec.Table{Name: "t"})
	state.entries[key] = &batchTableEntry{state: tableUnknown}
	parsed, err := parseSQL(context.Background(), "CREATE INDEX `primary` ON t(a);", spec.DialectMySQL)
	if err != nil || len(parsed.Statements) != 1 {
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
	if state.entries[key].state != tableUnknown {
		t.Fatalf("a provably refused create must keep the unknown entry, got %+v", state.entries[key])
	}
}

// A stale collection flag does not block the merge when every member name
// is individually proven — the authored member lands and the flag keeps
// name-keyed readers honest.
func TestT10R4AddIndexMergesWhenAllNamesProven(t *testing.T) {
	t.Parallel()
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("c", "int", true, 0),
	})
	shape.Indexes = append(shape.Indexes,
		spec.Index{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		spec.Index{Name: "u", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
	)
	shape.IndexNamesUnknown = true
	entry := t10r4Apply(t, spec.DialectMySQL, shape, "ALTER TABLE t ADD INDEX n(a);")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the provable merge publishes, got %+v", entry)
	}
	if entry.shape.IndexesUnknown || !entry.shape.HasIndex("n") || len(entry.shape.Indexes) != 3 {
		t.Fatalf("an authored name with all members proven merges, got %+v", entry.shape.Indexes)
	}
	if !entry.shape.IndexNamesUnknown {
		t.Fatalf("the surviving flag must keep name-keyed readers honest, got %+v", entry.shape)
	}
}

// Pinned defect (spec review): TiDB ADD PRIMARY KEY under a name space a
// member `primary` provably or possibly owns is shape-dependent — a handle
// PK coexists with the member while a real PRIMARY object collides (1061)
// — so only a proven PK-bound binding can publish; anything else
// invalidates.
func TestT10R4TiDBAddPrimaryKeyAmbiguousBindingInvalidates(t *testing.T) {
	t.Parallel()
	t.Run("proven member primary blocks the publish", func(t *testing.T) {
		t.Parallel()
		shape := t10r2Table("t", []spec.Column{
			t10r2Col("a", "int", true, 0),
			t10r2Col("c", "int", true, 0),
		})
		shape.Indexes = append(shape.Indexes,
			spec.Index{Name: "primary", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		)
		entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t ADD PRIMARY KEY(a);")
		if entry != nil && entry.state == tablePresent {
			t.Fatalf("a member-owned `primary` makes the ADD outcome unprovable, got %+v", entry)
		}
	})
	t.Run("unvouched name space blocks the publish", func(t *testing.T) {
		t.Parallel()
		shape := t10r2Table("t", []spec.Column{
			t10r2Col("a", "int", true, 0),
			t10r2Col("c", "int", true, 0),
		})
		shape.Indexes = append(shape.Indexes,
			spec.Index{Name: "p2", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, NameUnknown: true},
		)
		shape.IndexNamesUnknown = true
		entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t ADD PRIMARY KEY(a);")
		if entry != nil && entry.state == tablePresent {
			t.Fatalf("an unvouched name space cannot rule out member `primary`, got %+v", entry)
		}
	})
	t.Run("a PK-bound name space publishes", func(t *testing.T) {
		t.Parallel()
		shape := t10r2Table("t", []spec.Column{
			t10r2Col("a", "int", true, 0),
			t10r2Col("c", "int", true, 0),
		})
		shape.Indexes = append(shape.Indexes,
			spec.Index{Name: "s", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
		)
		entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t ADD PRIMARY KEY(a);")
		if entry == nil || entry.state != tablePresent || entry.shape == nil || !entry.shape.HasPrimaryKey() {
			t.Fatalf("a provably PK-bound name space publishes the add, got %+v", entry)
		}
	})
}
