// Package mysql verifies the T10-R6 provider attribution closure: the
// ordered KEY_COLUMN_USAGE sequence — not the STATISTICS index_name sort —
// separates a renamed PK object from same-column-set UNIQUE candidates,
// multi-candidate matches withhold instead of promoting the first fold,
// clean-read-but-unproven CLUSTERED coverage keeps the storage facts
// unknown (a missing or unrecognized row is never a proven NO), and an
// inconsistent constraint count demotes every maybe-PK fold rather than
// leaving it a proven UNIQUE member.
// input: sqlmock-shaped STATISTICS/table_constraints/TIDB_INDEXES/
// KEY_COLUMN_USAGE stubs
// output: snapshot PrimaryKey attribution, per-member kind provability,
// the three primary unknowns, and provider error propagation
// pos: provider-side T10-R6 attribution tests (issue #89)
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// A: two NON_UNIQUE=0 folds carry the same ordered column list — the
// PRIMARY KEY constraint is proven while STATISTICS order alone cannot
// pick the object (index_name sorts alphabetically, so aa reads first).
// The attribution must withhold: presence stays vouched, the stored name
// and every still-possible candidate demote — the first
// columnsSequenceEqualFold hit must never be promoted (verified catalog
// shape on TiDB v8.5.0: UNIQUE KEY aa(id) beside PRIMARY KEY(id) renamed
// to pk2 leaves both folds NON_UNIQUE=0 on the same column list).
func TestT10R6TiDBSameSequenceCandidatesWithhold(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"aa", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"aa", "NO"}, {"pk2", "NO"}, {"s", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"id"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.HasPrimaryKey() || snapshot.PrimaryKeyUnknown {
		t.Fatalf("the proven constraint row keeps PK presence vouched, got %+v", snapshot)
	}
	if snapshot.PrimaryKey == nil || !snapshot.PrimaryKey.NameUnknown {
		t.Fatalf("two equal-sequence candidates cannot attribute the object — withhold its name, got %+v", snapshot.PrimaryKey)
	}
	for _, index := range snapshot.Indexes {
		if index.Name == "s" {
			continue
		}
		if index.Kind == spec.IndexKindUnique {
			t.Fatalf("a maybe-PK fold must not publish as a proven UNIQUE member: %+v", index)
		}
	}
	if member := snapshot.FindProvenIndex("s"); member == nil || member.Kind != spec.IndexKindSecondary {
		t.Fatalf("the identity-proven secondary member stays published, got %+v", snapshot.Indexes)
	}
	if snapshot.PrimaryKeyClusteredUnknown || snapshot.PrimaryKeyHandleUnknown ||
		snapshot.PrimaryKeyClustered || snapshot.PrimaryKeyIsHandle {
		t.Fatalf("every candidate's recognized NO proves nonclustered storage even under withheld identity, got %+v", snapshot)
	}
}

// A: the same column SET in a different order is a different member
// sequence — a UNIQUE (b,a) fold must never match a PRIMARY KEY (a,b)
// constraint. The ordered KCU list resolves the pk2 fold uniquely while
// the reverse-order fold stays a proven UNIQUE member (native catalog
// shape verified on TiDB v8.5.0: UNIQUE KEY aa(b,a) beside PRIMARY
// KEY(a,b) renamed to pk2 reports KCU ordinal sequence a,b and keeps the
// member's own b,a order).
func TestT10R6TiDBReverseOrderCandidateRejected(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"a", "int(11)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"aa", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
		{"aa", 0, "BTREE", "a", nil, nil, "A", "YES", nil},
		{"pk2", 0, "BTREE", "a", nil, nil, "A", "YES", nil},
		{"pk2", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
		{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"aa", "NO"}, {"pk2", "NO"}, {"s", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"a"}, {"b"}}}
	queries["key_name = ?"] = testQueryResult{
		columns: []string{"clustered"}, rows: [][]driver.Value{{"NO"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "pk2" {
		t.Fatalf("the ordered (a,b) fold is the PK object, got %+v", snapshot.PrimaryKey)
	}
	if member := snapshot.FindProvenIndex("aa"); member == nil || member.Kind != spec.IndexKindUnique {
		t.Fatalf("the reverse-order fold stays a proven UNIQUE member, got %+v", snapshot.Indexes)
	}
}

// A positive control: exactly one clustered candidate attributes the
// object outright — only the PK object's TIDB_INDEXES row may read YES.
func TestT10R6TiDBUniqueClusteredCandidatePromotes(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "YES"}, {"uq1", "NO"}}}
	queries["key_name = ?"] = testQueryResult{
		columns: []string{"clustered"}, rows: [][]driver.Value{{"YES"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "pk2" {
		t.Fatalf("the single clustered candidate is the PK object, got %+v", snapshot.PrimaryKey)
	}
	if !snapshot.PrimaryKeyClustered {
		t.Fatalf("the clustered YES row proves clustered storage, got %+v", snapshot)
	}
	if member := snapshot.FindProvenIndex("uq1"); member == nil || member.Kind != spec.IndexKindUnique {
		t.Fatalf("the nonclustered candidate stays a proven UNIQUE member, got %+v", snapshot.Indexes)
	}
}

// A: a clean-read CLUSTERED result that simply lacks a candidate's row is
// unproven coverage, never a proven NO — "no YES row" is not "every
// candidate proved NO". With the column list resolving nothing, the
// withheld attribution must keep clustered/handle unknown instead of
// publishing NONCLUSTERED.
func TestT10R6TiDBMissingClusteredRowKeepsStorageUnknown(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"ghost"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.HasPrimaryKey() || snapshot.PrimaryKey == nil || !snapshot.PrimaryKey.NameUnknown {
		t.Fatalf("unresolvable candidates withhold the object identity, got %+v", snapshot.PrimaryKey)
	}
	if !snapshot.PrimaryKeyClusteredUnknown || !snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("uq1's missing clustered row is unproven coverage — keep storage unknown, got %+v", snapshot)
	}
}

// A: an unrecognized or NULL CLUSTERED value is equally unproven — only a
// recognized YES/NO answers the storage fact.
func TestT10R6TiDBUnrecognizedClusteredValueKeepsStorageUnknown(t *testing.T) {
	t.Parallel()
	for _, value := range []driver.Value{"PERHAPS", nil} {
		value := value
		cols := [][]driver.Value{
			{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
			{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
		}
		stats := [][]driver.Value{
			{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
			{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
		}
		queries := t10r5Base(cols, stats)
		queries["table_constraints"] = t10r5ConstraintRow(1)
		queries["key_name, clustered"] = testQueryResult{
			columns: []string{"key_name", "clustered"},
			rows:    [][]driver.Value{{"pk2", "NO"}, {"uq1", value}}}
		queries["information_schema.key_column_usage"] = testQueryResult{
			columns: []string{"column_name"},
			rows:    [][]driver.Value{{"ghost"}}}
		db := openTestDB(t, queries)
		snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
		db.Close()
		if err != nil {
			t.Fatalf("value %v: LoadTableSnapshot must not fail on an unrecognized clustered value: %v", value, err)
		}
		if !snapshot.PrimaryKeyClusteredUnknown || !snapshot.PrimaryKeyHandleUnknown {
			t.Fatalf("value %v: an unrecognized clustered value is unproven, not NO — got %+v", value, snapshot)
		}
	}
}

// A control: complete recognized NO coverage keeps the withheld
// attribution's storage answer proven — nonclustered AND not-handle both
// follow from every candidate's NO row.
func TestT10R6TiDBCompleteClusteredCoverageProvesNonclustered(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "NO"}, {"uq1", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"ghost"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.HasPrimaryKey() || snapshot.PrimaryKey == nil || !snapshot.PrimaryKey.NameUnknown {
		t.Fatalf("unresolvable candidates withhold the object identity, got %+v", snapshot.PrimaryKey)
	}
	if snapshot.PrimaryKeyClusteredUnknown || snapshot.PrimaryKeyHandleUnknown ||
		snapshot.PrimaryKeyClustered || snapshot.PrimaryKeyIsHandle {
		t.Fatalf("every candidate's recognized NO proves nonclustered/not-handle, got %+v", snapshot)
	}
}

// A: an inconsistent constraint count (pkCount>1) must obey the same
// candidate constraint — no maybe-PK fold may stay a proven UNIQUE member
// while the PK facts are withheld.
func TestT10R6TiDBPKCountMultipleDemotesCandidates(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
		{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(2)
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.PrimaryKeyUnknown || !snapshot.PrimaryKeyHandleUnknown || !snapshot.PrimaryKeyClusteredUnknown {
		t.Fatalf("an inconsistent constraint count withholds all three primary facts, got %+v", snapshot)
	}
	for _, index := range snapshot.Indexes {
		if index.Kind == spec.IndexKindUnique {
			t.Fatalf("a maybe-PK fold under an inconsistent constraint count must demote, got %+v", index)
		}
	}
}

// A pin: a failed KEY_COLUMN_USAGE probe is a provider error — the R5-B
// error chain stays intact through the column-list disambiguation.
func TestT10R6TiDBKeyColumnUsageProbeFailurePropagates(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"b", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"uq1", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
	}
	queries := t10r5Base(cols, stats)
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "NO"}, {"uq1", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns:  []string{"column_name"},
		queryErr: errors.New("connection reset by peer")}
	db := openTestDB(t, queries)
	defer db.Close()
	if _, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t"); err == nil {
		t.Fatal("a failed key_column_usage probe must surface as a provider error, not a withheld snapshot")
	}
}
