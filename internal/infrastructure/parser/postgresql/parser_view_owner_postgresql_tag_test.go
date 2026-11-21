//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewOwnerRetainsExactASTFacts(t *testing.T) {
	for _, tc := range []struct {
		sql, kind string
		guard     bool
		owner     spec.PGTableOwnerTarget
	}{
		{`ALTER VIEW IF EXISTS " Exact Schema "." Exact View " OWNER TO " Exact Role "`, "v", true, spec.PGTableOwnerTarget{Kind: "named", Name: " Exact Role "}},
		{`ALTER MATERIALIZED VIEW " Exact Schema "." Exact View " OWNER TO SESSION_USER`, "m", false, spec.PGTableOwnerTarget{Kind: "session_user"}},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		owner, kind := spec.PGViewOwnerSelection(s)
		lookup := spec.PGViewOwnerLookup(s)
		if owner != tc.owner || kind != tc.kind || lookup == nil || lookup.Schema != " Exact Schema " ||
			lookup.Qualifiers["table"] != " Exact View " || s.DDL.PGTableIfExists != tc.guard {
			t.Fatalf("statement=%+v lookup=%+v", s.DDL, lookup)
		}
		data, err := json.Marshal(s.DDL)
		if err != nil || strings.Contains(string(data), "PGTableOwner") || strings.Contains(string(data), " Exact Role ") {
			t.Fatalf("public DDL=%s (%v)", data, err)
		}
	}
}
