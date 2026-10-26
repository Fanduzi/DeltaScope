package audit

import (
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12-R2-tail remediation cells (#91 reviewer tail items A/B/D).
//   - MySQL 5.7.44/8.0.46: `--` opens a line comment only when followed by
//     whitespace or a control character — `v--1` lexes as `v - -1`
//     (MySQL manual comment syntax; locked TiDB lexer startWithDash).
//   - An undecidable generic DROP that may have hit the PK object must
//     unvouch the PK projection — the successor owes a primary_key gap.
//   - A same-statement proven DROP CHECK lifts the check-reference
//     premise a rename/change would violate (MySQL docs: the check drop
//     is applied so the column identity change may proceed). The
//     statement then falls outside the bounded template — withheld, never
//     a proven refusal that preserves the wrong pre-state.

const t12DropPKExists = "ddl.alter.drop_primary_key.exists.require"

// --- Tail A: `--` lexing ---------------------------------------------------

// `v--1` is `v - -1`, not a comment — the trailing NOT ENFORCED is a real
// authored clause that refuses 1064 on 5.7 (probed). The column must not
// land and the declaration must not masquerade as parsed-and-ignored.
func TestT12R2TailDashDashExprKeepsExplicitClauseOn57(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "5.7.44",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN v INT CHECK(v--1>0) NOT ENFORCED;",
		provider, t10Policy(t, t10Rule{id: t12IgnoredNotice}))
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			t.Fatalf("-- inside the expression swallowed the authored clause; refused declaration misreported as ignored: %+v", f)
		}
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CHECK(v--1>0) NOT ENFORCED;")
	if post == nil {
		t.Fatal("proven 1064 refusal must keep the pre-state, not tombstone")
	}
	if t12r1ColumnPresent(post, "v") {
		t.Fatal("-- swallowing NOT ENFORCED published column v on 5.7")
	}
	// Bare counterpart: no authored clause → the member parses and is
	// ignored while the column still lands (probed 5.7.44).
	result = t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN v INT CHECK(v--1>0);",
		provider, t10Policy(t, t10Rule{id: t12IgnoredNotice}))
	var notice bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			notice = true
		}
	}
	if !notice {
		t.Fatal("bare CHECK(v--1>0) must keep the parsed-and-ignored notice")
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CHECK(v--1>0);")
	if post == nil || !t12r1ColumnPresent(post, "v") {
		t.Fatal("bare CHECK(v--1>0) still lands column v on 5.7")
	}
	if t12r1FindCheckMember(t, post, "") != nil && len(post.Constraints) > 0 {
		for _, member := range post.Constraints {
			if member.Type == "check" {
				t.Fatalf("ignored arm stored a CHECK member: %+v", member)
			}
		}
	}
}

// A real `--` comment (space, tab, or newline after the dashes) still
// swallows everything to end-of-line — text inside it never reaches the
// clause scan, so the declaration stays bare.
func TestT12R2TailDashDashRealCommentStillComment(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"ALTER TABLE t ADD COLUMN v INT CHECK(v>0) -- NOT ENFORCED inside comment\n;",
		"ALTER TABLE t ADD COLUMN v INT CHECK(v>0) --\tNOT ENFORCED inside comment\n;",
	} {
		provider := &t10Provider{
			version: "5.7.44",
			tables: map[string]*spec.TableSnapshot{
				"t": t12Table("t", []string{"id"}),
			},
		}
		result := t10Audit(t, spec.DialectMySQL, sql,
			provider, t10Policy(t, t10Rule{id: t12IgnoredNotice}))
		var notice bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == t12IgnoredNotice {
				notice = true
			}
		}
		if !notice {
			t.Fatalf("real -- comment must still hide the authored clause — bare-CHECK notice owed: %q", sql)
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider, sql)
		if post == nil || !t12r1ColumnPresent(post, "v") {
			t.Fatalf("bare declaration behind a real comment still lands column v: %q", sql)
		}
	}
}

// --- Tail B: undecidable generic DROP unvouches the PK ---------------------

// A generic DROP that may bind the PK object — while other name spaces
// make the outcome undecidable — must synchronously unvouch the PK. The
// successor DROP PRIMARY KEY then owes a primary_key gap instead of
// answering on a falsely retained proof (injected fixture; the native
// PK-drop premise evidence is already archived).
func TestT12R2TailGenericDropUnvouchesPK(t *testing.T) {
	t.Parallel()
	fixture := func() *spec.TableSnapshot {
		return &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
			Columns:            []spec.Column{{Name: "id", Type: "int"}},
			PrimaryKey:         &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			ConstraintsUnknown: true,
			Indexes:            []spec.Index{},
		}
	}
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": fixture()},
	}
	// Statement 1 alone: the PK proof is unvouched — a member-level
	// degradation, not a whole-table tombstone.
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CONSTRAINT `primary`;")
	if post == nil {
		t.Fatal("undecidable generic drop must not tombstone the whole table")
	}
	if post.PrimaryKey != nil || !post.PrimaryKeyUnknown {
		t.Fatalf("PK possibly removed by the generic drop must be unvouched: %+v unknown=%v",
			post.PrimaryKey, post.PrimaryKeyUnknown)
	}
	if len(post.Columns) != 1 || post.Columns[0].Name != "id" {
		t.Fatalf("unvouching the PK must not disturb column facts: %+v", post.Columns)
	}

	// Public path over both statements: the successor owes the
	// primary_key gap and answers nothing on the stale proof.
	provider = &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": fixture()},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT `primary`; ALTER TABLE t DROP PRIMARY KEY;",
		provider, t10Policy(t, t10Rule{id: t12DropPKExists}))
	second := result.Statements[1]
	for _, f := range second.Findings {
		if f.RuleID == t12DropPKExists {
			t.Fatalf("undecidable predecessor still let the exists rule answer on a stale PK proof: %+v", f)
		}
	}
	var pkGap bool
	for _, g := range second.EvidenceGaps {
		if g.RuleID != t12DropPKExists {
			continue
		}
		for _, fact := range g.RequiredFacts {
			if fact == "target_table.primary_key" {
				pkGap = true
			}
		}
	}
	if !pkGap {
		t.Fatalf("successor DROP PRIMARY KEY must owe target_table.primary_key: gaps=%+v", second.EvidenceGaps)
	}

	// Control: an unrelated target name must not lower PK certainty.
	provider = &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": fixture()},
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CONSTRAINT ck;")
	if post == nil || post.PrimaryKey == nil || post.PrimaryKeyUnknown {
		t.Fatalf("unrelated generic target must keep the proven PK: %+v", post)
	}
	// Control: a proven-absent PK stays absent — stale unknown flags must
	// not reopen it.
	absentProvider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
				Columns:            []spec.Column{{Name: "id", Type: "int"}},
				ConstraintsUnknown: true,
				Indexes:            []spec.Index{},
			},
		},
	}
	post = t10r4PostState(t, spec.DialectMySQL, absentProvider,
		"ALTER TABLE t DROP CONSTRAINT ck;")
	if post == nil || post.PrimaryKey != nil || post.PrimaryKeyUnknown {
		t.Fatalf("proven-absent PK must not reopen on an unrelated undecidable drop: %+v", post)
	}
	// Control: the CHECK spelling only consults the check namespace —
	// `primary` is provably no member and the PK stays proven.
	provider = &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": fixture()},
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK `primary`;")
	if post == nil || post.PrimaryKey == nil || post.PrimaryKeyUnknown {
		t.Fatalf("CHECK-spelled `primary` must not disturb the PK proof: %+v", post)
	}
}

// When every other name space is vouched and only the PK answer is
// missing, the generic-binding gap must name the primary_key fact — not
// the blanket constraints/indexes pair.
func TestT12R2TailGenericDropGapNamesPrimaryKeyFact(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
				Columns:           []spec.Column{{Name: "id", Type: "int"}},
				PrimaryKeyUnknown: true,
				Indexes:           []spec.Index{},
				Constraints:       []spec.Constraint{},
			},
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT `primary`;",
		provider, t10Policy(t, t10Rule{id: t12DropCheckExists}))
	var gap *struct{ facts []string }
	for _, g := range result.Statements[0].EvidenceGaps {
		if g.RuleID == t12DropCheckExists {
			gap = &struct{ facts []string }{facts: g.RequiredFacts}
		}
	}
	if gap == nil {
		t.Fatalf("unproven PK binding must owe the exists-rule gap: %+v", result.Statements[0].EvidenceGaps)
	}
	if len(gap.facts) != 1 || gap.facts[0] != "target_table.primary_key" {
		t.Fatalf("only the PK fact is missing — RequiredFacts must be exactly [target_table.primary_key]: %+v", gap.facts)
	}
}

// --- Tail D: same-statement DROP CHECK lifts the identity premise ---------

// A same-statement proven DROP CHECK removes the referencing member
// before the identity validation — the rename/change is then allowed
// natively (MySQL docs: the check drop applies so the column change may
// proceed). The statement exceeds the bounded template — the ordered
// state stays withheld — but it must never claim a proven refusal that
// preserves the old member/column for the successor to consume.
func TestT12R2TailSameStmtDropCheckUnblocksIdentity(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"ALTER TABLE t DROP CHECK chk_a, RENAME COLUMN a TO a2;",
		"ALTER TABLE t RENAME COLUMN a TO a2, DROP CHECK chk_a;",
		"ALTER TABLE t DROP CHECK chk_a, CHANGE COLUMN a a2 INT;",
		"ALTER TABLE t CHANGE COLUMN a a2 INT, DROP CHECK chk_a;",
	} {
		provider := &t10Provider{
			version: "8.0.46",
			tables: map[string]*spec.TableSnapshot{
				"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
			},
		}
		// The statement must not claim a proven refusal preserving the old
		// member/column: either it lands both actions or stays withheld.
		post := t10r4PostState(t, spec.DialectMySQL, provider, sql)
		if post != nil && t12r1FindCheckMember(t, post, "chk_a") != nil {
			t.Fatalf("%q fabricated a refusal that keeps chk_a — the same-statement drop already removed it", sql)
		}
		if post != nil && t12r1ColumnPresent(post, "a") {
			t.Fatalf("%q fabricated a refusal that keeps column a — the rename may have landed", sql)
		}
		// Successor probe: a retained chk_a would let the drop answer
		// proven and strip it; withheld state owes a gap instead.
		provider = &t10Provider{
			version: "8.0.46",
			tables: map[string]*spec.TableSnapshot{
				"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
			},
		}
		result := t10Audit(t, spec.DialectMySQL,
			sql+" ALTER TABLE t DROP CHECK chk_a;",
			provider, t10Policy(t, t10Rule{id: t12DropCheckExists}))
		second := result.Statements[1]
		if len(second.EvidenceGaps) == 0 {
			t.Fatalf("%q left chk_a falsely proven — successor answers without a gap: findings=%+v",
				sql, second.Findings)
		}
	}
}

// The premise still holds when another proven member keeps referencing
// the column — dropping an unrelated check cannot unblock the rename.
func TestT12R2TailUnrelatedDropCheckKeepsIdentityRefusal(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"},
				t12CheckMember("chk_a", "enforced", "a"),
				t12CheckMember("chk_b", "enforced", "b")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK chk_b, RENAME COLUMN a TO a2;")
	if post == nil {
		t.Fatal("chk_a still references a — the 3959 refusal keeps the whole pre-state")
	}
	if !t12r1ColumnPresent(post, "a") || t12r1ColumnPresent(post, "a2") {
		t.Fatalf("refused rename still published a2: %+v", post.Columns)
	}
	if t12r1FindCheckMember(t, post, "chk_a") == nil || t12r1FindCheckMember(t, post, "chk_b") == nil {
		t.Fatalf("atomic refusal keeps every member: %+v", post.Constraints)
	}
	// A second referencing member keeps the refusal too — only dropping
	// every referencing check lifts the premise.
	provider = &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"},
				t12CheckMember("chk_a", "enforced", "a"),
				t12CheckMember("chk_a2", "enforced", "a")),
		},
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK chk_a, RENAME COLUMN a TO a2;")
	if post == nil {
		t.Fatal("chk_a2 still references a — the refusal keeps the pre-state")
	}
	if t12r1FindCheckMember(t, post, "chk_a2") == nil || t12r1ColumnPresent(post, "a2") {
		t.Fatalf("second referencing member must keep the refusal: %+v", post.Columns)
	}
}

// Guard the wording: a sibling drop on the CONSTRAINT spelling still
// lifts the premise the same way (the generic spelling removes the
// member too), and an unprovable drop target cannot prove the member
// leaves — the rename's fate is then undecidable, never a refusal.
func TestT12R2TailGenericSpellingAndUnprovenDrop(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2;")
	if post != nil && (t12r1FindCheckMember(t, post, "chk_a") != nil || t12r1ColumnPresent(post, "a")) {
		t.Fatalf("generic drop + rename must not fabricate a preserved pre-state: %+v", post)
	}
	provider = &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2; ALTER TABLE t DROP CHECK chk_a;",
		provider, t10Policy(t, t10Rule{id: t12DropCheckExists}))
	second := result.Statements[1]
	if len(second.EvidenceGaps) == 0 {
		t.Fatalf("generic drop + rename must not leave chk_a falsely proven: %+v", second.Findings)
	}
	// An unprovable drop binding (unknown constraints space) cannot prove
	// the member survives either — undecidable, not a proven refusal.
	provider = &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
				Columns:            []spec.Column{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
				ConstraintsUnknown: true,
				Indexes:            []spec.Index{},
			},
		},
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK chk_a, RENAME COLUMN a TO a2;")
	if post != nil && t12r1ColumnPresent(post, "a2") {
		t.Fatal("unprovable drop + rename must not fabricate the rename either")
	}
}

// --- Tail2 D: an unproven sibling drop cannot prop up a refusal -----------

// t12r2tAmbiguousCheckFixture builds the reviewer counterexample: a proven
// chk_a CHECK referencing column `a` plus a same-named index whose stored
// kind cannot be vouched. The generic DROP then binds unproven — the index
// may be secondary (the drop removes the check and the rename may land) or
// unique (the drop refuses 3939 and everything survives). Neither world is
// the proven refusal the buggy scan claimed.
func t12r2tAmbiguousCheckFixture(kind spec.IndexKind) *spec.TableSnapshot {
	snap := t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a"))
	snap.Indexes = []spec.Index{{Name: "chk_a", Kind: kind, Columns: []string{"a"}}}
	return snap
}

// Every undecidable-binding matrix cell must withhold the successor — never
// an exact pre-state the next statement can consume as proven.
func TestT12R2Tail2UnprovenSiblingDropWithholdsSuccessor(t *testing.T) {
	t.Parallel()
	for _, kind := range []spec.IndexKind{spec.IndexKindUnknown, spec.IndexKind("")} {
		for _, sql := range []string{
			"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2;",
			"ALTER TABLE t RENAME COLUMN a TO a2, DROP CONSTRAINT chk_a;",
			"ALTER TABLE t DROP CONSTRAINT chk_a, CHANGE COLUMN a a2 INT;",
			"ALTER TABLE t CHANGE COLUMN a a2 INT, DROP CONSTRAINT chk_a;",
		} {
			kind, sql := kind, sql
			t.Run(fmt.Sprintf("kind=%q/%s", kind, sql), func(t *testing.T) {
				t.Parallel()
				provider := &t10Provider{
					version: "8.0.46",
					tables:  map[string]*spec.TableSnapshot{"t": t12r2tAmbiguousCheckFixture(kind)},
				}
				post := t10r4PostState(t, spec.DialectMySQL, provider, sql)
				if post != nil {
					t.Fatalf("unproven generic drop cannot prove a refusal — kept an exact pre-state: %+v", post)
				}
			})
		}
	}
}

// The public two-statement path: after the undecidable statement the
// successor DROP CHECK must meet the unvouched table — a gap with the
// withheld-level RequiredFacts, unverified coverage, a review verdict —
// never a bound chk_a member answering silently, and never a fabricated
// missing finding.
func TestT12R2Tail2UnprovenSiblingDropSuccessorReport(t *testing.T) {
	t.Parallel()
	for _, kind := range []spec.IndexKind{spec.IndexKindUnknown, spec.IndexKind("")} {
		kind := kind
		t.Run(fmt.Sprintf("kind=%q", kind), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{
				version: "8.0.46",
				tables:  map[string]*spec.TableSnapshot{"t": t12r2tAmbiguousCheckFixture(kind)},
			}
			result := t10Audit(t, spec.DialectMySQL,
				"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2; ALTER TABLE t DROP CHECK chk_a;",
				provider, t10Policy(t, t10Rule{id: t12DropCheckExists}))
			second := result.Statements[1]
			for _, f := range second.Findings {
				if f.RuleID == t12DropCheckExists {
					t.Fatalf("the successor must not fabricate an exists finding on unvouched state: %+v", f)
				}
			}
			var existenceGap bool
			for _, g := range second.EvidenceGaps {
				if g.RuleID != t12DropCheckExists {
					continue
				}
				for _, fact := range g.RequiredFacts {
					if fact == "target_table.existence" {
						existenceGap = true
					}
				}
			}
			if !existenceGap {
				t.Fatalf("withheld successor owes target_table.existence under %s: gaps=%+v",
					t12DropCheckExists, second.EvidenceGaps)
			}
			if second.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("gap-bearing successor must be unverified, got %s", second.Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("evidence gap holds the review floor, got %s", result.Verdict)
			}
		})
	}
}

// Controls: only the maybe-dropped member's reference status distinguishes
// the outcomes — a proven-secondary same-name index keeps the tail1 unlock,
// a proven-unique one keeps the 3939/pre-state refusal, a definite second
// referencing member or an unrelated unproven drop keeps the identity
// refusal exactly.
func TestT12R2Tail2SiblingDropControls(t *testing.T) {
	t.Parallel()
	t.Run("proven secondary index keeps the unlock", func(t *testing.T) {
		t.Parallel()
		snap := t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a"))
		snap.Indexes = []spec.Index{{Name: "chk_a", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}}
		provider := &t10Provider{
			version: "8.0.46",
			tables:  map[string]*spec.TableSnapshot{"t": snap},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2;")
		if post != nil {
			t.Fatalf("proven generic drop of the check still lifts the premise — withheld, not refused: %+v", post)
		}
	})
	t.Run("proven unique index keeps the ambiguity refusal", func(t *testing.T) {
		t.Parallel()
		snap := t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a"))
		snap.Indexes = []spec.Index{{Name: "chk_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}}}
		provider := &t10Provider{
			version: "8.0.46",
			tables:  map[string]*spec.TableSnapshot{"t": snap},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2;")
		if post == nil {
			t.Fatal("ambiguous generic target provably refuses — the exact pre-state must survive")
		}
		if !t12r1ColumnPresent(post, "a") || t12r1FindCheckMember(t, post, "chk_a") == nil {
			t.Fatalf("refused statement keeps every member: %+v", post)
		}
	})
	t.Run("definite second referencing member keeps the refusal", func(t *testing.T) {
		t.Parallel()
		snap := t12Table("t", []string{"a", "b"},
			t12CheckMember("chk_a", "enforced", "a"),
			t12CheckMember("chk_b", "enforced", "a"))
		snap.Indexes = []spec.Index{{Name: "chk_a", Kind: spec.IndexKindUnknown, Columns: []string{"a"}}}
		provider := &t10Provider{
			version: "8.0.46",
			tables:  map[string]*spec.TableSnapshot{"t": snap},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP CONSTRAINT chk_a, RENAME COLUMN a TO a2;")
		if post == nil {
			t.Fatal("chk_b definitely remains and references a — the 3959 refusal is provable")
		}
		if !t12r1ColumnPresent(post, "a") || t12r1FindCheckMember(t, post, "chk_b") == nil {
			t.Fatalf("definite referencing member supports the refusal: %+v", post)
		}
	})
	t.Run("unrelated unproven drop keeps the refusal", func(t *testing.T) {
		t.Parallel()
		snap := t12Table("t", []string{"a", "b"},
			t12CheckMember("chk_a", "enforced", "a"),
			t12CheckMember("chk_x", "enforced", "b"))
		snap.Indexes = []spec.Index{{Name: "chk_x", Kind: spec.IndexKindUnknown, Columns: []string{"b"}}}
		provider := &t10Provider{
			version: "8.0.46",
			tables:  map[string]*spec.TableSnapshot{"t": snap},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP CONSTRAINT chk_x, RENAME COLUMN a TO a2;")
		if post == nil {
			t.Fatal("an unrelated unproven drop cannot lift the refusal chk_a supports")
		}
		if !t12r1ColumnPresent(post, "a") || t12r1FindCheckMember(t, post, "chk_a") == nil {
			t.Fatalf("chk_a definitely remains — the refusal keeps the pre-state: %+v", post)
		}
	})
}
