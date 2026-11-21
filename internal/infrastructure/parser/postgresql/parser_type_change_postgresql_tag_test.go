//go:build postgresql

package postgresql

import "testing"

func TestPGTypeChangeIdentity(t *testing.T) {
	for _, tc := range []struct {
		sql, action, destination string
		ownerKind, ownerName     string
	}{
		{sql: `ALTER TYPE " A "." Pair " RENAME TO " X "`, action: "rename", destination: " X "},
		{sql: `ALTER TYPE " A "." Pair " SET SCHEMA " B "`, action: "set_schema", destination: " B "},
		{sql: `ALTER TYPE " A "." Pair " OWNER TO " R "`, action: "set_owner", ownerKind: "named", ownerName: " R "},
		{sql: `ALTER TYPE " A "." Pair " OWNER TO CURRENT_ROLE`, action: "set_owner", ownerKind: "current_role"},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		if s.DDL == nil || s.DDL.PGTypeChange == nil {
			t.Fatalf("%s: missing type change", tc.sql)
		}
		d := s.DDL.PGTypeChange
		if d.Source.Table.Schema != " A " || d.Source.Table.Name != " Pair " || d.Action != tc.action || d.Destination != tc.destination ||
			d.Owner.Kind != tc.ownerKind || d.Owner.Name != tc.ownerName {
			t.Fatalf("%s: unexpected declaration %+v", tc.sql, d)
		}
	}
}
