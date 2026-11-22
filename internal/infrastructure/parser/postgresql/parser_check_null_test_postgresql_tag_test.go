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
		wholeRow   bool
	}{
		{`" Exact Column " IS NULL`, []string{" Exact Column "}, false},
		{`" Exact Table ".c IS NOT NULL`, []string{" Exact Table ", "c"}, false},
		{`t.* IS NOT NULL`, []string{"t"}, true},
		{`app.t.* IS NULL`, []string{"app", "t"}, true},
		{`app.t.c IS NOT NULL`, []string{"app", "t", "c"}, false},
		{`postgres.app.t.c IS NOT NULL`, []string{"postgres", "app", "t", "c"}, false},
		{`(c + 1) IS NULL`, nil, false},
	} {
		s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD CHECK (`+tc.expression+`) NOT VALID`)
		if s.DDL == nil || len(s.DDL.Alter) != 1 || !slices.Equal(s.DDL.Alter[0].PGCheckNullTest, tc.want) || s.DDL.Alter[0].PGCheckNullTestWholeRow != tc.wholeRow {
			t.Fatalf("%s: expected identifiers %q, got %+v", tc.expression, tc.want, s.DDL)
		}
	}
}
