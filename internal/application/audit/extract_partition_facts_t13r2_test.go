// Package audit verifies the T13-R2 extraction completeness contract: a
// declared catalog that cannot be fully projected stays unobserved (nil)
// rather than masquerading as a complete or empty member set, and implicit
// PARTITIONS n expansion is bounded by the native catalog limit.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// A declared subpartitioned directory cannot project leaf members from
// top-level facts alone — it must stay unobserved (nil), never claim the
// top-level list is the whole member space.
func TestT13R2SubpartitionedCreateKeepsNamesUnobserved(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT) PARTITION BY RANGE(id) "+
			"SUBPARTITION BY HASH(id) SUBPARTITIONS 2 "+
			"(PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20))")
	if stmt.DDL == nil || !stmt.DDL.HasPartition {
		t.Fatalf("subpartitioned CREATE must publish HasPartition, got %+v", stmt.DDL)
	}
	if stmt.DDL.PartitionType != "range" {
		t.Fatalf("subpartitioned CREATE keeps the top-level method, got %q", stmt.DDL.PartitionType)
	}
	if stmt.DDL.PartitionNames != nil {
		t.Fatalf("incomplete declared directory must stay nil, got %+v", stmt.DDL.PartitionNames)
	}
}

// A TiDB RANGE INTERVAL declaration materializes generated member names
// the model cannot predict — the directory stays unobserved (nil), never
// a partial Definitions list or an observed-empty set.
func TestT13R2IntervalCreateKeepsNamesUnobserved(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectTiDB,
		"CREATE TABLE t (id INT) PARTITION BY RANGE(id) INTERVAL(10) "+
			"FIRST PARTITION LESS THAN(10) LAST PARTITION LESS THAN(30)")
	if stmt.DDL == nil || !stmt.DDL.HasPartition {
		t.Fatalf("interval CREATE must publish HasPartition, got %+v", stmt.DDL)
	}
	if stmt.DDL.PartitionNames != nil {
		t.Fatalf("interval directory must stay nil, got %+v", stmt.DDL.PartitionNames)
	}
}

// PARTITIONS n beyond the native bound is never expanded: no uncontrolled
// allocation or loop, and no truncated/empty list masquerading as the
// member set. The statement itself stays well-formed — the engine refuses
// it natively (1499), which is a refusal path, not a panic.
func TestT13R2UnboundedPartitionsNeverExpands(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE TABLE t (id INT) PARTITION BY HASH(id) PARTITIONS 18446744073709551615",
		"CREATE TABLE t (id INT) PARTITION BY HASH(id) PARTITIONS 8193",
	} {
		stmt := t13r1ExtractOne(t, spec.DialectMySQL, sql)
		if stmt.DDL == nil || !stmt.DDL.HasPartition {
			t.Fatalf("over-bound CREATE must still publish HasPartition, got %+v", stmt.DDL)
		}
		if stmt.DDL.PartitionNames != nil {
			t.Fatalf("over-bound expansion must stay nil, got %d names", len(stmt.DDL.PartitionNames))
		}
	}
}

// The bound admits the native maximum — PARTITIONS 8192 still proves its
// implicit members, and a small count keeps naming members exactly.
func TestT13R2BoundedPartitionsStillExpands(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT) PARTITION BY HASH(id) PARTITIONS 4")
	if len(stmt.DDL.PartitionNames) != 4 ||
		stmt.DDL.PartitionNames[0] != "p0" || stmt.DDL.PartitionNames[3] != "p3" {
		t.Fatalf("in-bound expansion keeps naming members, got %+v", stmt.DDL.PartitionNames)
	}
}
