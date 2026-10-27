// Package audit verifies the T13-R1 extraction seam: CREATE TABLE publishes
// the partition catalog facts the applicability rules consume, and ALTER
// table carries ALGORITHM/LOCK modifier options so the ordered state can
// classify proven-combinable mixed forms.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t13r1ExtractOne(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()
	parsed, err := parseSQL(context.Background(), sql, dialect)
	if err != nil || len(parsed.Statements) != 1 {
		t.Fatalf("parse %q: %v (%d statements)", sql, err, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil || len(statements) != 1 {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return statements[0]
}

// A CREATE TABLE partitioned by RANGE publishes its catalog truth — type
// and member names — so later ALTER PARTITION checks can answer from the
// derived shape without guessing.
func TestT13R1CreatePartitionedPublishesCatalog(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT) PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20))")
	if stmt.DDL == nil || !stmt.DDL.HasPartition {
		t.Fatalf("partitioned CREATE must publish HasPartition, got %+v", stmt.DDL)
	}
	if stmt.DDL.PartitionType != "range" {
		t.Fatalf("partitioned CREATE must publish type range, got %+v", stmt.DDL.PartitionType)
	}
	if len(stmt.DDL.PartitionNames) != 2 || stmt.DDL.PartitionNames[0] != "p0" || stmt.DDL.PartitionNames[1] != "p1" {
		t.Fatalf("partitioned CREATE must publish member names, got %+v", stmt.DDL.PartitionNames)
	}
}

// HASH and KEY declarations publish the same catalog truth in their own
// family — the member list on HASH counts is the declared set.
func TestT13R1CreateHashPartitionPublishesCatalog(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT) PARTITION BY HASH(id) PARTITIONS 3")
	if stmt.DDL.PartitionType != "hash" {
		t.Fatalf("hash CREATE must publish type hash, got %+v", stmt.DDL.PartitionType)
	}
	// `PARTITIONS n` declares implicit members p0..p(n-1) — the catalog must
	// name them, not stay silent.
	if len(stmt.DDL.PartitionNames) != 3 || stmt.DDL.PartitionNames[0] != "p0" {
		t.Fatalf("PARTITIONS 3 must publish p0..p2, got %+v", stmt.DDL.PartitionNames)
	}
}

// A non-partitioned CREATE publishes a proven-false partitioned truth — not
// an unobserved one.
func TestT13R1CreatePlainPublishesNotPartitioned(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL, "CREATE TABLE t (id INT)")
	if stmt.DDL.HasPartition {
		t.Fatalf("plain CREATE must not publish HasPartition, got %+v", stmt.DDL)
	}
}

// ALGORITHM=INPLACE and LOCK=DEFAULT land on the alter spec as bounded
// option facts — the refusal classifier reads them by name.
func TestT13R1AlterPublishesModifierOptions(t *testing.T) {
	t.Parallel()
	stmt := t13r1ExtractOne(t, spec.DialectMySQL,
		"ALTER TABLE t ALGORITHM=INPLACE, LOCK=DEFAULT, ADD COLUMN c INT")
	if stmt.DDL == nil || len(stmt.DDL.Alter) == 0 {
		t.Fatalf("alter must be extracted, got %+v", stmt.DDL)
	}
	var algorithm, lock *spec.Alter
	for i := range stmt.DDL.Alter {
		switch stmt.DDL.Alter[i].Action {
		case "algorithm":
			algorithm = &stmt.DDL.Alter[i]
		case "lock":
			lock = &stmt.DDL.Alter[i]
		}
	}
	if algorithm == nil || algorithm.Options["algorithm"] != "inplace" {
		t.Fatalf("ALGORITHM=INPLACE must land as a bounded option, got %+v", stmt.DDL.Alter)
	}
	if lock == nil || lock.Options["lock"] != "default" {
		t.Fatalf("LOCK=DEFAULT must land as a bounded option, got %+v", stmt.DDL.Alter)
	}
}
