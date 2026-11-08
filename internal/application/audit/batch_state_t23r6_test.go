package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t23r6_test.go pins the T23-R6 ordered-state contract for
// the physical-layout actions: refused arms preserve the exact
// pre-state, undecidable premises unvouch only the facts they could
// move, and applied arms publish the catalog projection (issue #102).

func t23R6Shape() *spec.TableSnapshot {
	false_ := false
	return &spec.TableSnapshot{
		Exists:      true,
		Schema:      "req",
		Partitioned: &false_,
		Columns:     []spec.Column{{Name: "id", Type: "bigint(20)", Charset: "utf8mb4"}},
		Options:     map[string]string{"cache_status": "disable"},
	}
}

func TestT23R6CacheTransitions(t *testing.T) {
	t.Parallel()
	// CACHE on a proven-disabled, non-partitioned, utf8 table reaches the
	// 64MB data-size premise — undecidable, so the status key unvouches.
	state, _ := t23State(t, "ALTER TABLE t CACHE;", &t23Provider{snapshot: t23R6Shape()})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("cache undecidable keeps the entry, got %+v", entry)
	}
	if _, ok := entry.shape.Options["cache_status"]; ok {
		t.Fatalf("size premise must unvouch cache_status, got %+v", entry.shape.Options)
	}

	// NOCACHE on a proven-enabled table applies and publishes disable.
	enabled := t23R6Shape()
	enabled.Options["cache_status"] = "enable"
	state, _ = t23State(t, "ALTER TABLE t NOCACHE;", &t23Provider{snapshot: enabled})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil || entry.shape.Options["cache_status"] != "disable" {
		t.Fatalf("nocache applied must publish disable, got %+v", entry)
	}

	// CACHE on a proven-partitioned table is a proven refusal — the
	// pre-state status survives intact.
	partitioned := t23R6Shape()
	partTrue := true
	partitioned.Partitioned = &partTrue
	state, _ = t23State(t, "ALTER TABLE t CACHE;", &t23Provider{snapshot: partitioned})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil || entry.shape.Options["cache_status"] != "disable" {
		t.Fatalf("partitioned refusal preserves pre-state, got %+v", entry)
	}
}

// The cached-table guard refuses every other spec on a proven
// cache-enabled table — preserving instead of tombstoning.
func TestT23R6CachedTableGuardPreserves(t *testing.T) {
	t.Parallel()
	enabled := t23R6Shape()
	enabled.Options["cache_status"] = "enable"
	state, _ := t23State(t,
		"ALTER TABLE t MODIFY COLUMN id BIGINT(21);",
		&t23Provider{snapshot: enabled})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("guard refusal must preserve the entry, got %+v", entry)
	}
	if entry.shape.Options["cache_status"] != "enable" || entry.shape.FindColumn("id") == nil {
		t.Fatalf("guard refusal must preserve the exact shape, got %+v", entry.shape)
	}
}

func TestT23R6AttributesTransitions(t *testing.T) {
	t.Parallel()
	// Table-level valid spec applies and vouches presence only — the raw
	// value never enters the published shape.
	state, _ := t23State(t, "ALTER TABLE t ATTRIBUTES='merge_option=deny,x=1';", &t23Provider{snapshot: t23R6Shape()})
	entry := t23Entry(t, state)
	if entry == nil || entry.shape == nil || entry.shape.Options["attributes_present"] != "true" {
		t.Fatalf("attributes applied must vouch presence, got %+v", entry)
	}
	for key, value := range entry.shape.Options {
		if value == "merge_option=deny,x=1" || key == "attributes" {
			t.Fatalf("raw attribute text must never publish, got %s=%q", key, value)
		}
	}

	// ATTRIBUTES=DEFAULT clears to a proven absent marker.
	state, _ = t23State(t, "ALTER TABLE t ATTRIBUTES=DEFAULT;", &t23Provider{snapshot: t23R6Shape()})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil || entry.shape.Options["attributes_present"] != "false" {
		t.Fatalf("default must publish cleared presence, got %+v", entry)
	}

	// An unparseable spec refuses — the pre-state stays.
	state, _ = t23State(t, "ALTER TABLE t ATTRIBUTES='notalabel';", &t23Provider{snapshot: t23R6Shape()})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil {
		t.Fatalf("invalid refusal preserves entry, got %+v", entry)
	}
	if _, ok := entry.shape.Options["attributes_present"]; ok {
		t.Fatalf("refused spec must not vouch attributes, got %+v", entry.shape.Options)
	}

	// Partition attributes publish per member and leave the table level
	// and other members untouched.
	partitioned := t23R6Shape()
	partTrue := true
	partitioned.Partitioned = &partTrue
	partitioned.PartitionNames = []string{"p0", "p1"}
	state, _ = t23State(t,
		"ALTER TABLE t PARTITION p0 ATTRIBUTES='merge_option=deny';",
		&t23Provider{snapshot: partitioned})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil {
		t.Fatalf("partition attributes applied, got %+v", entry)
	}
	if entry.shape.Options["partition_attributes_present.p0"] != "true" {
		t.Fatalf("p0 presence missing: %+v", entry.shape.Options)
	}
	if _, ok := entry.shape.Options["partition_attributes_present.p1"]; ok {
		t.Fatalf("p1 must stay untouched: %+v", entry.shape.Options)
	}
	if _, ok := entry.shape.Options["attributes_present"]; ok {
		t.Fatalf("partition-level must not touch the table level: %+v", entry.shape.Options)
	}
}

func TestT23R6TiFlashTransitions(t *testing.T) {
	t.Parallel()
	// count>0 owes the live store-count premise — the replica keys
	// unvouch instead of claiming a desired definition.
	state, _ := t23State(t, "ALTER TABLE t SET TIFLASH REPLICA 2;", &t23Provider{snapshot: t23R6Shape()})
	entry := t23Entry(t, state)
	if entry == nil || entry.shape == nil {
		t.Fatalf("tiflash undecidable keeps entry, got %+v", entry)
	}
	if _, ok := entry.shape.Options["tiflash_replica_count"]; ok {
		t.Fatalf("store premise must unvouch replica keys, got %+v", entry.shape.Options)
	}

	// count=0 applies unconditionally and publishes the proven-absent
	// marker (upstream nils the record).
	state, _ = t23State(t, "ALTER TABLE t SET TIFLASH REPLICA 0;", &t23Provider{snapshot: t23R6Shape()})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil || entry.shape.Options["tiflash_replica_count"] != "0" {
		t.Fatalf("replica removal must publish absent marker, got %+v", entry)
	}

	// A proven same-definition redeclare is a no-op — the stored keys
	// survive unchanged.
	withReplica := t23R6Shape()
	withReplica.Options["tiflash_replica_count"] = "2"
	withReplica.Options["tiflash_replica_labels"] = "zone1"
	state, _ = t23State(t,
		"ALTER TABLE t SET TIFLASH REPLICA 2 LOCATION LABELS 'zone1';",
		&t23Provider{snapshot: withReplica})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil ||
		entry.shape.Options["tiflash_replica_count"] != "2" ||
		entry.shape.Options["tiflash_replica_labels"] != "zone1" {
		t.Fatalf("same-definition noop preserves replica keys, got %+v", entry)
	}

	// A utf16 column is a proven charset refusal — the shape survives.
	utf16 := t23R6Shape()
	utf16.Columns[0].Charset = "utf16"
	state, _ = t23State(t, "ALTER TABLE t SET TIFLASH REPLICA 2;", &t23Provider{snapshot: utf16})
	entry = t23Entry(t, state)
	if entry == nil || entry.shape == nil {
		t.Fatalf("charset refusal preserves entry, got %+v", entry)
	}
	if _, ok := entry.shape.Options["tiflash_replica_count"]; ok {
		t.Fatalf("refused replica must not publish, got %+v", entry.shape.Options)
	}
}

func t23R7ShapeStatus(status string) *spec.TableSnapshot {
	shape := t23R6Shape()
	if status == "" {
		delete(shape.Options, "cache_status")
	} else {
		shape.Options["cache_status"] = status
	}
	return shape
}

// TestT23R7CacheGuardCoverage exercises the shared upstream
// `!= Disable` gate across every supported mutation path (issue #102
// R7): proven non-Disable refuses and preserves, proven Disable keeps
// existing behavior, and an unobserved status unvouches only what the
// withheld transition could have moved.
func TestT23R7CacheGuardCoverage(t *testing.T) {
	t.Parallel()

	// Proven non-Disable refuses each covered write — exact pre-state.
	for _, sql := range []string{
		"DROP TABLE t;", "TRUNCATE TABLE t;", "RENAME TABLE t TO t2;",
		"CREATE INDEX ix ON t (id);", "ALTER TABLE t ADD COLUMN c INT;",
		"ALTER TABLE t DROP INDEX ix;", "ALTER TABLE t RENAME TO t2;",
	} {
		state, _ := t23State(t, sql, &t23Provider{snapshot: t23R7ShapeStatus("enable")})
		if entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]; entry == nil ||
			entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("%s on enable must preserve the pre-state, got %+v", sql, entry)
		}
	}

	// Proven Disable keeps the ordinary transition (drop is the clearest).
	state, _ := t23State(t, "DROP TABLE t;", &t23Provider{snapshot: t23R7ShapeStatus("disable")})
	if entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]; entry == nil || entry.state != tableAbsent {
		t.Fatalf("drop on disable must publish absent, got %+v", entry)
	}

	// Unobserved status: ALTER withholds to present-incomplete (presence
	// proven either way); DROP unvouches existence entirely.
	state, _ = t23State(t, "ALTER TABLE t ADD COLUMN c INT;", &t23Provider{snapshot: t23R7ShapeStatus("")})
	entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]
	if entry == nil || entry.state != tablePresent || entry.shape != nil {
		t.Fatalf("alter on unobserved status must withhold to present-incomplete, got %+v", entry)
	}
	state, _ = t23State(t, "DROP TABLE t;", &t23Provider{snapshot: t23R7ShapeStatus("")})
	if entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]; entry == nil || entry.state != tableUnknown {
		t.Fatalf("drop on unobserved status must unvouch existence, got %+v", entry)
	}
	state, _ = t23State(t, "CREATE INDEX ix ON t (id);", &t23Provider{snapshot: t23R7ShapeStatus("")})
	if entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]; entry == nil ||
		entry.state != tablePresent || entry.shape != nil {
		t.Fatalf("create index on unobserved status must withhold shape, got %+v", entry)
	}

	// NOCACHE on an unobserved status still publishes the converged
	// Disable post-state — every legal old status converges upstream,
	// without claiming the old state was observed.
	state, _ = t23State(t, "ALTER TABLE t NOCACHE;", &t23Provider{snapshot: t23R7ShapeStatus("")})
	entry = state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]
	if entry == nil || entry.shape == nil || entry.shape.Options["cache_status"] != "disable" {
		t.Fatalf("nocache on unobserved status must converge to disable, got %+v", entry)
	}

	// A batch-created table carries the proven Disable default — the
	// guard does not withhold its first mutation.
	state, _ = t23State(t, "CREATE TABLE t (id INT); ALTER TABLE t ADD COLUMN c INT;", &t23Provider{})
	entry = state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]
	if entry == nil || entry.shape == nil || entry.shape.FindColumn("c") == nil {
		t.Fatalf("batch-created table must publish disable and apply the alter, got %+v", entry)
	}
}

// TestT23R7CacheGuardRenameOccupiedDestination pins the review
// counterexample: when source and destination are both proven-present
// and the source cache status is unobserved, the statement refuses on
// endpoint identity upstream (ExtractTblInfos precedes the cache gate),
// so neither name may lose its proof to the withheld premise — a
// successor must not meet a fabricated existence gap (issue #102 R7).
func TestT23R7CacheGuardRenameOccupiedDestination(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"RENAME TABLE t TO t2;",
		"ALTER TABLE t RENAME TO t2;",
	} {
		state, _ := t23State(t, sql, &t23Provider{snapshot: t23R7ShapeStatus("")})
		for _, name := range []string{"t", "t2"} {
			entry := state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: name})]
			if entry == nil || entry.state != tablePresent || entry.shape == nil {
				t.Fatalf("%s with occupied proven destination must preserve %q, got %+v", sql, name, entry)
			}
		}
	}
}
