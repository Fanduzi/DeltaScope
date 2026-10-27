// Package mysql tests the T10-R2 provider corrections: the STATISTICS
// member classification must distinguish a SQL NULL column_name (expression
// member), TiDB's literal 'NULL' text for expression members, and a real
// column literally named `NULL` — decided on verifiable catalog facts
// (expression text, the loaded column set), never on the literal alone;
// and the TiDB PKIsHandle loader must keep the handle answer unknown
// whenever the clustered catalog fact is missing or NULL.
package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t10r2Str(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
func t10r2Int(v int64) sql.NullInt64   { return sql.NullInt64{Int64: v, Valid: true} }
func t10r2NullStr() sql.NullString     { return sql.NullString{} }

// D: a real column named `NULL` must load as a plain column member —
// TiDB emits the same literal for expression members, so the literal alone
// can never classify the row; the loaded column set is the arbiter.
func TestT10R2LiteralNullColumnNameIsColumn(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "NULL", Type: "int", NotNull: true}}
	cases := []struct {
		name       string
		memberName string
	}{
		{name: "uppercase NULL", memberName: "NULL"},
		{name: "lowercase null", memberName: "null"},
		{name: "mixed NuLl", memberName: "NuLl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			indexes := make(map[string]*spec.Index)
			order := make([]string, 0)
			// The catalog carries the member with an ascending collation and
			// a SQL NULL expression column — the shape a real reserved-name
			// column produces.
			accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_null", 1, "BTREE",
				indexMemberRow{columnName: t10r2Str(tc.memberName), collation: t10r2Str("A")},
				nil, columns)
			index := indexes["i_null"]
			index.SetKeyParts(index.KeyParts)
			if len(index.Columns) != 1 || index.Columns[0] != tc.memberName {
				t.Fatalf("Columns = %v, want [%s]", index.Columns, tc.memberName)
			}
			if index.ExpressionCount != 0 || index.HasExpressionKeys {
				t.Fatalf("a proven column must not load as expression, got %+v", index)
			}
			part := index.KeyParts[0]
			if part.Kind != spec.IndexKeyPartColumn || part.Column != tc.memberName {
				t.Fatalf("KeyPart = %+v, want column member %q", part, tc.memberName)
			}
		})
	}
}

// D: literal 'NULL' with catalog expression text is the TiDB expression
// member shape — it must classify as expression even when a column named
// NULL exists (the catalog text is authoritative).
func TestT10R2LiteralNullWithExpressionTextIsExpression(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{
		{Name: "NULL", Type: "int", NotNull: true},
		{Name: "a", Type: "int", NotNull: true},
	}
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_expr", 1, "BTREE",
		indexMemberRow{
			columnName: t10r2Str("NULL"), collation: t10r2Str("A"),
			expression: t10r2Str("upper(`a`)"),
		}, nil, columns)
	index := indexes["i_expr"]
	index.SetKeyParts(index.KeyParts)
	if index.ExpressionCount != 1 || !index.HasExpressionKeys {
		t.Fatalf("expression text must classify the member as expression, got %+v", index)
	}
	if len(index.Columns) != 0 {
		t.Fatalf("expression member must not append the sentinel name, got %v", index.Columns)
	}
}

// D: literal 'NULL' without expression text and without a vouched column is
// unprovable — unmodeled, never a guessed column or expression.
func TestT10R2LiteralNullUnprovenIsUnmodeled(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "a", Type: "int", NotNull: true}}
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_x", 1, "BTREE",
		indexMemberRow{columnName: t10r2Str("NULL"), collation: t10r2Str("A")},
		nil, columns)
	index := indexes["i_x"]
	index.SetKeyParts(index.KeyParts)
	if index.UnmodeledKeyParts != 1 {
		t.Fatalf("unproven literal NULL must stay unmodeled, got %+v", index)
	}
	if len(index.Columns) != 0 || index.ExpressionCount != 0 {
		t.Fatalf("unmodeled member must not invent identity, got %+v", index)
	}
}

// D: a SQL NULL column_name without expression text is still an expression
// member (the catalog stores no column name for one) — marked, not
// fabricated.
func TestT10R2SQLNullColumnNameWithoutExpression(t *testing.T) {
	t.Parallel()
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_e", 1, "BTREE",
		indexMemberRow{columnName: t10r2NullStr(), collation: t10r2Str("A")},
		nil, nil)
	index := indexes["i_e"]
	index.SetKeyParts(index.KeyParts)
	if index.ExpressionCount != 1 || index.KeyParts[0].Kind != spec.IndexKeyPartExpression {
		t.Fatalf("SQL NULL column_name is an expression member, got %+v", index)
	}
}

// D: a mixed member list — plain column, literal-NULL column, and a SQL
// NULL expression member — keeps each identity and ordinal.
func TestT10R2MixedMembersKeepIdentities(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{
		{Name: "a", Type: "int", NotNull: true},
		{Name: "NULL", Type: "int", NotNull: true},
	}
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i", 1, "BTREE",
		indexMemberRow{columnName: t10r2Str("a"), collation: t10r2Str("A")}, nil, columns)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i", 1, "BTREE",
		indexMemberRow{columnName: t10r2Str("NULL"), collation: t10r2Str("A")}, nil, columns)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i", 1, "BTREE",
		indexMemberRow{columnName: t10r2NullStr(), collation: t10r2Str("A"), expression: t10r2Str("`a`+1")},
		nil, columns)
	index := indexes["i"]
	index.SetKeyParts(index.KeyParts)
	if len(index.KeyParts) != 3 {
		t.Fatalf("KeyParts = %d, want 3", len(index.KeyParts))
	}
	if index.KeyParts[0].Kind != spec.IndexKeyPartColumn || index.KeyParts[0].Column != "a" {
		t.Fatalf("member 1 = %+v", index.KeyParts[0])
	}
	if index.KeyParts[1].Kind != spec.IndexKeyPartColumn || index.KeyParts[1].Column != "NULL" {
		t.Fatalf("member 2 (real NULL column) = %+v", index.KeyParts[1])
	}
	if index.KeyParts[2].Kind != spec.IndexKeyPartExpression || index.KeyParts[2].Expression == nil {
		t.Fatalf("member 3 (expression) = %+v", index.KeyParts[2])
	}
	if index.KeyParts[2].Ordinal != 3 {
		t.Fatalf("expression ordinal = %d, want 3", index.KeyParts[2].Ordinal)
	}
}

// B4: the clustered catalog row missing or NULL must leave the handle
// answer unknown — a boolean false was a fabricated "real object exists".
func TestT10R2TiDBHandleUnknownOnMissingClusteredRow(t *testing.T) {
	t.Parallel()
	base := map[string]testQueryResult{
		"select engine, table_collation": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
		},
		"information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows: [][]driver.Value{
				{"id", "int(11)", nil, nil, "", nil, "NO", "", nil},
			},
		},
		"column_name = 'IS_VISIBLE'": {
			columns: []string{"count"},
			rows:    [][]driver.Value{{1}},
		},
		"column_name = 'EXPRESSION'": {
			columns: []string{"count"},
			rows:    [][]driver.Value{{1}},
		},
		"information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation", "is_visible", "expression"},
			rows: [][]driver.Value{
				{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
			},
		},
		"information_schema.partitions": {
			columns: []string{"partition_method", "partition_name", "subpartition_name"},
		},
		"table_constraints": {
			columns: []string{"count"},
			rows:    [][]driver.Value{{1}},
		},
	}

	// The clustered row absent → unknown, never a guessed real object.
	missing := map[string]testQueryResult{}
	for k, v := range base {
		missing[k] = v
	}
	db := openTestDB(t, missing)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("missing TIDB_INDEXES row must leave the handle unknown, got handle=%v unknown=%v",
			snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown)
	}

	// A NULL clustered value → unknown.
	nullRow := map[string]testQueryResult{}
	for k, v := range base {
		nullRow[k] = v
	}
	nullRow["information_schema.tidb_indexes"] = testQueryResult{
		columns: []string{"clustered"},
		rows:    [][]driver.Value{{nil}},
	}
	db = openTestDB(t, nullRow)
	defer db.Close()
	snapshot, err = NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("NULL clustered must leave the handle unknown, got handle=%v unknown=%v",
			snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown)
	}

	// clustered=yes + single int column → the handle (determined true).
	handleYes := map[string]testQueryResult{}
	for k, v := range base {
		handleYes[k] = v
	}
	handleYes["information_schema.tidb_indexes"] = testQueryResult{
		columns: []string{"clustered"},
		rows:    [][]driver.Value{{"YES"}},
	}
	db = openTestDB(t, handleYes)
	defer db.Close()
	snapshot, err = NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.PrimaryKeyIsHandle || snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("clustered single-int PK must resolve to handle, got handle=%v unknown=%v",
			snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown)
	}

	// clustered=no → a real PRIMARY object (determined false).
	handleNo := map[string]testQueryResult{}
	for k, v := range base {
		handleNo[k] = v
	}
	handleNo["information_schema.tidb_indexes"] = testQueryResult{
		columns: []string{"clustered"},
		rows:    [][]driver.Value{{"NO"}},
	}
	db = openTestDB(t, handleNo)
	defer db.Close()
	snapshot, err = NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKeyIsHandle || snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("clustered=no must resolve to a real object, got handle=%v unknown=%v",
			snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown)
	}
}
