package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T21-R1 B3: the official ResolveObject sequence path reads
// information_schema.SEQUENCES definition columns through the real provider
// query. These tests pin the loader against the stub driver — no fake
// resolver is substituted.
const testSequenceFragment = "information_schema.sequences"

var testSequenceColumns = []string{
	"SEQUENCE_SCHEMA", "SEQUENCE_NAME", "INCREMENT", "START",
	"MIN_VALUE", "MAX_VALUE", "CACHE", "CACHE_VALUE", "CYCLE",
}

func t21r1Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

// A well-formed catalog row proves existence and publishes every observed
// definition member — runtime issuance is never modeled.
func TestT21R1SequenceLoaderPublishesDefinition(t *testing.T) {
	snapshot, err := t21r1Resolve(t, map[string]testQueryResult{
		testSequenceFragment: {
			columns: testSequenceColumns,
			rows: [][]driver.Value{{
				"t21", "seq", int64(1), int64(1), int64(1), int64(9223372036854775806), "YES", int64(1000), "NO",
			}},
		},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Schema: "t21", Type: "sequence", Name: "seq"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	def := snapshot.Sequence
	if def == nil || def.Increment == nil || *def.Increment != 1 ||
		def.Start == nil || *def.Start != 1 ||
		def.MinValue == nil || *def.MinValue != 1 ||
		def.MaxValue == nil || *def.MaxValue != 9223372036854775806 ||
		def.CacheValue == nil || *def.CacheValue != 1000 ||
		def.Cache == nil || !*def.Cache ||
		def.Cycle == nil || *def.Cycle {
		t.Fatalf("definition = %+v, want every catalog member published", def)
	}
	if !snapshot.SequenceReferencesUnknown {
		t.Fatal("SequenceReferencesUnknown = false — the bounded read must leave the edge set unvouched")
	}
	if snapshot.SequenceReferences != nil {
		t.Fatalf("SequenceReferences = %#v, want nil — no edge read happened", snapshot.SequenceReferences)
	}
}

// Zero rows stay unknown: SEQUENCES filters by privilege, so an invisible
// sequence is never a proven absence.
func TestT21R1SequenceLoaderZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t21r1Resolve(t, map[string]testQueryResult{
		testSequenceFragment: {columns: testSequenceColumns},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Schema: "t21", Type: "sequence", Name: "ghost"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.IsNotFound() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — never fabricated absence", snapshot)
	}
}

// NULL and unrecognized catalog members map to unproven fields — never a
// guessed zero, default, or cleared flag.
func TestT21R1SequenceLoaderNullMembersStayUnproven(t *testing.T) {
	snapshot, err := t21r1Resolve(t, map[string]testQueryResult{
		testSequenceFragment: {
			columns: testSequenceColumns,
			rows: [][]driver.Value{{
				"t21", "seq", nil, int64(1), nil, int64(9223372036854775806), "MAYBE", nil, "NO",
			}},
		},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Schema: "t21", Type: "sequence", Name: "seq"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	def := snapshot.Sequence
	if def == nil {
		t.Fatal("definition = nil on a confirmed row")
	}
	if def.Increment != nil || def.MinValue != nil || def.CacheValue != nil || def.Cache != nil {
		t.Fatalf("definition = %+v, want NULL/unrecognized members unproven (nil)", def)
	}
	if def.Start == nil || *def.Start != 1 || def.MaxValue == nil || *def.MaxValue != 9223372036854775806 ||
		def.Cycle == nil || *def.Cycle {
		t.Fatalf("definition = %+v, want observed members published", def)
	}
}

// Catalog errors propagate wrapped with identity context — they are never
// degraded to unavailable and never cached as absence.
func TestT21R1SequenceLoaderErrorPropagates(t *testing.T) {
	want := errors.New("sequences select denied")
	snapshot, err := t21r1Resolve(t, map[string]testQueryResult{
		testSequenceFragment: {columns: testSequenceColumns, queryErr: want},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Schema: "t21", Type: "sequence", Name: "seq"})
	if err == nil {
		t.Fatalf("snapshot = %#v, want propagated error", snapshot)
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// MySQL has no sequence catalog — the resolver must not emit the TiDB-only
// query under the wrong dialect.
func TestT21R1SequenceLoaderDialectGated(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		testSequenceFragment: {
			columns: testSequenceColumns,
			rows:    [][]driver.Value{{"t21", "seq", int64(1), int64(1), int64(1), int64(2), "YES", int64(1), "NO"}},
		},
	})
	provider := NewProvider(db)
	snapshot, err := provider.ResolveObject(context.Background(), spec.DialectMySQL, spec.ObjectLookupRequest{
		Schema: "t21", Type: "sequence", Name: "seq",
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — MySQL must not read the TiDB catalog", snapshot)
	}
}

// Unsupported object types keep the pre-existing unavailable answer.
func TestT21R1SequenceLoaderUnrelatedTypeUntouched(t *testing.T) {
	snapshot, err := t21r1Resolve(t, map[string]testQueryResult{},
		spec.DialectTiDB, spec.ObjectLookupRequest{Schema: "t21", Type: "procedure", Name: "p"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() {
		t.Fatalf("snapshot = %#v, want unavailable for unmodeled type", snapshot)
	}
}
