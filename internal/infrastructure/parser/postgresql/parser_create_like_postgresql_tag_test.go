//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGCreateLikeRetainsOrderedSourcesAndEffectiveOptions(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE TABLE IF NOT EXISTS app.copy (LIKE app." Exact Source " INCLUDING DEFAULTS EXCLUDING DEFAULTS, LIKE other.t INCLUDING IDENTITY)`)
	if s.DDL == nil || len(s.DDL.PGCreateLike) != 2 || s.DDL.Options["if_not_exists"] != "true" {
		t.Fatalf("unexpected CREATE TABLE LIKE declaration: %+v", s.DDL)
	}
	first, second := s.DDL.PGCreateLike[0], s.DDL.PGCreateLike[1]
	if first.Target.Table.Schema != "app" || first.Target.Table.Name != " Exact Source " || first.Options != 0 ||
		second.Target.Table.Schema != "other" || second.Target.Table.Name != "t" || second.Options == 0 {
		t.Fatalf("unexpected LIKE sources: %+v", s.DDL.PGCreateLike)
	}
	body, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(body), "PGCreateLike") || strings.Contains(string(body), " Exact Source ") {
		t.Fatalf("private LIKE data in DDL: %s (%v)", body, err)
	}
}
