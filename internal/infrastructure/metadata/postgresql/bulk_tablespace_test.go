package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveBulkTablespace(t *testing.T) {
	for _, tc := range []struct {
		name       string
		roleResult string
		roles      []spec.PGTableOwnerTarget
		want       []bool
	}{
		{name: "no filter", roleResult: "[]", want: []bool{}},
		{name: "ordered observations", roleResult: "[true,false,true]", roles: []spec.PGTableOwnerTarget{{Kind: "named", Name: " Exact Alias "}, {Kind: "public"}, {Kind: "session_user"}}, want: []bool{true, false, true}},
		{name: "partial observation", roleResult: "[false]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.jsonb_agg": {
				columns: []string{"source", "destination", "source_global", "destination_global", "same", "roles"},
				rows:    [][]driver.Value{{true, false, true, false, false, []byte(tc.roleResult)}},
			}})
			defer db.Close()
			target := spec.PGBulkTablespaceTarget{Source: "pg_global", Destination: " Missing Space ", Roles: tc.roles, Nowait: true}
			req := spec.ObjectLookupRequest{Type: "bulk_tablespace", PGBulkTablespace: &target}
			object, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (object != nil) != (tc.want != nil) {
				t.Fatalf("observation = %+v, error = %v", object, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || !strings.Contains(queries[0], "order by authored.ordinality") || !strings.Contains(queries[0], "session_user::pg_catalog.text") || strings.Contains(queries[0], " Missing Space ") || strings.Contains(queries[0], " Exact Alias ") || strings.Contains(queries[0], "pg_class") {
				t.Fatalf("unbound query: %v", queries)
			}
			if object == nil {
				return
			}
			got := object.PGBulkTablespace
			if got == nil || !got.Target.Matches(target) || !got.SourceExists || got.DestinationExists || !got.SourceGlobal || got.DestinationGlobal || got.Same || !slices.Equal(got.RoleExists, tc.want) {
				t.Fatalf("observation = %+v", got)
			}
			b, err := json.Marshal(object)
			if err != nil || strings.Contains(string(b), "PGBulkTablespace") || strings.Contains(string(b), target.Destination) {
				t.Fatalf("private facts leaked: %s, %v", b, err)
			}
		})
	}
}
