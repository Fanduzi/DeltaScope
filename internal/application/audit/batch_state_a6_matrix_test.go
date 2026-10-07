// Package audit verifies the T05-A6 DROP COLUMN boundaries beyond the first path.
// input: real parser statements and shared AuditSQL requests, plus loaded snapshots
// output: precise removal only for one ordinary dependency-free column; every other form tombstones the whole affected set
// pos: T05-A6 regression for absence, members, dependents, multi-action scope, stats, and policy orthogonality
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05A6DropProbeSQL = "ALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t ADD COLUMN probe_z INT;"

const t05A6ReaddSQL = "CREATE TABLE t (\n" +
	"  id INT PRIMARY KEY,\n" +
	"  obsolete INT UNSIGNED NOT NULL DEFAULT 7 COMMENT 'retired',\n" +
	"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
	");\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"ALTER TABLE t ADD COLUMN obsolete VARCHAR(12) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN obsolete VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_readded ON t(obsolete);"

func t05A6Base() *spec.TableSnapshot {
	shape := presentTable("golden", "t", "id", "obsolete", "keep_c")
	shape.Columns[0] = spec.Column{Name: "id", Type: "int", NotNull: true}
	shape.Columns[1] = spec.Column{Name: "obsolete", Type: "int"}
	shape.Columns[2] = spec.Column{Name: "keep_c", Type: "varchar", Length: 10, Charset: "utf8mb4", Collation: "utf8mb4_bin", NotNull: true}
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	return shape
}

func t05A6Provider(shape *spec.TableSnapshot) *t05PresentProvider {
	return &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}
}

func t05A6Enrich(t *testing.T, sql string, provider MetadataProvider) []spec.Statement {
	t.Helper()
	return enrichA3(t, sql, spec.DialectMySQL, provider)
}

func t05A6WantPrecise(t *testing.T, enriched []spec.Statement) *spec.TableSnapshot {
	t.Helper()
	if len(enriched) < 2 || enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable == nil {
		t.Fatalf("post-state = %+v, want a present table", enriched)
	}
	snapshot := enriched[1].Metadata.TargetTable
	got := t05A6ColumnNames(snapshot)
	if len(got) != 2 || got[0] != "id" || got[1] != "keep_c" || snapshot.FindColumn("obsolete") != nil {
		t.Fatalf("columns = %#v, want [id keep_c]", got)
	}
	keep := snapshot.FindColumn("keep_c")
	if keep == nil || keep.Length != 10 || keep.Charset != "utf8mb4" || keep.Collation != "utf8mb4_bin" || !keep.NotNull {
		t.Fatalf("keep_c = %+v, want the original varchar(10)", keep)
	}
	return snapshot
}

func TestAuditSQLT05A6RepeatedDropReportsAbsenceThenUnknown(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  obsolete INT,\n" +
		"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
		");\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
			if result.Statements[1].Coverage.Status != report.CoverageComplete || len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
				t.Fatalf("first DROP = findings %#v gaps %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
			}
			findings := t05FindingsByRule(result, 2, t05RuleDropColumnExists)
			if len(findings) != 1 || len(result.Statements[2].Findings) != 1 || len(result.Statements[2].EvidenceGaps) != 0 || result.Statements[2].Coverage.Status != report.CoverageComplete {
				t.Fatalf("second DROP = %#v gaps %#v", result.Statements[2].Findings, result.Statements[2].EvidenceGaps)
			}
			if findings[0].Level != rule.LevelBlocker || findings[0].Metadata["table"] != "t" || findings[0].Metadata["action"] != "drop_column" || findings[0].Metadata["name"] != "obsolete" || findings[0].Metadata["exists"] != false {
				t.Fatalf("absence metadata = %#v", findings[0].Metadata)
			}
			if _, ok := findings[0].Metadata["schema"]; ok {
				t.Fatalf("drop-column existence metadata includes schema: %#v", findings[0].Metadata)
			}
			gaps := t05GapsByRule(result, 3, t05RuleModifyExists)
			if len(result.Statements[3].Findings) != 0 || len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("statement after the absent drop = findings %#v gaps %#v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
			}
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			t05A4R1WantUnknown(t, enriched[3])
		})
	}
}

func TestAuditSQLT05A6LastColumnDoesNotPublishAnEmptyTable(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t DROP COLUMN id;\nALTER TABLE t ADD COLUMN x INT;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
	if len(t05FindingsByRule(result, 1, t05RuleDropColumnExists)) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("dropping the known last column must not invent an existence finding, got %#v / %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
	gaps := t05GapsByRule(result, 2, t05RuleAlterRequire)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("following ADD = %#v, want unknown rather than an empty table", result.Statements[2].EvidenceGaps)
	}
	enriched := t05A6Enrich(t, sql, &t05AbsentProvider{})
	t05A4R1WantUnknown(t, enriched[2])
}

func TestAuditSQLT05A6AbsentTableKeepsParentFindingWithoutMemberGap(t *testing.T) {
	t.Parallel()
	const sql = "ALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t DROP COLUMN obsolete;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
	findings := t05FindingsByRule(result, 0, t05RuleAlterRequire)
	if len(findings) != 1 || findings[0].Metadata["table"] != "t" || findings[0].Metadata["exists"] != false {
		t.Fatalf("parent finding = %#v", result.Statements[0].Findings)
	}
	if len(t05FindingsByRule(result, 0, t05RuleDropColumnExists)) != 0 || len(t05GapsByRule(result, 0, t05RuleDropColumnExists)) != 0 {
		t.Fatalf("confirmed absence must not add a column-member gap, got %#v", result.Statements[0].EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 1, t05RuleDropColumnExists)) != 0 {
		t.Fatalf("later statement must not repeat a fabricated absence, got %#v", result.Statements[1].Findings)
	}
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleDropColumnExists), []string{"target_table.columns", "target_table.existence"})
	enriched := t05A6Enrich(t, sql, &t05AbsentProvider{})
	t05A4R1WantUnknown(t, enriched[1])
}

func TestAuditSQLT05A6IsolatedUnknownDropKeepsColumnFacts(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t DROP COLUMN obsolete;",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, map[string]string{t05RuleDropColumnExists: ""}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("unknown drop must not fabricate a finding, got %#v", result.Statements[0].Findings)
	}
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 0, t05RuleDropColumnExists), []string{"target_table.columns", "target_table.existence"})
}

func TestBatchStateA6KnownEmptyMembersAllowPreciseDrop(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*spec.TableSnapshot)
	}{
		{name: "nil index and constraint sets", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = nil
			shape.Constraints = nil
		}},
		{name: "empty index and constraint sets", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{}
			shape.Constraints = []spec.Constraint{}
		}},
		{name: "nil primary key with unrelated primary constraint", mutate: func(shape *spec.TableSnapshot) {
			shape.PrimaryKey = nil
			shape.Constraints = []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"id"}}}
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := t05A6Base()
			tc.mutate(shape)
			snapshot := t05A6WantPrecise(t, t05A6Enrich(t, t05A6DropProbeSQL, t05A6Provider(shape)))
			if tc.name == "nil primary key with unrelated primary constraint" {
				if snapshot.PrimaryKey != nil || len(snapshot.Constraints) != 1 || snapshot.Constraints[0].Columns[0] != "id" {
					t.Fatalf("primary constraint = pk %+v constraints %+v", snapshot.PrimaryKey, snapshot.Constraints)
				}
			}
		})
	}
}

func TestBatchStateA6UnknownOrSpecialMembersRefusePreciseDrop(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*spec.TableSnapshot)
	}{
		{name: "primary key unknown", mutate: func(shape *spec.TableSnapshot) { shape.PrimaryKeyUnknown = true }},
		{name: "indexes unknown", mutate: func(shape *spec.TableSnapshot) { shape.IndexesUnknown = true }},
		{name: "constraints unknown", mutate: func(shape *spec.TableSnapshot) { shape.ConstraintsUnknown = true }},
		{name: "columns withheld", mutate: func(shape *spec.TableSnapshot) { shape.Columns = nil }},
		{name: "primary key names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"obsolete"}}
		}},
		{name: "primary constraint names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.PrimaryKey = nil
			shape.Constraints = []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"obsolete"}}}
		}},
		{name: "primary key column list is empty", mutate: func(shape *spec.TableSnapshot) {
			shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: nil}
		}},
		{name: "target is auto increment", mutate: func(shape *spec.TableSnapshot) { shape.Columns[1].AutoIncrement = true }},
		{name: "target is auto random", mutate: func(shape *spec.TableSnapshot) { shape.Columns[1].AutoRandom = true }},
		{name: "target is identity", mutate: func(shape *spec.TableSnapshot) { shape.Columns[1].IsIdentity = true }},
		{name: "target is generated", mutate: func(shape *spec.TableSnapshot) { shape.Columns[1].GeneratedWhen = "stored" }},
		{name: "target has an unextracted option", mutate: func(shape *spec.TableSnapshot) {
			shape.Columns[1].UnextractedOptions = []string{"generated"}
		}},
		{name: "ordinary index names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_old", Kind: spec.IndexKindSecondary, Columns: []string{"obsolete"}}}
		}},
		{name: "composite unique names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "uk_old", Kind: spec.IndexKindUnique, Columns: []string{"id", "obsolete"}}}
		}},
		{name: "fulltext index is not a complete ordinary key", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "ft", Kind: spec.IndexKindFulltext, Columns: []string{"keep_c"}}}
		}},
		{name: "prefix index", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_pre", Kind: spec.IndexKindSecondary, Columns: []string{"keep_c"}, PrefixParts: 1}}
		}},
		{name: "expression index", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_expr", Kind: spec.IndexKindSecondary, Columns: []string{"keep_c"}, HasExpressionKeys: true, ExpressionCount: 1}}
		}},
		{name: "included column names the target", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_inc", Kind: spec.IndexKindSecondary, Columns: []string{"id"}, IncludedColumns: []string{"obsolete"}}}
		}},
		{name: "index column list is empty", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_empty", Kind: spec.IndexKindSecondary}}
		}},
		{name: "index column name is blank", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_blank", Kind: spec.IndexKindSecondary, Columns: []string{" "}}}
		}},
		{name: "index has an unmodeled option", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_opt", Kind: spec.IndexKindSecondary, Columns: []string{"keep_c"}, UnmodeledOptions: []string{"comment"}}}
		}},
		{name: "local foreign key names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk", Columns: []string{"obsolete"}, ReferencedSchema: "aux", ReferencedTable: "p", ReferencedColumns: []string{"id"}}}
		}},
		{name: "self reference names the column", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk_self", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t", ReferencedColumns: []string{"obsolete"}}}
		}},
		{name: "check list is empty", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "check", Name: "ck", Columns: nil}}
		}},
		{name: "check has unmodeled parts", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "check", Name: "ck", Columns: []string{"id"}, UnmodeledParts: 1}}
		}},
		{name: "sibling is generated", mutate: func(shape *spec.TableSnapshot) { shape.Columns[2].GeneratedWhen = "virtual" }},
		{name: "sibling default is not provably column-free", mutate: func(shape *spec.TableSnapshot) {
			shape.Columns[2].HasDefault = true
			shape.Columns[2].DefaultValue = "7"
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := t05A6Base()
			tc.mutate(shape)
			t05A4R1WantUnknown(t, t05A6Enrich(t, t05A6DropProbeSQL, t05A6Provider(shape))[1])
		})
	}
}

func TestBatchStateA6UnrelatedMembersStay(t *testing.T) {
	t.Parallel()
	shape := t05A6Base()
	rows := int64(4)
	shape.PrimaryKey.Cardinality = &rows
	shape.Indexes = []spec.Index{
		{Name: "idx_existing", Kind: spec.IndexKindSecondary, Columns: []string{"keep_c"}, Cardinality: &rows},
		{Name: "uk_id_keep", Kind: spec.IndexKindUnique, Columns: []string{"id", "keep_c"}, Cardinality: &rows, IncludedColumns: []string{"keep_c"}},
	}
	shape.Constraints = []spec.Constraint{
		{Type: "foreign_key", Name: "fk_id", Columns: []string{"id"}, ReferencedSchema: "aux", ReferencedTable: "parent", ReferencedColumns: []string{"id"}},
		{Type: "check", Name: "ck_id", Columns: []string{"id"}},
		{Type: "foreign_key", Name: "fk_self", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t", ReferencedColumns: []string{"id"}},
	}
	shape.Columns[2].HasDefault = true
	shape.Columns[2].DefaultIsCurrentTimestamp = true
	shape.Columns[0].HasDefault = true
	shape.Columns[0].DefaultIsNull = true
	shape.Columns[0].IsIdentity = true
	snapshot := t05A6WantPrecise(t, t05A6Enrich(t, t05A6DropProbeSQL, t05A6Provider(shape)))
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "primary" || len(snapshot.PrimaryKey.Columns) != 1 || snapshot.PrimaryKey.Columns[0] != "id" || snapshot.PrimaryKey.Cardinality != nil {
		t.Fatalf("primary key = %+v", snapshot.PrimaryKey)
	}
	if len(snapshot.Indexes) != 2 || snapshot.Indexes[0].Name != "idx_existing" || snapshot.Indexes[0].Columns[0] != "keep_c" || snapshot.Indexes[0].Cardinality != nil {
		t.Fatalf("ordinary index = %+v", snapshot.Indexes)
	}
	if snapshot.Indexes[1].Name != "uk_id_keep" || len(snapshot.Indexes[1].Columns) != 2 || snapshot.Indexes[1].Columns[0] != "id" || snapshot.Indexes[1].Columns[1] != "keep_c" || snapshot.Indexes[1].IncludedColumns[0] != "keep_c" {
		t.Fatalf("unique index = %+v", snapshot.Indexes[1])
	}
	if len(snapshot.Constraints) != 3 || snapshot.Constraints[0].Name != "fk_id" || snapshot.Constraints[2].Name != "fk_self" {
		t.Fatalf("constraints = %+v", snapshot.Constraints)
	}
}

func TestAuditSQLT05A6LoadedChildFollowsTheDroppedColumn(t *testing.T) {
	t.Parallel()
	const related = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t DROP COLUMN obsolete;\nALTER TABLE aux.child ADD COLUMN x INT;\nALTER TABLE golden.t ADD COLUMN y INT;"
	t.Run("referencing child falls with the table", func(t *testing.T) {
		t.Parallel()
		provider := t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"obsolete"}, 0))
		enriched := t05A6Enrich(t, related, provider)
		t05A4R1WantUnknown(t, enriched[2])
		t05A4R1WantUnknown(t, enriched[3])
	})
	t.Run("unmodeled referenced parts fall with the table", func(t *testing.T) {
		t.Parallel()
		enriched := t05A6Enrich(t, related, t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"id"}, 1)))
		t05A4R1WantUnknown(t, enriched[2])
		t05A4R1WantUnknown(t, enriched[3])
	})
	t.Run("complete child on id stays", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t DROP COLUMN obsolete;\nCREATE INDEX after ON aux.child(f);\nALTER TABLE golden.t ADD COLUMN y INT;"
		enriched := t05A6Enrich(t, sql, t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"id"}, 0)))
		if enriched[2].Metadata.TargetTable == nil || enriched[2].Metadata.TargetTable.FindColumn("f") == nil {
			t.Fatalf("unrelated child = %+v", enriched[2].Metadata)
		}
		if got := t05A6ColumnNames(enriched[3].Metadata.TargetTable); len(got) != 2 || got[1] != "keep_c" {
			t.Fatalf("precise parent = %#v", got)
		}
	})
	t.Run("unqualified reference uses the owner schema", func(t *testing.T) {
		t.Parallel()
		child := presentTable("golden", "child", "f")
		child.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk", Columns: []string{"f"}, ReferencedTable: "t", ReferencedColumns: []string{"obsolete"}}}
		provider := t05A6Provider(t05A6Base())
		provider.snapshots["golden.child"] = child
		const sql = "CREATE INDEX warm ON child(f);\nALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE child ADD COLUMN x INT;"
		enriched := t05A6Enrich(t, sql, provider)
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("same table name in another schema stays", func(t *testing.T) {
		t.Parallel()
		other := t05A6Base()
		other.Schema = "other"
		other.Table = &spec.Table{Schema: "other", Name: "t"}
		provider := t05A6Provider(t05A6Base())
		provider.snapshots["other.t"] = other
		const sql = "ALTER TABLE other.t ADD COLUMN extra INT;\nALTER TABLE golden.t DROP COLUMN obsolete;\nALTER TABLE other.t ADD COLUMN extra2 INT;"
		enriched := t05A6Enrich(t, sql, provider)
		if enriched[2].Metadata.TargetTable.FindColumn("obsolete") == nil {
			t.Fatalf("other.t lost obsolete: %+v", enriched[2].Metadata.TargetTable.Columns)
		}
	})
	t.Run("unloaded child is not discovered", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE golden.t DROP COLUMN obsolete;\nCREATE INDEX warm ON aux.child(f);"
		enriched := t05A6Enrich(t, sql, t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"obsolete"}, 0)))
		if enriched[1].Metadata.TargetTable == nil || enriched[1].Metadata.TargetTable.FindColumn("f") == nil {
			t.Fatalf("unloaded child = %+v", enriched[1].Metadata)
		}
	})
}

func TestAuditSQLT05A6MultiActionTombstonesTheWholeStatement(t *testing.T) {
	t.Parallel()
	t.Run("drop and rename tombstones an unloaded destination", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE golden.t DROP COLUMN obsolete, RENAME TO dst.u;\nALTER TABLE dst.u ADD COLUMN x INT;\nALTER TABLE golden.t ADD COLUMN y INT;"
		provider := t05A6Provider(t05A6Base())
		provider.snapshots["dst.u"] = absentTable("dst", "u")
		result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
		t05A4R2WantNoAbsence(t, result, 1)
		t05A4R2WantNoAbsence(t, result, 2)
		if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
			t.Fatalf("provider ledger = %#v, want one source read", provider.calls)
		}
		observed := t05A6Provider(t05A6Base())
		observed.snapshots["dst.u"] = absentTable("dst", "u")
		enriched := t05A6Enrich(t, sql, observed)
		t05A4R1WantUnknown(t, enriched[1])
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("cached absent destination is not reread", func(t *testing.T) {
		t.Parallel()
		const sql = "UPDATE dst.u SET id = 1;\nALTER TABLE golden.t DROP COLUMN obsolete, RENAME TO dst.u;\nALTER TABLE dst.u ADD COLUMN x INT;"
		provider := t05A6Provider(t05A6Base())
		provider.snapshots["dst.u"] = absentTable("dst", "u")
		result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4R1Policy(t))
		t05A4R2WantNoAbsence(t, result, 2)
		if len(provider.calls) != 2 || provider.calls[0] != "dst.u" || provider.calls[1] != "golden.t" {
			t.Fatalf("provider ledger = %#v", provider.calls)
		}
	})
	t.Run("second dropped column still collects its child", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t DROP COLUMN obsolete, DROP COLUMN keep_c;\nALTER TABLE aux.child ADD COLUMN x INT;"
		child := t05A4R1Child([]string{"keep_c"}, 0)
		enriched := t05A6Enrich(t, sql, t05A4R1Provider(t05A6Base(), child))
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("drop plus modify collects a child of the other column", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t DROP COLUMN obsolete, MODIFY COLUMN id INT;\nALTER TABLE aux.child ADD COLUMN x INT;"
		enriched := t05A6Enrich(t, sql, t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"keep_c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("drop plus change collects a child of the other column", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE INDEX warm ON aux.child(f);\nALTER TABLE golden.t DROP COLUMN obsolete, CHANGE COLUMN keep_c renamed VARCHAR(20);\nALTER TABLE aux.child ADD COLUMN x INT;"
		enriched := t05A6Enrich(t, sql, t05A4R1Provider(t05A6Base(), t05A4R1Child([]string{"keep_c"}, 0)))
		t05A4R1WantUnknown(t, enriched[2])
	})
	t.Run("if exists refuses the precise template", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c INT);\nALTER TABLE t DROP COLUMN IF EXISTS obsolete;\nALTER TABLE t ADD COLUMN x INT;"
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
				t05A4R1WantUnknown(t, enriched[2])
			})
		}
	})
}

func TestBatchStateA6UnsupportedDropStillTombstonesDependents(t *testing.T) {
	t.Parallel()
	state := newBatchState(spec.DialectMySQL, "golden", &t05AbsentProvider{})
	targetKey := state.keyFor("golden", spec.Table{Name: "t"})
	childKey := state.keyFor("golden", spec.Table{Schema: "aux", Name: "child"})
	state.entries[targetKey] = &batchTableEntry{state: tablePresent, shape: t05A6Base(), displaySchema: "golden", displayTable: "t"}
	child := t05A4R1Child([]string{"keep_c"}, 0)
	state.entries[childKey] = &batchTableEntry{state: tablePresent, shape: child, displaySchema: "aux", displayTable: "child"}
	statements := t05A2R1Extract(t, "ALTER TABLE t DROP COLUMN obsolete;")
	statements[0].Unsupported = &spec.UnsupportedDetail{Feature: "drop_column.injected", Reason: spec.UnsupportedUnauditedReason}
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if state.entries[targetKey].state != tableUnknown || state.entries[childKey].state != tableUnknown {
		t.Fatalf("unsupported drop left target=%v child=%v", state.entries[targetKey].state, state.entries[childKey].state)
	}
	if statements[0].Unsupported == nil || statements[0].Unsupported.Feature != "drop_column.injected" {
		t.Fatalf("marker = %+v", statements[0].Unsupported)
	}
}

func TestAuditSQLT05A6SameNameAddUsesTheNewDefinition(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t05A4Audit(t, t05A6ReaddSQL, dialect, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
			if len(result.Statements) != 5 || result.RuleSummary == nil || result.RuleSummary.Loaded != 6 || result.Verdict != report.VerdictPass {
				t.Fatalf("result statements=%d loaded=%v verdict=%s", len(result.Statements), result.RuleSummary, result.Verdict)
			}
			for index, statement := range result.Statements {
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %s findings %#v gaps %#v", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			enriched := enrichA3(t, t05A6ReaddSQL, dialect, &t05AbsentProvider{})
			old := enriched[1].Metadata.TargetTable.FindColumn("obsolete")
			if old == nil || !old.Unsigned || old.DefaultValue != "7" || old.Comment != "retired" {
				t.Fatalf("DROP pre-state obsolete = %+v", old)
			}
			if enriched[2].Metadata.TargetTable.FindColumn("obsolete") != nil {
				t.Fatalf("ADD pre-state still has obsolete: %#v", t05A6ColumnNames(enriched[2].Metadata.TargetTable))
			}
			added := enriched[3].Metadata.TargetTable
			if got := t05A6ColumnNames(added); len(got) != 3 || got[0] != "id" || got[1] != "keep_c" || got[2] != "obsolete" {
				t.Fatalf("MODIFY pre-state columns = %#v", got)
			}
			column := added.FindColumn("obsolete")
			if column == nil || column.Length != 12 || column.Unsigned || column.DefaultValue != "" || column.Comment != "" || column.Charset != "utf8mb4" || !column.NotNull {
				t.Fatalf("re-added obsolete = %+v", column)
			}
			if got := t05A4Length(t, enriched[4], "obsolete"); got != 20 {
				t.Fatalf("CREATE INDEX pre-state obsolete.Length = %d", got)
			}
			probed := enrichA3(t, t05A6ReaddSQL+"\nALTER TABLE t ADD COLUMN probe_z INT;", dialect, &t05AbsentProvider{})
			idx := probed[5].Metadata.TargetTable.FindIndex("idx_readded")
			if idx == nil || len(idx.Columns) != 1 || idx.Columns[0] != "obsolete" {
				t.Fatalf("idx_readded = %+v", probed[5].Metadata.TargetTable.Indexes)
			}
			if old.DefaultValue != "7" || old.Comment != "retired" {
				t.Fatalf("later statements rewrote the DROP pre-state obsolete to %+v", old)
			}
		})
	}
}

func TestBatchStateA6StatsCopyAndIdentityIsolation(t *testing.T) {
	t.Parallel()
	rows := int64(11)
	shape := t05A6Base()
	shape.PrimaryKey.Cardinality = &rows
	shape.Indexes = []spec.Index{{Name: "idx_existing", Kind: spec.IndexKindSecondary, Columns: []string{"keep_c"}, Cardinality: &rows}}
	shape.Options = map[string]string{
		"table_rows": "9", "data_length": "100", "index_length": "40", "avg_row_length": "8", "auto_increment": "4", "comment": "kept",
	}
	provider := t05A6Provider(shape)
	enriched := t05A6Enrich(t, t05A6DropProbeSQL, provider)
	snapshot := t05A6WantPrecise(t, enriched)
	if snapshot.Options["table_rows"] != "9" || snapshot.Options["comment"] != "kept" {
		t.Fatalf("kept options = %#v", snapshot.Options)
	}
	for _, key := range []string{"data_length", "index_length", "avg_row_length", "auto_increment"} {
		if _, ok := snapshot.Options[key]; ok {
			t.Fatalf("post-drop options still contain %s: %#v", key, snapshot.Options)
		}
	}
	if snapshot.PrimaryKey.Cardinality != nil || snapshot.Indexes[0].Cardinality != nil || snapshot.Indexes[0].Name != "idx_existing" {
		t.Fatalf("index stats = %+v / %+v", snapshot.PrimaryKey, snapshot.Indexes)
	}
	if len(shape.Columns) != 3 || shape.Options["data_length"] != "100" || shape.PrimaryKey.Cardinality == nil {
		t.Fatalf("provider backing changed: columns=%d options=%#v", len(shape.Columns), shape.Options)
	}
	enriched[0].Metadata = nil
	if snapshot.FindColumn("keep_c").Length != 10 {
		t.Fatal("clearing an earlier statement rewrote the published post-state")
	}
	shape.Columns[2].Length = 999
	if snapshot.FindColumn("keep_c").Length != 10 {
		t.Fatal("mutating the provider slice rewrote the published post-state")
	}

	t.Run("requests do not share state", func(t *testing.T) {
		t.Parallel()
		_ = t05A4Audit(t, t05A6FirstPathSQL, spec.DialectMySQL, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
		second := t05A4Audit(t, "ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;", spec.DialectMySQL, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
		if len(t05FindingsByRule(second, 0, t05RuleAlterRequire)) != 1 {
			t.Fatalf("a new request must not see the previous drop, got %#v", second.Statements[0])
		}
	})
	t.Run("dotted name stays whole", func(t *testing.T) {
		t.Parallel()
		dotted := presentTable("golden", "a.b", "id", "obsolete")
		dotted.Columns[0] = spec.Column{Name: "id", Type: "int", NotNull: true}
		dotted.Columns[1] = spec.Column{Name: "obsolete", Type: "int"}
		dotted.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.a.b": dotted}}
		enriched := t05A6Enrich(t, "ALTER TABLE `a.b` DROP COLUMN obsolete;\nALTER TABLE `a.b` ADD COLUMN x INT;", provider)
		if got := t05A6ColumnNames(enriched[1].Metadata.TargetTable); len(got) != 1 || got[0] != "id" {
			t.Fatalf("dotted post-state = %#v", got)
		}
		if len(provider.calls) != 1 || provider.calls[0] != "golden.a.b" {
			t.Fatalf("provider ledger = %#v", provider.calls)
		}
	})
}

func TestBatchStateA6CancelAndProviderErrorPublishNothing(t *testing.T) {
	t.Parallel()
	load := func(t *testing.T) (*batchState, batchTableKey, *spec.TableSnapshot) {
		t.Helper()
		shape := t05A6Base()
		provider := t05A6Provider(shape)
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load: %v", err)
		}
		key := state.keyFor("golden", spec.Table{Name: "t"})
		return state, key, cloneTableSnapshot(state.entries[key].shape)
	}
	t.Run("canceled before apply", func(t *testing.T) {
		t.Parallel()
		state, key, before := load(t)
		statements := t05A2R1Extract(t, "ALTER TABLE t DROP COLUMN obsolete;")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		defer cancel()
		err := state.apply(ctx, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply = %v", err)
		}
		if state.entries[key].state != tablePresent || len(state.entries[key].shape.Columns) != len(before.Columns) {
			t.Fatal("canceled apply published a drop")
		}
	})
	t.Run("canceled before publication", func(t *testing.T) {
		t.Parallel()
		state, key, before := load(t)
		statements := t05A2R1Extract(t, "ALTER TABLE t DROP COLUMN obsolete;")
		err := state.apply(&t05A6CancelAfter{Context: context.Background(), remain: 1}, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply = %v", err)
		}
		if state.entries[key].state != tablePresent || len(state.entries[key].shape.Columns) != len(before.Columns) || state.entries[key].shape.FindColumn("obsolete") == nil {
			t.Fatal("the publication cancel point wrote a post-state")
		}
	})
	t.Run("provider error keeps identity", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sentinel drop read failure")
		provider := &t05PresentProvider{errOn: map[string]error{"golden.t": wantErr}}
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t DROP COLUMN obsolete;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A6SixRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("provider error = %v", err)
		}
	})
}

type t05A6CancelAfter struct {
	context.Context
	remain int
}

func (c *t05A6CancelAfter) Err() error {
	if c.remain <= 0 {
		return context.Canceled
	}
	c.remain--
	return nil
}

func TestAuditSQLT05A6PolicyBlockerStillPublishes(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:            "",
		t05RuleAlterRequire:            "",
		t05RuleDropColumnExists:        "",
		t05RuleModifyExists:            "",
		t05RuleModifyCompat:            "      required: true\n",
		t05RuleCreateIndexColumns:      "      required: true\n",
		"ddl.alter.drop_column.forbid": "",
	})
	result := t05A4Audit(t, t05A6FirstPathSQL, spec.DialectMySQL, &t05AbsentProvider{}, policy)
	findings := t05FindingsByRule(result, 1, "ddl.alter.drop_column.forbid")
	if len(findings) != 1 || findings[0].Level != rule.LevelBlocker || len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("forbid finding = %#v gaps %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict = %s, want reject", result.Verdict)
	}
	if result.Statements[2].Coverage.Status != report.CoverageComplete || len(result.Statements[2].EvidenceGaps) != 0 || len(result.Statements[2].Findings) != 0 {
		t.Fatalf("MODIFY after a policy blocker = %#v", result.Statements[2])
	}
	enriched := t05A6Enrich(t, t05A6FirstPathSQL, &t05AbsentProvider{})
	if got := t05A4Length(t, enriched[2], "keep_c"); got != 10 {
		t.Fatalf("policy blocker suppressed the post-state, length = %d", got)
	}
}

func TestAuditSQLT05A6DisabledAndInapplicableDropRule(t *testing.T) {
	t.Parallel()
	t.Run("disabled existence rule still publishes", func(t *testing.T) {
		t.Parallel()
		policy := t05PolicyPath(t, map[string]string{
			t05RuleCreateForbid: "",
			t05RuleAlterRequire: "",
			t05RuleModifyExists: "",
			t05RuleModifyCompat: "      required: true\n",
		})
		result := t05A4Audit(t, t05A6FirstPathSQL, spec.DialectMySQL, &t05AbsentProvider{}, policy)
		if len(t05FindingsByRule(result, 1, t05RuleDropColumnExists)) != 0 || result.Statements[2].Coverage.Status != report.CoverageComplete {
			t.Fatalf("disabled drop rule changed the successor: %#v / %#v", result.Statements[1], result.Statements[2])
		}
	})
	t.Run("drop rule does not apply to create index", func(t *testing.T) {
		t.Parallel()
		result := t05A4Audit(t, t05A6FirstPathSQL, spec.DialectMySQL, &t05AbsentProvider{}, t05PolicyPath(t, map[string]string{t05RuleDropColumnExists: ""}))
		if len(result.Statements[3].Findings) != 0 || len(result.Statements[3].EvidenceGaps) != 0 {
			t.Fatalf("create index gained a drop-column result: %#v", result.Statements[3])
		}
	})
	t.Run("only drop exists on an absent table", func(t *testing.T) {
		t.Parallel()
		result := t05A4Audit(t, "ALTER TABLE t DROP COLUMN obsolete;", spec.DialectMySQL, &t05AbsentProvider{}, t05PolicyPath(t, map[string]string{t05RuleDropColumnExists: ""}))
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("absent table with only drop.exists = findings %#v gaps %#v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
}

func TestAuditSQLT05A6ExecuteDoesNotPublishASuccessor(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(10) NOT NULL);\nEXECUTE stmt1;\nALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) NOT NULL;"
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t05A6SixRulePolicy(t),
		MetadataProvider: provider,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit error = %v, want ErrUnsupportedStatement", err)
	}
	if len(t05GapsByRule(result, 3, t05RuleModifyExists)) == 0 || len(result.Statements[3].Findings) != 0 {
		t.Fatalf("contaminated MODIFY = findings %#v gaps %#v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
	}
	enriched := t05A6Enrich(t, sql, &t05AbsentProvider{})
	t05A4R1WantUnknown(t, enriched[3])
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
		t.Fatalf("provider ledger = %#v, want the CREATE read and no later reread", provider.calls)
	}
}

func TestBatchStateA6DuplicateColumnIdentityIsNotGuessed(t *testing.T) {
	t.Parallel()
	shape := t05A6Base()
	shape.Columns = append(shape.Columns, spec.Column{Name: "Obsolete", Type: "int"})
	state := newBatchState(spec.DialectMySQL, "golden", t05A6Provider(shape))
	if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
		t.Fatalf("load: %v", err)
	}
	key := state.keyFor("golden", spec.Table{Name: "t"})
	statements := t05A2R1Extract(t, "ALTER TABLE t DROP COLUMN obsolete;")
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if state.entries[key].state != tableUnknown {
		t.Fatalf("duplicate identities published state %v columns %#v", state.entries[key].state, t05A6ColumnNames(state.entries[key].shape))
	}
}

func TestBatchStateA6TemplateModifiersRefusePreciseDrop(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*spec.Statement)
	}{
		{name: "position", mutate: func(statement *spec.Statement) { statement.DDL.Alter[0].HasColumnPosition = true }},
		{name: "unextracted alter option", mutate: func(statement *spec.Statement) {
			statement.DDL.Alter[0].UnextractedOptions = []string{"algorithm"}
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := newBatchState(spec.DialectMySQL, "golden", t05A6Provider(t05A6Base()))
			if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
				t.Fatalf("load: %v", err)
			}
			key := state.keyFor("golden", spec.Table{Name: "t"})
			statements := t05A2R1Extract(t, "ALTER TABLE t DROP COLUMN obsolete;")
			tc.mutate(&statements[0])
			if err := state.apply(context.Background(), statements[0]); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if state.entries[key].state != tableUnknown {
				t.Fatalf("%s published a precise drop", tc.name)
			}
		})
	}
}

func TestAuditSQLT05A6TiDBProviderDropMatchesMySQL(t *testing.T) {
	t.Parallel()
	snapshot := t05A6WantPrecise(t, enrichA3(t, t05A6DropProbeSQL, spec.DialectTiDB, t05A6Provider(t05A6Base())))
	if snapshot.FindColumn("keep_c").Length != 10 {
		t.Fatalf("tidb keep_c = %+v", snapshot.FindColumn("keep_c"))
	}
}

func TestAuditSQLT05A6SourceColumnAbsenceKeepsTheFinding(t *testing.T) {
	t.Parallel()
	shape := presentTable("golden", "t", "id", "keep_c")
	shape.Columns[1] = spec.Column{Name: "keep_c", Type: "varchar", Length: 10}
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	const sql = "ALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20);"
	result := t05A4Audit(t, sql, spec.DialectMySQL, t05A6Provider(shape), t05A6SixRulePolicy(t))
	findings := t05FindingsByRule(result, 0, t05RuleDropColumnExists)
	if len(findings) != 1 || findings[0].Metadata["exists"] != false || findings[0].Metadata["name"] != "obsolete" || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("missing source = %#v gaps %#v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	enriched := t05A6Enrich(t, sql, t05A6Provider(shape))
	t05A4R1WantUnknown(t, enriched[1])
}

func TestAuditSQLT05A6EmptyColumnSetDoesNotBecomeDropTable(t *testing.T) {
	t.Parallel()
	shape := presentTable("golden", "t")
	shape.Columns = []spec.Column{}
	const sql = "ALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t DROP COLUMN obsolete;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, t05A6Provider(shape), t05A6SixRulePolicy(t))
	findings := t05FindingsByRule(result, 0, t05RuleDropColumnExists)
	if len(findings) != 1 || findings[0].Metadata["exists"] != false || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("empty column set = %#v", result.Statements[0])
	}
	if len(t05FindingsByRule(result, 1, t05RuleDropColumnExists)) != 0 {
		t.Fatalf("second statement fabricated another absence: %#v", result.Statements[1].Findings)
	}
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleDropColumnExists), []string{"target_table.columns", "target_table.existence"})
}

func TestBatchStateA6WithheldColumnsInvalidateTheWholeTable(t *testing.T) {
	t.Parallel()
	shape := presentTable("golden", "t", "id", "obsolete")
	shape.Columns = nil
	const sql = "ALTER TABLE t DROP COLUMN obsolete;\nALTER TABLE t DROP COLUMN id;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, t05A6Provider(shape), t05PolicyPath(t, map[string]string{t05RuleDropColumnExists: ""}))
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 0, t05RuleDropColumnExists), []string{"target_table.columns"})
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleDropColumnExists), []string{"target_table.columns", "target_table.existence"})
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[1].Findings) != 0 {
		t.Fatalf("withheld columns fabricated findings: %#v / %#v", result.Statements[0].Findings, result.Statements[1].Findings)
	}
}

func TestAuditSQLT05A6UnrelatedIndexSQLKeepsKeyOrder(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  obsolete INT,\n" +
		"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,\n" +
		"  KEY idx_existing (keep_c),\n" +
		"  UNIQUE KEY uk_id_keep (id, keep_c)\n" +
		");\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX idx_keep ON t(keep_c);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A6SixRulePolicy(t))
			for index, statement := range result.Statements {
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %s findings %#v gaps %#v", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			for _, index := range []int{2, 3} {
				snapshot := enriched[index].Metadata.TargetTable
				if snapshot.FindColumn("obsolete") != nil || snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Columns[0] != "id" {
					t.Fatalf("statement %d shape lost the primary key or kept obsolete", index)
				}
				ordinary := snapshot.FindIndex("idx_existing")
				unique := snapshot.FindIndex("uk_id_keep")
				if ordinary == nil || len(ordinary.Columns) != 1 || ordinary.Columns[0] != "keep_c" {
					t.Fatalf("statement %d idx_existing = %+v", index, snapshot.Indexes)
				}
				if unique == nil || len(unique.Columns) != 2 || unique.Columns[0] != "id" || unique.Columns[1] != "keep_c" {
					t.Fatalf("statement %d uk_id_keep = %+v", index, unique)
				}
			}
		})
	}
}

func TestBatchStateA6CaseFoldDropsTheSingleColumn(t *testing.T) {
	t.Parallel()
	enriched := t05A6Enrich(t, "ALTER TABLE t DROP COLUMN Obsolete;\nALTER TABLE t ADD COLUMN probe_z INT;", t05A6Provider(t05A6Base()))
	if got := t05A6ColumnNames(enriched[1].Metadata.TargetTable); len(got) != 2 || got[0] != "id" || got[1] != "keep_c" {
		t.Fatalf("case-folded drop = %#v", got)
	}
}
