package audit

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12-R2 remediation cells (#91 reviewer groups A–D). Native anchors:
//   - MySQL 5.7.44: `enforced` is a legal identifier; a bare CHECK is
//     ignored while an authored [NOT] ENFORCED clause refuses 1064 —
//     comments never delimit the element body or alter clauses.
//   - MySQL 8.0.17: CHECK spelling executes; CONSTRAINT spelling 1064.
//   - MySQL 8.0.46: generic DROP of a dependent PK/UNIQUE refuses 1553;
//     in-column CHECK referencing another column refuses 3813; a
//     multi-action rename of a check-referenced column refuses 3959.
//   - TiDB v8.5.0 flag ON: in-column CHECK members never store (fresh or
//     duplicate names); CHANGE COLUMN drops every referencing check
//     member, single- or multi-column.

func t12r2SelfRefTable() *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:  true,
		Table:   &spec.Table{Schema: "golden", Name: "t"},
		Columns: []spec.Column{{Name: "id", Type: "int"}, {Name: "parent_id", Type: "int"}},
		PrimaryKey: &spec.Index{
			Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"},
		},
		Indexes: []spec.Index{
			{Name: "ix_parent", Kind: spec.IndexKindSecondary, Columns: []string{"parent_id"}},
		},
		Constraints: []spec.Constraint{
			{Type: "foreign_key", Name: "fk_parent", Columns: []string{"parent_id"},
				ReferencedTable: "t", ReferencedColumns: []string{"id"}},
		},
	}
}

func t12r2FindIndex(snap *spec.TableSnapshot, name string) *spec.Index {
	if snap == nil {
		return nil
	}
	return snap.FindProvenIndex(name)
}

// --- Group A: authored-clause attribution --------------------------------

// A1: `enforced` inside the CHECK expression is an identifier, not the
// clause — the declaration is a bare CHECK: ignored notice + table lands.
func TestT12R2EnforcedIdentifierNotClauseOn57(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE e1(enforced INT, CHECK(enforced > 0));", provider, policy)
	var notice bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			notice = true
		}
	}
	if !notice {
		t.Fatal("identifier 'enforced' must not masquerade as the ENFORCED clause — bare-CHECK ignored notice owed")
	}
	post := t12r1PostStateFor(t, spec.DialectMySQL, provider, "e1",
		"CREATE TABLE e1(enforced INT, CHECK(enforced > 0));")
	if post == nil {
		t.Fatal("5.7 identifier-only declaration wrongly withheld the table")
	}
	for _, member := range post.Constraints {
		if member.Type == "check" {
			t.Fatalf("ignored arm stored a CHECK member: %+v", member)
		}
	}
}

// A2: a comment carrying parens is not the CREATE element body — the
// authored NOT ENFORCED clause must still be attributed and refuse 1064.
func TestT12R2CommentParensNotElementBodyOn57(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE e2 /* (ignored) */ (id INT, CHECK(id > 0) NOT ENFORCED);", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			t.Fatalf("refused declaration misreported as parsed-and-ignored: %+v", f)
		}
	}
	post := t12r1PostStateFor(t, spec.DialectMySQL, provider, "e2",
		"CREATE TABLE e2 /* (ignored) */ (id INT, CHECK(id > 0) NOT ENFORCED);")
	if post != nil {
		t.Fatalf("comment-hidden explicit clause still published table e2: %+v", post)
	}
}

// A3: a comma inside a comment must not split the clause list — the second
// action keeps its CONSTRAINT spelling and the whole statement refuses
// atomically on 8.0.17 (1064): both members survive.
func TestT12R2CommentedCommaKeepsSpellingOn8017(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"},
				t12CheckMember("a", "enforced", "id"),
				t12CheckMember("b", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CHECK a /* , */ , DROP CONSTRAINT b;", provider, policy)
	var blocker bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12AlterCheckVersion {
			blocker = true
		}
	}
	if !blocker {
		t.Fatal("CONSTRAINT-spelled action lost its spelling — the 8.0.19 gate must refuse on 8.0.17")
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK a /* , */ , DROP CONSTRAINT b;")
	if post == nil {
		t.Fatal("proven whole-statement refusal must keep the pre-state, not tombstone")
	}
	if t12r1FindCheckMember(t, post, "a") == nil || t12r1FindCheckMember(t, post, "b") == nil {
		t.Fatalf("atomic 1064 refusal must keep both members: %+v", post.Constraints)
	}
}

// A4: TiDB flag OFF accepts and ignores an authored ENFORCED clause —
// the ignored notice is owed; only MySQL's ignored arm refuses it.
func TestT12R2TiDBOffExplicitClauseStillIgnored(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE TABLE n (a INT, CONSTRAINT chk_a CHECK(a > 0) ENFORCED);",
		"CREATE TABLE n (a INT, CONSTRAINT chk_a CHECK(a > 0) NOT ENFORCED);",
	} {
		provider := &t10Provider{version: "v8.5.0", tidbCheck: t12Bool(false), tables: map[string]*spec.TableSnapshot{}}
		policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
		result := t10Audit(t, spec.DialectTiDB, sql, provider, policy)
		var notice bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == t12IgnoredNotice {
				notice = true
			}
		}
		if !notice {
			t.Fatalf("TiDB flag-OFF explicit clause must stay parsed-and-ignored: %s", sql)
		}
		post := t12r1PostStateFor(t, spec.DialectTiDB, provider, "n", sql)
		if post == nil {
			t.Fatalf("TiDB flag-OFF wrongly withheld the table: %s", sql)
		}
		for _, member := range post.Constraints {
			if member.Type == "check" {
				t.Fatalf("TiDB flag-OFF stored a CHECK member: %+v", member)
			}
		}
	}
	// Bare-CHECK control keeps the same answer.
	provider := &t10Provider{version: "v8.5.0", tidbCheck: t12Bool(false), tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE n (a INT, CONSTRAINT chk_a CHECK(a > 0));", provider, policy)
	var notice bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			notice = true
		}
	}
	if !notice {
		t.Fatal("TiDB flag-OFF bare CHECK must keep the ignored notice")
	}
}

// --- Group B: binding must share refusal/unknown premises ----------------

// B1: a bound generic target still faces the native dependency refusals —
// DROP CONSTRAINT `primary` on a PK sole-covering a proven FK refuses
// 1553 exactly like DROP PRIMARY KEY (probed 8.0.46).
func TestT12R2GenericDropPrimaryRespectsFKDependency(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12r2SelfRefTable()},
	}
	policy := t10Policy(t, t10Rule{id: t12DropCheckExists})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT `primary`;", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12DropCheckExists {
			t.Fatalf("bound `primary` is not a missing-name case: %+v", f)
		}
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CONSTRAINT `primary`; ALTER TABLE t DROP PRIMARY KEY;")
	if post == nil {
		t.Fatal("proven 1553 refusal must keep the pre-state, not tombstone")
	}
	if post.PrimaryKey == nil {
		t.Fatal("1553 refusal removed the PK anyway")
	}
	// Control: the generic UNIQUE target sole-covering a proven FK also
	// refuses (probed 1553 on ux_p) — the member survives.
	provider2 := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "id", Type: "int"}, {Name: "parent_id", Type: "int"}},
				Indexes: []spec.Index{
					{Name: "ux_p", Kind: spec.IndexKindUnique, Columns: []string{"parent_id"}},
				},
				Constraints: []spec.Constraint{
					{Type: "foreign_key", Name: "fk2", Columns: []string{"parent_id"},
						ReferencedTable: "t", ReferencedColumns: []string{"parent_id"}},
				},
			},
		},
	}
	post2 := t10r4PostState(t, spec.DialectMySQL, provider2,
		"ALTER TABLE t DROP CONSTRAINT ux_p;")
	if post2 == nil {
		t.Fatal("proven 1553 refusal must keep the pre-state, not tombstone")
	}
	if t12r2FindIndex(post2, "ux_p") == nil {
		t.Fatal("generic DROP of sole-covering UNIQUE member removed it despite 1553")
	}
}

// B2: a same-name index of unknown kind leaves two live worlds — drop
// lands (index actually secondary) or 3939 refuses (index actually
// unique). The CHECK member's fate is undecidable: the successor owes a
// constraints gap, never a proven-answer.
func TestT12R2UnknownKindSameNameLeavesCheckUnvouched(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "id", Type: "int"}},
				Indexes: []spec.Index{
					{Name: "ck", Kind: spec.IndexKindUnknown, Columns: []string{"id"}},
				},
				Constraints: []spec.Constraint{
					t12CheckMember("ck", "enforced", "id"),
				},
			},
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckExists})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT ck; ALTER TABLE t ALTER CHECK ck NOT ENFORCED;",
		provider, policy)
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	second := result.Statements[1]
	var gapForConstraints bool
	for _, g := range second.EvidenceGaps {
		for _, fact := range g.RequiredFacts {
			if strings.Contains(fact, "constraints") {
				gapForConstraints = true
			}
		}
	}
	for _, f := range second.Findings {
		if f.RuleID == t12AlterCheckExists {
			t.Fatalf("undecidable predecessor still answered alter-check exists: %+v", f)
		}
	}
	if !gapForConstraints {
		t.Fatalf("successor owes a constraints gap after an undecidable generic drop: gaps=%+v findings=%+v",
			second.EvidenceGaps, second.Findings)
	}
	if second.Coverage.Status == report.CoverageComplete {
		t.Fatalf("undecidable generic drop must not leave complete coverage: %s", second.Coverage.Status)
	}
}

// B3: PrimaryKey=nil + PrimaryKeyUnknown means `primary` may bind — the
// generic spelling owes an unproven answer, not a proven-missing blocker.
func TestT12R2GenericPrimaryBindsPKUnknown(t *testing.T) {
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
	policy := t10Policy(t, t10Rule{id: t12DropCheckExists})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT `primary`;", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12DropCheckExists {
			t.Fatalf("PK presence unproven — `primary` is not a proven-missing blocker: %+v", f)
		}
	}
	var gap bool
	for _, g := range result.Statements[0].EvidenceGaps {
		if g.RuleID == t12DropCheckExists {
			gap = true
		}
	}
	if !gap {
		t.Fatalf("unproven PK binding must owe the exists-rule gap: %+v", result.Statements[0].EvidenceGaps)
	}
	// Control: CHECK spelling consults only the check namespace —
	// `primary` is provably no CHECK member even under PK unknown.
	result2 := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CHECK `primary`;", provider, policy)
	var missing bool
	for _, f := range result2.Statements[0].Findings {
		if f.RuleID == t12DropCheckExists {
			missing = true
		}
	}
	if !missing {
		t.Fatal("CHECK spelling must still prove `primary` missing from the check namespace")
	}
}

// --- Group C: in-column CHECK dialect truth -------------------------------

// C1: TiDB flag-ON never stores in-column CHECK members — the column
// lands while a fresh-named member is ignored (probed: catalog empty,
// SHOW CREATE lacks the member).
func TestT12R2TiDBInColumnFreshCheckNeverStores(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "v8.5.0", tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck_v CHECK(v > 0) NOT ENFORCED;")
	if post == nil || !t12r1ColumnPresent(post, "v") {
		t.Fatal("TiDB lands the column even though it ignores the inline CHECK")
	}
	if t12r1FindCheckMember(t, post, "ck_v") != nil {
		t.Fatal("TiDB in-column CHECK must never store — the member was fabricated")
	}
	// The successor reads the proven absence natively: DROP CHECK ck_v is
	// a proven-missing name, not an undecidable one.
	post = t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck_v CHECK(v > 0) NOT ENFORCED;"+
			" ALTER TABLE t ALTER CHECK ck_v NOT ENFORCED;")
	if post != nil && t12r1FindCheckMember(t, post, "ck_v") != nil {
		t.Fatal("successor must not find an in-column member TiDB never stored")
	}
	// Controls: table-level declarations and CREATE column checks still
	// land on the enforced arm.
	post2 := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t ADD CONSTRAINT ck_t CHECK(id > 0) NOT ENFORCED;")
	if post2 == nil || t12r1FindCheckMember(t, post2, "ck_t") == nil {
		t.Fatal("table-level ADD CONSTRAINT CHECK must still land on TiDB flag-ON")
	}
}

// C2: a MySQL in-column CHECK may reference only its own column — one
// referencing another column refuses 3813 atomically (probed 8.0.46):
// neither the column nor the member lands.
func TestT12R2MySQLInColumnCrossColumnRefRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck_v CHECK(v > id) NOT ENFORCED;"+
			" ALTER TABLE t DROP CHECK ck_v;")
	if post == nil {
		t.Fatal("proven 3813 refusal must keep the pre-state, not tombstone")
	}
	if t12r1ColumnPresent(post, "v") {
		t.Fatal("3813-refused ADD COLUMN still published column v")
	}
	if t12r1FindCheckMember(t, post, "ck_v") != nil {
		t.Fatal("3813-refused declaration still published member ck_v")
	}
	// Control: self-reference lands column and member.
	post2 := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck_v CHECK(v > 0) NOT ENFORCED;")
	if post2 == nil || !t12r1ColumnPresent(post2, "v") || t12r1FindCheckMember(t, post2, "ck_v") == nil {
		t.Fatal("self-referencing in-column CHECK must land column + member")
	}
	// Control: the same reference is legal at table level (existing
	// contract) when both columns already exist.
	provider3 := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id", "v"}),
		},
	}
	post3 := t10r4PostState(t, spec.DialectMySQL, provider3,
		"ALTER TABLE t ADD CONSTRAINT ck_v CHECK(v > id) NOT ENFORCED;")
	if post3 == nil || t12r1FindCheckMember(t, post3, "ck_v") == nil {
		t.Fatal("table-level CHECK(v > id) must still land when both columns exist")
	}
}

// --- Group D: multi-action rename/change shares the CHECK identity gate ---

// D: RENAME COLUMN on a check-referenced column refuses 3959 — the
// whole multi-action statement fails atomically; the sibling ADD COLUMN
// must not land (probed 8.0.46: columns stay a,b).
func TestT12R2MultiActionRenameCheckRefAtomic(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t RENAME COLUMN a TO a2, ADD COLUMN c INT;")
	if post == nil {
		t.Fatal("proven 3959 refusal must keep the pre-state, not tombstone")
	}
	if !t12r1ColumnPresent(post, "a") || t12r1ColumnPresent(post, "a2") || t12r1ColumnPresent(post, "c") {
		t.Fatalf("atomic 3959 refusal must keep columns a,b only: %+v", post.Columns)
	}
	if t12r1FindCheckMember(t, post, "chk_a") == nil {
		t.Fatal("3959 refusal must keep the referencing check member")
	}
	// Control: renaming an unreferenced column + add lands both actions.
	post2 := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t RENAME COLUMN b TO b2, ADD COLUMN c INT;")
	if post2 == nil || !t12r1ColumnPresent(post2, "b2") || !t12r1ColumnPresent(post2, "c") {
		t.Fatalf("unrelated rename+add must land: %+v", post2)
	}
}

// CHANGE COLUMN hits the same gate — MySQL refuses 3959 atomically.
func TestT12R2MultiActionChangeCheckRefAtomic(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_a", "enforced", "a")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t CHANGE COLUMN a a2 INT, ADD COLUMN c INT;")
	if post == nil {
		t.Fatal("proven 3959 refusal must keep the pre-state, not tombstone")
	}
	if !t12r1ColumnPresent(post, "a") || t12r1ColumnPresent(post, "a2") || t12r1ColumnPresent(post, "c") {
		t.Fatalf("atomic 3959 refusal must keep columns a,b only: %+v", post.Columns)
	}
	if t12r1FindCheckMember(t, post, "chk_a") == nil {
		t.Fatal("3959 refusal must keep the referencing check member")
	}
}

// TiDB flag-ON CHANGE silently drops referencing members — including a
// multi-column reference (probed v8.5.0: chk_ab CHECK(a<b) disappears,
// never left citing the old name).
func TestT12R2TiDBMultiActionChangeDropsMultiColCheck(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "v8.5.0", tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"a", "b"}, t12CheckMember("chk_ab", "enforced", "a", "b")),
		},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t CHANGE COLUMN a a2 INT, ADD COLUMN c INT;")
	if post == nil {
		t.Fatal("TiDB CHANGE lands the rename + add atomically")
	}
	if !t12r1ColumnPresent(post, "a2") || !t12r1ColumnPresent(post, "c") {
		t.Fatalf("TiDB CHANGE+ADD must land a2 and c: %+v", post.Columns)
	}
	for _, member := range post.Constraints {
		if member.Type == "check" {
			t.Fatalf("TiDB CHANGE must drop the referencing member, not leave it citing a dropped name: %+v", member)
		}
	}
}
