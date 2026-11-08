package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT23OptionRefusalPublishesNoShape(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	for _, tc := range []struct{ name, sql string }{
		{"auto_id_cache_overflow", "CREATE TABLE t (id INT) AUTO_ID_CACHE=9223372036854775808;"},
		{"shard_clustered", "CREATE TABLE t (id BIGINT PRIMARY KEY CLUSTERED) SHARD_ROW_ID_BITS=5;"},
		{"shard_clustered_by_mode", "CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=5;"},
		{"pre_split_temp", "CREATE TEMPORARY TABLE t (id INT) PRE_SPLIT_REGIONS=2;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t23State(t, tc.sql, &t23Provider{facts: facts})
			if entry := t23Entry(t, state); entry != nil && entry.state == tablePresent {
				t.Fatalf("%s: refused create published t: %+v", tc.name, entry)
			}
		})
	}
}

func TestT23OptionUndecidableInvalidates(t *testing.T) {
	t.Parallel()
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=5;",
		&t23Provider{})
	if entry := t23Entry(t, state); entry == nil || entry.state != tableUnknown {
		t.Fatalf("clustered-unproven shard must invalidate, got %+v", entry)
	}
}

func TestT23OptionDeclaredPublishes(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "off"}
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=20 AUTO_ID_CACHE=3000 AUTO_RANDOM_BASE=5 PRE_SPLIT_REGIONS=4;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("legal options must publish, got %+v", entry)
	}
	want := map[string]string{
		"shard_row_id_bits": "20", // authored value stays; >15 clamp is effective, not declared
		"auto_id_cache":     "3000",
		"auto_random_base":  "5",
		"pre_split_regions": "4",
	}
	for k, v := range want {
		if entry.shape.Options[k] != v {
			t.Fatalf("option %s = %q, want %q (shape %+v)", k, entry.shape.Options[k], v, entry.shape.Options)
		}
	}
}
