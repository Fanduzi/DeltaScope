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

func TestResolveTablePersistence(t *testing.T) {
	failure := errors.New("read unavailable")
	for _, tc := range []struct {
		name, action, kind string
		row                []driver.Value
		want               spec.PGTablePersistence
		err                error
	}{
		{name: "out of scope", action: "set_logged"},
		{name: "partition root publication", action: "set_unlogged", kind: "p", row: []driver.Value{"p", true, false, false}, want: spec.PGTablePersistence{Current: "p", PartitionRoot: true, DirectPublication: true}},
		{name: "partition root logged out of scope", action: "set_logged", kind: "p"},
		{name: "same persistence", action: "set_logged", row: []driver.Value{"p", false, false, false}, want: spec.PGTablePersistence{Current: "p", Logged: true}},
		{name: "publication and inbound", action: "set_unlogged", row: []driver.Value{"p", true, true, false}, want: spec.PGTablePersistence{Current: "p", DirectPublication: true, ForeignKeyConflict: true}},
		{name: "outbound", action: "set_logged", row: []driver.Value{"u", false, true, false}, want: spec.PGTablePersistence{Current: "u", Logged: true, ForeignKeyConflict: true}},
		{name: "unobserved counterpart", action: "set_logged", row: []driver.Value{"u", false, false, true}, want: spec.PGTablePersistence{Current: "u", Logged: true, ForeignKeyUnknown: true}},
		{name: "query error", action: "set_logged", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"current", "direct", "conflict", "unknown"}, err: tc.err}
			if tc.row != nil {
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.pg_publication_rel": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: " app ", Name: " Exact Table ", Type: "table_persistence", Qualifiers: map[string]string{"action": tc.action, "kind": tc.kind}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if !errors.Is(err, tc.err) || (got != nil) != (tc.row != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			want := tc.want
			want.Schema, want.Table = req.Schema, req.Name
			if got.PGTablePersistence == nil || *got.PGTablePersistence != want {
				t.Fatalf("got=%+v want=%+v", got.PGTablePersistence, want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "con.conrelid OPERATOR(pg_catalog.<>) con.confrelid") ||
				!strings.Contains(q[0], "bool_or(peer.oid is null)") || !strings.Contains(q[0], "not c.relisshared") ||
				strings.Contains(q[0], req.Name) || strings.Contains(q[0], "conname") || strings.Contains(q[0], "pubname") {
				t.Fatalf("unexpected query: %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGTablePersistence") {
				t.Fatalf("private observation leaked: %s %v", b, err)
			}
		})
	}
}
