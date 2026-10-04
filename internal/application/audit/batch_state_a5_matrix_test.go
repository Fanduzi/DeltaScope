// Package audit verifies the T05-A5 column-identity matrix beyond the first path.
// input: real parser statements and AuditSQL requests for CHANGE and RENAME COLUMN
// output: public findings, gaps, and direct pre-state for conflict, version, members, and publication
// pos: T05-A5 shared regression for the frozen identity, attribute, and version contracts
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t05A5ChangeTarget  = "ddl.alter.change_column.target.exists.forbid"
	t05A5RenameTarget  = "ddl.alter.rename_column.target.exists.forbid"
	t05A5RenameVersion = "ddl.alter.rename_column.version.require"
)

type t05AbsentVersionProvider struct {
	t05AbsentProvider
	banner string
}

func (p *t05AbsentVersionProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.banner}, nil
}

func t05A5AuditVersion(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, version string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       t05A5IdentityPolicy(t),
		MetadataProvider: provider,
		TargetVersion:    version,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

func t05A5ColumnNames(t *testing.T, statement spec.Statement) []string {
	t.Helper()
	if statement.Metadata == nil || statement.Metadata.TargetTable == nil {
		t.Fatalf("missing pre-state: %+v", statement.Metadata)
	}
	names := make([]string, 0, len(statement.Metadata.TargetTable.Columns))
	for _, column := range statement.Metadata.TargetTable.Columns {
		names = append(names, column.Name)
	}
	return names
}

func TestAuditSQLT05A5ChangeOldNameIndex(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
		");\n" +
		"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX ix_old ON t(c);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A5IdentityPolicy(t))
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want complete/reject", result.Coverage.Status, result.Verdict)
			}
			findings := t05FindingsByRule(result, 3, t05RuleCreateIndexColumns)
			if len(findings) != 1 || findings[0].Level != rule.LevelBlocker || findings[0].Metadata["column"] != "c" || findings[0].Metadata["exists"] != false || findings[0].Metadata["index"] != "ix_old" || findings[0].Metadata["table"] != "t" {
				t.Fatalf("old-name index finding = %#v", findings)
			}
			if len(result.Statements[3].Findings) != 1 || len(result.Statements[3].EvidenceGaps) != 0 {
				t.Fatalf("statement 3 = findings %#v gaps %#v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
			}
		})
	}
}

func TestAuditSQLT05A5RenameFirstPath(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
		");\n" +
		"ALTER TABLE t RENAME COLUMN c TO c2;\n" +
		"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX idx_c2 ON t(c2);"
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectMySQL, "8.4.10"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect)+"/"+tc.banner, func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentVersionProvider{banner: tc.banner}
			result := t05A4Audit(t, sql, tc.dialect, provider, t05A5IdentityPolicy(t))
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s findings=%#v gaps=%#v", result.Coverage.Status, result.Verdict, result.Statements, result.Statements)
			}
			for index, statement := range result.Statements {
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %s findings %#v gaps %#v", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider ledger = %#v", provider.calls)
			}
			enriched := enrichA3(t, sql, tc.dialect, &t05AbsentVersionProvider{banner: tc.banner})
			if !reflect.DeepEqual(t05A5ColumnNames(t, enriched[1]), []string{"id", "c"}) || t05A4Length(t, enriched[1], "c") != 10 {
				t.Fatalf("RENAME pre-state = %#v", enriched[1].Metadata.TargetTable.Columns)
			}
			if !reflect.DeepEqual(t05A5ColumnNames(t, enriched[2]), []string{"id", "c2"}) || t05A4Length(t, enriched[2], "c2") != 10 {
				t.Fatalf("MODIFY pre-state = %#v", enriched[2].Metadata.TargetTable.Columns)
			}
			if got := t05A4Length(t, enriched[3], "c2"); got != 20 || enriched[3].Metadata.TargetTable.FindColumn("c") != nil {
				t.Fatalf("CREATE INDEX pre-state length %d columns %#v", got, enriched[3].Metadata.TargetTable.Columns)
			}
		})
	}
}

func TestAuditSQLT05A5TargetConflictAndSameName(t *testing.T) {
	t.Parallel()
	t.Run("change conflict", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (c INT, c2 INT);\nALTER TABLE t CHANGE COLUMN c c2 INT;\nALTER TABLE t ADD COLUMN d INT;"
		result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A5IdentityPolicy(t))
		findings := t05FindingsByRule(result, 1, t05A5ChangeTarget)
		if len(findings) != 1 || findings[0].Metadata["source_column"] != "c" || findings[0].Metadata["target_column"] != "c2" || findings[0].Metadata["exists"] != true || findings[0].Metadata["action"] != "change_column" || findings[0].Metadata["table"] != "t" {
			t.Fatalf("conflict finding = %#v", findings)
		}
		if result.Statements[1].Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
			t.Fatalf("conflict result = %s/%s", result.Coverage.Status, result.Verdict)
		}
		if len(t05GapsByRule(result, 2, t05RuleAlterRequire)) != 1 {
			t.Fatalf("successor after conflict = %#v", result.Statements[2].EvidenceGaps)
		}
	})
	t.Run("rename conflict and absent source", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (c INT, c2 INT);\nALTER TABLE t RENAME COLUMN missing TO c2;"
		result := t05A5AuditVersion(t, sql, spec.DialectMySQL, nil, "8.4.10")
		if len(t05FindingsByRule(result, 1, "ddl.alter.rename_column.exists.require")) != 1 || len(t05FindingsByRule(result, 1, t05A5RenameTarget)) != 1 {
			t.Fatalf("source and target findings = %#v", result.Statements[1].Findings)
		}
	})
	t.Run("same name is not a conflict", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10));\nALTER TABLE t CHANGE COLUMN c c VARCHAR(20);\nALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A5IdentityPolicy(t))
		if len(t05FindingsByRule(result, 1, t05A5ChangeTarget)) != 0 {
			t.Fatalf("same-name CHANGE reported a conflict: %#v", result.Statements[1].Findings)
		}
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		if got := t05A4Length(t, enriched[2], "c"); got != 20 {
			t.Fatalf("same-name CHANGE pre-state length = %d, want 20", got)
		}
	})
	t.Run("same name rename keeps attributes", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\nALTER TABLE t RENAME COLUMN c TO c;\nALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentVersionProvider{banner: "8.4.10"})
		column := enriched[2].Metadata.TargetTable.FindColumn("c")
		if column == nil || column.Length != 10 || column.Charset != "utf8mb4" || !column.NotNull {
			t.Fatalf("same-name RENAME changed the column: %+v", column)
		}
	})
	t.Run("case and schema identity", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE a.t (C INT PRIMARY KEY);\nCREATE TABLE b.t (c INT PRIMARY KEY);\nALTER TABLE a.t CHANGE COLUMN c c2 INT;\nALTER TABLE a.t ADD COLUMN d INT;\nALTER TABLE b.t MODIFY COLUMN c BIGINT NOT NULL;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		renamed := enriched[3].Metadata.TargetTable
		if renamed == nil || renamed.Schema != "a" || renamed.FindColumn("c2") == nil || renamed.FindColumn("c") != nil {
			t.Fatalf("a.t post-state = %#v", renamed)
		}
		untouched := enriched[4].Metadata.TargetTable
		if untouched == nil || untouched.Schema != "b" || untouched.FindColumn("c") == nil || untouched.FindColumn("c2") != nil {
			t.Fatalf("b.t was rewritten from a.t: %#v", untouched)
		}
	})
}

func TestBatchStateA5MembersAndPartialKnowledge(t *testing.T) {
	t.Parallel()
	t.Run("primary key and index references", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (c INT PRIMARY KEY, k INT, KEY idx_c(c), UNIQUE KEY uk_c_k(c, k));\n" +
			"ALTER TABLE t CHANGE COLUMN c c2 INT;\n" +
			"ALTER TABLE t MODIFY COLUMN c2 BIGINT NOT NULL;\n" +
			"CREATE INDEX ix_new ON t(c2);"
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			shape := enriched[2].Metadata.TargetTable
			column := shape.FindColumn("c2")
			if column == nil || !column.NotNull || !strings.HasPrefix(strings.ToLower(column.Type), "int") {
				t.Fatalf("%s c2 = %+v", dialect, column)
			}
			if shape.PrimaryKey == nil || !reflect.DeepEqual(shape.PrimaryKey.Columns, []string{"c2"}) {
				t.Fatalf("%s primary key = %+v", dialect, shape.PrimaryKey)
			}
			idx := shape.FindIndex("idx_c")
			uk := shape.FindIndex("uk_c_k")
			if idx == nil || !reflect.DeepEqual(idx.Columns, []string{"c2"}) || uk == nil || !reflect.DeepEqual(uk.Columns, []string{"c2", "k"}) {
				t.Fatalf("%s indexes = %#v", dialect, shape.Indexes)
			}
			widened := enriched[3].Metadata.TargetTable.FindColumn("c2")
			if widened == nil || !widened.NotNull || !strings.Contains(strings.ToLower(widened.Type), "bigint") {
				t.Fatalf("%s widened column = %+v", dialect, widened)
			}
		}
	})
	t.Run("rename keeps stats and change clears them", func(t *testing.T) {
		t.Parallel()
		cardinality := int64(4)
		shape := presentTable("golden", "t", "c")
		shape.Columns[0] = spec.Column{Name: "c", Type: "varchar", Length: 10, Charset: "utf8mb4", NotNull: true}
		shape.Options = map[string]string{"data_length": "100", "table_rows": "3"}
		shape.Indexes = []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, Cardinality: &cardinality}}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}
		renamed := enrichA3(t, "ALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05BannerPresent{inner: provider, banner: "8.4.10"})
		next := renamed[1].Metadata.TargetTable
		if next.Options["data_length"] != "100" || next.Indexes[0].Cardinality == nil || *next.Indexes[0].Cardinality != 4 || next.Indexes[0].Name != "idx_c" {
			t.Fatalf("RENAME dropped stats or renamed the index: %+v %#v", next.Options, next.Indexes)
		}
		changed := enrichA3(t, "ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10);\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": cloneTableSnapshot(shape)}})
		cleared := changed[1].Metadata.TargetTable
		if _, ok := cleared.Options["data_length"]; ok || cleared.Indexes[0].Cardinality != nil || cleared.Options["table_rows"] != "3" {
			t.Fatalf("CHANGE stats = options %#v cardinality %#v", cleared.Options, cleared.Indexes[0].Cardinality)
		}
	})
	t.Run("withheld columns scrub old references", func(t *testing.T) {
		t.Parallel()
		shape := &spec.TableSnapshot{
			Exists: true, Schema: "golden", Table: &spec.Table{Schema: "golden", Name: "t"},
			Indexes: []spec.Index{
				{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
				{Name: "idx_k", Kind: spec.IndexKindSecondary, Columns: []string{"k"}, PrefixParts: 1},
			},
			PrimaryKey:  &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"c"}},
			Constraints: []spec.Constraint{},
		}
		enriched := enrichA3(t, "ALTER TABLE t CHANGE COLUMN c c2 INT;\nCREATE INDEX ix ON t(k);", spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		got := enriched[1].Metadata.TargetTable
		if got == nil || !got.Exists || got.Columns != nil || !got.PrimaryKeyUnknown || got.PrimaryKey != nil || !got.IndexesUnknown {
			t.Fatalf("scrubbed snapshot = %+v", got)
		}
		if len(got.Indexes) != 1 || got.Indexes[0].Name != "idx_k" {
			t.Fatalf("unrelated prefix index = %#v", got.Indexes)
		}
	})
	t.Run("missing source type still accepts a full change", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "c")
		shape.Columns[0].Type = ""
		enriched := enrichA3(t, "ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10);\nALTER TABLE t MODIFY COLUMN c2 VARCHAR(12);", spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		if got := t05A4Length(t, enriched[1], "c2"); got != 10 {
			t.Fatalf("partial source CHANGE length = %d", got)
		}
	})
	t.Run("rename keeps an unknown type", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "c")
		shape.Columns[0].Type = ""
		shape.Columns[0].Length = 0
		enriched := enrichA3(t, "ALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05BannerPresent{inner: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}, banner: "8.4.10"})
		column := enriched[1].Metadata.TargetTable.FindColumn("c2")
		if column == nil || column.Type != "" || column.Length != 0 {
			t.Fatalf("RENAME invented a type: %+v", column)
		}
	})
}

type t05BannerPresent struct {
	inner  *t05PresentProvider
	banner string
}

func (p *t05BannerPresent) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.banner}, nil
}

func (p *t05BannerPresent) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
}

func TestBatchStateA5UnsafeAndNonPrecise(t *testing.T) {
	t.Parallel()
	t.Run("self foreign key invalidates the target", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (parent_c VARCHAR(10), c VARCHAR(10), CONSTRAINT fk_self FOREIGN KEY (parent_c) REFERENCES t(c));\n" +
			"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10);\nALTER TABLE t ADD COLUMN x INT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("self-fk CHANGE published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("expression index invalidates", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "c", "k")
		shape.Indexes = []spec.Index{{Name: "idx_expr", Kind: spec.IndexKindSecondary, Columns: []string{"k"}, HasExpressionKeys: true}}
		enriched := enrichA3(t, "ALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05BannerPresent{inner: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}, banner: "8.4.10"})
		if enriched[1].Metadata.TargetTable != nil {
			t.Fatalf("expression index RENAME published %+v", enriched[1].Metadata.TargetTable)
		}
	})
	t.Run("multi-action and position stay unknown", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (id INT PRIMARY KEY, c INT);\nALTER TABLE t CHANGE COLUMN c c2 INT, RENAME TO u;\nALTER TABLE u ADD COLUMN d INT;"
		provider := &t05AbsentProvider{}
		enriched := enrichA3(t, sql, spec.DialectMySQL, provider)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("multi-action published a destination: %+v", enriched[2].Metadata.TargetTable)
		}
		for _, call := range provider.calls {
			if call == "golden.u" {
				t.Fatalf("new name reread the provider: %#v", provider.calls)
			}
		}
		position := "CREATE TABLE t (id INT PRIMARY KEY, c INT);\nALTER TABLE t CHANGE COLUMN c c2 INT FIRST;\nALTER TABLE t ADD COLUMN d INT;"
		enriched = enrichA3(t, position, spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatal("positional CHANGE published a column")
		}
	})
	t.Run("prior parser error does not publish", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "SELECT * FROM WHERE;\nALTER TABLE t CHANGE COLUMN c c2 INT;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A5IdentityPolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, errParserUnsupported) {
			t.Fatalf("err = %v", err)
		}
		if len(provider.calls) != 0 {
			t.Fatalf("contaminated batch read the provider: %#v", provider.calls)
		}
	})
}

func TestAuditSQLT05A5VersionGate(t *testing.T) {
	t.Parallel()
	rename := "CREATE TABLE t (c INT);\nALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t ADD COLUMN d INT;"
	cases := []struct {
		name    string
		version string
		wantGap string
		wantHit bool
	}{
		{name: "missing", wantGap: "missing_target_version"},
		{name: "5.7.44", version: "5.7.44", wantHit: true},
		{name: "8.0.2", version: "8.0.2", wantHit: true},
		{name: "8.0.3", version: "8.0.3"},
		{name: "8.0.46", version: "8.0.46"},
		{name: "8.4.10", version: "8.4.10"},
		{name: "9.0.0", version: "9.0.0", wantGap: "target_version_out_of_validated_range"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := t05A5AuditVersion(t, rename, spec.DialectMySQL, nil, tc.version)
			findings := t05FindingsByRule(result, 1, t05A5RenameVersion)
			gaps := t05GapsByRule(result, 1, t05A5RenameVersion)
			if tc.wantHit {
				if len(findings) != 1 || len(gaps) != 0 || findings[0].Metadata["target_version"] != tc.version || findings[0].Metadata["minimum_supported_version"] != "8.0.3" || findings[0].Metadata["product"] != "mysql" || findings[0].Metadata["action"] != "rename_column" {
					t.Fatalf("finding = %#v gaps %#v", findings, gaps)
				}
				if result.Statements[1].Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
					t.Fatalf("incompatible coverage = %s/%s", result.Coverage.Status, result.Verdict)
				}
			} else if len(findings) != 0 {
				t.Fatalf("unexpected version finding %#v", findings)
			}
			if tc.wantGap != "" {
				if len(gaps) != 1 || gaps[0].ReasonCode != tc.wantGap {
					t.Fatalf("gaps = %#v, want %s", gaps, tc.wantGap)
				}
			}
			published := len(t05GapsByRule(result, 2, t05RuleModifyExists)) == 0 && len(result.Statements[2].EvidenceGaps) == 0
			if tc.wantHit || tc.wantGap != "" {
				if published {
					t.Fatal("unsupported RENAME published c2")
				}
			} else if !published || len(result.Statements[2].Findings) != 0 {
				t.Fatalf("supported RENAME did not publish: %#v", result.Statements[2])
			}
		})
	}
	t.Run("tidb 8.5 publishes", func(t *testing.T) {
		t.Parallel()
		result := t05A5AuditVersion(t, rename, spec.DialectTiDB, nil, "8.5.0")
		if len(t05FindingsByRule(result, 1, t05A5RenameVersion)) != 0 || len(result.Statements[1].EvidenceGaps) != 0 || len(result.Statements[2].EvidenceGaps) != 0 {
			t.Fatalf("tidb rename = %#v / %#v", result.Statements[1], result.Statements[2])
		}
	})
	t.Run("disabled and optional rules do not relax state", func(t *testing.T) {
		t.Parallel()
		for _, params := range []string{"", "      required: false\n"} {
			policy := t05PolicyPath(t, map[string]string{
				t05RuleCreateForbid:                      "",
				t05RuleAlterRequire:                      "",
				t05RuleModifyExists:                      "",
				t05RuleModifyCompat:                      "      required: true\n",
				"ddl.alter.rename_column.exists.require": "",
				t05A5RenameVersion:                       params,
			})
			if params == "" {
				policy = t05PolicyPath(t, map[string]string{
					t05RuleCreateForbid:                      "",
					t05RuleAlterRequire:                      "",
					t05RuleModifyExists:                      "",
					t05RuleModifyCompat:                      "      required: true\n",
					"ddl.alter.rename_column.exists.require": "",
				})
			}
			result := t05A5AuditWithPolicy(t, rename, spec.DialectMySQL, nil, "5.7.44", policy)
			if len(t05FindingsByRule(result, 1, t05A5RenameVersion)) != 0 || len(t05GapsByRule(result, 1, t05A5RenameVersion)) != 0 {
				t.Fatalf("disabled version rule still spoke: %#v %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
			}
			if len(result.Statements[2].EvidenceGaps) == 0 {
				t.Fatal("disabled version rule published c2")
			}
		}
	})
	t.Run("change does not inherit the version gap", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (c VARCHAR(10));\nALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10);\nALTER TABLE t MODIFY COLUMN c2 VARCHAR(20);"
		result := t05A5AuditVersion(t, sql, spec.DialectMySQL, nil, "")
		if len(t05GapsByRule(result, 1, t05A5RenameVersion)) != 0 || len(result.Statements[2].EvidenceGaps) != 0 {
			t.Fatalf("CHANGE version behavior = %#v %#v", result.Statements[1].EvidenceGaps, result.Statements[2].EvidenceGaps)
		}
	})
	t.Run("observed banner wins", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentVersionProvider{banner: "5.7.44"}
		result := t05A4Audit(t, rename, spec.DialectMySQL, provider, t05A5IdentityPolicy(t))
		findings := t05FindingsByRule(result, 1, t05A5RenameVersion)
		if len(findings) != 1 || findings[0].Metadata["target_version"] != "5.7.44" {
			t.Fatalf("observed finding = %#v", findings)
		}
		if len(result.Statements[2].EvidenceGaps) == 0 {
			t.Fatal("observed 5.7 published c2")
		}
	})
}

func t05A5AuditWithPolicy(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, version, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: policy, MetadataProvider: provider, TargetVersion: version,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

func TestBatchStateA5PolicyIsolationAndCancel(t *testing.T) {
	t.Parallel()
	t.Run("policy forbid still publishes", func(t *testing.T) {
		t.Parallel()
		policy := t05PolicyPath(t, map[string]string{
			t05RuleCreateForbid:              "",
			t05RuleAlterRequire:              "",
			"ddl.alter.change_column.forbid": "",
			t05RuleModifyExists:              "",
		})
		sql := "CREATE TABLE t (c VARCHAR(20));\nALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10);\nALTER TABLE t MODIFY COLUMN c2 VARCHAR(12);"
		result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, policy)
		if len(t05FindingsByRule(result, 1, "ddl.alter.change_column.forbid")) != 1 {
			t.Fatalf("forbid finding missing: %#v", result.Statements[1].Findings)
		}
		if len(result.Statements[2].EvidenceGaps) != 0 || len(t05FindingsByRule(result, 2, t05RuleModifyExists)) != 0 {
			t.Fatalf("policy forbid erased the successor: %#v", result.Statements[2])
		}
	})
	t.Run("requests stay isolated", func(t *testing.T) {
		t.Parallel()
		first := &t05AbsentProvider{}
		_ = t05A4Audit(t, t05A5ChangeFirstSQL, spec.DialectMySQL, first, t05A5IdentityPolicy(t))
		second := &t05AbsentProvider{}
		result := t05A4Audit(t, "ALTER TABLE t MODIFY COLUMN c2 VARCHAR(12);", spec.DialectMySQL, second, t05A5IdentityPolicy(t))
		if len(result.Statements[0].EvidenceGaps) == 0 {
			t.Fatal("a new request saw the previous c2")
		}
		if len(first.calls) != 1 || len(second.calls) != 1 {
			t.Fatalf("ledgers %#v %#v", first.calls, second.calls)
		}
	})
	t.Run("snapshots are deep copies", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, t05A5ChangeFirstSQL, spec.DialectMySQL, &t05AbsentProvider{})
		enriched[2].Metadata.TargetTable.Columns[1].Length = 999
		if got := t05A4Length(t, enriched[3], "c2"); got != 20 {
			t.Fatalf("mutated snapshot leaked length %d", got)
		}
	})
	t.Run("cancel publishes nothing", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "c")
		shape.Columns[0] = spec.Column{Name: "c", Type: "int"}
		state := newBatchState(spec.DialectMySQL, "golden", &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.4.10", Major: 8, Minor: 4, Patch: 10, ValidatedRange: true}
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load: %v", err)
		}
		key := state.keyFor("golden", spec.Table{Name: "t"})
		before := cloneTableSnapshot(state.entries[key].shape)
		statements := t05A2R1Extract(t, "ALTER TABLE t RENAME COLUMN c TO c2;")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := state.apply(ctx, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply = %v", err)
		}
		if !reflect.DeepEqual(state.entries[key].shape, before) {
			t.Fatal("canceled RENAME changed the loaded shape")
		}
	})
	t.Run("provider error keeps identity", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sentinel identity read failure")
		provider := &t05PresentProvider{errOn: map[string]error{"golden.t": wantErr}}
		_, err := AuditSQL(context.Background(), Request{
			SQL: "ALTER TABLE t CHANGE COLUMN c c2 INT;", Dialect: spec.DialectMySQL, Schema: "golden",
			ConfigPath: t05A5IdentityPolicy(t), MetadataProvider: provider,
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v", err)
		}
	})
}

func t05A5GapKeys(result report.Result, index int) []string {
	keys := make([]string, 0, len(result.Statements[index].EvidenceGaps))
	for _, gap := range result.Statements[index].EvidenceGaps {
		keys = append(keys, gap.RuleID+"|"+gap.ReasonCode+"|"+strings.Join(gap.RequiredFacts, ","))
	}
	return keys
}

func TestAuditSQLT05A5OfflineContractShapes(t *testing.T) {
	t.Parallel()
	change := "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
		"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX idx_c2 ON t(c2);"
	rename := strings.Replace(change, "CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL", "RENAME COLUMN c TO c2", 1)
	createGap := []string{"ddl.table.exists.create.forbid|unknown_table_state|target_table.existence"}
	unknownModify := []string{
		"ddl.alter.modify_column.compatibility.require|missing_source_column|source_column.definition",
		"ddl.table.exists.alter.require|unknown_table_state|target_table.existence",
		"ddl.alter.modify_column.exists.require|unknown_table_state|target_table.columns,target_table.existence",
	}
	indexGap := []string{"ddl.create_index.columns.exists.require|unknown_table_state|target_table.columns,target_table.existence"}

	t.Run("offline change publishes c2", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			result := t05A5AuditVersion(t, change, dialect, nil, "")
			if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("%s aggregate = %s/%s", dialect, result.Coverage.Status, result.Verdict)
			}
			if !reflect.DeepEqual(t05A5GapKeys(result, 0), createGap) {
				t.Fatalf("%s create gaps = %#v", dialect, t05A5GapKeys(result, 0))
			}
			for index := 1; index <= 3; index++ {
				if result.Statements[index].Coverage.Status != report.CoverageComplete || len(result.Statements[index].Findings) != 0 || len(result.Statements[index].EvidenceGaps) != 0 {
					t.Fatalf("%s statement %d = %s findings %#v gaps %#v", dialect, index, result.Statements[index].Coverage.Status, result.Statements[index].Findings, result.Statements[index].EvidenceGaps)
				}
			}
		}
	})
	t.Run("offline rename version gaps", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			version string
			reason  string
			facts   string
		}{
			{reason: "missing_target_version", facts: "target.version"},
			{version: "9.0.0", reason: "target_version_out_of_validated_range", facts: "target.version.validated_range"},
		}
		for _, tc := range cases {
			result := t05A5AuditVersion(t, rename, spec.DialectMySQL, nil, tc.version)
			if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("%s aggregate = %s/%s", tc.reason, result.Coverage.Status, result.Verdict)
			}
			wantRename := []string{t05A5RenameVersion + "|" + tc.reason + "|" + tc.facts}
			if !reflect.DeepEqual(t05A5GapKeys(result, 0), createGap) || !reflect.DeepEqual(t05A5GapKeys(result, 1), wantRename) || !reflect.DeepEqual(t05A5GapKeys(result, 2), unknownModify) || !reflect.DeepEqual(t05A5GapKeys(result, 3), indexGap) {
				t.Fatalf("%s gaps =\n0 %#v\n1 %#v\n2 %#v\n3 %#v", tc.reason, t05A5GapKeys(result, 0), t05A5GapKeys(result, 1), t05A5GapKeys(result, 2), t05A5GapKeys(result, 3))
			}
			if len(result.Statements[1].Findings) != 0 {
				t.Fatalf("%s invented a finding %#v", tc.reason, result.Statements[1].Findings)
			}
		}
	})
	t.Run("observed 5.7 rename is one blocker", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "c")
		provider := &t05BannerPresent{inner: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}, banner: "5.7.44"}
		result := t05A4Audit(t, "ALTER TABLE t RENAME COLUMN c TO c2;", spec.DialectMySQL, provider, t05A5IdentityPolicy(t))
		findings := t05FindingsByRule(result, 0, t05A5RenameVersion)
		if len(findings) != 1 || len(result.Statements[0].EvidenceGaps) != 0 || result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
			t.Fatalf("5.7 result = %s/%s findings %#v gaps %#v", result.Coverage.Status, result.Verdict, result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
		if findings[0].Metadata["target_version"] != "5.7.44" || findings[0].Metadata["product"] != "mysql" || findings[0].Metadata["minimum_supported_version"] != "8.0.3" || findings[0].Metadata["action"] != "rename_column" {
			t.Fatalf("metadata = %#v", findings[0].Metadata)
		}
	})
}
