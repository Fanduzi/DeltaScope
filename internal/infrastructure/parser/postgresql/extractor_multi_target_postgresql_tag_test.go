//go:build postgresql

// Package postgresql verifies PostgreSQL multi-target table extraction.
// input: PostgreSQL DROP TABLE and TRUNCATE statements naming several relations
// output: coverage that every named relation lands in the shared DDL target list
// pos: infrastructure parser extractor test coverage for multi-target completeness
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPostgreSQLDropTablePreservesEveryTarget(t *testing.T) {
	t.Parallel()

	statement := extractPostgreSQLStatement(t, "drop table if exists app.users, \"Sensitive\", audit.log_archive")
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationDropTable {
		t.Fatalf("expected drop table facts, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Schema != "app" || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected primary table to stay the first target, got %#v", statement.DDL.Table)
	}
	want := []spec.Table{
		{Schema: "app", Name: "users"},
		{Name: "Sensitive"},
		{Schema: "audit", Name: "log_archive"},
	}
	targets := statement.DDL.TableTargets()
	if len(targets) != len(want) {
		t.Fatalf("expected %d drop targets, got %#v", len(want), targets)
	}
	for i := range want {
		if targets[i].Schema != want[i].Schema || targets[i].Name != want[i].Name {
			t.Fatalf("target %d = %#v, want %#v", i, targets[i], want[i])
		}
	}
}

func TestPostgreSQLTruncatePreservesEveryTarget(t *testing.T) {
	t.Parallel()

	statement := extractPostgreSQLStatement(t, "truncate table app.users, staging.orders")
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationTruncateTable {
		t.Fatalf("expected truncate facts, got %#v", statement.DDL)
	}
	want := []spec.Table{
		{Schema: "app", Name: "users"},
		{Schema: "staging", Name: "orders"},
	}
	targets := statement.DDL.TableTargets()
	if len(targets) != len(want) {
		t.Fatalf("expected %d truncate targets, got %#v", len(want), targets)
	}
	for i := range want {
		if targets[i].Schema != want[i].Schema || targets[i].Name != want[i].Name {
			t.Fatalf("target %d = %#v, want %#v", i, targets[i], want[i])
		}
	}
}
