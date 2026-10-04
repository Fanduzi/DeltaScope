// Package audit verifies the T05-A4-R2 closure of MODIFY affected-set scope.
// input: real parser statements and shared AuditSQL requests for multi-action
// ALTER targets and a precise MODIFY whose loaded foreign key cannot be recomputed
// output: unknown tombstones for every touched table identity, no fabricated
// absence finding, and no precise column successor beside an unrecomputable child
// pos: T05-A4-R2 regression for the two remaining R1 publication boundaries
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t05A4R2ChildRef(schema, table string, columns []string) *spec.TableSnapshot {
	child := t05A4R1Child(columns, 0)
	child.Constraints[0].ReferencedSchema = schema
	child.Constraints[0].ReferencedTable = table
	return child
}

func t05A4R2WantGap(t *testing.T, result report.Result, index int, ruleID string, facts []string) {
	t.Helper()
	gaps := t05GapsByRule(result, index, ruleID)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("%s gaps = %+v, want one unknown_table_state gap", ruleID, result.Statements[index].EvidenceGaps)
	}
	if len(gaps[0].RequiredFacts) != len(facts) {
		t.Fatalf("%s facts = %+v, want %v", ruleID, gaps[0].RequiredFacts, facts)
	}
	for i := range facts {
		if gaps[0].RequiredFacts[i] != facts[i] {
			t.Fatalf("%s facts = %+v, want %v", ruleID, gaps[0].RequiredFacts, facts)
		}
	}
}

func t05A4R2WantNoAbsence(t *testing.T, result report.Result, index int) {
	t.Helper()
	if len(result.Statements[index].Findings) != 0 {
		t.Fatalf("statement %d findings = %+v, want none", index, result.Statements[index].Findings)
	}
	t05A4R2WantGap(t, result, index, t05RuleAlterRequire, []string{"target_table.existence"})
	t05A4R2WantGap(t, result, index, t05RuleAddColumnForbid, []string{"target_table.columns", "target_table.existence"})
}

func TestAuditSQLT05A4R2RenameInvalidatesBothEnds(t *testing.T) {
	t.Parallel()
	const sql = "ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), RENAME TO dst.u;\n" +
		"ALTER TABLE dst.u ADD COLUMN x INT;\n" +
		"ALTER TABLE golden.t ADD COLUMN y INT;"
	provider := t05A4R1Provider(t05A4R1Parent(), nil)
	provider.snapshots["dst.u"] = absentTable("dst", "u")
	result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
	t05A4R2WantNoAbsence(t, result, 1)
	t05A4R2WantNoAbsence(t, result, 2)
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
		t.Fatalf("provider ledger = %#v, want only the pre-batch source read", provider.calls)
	}
	observed := t05A4R1Provider(t05A4R1Parent(), nil)
	observed.snapshots["dst.u"] = absentTable("dst", "u")
	enriched := enrichA3(t, sql, spec.DialectMySQL, observed)
	t05A4R1WantUnknown(t, enriched[1])
	t05A4R1WantUnknown(t, enriched[2])
}

func TestAuditSQLT05A4R2CachedAbsentDestinationBecomesUnknown(t *testing.T) {
	t.Parallel()
	const sql = "UPDATE dst.u SET id = 1;\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), RENAME TO dst.u;\n" +
		"ALTER TABLE dst.u ADD COLUMN x INT;"
	provider := t05A4R1Provider(t05A4R1Parent(), nil)
	provider.snapshots["dst.u"] = absentTable("dst", "u")
	result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
	t05A4R2WantNoAbsence(t, result, 2)
	if len(provider.calls) != 2 || provider.calls[0] != "dst.u" || provider.calls[1] != "golden.t" {
		t.Fatalf("provider ledger = %#v, want the cached absence read once and no later reread", provider.calls)
	}
}

func TestAuditSQLT05A4R2RenameDropsLoadedDependents(t *testing.T) {
	t.Parallel()
	t.Run("destination reference", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\n" +
			"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), RENAME TO dst.u;\n" +
			"ALTER TABLE aux.child ADD COLUMN x INT;"
		provider := t05A4R1Provider(t05A4R1Parent(), t05A4R2ChildRef("dst", "u", []string{"id"}))
		result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
		t05A4R2WantNoAbsence(t, result, 2)
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R2ChildRef("dst", "u", []string{"id"})))
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("source reference to a column the modify does not name", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\n" +
			"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), RENAME TO dst.u;\n" +
			"ALTER TABLE aux.child ADD COLUMN x INT;"
		child := t05A4R1Child([]string{"id"}, 0)
		provider := t05A4R1Provider(t05A4R1Parent(), child)
		result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
		t05A4R2WantNoAbsence(t, result, 2)
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"id"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})
}

func TestAuditSQLT05A4R2DropColumnDropsReferencingChild(t *testing.T) {
	t.Parallel()
	parent := t05A4R1Parent()
	parent.Columns = append(parent.Columns, spec.Column{Name: "d", Type: "varchar", Length: 10})
	const sql = "CREATE INDEX warm ON aux.child(f);\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), DROP COLUMN d;\n" +
		"ALTER TABLE aux.child ADD COLUMN x INT;"
	child := t05A4R1Child([]string{"d"}, 0)
	provider := t05A4R1Provider(parent, child)
	result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
	t05A4R2WantNoAbsence(t, result, 2)
	enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(parent, t05A4R1Child([]string{"d"}, 0)))
	t05A4R1WantUnknown(t, enriched[2])
}

func TestAuditSQLT05A4R2RelatedForeignKeyBlocksPreciseSuccessor(t *testing.T) {
	t.Parallel()
	const sql = "CREATE INDEX warm ON aux.child(f);\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\n" +
		"ALTER TABLE golden.t ADD COLUMN x INT;\n" +
		"ALTER TABLE aux.child ADD COLUMN y INT;"
	provider := t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0))
	result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
	if findings := t05FindingsByRule(result, 1, t05RuleModifyCompat); len(findings) != 0 {
		t.Fatalf("widen finding = %+v, want the existing no-finding widen", result.Statements[1].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("modify gaps = %+v, want none while the pre-state is known", result.Statements[1].EvidenceGaps)
	}
	t05A4R2WantNoAbsence(t, result, 2)
	t05A4R2WantNoAbsence(t, result, 3)
	enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
	t05A4R1WantUnknown(t, enriched[2])
	t05A4R1WantUnknown(t, enriched[3])
}

func TestAuditSQLT05A4R2UnrelatedReferenceStillPublishes(t *testing.T) {
	t.Parallel()
	const sql = "CREATE INDEX warm ON aux.child(f);\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\n" +
		"CREATE INDEX after ON aux.child(f);\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c VARCHAR(15);"
	enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"id"}, 0)))
	snapshot := enriched[2].Metadata.TargetTable
	if snapshot == nil || !snapshot.Exists || snapshot.FindColumn("f") == nil {
		t.Fatalf("unrelated child = %+v, want the loaded table", snapshot)
	}
	if got := t05A4Length(t, enriched[3], "c"); got != 20 {
		t.Fatalf("precise successor length = %d, want 20", got)
	}
}
