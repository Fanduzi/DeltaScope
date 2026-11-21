//go:build postgresql

package postgresql

import (
	"slices"
	"testing"
)

func TestPGCheckNullTestBinding(t *testing.T) {
	for _, tc := range []struct {
		expression string
		want       []string
	}{
		{`" Exact Column " IS NULL`, []string{" Exact Column "}},
		{`" Exact Table ".c IS NOT NULL`, []string{" Exact Table ", "c"}},
		{`t.* IS NOT NULL`, nil},
		{`app.t.c IS NOT NULL`, nil},
		{`(c + 1) IS NULL`, nil},
	} {
		s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD CHECK (`+tc.expression+`) NOT VALID`)
		if s.DDL == nil || len(s.DDL.Alter) != 1 || !slices.Equal(s.DDL.Alter[0].PGCheckNullTest, tc.want) {
			t.Fatalf("%s: expected identifiers %q, got %+v", tc.expression, tc.want, s.DDL)
		}
	}
}
