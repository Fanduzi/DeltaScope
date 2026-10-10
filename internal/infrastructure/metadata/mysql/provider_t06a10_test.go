// Package mysql verifies the T06-A10 provider provenance boundary.
// input: constructed information_schema rows for a single table
// output: TABLES.AUTO_INCREMENT lands on snapshot.Options["auto_increment"]
// and COLUMNS.EXTRA lands on Column.AutoIncrement — both as catalog
// observations, never as reconstructed input declarations
// pos: infrastructure provider regression for issue #85 T06-A10
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06A10CatalogRows builds the constructed catalog rows for one table whose
// catalog allocator value is `allocator` (nil → SQL NULL) and whose id column
// carries extra='auto_increment'. This is a driver-level fixture — a
// controlled catalog row shape, not a live four-anchor run.
func t06A10CatalogRows(allocator driver.Value) map[string]testQueryResult {
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", allocator, "Dynamic", int64(0)}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows:    [][]driver.Value{{"id", "bigint(20)", nil, nil, "", nil, "NO", "auto_increment"}},
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows:    [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		},
	}
}

// TestProviderT06A10CatalogAllocatorValueWritesSnapshotOption proves the
// catalog-sourced allocator value lands on the same Options key the
// declaration layer uses — same key, different provenance. A NULL catalog
// value keeps the key absent rather than fabricating a declaration.
func TestProviderT06A10CatalogAllocatorValueWritesSnapshotOption(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("catalog_value_lands_on_snapshot_option", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t, t06A10CatalogRows(int64(42)))
		defer db.Close()

		snapshot, err := NewProvider(db).LoadTableSnapshot(ctx, spec.DialectMySQL, "golden", "t")
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}
		if !snapshot.Exists || snapshot.Options["auto_increment"] != "42" {
			t.Fatalf("snapshot options = %#v, want auto_increment=42 from catalog", snapshot.Options)
		}
		if len(snapshot.Columns) != 1 || !snapshot.Columns[0].AutoIncrement || snapshot.Columns[0].Name != "id" {
			t.Fatalf("snapshot columns = %#v, want id with auto_increment", snapshot.Columns)
		}
	})

	t.Run("null_catalog_value_keeps_key_absent", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t, t06A10CatalogRows(nil))
		defer db.Close()

		snapshot, err := NewProvider(db).LoadTableSnapshot(ctx, spec.DialectMySQL, "golden", "t")
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}
		if !snapshot.Exists {
			t.Fatal("snapshot must exist")
		}
		if got, present := snapshot.Options["auto_increment"]; present {
			t.Fatalf("NULL catalog value must not fabricate a key, got %q", got)
		}
	})
}
