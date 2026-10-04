// Package audit verifies the T05-A4-R1 MODIFY rework boundaries.
// input: real parser statements and shared AuditSQL requests whose later
// statements read the ordered pre-state left by an ordinary MODIFY
// output: findings, evidence gaps, and pre-state assertions for loaded
// dependents, inline PRIMARY KEY declarations, and TiDB primary-key signedness
// pos: T05-A4-R1 regression for the three rejected MODIFY boundaries
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05A4R1DropPrimaryKey = "ddl.alter.drop_primary_key.exists.require"

func t05A4R1Policy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:    "",
		t05RuleAlterRequire:    "",
		t05RuleAddColumnForbid: "",
		t05RuleModifyExists:    "",
		t05RuleModifyCompat:    "      required: true\n",
		t05A4R1DropPrimaryKey:  "",
	})
}

func t05A4R1Parent() *spec.TableSnapshot {
	shape := presentTable("golden", "t", "id", "c")
	shape.Columns[0] = spec.Column{Name: "id", Type: "int", NotNull: true}
	shape.Columns[1] = spec.Column{Name: "c", Type: "varchar", Length: 10}
	shape.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	shape.Indexes = []spec.Index{{Name: "uc", Kind: spec.IndexKindUnique, Columns: []string{"c"}}}
	return shape
}

func t05A4R1Child(referenced []string, unmodeledReferenced int) *spec.TableSnapshot {
	child := presentTable("aux", "child", "f")
	child.Columns[0] = spec.Column{Name: "f", Type: "varchar", Length: 10}
	child.Constraints = []spec.Constraint{{
		Type:                     "foreign_key",
		Name:                     "fk_f",
		Columns:                  []string{"f"},
		ReferencedSchema:         "golden",
		ReferencedTable:          "t",
		ReferencedColumns:        append([]string(nil), referenced...),
		UnmodeledReferencedParts: unmodeledReferenced,
	}}
	return child
}

func t05A4R1Provider(parent, child *spec.TableSnapshot) *t05PresentProvider {
	snapshots := map[string]*spec.TableSnapshot{}
	if parent != nil {
		snapshots["golden.t"] = parent
	}
	if child != nil {
		snapshots["aux.child"] = child
	}
	return &t05PresentProvider{snapshots: snapshots}
}

func t05A4R1WantUnknown(t *testing.T, statement spec.Statement) {
	t.Helper()
	if statement.Metadata == nil || statement.Metadata.TargetTable != nil {
		t.Fatalf("pre-state = %+v, want an unknown table", statement.Metadata)
	}
}

// TestAuditSQLT05A4R1CrossFamilyDropsLoadedChild is the rejected loaded-child
// counterexample. A cross-family MODIFY cannot publish a precise successor,
// and the already-loaded foreign key must not stay available to the next
// statement.
func TestAuditSQLT05A4R1CrossFamilyDropsLoadedChild(t *testing.T) {
	t.Parallel()
	const sql = "CREATE INDEX warm ON aux.child(f);\n" +
		"ALTER TABLE golden.t MODIFY COLUMN c BIGINT;\n" +
		"ALTER TABLE aux.child ADD COLUMN x INT;"
	provider := t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0))
	result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
	findings := t05FindingsByRule(result, 1, t05RuleModifyCompat)
	if len(findings) != 1 || findings[0].Metadata["source_family"] != "string" || findings[0].Metadata["target_family"] != "integer" {
		t.Fatalf("cross-family finding = %+v, want one string-to-integer compatibility blocker", result.Statements[1].Findings)
	}
	if len(result.Statements[2].Findings) != 0 {
		t.Fatalf("invalidated child must not invent a finding, got %+v", result.Statements[2].Findings)
	}
	gaps := t05GapsByRule(result, 2, t05RuleAddColumnForbid)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("add column gaps = %+v, want one unknown_table_state gap", result.Statements[2].EvidenceGaps)
	}
	wantFacts := []string{"target_table.columns", "target_table.existence"}
	if len(gaps[0].RequiredFacts) != len(wantFacts) || gaps[0].RequiredFacts[0] != wantFacts[0] || gaps[0].RequiredFacts[1] != wantFacts[1] {
		t.Fatalf("required facts = %+v, want %v", gaps[0].RequiredFacts, wantFacts)
	}
	enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
	t05A4R1WantUnknown(t, enriched[2])
	if len(provider.calls) != 2 || provider.calls[0] != "aux.child" || provider.calls[1] != "golden.t" {
		t.Fatalf("provider ledger = %#v, want [aux.child golden.t]", provider.calls)
	}
}

func TestBatchStateA4R1AffectedSet(t *testing.T) {
	t.Parallel()

	t.Run("related foreign key invalidates the target with the child", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t ADD COLUMN x INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
		t05A4R1WantUnknown(t, enriched[3])
	})

	t.Run("conditional modify drops a loaded child", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN IF EXISTS c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectTiDB, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("position modify drops a loaded child", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20) FIRST;\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("multi-action modify drops a loaded child", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20), ADD COLUMN d INT;\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("withheld columns keep the target and drop the child", func(t *testing.T) {
		t.Parallel()
		parent := t05A4R1Parent()
		parent.Columns = nil
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t ADD COLUMN z INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(parent, t05A4R1Child([]string{"c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
		snapshot := enriched[3].Metadata.TargetTable
		if snapshot == nil || !snapshot.Exists || snapshot.Columns != nil || snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "PRIMARY" {
			t.Fatalf("withheld target = %+v, want existence and the known primary key without a column set", snapshot)
		}
	})

	t.Run("confirmed-absent column drops a loaded reference", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN missing VARCHAR(20);\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"missing"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("explicit null on a known primary key drops a loaded reference", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN id BIGINT NULL;\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"id"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("unknown primary-key membership drops a loaded reference", func(t *testing.T) {
		t.Parallel()
		parent := t05A4R1Parent()
		parent.PrimaryKey = nil
		parent.PrimaryKeyUnknown = true
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN id BIGINT;\nCREATE INDEX after ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(parent, t05A4R1Child([]string{"id"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("self reference on the referenced side invalidates the target", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TABLE t (parent_c VARCHAR(10), c VARCHAR(10), CONSTRAINT fk_self FOREIGN KEY (parent_c) REFERENCES t(c));\n" +
			"ALTER TABLE t MODIFY COLUMN c VARCHAR(20);\n" +
			"ALTER TABLE t ADD COLUMN x INT;"
		result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A4R1Policy(t))
		if len(result.Statements[2].Findings) != 0 {
			t.Fatalf("self-reference must not invent a finding, got %+v", result.Statements[2].Findings)
		}
		gaps := t05GapsByRule(result, 2, t05RuleAddColumnForbid)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("add after self-reference = %+v, want unknown_table_state", result.Statements[2].EvidenceGaps)
		}
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		t05A4R1WantUnknown(t, enriched[2])
	})

	t.Run("unmodeled referenced parts do not prove unrelated", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t ADD COLUMN x INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"id"}, 1)))
		t05A4R1WantUnknown(t, enriched[2])
		t05A4R1WantUnknown(t, enriched[3])
	})

	t.Run("unmodeled local parts do not prove unrelated", func(t *testing.T) {
		t.Parallel()
		parent := t05A4R1Parent()
		parent.Constraints = []spec.Constraint{{
			Type:              "foreign_key",
			Name:              "fk_id",
			Columns:           []string{"id"},
			UnmodeledParts:    1,
			ReferencedTable:   "p",
			ReferencedColumns: []string{"id"},
		}}
		const sql = "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);\nALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(parent, nil))
		t05A4R1WantUnknown(t, enriched[1])
	})

	t.Run("complete unrelated reference stays", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(15);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"id"}, 0)))
		snapshot := enriched[2].Metadata.TargetTable
		if snapshot == nil || !snapshot.Exists || snapshot.FindColumn("f") == nil {
			t.Fatalf("unrelated child = %+v, want the loaded table", snapshot)
		}
		if got := t05A4Length(t, enriched[3], "c"); got != 20 {
			t.Fatalf("precise successor length = %d, want 20", got)
		}
	})

	t.Run("empty referenced list does not prove unrelated", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t ADD COLUMN x INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child(nil, 0)))
		t05A4R1WantUnknown(t, enriched[2])
		t05A4R1WantUnknown(t, enriched[3])
	})

	t.Run("same table name in another schema stays", func(t *testing.T) {
		t.Parallel()
		other := t05A4R1Parent()
		other.Schema = "other"
		other.Table = &spec.Table{Schema: "other", Name: "t"}
		provider := t05A4R1Provider(t05A4R1Parent(), nil)
		provider.snapshots["other.t"] = other
		const sql = "ALTER TABLE other.t ADD COLUMN extra INT;\nALTER TABLE golden.t MODIFY COLUMN c VARCHAR(20);\nALTER TABLE other.t ADD COLUMN extra2 INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, provider)
		if got := t05A4Length(t, enriched[2], "c"); got != 10 {
			t.Fatalf("other.t c length = %d, want the untouched 10", got)
		}
	})

	t.Run("unloaded child is not discovered", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE golden.t MODIFY COLUMN c BIGINT;\nCREATE INDEX warm ON aux.child(f);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, t05A4R1Provider(t05A4R1Parent(), t05A4R1Child([]string{"c"}, 0)))
		snapshot := enriched[1].Metadata.TargetTable
		if snapshot == nil || !snapshot.Exists || snapshot.FindColumn("f") == nil {
			t.Fatalf("unloaded child = %+v, want the provider snapshot", snapshot)
		}
	})
}

func TestAuditSQLT05A4R1InlinePrimaryKeyStaysOutOfTemplate(t *testing.T) {
	t.Parallel()
	forms := []string{
		"ALTER TABLE t MODIFY COLUMN c INT PRIMARY KEY NOT NULL",
		"ALTER TABLE t MODIFY COLUMN c INT NOT NULL PRIMARY KEY",
		"ALTER TABLE t MODIFY COLUMN c INT PRIMARY KEY NULL",
		"ALTER TABLE t MODIFY COLUMN c INT PRIMARY KEY",
	}
	for _, form := range forms {
		form := form
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			sql := "CREATE TABLE t (c INT NOT NULL);\n" + form + ";\nALTER TABLE t DROP PRIMARY KEY;"
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				dialect := dialect
				t.Run(string(dialect), func(t *testing.T) {
					t.Parallel()
					result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A4R1Policy(t))
					if len(result.Statements[2].Findings) != 0 {
						t.Fatalf("drop primary key findings = %+v, want none", result.Statements[2].Findings)
					}
					gaps := t05GapsByRule(result, 2, t05A4R1DropPrimaryKey)
					if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
						t.Fatalf("drop primary key gaps = %+v, want unknown_table_state", result.Statements[2].EvidenceGaps)
					}
					wantFacts := []string{"target_table.primary_key", "target_table.existence"}
					if len(gaps[0].RequiredFacts) != 2 || gaps[0].RequiredFacts[0] != wantFacts[0] || gaps[0].RequiredFacts[1] != wantFacts[1] {
						t.Fatalf("required facts = %+v, want %v", gaps[0].RequiredFacts, wantFacts)
					}
					enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
					t05A4R1WantUnknown(t, enriched[2])
				})
			}
		})
	}
}

func TestAuditSQLT05A4R1OrdinaryNotNullStillPublishes(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (c INT);\nALTER TABLE t MODIFY COLUMN c INT NOT NULL;\nALTER TABLE t MODIFY COLUMN c BIGINT;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			column := enriched[2].Metadata.TargetTable.FindColumn("c")
			if column == nil || !column.NotNull || modifyBaseType(*column) != "int" {
				t.Fatalf("ordinary NOT NULL pre-state = %+v, want int not null", column)
			}
		})
	}
}

func TestAuditSQLT05A4R1TiDBPrimaryKeySignednessStaysConservative(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\n" +
		"ALTER TABLE t MODIFY COLUMN id INT UNSIGNED;\n" +
		"ALTER TABLE t MODIFY COLUMN id BIGINT UNSIGNED;"
	result := t05A4Audit(t, sql, spec.DialectTiDB, &t05AbsentProvider{}, t05A4FourRulePolicy(t))
	findings := t05FindingsByRule(result, 1, t05RuleModifyCompat)
	if len(findings) != 1 || findings[0].Metadata["source_unsigned"] != false || findings[0].Metadata["target_unsigned"] != true {
		t.Fatalf("signedness finding = %+v, want one false-to-true compatibility blocker", result.Statements[1].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("known primary-key pre-state must keep the finding without a gap, got %+v", result.Statements[1].EvidenceGaps)
	}
	if len(result.Statements[2].Findings) != 0 {
		t.Fatalf("unknown successor must not invent a finding, got %+v", result.Statements[2].Findings)
	}
	gaps := t05GapsByRule(result, 2, t05RuleModifyExists)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("third statement gaps = %+v, want unknown_table_state", result.Statements[2].EvidenceGaps)
	}
	enriched := enrichA3(t, sql, spec.DialectTiDB, &t05AbsentProvider{})
	t05A4R1WantUnknown(t, enriched[2])
}

func TestAuditSQLT05A4R1MySQLPrimaryKeySignednessStillPublishes(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\n" +
		"ALTER TABLE t MODIFY COLUMN id INT UNSIGNED;\n" +
		"ALTER TABLE t MODIFY COLUMN id BIGINT UNSIGNED;"
	enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
	column := enriched[2].Metadata.TargetTable.FindColumn("id")
	if column == nil || !column.Unsigned || !column.NotNull {
		t.Fatalf("MySQL primary-key signedness pre-state = %+v, want unsigned not null", column)
	}
}

func TestAuditSQLT05A4R1TiDBPrimaryKeyWidenStillPublishes(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t MODIFY COLUMN id BIGINT;\nALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL;"
	enriched := enrichA3(t, sql, spec.DialectTiDB, &t05AbsentProvider{})
	column := enriched[2].Metadata.TargetTable.FindColumn("id")
	if column == nil || column.Unsigned || !column.NotNull || modifyBaseType(*column) != "bigint" {
		t.Fatalf("TiDB primary-key widen pre-state = %+v, want signed bigint not null", column)
	}
}

func TestAuditSQLT05A4R1TiDBNonPrimaryKeySignednessStillPublishes(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY, c INT);\nALTER TABLE t MODIFY COLUMN c INT UNSIGNED;\nALTER TABLE t MODIFY COLUMN c BIGINT UNSIGNED;"
	enriched := enrichA3(t, sql, spec.DialectTiDB, &t05AbsentProvider{})
	column := enriched[2].Metadata.TargetTable.FindColumn("c")
	if column == nil || !column.Unsigned {
		t.Fatalf("non-primary-key signedness pre-state = %+v, want unsigned", column)
	}
}
