package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveTableAccessMethod(t *testing.T) {
	failure := errors.New("read unavailable")
	for _, tc := range []struct {
		name       string
		row        []driver.Value
		want       spec.PGTableAccessMethod
		err        error
		useDefault bool
	}{
		{name: "unobserved scope"},
		{name: "missing", row: []driver.Value{"", nil}},
		{name: "index kind", row: []driver.Value{"i", false}, want: spec.PGTableAccessMethod{Kind: "i", SameKnown: true}},
		{name: "same", row: []driver.Value{"t", true}, want: spec.PGTableAccessMethod{Kind: "t", SameKnown: true, Same: true}},
		{name: "different", row: []driver.Value{"t", false}, want: spec.PGTableAccessMethod{Kind: "t", SameKnown: true}},
		{name: "unknown current", row: []driver.Value{"t", nil}, want: spec.PGTableAccessMethod{Kind: "t"}},
		{name: "query error", err: failure},
		{name: "effective default", row: []driver.Value{"t", true}, useDefault: true, want: spec.PGTableAccessMethod{Kind: "t", Same: true, SameKnown: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"kind", "same"}, err: tc.err}
			if tc.row != nil {
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.current_setting('default_table_access_method', true)": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact Method ", Type: "table_access_method", Qualifiers: map[string]string{"table": "t", "is_default": strconv.FormatBool(tc.useDefault)}}
			if tc.useDefault {
				req.Name = "default"
			}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if !errors.Is(err, tc.err) || (got != nil) != (tc.row != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			want := tc.want
			want.Schema, want.Table, want.Name = req.Schema, req.Qualifiers["table"], req.Name
			want.Default = tc.useDefault
			if got.PGTableAccessMethod == nil || *got.PGTableAccessMethod != want {
				t.Fatalf("got=%+v want=%+v", got.PGTableAccessMethod, want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "then pg_catalog.current_setting('default_table_access_method', true)") ||
				!strings.Contains(q[0], "not c.relisshared") || !strings.Contains(q[0], "c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid") ||
				(!tc.useDefault && strings.Contains(q[0], req.Name)) || strings.Contains(q[0], "amhandler") {
				t.Fatalf("unexpected query: %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGTableAccessMethod") {
				t.Fatalf("private observation leaked: %s %v", b, err)
			}
		})
	}
}
