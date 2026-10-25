// Package mysql verifies the T10-R5 provider corrections: a PK object
// renamed off `primary` keeps its logical identity on reload (constraint
// presence and index-object attribution are judged separately — a proven
// PRIMARY KEY constraint plus zero `primary` folds means the object was
// renamed, not that no PK exists), multi-candidate attribution withholds
// the binding instead of publishing maybe-PK folds as ordinary UNIQUE
// members, catalog probe failures propagate as provider errors per the
// issue #79 contract (only clean-read absences stay unknown), and a member
// `primary` coexisting with a handle PK does not poison the handle probe.
// input: sqlmock-shaped STATISTICS/table_constraints/TIDB_INDEXES stubs
// output: snapshot PrimaryKey attribution, per-member kind provability,
// the three primary unknowns, and provider error propagation
// pos: provider-side T10-R5 catalog-fidelity tests (issue #89)
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r5Base mirrors the T10-R4 base stub minus the pk probes: table row,
// columns, the two capability probes, statistics, partitions — the test
// overlays table_constraints and tidb_indexes per case.
func t10r5Base(columnRows, statRows [][]driver.Value) map[string]testQueryResult {
	return map[string]testQueryResult{
		"select engine, table_collation": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
		},
		"information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    columnRows,
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
			rows:    statRows,
		},
		"information_schema.partitions": {
			columns: []string{"count"},
			rows:    [][]driver.Value{{0}},
		},
	}
}

func t10r5ConstraintRow(count driver.Value) testQueryResult {
	return testQueryResult{columns: []string{"count"}, rows: [][]driver.Value{{count}}}
}

// R5-A2: after RENAME INDEX `PRIMARY` TO pk2 the catalog shows a pk2
// NON_UNIQUE=0 fold, no `primary` fold, and a still-present PRIMARY KEY
// constraint row — the constraint proves presence while the pk2 fold is
// the object. Zero `primary` folds must not read as no-PK, and pk2 must
// not stay an ordinary UNIQUE member (verified on TiDB v8.5.0).
func TestT10R5TiDBRenamedPKReload(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"pk2", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	queries := withQuery(
		withQuery(t10r5Base(cols, stats), "table_constraints", t10r5ConstraintRow(1)),
		"key_name = ?",
		testQueryResult{columns: []string{"clustered"}, rows: [][]driver.Value{{"NO"}}})
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKey == nil {
		t.Fatalf("the renamed PK object must still load as the primary key, got %+v", snapshot)
	}
	if snapshot.PrimaryKey.Name != "pk2" {
		t.Fatalf("the PK object carries its actual stored name pk2, got %q", snapshot.PrimaryKey.Name)
	}
	if snapshot.FindProvenIndex("pk2") != nil {
		t.Fatalf("the PK object must not stay a member under its stored name, got %+v", snapshot.Indexes)
	}
	if snapshot.FindProvenIndex("s") == nil || len(snapshot.Indexes) != 1 {
		t.Fatalf("the ordinary member survives the promotion, got %+v", snapshot.Indexes)
	}
	if snapshot.PrimaryKeyUnknown {
		t.Fatalf("the proven constraint row keeps the PK fact vouched, got %+v", snapshot)
	}
}

// R5-A2 resolved attribution: zero `primary` folds and two NON_UNIQUE=0
// candidates separate through the constraint's declared column list —
// the fold whose columns match the PRIMARY KEY constraint is the object;
// the other unique fold stays a proven member.
func TestT10R5TiDBRenamedPKColumnResolvedAttribution(t *testing.T) {
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
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "NO"}, {"uq1", "NO"}, {"s", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"id"}}}
	queries["key_name = ?"] = testQueryResult{
		columns: []string{"clustered"},
		rows:    [][]driver.Value{{"NO"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "pk2" {
		t.Fatalf("the column-matched fold is the PK object, got %+v", snapshot.PrimaryKey)
	}
	if member := snapshot.FindProvenIndex("uq1"); member == nil || member.Kind != spec.IndexKindUnique {
		t.Fatalf("the resolved-away candidate stays a proven UNIQUE member, got %+v", snapshot.Indexes)
	}
}

// R5-A2 ambiguous attribution: a proven PRIMARY KEY constraint with zero
// `primary` folds and multiple NON_UNIQUE=0 candidates cannot prove which
// fold is the object when the declared-column match resolves none — the
// PK presence stays vouched (the constraint row is real) while the object
// identity withholds, and no candidate may stay a proven UNIQUE member
// for overlap/duplicate comparisons.
func TestT10R5TiDBRenamedPKAmbiguousAttribution(t *testing.T) {
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
	queries["table_constraints"] = t10r5ConstraintRow(1)
	queries["key_name, clustered"] = testQueryResult{
		columns: []string{"key_name", "clustered"},
		rows:    [][]driver.Value{{"pk2", "NO"}, {"uq1", "NO"}, {"s", "NO"}}}
	queries["information_schema.key_column_usage"] = testQueryResult{
		columns: []string{"column_name"},
		rows:    [][]driver.Value{{"ghost"}}}
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.HasPrimaryKey() {
		t.Fatalf("the proven constraint row keeps PK presence vouched, got %+v", snapshot)
	}
	if snapshot.PrimaryKeyUnknown {
		t.Fatalf("presence is proven — only the object identity is withheld, got %+v", snapshot)
	}
	for _, index := range snapshot.Indexes {
		if index.Name == "s" {
			continue
		}
		if index.Kind == spec.IndexKindUnique {
			t.Fatalf("an attribution-ambiguous candidate must not stay a proven UNIQUE member: %+v", index)
		}
	}
}

// R5-A2 no-candidate inconsistency: a proven PRIMARY KEY constraint with
// neither `primary` folds nor any NON_UNIQUE=0 fold contradicts itself —
// withhold instead of claiming proven absence.
func TestT10R5TiDBPKProvenNoCandidateFold(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{{"c", "int(11)", nil, nil, "", nil, "NO", "", nil}}
	stats := [][]driver.Value{{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil}}
	queries := withQuery(t10r5Base(cols, stats), "table_constraints", t10r5ConstraintRow(1))
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if !snapshot.HasPrimaryKey() {
		t.Fatalf("the constraint row still proves PK presence, got %+v", snapshot)
	}
	if snapshot.PrimaryKey != nil && !snapshot.PrimaryKey.NameUnknown && snapshot.PrimaryKey.Name != "" {
		t.Fatalf("the object identity stays unproven under the inconsistency, got %+v", snapshot.PrimaryKey)
	}
}

// R5-A2 handle-PK coexistence: a PKIsHandle table's synthesized PRIMARY
// fold and a member literally named `primary` are both legal — the unique
// `primary` fold is the constraint object while the member stays a member
// (verified v8.5.0: the member is dropped by name while pk_left=1).
func TestT10R5TiDBHandleWithMemberPrimaryFolds(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{
		{"id", "int(11)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	stats := [][]driver.Value{
		{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
		{"primary", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	// The case-insensitive key_name match returns both folds' clustered
	// facts — only the object's row may read YES, so the member's NO row
	// must not poison the handle answer.
	queries := withQuery(
		withQuery(t10r5Base(cols, stats), "table_constraints", t10r5ConstraintRow(1)),
		"key_name = ?",
		testQueryResult{columns: []string{"clustered"}, rows: [][]driver.Value{{"NO"}, {"YES"}}})
	db := openTestDB(t, queries)
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "PRIMARY" && snapshot.PrimaryKey.Name != "primary" {
		t.Fatalf("the synthesized fold is the constraint object, got %+v", snapshot.PrimaryKey)
	}
	if member := snapshot.FindProvenIndex("primary"); member == nil {
		t.Fatalf("the member named primary stays a member, got %+v", snapshot.Indexes)
	}
	if !snapshot.PrimaryKeyIsHandle || snapshot.PrimaryKeyHandleUnknown {
		t.Fatalf("the clustered=yes row proves the handle, got handle=%v unknown=%v",
			snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown)
	}
}

// R5-B: a failed table_constraints probe is a provider error, not an
// unknown fact — issue #79 requires actual provider/connection failures
// to propagate rather than degrade into evidence gaps.
func TestT10R5TiDBConstraintProbeFailurePropagates(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{{"id", "int(11)", nil, nil, "", nil, "NO", "", nil}}
	stats := [][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}}
	queries := withQuery(t10r5Base(cols, stats), "table_constraints",
		testQueryResult{columns: []string{"count"}, queryErr: errors.New("connection reset by peer")})
	db := openTestDB(t, queries)
	defer db.Close()
	_, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err == nil {
		t.Fatal("a failed catalog probe must surface as a provider error, not a withheld snapshot")
	}
}

// R5-B: a failed tidb_indexes handle probe likewise propagates — only a
// clean ErrNoRows (row genuinely absent) stays an unknown fact.
func TestT10R5TiDBClusteredProbeFailurePropagates(t *testing.T) {
	t.Parallel()
	cols := [][]driver.Value{{"id", "int(11)", nil, nil, "", nil, "NO", "", nil}}
	stats := [][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}}
	queries := withQuery(
		withQuery(t10r5Base(cols, stats), "table_constraints", t10r5ConstraintRow(1)),
		"key_name = ?",
		testQueryResult{columns: []string{"clustered"}, queryErr: errors.New("connection reset by peer")})
	db := openTestDB(t, queries)
	defer db.Close()
	_, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err == nil {
		t.Fatal("a failed catalog probe must surface as a provider error, not a withheld snapshot")
	}
}
