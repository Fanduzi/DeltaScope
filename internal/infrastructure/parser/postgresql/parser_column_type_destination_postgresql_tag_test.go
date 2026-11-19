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
