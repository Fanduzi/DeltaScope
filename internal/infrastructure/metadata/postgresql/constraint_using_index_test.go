package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveConstraintUsingIndex(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows [][]driver.Value
		want *spec.PGConstraintUsingIndex
	}{
		{name: "unobserved table"},
		{name: "missing index", rows: [][]driver.Value{{"", false, false, false, false, false, false, false, false, false, nil}}, want: &spec.PGConstraintUsingIndex{}},
		{name: "eligible shape", rows: [][]driver.Value{{"i", true, true, true, true, false, false, true, true, false, true}}, want: &spec.PGConstraintUsingIndex{Kind: "i", IndexKnown: true, OnTable: true, Valid: true, Unique: true, BTree: true, NullsNotDistinct: true, KeyDefaults: true, KeyDefaultsKnown: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.bool_and(slots.flags": {columns: []string{"kind", "index", "on_table", "valid", "unique", "expr", "partial", "btree", "nnd", "owned", "key_defaults"}, rows: tc.rows}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact.Index ", Type: "constraint_using_index", Qualifiers: map[string]string{"table": "t"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (got == nil) != (tc.want == nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			tc.want.Schema, tc.want.Table, tc.want.Name = req.Schema, req.Qualifiers["table"], req.Name
			if got.PGConstraintUsingIndex == nil || *got.PGConstraintUsingIndex != *tc.want {
				t.Fatalf("observation=%+v want=%+v", got.PGConstraintUsingIndex, tc.want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "dep.deptype OPERATOR(pg_catalog.=) 'i'") || !strings.Contains(q[0], "slots.slot OPERATOR(pg_catalog.<=) i.indnkeyatts") || strings.Contains(q[0], "conindid") || strings.Contains(q[0], req.Name) {
				t.Fatalf("unexpected query %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGConstraintUsingIndex") {
				t.Fatalf("private facts leaked: %s %v", b, err)
			}
		})
	}
}
