// Package audit verifies the T05-A4 MODIFY state matrix beyond the first path.
// input: ordered MySQL/TiDB batches and loaded snapshots covering widening,
// continuation, omitted attributes, primary-key nullability, charset
// defaults, loaded dependents, and the conservative non-template boundaries
// output: public findings/gaps plus direct pre-state facts for each frozen cell
// pos: T05-A4 proof groups for conditional MODIFY state supply
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLT05A4ModifyWideAndContinuations(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect)+"/wide", func(t *testing.T) {
			t.Parallel()
			sql := strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1)
			provider := &t05AbsentProvider{}
			result := t05A4Audit(t, sql, dialect, provider, t05A4FourRulePolicy(t))
			for i, statement := range result.Statements {
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %+v, want complete with none", i, statement)
				}
			}
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider read ledger = %#v, want [golden.t]", provider.calls)
			}
		})
		t.Run(string(dialect)+"/narrow-then-18", func(t *testing.T) {
			t.Parallel()
			sql := t05A4NarrowSQL + "\nALTER TABLE t MODIFY COLUMN c VARCHAR(18) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
			result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A4FourRulePolicy(t))
			if len(result.Statements) != 4 {
				t.Fatalf("statements = %d, want 4", len(result.Statements))
			}
			t05A4Shrink(t, result, 2, 20, 15)
			for _, index := range []int{0, 1, 3} {
				statement := result.Statements[index]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 || statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d = %+v, want complete with none", index, statement)
				}
			}
			if result.Verdict != report.VerdictReject || result.Summary.Blockers != 1 {
				t.Fatalf("aggregate = %s blockers %d, want reject with one blocker", result.Verdict, result.Summary.Blockers)
			}
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			if got := t05A4Length(t, enriched[3], "c"); got != 15 {
				t.Fatalf("statement 4 pre-state c.Length = %d, want 15", got)
			}
		})
		t.Run(string(dialect)+"/wide-then-25", func(t *testing.T) {
			t.Parallel()
			sql := strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1) +
				"\nALTER TABLE t MODIFY COLUMN c VARCHAR(25) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
			result := t05A4Audit(t, sql, dialect, &t05AbsentProvider{}, t05A4FourRulePolicy(t))
			for _, index := range []int{0, 1, 2} {
				statement := result.Statements[index]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = findings %#v gaps %#v, want none", index, statement.Findings, statement.EvidenceGaps)
				}
			}
			t05A4Shrink(t, result, 3, 30, 25)
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			if got := t05A4Length(t, enriched[3], "c"); got != 30 {
				t.Fatalf("statement 4 pre-state c.Length = %d, want 30", got)
			}
		})
	}
}

func TestAuditSQLT05A4ModifyNoProviderKeepsCreateGap(t *testing.T) {
	t.Parallel()
	narrow := t05A4Audit(t, t05A4NarrowSQL, spec.DialectMySQL, nil, t05A4FourRulePolicy(t))
	if narrow.Verdict != report.VerdictReject || narrow.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("narrow aggregate = %s/%s, want reject/unverified", narrow.Verdict, narrow.Coverage.Status)
	}
	if len(narrow.Statements[0].EvidenceGaps) == 0 || len(narrow.Statements[0].Findings) != 0 {
		t.Fatalf("create gap must stay on statement 0, got %+v", narrow.Statements[0])
	}
	t05A4Shrink(t, narrow, 2, 20, 15)
	wideSQL := strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1)
	wide := t05A4Audit(t, wideSQL, spec.DialectTiDB, nil, t05A4FourRulePolicy(t))
	if wide.Verdict != report.VerdictReview || wide.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("wide aggregate = %s/%s, want review/unverified", wide.Verdict, wide.Coverage.Status)
	}
	if len(wide.Statements[0].EvidenceGaps) == 0 {
		t.Fatal("create gap must stay on the wide no-provider path")
	}
	for _, index := range []int{1, 2} {
		statement := wide.Statements[index]
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 || statement.Coverage.Status != report.CoverageComplete {
			t.Fatalf("statement %d = %+v, want complete derived state", index, statement)
		}
	}
}

func TestBatchStateA4PartialDefinitionPublishesNext(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		column spec.Column
		facts  []string
	}{
		{name: "empty type", column: spec.Column{Name: "c"}, facts: []string{"source_column.length", "source_column.type"}},
		{name: "unknown varchar length", column: spec.Column{Name: "c", Type: "varchar"}, facts: []string{"source_column.length"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := presentTable("golden", "t", "id")
			shape.Columns = append(shape.Columns, tc.column)
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}
			sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);\nALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
			result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4FourRulePolicy(t))
			gaps := t05GapsByRule(result, 0, t05RuleModifyCompat)
			if len(gaps) != 1 || gaps[0].ReasonCode != "incomplete_source_column" || !reflect.DeepEqual(gaps[0].RequiredFacts, tc.facts) {
				t.Fatalf("statement 0 gaps = %#v, want incomplete_source_column %v", gaps, tc.facts)
			}
			t05A4Shrink(t, result, 1, 20, 15)
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider ledger = %#v, want one read", provider.calls)
			}
		})
	}
}

func TestBatchStateA4AbsentAndUnknownStayDistinct(t *testing.T) {
	t.Parallel()
	t.Run("confirmed absent column", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}}
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		result := t05A4Audit(t, sql, spec.DialectMySQL, provider, t05A4FourRulePolicy(t))
		gaps := t05GapsByRule(result, 0, t05RuleModifyCompat)
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_source_column" {
			t.Fatalf("absent column must keep the source gap, got %#v", gaps)
		}
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}})
		if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable != nil {
			t.Fatalf("next pre-state must be unknown, got %+v", enriched[1].Metadata)
		}
	})
	t.Run("known absent table", func(t *testing.T) {
		t.Parallel()
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable != nil {
			t.Fatalf("absent premise must become unknown, got %+v", enriched[1].Metadata)
		}
	})
	t.Run("withheld column set", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t")
		shape.Columns = nil
		shape.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN id VARCHAR(20);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, provider)
		next := enriched[1].Metadata.TargetTable
		if next == nil || !next.Exists || next.Columns != nil || next.PrimaryKey == nil || len(next.PrimaryKey.Columns) != 1 {
			t.Fatalf("withheld columns must stay withheld and keep the primary key, got %+v", next)
		}
		if next.FindColumn("c") != nil {
			t.Fatal("withheld column set must not become a one-column table")
		}
	})
	t.Run("unknown provider stays unknown", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, provider)
		if enriched[0].Metadata.TargetTable != nil || enriched[1].Metadata.TargetTable != nil {
			t.Fatal("unknown must not become present")
		}
		if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
			t.Fatalf("unknown entry must not be reread, ledger = %#v", provider.calls)
		}
	})
}

func TestBatchStateA4OmittedAttributesAndCharset(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (id INT PRIMARY KEY, c INT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'old');\nALTER TABLE t MODIFY COLUMN c INT;\nALTER TABLE t MODIFY COLUMN c INT NOT NULL;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A4FourRulePolicy(t))
	if len(t05FindingsByRule(result, 1, t05RuleModifyCompat)) != 1 || len(t05FindingsByRule(result, 2, t05RuleModifyCompat)) != 1 {
		t.Fatalf("want one signedness blocker then one nullability blocker, got %#v / %#v", result.Statements[1].Findings, result.Statements[2].Findings)
	}
	if result.Statements[1].Findings[0].Metadata["source_unsigned"] != true || result.Statements[2].Findings[0].Metadata["source_not_null"] != false {
		t.Fatalf("finding metadata = %#v / %#v", result.Statements[1].Findings[0].Metadata, result.Statements[2].Findings[0].Metadata)
	}
	enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
	first := enriched[1].Metadata.TargetTable.FindColumn("c")
	if first == nil || !first.Unsigned || !first.NotNull || !first.HasDefault || first.Comment == "" {
		t.Fatalf("statement 2 pre-state must still carry the old attributes, got %+v", first)
	}
	second := enriched[2].Metadata.TargetTable.FindColumn("c")
	if second == nil || second.Unsigned || second.NotNull || second.HasDefault || second.DefaultValue != "" || second.Comment != "" {
		t.Fatalf("omitted attributes must be cleared as a group, got %+v", second)
	}

	t.Run("both omitted use table defaults", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET latin1) CHARSET=utf8mb4;\nALTER TABLE t MODIFY COLUMN c VARCHAR(20);\nALTER TABLE t MODIFY COLUMN c VARCHAR(30);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		column := enriched[2].Metadata.TargetTable.FindColumn("c")
		if column == nil || column.Charset != "utf8mb4" || column.Collation != "" || column.Length != 20 {
			t.Fatalf("both-omitted charset = %+v, want table charset and unknown collation", column)
		}
	})
	t.Run("charset only does not copy table collation", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "id")
		shape.Columns = append(shape.Columns, spec.Column{Name: "c", Type: "varchar", Length: 10, Charset: "latin1", Collation: "latin1_bin"})
		shape.Options = map[string]string{"charset": "utf8mb4", "collate": "utf8mb4_bin"}
		parsed, parseErr := parseSQL(context.Background(), "ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4; ALTER TABLE t MODIFY COLUMN c VARCHAR(30);", spec.DialectMySQL)
		if parseErr != nil {
			t.Fatalf("parse: %v", parseErr)
		}
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		statements[0].DDL.Alter[0].Column.Definition.Collation = ""
		enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL, &MetadataRequest{Schema: "golden", Provider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}}, statements, nil)
		if err != nil {
			t.Fatalf("enrich: %v", err)
		}
		column := enriched[1].Metadata.TargetTable.FindColumn("c")
		if column == nil || column.Charset != "utf8mb4" || column.Collation != "" {
			t.Fatalf("one-sided charset = %+v, want charset only", column)
		}
	})
	t.Run("contradictory table collations stay unknown", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "id")
		shape.Columns = append(shape.Columns, spec.Column{Name: "c", Type: "varchar", Length: 10, Charset: "latin1", Collation: "latin1_bin"})
		shape.Options = map[string]string{"charset": "utf8mb4", "collate": "utf8mb4_bin", "collation": "latin1_swedish_ci"}
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(30);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		column := enriched[1].Metadata.TargetTable.FindColumn("c")
		if column == nil || column.Charset != "utf8mb4" || column.Collation != "" || column.Length != 20 {
			t.Fatalf("contradictory defaults = %+v, want charset and unknown collation", column)
		}
	})
}

func TestBatchStateA4PrimaryKeyNullability(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (id INT PRIMARY KEY, c INT);\nALTER TABLE t MODIFY COLUMN id BIGINT;\nALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL;"
	result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, t05A4FourRulePolicy(t))
	for i, statement := range result.Statements {
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d must not invent a nullability blocker, got %+v", i, statement)
		}
	}
	enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
	for _, index := range []int{1, 2} {
		column := enriched[index].Metadata.TargetTable.FindColumn("id")
		if column == nil || !column.NotNull {
			t.Fatalf("statement %d pre-state id = %+v, want NotNull", index+1, column)
		}
	}
	if enriched[2].Metadata.TargetTable.FindColumn("c") == nil || enriched[2].Metadata.TargetTable.PrimaryKey == nil {
		t.Fatal("unmodified primary key and ordinary column must stay")
	}

	t.Run("constraint payload", func(t *testing.T) {
		t.Parallel()
		shape := presentTable("golden", "t", "id", "c")
		shape.PrimaryKey = nil
		shape.Constraints = []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"id"}}}
		shape.Columns[0].NotNull = false
		sql := "ALTER TABLE t MODIFY COLUMN id BIGINT; ALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		column := enriched[1].Metadata.TargetTable.FindColumn("id")
		if column == nil || !column.NotNull || modifyBaseType(*column) != "bigint" {
			t.Fatalf("constraint PK post-state = %+v, want bigint NotNull", column)
		}
	})
	t.Run("explicit null invalidates", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (id INT PRIMARY KEY, c INT);\nALTER TABLE t MODIFY COLUMN id BIGINT NULL;\nALTER TABLE t MODIFY COLUMN id BIGINT;"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata == nil || enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("explicit NULL on a primary key must not publish a nullable key, got %+v", enriched[2].Metadata)
		}
	})
}

func TestBatchStateA4OrdinaryIndexAndLoadedDependents(t *testing.T) {
	t.Parallel()
	rows := int64(7)
	shape := presentTable("golden", "t", "id", "c")
	shape.Columns[1] = spec.Column{Name: "c", Type: "varchar", Length: 10}
	shape.Columns[0] = spec.Column{Name: "id", Type: "int", NotNull: true}
	shape.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &rows}
	shape.Indexes = []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, Cardinality: &rows}}
	shape.Options = map[string]string{"table_rows": "9", "data_length": "100", "auto_increment": "4"}
	other := presentTable("other", "t", "id", "c")
	other.Columns[1] = spec.Column{Name: "c", Type: "varchar", Length: 10}
	childHit := presentTable("aux", "child", "id")
	childHit.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk_c", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t", ReferencedColumns: []string{"c"}}}
	childMiss := presentTable("aux", "other", "id")
	childMiss.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk_id", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t", ReferencedColumns: []string{"id"}}}
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		"golden.t": shape, "other.t": other, "aux.child": childHit, "aux.other": childMiss,
	}}
	sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE other.t ADD COLUMN extra INT; ALTER TABLE t MODIFY COLUMN c VARCHAR(30); CREATE INDEX ix ON aux.other(id);"
	enriched := enrichA3(t, sql, spec.DialectMySQL, provider)
	if enriched[1].Metadata.TargetTable.FindColumn("c").Length != 10 {
		t.Fatalf("other schema must keep its own column, got %+v", enriched[1].Metadata.TargetTable.Columns)
	}
	next := enriched[2].Metadata.TargetTable
	if next.FindColumn("id").Type != "int" || next.FindColumn("c").Length != 20 || modifyBaseType(*next.FindColumn("c")) != "varchar" {
		t.Fatalf("column order and untouched id = %+v", next.Columns)
	}
	if next.Indexes[0].Name != "idx_c" || next.Indexes[0].Cardinality != nil || next.PrimaryKey.Cardinality != nil {
		t.Fatalf("index identity/cardinality = %+v / %+v", next.Indexes, next.PrimaryKey)
	}
	if next.Options["table_rows"] != "9" || next.Options["data_length"] != "" || next.Options["auto_increment"] != "" {
		t.Fatalf("stats = %#v, want table_rows kept and storage counters cleared", next.Options)
	}
	if enriched[3].Metadata.TargetTable == nil || !enriched[3].Metadata.TargetTable.Exists || enriched[3].Metadata.TargetTable.FindColumn("id") == nil {
		t.Fatalf("unrelated child must stay present, got %+v", enriched[3].Metadata)
	}
	childEnriched := enrichA3(t, "CREATE INDEX warm ON aux.child(id); ALTER TABLE t MODIFY COLUMN c VARCHAR(20); CREATE INDEX after ON aux.child(id);", spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape, "aux.child": childHit}})
	if childEnriched[2].Metadata == nil || childEnriched[2].Metadata.TargetTable != nil {
		t.Fatalf("referencing child must be unknown, got %+v", childEnriched[2].Metadata)
	}
	if !reflect.DeepEqual(shape.Columns[1], spec.Column{Name: "c", Type: "varchar", Length: 10}) || shape.Options["data_length"] != "100" {
		t.Fatal("provider snapshot must stay unchanged")
	}
}

func TestBatchStateA4UnsafeDependentsInvalidate(t *testing.T) {
	t.Parallel()
	base := func() *spec.TableSnapshot {
		shape := presentTable("golden", "t", "id", "c")
		shape.Columns[1] = spec.Column{Name: "c", Type: "varchar", Length: 10}
		return shape
	}
	cases := []struct {
		name   string
		mutate func(shape *spec.TableSnapshot)
	}{
		{name: "check", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "check", Name: "chk"}}
		}},
		{name: "prefix index", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, PrefixParts: 1}}
		}},
		{name: "expression index", mutate: func(shape *spec.TableSnapshot) {
			shape.Indexes = []spec.Index{{Name: "idx_expr", Kind: spec.IndexKindSecondary, Columns: []string{"id"}, HasExpressionKeys: true, ExpressionCount: 1}}
		}},
		{name: "generated column", mutate: func(shape *spec.TableSnapshot) {
			shape.Columns[0].GeneratedWhen = "virtual"
		}},
		{name: "foreign key on column", mutate: func(shape *spec.TableSnapshot) {
			shape.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk_c", Columns: []string{"c"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}}}
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := base()
			tc.mutate(shape)
			sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
			enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
			if enriched[1].Metadata.TargetTable != nil {
				t.Fatalf("%s must invalidate the target, got %+v", tc.name, enriched[1].Metadata.TargetTable)
			}
		})
	}
	t.Run("unrelated prefix stays", func(t *testing.T) {
		t.Parallel()
		shape := base()
		shape.Indexes = []spec.Index{{Name: "idx_id", Kind: spec.IndexKindSecondary, Columns: []string{"id"}, PrefixParts: 1}}
		sql := "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}})
		if got := t05A4Length(t, enriched[1], "c"); got != 20 {
			t.Fatalf("prefix on another column must keep the replacement, length = %d", got)
		}
	})
}

func TestBatchStateA4NonTemplateStaysConservative(t *testing.T) {
	t.Parallel()
	cases := []string{
		"CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10)); ALTER TABLE t MODIFY COLUMN c VARCHAR(20), ADD COLUMN d INT; ALTER TABLE t MODIFY COLUMN c VARCHAR(15);",
		"CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10)); ALTER TABLE t MODIFY COLUMN c VARCHAR(20) FIRST; ALTER TABLE t MODIFY COLUMN c VARCHAR(15);",
		"CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10)); ALTER TABLE t MODIFY COLUMN c INT; ALTER TABLE t MODIFY COLUMN c BIGINT;",
		"CREATE TABLE t (id INT PRIMARY KEY, c INT); ALTER TABLE t MODIFY COLUMN c INT AUTO_INCREMENT; ALTER TABLE t MODIFY COLUMN c BIGINT;",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
			if enriched[2].Metadata.TargetTable != nil {
				t.Fatalf("non-template must not publish a later definition, got %+v", enriched[2].Metadata.TargetTable.Columns)
			}
		})
	}
	t.Run("tidb if exists", func(t *testing.T) {
		t.Parallel()
		sql := "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10)); ALTER TABLE t MODIFY COLUMN IF EXISTS c VARCHAR(20); ALTER TABLE t MODIFY COLUMN c VARCHAR(15);"
		parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectTiDB)
		if parseErr != nil || len(parsed.Statements) != 3 {
			t.Fatalf("TiDB conditional MODIFY must parse, err=%v statements=%d", parseErr, len(parsed.Statements))
		}
		enriched := enrichA3(t, sql, spec.DialectTiDB, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("TiDB IF EXISTS must not publish VARCHAR(20), got %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("prior contamination", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "SELECT * FROM WHERE;\nALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A4FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, errParserUnsupported) {
			t.Fatalf("expected parser error, got %v", err)
		}
		if len(provider.calls) != 0 {
			t.Fatalf("contaminated batch must not read the provider, got %#v", provider.calls)
		}
		if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
			t.Fatalf("contaminated MODIFY must not publish findings, got %+v", result.Statements)
		}
	})
}

func TestBatchStateA4PolicyDoesNotBlockTransfer(t *testing.T) {
	t.Parallel()
	sql := t05A4NarrowSQL + "\nALTER TABLE t MODIFY COLUMN c VARCHAR(18) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
	disabled := t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid: "",
		t05RuleAlterRequire: "",
		t05RuleModifyExists: "",
	})
	result := t05A4Audit(t, sql, spec.DialectMySQL, &t05AbsentProvider{}, disabled)
	for i, statement := range result.Statements {
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("disabled compatibility must stay silent, statement %d = %+v", i, statement)
		}
	}
	enriched := enrichA3(t, sql, spec.DialectMySQL, &t05AbsentProvider{})
	if got := t05A4Length(t, enriched[3], "c"); got != 15 {
		t.Fatalf("disabled policy must still publish 15, got %d", got)
	}
	optional := t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid: "",
		t05RuleAlterRequire: "",
		t05RuleModifyExists: "",
		t05RuleModifyCompat: "      required: false\n",
	})
	unknown := t05A4Audit(t, "ALTER TABLE t MODIFY COLUMN c VARCHAR(15);", spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}, optional)
	if len(t05GapsByRule(unknown, 0, t05RuleModifyCompat)) != 0 {
		t.Fatalf("required=false must not emit a compatibility gap, got %#v", unknown.Statements[0].EvidenceGaps)
	}
}

func TestBatchStateA4ProviderIsolationCopyAndCancel(t *testing.T) {
	t.Parallel()
	t.Run("requests are isolated", func(t *testing.T) {
		t.Parallel()
		first := &t05AbsentProvider{}
		_ = t05A4Audit(t, t05A4NarrowSQL, spec.DialectMySQL, first, t05A4FourRulePolicy(t))
		second := &t05AbsentProvider{}
		result := t05A4Audit(t, "ALTER TABLE t MODIFY COLUMN c VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;", spec.DialectMySQL, second, t05A4FourRulePolicy(t))
		if len(t05FindingsByRule(result, 0, t05RuleModifyCompat)) != 0 {
			t.Fatalf("a new request must not see the previous VARCHAR(20), got %#v", result.Statements[0].Findings)
		}
		if len(first.calls) != 1 || len(second.calls) != 1 {
			t.Fatalf("ledgers = %#v / %#v, want one read each", first.calls, second.calls)
		}
	})
	t.Run("pre-state copies are independent", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, t05A4NarrowSQL, spec.DialectMySQL, &t05AbsentProvider{})
		enriched[1].Metadata.TargetTable.Columns[1].Length = 999
		if got := t05A4Length(t, enriched[2], "c"); got != 20 {
			t.Fatalf("mutating an earlier snapshot changed the next pre-state to %d", got)
		}
	})
	t.Run("canceled apply publishes nothing", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id", "c")}}
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load: %v", err)
		}
		key := state.keyFor("golden", spec.Table{Name: "t"})
		before := cloneTableSnapshot(state.entries[key].shape)
		statements := t05A2R1Extract(t, "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		defer cancel()
		err := state.apply(ctx, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply = %v, want context.Canceled", err)
		}
		if !reflect.DeepEqual(state.entries[key].shape, before) || state.entries[key].state != tablePresent {
			t.Fatal("canceled MODIFY must leave the loaded shape unchanged")
		}
	})
	t.Run("provider error keeps identity", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sentinel modify read failure")
		provider := &t05PresentProvider{errOn: map[string]error{"golden.t": wantErr}}
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A4FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("provider error = %v, want the sentinel", err)
		}
	})
}
