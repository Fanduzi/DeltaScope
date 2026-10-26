// Package audit pins the T11-R2 DROP CHECK|CONSTRAINT ambiguity across
// every removable target kind: MySQL 8.x shares one AST node for both
// keywords, so the authored keyword is unrecoverable and any proven
// target the two keywords treat differently leaves an unprovable fate —
// a proven unique member, the primary-key object, or a proven foreign
// key drops under DROP CONSTRAINT but refuses under DROP CHECK
// (rc=0/3821 probed on 8.0.46 and 8.4.10). A non-unique member refuses
// under both keywords and keeps its proven state; an unmatched name
// refuses under both; TiDB removes nothing this way under either
// tidb_enable_check_constraint arm (warned no-op / 3940, probed).
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r2DropConstraintFixture is the proven target table: a unique member
// uk_v, a plain secondary member ix_u, and authored columns.
func t11r2DropConstraintFixture() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{
		t10r2Col("id", "int", true, 0),
		t10r2Col("v", "int", false, 0),
		t10r2Col("u", "int", false, 0),
	})
	shape.Indexes = []spec.Index{
		{Name: "uk_v", Kind: spec.IndexKindUnique, Columns: []string{"v"}},
		{Name: "ix_u", Kind: spec.IndexKindSecondary, Columns: []string{"u"}},
	}
	return shape
}

// DROP CONSTRAINT uk_v removes the proven unique member outright — the
// T12-R1 spelling fact proves the generic keyword, and the probed 8.0.46
// answer (rc=0) drops the member deterministically. The successor DROP
// INDEX therefore binds a proven-missing name and reports the blocker.
func TestT11R2DropConstraintUnvouchesUniqueMember(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;",
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("a deterministic removal owes no gap, got %#v", drop.EvidenceGaps)
	}
	found := false
	for _, f := range drop.Findings {
		if f.RuleID == "ddl.alter.drop_index.exists.require" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the removed member is a proven-missing blocker for the successor drop, got %#v", drop.Findings)
	}
}

// The backticked primary name binds the PK object on MySQL 8.x — the
// generic spelling removes it deterministically (probed rc=0), so the
// successor DROP PRIMARY KEY reports a proven-missing blocker rather
// than a gap.
func TestT11R2DropConstraintUnvouchesPrimaryKeyObject(t *testing.T) {
	t.Parallel()
	fixture := t11r2DropConstraintFixture()
	fixture.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT `primary`; ALTER TABLE t DROP PRIMARY KEY;",
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": fixture}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_primary_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("a deterministic PK removal owes no gap, got %#v", drop.EvidenceGaps)
	}
	found := false
	for _, f := range drop.Findings {
		if f.RuleID == "ddl.alter.drop_primary_key.exists.require" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the dropped PK is a proven-missing blocker for the successor, got %#v", drop.Findings)
	}
}

// A proven name with an unproven kind cannot prove survival — the member
// may be a UNIQUE (DROP CONSTRAINT removes it). Both the IndexKindUnknown
// and the empty-kind cell leave the member's fate undecidable: the member
// projection drops, the index set unvouches, and the successor drop owes
// the attributed index gap instead of binding a maybe-gone member or
// fabricating a proven-missing answer.
func TestT11R2DropConstraintUnknownKindUnvouches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kind spec.IndexKind
	}{
		{name: "index_kind_unknown", kind: spec.IndexKindUnknown},
		{name: "empty_kind", kind: ""},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := t11r2DropConstraintFixture()
			fixture.Indexes = []spec.Index{
				{Name: "uk_v", Kind: tc.kind, Columns: []string{"v"}},
				{Name: "ix_u", Kind: spec.IndexKindSecondary, Columns: []string{"u"}},
			}
			result := t10Audit(t, spec.DialectMySQL,
				"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;",
				&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": fixture}},
				t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
			drop := result.Statements[1]
			if len(drop.Findings) != 0 {
				t.Fatalf("the member may already be gone — no proven-missing and no proven drop, got %#v", drop.Findings)
			}
			if len(drop.EvidenceGaps) == 0 {
				t.Fatalf("an undecidable member fate owes the index-set gap, got none")
			}
			factFound := false
			for _, gap := range drop.EvidenceGaps {
				for _, fact := range gap.RequiredFacts {
					if fact == "target_table.indexes" {
						factFound = true
					}
				}
			}
			if !factFound {
				t.Fatalf("the gap must attribute the index fact — required_facts=%#v", drop.EvidenceGaps)
			}
			if drop.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("the successor statement's own coverage stays unverified, got %s", drop.Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("an undecidable fate reviews rather than passes, got %s", result.Verdict)
			}
		})
	}
}

// The successor statement's own pre-state must not keep a proven uk_v —
// the ambiguous drop cannot leave the member bindable by name.
func TestT11R2DropConstraintUnknownKindPreStateUnvouched(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": func() *spec.TableSnapshot {
		shape := t11r2DropConstraintFixture()
		shape.Indexes = []spec.Index{
			{Name: "uk_v", Kind: spec.IndexKindUnknown, Columns: []string{"v"}},
			{Name: "ix_u", Kind: spec.IndexKindSecondary, Columns: []string{"u"}},
		}
		return shape
	}()}}
	parsed, err := parseSQL(context.Background(),
		"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	pre := enriched[1].Metadata.TargetTable
	if pre == nil || !pre.IndexesUnknown {
		t.Fatalf("the successor pre-state must carry the unvouched index set, got %+v", pre)
	}
	if member := pre.FindProvenIndex("uk_v"); member != nil {
		t.Fatalf("the maybe-gone member cannot stay provable by name, got %+v", member)
	}
	if member := pre.FindProvenIndex("ix_u"); member == nil {
		t.Fatalf("the untouched member keeps its proven facts, got %+v", pre.Indexes)
	}
}

// A non-unique secondary member refuses under BOTH keywords (3940/3821
// probed) — it provably survives, the set stays vouched, and the
// successor DROP INDEX binds it cleanly.
func TestT11R2DropConstraintKeepsSecondaryMember(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT ix_u; ALTER TABLE t DROP INDEX ix_u;",
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the surviving member drops cleanly, got %#v", drop.Findings)
	}
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("a provably-surviving member owes no unvouch gap, got %#v", drop.EvidenceGaps)
	}
}

// An unmatched name on a fully vouched shape refuses under both keywords —
// nothing unvouches.
func TestT11R2DropConstraintMissingNameKeepsFacts(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT missing_x; ALTER TABLE t DROP INDEX ix_u;",
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("a proven refusal changes nothing — no gap owed, got %#v", result.Statements[0].EvidenceGaps)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("the untouched member still drops cleanly, got %#v", result.Statements[1].Findings)
	}
}

// DROP CHECK binds only the CHECK name space — on a unique member it
// refuses 3821 (probed 8.0.46), so the member provably survives and the
// successor DROP INDEX binds it cleanly.
func TestT11R2DropCheckKeywordSameAmbiguity(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CHECK uk_v; ALTER TABLE t DROP INDEX uk_v;",
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the surviving member drops cleanly, got %#v", drop.Findings)
	}
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("a proven refusal under the CHECK keyword owes no gap, got %#v", drop.EvidenceGaps)
	}
}

// TiDB removes nothing through this action under either
// tidb_enable_check_constraint arm — the unique member stays proven and
// the successor drop binds it.
func TestT11R2TiDBDropConstraintNoOp(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;",
		&t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("TiDB provably removes nothing — no gap owed, got %#v", result.Statements[0].EvidenceGaps)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("the surviving member drops cleanly, got %#v", result.Statements[1].Findings)
	}
}

// MySQL 5.7 rejects the syntax outright — the unique member keeps its
// proven state.
func TestT11R2MySQL57DropConstraintSyntaxRefusal(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;",
		&t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("5.7 refuses the syntax — no gap owed, got %#v", result.Statements[0].EvidenceGaps)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("the surviving member drops cleanly, got %#v", result.Statements[1].Findings)
	}
}

// An unproven MySQL version cannot rule out the 8.x removal arm — the
// unique member's fate stays undecidable.
func TestT11R2UnknownVersionDropConstraintUnvouches(t *testing.T) {
	t.Parallel()
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT uk_v; ALTER TABLE t DROP INDEX uk_v;",
		&t10Provider{version: "", tables: map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()}},
		t10Policy(t, t10Rule{id: "ddl.alter.drop_index.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the member may be gone — no finding, got %#v", drop.Findings)
	}
	if len(drop.EvidenceGaps) == 0 {
		t.Fatalf("version unproven leaves the fate undecidable — gap owed")
	}
}

// The generic drop removes the bound member and nothing else: the columns
// and the untargeted secondary member keep their proven facts, and the
// index set stays vouched — a deterministic removal never unvouches.
func TestT11R2DropConstraintPreservesUntargetedFacts(t *testing.T) {
	t.Parallel()
	state := t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": t11r2DropConstraintFixture()},
		&t10Provider{version: "8.0.46"},
		"ALTER TABLE t DROP CONSTRAINT uk_v;")
	entry := t11Entry(state, "t")
	if entry == nil || entry.shape == nil {
		t.Fatal("entry kept for assertion")
	}
	if entry.shape.Columns == nil || len(entry.shape.Columns) != 3 {
		t.Fatalf("column facts are unaffected, got %+v", entry.shape.Columns)
	}
	if entry.shape.IndexesUnknown {
		t.Fatalf("a deterministic removal never unvouches the index set, got %+v", entry.shape.Indexes)
	}
	if member := entry.shape.FindProvenIndex("uk_v"); member != nil {
		t.Fatalf("the bound unique member is removed, got %+v", member)
	}
	if member := entry.shape.FindProvenIndex("ix_u"); member == nil {
		t.Fatalf("the untargeted member keeps its proven facts, got %+v", entry.shape.Indexes)
	}
}
