package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveTableTablespace(t *testing.T) {
	failure := errors.New("read unavailable")
	for _, tc := range []struct {
		name string
		row  []driver.Value
		want spec.PGTableTablespace
		err  error
	}{
		{name: "unobserved scope"},
		{name: "missing", row: []driver.Value{false, false, nil}},
		{name: "global", row: []driver.Value{true, true, false}, want: spec.PGTableTablespace{Exists: true, Global: true, SameKnown: true}},
		{name: "same", row: []driver.Value{true, false, true}, want: spec.PGTableTablespace{Exists: true, SameKnown: true, Same: true}},
		{name: "different", row: []driver.Value{true, false, false}, want: spec.PGTableTablespace{Exists: true, SameKnown: true}},
		{name: "unknown current", row: []driver.Value{true, false, nil}, want: spec.PGTableTablespace{Exists: true}},
		{name: "query error", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"exists", "global", "same"}, err: tc.err}
			if tc.row != nil {
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.current_database()": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact Space ", Type: "table_tablespace", Qualifiers: map[string]string{"table": "t"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if !errors.Is(err, tc.err) || (got != nil) != (tc.row != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			want := tc.want
			want.Schema, want.Table, want.Name = req.Schema, req.Qualifiers["table"], req.Name
			if got.PGTableTablespace == nil || *got.PGTableTablespace != want {
				t.Fatalf("got=%+v want=%+v", got.PGTableTablespace, want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "then db.dattablespace else c.reltablespace end") ||
				!strings.Contains(q[0], "not c.relisshared") || !strings.Contains(q[0], "c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid") ||
				strings.Contains(q[0], req.Name) || strings.Contains(q[0], "pg_tablespace_location") {
				t.Fatalf("unexpected query: %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGTableTablespace") {
				t.Fatalf("private observation leaked: %s %v", b, err)
			}
		})
	}
}
