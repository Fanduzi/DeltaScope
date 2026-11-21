package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveRefreshMaterializedView(t *testing.T) {
	for _, tc := range []struct {
		name, kind             string
		populated, uniqueIndex bool
	}{
		{name: "absent"},
		{name: "ordinary view", kind: "v"},
		{name: "unpopulated with usable index", kind: "m", uniqueIndex: true},
		{name: "populated without usable index", kind: "m", populated: true},
		{name: "eligible", kind: "m", populated: true, uniqueIndex: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{
				"c.relispopulated": {columns: []string{"kind", "populated", "unique_index"}, rows: [][]driver.Value{{tc.kind, tc.populated, tc.uniqueIndex}}},
			})
			defer db.Close()
			target := spec.PGRefreshTarget{Schema: " Exact.Schema ", Name: " Exact View "}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{Type: "refresh_materialized_view", Schema: target.Schema, Name: target.Name})
			want := spec.PGRefreshState{Target: target, Kind: tc.kind, Populated: tc.populated, UniqueIndex: tc.uniqueIndex}
			if err != nil || got == nil || got.PGRefreshState == nil || *got.PGRefreshState != want {
				t.Fatalf("snapshot=%+v want=%+v err=%v", got, want, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || strings.Contains(queries[0], target.Name) || !strings.Contains(queries[0], "0 OPERATOR(pg_catalog.<) all(i.indkey)") {
				t.Fatalf("unexpected catalog query %v", queries)
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.Contains(string(encoded), target.Name) || strings.Contains(string(encoded), "PGRefreshState") {
				t.Fatalf("private refresh state appeared in %s (%v)", encoded, err)
			}
		})
	}
}
