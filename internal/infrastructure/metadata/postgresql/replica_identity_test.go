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

func TestResolveReplicaIdentityIndex(t *testing.T) {
	failure := errors.New("read failed")
	for _, tc := range []struct {
		name string
		rows [][]driver.Value
		err  error
		want *spec.PGReplicaIdentityIndex
	}{
		{name: "unobserved table"},
		{name: "absent index", rows: [][]driver.Value{{"", false, false, nil, false, false, false, false, nil}}, want: &spec.PGReplicaIdentityIndex{}},
		{name: "wrong kind", rows: [][]driver.Value{{"v", false, false, nil, false, false, false, false, nil}}, want: &spec.PGReplicaIdentityIndex{Kind: "v"}},
		{name: "wrong owner", rows: [][]driver.Value{{"i", true, false, true, true, true, false, false, nil}}, want: &spec.PGReplicaIdentityIndex{Kind: "i", IndexKnown: true, CanUniqueKnown: true, CanUnique: true, Unique: true, Immediate: true}},
		{name: "unknown access method", rows: [][]driver.Value{{"i", true, true, nil, true, true, false, false, true}}, want: &spec.PGReplicaIdentityIndex{Kind: "i", IndexKnown: true, OnTable: true, Unique: true, Immediate: true, KeyAttributesKnown: true, KeyAttributesValid: true}},
		{name: "nullable key", rows: [][]driver.Value{{"i", true, true, true, true, true, false, false, false}}, want: &spec.PGReplicaIdentityIndex{Kind: "i", IndexKnown: true, OnTable: true, CanUniqueKnown: true, CanUnique: true, Unique: true, Immediate: true, KeyAttributesKnown: true}},
		{name: "error", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"pg_indexam_has_property": {
				columns: []string{"kind", "index_known", "on_table", "can_unique", "unique", "immediate", "expression", "partial", "keys"}, rows: tc.rows, err: tc.err,
			}})
			defer db.Close()
			request := spec.ObjectLookupRequest{Schema: "app", Type: "replica_identity_index", Name: " exact index ", Qualifiers: map[string]string{"table": "t"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, request)
			if !errors.Is(err, tc.err) {
				t.Fatalf("error=%v want=%v", err, tc.err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("unexpected observation: %+v", got)
				}
				return
			}
			want := *tc.want
			want.Schema, want.Table, want.Name = request.Schema, request.Qualifiers["table"], request.Name
			if got == nil || got.PGReplicaIdentity == nil || *got.PGReplicaIdentity != want {
				t.Fatalf("snapshot=%+v, want=%+v", got, want)
			}
			queries := log.Queries()
			if len(queries) != 1 || !strings.Contains(queries[0], "k.position OPERATOR(pg_catalog.<=) i.indnkeyatts") ||
				!strings.Contains(queries[0], "idx.relam OPERATOR(pg_catalog.=) 403::pg_catalog.oid") ||
				strings.Contains(queries[0], "pg_get_indexdef") || strings.Contains(queries[0], "indisvalid") || strings.Contains(queries[0], request.Name) {
				t.Fatalf("unexpected query shape: %q", queries)
			}
			data, err := json.Marshal(got)
			if err != nil || strings.Contains(string(data), request.Name) || strings.Contains(string(data), "PGReplicaIdentity") {
				t.Fatalf("private observation leaked: %s %v", data, err)
			}
		})
	}
}
