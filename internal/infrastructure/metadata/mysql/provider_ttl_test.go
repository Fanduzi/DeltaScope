package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t19TiDBSnapshotResults(createSQL string) map[string]testQueryResult {
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows", "table_type"},
			rows: [][]driver.Value{{
				"InnoDB", "utf8mb4_bin", "", nil, "Dynamic", nil, "BASE TABLE",
			}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows: [][]driver.Value{{
				"created_at", "datetime", nil, nil, "", nil, "YES", "", nil,
			}},
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation"},
		},
		"constraint_type = 'PRIMARY KEY'": {
			columns: []string{"count"},
			rows:    [][]driver.Value{{int64(0)}},
		},
		testTTLFragment: {
			columns: []string{"Table", "Create Table"},
			rows:    [][]driver.Value{{"t", createSQL}},
		},
	}
}

func TestT19LoadTableSnapshotTiDBTTL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		createSQL     string
		wantConfig    *spec.TTLConfig
		wantEnabled   *bool
		wantKnownOnly bool
	}{
		{
			name:          "clean catalog read proves no ttl",
			createSQL:     "CREATE TABLE `t` (`created_at` datetime)",
			wantKnownOnly: true,
		},
		{
			name:      "full ttl config",
			createSQL: "CREATE TABLE `t` (`created_at` datetime) /*T![ttl] TTL=`created_at` + INTERVAL 30 DAY */ /*T![ttl] TTL_ENABLE='OFF' */ /*T![ttl] TTL_JOB_INTERVAL='5m' */",
			wantConfig: &spec.TTLConfig{
				TimeColumn:    "created_at",
				IntervalValue: "30",
				IntervalUnit:  "day",
				JobInterval:   "5m",
			},
		},
		{
			name:      "unrecognized enable scalar stays unobserved",
			createSQL: "CREATE TABLE `t` (`created_at` datetime) /*T![ttl] TTL=`created_at` + INTERVAL 30 DAY */ /*T![ttl] TTL_ENABLE='MAYBE' */",
			wantConfig: &spec.TTLConfig{
				TimeColumn:    "created_at",
				IntervalValue: "30",
				IntervalUnit:  "day",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestDB(t, t19TiDBSnapshotResults(tc.createSQL))
			defer db.Close()

			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			if !snapshot.TTLKnown {
				t.Fatalf("TTLKnown = false, want proven catalog answer")
			}
			if tc.wantKnownOnly {
				if snapshot.TTL != nil {
					t.Fatalf("TTL = %#v, want proven nil", snapshot.TTL)
				}
				return
			}
			if snapshot.TTL == nil {
				t.Fatalf("TTL = nil, want config %#v", tc.wantConfig)
			}
			if snapshot.TTL.TimeColumn != tc.wantConfig.TimeColumn || snapshot.TTL.IntervalValue != tc.wantConfig.IntervalValue || snapshot.TTL.IntervalUnit != tc.wantConfig.IntervalUnit || snapshot.TTL.JobInterval != tc.wantConfig.JobInterval {
				t.Fatalf("TTL = %#v, want %#v", snapshot.TTL, tc.wantConfig)
			}
			if tc.name == "full ttl config" {
				if snapshot.TTL.Enabled == nil || *snapshot.TTL.Enabled {
					t.Fatalf("TTL_ENABLE='OFF' produced Enabled=%#v", snapshot.TTL.Enabled)
				}
			} else if snapshot.TTL.Enabled != nil {
				t.Fatalf("unrecognized TTL_ENABLE must leave Enabled unobserved, got %v", *snapshot.TTL.Enabled)
			}
		})
	}
}

func TestT19LoadTableSnapshotTiDBTTLErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("show create denied")
	results := t19TiDBSnapshotResults("")
	results[testTTLFragment] = testQueryResult{queryErr: sentinel}
	db := openTestDB(t, results)
	defer db.Close()

	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if snapshot != nil || !errors.Is(err, sentinel) {
		t.Fatalf("snapshot=%#v err=%v, want propagated %v", snapshot, err, sentinel)
	}
}

func TestT23R7CacheStatusRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		createSQL string
		metaRows  [][]driver.Value // nil = shared stub (anchor row + NULL lock cell)
		stub      bool             // true = override the shared stub even for empty metaRows
		want      string           // "" means the key must stay unobserved
	}{
		// A complete successful SHOW CREATE carrying the bare upstream
		// marker publishes proven Enable — the marker is direct status
		// evidence and does not need the lock row.
		{name: "cached_on marker proves enable", createSQL: "CREATE TABLE `t` (`id` int) /* CACHED ON */", want: "enable"},
		// Anchor row + NULL lock cell proves never-cached Disable — the
		// lock row is only ever inserted/updated upstream.
		{name: "no marker plus absent lock row proves disable", createSQL: "CREATE TABLE `t` (`id` int)", want: "disable"},
		{name: "quoted fake marker is still proven disable", createSQL: "CREATE TABLE `t` (`id` int) COMMENT='x /* CACHED ON */ y'", want: "disable"},
		{name: "comment feature marker is not the cache marker", createSQL: "CREATE TABLE `t` (`id` int) /*T![auto_id_cache] AUTO_ID_CACHE=5 */", want: "disable"},
		// A live lock row without the marker is ambiguous — completed
		// NOCACHE, pending CACHE, and real Switching share that exact
		// projection — so the status stays unobserved rather than
		// claiming Switching (review-finding fix: the lock record is
		// not a status projection).
		{name: "lock row without marker stays unobserved", createSQL: "CREATE TABLE `t` (`id` int)", metaRows: [][]driver.Value{{int64(42)}}, stub: true, want: ""},
		{name: "lock row plus marker proves enable", createSQL: "CREATE TABLE `t` (`id` int) /* CACHED ON */", metaRows: [][]driver.Value{{int64(42)}}, stub: true, want: "enable"},
		// Zero rows means the anchor table row itself was not observed —
		// that is an unobserved fact, never a fabricated never-cached.
		{name: "missing anchor row stays unobserved", createSQL: "CREATE TABLE `t` (`id` int)", metaRows: [][]driver.Value{}, stub: true, want: ""},
		{name: "missing anchor row plus marker still proves enable", createSQL: "CREATE TABLE `t` (`id` int) /* CACHED ON */", metaRows: [][]driver.Value{}, stub: true, want: "enable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results := t19TiDBSnapshotResults(tc.createSQL)
			if tc.stub {
				results[testCacheMetaFragment] = testQueryResult{columns: []string{"tid"}, rows: tc.metaRows}
			}
			db := openTestDB(t, results)
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			got, ok := snapshot.Options["cache_status"]
			if tc.want == "" {
				if ok {
					t.Fatalf("cache_status = %q, want unobserved (ambiguous projections never claim a status)", got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Fatalf("cache_status = %q ok=%v, want %q", got, ok, tc.want)
			}
		})
	}
}

// TestT23R7CacheStatusReadErrorPropagates proves the auxiliary
// mysql.table_cache_meta read keeps the provider fail-loud contract:
// a real query error is wrapped and returned, never swallowed into a
// missing cache_status key (issue #102 R7 review).
func TestT23R7CacheStatusReadErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cache meta denied")
	results := t19TiDBSnapshotResults("CREATE TABLE `t` (`id` int)")
	results[testCacheMetaFragment] = testQueryResult{columns: []string{"tid"}, queryErr: sentinel}
	db := openTestDB(t, results)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if snapshot != nil || !errors.Is(err, sentinel) {
		t.Fatalf("snapshot=%#v err=%v, want propagated %v", snapshot, err, sentinel)
	}
}

func TestT19LoadTableSnapshotTiDBReferencedBy(t *testing.T) {
	t.Parallel()
	t.Run("clean catalog read proves empty inbound set", func(t *testing.T) {
		t.Parallel()
		db := openTestDB(t, t19TiDBSnapshotResults("CREATE TABLE `t` (`created_at` datetime)"))
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if snapshot.ReferencedBy == nil || len(snapshot.ReferencedBy) != 0 || snapshot.ReferencedByUnknown {
			t.Fatalf("ReferencedBy = %#v unknown=%v, want proven empty set", snapshot.ReferencedBy, snapshot.ReferencedByUnknown)
		}
	})
	t.Run("catalog rows publish child table identities", func(t *testing.T) {
		t.Parallel()
		results := t19TiDBSnapshotResults("CREATE TABLE `t` (`created_at` datetime)")
		results[testInboundFKFragment] = testQueryResult{
			columns: []string{"table_schema", "table_name"},
			rows:    [][]driver.Value{{"app", "child"}},
		}
		db := openTestDB(t, results)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if err != nil {
			t.Fatalf("LoadTableSnapshot: %v", err)
		}
		if len(snapshot.ReferencedBy) != 1 || snapshot.ReferencedBy[0].Schema != "app" || snapshot.ReferencedBy[0].Name != "child" || snapshot.ReferencedByUnknown {
			t.Fatalf("ReferencedBy = %#v unknown=%v, want app.child", snapshot.ReferencedBy, snapshot.ReferencedByUnknown)
		}
	})
	t.Run("failed reverse-edge read propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("inbound edge denied")
		results := t19TiDBSnapshotResults("CREATE TABLE `t` (`created_at` datetime)")
		results[testInboundFKFragment] = testQueryResult{queryErr: sentinel}
		db := openTestDB(t, results)
		defer db.Close()
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		if snapshot != nil || !errors.Is(err, sentinel) {
			t.Fatalf("snapshot=%#v err=%v, want propagated %v", snapshot, err, sentinel)
		}
	})
}
