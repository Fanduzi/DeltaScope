//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGCheckLiteralClassification(t *testing.T) {
	for _, tc := range []struct{ predicate, class string }{
		{"42", "integer"},
		{"'private_invalid_input'", "invalid_boolean"},
		{"'  Yes  '", ""},
		{"'of'", ""},
		{"'o'", "invalid_boolean"},
		{"NULL", ""},
		{"FALSE", ""},
		{"1.5", "numeric"},
		{"1e2", "numeric"},
		{"1e1000000", ""},
		{"-2147483648", "integer"},
		{"2147483648", "bigint"},
		{"-2147483649", "bigint"},
		{"9223372036854775808", "numeric"},
		{"0x80000000", "bigint"},
		{"2_147_483_648", "bigint"},
		{"0002147483648", "bigint"},
		{"id", ""},
	} {
		t.Run(tc.predicate, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, "ALTER TABLE app.t ADD CHECK ("+tc.predicate+") NOT VALID")
			if s.DDL == nil || len(s.DDL.Alter) != 1 || s.DDL.Alter[0].PGCheckLiteral != tc.class {
				t.Fatalf("expected class %q, got %#v", tc.class, s.DDL)
			}
			body, err := json.Marshal(s.DDL)
			if err != nil || strings.Contains(string(body), "PGCheckLiteral") || strings.Contains(string(body), "private_invalid_input") {
				t.Fatalf("private CHECK input in DDL output: %s (%v)", body, err)
			}
		})
	}
}
