package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12-R1 remediation cells (#91 reviewer groups A–E). Native anchors:
//   - MySQL 8.0.17 (in-band anchor): ALTER|DROP CHECK executes; the
//     CONSTRAINT alias refuses 1064 until 8.0.19.
//   - MySQL 5.7.44: a bare CHECK declaration is parsed and ignored, but an
//     authored ENFORCED/NOT ENFORCED clause refuses 1064.
//   - MySQL 8.0.46: DROP|ALTER CHECK on a secondary index refuses 3821;
//     the generic CONSTRAINT keyword refuses missing names 3940, a
//     cross-type same-name pair 3939, and non-check alteration 3941.
//   - TiDB v8.5.0 flag ON: the generic keyword binds the CHECK member
//     first (no 3939) and an in-column duplicate CHECK name is silently
//     ignored while the column itself lands.

func t12r1Index(name string, kind spec.IndexKind, columns ...string) spec.Index {
	return spec.Index{Name: name, Kind: kind, Columns: columns}
}

func t12r1FindCheckMember(t *testing.T, snap *spec.TableSnapshot, name string) *spec.Constraint {
	t.Helper()
	if snap == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	return snap.FindCheckConstraint(name)
}

func t12r1ColumnPresent(snap *spec.TableSnapshot, name string) bool {
	if snap == nil {
		return false
	}
	for _, col := range snap.Columns {
		if strings.EqualFold(col.Name, name) {
			return true
		}
	}
	return false
}

// t12r1PostStateFor is t10r4PostState parametric on the target table —
// create-side refusal cells need the post-state of a table other than the
// fixture's seeded "t".
func t12r1PostStateFor(t *testing.T, dialect spec.Dialect, provider MetadataProvider, table, sql string) *spec.TableSnapshot {
	t.Helper()
	state := newBatchState(dialect, "golden", provider)
	facts, err := provider.LoadInstanceFacts(context.Background(), dialect, "golden")
	if err != nil {
		t.Fatalf("instance facts: %v", err)
	}
	state.instanceFacts = facts
	state.resolvedVersion = spec.ParseObservedVersion(facts.Version)
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
	entry := state.entries[state.keyFor("golden", spec.Table{Name: table})]
	if entry == nil || entry.state != tablePresent {
		return nil
	}
	return entry.shape
}

// --- Group A: spelling-aware version gate -------------------------------

// 8.0.17 lies inside the band where ALTER CHECK executes but the
// ALTER CONSTRAINT alias refuses. The version gate must split the two
// spellings the authored text actually used.
func TestT12R1AlterCheckCheckSpellingOn8017Executes(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion}, t10Rule{id: t12AlterCheckExists})
	_ = policy
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CHECK chk NOT ENFORCED;", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12AlterCheckVersion {
			t.Fatalf("8.0.17 ALTER CHECK misreported as version refusal: %+v", f)
		}
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CHECK chk NOT ENFORCED;")
	member := t12r1FindCheckMember(t, post, "chk")
	if member == nil || member.Enforced != "not_enforced" {
		t.Fatalf("8.0.17 ALTER CHECK post-state member=%+v, want enforced=not_enforced", member)
	}
}

func TestT12R1AlterConstraintSpellingOn8017Refused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CONSTRAINT chk NOT ENFORCED;", provider, policy)
	var found bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12AlterCheckVersion {
			found = true
			if f.Metadata["required_version"] != "8.0.19" {
				t.Fatalf("generic spelling required_version=%v, want 8.0.19", f.Metadata["required_version"])
			}
		}
	}
	if !found {
		t.Fatalf("8.0.17 ALTER CONSTRAINT must refuse 1064 — version finding missing: %+v", result.Statements[0].Findings)
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CONSTRAINT chk NOT ENFORCED;")
	member := t12r1FindCheckMember(t, post, "chk")
	if member == nil || member.Enforced != "enforced" {
		t.Fatalf("refused ALTER CONSTRAINT mutated member=%+v, want enforced kept", member)
	}
}

func TestT12R1DropCheckSpellingOn8017(t *testing.T) {
	t.Parallel()
	checkProvider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, checkProvider,
		"ALTER TABLE t DROP CHECK chk;")
	if t12r1FindCheckMember(t, post, "chk") != nil {
		t.Fatal("8.0.17 DROP CHECK must remove the member")
	}
	constraintProvider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT chk;", constraintProvider, policy)
	var found bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12AlterCheckVersion {
			found = true
		}
	}
	if !found {
		t.Fatal("8.0.17 DROP CONSTRAINT must refuse 1064 — version finding missing")
	}
	post = t10r4PostState(t, spec.DialectMySQL, constraintProvider,
		"ALTER TABLE t DROP CONSTRAINT chk;")
	if t12r1FindCheckMember(t, post, "chk") == nil {
		t.Fatal("refused DROP CONSTRAINT removed the member")
	}
}

// The version rule being disabled must never let the ordered state publish
// a flip the server provably refused.
func TestT12R1ConstraintSpellingRefusalIsPolicyIndependent(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.17",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion, params: "      required: false\n"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CONSTRAINT chk NOT ENFORCED;", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12AlterCheckVersion {
			t.Fatalf("disabled version rule still fired: %+v", f)
		}
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CONSTRAINT chk NOT ENFORCED;")
	member := t12r1FindCheckMember(t, post, "chk")
	if member == nil || member.Enforced != "enforced" {
		t.Fatalf("disabled version rule published a refused flip: %+v", member)
	}
}

// MySQL 5.7 distinguishes a bare CHECK declaration (parsed and ignored)
// from an authored ENFORCED/NOT ENFORCED clause (1064 refusal — the whole
// statement, including sibling actions, lands nothing).
func TestT12R1ExplicitEnforcedDeclOn57Refuses(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "5.7.44",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT c1 CHECK(id>0) ENFORCED;", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			t.Fatalf("refused declaration misreported as parsed-and-ignored: %+v", f)
		}
	}
	// The refusal is statement-atomic: the sibling column must not land.
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT c1 CHECK(id>0) ENFORCED, ADD COLUMN v INT;")
	if t12r1ColumnPresent(post, "v") {
		t.Fatal("5.7 explicit-ENFORCED refusal still published sibling column v")
	}
	// The inline-CHECK spelling refuses the same way (probed 5.7.44:
	// ALTER ADD COLUMN ... CHECK NOT ENFORCED answers 1064) — the added
	// column shares the refusal and never lands.
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN w INT CHECK(w>0) NOT ENFORCED;")
	if t12r1ColumnPresent(post, "w") {
		t.Fatal("5.7 inline explicit-ENFORCED refusal still published column w")
	}
}

func TestT12R1ExplicitEnforcedCreateOn57Refuses(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE n1 (a INT, CHECK(a>0) NOT ENFORCED);", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			t.Fatalf("refused CREATE declaration misreported as ignored: %+v", f)
		}
	}
	// The 1064 refusal fails the statement — the table itself never lands,
	// so a successor statement against it must still see it absent.
	post := t12r1PostStateFor(t, spec.DialectMySQL, provider, "n1",
		"CREATE TABLE n1 (a INT, CHECK(a>0) NOT ENFORCED);")
	if post != nil {
		t.Fatalf("refused CREATE still published table n1: %+v", post)
	}
}

func TestT12R1BareDeclOn57StillIgnored(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE n2 (a INT, CHECK(a>0));", provider, policy)
	var found bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12IgnoredNotice {
			found = true
		}
	}
	if !found {
		t.Fatal("bare CHECK on 5.7 must keep the parsed-and-ignored notice")
	}
	// The ignored declaration still lets the table land — the check member
	// alone is silently dropped.
	post := t12r1PostStateFor(t, spec.DialectMySQL, provider, "n2",
		"CREATE TABLE n2 (a INT, CHECK(a>0));")
	if post == nil {
		t.Fatal("bare-CHECK CREATE wrongly withheld the table on 5.7")
	}
	for _, member := range post.Constraints {
		if member.Type == "check" {
			t.Fatalf("ignored arm stored a CHECK member: %+v", member)
		}
	}
}

// --- Group B: target type + same-name binding ---------------------------

// B1: a secondary index is outside the constraint namespace under both
// spellings — 3821/3940 proven refusal, never a clean pass.
func TestT12R1DropCheckOnSecondaryIndexRefuses(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"ALTER TABLE t DROP CHECK ix_u;",
		"ALTER TABLE t DROP CONSTRAINT ix_u;",
	} {
		provider := &t10Provider{
			version: "8.0.46",
			tables: map[string]*spec.TableSnapshot{
				"t": {
					Exists:  true,
					Table:   &spec.Table{Schema: "golden", Name: "t"},
					Columns: []spec.Column{{Name: "u", Type: "int"}},
					Indexes: []spec.Index{t12r1Index("ix_u", spec.IndexKindSecondary, "u")},
				},
			},
		}
		policy := t10Policy(t, t10Rule{id: t12DropCheckExists})
		result := t10Audit(t, spec.DialectMySQL, sql, provider, policy)
		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == t12DropCheckExists {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s on a secondary index must refuse (3821/3940) — no finding: %+v", sql, result.Statements[0].Findings)
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider, sql)
		if post == nil || len(post.Indexes) != 1 {
			t.Fatalf("%s mutated the index set: %+v", sql, post)
		}
	}
}

// B2: the generic keyword on a cross-type same-name pair refuses 3939 on
// MySQL — the statement publishes nothing and the successor still sees the
// CHECK member.
func TestT12R1GenericDropSameNameAmbiguityRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "a", Type: "int"}},
				Indexes: []spec.Index{t12r1Index("ck", spec.IndexKindUnique, "a")},
				Constraints: []spec.Constraint{
					t12CheckMember("ck", "enforced", "a"),
				},
			},
		},
	}
	policy := t10Policy(t,
		t10Rule{id: t12DropCheckExists},
		t10Rule{id: t12AlterCheckExists},
		t10Rule{id: t12NotEnforcedNotice})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT ck;\nALTER TABLE t ALTER CHECK ck NOT ENFORCED;", provider, policy)
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d, want 2", len(result.Statements))
	}
	var found bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12DropCheckExists {
			found = true
		}
	}
	if !found {
		t.Fatalf("DROP CONSTRAINT on a same-name CHECK+UNIQUE pair must refuse 3939: %+v", result.Statements[0].Findings)
	}
	// The refused first statement leaves the member; the successor sees it
	// in its own pre-state and flips it normally.
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CONSTRAINT ck;\nALTER TABLE t ALTER CHECK ck NOT ENFORCED;")
	member := t12r1FindCheckMember(t, post, "ck")
	if member == nil || member.Enforced != "not_enforced" {
		t.Fatalf("successor after refused generic drop sees member=%+v, want not_enforced", member)
	}
	if post == nil || len(post.Indexes) != 1 {
		t.Fatalf("refused generic drop touched the unique index: %+v", post)
	}
}

// The CHECK keyword binds the CHECK namespace only — same-name unique
// member survives untouched.
func TestT12R1CheckSpellingDropsCheckKeepsUnique(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "a", Type: "int"}},
				Indexes: []spec.Index{t12r1Index("ck", spec.IndexKindUnique, "a")},
				Constraints: []spec.Constraint{
					t12CheckMember("ck", "enforced", "a"),
				},
			},
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP CHECK ck;")
	if t12r1FindCheckMember(t, post, "ck") != nil {
		t.Fatal("DROP CHECK ck must remove the check member")
	}
	if len(post.Indexes) != 1 {
		t.Fatalf("DROP CHECK ck touched the same-name unique index: %+v", post.Indexes)
	}
}

func TestT12R1GenericAlterSameNameAmbiguityRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "a", Type: "int"}},
				Indexes: []spec.Index{t12r1Index("ck", spec.IndexKindUnique, "a")},
				Constraints: []spec.Constraint{
					t12CheckMember("ck", "enforced", "a"),
				},
			},
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CONSTRAINT ck NOT ENFORCED;")
	member := t12r1FindCheckMember(t, post, "ck")
	if member == nil || member.Enforced != "enforced" {
		t.Fatalf("3939-refused generic alter flipped member=%+v, want enforced kept", member)
	}
}

// --- Group C: DROP COLUMN cascade proof ---------------------------------

// A check member provably referencing several columns refuses the drop
// (3959) — the column and the member both stay, and the successor sees
// the member.
func TestT12R1MultiColumnCheckBlocksDropColumn(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
				Constraints: []spec.Constraint{
					{Type: "check", Name: "chk_ab", Enforced: "enforced", Columns: []string{"a", "b"},
						Expression: &spec.ExpressionFacts{Known: true}},
				},
			},
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP COLUMN a;")
	if post == nil {
		t.Fatal("3959-refused drop tombstoned the table — a proven refusal keeps the pre-state")
	}
	if !t12r1ColumnPresent(post, "a") {
		t.Fatal("3959-refused drop removed column a")
	}
	if t12r1FindCheckMember(t, post, "chk_ab") == nil {
		t.Fatal("3959-refused drop removed the multi-column check member")
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP COLUMN a;\nALTER TABLE t DROP CHECK chk_ab;")
	if t12r1FindCheckMember(t, post, "chk_ab") != nil {
		t.Fatal("successor DROP CHECK must remove the still-present member")
	}
}

// A partially-modeled expression cannot vouch single-column cascade even
// when the modeled part hits the column — withhold, never fabricate.
func TestT12R1UnmodeledCheckExpressionNeverCascades(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
				Constraints: []spec.Constraint{
					{Type: "check", Name: "chk_x", Enforced: "enforced", Columns: []string{"a"},
						Expression: &spec.ExpressionFacts{Known: true}, UnmodeledParts: 1},
				},
			},
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t DROP COLUMN a;")
	if post != nil && t12r1ColumnPresent(post, "a") == false && post.Constraints != nil && !post.ConstraintsUnknown {
		t.Fatalf("unvouched reference set still produced a clean cascade: %+v", post)
	}
}

// --- Group D: ADD prerequisites and atomic publication ------------------

// D1: a check whose referenced column is proven absent refuses the whole
// statement (1054) — no member lands and the successor sees the proven
// absence.
func TestT12R1AddCheckMissingColumnRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT ck_ghost CHECK(ghost>0) NOT ENFORCED;")
	if t12r1FindCheckMember(t, post, "ck_ghost") != nil {
		t.Fatal("1054-refused ADD CHECK published the ghost member")
	}
	policy := t10Policy(t, t10Rule{id: t12DropCheckExists})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT ck_ghost CHECK(ghost>0) NOT ENFORCED;\nALTER TABLE t DROP CHECK ck_ghost;",
		provider, policy)
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d, want 2", len(result.Statements))
	}
	var found bool
	for _, f := range result.Statements[1].Findings {
		if f.RuleID == t12DropCheckExists {
			found = true
		}
	}
	if !found {
		t.Fatalf("successor DROP CHECK must see the proven absence: %+v", result.Statements[1].Findings)
	}
}

// D2: an in-column CHECK name colliding with a proven member refuses the
// whole statement on MySQL — the new column must not land, and the
// constraint collection keeps its proof.
func TestT12R1InColumnDuplicateNameRefusesAtomically(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("ck", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck CHECK(v>0) NOT ENFORCED;")
	if post == nil {
		t.Fatal("3822 refusal must keep the proven pre-state, not tombstone")
	}
	if t12r1ColumnPresent(post, "v") {
		t.Fatal("3822-refused statement still published column v")
	}
	if post.ConstraintsUnknown {
		t.Fatal("3822 refusal degraded the proven constraint collection")
	}
	if len(post.Constraints) != 1 {
		t.Fatalf("constraint set mutated: %+v", post.Constraints)
	}
	post = t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck CHECK(v>0) NOT ENFORCED;\nALTER TABLE t DROP COLUMN v;")
	if t12r1ColumnPresent(post, "v") {
		t.Fatal("successor must still see v absent")
	}
}

// TiDB flag-ON diverges: the in-column duplicate is silently ignored but
// the column lands — no atomic refusal.
func TestT12R1TiDBInColumnDuplicateLandsColumn(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("ck", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT ck CHECK(v>0) NOT ENFORCED;")
	if post == nil || !t12r1ColumnPresent(post, "v") {
		t.Fatalf("TiDB flag-ON must land the column while ignoring the duplicate check: %+v", post)
	}
	if post.ConstraintsUnknown {
		t.Fatal("TiDB silent-ignore must not degrade the proven constraint collection")
	}
}

// --- Group E: in-column check naming governance -------------------------

func TestT12R1InColumnCheckNameGoverned(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12CheckNamePrefix, params: "      prefix: chk_\n"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT bad CHECK(v>0);", provider, policy)
	var found bool
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12CheckNamePrefix {
			found = true
		}
	}
	if !found {
		t.Fatalf("in-column CHECK name escaped naming governance: %+v", result.Statements[0].Findings)
	}
	result = t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN v INT CONSTRAINT chk_v CHECK(v>0);", provider, policy)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t12CheckNamePrefix {
			t.Fatalf("compliant in-column CHECK name fired: %+v", f)
		}
	}
}
