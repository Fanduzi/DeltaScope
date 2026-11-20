//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeExactDestination(t *testing.T) {
	for _, tc := range []struct {
		sql       string
		want      spec.PGColumnTypeTarget
		qualified bool
	}{
		{sql: `integer`, want: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, qualified: true},
		{sql: `" Odd.Schema "."Exact Type"[][]`, want: spec.PGColumnTypeTarget{Schema: " Odd.Schema ", Name: "Exact Type", Array: true}, qualified: true},
		{sql: `pg_catalog.varchar(10)`, want: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "varchar"}, qualified: true},
		{sql: `pg_temp."Temp"`, want: spec.PGColumnTypeTarget{Schema: "pg_temp", Name: "Temp"}},
		{sql: `currentdb.app."Three-Part"`},
		{sql: `"Unqualified"`},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractStmt(t, "ALTER TABLE app.t ALTER COLUMN \"Id\" TYPE "+tc.sql)
			alter := s.DDL.Alter[0]
			if alter.Name != "Id" || alter.PGColumnType != tc.want || alter.PGColumnType.Qualified() != tc.qualified {
				t.Fatalf("destination: %+v", alter)
			}
			encoded, err := json.Marshal(s.DDL)
			if err != nil || strings.Contains(string(encoded), "PGColumnType") || strings.Contains(string(encoded), "Exact Type") {
				t.Fatalf("private destination leaked: %s %v", encoded, err)
			}
		})
	}
}

func TestPGColumnTypeModifiersAndCollation(t *testing.T) {
	s := extractStmt(t, `ALTER TABLE app.t ALTER COLUMN v TYPE pg_catalog.numeric(8,-2) COLLATE " Exact Schema "." Exact Collation " USING v`)
	a := s.DDL.Alter[0]
	if a.PGColumnTypeModifiers == nil || *a.PGColumnTypeModifiers != (spec.PGColumnTypeModifiers{Count: 2, Values: [2]int32{8, -2}, Known: true}) {
		t.Fatalf("type modifier facts=%+v", a.PGColumnTypeModifiers)
	}
	if a.PGColumnCollation == nil || *a.PGColumnCollation != (spec.PGColumnCollation{Schema: " Exact Schema ", Name: " Exact Collation "}) || a.Options["has_using"] != "true" {
		t.Fatalf("collation facts=%+v", a.PGColumnCollation)
	}
	payload, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(payload), "PGColumnTypeModifiers") || strings.Contains(string(payload), "Exact Collation") {
		t.Fatalf("private declaration appeared in output: %s (%v)", payload, err)
	}
}
