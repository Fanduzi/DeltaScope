// Package mysql verifies the T13-R2 partition-catalog completeness
// contract on the provider seam: a catalog answer that carries
// subpartition members is an incomplete top-level projection — the
// member directory stays unobserved rather than claiming the partial
// top-level list is complete.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// A subpartitioned table's catalog rows repeat PARTITION_NAME per leaf and
// carry SUBPARTITION_NAME — the member projection must stay nil so no rule
// can claim a leaf or top-level absence was proven.
func TestT13R2SubpartitionedCatalogKeepsNamesUnobserved(t *testing.T) {
	t.Parallel()
	db := openTestDB(t, map[string]testQueryResult{
		"select engine, table_collation": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
		},
		"information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    [][]driver.Value{{"id", "int(11)", nil, nil, "", nil, "NO", "", nil}},
		},
		"column_name = 'IS_VISIBLE'": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
		"column_name = 'EXPRESSION'": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
		"information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation", "is_visible", "expression"},
		},
		"information_schema.partitions": {
			columns: []string{"partition_method", "partition_name", "subpartition_name"},
			rows: [][]driver.Value{
				{"RANGE", "p0", "p0sp0"}, {"RANGE", "p0", "p0sp1"},
				{"RANGE", "p1", "p1sp0"}, {"RANGE", "p1", "p1sp1"},
			},
		},
		"table_constraints": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
	})
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.Partitioned == nil || !*snapshot.Partitioned {
		t.Fatalf("subpartitioned table is still partitioned, got %+v", snapshot.Partitioned)
	}
	if snapshot.PartitionNames != nil {
		t.Fatalf("subpartitioned projection is incomplete — names must stay unobserved, got %+v",
			snapshot.PartitionNames)
	}
}

// A plain partitioned catalog (leaf-column all NULL) keeps projecting the
// complete member list — the completeness marker never over-fires.
func TestT13R2PlainCatalogStillComplete(t *testing.T) {
	t.Parallel()
	db := openTestDB(t, map[string]testQueryResult{
		"select engine, table_collation": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
		},
		"information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    [][]driver.Value{{"id", "int(11)", nil, nil, "", nil, "NO", "", nil}},
		},
		"column_name = 'IS_VISIBLE'": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
		"column_name = 'EXPRESSION'": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
		"information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation", "is_visible", "expression"},
		},
		"information_schema.partitions": {
			columns: []string{"partition_method", "partition_name", "subpartition_name"},
			rows:    [][]driver.Value{{"RANGE", "p0", nil}, {"RANGE", "p1", nil}},
		},
		"table_constraints": {columns: []string{"count"}, rows: [][]driver.Value{{1}}},
	})
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if len(snapshot.PartitionNames) != 2 || snapshot.PartitionNames[0] != "p0" {
		t.Fatalf("plain catalog keeps complete members, got %+v", snapshot.PartitionNames)
	}
}
