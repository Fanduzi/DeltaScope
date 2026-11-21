package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveViewOwnerBindsObservedSourceAndRole(t *testing.T) {
	for _, tc := range []struct {
		name, wantKind string
		kind           driver.Value
		exists         bool
	}{
		{name: "absent"},
		{name: "wrong kind", kind: "r", wantKind: "r"},
		{name: "missing literal role", kind: "m", wantKind: "m"},
		{name: "present literal role", kind: "m", wantKind: "m", exists: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{
				"select c.relkind::pg_catalog.text": {columns: []string{"relkind", "owner_exists", "same_owner"}, rows: [][]driver.Value{{tc.kind, tc.exists, false}}},
			})
			defer db.Close()
			request := spec.ObjectLookupRequest{Type: "view_owner", Schema: " Exact Schema ", Name: " Private Role ",
				Qualifiers: map[string]string{"table": " Exact View ", "kind": "named", "relation_kind": "m"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, request)
			want := spec.PGTableOwnerState{Schema: request.Schema, Table: request.Qualifiers["table"], Target: spec.PGTableOwnerTarget{Kind: "named", Name: request.Name},
				SourceKind: tc.wantKind, SourceKindKnown: true, Exists: tc.exists}
			if err != nil || got == nil || got.PGTableOwner == nil || *got.PGTableOwner != want {
				t.Fatalf("snapshot = %+v, want = %+v, err = %v", got, want, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || strings.Contains(queries[0], request.Name) || !strings.Contains(queries[0], `c.relkind OPERATOR(pg_catalog.=) $5::pg_catalog."char"`) {
				t.Fatalf("query = %v", queries)
			}
			data, err := json.Marshal(got)
			if err != nil || strings.Contains(string(data), request.Name) || strings.Contains(string(data), "SourceKind") {
				t.Fatalf("public snapshot = %s (%v)", data, err)
			}
		})
	}
}
