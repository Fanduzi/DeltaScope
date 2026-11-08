// Package mysqlmeta tests the TiDB sharding/layout catalog read that feeds
// the T23 ALTER verdicts.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// provider_t23_test.go pins the T23 reader-chain contract:
// TIDB_ROW_ID_SHARDING_INFO and the SHOW CREATE feature comments are the
// only authoritative sources for the AUTO_RANDOM layout and the
// auto_id_cache catalog value. A clean read publishes the proven facts, a
// nil cell or an unrecognized shape leaves facts unobserved or fails the
// read outright — a parser/spec declaration never impersonates a catalog
// observation.
func TestT23ShardingInfoReaderMatrix(t *testing.T) {
	t.Parallel()
	base := func() map[string]testQueryResult {
		return map[string]testQueryResult{
			"select engine, table_collation": {
				columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows", "table_type"},
				rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil, "BASE TABLE"}},
			},
			"information_schema.columns": {
				columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
				rows:    [][]driver.Value{{"id", "bigint(20)", nil, nil, "", nil, "NO", "", nil}},
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
				rows:    [][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}},
			},
			"information_schema.partitions": {
				columns: []string{"partition_method", "partition_name", "subpartition_name"},
			},
			"table_constraints": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"information_schema.tidb_indexes": {
				columns: []string{"clustered"},
				rows:    [][]driver.Value{{"YES"}},
			},
		}
	}
	t.Run("pk_auto_random_bits marks the pk column", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testShardingInfoFragment, testQueryResult{
			columns: []string{"tidb_row_id_sharding_info"},
			rows:    [][]driver.Value{{"PK_AUTO_RANDOM_BITS=7, RANGE BITS=48"}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		column := snapshot.FindColumn("id")
		if column == nil || !column.AutoRandom {
			t.Fatalf("auto_random must land on the pk column, got %+v", snapshot.Columns)
		}
		if column.AutoRandomShardBits == nil || *column.AutoRandomShardBits != 7 ||
			column.AutoRandomRangeBits == nil || *column.AutoRandomRangeBits != 48 {
			t.Fatalf("stored bits = %+v/%+v, want 7/48", column.AutoRandomShardBits, column.AutoRandomRangeBits)
		}
	})
	t.Run("omitted range bits mean the stored default 64", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testShardingInfoFragment, testQueryResult{
			columns: []string{"tidb_row_id_sharding_info"},
			rows:    [][]driver.Value{{"PK_AUTO_RANDOM_BITS=5"}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		column := snapshot.FindColumn("id")
		if column == nil || column.AutoRandomRangeBits == nil || *column.AutoRandomRangeBits != 64 {
			t.Fatalf("range bits = %+v, want default 64", column)
		}
	})
	t.Run("shard_bits publishes the stored option value", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testShardingInfoFragment, testQueryResult{
			columns: []string{"tidb_row_id_sharding_info"},
			rows:    [][]driver.Value{{"SHARD_BITS=4"}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if got := snapshot.Options["shard_row_id_bits"]; got != "4" {
			t.Fatalf("shard_row_id_bits = %q, want 4", got)
		}
		if column := snapshot.FindColumn("id"); column != nil && column.AutoRandom {
			t.Fatal("non-sharded pk column must not gain auto_random")
		}
	})
	t.Run("not_sharded publishes the proven zero", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t, base())
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if got := snapshot.Options["shard_row_id_bits"]; got != "0" {
			t.Fatalf("shard_row_id_bits = %q, want proven 0", got)
		}
	})
	t.Run("unrecognized sharding text fails the read", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testShardingInfoFragment, testQueryResult{
			columns: []string{"tidb_row_id_sharding_info"},
			rows:    [][]driver.Value{{"FUTURE_SHAPE=9"}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		if _, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t"); err == nil {
			t.Fatal("unrecognized sharding shape must propagate as a failed read")
		}
	})
	t.Run("nil sharding cell leaves the facts unobserved", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testShardingInfoFragment, testQueryResult{
			columns: []string{"tidb_row_id_sharding_info"},
			rows:    [][]driver.Value{{nil}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("nil sharding cell must not fail the read: %v", err)
		}
		if _, ok := snapshot.Options["shard_row_id_bits"]; ok {
			t.Fatal("nil sharding must not fabricate an option value")
		}
		if column := snapshot.FindColumn("id"); column != nil && column.AutoRandom {
			t.Fatal("nil sharding must not fabricate auto_random")
		}
	})
}

// The auto_id_cache catalog value comes from the SHOW CREATE feature
// comment: a stored value publishes the proven number, an absent comment
// is the upstream-proven zero.
func TestT23AutoIDCacheCatalogValue(t *testing.T) {
	t.Parallel()
	base := func() map[string]testQueryResult {
		return map[string]testQueryResult{
			"select engine, table_collation": {
				columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows", "table_type"},
				rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil, "BASE TABLE"}},
			},
			"information_schema.columns": {
				columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
				rows:    [][]driver.Value{{"id", "bigint(20)", nil, nil, "", nil, "NO", "", nil}},
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
				rows:    [][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}},
			},
			"information_schema.partitions": {
				columns: []string{"partition_method", "partition_name", "subpartition_name"},
			},
			"table_constraints": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"information_schema.tidb_indexes": {
				columns: []string{"clustered"},
				rows:    [][]driver.Value{{"YES"}},
			},
		}
	}
	t.Run("stored auto_id_cache publishes the catalog value", func(t *testing.T) {
		t.Parallel()
		queries := withQuery(base(), testTTLFragment, testQueryResult{
			columns: []string{"Table", "Create Table"},
			rows: [][]driver.Value{{"t",
				"CREATE TABLE `t` (\n  `id` bigint(20) NOT NULL\n) /*T![auto_id_cache] AUTO_ID_CACHE=5 */"}},
		})
		db := openTestDB(t, queries)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if got := snapshot.Options["auto_id_cache"]; got != "5" {
			t.Fatalf("auto_id_cache = %q, want 5", got)
		}
	})
	t.Run("absent comment is the proven zero", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t, base())
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if got := snapshot.Options["auto_id_cache"]; got != "0" {
			t.Fatalf("auto_id_cache = %q, want proven 0", got)
		}
	})
	t.Run("mysql never issues the tidb probes", func(t *testing.T) {
		t.Parallel()
		// The MySQL path must not touch tidb_row_id_sharding_info or the
		// TiDB SHOW CREATE surface — the default stubs would answer, so a
		// wrongly-scoped query lands the TiDB facts on a MySQL snapshot.
		db := openTestDB(t, base())
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if _, ok := snapshot.Options["shard_row_id_bits"]; ok {
			t.Fatal("mysql snapshot must not carry tidb layout options")
		}
		if _, ok := snapshot.Options["auto_id_cache"]; ok {
			t.Fatal("mysql snapshot must not carry auto_id_cache")
		}
	})
}
