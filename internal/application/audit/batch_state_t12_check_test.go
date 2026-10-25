package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12 CHECK-constraint semantics: enforcement facts on normalized members,
// the alter_check transition, dialect/version/runtime boundaries, and the
// ordered post-state — all through the public Audit path.

const (
	t12NotEnforcedNotice = "ddl.constraint.check.not_enforced.notice"
	t12IgnoredNotice     = "ddl.constraint.check.ignored.notice"
	t12AlterCheckExists  = "ddl.alter.alter_check.exists.require"
	t12DropCheckExists   = "ddl.alter.drop_check.exists.require"
	t12AlterCheckVersion = "ddl.alter.alter_check.version.require"
	t12CheckNamePrefix   = "ddl.constraint.check.name.prefix.require"
)

func t12CheckMember(name, enforced string, columns ...string) spec.Constraint {
	return spec.Constraint{Type: "check", Name: name, Enforced: enforced, Columns: columns}
}

func t12Table(name string, columns []string, checks ...spec.Constraint) *spec.TableSnapshot {
	cols := make([]spec.Column, 0, len(columns))
	for _, c := range columns {
		cols = append(cols, spec.Column{Name: c, Type: "int"})
	}
	constraints := make([]spec.Constraint, 0, len(checks))
	constraints = append(constraints, checks...)
	return &spec.TableSnapshot{
		Exists:  true,
		Table:   &spec.Table{Schema: "golden", Name: name},
		Columns: cols, Constraints: constraints,
	}
}

func t12Bool(v bool) *bool { return &v }

// The first Golden Path (#91): ALTER CHECK chk_id NOT ENFORCED on a proven
// enforcing member must publish enforced=not_enforced in the post-state and
// emit the isolated risk finding — neither may be silent.
func TestT12AlterCheckNotEnforcedGoldenPath(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;", provider, policy)

	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d, want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 {
		t.Fatalf("findings=%v, want exactly 1 not_enforced finding", stmt.Findings)
	}
	f := stmt.Findings[0]
	if f.RuleID != t12NotEnforcedNotice || f.Level != "blocker" {
		t.Fatalf("finding=%+v, want %s blocker", f, t12NotEnforcedNotice)
	}
	if f.Metadata["table"] != "t" || f.Metadata["constraint"] != "chk_id" || f.Metadata["enforced"] != "not_enforced" {
		t.Fatalf("finding metadata=%v, want table=t constraint=chk_id enforced=not_enforced", f.Metadata)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage=%q, want complete", stmt.Coverage.Status)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%q, want reject", result.Verdict)
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk_id")
	if member == nil {
		t.Fatalf("post-state lost the proven check member: %+v", post.Constraints)
	}
	if member.Enforced != "not_enforced" {
		t.Fatalf("post-state member Enforced=%q, want not_enforced", member.Enforced)
	}
}

// Counter-case: the ENFORCED direction must not emit the disabled-state
// finding; the member survives but its enforcement state becomes unproven
// (the flip is data-conditional — native 3819 can refuse it).
func TestT12AlterCheckEnforcedReverseDoesNotFire(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "not_enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CHECK chk_id ENFORCED;", provider, policy)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("ENFORCED direction fired %v, want 0 findings", result.Statements[0].Findings)
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CHECK chk_id ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk_id")
	if member == nil {
		t.Fatal("post-state lost the check member")
	}
	if member.Enforced != "" {
		t.Fatalf("data-conditional flip left Enforced=%q, want unproven (empty)", member.Enforced)
	}
}

// Counter-case: an ordinary column change must not fire the check
// enforcement finding.
func TestT12OrdinaryColumnChangeDoesNotFire(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN v INT;", provider, policy)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("column add fired %v, want 0 findings", result.Statements[0].Findings)
	}
}

// ALTER CHECK on a proven-missing name is a refused statement (native 3821)
// — the exists rule reports it; ALTER CONSTRAINT alias binds identically.
func TestT12AlterCheckMissingNameExistsFinding(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckExists})
	for _, sql := range []string{
		"ALTER TABLE t ALTER CHECK missing_x NOT ENFORCED;",
		"ALTER TABLE t ALTER CONSTRAINT missing_x ENFORCED;",
	} {
		result := t10Audit(t, spec.DialectMySQL, sql, provider, policy)
		stmt := result.Statements[0]
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12AlterCheckExists {
			t.Fatalf("%q findings=%v, want 1 %s finding", sql, stmt.Findings, t12AlterCheckExists)
		}
	}
}

// A proven non-CHECK name under ALTER CHECK refuses identically (3821) —
// the unique member keeps its state and the exists rule fires.
func TestT12AlterCheckNonCheckNameExistsFinding(t *testing.T) {
	t.Parallel()
	table := t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id"))
	table.Indexes = []spec.Index{{Name: "uk_v", Kind: spec.IndexKindUnique, Columns: []string{"id"}}}
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckExists})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER CHECK uk_v NOT ENFORCED;", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12AlterCheckExists {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12AlterCheckExists)
	}
	// The unique member must survive untouched.
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CHECK uk_v NOT ENFORCED;")
	if post == nil || post.FindProvenIndex("uk_v") == nil {
		t.Fatal("non-check target lost its proven member state")
	}
}

// MySQL 5.7 parses CHECK declarations but ignores them — the ignored notice
// reveals the accepted-but-ignored arm and no enforcement claim is made.
func TestT12MySQL57CheckDeclarationIgnoredReveal(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice, level: "warning"})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t57 (id INT, CONSTRAINT chk_i CHECK (id > 0));", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12IgnoredNotice {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12IgnoredNotice)
	}
	// The ignored member must not land in the derived post-state.
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"CREATE TABLE t57 (id INT, CONSTRAINT chk_i CHECK (id > 0));")
	if post != nil && len(post.Constraints) != 0 {
		t.Fatalf("ignored check published members on 5.7: %+v", post.Constraints)
	}
}

// MySQL 5.7 rejects ALTER|DROP CHECK syntax (native 1064) — the version rule
// reveals the refusal; state stays provably unchanged.
func TestT12MySQL57AlterCheckSyntaxRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "5.7.44",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	policy := t10Policy(t, t10Rule{id: t12AlterCheckVersion})
	for _, sql := range []string{
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;",
		"ALTER TABLE t DROP CHECK chk_id;",
	} {
		result := t10Audit(t, spec.DialectMySQL, sql, provider, policy)
		stmt := result.Statements[0]
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12AlterCheckVersion {
			t.Fatalf("%q findings=%v, want 1 %s finding", sql, stmt.Findings, t12AlterCheckVersion)
		}
	}
}

// TiDB flag OFF parses CHECK but stores/enforces nothing — declarations get
// the ignored reveal; ALTER CHECK against the never-stored member is a no-op.
func TestT12TiDBFlagOffIgnoredArm(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version:   "v8.5.0",
		tidbCheck: t12Bool(false),
		tables:    map[string]*spec.TableSnapshot{},
	}
	policy := t10Policy(t, t10Rule{id: t12IgnoredNotice, level: "warning"})
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE tt (id INT, CONSTRAINT chk_t CHECK (id > 0));", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12IgnoredNotice {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12IgnoredNotice)
	}
}

// TiDB flag ON mirrors MySQL: ALTER CHECK flips the stored member and the
// risk finding fires.
func TestT12TiDBFlagOnAlterCheckNotEnforced(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version:   "v8.5.0",
		tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12NotEnforcedNotice {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12NotEnforcedNotice)
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk_id")
	if member == nil || member.Enforced != "not_enforced" {
		t.Fatalf("post-state member=%+v, want chk_id enforced=not_enforced", member)
	}
}

// ADD CONSTRAINT ... CHECK ... NOT ENFORCED declares the member already
// disabled — the same risk finding fires and the member lands disabled.
func TestT12AddCheckNotEnforcedDeclaresDisabled(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT chk2 CHECK (id < 100) NOT ENFORCED;", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12NotEnforcedNotice {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12NotEnforcedNotice)
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT chk2 CHECK (id < 100) NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk2")
	if member == nil || member.Enforced != "not_enforced" {
		t.Fatalf("post-state member=%+v, want chk2 enforced=not_enforced", member)
	}
}

// Unobserved TiDB flag keeps every answer undecidable: no finding, an
// attributable evidence gap, and a review verdict — never a fabricated arm.
func TestT12TiDBFlagUnobservedGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "v8.5.0",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	policy := t10Policy(t, t10Rule{id: t12NotEnforcedNotice, level: "blocker"})
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("unobserved flag fabricated findings: %v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 {
		t.Fatal("unobserved flag owes an evidence gap")
	}
	if result.Verdict == report.VerdictPass {
		t.Fatalf("verdict=%q, want review floor", result.Verdict)
	}
}

// ALTER TABLE ADD CONSTRAINT ... CHECK names join the check naming family —
// the payload lives on alter.Constraint, never on DDL.Constraints.
func TestT12AlterAddCheckNameJudged(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	policy := t10Policy(t, t10Rule{id: t12CheckNamePrefix, level: "warning", params: "      prefix: chk_\n"})
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT badname CHECK (id > 0);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t12CheckNamePrefix {
		t.Fatalf("findings=%v, want 1 %s finding", stmt.Findings, t12CheckNamePrefix)
	}
}
