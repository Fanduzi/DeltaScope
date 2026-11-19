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

func TestResolveTableOwner(t *testing.T) {
	failure := errors.New("read unavailable")
	for _, tc := range []struct {
		name string
		row  []driver.Value
		want spec.PGTableOwnerState
		err  error
		kind string
	}{
		{name: "unobserved scope", kind: "named"},
		{name: "missing literal", kind: "named", row: []driver.Value{false, false}},
		{name: "different literal", kind: "named", row: []driver.Value{true, false}, want: spec.PGTableOwnerState{Exists: true}},
		{name: "effective owner", kind: "current_role", row: []driver.Value{true, true}, want: spec.PGTableOwnerState{Exists: true, Same: true}},
		{name: "session owner", kind: "session_user", row: []driver.Value{true, true}, want: spec.PGTableOwnerState{Exists: true, Same: true}},
		{name: "public", kind: "public", row: []driver.Value{false, false}},
		{name: "query error", kind: "named", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"exists", "same"}, err: tc.err}
			if tc.row != nil {
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.pg_roles role": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact Owner ", Type: "table_owner", Qualifiers: map[string]string{"table": "t", "kind": tc.kind}}
			if tc.kind != "named" {
				req.Name = ""
			}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if !errors.Is(err, tc.err) || (got != nil) != (tc.row != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			want := tc.want
			want.Schema, want.Table = req.Schema, req.Qualifiers["table"]
			want.Target = spec.PGTableOwnerTarget{Kind: tc.kind, Name: req.Name}
			if got.PGTableOwner == nil || *got.PGTableOwner != want {
				t.Fatalf("got=%+v want=%+v", got.PGTableOwner, want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "then session_user::pg_catalog.text") ||
				!strings.Contains(q[0], "not c.relisshared") || !strings.Contains(q[0], "c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid") ||
				(req.Name != "" && strings.Contains(q[0], req.Name)) || strings.Contains(q[0], "pg_authid") || strings.Contains(q[0], "pg_auth_members") {
				t.Fatalf("unexpected query: %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || req.Name != "" && strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGTableOwner") {
				t.Fatalf("private observation leaked: %s %v", b, err)
			}
		})
	}
}
