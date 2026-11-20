package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolvePGDestructiveReferences(t *testing.T) {
	targets := []spec.PGDestructiveTableTarget{
		{Table: spec.Table{Schema: "app", Name: " Exact Target "}},
		{Table: spec.Table{Schema: " Exact.Schema ", Name: "second"}, Only: true},
	}
	for _, tc := range []struct {
		name                string
		value, onlyPosition driver.Value
		known               bool
		position            int
	}{
		{name: "outside reference", value: int64(2), onlyPosition: int64(0), known: true, position: 2},
		{name: "known empty", value: int64(0), onlyPosition: int64(0), known: true},
		{name: "partitioned ONLY target", value: int64(0), onlyPosition: int64(2), known: true},
		{name: "undefined set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"pg_catalog.jsonb_to_recordset": {columns: []string{"position", "only_position"}, rows: [][]driver.Value{{tc.value, tc.onlyPosition}}}})
			defer db.Close()
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{Type: "truncate_references", PGDestructiveTargets: targets})
			if err != nil || (got != nil) != tc.known {
				t.Fatalf("observation=%+v error=%v", got, err)
			}
			query := log.Queries()
			if len(query) != 1 || strings.Contains(query[0], targets[0].Table.Name) ||
				!strings.Contains(query[0], "m.oid OPERATOR(pg_catalog.=) fk.conrelid") ||
				!strings.Contains(query[0], "not h.inhdetachpending") || !strings.Contains(query[0], "c.conparentid") {
				t.Fatal(query)
			}
			if got == nil {
				return
			}
			if f := got.PGDestructiveReferences; f == nil || !f.Matches(targets) || f.BlockedPosition != tc.position || f.IneligibleOnlyPosition != int(tc.onlyPosition.(int64)) {
				t.Fatalf("fact=%+v", f)
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.Contains(string(encoded), "BlockedPosition") || strings.Contains(string(encoded), targets[0].Table.Name) {
				t.Fatalf("private result: %s (%v)", encoded, err)
			}
		})
	}
}
