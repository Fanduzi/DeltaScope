package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t23r8_test.go pins the T23-R8 ordered-state contract for
// the standalone physical-layout statements: neither SPLIT nor COMPACT
// moves a catalog fact, so the successor state is always the exact
// pre-state — refused, accepted, and undecidable arms alike
// (issue #102).

func t23R8Shape() *spec.TableSnapshot {
	false_ := false
	return &spec.TableSnapshot{
		Exists:      true,
		Schema:      "req",
		Partitioned: &false_,
		Columns:     []spec.Column{{Name: "id", Type: "bigint(20)"}},
		Options:     map[string]string{"cache_status": "disable"},
	}
}

func t23R8EntryPreserved(t *testing.T, sql string) {
	t.Helper()
	state, _ := t23State(t, sql, &t23Provider{snapshot: t23R8Shape()})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("%q must preserve the entry, got %+v", sql, entry)
	}
	if entry.shape.Options["cache_status"] != "disable" || entry.shape.FindColumn("id") == nil {
		t.Fatalf("%q must preserve the exact pre-state shape, got %+v", sql, entry.shape)
	}
}

func TestT23R8SplitPreservesPreState(t *testing.T) {
	t.Parallel()
	// A SPLIT that clears the modeled premises changes region topology,
	// never the catalog — the entry and its shape must survive verbatim.
	t23R8EntryPreserved(t, "SPLIT TABLE t BETWEEN (0) AND (100) REGIONS 4;")
	t23R8EntryPreserved(t, "SPLIT TABLE t BY (1),(2),(3);")
	t23R8EntryPreserved(t, "SPLIT TABLE t INDEX ix BETWEEN (0) AND (10) REGIONS 2;")
	// A statically refused arm keeps the same pre-state.
	t23R8EntryPreserved(t, "SPLIT TABLE t BETWEEN (0) AND (100) REGIONS 0;")
}

func TestT23R8CompactPreservesPreState(t *testing.T) {
	t.Parallel()
	t23R8EntryPreserved(t, "ALTER TABLE t COMPACT TIFLASH REPLICA;")
	t23R8EntryPreserved(t, "ALTER TABLE t COMPACT;")
}

func TestT23R8SplitCompactDoNotCreateEntry(t *testing.T) {
	t.Parallel()
	// With an absent target the statement must not conjure a catalog
	// identity — no entry is published.
	for _, sql := range []string{
		"SPLIT TABLE ghost BETWEEN (0) AND (100) REGIONS 4;",
		"ALTER TABLE ghost COMPACT TIFLASH REPLICA;",
	} {
		state, _ := t23State(t, sql, &t23Provider{snapshot: nil})
		key := state.keyFor("req", spec.Table{Schema: "req", Name: "ghost"})
		if entry := state.entries[key]; entry != nil && entry.state == tablePresent {
			t.Fatalf("%q must not create an entry, got %+v", sql, entry)
		}
	}
}
