// Package audit covers the R1-F TiDB DROP CHECK|CONSTRAINT seam through
// the public audit path: one shared AST shape, two authored keywords, and
// a target-type-decided post-state that never guesses the keyword.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r1FKShape returns a present table carrying a proven foreign key and
// its supporting index — the shape the drop seam must either keep or
// honestly unvouch.
func t11r1FKShape() *spec.TableSnapshot {
	shape := t10r2Table("t", []spec.Column{t10r2Col("c", "int", false, 0)})
	shape.Constraints = []spec.Constraint{{
		Type: "foreign_key", Name: "fk_c", Columns: []string{"c"},
		ReferencedTable: "p", ReferencedColumns: []string{"id"},
	}}
	shape.Indexes = []spec.Index{{Name: "fk_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}}
	return shape
}

// TiDB DROP CONSTRAINT on a proven foreign-key name is a provable no-op
// (v8.5.0: warned no-op under tidb_enable_check_constraint=OFF, 3940 under
// ON — the member survives either way). The successor still binds it.
func TestT11R1FTiDBDropConstraintKeepsProvenForeignKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{
		"t": t11r1FKShape()}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP CONSTRAINT fk_c;"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the member survived the no-op and the drop lands it — no finding, got %+v", drop.Findings)
	}
}

// TiDB DROP CONSTRAINT with a name nothing proven binds is equally a
// no-op — the shape and its vouched member set keep their facts.
func TestT11R1FTiDBDropConstraintUnmatchedKeepsFacts(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{
		"t": t11r1FKShape()}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t DROP CONSTRAINT nosuch;"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("an unmatched constraint name kept every member — drop lands, got %+v", drop.Findings)
	}
}

// The DROP FOREIGN KEY comparison: TiDB honors that spelling natively —
// the member goes while its supporting index stays.
func TestT11R1FTiDBDropForeignKeyRemovesMemberKeepsIndex(t *testing.T) {
	t.Parallel()
	entry := t11Entry(t11r1Apply(t, spec.DialectTiDB,
		map[string]*spec.TableSnapshot{"t": t11r1FKShape()}, nil,
		"ALTER TABLE t DROP FOREIGN KEY fk_c;"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept, got %+v", entry)
	}
	if len(entry.shape.Constraints) != 0 {
		t.Fatalf("DROP FOREIGN KEY removes the member, got %+v", entry.shape.Constraints)
	}
	if len(entry.shape.Indexes) != 1 {
		t.Fatalf("the supporting index survives, got %+v", entry.shape.Indexes)
	}
}

// TiDB add-then-DROP-CONSTRAINT keeps the landed member — the add's
// supporting index stays too, matching the provable no-op semantics.
func TestT11R1FTiDBAddThenDropConstraintKeepsMemberAndIndex(t *testing.T) {
	t.Parallel()
	child := t11r1Child("c")
	entry := t11Entry(t11r1Apply(t, spec.DialectTiDB,
		map[string]*spec.TableSnapshot{"t": child},
		&t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"p": t11fkParent("id")}},
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES p(id);"+
			"ALTER TABLE t DROP CONSTRAINT fk_c;"), "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept, got %+v", entry)
	}
	if t11fkMember(entry.shape, "fk_c") == nil {
		t.Fatalf("the member survives the TiDB constraint no-op, got %+v", entry.shape.Constraints)
	}
	if t11fkIndex(entry.shape, "fk_c") == nil {
		t.Fatalf("the supporting index survives too, got %+v", entry.shape.Indexes)
	}
}

// MySQL 8.x: the recovered generic spelling binds the lone FK member and
// removes it deterministically (probed rc=0 on 8.0.46) — the successor
// DROP FOREIGN KEY binds a proven-missing name and reports the blocker
// while the supporting index stays vouched.
func TestT11R1FMySQLDropConstraintAmbiguousOnProvenForeignKey(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t11r1FKShape()}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT fk_c;"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("a deterministic removal owes no gap, got %+v", drop.EvidenceGaps)
	}
	found := false
	for _, f := range drop.Findings {
		if f.RuleID == "ddl.alter.drop_foreign_key.exists.require" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the removed member is a proven-missing blocker for the successor, got %+v", drop.Findings)
	}
	// The index side never joined the drop — name-keyed index work still
	// reads the proven member.
	post := t11r1Apply(t, spec.DialectMySQL,
		map[string]*spec.TableSnapshot{"t": t11r1FKShape()}, nil,
		"ALTER TABLE t DROP CONSTRAINT fk_c;")
	entry := t11Entry(post, "t")
	if entry == nil || entry.shape == nil {
		t.Fatalf("entry kept, got %+v", entry)
	}
	if len(entry.shape.Indexes) != 1 {
		t.Fatalf("the supporting index stays vouched, got %+v", entry.shape.Indexes)
	}
}

// MySQL 5.7 rejects the syntax outright — the proven member provably
// survives and the successor binds it.
func TestT11R1FMySQL57DropConstraintSyntaxRefused(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{
		"t": t11r1FKShape()}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT fk_c;"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("the refused statement kept the member — drop lands cleanly, got %+v", drop.Findings)
	}
}

// A vouched member set proving the name absent makes MySQL's answer
// decidable too — both keywords refuse and the shape keeps its facts.
func TestT11R1FMySQLDropConstraintUnmatchedKeepsFacts(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t11r1FKShape()}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP CONSTRAINT nosuch;"+
			"ALTER TABLE t DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	drop := result.Statements[1]
	if len(drop.Findings) != 0 {
		t.Fatalf("an unmatched name refuses under either keyword — members kept, got %+v", drop.Findings)
	}
}
