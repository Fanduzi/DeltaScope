//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateColumnFacts(t *testing.T) {
	ordinary := extractPostgreSQLStatement(t, `CREATE TABLE app.t (id integer, body varchar(40), tags text[])`)
	if ordinary.DDL == nil || ordinary.DDL.PGCreateColumns != nil {
		t.Fatalf("ordinary types should need no catalog check: %+v", ordinary.DDL)
	}
	typed := extractPostgreSQLStatement(t, `CREATE TABLE IF NOT EXISTS app.t (payload " Type Schema "." Pair "[], special pg_catalog.cstring)`)
	if typed.DDL == nil || typed.DDL.PGCreateColumns == nil {
		t.Fatalf("column facts missing: %+v", typed.DDL)
	}
	facts := typed.DDL.PGCreateColumns
	if !facts.IfNotExists || len(facts.Types) != 2 || facts.Types[0] != (spec.PGColumnTypeTarget{Schema: " Type Schema ", Name: " Pair ", Array: true}) ||
		facts.Types[1] != (spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "cstring"}) {
		t.Fatalf("unexpected column type targets: %+v", facts)
	}
	duplicate := extractPostgreSQLStatement(t, `CREATE TABLE app.t (ctid integer, " X " integer, " X " text)`)
	if duplicate.DDL == nil || duplicate.DDL.PGCreateColumns == nil || duplicate.DDL.PGCreateColumns.Refusal != "create_column_duplicate" {
		t.Fatalf("duplicate should precede system name: %+v", duplicate.DDL)
	}
}
