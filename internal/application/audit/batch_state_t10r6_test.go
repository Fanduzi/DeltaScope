// Package audit verifies the T10-R6 consumer and batch-state closure:
// attribution-demoted candidates feed rules, gaps, coverage, verdicts and
// the published post-state through the same shared binding — an INVISIBLE
// alter on a maybe-PK fold reports the missing PK-identity fact instead of
// silently passing, kind-withheld members keep pair comparisons honest,
// DROP PRIMARY KEY existence routes through the actual stored name, and a
// proven PK-object name (pk2) binds even while member names stay unproven.
// input: hand-built provider snapshots plus parsed TiDB statements through
// the public audit seams (AuditSQL/enrich/batch apply)
// output: findings, evidence gaps, coverage, verdicts and published
// post-state shape assertions
// pos: T10-R6 audit-side closure tests (issue #89); the
// IndexNamesUnknown fixtures below are explicitly labeled partial
// snapshots — they model provider states the reload can produce, not a
// claim that pure-SQL transitions mint this shape
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r6CompatVisibilityPolicy isolates the advanced-index compatibility
// rule with only the visibility feature enabled.
func t10r6CompatVisibilityPolicy(t *testing.T) string {
	return t10Policy(t, t10Rule{
		id:     t10IndexCompat,
		params: "      required: true\n      features:\n        - visibility\n",
	})
}

// t10r6AmbiguousProvider serves the ambiguous-attribution shape R5 modeled:
// PK presence vouched, the object identity withheld (NameUnknown), and the
// two unique-candidate folds demoted to kind unknown beside a proven
// secondary member.
func t10r6AmbiguousProvider() *t10Provider {
	return &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r5AmbiguousAttributionShape()},
	}
}

// t10r6UnvouchedRenamedPKShape is an explicitly labeled partial snapshot:
// the proven renamed PK object pk2 beside one member whose stored name the
// projection cannot prove and a member name space flagged unvouched. It is
// a fixture for the name-keyed consumer ordering question — not a claim
// that a pure-SQL chain mints this exact shape.
func t10r6UnvouchedRenamedPKShape() *spec.TableSnapshot {
	shape := t10r5RenamedPKShape()
	shape.IndexNamesUnknown = true
	shape.Indexes[0].NameUnknown = true
	return shape
}

// B1: ALTER INDEX pk2 INVISIBLE on the attribution-ambiguous shape — pk2
// may be the PK object (3522-refused) or an ordinary UNIQUE member (lands).
// The enabled compatibility rule must surface that missing PK-identity
// fact as an evidence gap with unverified coverage and a review verdict —
// never a fabricated 3522 finding and never a clean pass.
func TestT10R6AlterIndexInvisibleOnAmbiguousCandidate(t *testing.T) {
	t.Parallel()
	provider := t10r6AmbiguousProvider()
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX pk2 INVISIBLE; ALTER TABLE t ALTER INDEX s VISIBLE;",
		provider, t10r6CompatVisibilityPolicy(t))
	if len(result.Statements) != 2 {
		t.Fatalf("both statements must audit, got %+v", result.Statements)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("pk2's kind is withheld — no fabricated 3522 finding, got %+v", stmt.Findings)
	}
	gap := findGapByFact(stmt.EvidenceGaps, "target_table.primary_key")
	if gap == nil || gap.RuleID != t10IndexCompat {
		t.Fatalf("the withheld PK identity must be the enabled rule's own gap, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("an outcome-blocking withheld fact means unverified coverage, got %+v", stmt.Coverage)
	}
	if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("the same withheld fact holds the verdict at review/unverified, got verdict=%v coverage=%+v", result.Verdict, result.Coverage)
	}
	enriched := enrichA3(t,
		"ALTER TABLE t ALTER INDEX pk2 INVISIBLE; ALTER TABLE t ALTER INDEX s VISIBLE;",
		spec.DialectTiDB, provider)
	// The ambiguous INVISIBLE may have been refused (3522 on the object)
	// or may have flipped a member — the post-state is unprovable, so the
	// publication gate tombstones the table and the next statement's
	// pre-state carries no target-table snapshot at all.
	if enriched[1].Metadata.TargetTable != nil {
		t.Fatalf("an unprovable post-state must not publish a target-table snapshot, got %+v",
			enriched[1].Metadata.TargetTable)
	}
}

// B2: adding the exists rule must not mask the missing PK-identity fact —
// pk2 "exists" as a member name but that answers nothing about its
// PK-vs-UNIQUE identity.
func TestT10R6AlterIndexInvisibleAmbiguousExistsDoesNotMask(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX pk2 INVISIBLE;",
		t10r6AmbiguousProvider(), t10Policy(t,
			t10Rule{id: t10AlterIndexExist},
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
		))
	if len(result.Statements) != 1 {
		t.Fatalf("the statement must audit, got %+v", result.Statements)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("exists answers the name but not the kind — no fabricated finding, got %+v", stmt.Findings)
	}
	if findGapByFact(stmt.EvidenceGaps, "target_table.primary_key") == nil {
		t.Fatalf("the exists answer must not mask the PK-identity gap, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("coverage/verdict mirror the withheld fact, got %+v / %v", stmt.Coverage, result.Verdict)
	}
}

// B3: a kind-withheld existing member makes every plain-key pair
// comparison that could involve it unanswerable — the comparison rule must
// report the member-kind gap instead of silently skipping the pair.
func TestT10R6ComparisonRulesReportKindWithheldPairs(t *testing.T) {
	t.Parallel()
	t.Run("duplicate pairing", func(t *testing.T) {
		t.Parallel()
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE UNIQUE INDEX nq ON t(id);",
			t10r6AmbiguousProvider(), t10Policy(t, t10Rule{id: "ddl.index.duplicate.forbid"}))
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("pk2's kind is withheld — no proven duplicate pair, got %+v", stmt.Findings)
		}
		if findGapByFact(stmt.EvidenceGaps, "target_table.indexes") == nil {
			t.Fatalf("a same-columns candidate of withheld kind must owe the member-kind fact, got %+v", stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("an unanswerable pair means unverified coverage, got %+v", stmt.Coverage)
		}
	})
	t.Run("unique overlap pairing", func(t *testing.T) {
		t.Parallel()
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE INDEX ns ON t(id);",
			t10r6AmbiguousProvider(), t10Policy(t, t10Rule{id: "ddl.index.redundant_unique_overlap.forbid"}))
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("pk2 may be UNIQUE but is not proven so — no fabricated overlap, got %+v", stmt.Findings)
		}
		if findGapByFact(stmt.EvidenceGaps, "target_table.indexes") == nil {
			t.Fatalf("a same-columns candidate of withheld kind must owe the member-kind fact, got %+v", stmt.EvidenceGaps)
		}
	})
}

// B3 control: a proven secondary member still produces its independent
// duplicate finding while the withheld-kind candidates stay out of the
// verdict — the comparison must not demote the whole index set.
func TestT10R6ComparisonKeepsProvenFindings(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE INDEX s2 ON t(c);",
		t10r6AmbiguousProvider(), t10Policy(t, t10Rule{id: "ddl.index.duplicate.forbid"}))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 {
		t.Fatalf("s2 provably duplicates the proven member s, got %+v", stmt.Findings)
	}
	if findGapByFact(stmt.EvidenceGaps, "target_table.indexes") != nil {
		t.Fatalf("no declared column matches the withheld candidates — no spurious kind gap, got %+v", stmt.EvidenceGaps)
	}
}

// B4 controls: a proven secondary member's VISIBLE flip and a rule-off run
// stay clean — no gaps minted where nothing is withheld.
func TestT10R6AmbiguousControlsStayClean(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX s VISIBLE;",
		t10r6AmbiguousProvider(), t10r6CompatVisibilityPolicy(t))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("a proven secondary member answers cleanly, got findings=%+v gaps=%+v", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("the visible no-op on a proven member is fully covered, got %+v", stmt.Coverage)
	}
}

// C audit seam: after the PK object's rename to pk2, `primary` is freed —
// DROP PRIMARY KEY then hits nothing (1091, verified on TiDB v8.5.0
// probe a2b). The exists rule must fire the proven-missing blocker instead
// of reading mere PK presence as target existence.
func TestT10R6DropPrimaryKeyAfterRenameThroughAudit(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r5RenamedPKShape()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP PRIMARY KEY;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_primary_key.exists.require"}))
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 {
		t.Fatalf("the freed `primary` name binds nothing — the proven-missing finding must fire, got %+v", stmt.Findings)
	}
	if got := stmt.Findings[0].Metadata["exists"]; got != false {
		t.Fatalf("the finding carries the proven-absent answer, got %+v", stmt.Findings[0].Metadata)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("every needed fact is proven — no gaps, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
		t.Fatalf("a proven-missing blocker is complete+reject, got %+v / %v", stmt.Coverage, result.Verdict)
	}
}

// C audit seam through post-state: the renamed PK object is dropped by
// its actual name first (pk_left=0 verified), then DROP PRIMARY KEY still
// reports the proven absence — never blocked by a stale PK presence.
func TestT10R6DropPrimaryKeyAfterObjectDrop(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r5RenamedPKShape()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP INDEX pk2; ALTER TABLE t DROP PRIMARY KEY;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_primary_key.exists.require"}))
	if len(result.Statements) != 2 {
		t.Fatalf("both statements must audit, got %+v", result.Statements)
	}
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("the actual-name drop resolves cleanly, got findings=%+v gaps=%+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	stmt := result.Statements[1]
	if len(stmt.Findings) != 1 {
		t.Fatalf("post-drop PK absence is proven — the missing finding must fire, got %+v", stmt.Findings)
	}
	enriched := enrichA3(t,
		"ALTER TABLE t DROP INDEX pk2; ALTER TABLE t DROP PRIMARY KEY;",
		spec.DialectTiDB, provider)
	post := enriched[1].Metadata.TargetTable
	if post != nil && post.HasPrimaryKey() {
		t.Fatalf("the dropped PK object must not linger in the post-state, got %+v", post)
	}
}

// D main counterexample: RENAME INDEX pk2 TO pk3 while the member name
// space is unvouched. The source is the proven PK object — a hidden member
// holding pk3 makes the outcome undecidable (rename vs 1061 refusal), so
// the post-state must withhold the PK object's stored name; the follow-up
// ALTER INDEX pk2 then owes the identity gap instead of treating pk2 as
// proven present.
func TestT10R6RenamePKObjectUnderUnvouchedMembers(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r6UnvouchedRenamedPKShape()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t RENAME INDEX pk2 TO pk3; ALTER TABLE t ALTER INDEX pk2 VISIBLE;",
		provider, t10ExistsPolicy(t))
	if len(result.Statements) != 2 {
		t.Fatalf("both statements must audit, got %+v", result.Statements)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("pk2 provably exists as the PK object — no fabricated absent finding, got %+v", result.Statements[0].Findings)
	}
	stmt := result.Statements[1]
	if len(stmt.Findings) != 0 {
		t.Fatalf("pk2's post-rename binding is ambiguous — the absent blocker suppresses, got %+v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 {
		t.Fatal("the withheld identity must surface as an evidence gap, got none")
	}
	enriched := enrichA3(t,
		"ALTER TABLE t RENAME INDEX pk2 TO pk3; ALTER TABLE t ALTER INDEX pk2 VISIBLE;",
		spec.DialectTiDB, provider)
	post := enriched[1].Metadata.TargetTable
	if post == nil || post.PrimaryKey == nil || !post.PrimaryKey.NameUnknown {
		t.Fatalf("the possibly-moved PK object name withholds, got %+v", post)
	}
}

// D mirror: DROP INDEX pk2 under the same unvouched fixture — pk2 is the
// proven PK object, so the drop provably removes it regardless of member
// name provability (verified v8.5.0: DROP INDEX pk2 left pk_left=0). The
// stale member-name early exit must not keep the PK alive.
func TestT10R6DropPKObjectUnderUnvouchedMembers(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		name  string
		shape func() *spec.TableSnapshot
	}{
		{"names unvouched", t10r6UnvouchedRenamedPKShape},
		{"indexes unknown", func() *spec.TableSnapshot {
			shape := t10r5RenamedPKShape()
			shape.IndexesUnknown = true
			return shape
		}},
	} {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()
			entry := t10r4Apply(t, spec.DialectTiDB, variant.shape(),
				"ALTER TABLE t DROP INDEX pk2;")
			if entry == nil || entry.state != tablePresent || entry.shape == nil {
				t.Fatalf("a proven-object drop still publishes a bounded post-state, got %+v", entry)
			}
			if entry.shape.HasPrimaryKey() || entry.shape.PrimaryKeyUnknown {
				t.Fatalf("DROP INDEX pk2 provably removes the PK object, got %+v", entry.shape)
			}
		})
	}
}

// D mirror: ALTER INDEX pk2 under the unvouched fixture — VISIBLE is a
// proven no-op on the real object while INVISIBLE refuses (3522), so the
// member-name early exit must not unvouch members for VISIBLE nor keep the
// entry published for INVISIBLE.
func TestT10R6AlterIndexVisibilityPKObjectUnderUnvouchedMembers(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectTiDB, t10r6UnvouchedRenamedPKShape(),
		"ALTER TABLE t ALTER INDEX pk2 INVISIBLE;")
	if entry == nil || entry.state == tablePresent {
		t.Fatalf("INVISIBLE on the real PK object refuses (3522) — the entry must not survive as published, got %+v", entry)
	}
	entry = t10r4Apply(t, spec.DialectTiDB, t10r6UnvouchedRenamedPKShape(),
		"ALTER TABLE t ALTER INDEX pk2 VISIBLE;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("VISIBLE on the real PK object is a proven no-op, got %+v", entry)
	}
	if entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk2" {
		t.Fatalf("the PK object stays pk2 under the no-op, got %+v", entry.shape.PrimaryKey)
	}
}

// D ambiguity split: DROP INDEX x where both sides are unproven — a member
// may carry x and the withheld PK object may carry x — unvouches the
// member collection and the PK presence together.
func TestT10R6DropIndexAmbiguousBothSidesUnvouch(t *testing.T) {
	t.Parallel()
	shape := t10r5RenamedPKShape()
	shape.PrimaryKey.NameUnknown = true
	shape.PrimaryKey.Name = ""
	shape.Indexes[0].NameUnknown = true
	entry := t10r4Apply(t, spec.DialectTiDB, shape, "ALTER TABLE t DROP INDEX x;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("an ambiguous drop withholds rather than invalidating, got %+v", entry)
	}
	if !entry.shape.PrimaryKeyUnknown {
		t.Fatalf("x may be the withheld PK object — presence unvouches, got %+v", entry.shape)
	}
	if !entry.shape.IndexesUnknown {
		t.Fatalf("x may be an unproven member — the collection unvouches, got %+v", entry.shape)
	}
}

// D provider-minted cell (spec-review F1): the object name can be proven
// while the handle/clustered answers stay unproven — a missing or
// unrecognized tidb_indexes row on the object's keyed probe leaves
// PrimaryKeyHandleUnknown/PrimaryKeyClusteredUnknown set. The proven
// stored name still binds the real object: a handle synth row is always
// named PRIMARY, so a proven pk2 cannot be one. Name-keyed drops,
// renames, and visibility flips must route to the object instead of
// stale-keeping or tombstoning the entry.
func t10r6HandleUnknownPKObjectShape() *spec.TableSnapshot {
	shape := t10r5RenamedPKShape()
	shape.PrimaryKeyHandleUnknown = true
	shape.PrimaryKeyClusteredUnknown = true
	return shape
}

func TestT10R6HandleUnknownPKObjectNameStillBinds(t *testing.T) {
	t.Parallel()
	entry := t10r4Apply(t, spec.DialectTiDB, t10r6HandleUnknownPKObjectShape(),
		"ALTER TABLE t DROP INDEX pk2;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("the proven object name binds under an unproven handle answer, got %+v", entry)
	}
	if entry.shape.HasPrimaryKey() || entry.shape.PrimaryKeyUnknown {
		t.Fatalf("pk2 provably is the real object — the drop removes it, got %+v", entry.shape)
	}
	entry = t10r4Apply(t, spec.DialectTiDB, t10r6HandleUnknownPKObjectShape(),
		"ALTER TABLE t RENAME INDEX pk2 TO pk3;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil ||
		entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk3" {
		t.Fatalf("the proven object renames through its stored name, got %+v", entry)
	}
	entry = t10r4Apply(t, spec.DialectTiDB, t10r6HandleUnknownPKObjectShape(),
		"ALTER TABLE t ALTER INDEX pk2 VISIBLE;")
	if entry == nil || entry.state != tablePresent || entry.shape == nil ||
		entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Name != "pk2" {
		t.Fatalf("VISIBLE on the proven object is a no-op keeping pk2, got %+v", entry)
	}
}

// D positive control: a second rename on the proven renamed PK object
// keeps the stored-name continuity — pk2 → pk3 → pk4 each updates the
// object's actual catalog name.
func TestT10R6SecondRenameOnProvenRenamedPK(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r5RenamedPKShape()},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t RENAME INDEX pk2 TO pk3; ALTER TABLE t RENAME INDEX pk3 TO pk4;")
	if post == nil || post.PrimaryKey == nil {
		t.Fatalf("both renames publish, got %+v", post)
	}
	if post.PrimaryKey.Name != "pk4" {
		t.Fatalf("the PK object tracks its actual stored name through every rename, got %q", post.PrimaryKey.Name)
	}
}

// findGapByFact returns the first gap requiring the named fact.
func findGapByFact(gaps []rule.EvidenceGap, fact string) *rule.EvidenceGap {
	for i := range gaps {
		for _, required := range gaps[i].RequiredFacts {
			if required == fact {
				return &gaps[i]
			}
		}
	}
	return nil
}
