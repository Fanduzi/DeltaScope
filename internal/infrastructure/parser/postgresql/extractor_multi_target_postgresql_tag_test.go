//go:build postgresql

// Package postgresql verifies PostgreSQL multi-target table extraction.
package postgresql

import (
	"encoding/json"
	"strings"
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

func TestPostgreSQLDestructiveTableModifiers(t *testing.T) {
	s := extractPostgreSQLStatement(t, `DROP TABLE IF EXISTS "App"." T ", db.audit.log, "App"." T " CASCADE`)
	d := s.DDL.PGDestructiveTable
	if d == nil || !d.IfExists || !d.Cascade || d.RestartIdentity || len(d.Targets) != 3 ||
		d.Targets[0].Table.Name != " T " || d.Targets[0].Table.Schema != "App" || d.Targets[0].CatalogQualified ||
		!d.Targets[1].CatalogQualified || d.Targets[1].Table.Schema != "audit" || d.Targets[2].Table.Name != d.Targets[0].Table.Name {
		t.Fatalf("DROP facts=%+v", d)
	}
	s = extractPostgreSQLStatement(t, `TRUNCATE TABLE ONLY "App"." T ", db.audit.log *, unqualified RESTART IDENTITY CASCADE`)
	d = s.DDL.PGDestructiveTable
	if d == nil || d.IfExists || !d.Cascade || !d.RestartIdentity || len(d.Targets) != 3 || !d.Targets[0].Only ||
		d.Targets[1].Only || !d.Targets[1].CatalogQualified || d.Targets[2].Only || d.Targets[2].Table.Schema != "" {
		t.Fatalf("TRUNCATE facts=%+v", d)
	}
	encoded, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(encoded), "PGDestructiveTable") || strings.Contains(string(encoded), "RestartIdentity") {
		t.Fatalf("private facts=%s err=%v", encoded, err)
	}
	s = extractPostgreSQLStatement(t, `TRUNCATE "App"." T " CONTINUE IDENTITY RESTRICT`)
	d = s.DDL.PGDestructiveTable
	if d == nil || d.IfExists || d.Cascade || d.RestartIdentity || len(d.Targets) != 1 || d.Targets[0].Only {
		t.Fatalf("explicit defaults=%+v", d)
	}
}
