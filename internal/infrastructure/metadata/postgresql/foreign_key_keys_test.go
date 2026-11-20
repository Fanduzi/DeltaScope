package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveForeignKeyKeys(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rows  [][]driver.Value
		known bool
		keys  []spec.PGForeignKeyKey
	}{
		{name: "table outside scope"},
		{name: "known empty", rows: [][]driver.Value{{false, nil, nil, nil, nil}}, known: true},
		{name: "exact ordered keys", rows: [][]driver.Value{
			{true, true, false, int64(1), []byte(`[" PK "]`)},
			{true, false, true, int64(2), []byte(`["A,B", "c"]`)},
		}, known: true, keys: []spec.PGForeignKeyKey{
			{Columns: []string{" PK "}, Primary: true}, {Columns: []string{"A,B", "c"}, Immediate: true},
		}},
		{name: "missing column", rows: [][]driver.Value{{true, false, true, int64(2), []byte(`["a", null]`)}}},
		{name: "incomplete positions", rows: [][]driver.Value{{true, false, true, int64(2), []byte(`["a"]`)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.jsonb_agg(a.attname": {
				columns: []string{"index", "primary", "immediate", "width", "keys"}, rows: tc.rows,
			}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: " Exact.Schema ", Type: "foreign_key_reference_keys", Name: " Ref "}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (got != nil) != tc.known {
				t.Fatalf("keys=%+v err=%v", got, err)
			}
			q := log.Queries()
			if len(q) != 1 {
				t.Fatalf("queries=%v", q)
			}
			for _, fragment := range []string{"i.indislive and i.indisvalid and i.indisunique", "i.indpred is null and i.indexprs is null", "k.slot OPERATOR(pg_catalog.<=) i.indnkeyatts", "tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text"} {
				if !strings.Contains(q[0], fragment) {
					t.Fatalf("missing key condition %q", fragment)
				}
			}
			if got != nil {
				want := &spec.PGForeignKeyKeys{Schema: req.Schema, Table: req.Name, Keys: tc.keys}
				if !reflect.DeepEqual(got.PGForeignKeyKeys, want) {
					t.Fatalf("keys=%+v want=%+v", got.PGForeignKeyKeys, want)
				}
				b, err := json.Marshal(got)
				if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGForeignKeyKeys") {
					t.Fatalf("private keys leaked: %s (%v)", b, err)
				}
			}
		})
	}
}
