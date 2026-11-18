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

func TestLoadTableSnapshotObservesAllPGRelationKinds(t *testing.T) {
	for _, tc := range []struct {
		name, schema, kind string
		queryErr           error
		known              bool
	}{
		{name: "absent", schema: "app", known: true},
		{name: "sequence", schema: "app", kind: "S", known: true},
		{name: "index", schema: "app", kind: "i", known: true},
		{name: "composite", schema: "app", kind: "c", known: true},
		{name: "temporary alias", schema: "pg_temp"},
		{name: "unbound schema"},
		{name: "catalog error", schema: "app", queryErr: errors.New("read failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"relkind", "table_comment", "reltuples", "heap"}, err: tc.queryErr}
			if tc.kind != "" {
				result.rows = [][]driver.Value{{tc.kind, nil, nil, false}}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.obj_description": result})
			defer db.Close()
			s, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectPostgreSQL, tc.schema, " exact table ")
			if tc.queryErr != nil {
				if !errors.Is(err, tc.queryErr) || s != nil {
					t.Fatalf("snapshot=%+v err=%v", s, err)
				}
				return
			}
			if err != nil || s == nil || s.Exists || s.Table != nil || s.Schema != tc.schema {
				t.Fatalf("legacy table shape changed: %+v err=%v", s, err)
			}
			var want spec.PGRelationState
			if tc.known {
				want = spec.PGRelationState{Name: " exact table ", Kind: tc.kind}
			}
			if s.PGRelation != want {
				t.Fatalf("relation=%+v want=%+v", s.PGRelation, want)
			}
			queries := log.Queries()
			if len(queries) != 1 || strings.Count(queries[0], "OPERATOR(pg_catalog.=)") != 4 || strings.Contains(queries[0], "relkind in") {
				t.Fatalf("unexpected catalog query: %v", queries)
			}
			encoded, err := json.Marshal(s)
			if err != nil || strings.Contains(string(encoded), " exact table ") || strings.Contains(string(encoded), "PGRelation") {
				t.Fatalf("private observation leaked: %s %v", encoded, err)
			}
		})
	}
}
