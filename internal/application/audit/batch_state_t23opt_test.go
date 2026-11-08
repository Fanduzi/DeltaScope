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
	state, stmts := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=20 AUTO_ID_CACHE=3000 AUTO_RANDOM_BASE=5 PRE_SPLIT_REGIONS=4;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("legal options must publish, got %+v", entry)
	}
	// shape.Options is the catalog projection: shard_row_id_bits clamps
	// to MaxShardRowIDBits (15) exactly like upstream TableInfo, while
	// verbatim options keep the authored text. The authored "20" remains
	// on the statement spec (ddl.Options) — proven below.
	want := map[string]string{
		"shard_row_id_bits": "15",
		"auto_id_cache":     "3000",
		"auto_random_base":  "5",
		"pre_split_regions": "4",
	}
	for k, v := range want {
		if entry.shape.Options[k] != v {
			t.Fatalf("option %s = %q, want %q (shape %+v)", k, entry.shape.Options[k], v, entry.shape.Options)
		}
	}
	if len(stmts) != 1 || stmts[0].DDL == nil {
		t.Fatalf("expected one ddl statement, got %+v", stmts)
	}
	if got := stmts[0].DDL.Options["shard_row_id_bits"]; got != "20" {
		t.Fatalf("authored shard_row_id_bits must stay 20 on the spec, got %q", got)
	}
}

// pre_split_regions clamps to the effective sharding bits — declared
// shard bits when present, else the AUTO_RANDOM column's shard bits,
// else zero. The projected catalog value is what successors read; the
// authored text stays on the statement.
func TestT23OptionEffectiveNormalization(t *testing.T) {
	t.Parallel()
	off := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "off"}
	on := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	for _, tc := range []struct {
		name, sql, wantPreSplit, wantShard string
		facts                              *spec.InstanceFacts
	}{
		{"shard_clamps_presplit",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=4 PRE_SPLIT_REGIONS=10;",
			"4", "4", off},
		{"auto_random_bits_clamp_presplit",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM) PRE_SPLIT_REGIONS=10;",
			"5", "", on},
		{"unsharded_presplit_clamps_to_zero",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) PRE_SPLIT_REGIONS=4;",
			"0", "", off},
		{"declared_zero_shard_keeps_zero",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=0 PRE_SPLIT_REGIONS=6;",
			"0", "0", off},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t23State(t, tc.sql, &t23Provider{facts: tc.facts})
			entry := t23Entry(t, state)
			if entry == nil || entry.state != tablePresent || entry.shape == nil {
				t.Fatalf("%s: must publish, got %+v", tc.name, entry)
			}
			if got := entry.shape.Options["pre_split_regions"]; got != tc.wantPreSplit {
				t.Fatalf("%s: effective pre_split_regions = %q, want %q", tc.name, got, tc.wantPreSplit)
			}
			if tc.wantShard != "" && entry.shape.Options["shard_row_id_bits"] != tc.wantShard {
				t.Fatalf("%s: effective shard_row_id_bits = %q, want %q",
					tc.name, entry.shape.Options["shard_row_id_bits"], tc.wantShard)
			}
		})
	}
}
