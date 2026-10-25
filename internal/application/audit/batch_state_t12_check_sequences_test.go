package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12 CHECK ordered-state sequences: add-then-alter, add-then-drop,
// column-declared checks, create-table arms, the CONSTRAINT alias, the
// drop-column cascade, server-derived names, and duplicate-name refusal —
// all through the public Audit/enrichment path.

// ADD CHECK lands a proven member; ALTER CHECK flips its enforcement
// without losing identity or the sibling members.
func TestT12AddThenAlterSequence(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("sibling", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT chk_new CHECK (id < 50) ENFORCED;\n"+
			"ALTER TABLE t ALTER CHECK chk_new NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk_new")
	if member == nil {
		t.Fatalf("add-then-alter lost the member: %+v", post.Constraints)
	}
	if member.Enforced != "not_enforced" {
		t.Fatalf("Enforced=%q after NOT ENFORCED flip, want not_enforced", member.Enforced)
	}
	if post.FindCheckConstraint("sibling") == nil {
		t.Fatal("sibling member lost during the sequence")
	}
}

// ADD CHECK then DROP CHECK removes the proven member and keeps the
// collection vouched — a deterministic post-state under both keywords.
func TestT12AddThenDropSequence(t *testing.T) {
	t.Parallel()
	for _, keyword := range []string{"CHECK", "CONSTRAINT"} {
		keyword := keyword
		t.Run(keyword, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{
				version: "8.0.46",
				tables: map[string]*spec.TableSnapshot{
					"t": t12Table("t", []string{"id"}, t12CheckMember("sibling", "enforced", "id")),
				},
			}
			post := t10r4PostState(t, spec.DialectMySQL, provider,
				"ALTER TABLE t ADD CONSTRAINT chk_new CHECK (id < 50);\n"+
					"ALTER TABLE t DROP "+keyword+" chk_new;")
			if post == nil {
				t.Fatal("post-state snapshot missing entirely")
			}
			if post.FindCheckConstraint("chk_new") != nil {
				t.Fatalf("dropped member still bound: %+v", post.Constraints)
			}
			if post.ConstraintsUnknown {
				t.Fatal("deterministic drop unvouched the collection")
			}
			if post.FindCheckConstraint("sibling") == nil {
				t.Fatal("sibling member lost during add-then-drop")
			}
		})
	}
}

// A CHECK declared inside an ADD COLUMN lands as an in-column member on
// the enforcing arm — name, enforcement, and referenced columns intact.
func TestT12AddColumnInColumnCheckLands(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD COLUMN v INT CHECK (v > 0) NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	var member *spec.Constraint
	for i := range post.Constraints {
		if post.Constraints[i].Type == "check" {
			member = &post.Constraints[i]
		}
	}
	if member == nil {
		t.Fatalf("in-column check never landed: %+v", post.Constraints)
	}
	if !member.InColumn {
		t.Fatal("in-column member lost its origin mark")
	}
	if member.Enforced != "not_enforced" {
		t.Fatalf("Enforced=%q, want not_enforced", member.Enforced)
	}
	if len(member.Columns) != 1 || member.Columns[0] != "v" {
		t.Fatalf("referenced columns wrong: %+v", member.Columns)
	}
	if member.Expression == nil || !member.Expression.Known {
		t.Fatalf("expression facts missing: %+v", member.Expression)
	}
}

// CREATE TABLE check members land on the enforcing arm and are absent on
// the ignored arm — the declared member's fate follows execution, not
// parser acceptance.
func TestT12CreateTableCheckArmSplit(t *testing.T) {
	t.Parallel()
	t.Run("enforcing arm keeps members", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{}}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT, CONSTRAINT chk_id CHECK (id > 0) NOT ENFORCED);")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		member := post.FindCheckConstraint("chk_id")
		if member == nil {
			t.Fatalf("enforcing arm dropped the declared member: %+v", post.Constraints)
		}
		if member.Enforced != "not_enforced" {
			t.Fatalf("Enforced=%q, want not_enforced", member.Enforced)
		}
	})
	t.Run("ignored arm stores no member", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT, CONSTRAINT chk_id CHECK (id > 0));")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		if post.FindCheckConstraint("chk_id") != nil {
			t.Fatal("ignored arm must not store a check member")
		}
	})
	t.Run("unobserved version unvouches the collection", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{version: "", tables: map[string]*spec.TableSnapshot{}}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"CREATE TABLE t (id INT, CONSTRAINT chk_id CHECK (id > 0));")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		if !post.ConstraintsUnknown {
			t.Fatal("unobserved arm must unvouch the constraint collection")
		}
		if post.FindCheckConstraint("chk_id") != nil {
			t.Fatal("unproven landing must not be published as fact")
		}
	})
}

// ALTER CONSTRAINT is the same normalized action — alias handling is
// identical on every arm.
func TestT12AlterConstraintAlias(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ALTER CONSTRAINT chk_id NOT ENFORCED;")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	member := post.FindCheckConstraint("chk_id")
	if member == nil || member.Enforced != "not_enforced" {
		t.Fatalf("alias path failed: %+v", post.Constraints)
	}
}

// Dropping a column silently drops the CHECK members that reference it
// (probed 8.0.46/8.4.10/TiDB-ON); a member whose reference set cannot
// vouch keeps the drop unsafe and tombstones instead.
func TestT12DropColumnCheckCascade(t *testing.T) {
	t.Parallel()
	t.Run("referencing member cascades silently", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "8.0.46",
			tables: map[string]*spec.TableSnapshot{
				"t": t12Table("t", []string{"id", "obsolete"},
					t12CheckMember("chk_id", "enforced", "obsolete"),
					t12CheckMember("chk_other", "enforced", "id")),
			},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP COLUMN obsolete;")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		if post.FindCheckConstraint("chk_id") != nil {
			t.Fatal("referencing check must cascade with its column")
		}
		if post.FindCheckConstraint("chk_other") == nil {
			t.Fatal("unrelated check must survive the cascade")
		}
	})
	t.Run("unvouched reference set stays unsafe", func(t *testing.T) {
		t.Parallel()
		snapshot := t12Table("t", []string{"id", "obsolete"},
			spec.Constraint{Type: "check", Name: "chk_blind"})
		provider := &t10Provider{
			version: "8.0.46",
			tables:  map[string]*spec.TableSnapshot{"t": snapshot},
		}
		post := t10r4PostState(t, spec.DialectMySQL, provider,
			"ALTER TABLE t DROP COLUMN obsolete;")
		if post != nil {
			t.Fatalf("unvouched check must tombstone the table, got %+v", post)
		}
	})
}

// A bare ADD CHECK lands with an unvouched server-derived name — never
// bound by name-keyed lookups.
func TestT12BareAddCheckNameUnknown(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CHECK (id > 0);")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	var member *spec.Constraint
	for i := range post.Constraints {
		if post.Constraints[i].Type == "check" {
			member = &post.Constraints[i]
		}
	}
	if member == nil {
		t.Fatal("bare check never landed")
	}
	if !member.NameUnknown {
		t.Fatal("server-derived name must stay unvouched")
	}
	if post.FindCheckConstraint("t_chk_1") != nil {
		t.Fatal("guessed server name must never bind")
	}
}

// A duplicate authored name refuses 3822 — the collection keeps exactly
// the original member.
func TestT12AddCheckDuplicateNameRefuses(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t12Table("t", []string{"id"}, t12CheckMember("chk_id", "enforced", "id")),
		},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t ADD CONSTRAINT chk_id CHECK (id < 10);")
	if post == nil {
		t.Fatal("post-state snapshot missing entirely")
	}
	count := 0
	for i := range post.Constraints {
		if post.Constraints[i].Type == "check" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate add mutated the collection: %+v", post.Constraints)
	}
}

// TiDB ON: add-then-alter works like MySQL; OFF: nothing is stored and
// later alters/drops are no-ops.
func TestT12TiDBSequenceArms(t *testing.T) {
	t.Parallel()
	t.Run("flag ON stores and flips members", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			tidbCheck: t12Bool(true),
			tables:    map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
		}
		post := t10r4PostState(t, spec.DialectTiDB, provider,
			"ALTER TABLE t ADD CONSTRAINT chk_t CHECK (id > 0);\n"+
				"ALTER TABLE t ALTER CHECK chk_t NOT ENFORCED;")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		member := post.FindCheckConstraint("chk_t")
		if member == nil || member.Enforced != "not_enforced" {
			t.Fatalf("TiDB ON sequence failed: %+v", post.Constraints)
		}
	})
	t.Run("flag OFF stores nothing and later ops no-op", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			tidbCheck: t12Bool(false),
			tables:    map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
		}
		post := t10r4PostState(t, spec.DialectTiDB, provider,
			"ALTER TABLE t ADD CONSTRAINT chk_t CHECK (id > 0);\n"+
				"ALTER TABLE t ALTER CHECK chk_t NOT ENFORCED;\n"+
				"ALTER TABLE t DROP CHECK chk_t;")
		if post == nil {
			t.Fatal("post-state snapshot missing entirely")
		}
		if post.FindCheckConstraint("chk_t") != nil {
			t.Fatal("flag OFF must store no member")
		}
		if post.ConstraintsUnknown {
			t.Fatal("no-op arm must not unvouch the collection")
		}
	})
}

// RENAME COLUMN on a check-referenced column refuses 3959 on every probed
// enforcing arm (MySQL 8.0.46/8.4.10, TiDB flag-ON) — the statement never
// executes, so the post-state is the exact pre-state, not a tombstone.
func TestT12RenameColumnCheckReferenceRefused(t *testing.T) {
	t.Parallel()
	snapshot := t12Table("t", []string{"a", "id"},
		spec.Constraint{
			Type: "check", Name: "chk_id", Enforced: "enforced",
			Columns:    []string{"a"},
			Expression: &spec.ExpressionFacts{Known: true, Fingerprint: "fp", References: []spec.ColumnReference{{Name: "a"}}},
		})
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": snapshot},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t RENAME COLUMN a TO id2;")
	if post == nil {
		t.Fatal("refused rename must keep a precise pre-state")
	}
	member := post.FindCheckConstraint("chk_id")
	if member == nil {
		t.Fatal("refused rename lost the check member")
	}
	if len(member.Columns) != 1 || member.Columns[0] != "a" {
		t.Fatalf("refused rename mutated member columns: %+v", member.Columns)
	}
	if post.FindColumn("id2") != nil {
		t.Fatal("refused rename claimed the new column")
	}
	if post.FindColumn("a") == nil {
		t.Fatal("refused rename lost the old column")
	}
}

// CHANGE COLUMN renaming a check-referenced column refuses the same 3959 on
// MySQL — provably unchanged.
func TestT12ChangeColumnCheckReferenceRefusedMySQL(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id", "v"},
			spec.Constraint{Type: "check", Name: "chk_id", Enforced: "enforced", Columns: []string{"id"},
				Expression: &spec.ExpressionFacts{Known: true, References: []spec.ColumnReference{{Name: "id"}}}})},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t CHANGE COLUMN id id2 BIGINT;")
	if post == nil {
		t.Fatal("refused change must keep a precise pre-state")
	}
	if post.FindCheckConstraint("chk_id") == nil || post.FindColumn("id") == nil || post.FindColumn("id2") != nil {
		t.Fatalf("refused change mutated the shape: %+v", post)
	}
}

// TiDB flag-ON diverges: CHANGE COLUMN lands the rename but silently drops
// the referencing check member — a deterministic post-state.
func TestT12ChangeColumnCheckReferenceCascadeTiDB(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id", "v"},
			spec.Constraint{Type: "check", Name: "chk_id", Enforced: "enforced", Columns: []string{"id"},
				Expression: &spec.ExpressionFacts{Known: true, References: []spec.ColumnReference{{Name: "id"}}}},
			spec.Constraint{Type: "check", Name: "chk_v", Enforced: "enforced", Columns: []string{"v"},
				Expression: &spec.ExpressionFacts{Known: true, References: []spec.ColumnReference{{Name: "v"}}}})},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t CHANGE COLUMN id id2 BIGINT;")
	if post == nil {
		t.Fatal("TiDB change-column cascade must publish a post-state")
	}
	if post.FindColumn("id2") == nil {
		t.Fatal("TiDB change-column rename did not land")
	}
	if post.FindCheckConstraint("chk_id") != nil {
		t.Fatal("referencing check survived the silent drop")
	}
	if post.FindCheckConstraint("chk_v") == nil {
		t.Fatal("unrelated check member lost")
	}
	if post.ConstraintsUnknown {
		t.Fatal("deterministic cascade must not unvouch the collection")
	}
}

// RENAME COLUMN on TiDB flag-ON refuses 3959 like MySQL — provably
// unchanged.
func TestT12RenameColumnCheckReferenceRefusedTiDB(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		tidbCheck: t12Bool(true),
		tables: map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id", "v"},
			spec.Constraint{Type: "check", Name: "chk_id", Enforced: "enforced", Columns: []string{"id"},
				Expression: &spec.ExpressionFacts{Known: true, References: []spec.ColumnReference{{Name: "id"}}}})},
	}
	post := t10r4PostState(t, spec.DialectTiDB, provider,
		"ALTER TABLE t RENAME COLUMN id TO id2;")
	if post == nil {
		t.Fatal("TiDB refused rename must keep a precise pre-state")
	}
	if post.FindCheckConstraint("chk_id") == nil || post.FindColumn("id") == nil || post.FindColumn("id2") != nil {
		t.Fatalf("TiDB refused rename mutated the shape: %+v", post)
	}
}

// An unvouched check member cannot prove it ignores the renamed column —
// the transition stays withheld instead of guessing.
func TestT12RenameColumnCheckUnvouchedUnsafe(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"},
			spec.Constraint{Type: "check", Name: "chk_x", Enforced: "enforced"})},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t RENAME COLUMN id TO id2;")
	if post != nil {
		t.Fatalf("unvouched check reference published a post-state: %+v", post)
	}
}

// A CHANGE/MODIFY COLUMN carrying an in-column CHECK cannot publish a
// silent member drop or a fabricated landing — the transition tombstones.
func TestT12ChangeColumnCarryingCheckTombstones(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": t12Table("t", []string{"id"})},
	}
	post := t10r4PostState(t, spec.DialectMySQL, provider,
		"ALTER TABLE t CHANGE COLUMN id v INT CHECK (v > 0);")
	if post != nil {
		t.Fatalf("check-carrying change published a precise post-state: %+v", post)
	}
}
