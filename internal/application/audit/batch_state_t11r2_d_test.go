// Package audit pins the T11-R2 complete-key versus partial-reference
// boundary: MySQL 8.4 under observed restrict_fk_on_non_standard_key=ON
// refuses a foreign key whose referenced columns cover only the leftmost
// prefix of a composite unique or primary key (6125, probed on 8.4.10) —
// the leftmost-prefix rule applies to supporting indexes on the child and
// to non-strict anchors, never to referenced-key completeness under the
// restrictive setting. Under OFF, on pre-8.4 MySQL, and on TiDB the
// prefix reference lands (probed). An unproven flag or unproven version
// leaves the landing undecidable rather than defaulting either arm.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r2CompositeParent returns a proven parent shape carrying a composite
// unique member (a,b) — the partial-reference fixture.
func t11r2CompositeParent() *spec.TableSnapshot {
	shape := t10r2Table("puq", []spec.Column{t10r2Col("a", "int", true, 0), t10r2Col("b", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "uq_ab", Kind: spec.IndexKindUnique, Columns: []string{"a", "b"}}}
	return shape
}

// t11r2CompositePKParent returns a proven parent whose primary object
// spans (a,b) — the partial-PK fixture.
func t11r2CompositePKParent() *spec.TableSnapshot {
	shape := t10r2Table("ppk", []spec.Column{t10r2Col("a", "int", true, 0), t10r2Col("b", "int", true, 0)})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a", "b"}}
	return shape
}

// t11r2CompositeSecondaryParent carries only a composite secondary
// member — non-unique on top of partial.
func t11r2CompositeSecondaryParent() *spec.TableSnapshot {
	shape := t10r2Table("psc", []spec.Column{t10r2Col("a", "int", true, 0), t10r2Col("b", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "ix_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}}}
	return shape
}

// t11r2SingleUniqueParent carries an exact single-column unique member —
// the complete standard key.
func t11r2SingleUniqueParent() *spec.TableSnapshot {
	shape := t10r2Table("pu", []spec.Column{t10r2Col("a", "int", true, 0)})
	shape.Indexes = []spec.Index{{Name: "uq_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}}}
	return shape
}

// t11r2PartialAudit runs add-then-drop through the public audit path and
// returns the successor drop's statement report. The drop targets the
// same child table the add authored.
func t11r2PartialAudit(t *testing.T, dialect spec.Dialect, provider *t10Provider, child, addSQL string) report.StatementResult {
	t.Helper()
	result := t10Audit(t, dialect, addSQL+" ALTER TABLE "+child+" DROP FOREIGN KEY fk_c;",
		provider, t10Policy(t, t10Rule{id: "ddl.alter.drop_foreign_key.exists.require"}))
	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	return result.Statements[1]
}

// Under observed restrict ON, every partial composite reference is a
// proven refusal — the successor drop answers proven-missing.
func TestT11R2RestrictOnPartialKeysRefuse(t *testing.T) {
	t.Parallel()
	provider := func() *t10Provider {
		return &t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
			"t":   t11r1Child("c"),
			"t2":  t11r1Child("c"),
			"t3":  t11r1Child("c"),
			"puq": t11r2CompositeParent(),
			"ppk": t11r2CompositePKParent(),
			"psc": t11r2CompositeSecondaryParent(),
		}}
	}
	t.Run("partial unique", func(t *testing.T) {
		t.Parallel()
		drop := t11r2PartialAudit(t, spec.DialectMySQL, provider(), "t",
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES puq(a);")
		if len(drop.Findings) != 1 {
			t.Fatalf("6125 refuses a partial unique key — drop must answer missing, got findings=%#v gaps=%#v",
				drop.Findings, drop.EvidenceGaps)
		}
		if drop.Coverage.Status != report.CoverageComplete {
			t.Fatalf("the refusal is fully proven — no gap owed, got %#v", drop.EvidenceGaps)
		}
	})
	t.Run("partial primary key", func(t *testing.T) {
		t.Parallel()
		drop := t11r2PartialAudit(t, spec.DialectMySQL, provider(), "t2",
			"ALTER TABLE t2 ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES ppk(a);")
		if len(drop.Findings) != 1 {
			t.Fatalf("6125 refuses a partial primary key too — drop must answer missing, got %#v", drop.Findings)
		}
	})
	t.Run("partial secondary", func(t *testing.T) {
		t.Parallel()
		drop := t11r2PartialAudit(t, spec.DialectMySQL, provider(), "t3",
			"ALTER TABLE t3 ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES psc(a);")
		if len(drop.Findings) != 1 {
			t.Fatalf("a non-unique partial key stays refused — drop must answer missing, got %#v", drop.Findings)
		}
	})
}

// Under observed restrict ON the complete keys still land — a full unique
// match and a full composite match owe no finding and no config gap.
func TestT11R2RestrictOnFullKeysLand(t *testing.T) {
	t.Parallel()
	t.Run("full single unique", func(t *testing.T) {
		t.Parallel()
		drop := t11r2PartialAudit(t, spec.DialectMySQL,
			&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
				"t":  t11r1Child("c"),
				"pu": t11r2SingleUniqueParent(),
			}}, "t",
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES pu(a);")
		if len(drop.Findings) != 0 || len(drop.EvidenceGaps) != 0 {
			t.Fatalf("a complete unique key owes no finding and no restrict gap, got findings=%#v gaps=%#v",
				drop.Findings, drop.EvidenceGaps)
		}
	})
	t.Run("full composite unique", func(t *testing.T) {
		t.Parallel()
		child := t11r1Child("a", "b")
		drop := t11r2PartialAudit(t, spec.DialectMySQL,
			&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
				"t":   child,
				"puq": t11r2CompositeParent(),
			}}, "t",
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (a,b) REFERENCES puq(a,b);")
		if len(drop.Findings) != 0 || len(drop.EvidenceGaps) != 0 {
			t.Fatalf("the complete composite key lands under restrict too, got findings=%#v gaps=%#v",
				drop.Findings, drop.EvidenceGaps)
		}
	})
	t.Run("wrong order unique", func(t *testing.T) {
		t.Parallel()
		drop := t11r2PartialAudit(t, spec.DialectMySQL,
			&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(true), tables: map[string]*spec.TableSnapshot{
				"t":   t11r1Child("c"),
				"puq": t11r2CompositeParent(),
			}}, "t",
			"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES puq(b);")
		if len(drop.Findings) != 1 {
			t.Fatalf("b is not the composite key's sequence — refused, got %#v", drop.Findings)
		}
	})
}

// Under observed restrict OFF every partial reference lands — the flag's
// permissive arm must not borrow the strict one.
func TestT11R2RestrictOffPartialKeysLand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		parent string
		child  string
	}{
		{"partial unique", "puq", "t"},
		{"partial primary key", "ppk", "t2"},
		{"partial secondary", "psc", "t3"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			drop := t11r2PartialAudit(t, spec.DialectMySQL,
				&t10Provider{version: "8.4.10", restrictNonStandard: boolPtr(false), tables: map[string]*spec.TableSnapshot{
					tc.child: t11r1Child("c"),
					"puq":    t11r2CompositeParent(),
					"ppk":    t11r2CompositePKParent(),
					"psc":    t11r2CompositeSecondaryParent(),
				}}, tc.child,
				"ALTER TABLE "+tc.child+" ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES "+tc.parent+"(a);")
			if len(drop.Findings) != 0 {
				t.Fatalf("restrict OFF accepts the partial reference — the drop lands cleanly, got %#v", drop.Findings)
			}
		})
	}
}

// An unread flag on 8.4 leaves the partial-unique landing undecidable —
// no proven-missing finding and no proven landing; the successor owes
// the gap instead.
func TestT11R2RestrictUnreadPartialUndecidable(t *testing.T) {
	t.Parallel()
	drop := t11r2PartialAudit(t, spec.DialectMySQL,
		&t10Provider{version: "8.4.10", tables: map[string]*spec.TableSnapshot{
			"t":   t11r1Child("c"),
			"puq": t11r2CompositeParent(),
		}}, "t",
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES puq(a);")
	if len(drop.Findings) != 0 {
		t.Fatalf("an unread flag cannot prove refusal — no missing finding, got %#v", drop.Findings)
	}
	if len(drop.EvidenceGaps) == 0 {
		t.Fatalf("the undecidable landing owes the evidence gap, got none")
	}
}

// Pre-8.4 MySQL carries no restrict variable at all — partial composite
// references land on the plain prefix rule, the same as probed 8.4-OFF.
func TestT11R2Pre84PartialKeysLand(t *testing.T) {
	t.Parallel()
	drop := t11r2PartialAudit(t, spec.DialectMySQL,
		&t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
			"t":   t11r1Child("c"),
			"puq": t11r2CompositeParent(),
		}}, "t",
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES puq(a);")
	if len(drop.Findings) != 0 || len(drop.EvidenceGaps) != 0 {
		t.Fatalf("8.0 has no restrict concept — the partial unique lands, got findings=%#v gaps=%#v",
			drop.Findings, drop.EvidenceGaps)
	}
}

// TiDB accepts every partial composite reference (probed v8.5.0: partial
// unique, partial PK, and partial secondary all rc=0).
func TestT11R2TiDBPartialKeysLand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, parent, child string }{
		{"partial unique", "puq", "t"},
		{"partial primary key", "ppk", "t2"},
		{"partial secondary", "psc", "t3"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			drop := t11r2PartialAudit(t, spec.DialectTiDB,
				&t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{
					tc.child: t11r1Child("c"),
					"puq":    t11r2CompositeParent(),
					"ppk":    t11r2CompositePKParent(),
					"psc":    t11r2CompositeSecondaryParent(),
				}}, tc.child,
				"ALTER TABLE "+tc.child+" ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES "+tc.parent+"(a);")
			if len(drop.Findings) != 0 {
				t.Fatalf("TiDB accepts the partial reference — the drop lands cleanly, got %#v", drop.Findings)
			}
		})
	}
}

// An unproven MySQL version cannot rule out an 8.4+ restrictive instance —
// the partial-unique landing stays undecidable.
func TestT11R2UnknownVersionPartialUndecidable(t *testing.T) {
	t.Parallel()
	drop := t11r2PartialAudit(t, spec.DialectMySQL,
		&t10Provider{version: "", tables: map[string]*spec.TableSnapshot{
			"t":   t11r1Child("c"),
			"puq": t11r2CompositeParent(),
		}}, "t",
		"ALTER TABLE t ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES puq(a);")
	if len(drop.Findings) != 0 {
		t.Fatalf("an unproven version cannot prove refusal — no missing finding, got %#v", drop.Findings)
	}
	if len(drop.EvidenceGaps) == 0 {
		t.Fatalf("an undecidable landing owes the evidence gap, got none")
	}
}
