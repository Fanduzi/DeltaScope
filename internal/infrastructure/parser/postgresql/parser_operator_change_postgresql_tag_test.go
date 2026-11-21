//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGOperatorEstimatorOrder(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER OPERATOR app.=== (pg_catalog.int4, pg_catalog.int4) SET (RESTRICT = app.first, JOIN = app.second, RESTRICT = NONE)`)
	if s.DDL == nil || s.DDL.PGOperatorChange == nil {
		t.Fatal("missing ALTER OPERATOR declaration")
	}
	decl := s.DDL.PGOperatorChange
	options := s.DDL.PGOperatorEstimatorOptions
	if decl.Action != "set_estimators" || decl.Restrict != (spec.Table{}) || decl.Join != (spec.Table{Schema: "app", Name: "second"}) ||
		len(options) != 3 || options[0].Function.Name != "first" || options[2].Kind != "restrict" || options[2].Function != (spec.Table{}) {
		t.Fatalf("unexpected declaration %+v and options %+v", decl, options)
	}
}

func TestPGOperatorChangeSignatures(t *testing.T) {
	for _, tc := range []struct {
		sql, action, destination, owner string
		unary, array, exact             bool
	}{
		{sql: `ALTER OPERATOR " Op Schema ".=== (NONE, pg_catalog.int4) OWNER TO CURRENT_ROLE`, action: "set_owner", owner: "current_role", unary: true, exact: true},
		{sql: `ALTER OPERATOR " Op Schema ".=== (pg_catalog.int4, pg_catalog.int4[]) SET SCHEMA " Dest "`, action: "set_schema", destination: " Dest ", array: true, exact: true},
		{sql: `ALTER OPERATOR " Op Schema ".=== (pg_catalog.int4, custom) SET SCHEMA " Dest "`, action: "set_schema", destination: " Dest "},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		if s.DDL == nil || s.DDL.PGOperatorChange == nil {
			t.Fatalf("missing operator declaration for %q", tc.sql)
		}
		decl := s.DDL.PGOperatorChange
		if decl.Source.Table != (spec.Table{Schema: " Op Schema ", Name: "==="}) || decl.Unary != tc.unary ||
			decl.Signature != tc.exact || decl.Action != tc.action || decl.Destination != tc.destination ||
			decl.Owner.Kind != tc.owner || decl.Right.Array != tc.array {
			t.Fatalf("%s: %+v", tc.sql, decl)
		}
	}
}
