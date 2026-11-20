package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveDropConstraintReference(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    [][]driver.Value
		blocked bool
	}{
		{name: "observed dependency", rows: [][]driver.Value{{true}}, blocked: true},
		{name: "no direct dependency", rows: [][]driver.Value{{false}}},
		{name: "unobserved key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"from pg_catalog.pg_depend usage": {columns: []string{"blocked"}, rows: tc.rows}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: " Exact.Schema ", Type: "drop_constraint_reference", Name: " Exact Key ", Qualifiers: map[string]string{"table": " Ref ", "kind": "p"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (got != nil) != (tc.rows != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			q := log.Queries()
			if len(q) != 1 || strings.Contains(q[0], req.Name) {
				t.Fatalf("queries=%v", q)
			}
			for _, predicate := range []string{"usage.deptype OPERATOR(pg_catalog.=) 'n'", "owner.deptype OPERATOR(pg_catalog.=) 'i'", "owner.refobjid OPERATOR(pg_catalog.=) c.oid", "fk.conindid OPERATOR(pg_catalog.=) idx.indexrelid"} {
				if !strings.Contains(q[0], predicate) {
					t.Fatalf("missing exact dependency predicate %q", predicate)
				}
			}
			if got != nil {
				want := spec.PGDropConstraintReference{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Kind: "p", Blocked: tc.blocked}
				if got.PGDropConstraintReference == nil || *got.PGDropConstraintReference != want {
					t.Fatalf("observation=%+v want=%+v", got.PGDropConstraintReference, want)
				}
				b, err := json.Marshal(got)
				if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGDropConstraintReference") {
					t.Fatalf("private dependency leaked: %s (%v)", b, err)
				}
			}
		})
	}
}
