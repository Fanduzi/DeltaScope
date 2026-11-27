package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolvePGColumnTraversal(t *testing.T) {
	for _, tc := range []struct {
		name, action string
		only         bool
		rows         [][]driver.Value
		want         *spec.PGColumnTraversal
	}{
		{name: "unobserved attribute"},
		{name: "partition blocks identity change before column lookup", action: "drop_identity", only: true, rows: [][]driver.Value{{false, false, false, false, false, false, false, false, false, int64(0), nil}}, want: &spec.PGColumnTraversal{IdentityChangeBlocked: true}},
		{name: "validated exact null test", action: "set_not_null", rows: [][]driver.Value{{false, false, false, false, true, false, false, false, false, int64(0), nil}}, want: &spec.PGColumnTraversal{ColumnExists: true, ValidatedNotNullCheck: true}},
		{name: "recursive rename collision", action: "rename_column", rows: [][]driver.Value{{false, false, true, false, true, false, false, false, false, int64(0), true}}, want: &spec.PGColumnTraversal{AttachedChildren: true, ColumnExists: true, RenameDestination: " Exact Destination ", RenameDescendantsKnown: true, RenameDescendantConflict: true}},
		{name: "recursive rename catalog is clear", action: "rename_column", rows: [][]driver.Value{{false, false, true, false, true, false, false, false, false, int64(0), false}}, want: &spec.PGColumnTraversal{AttachedChildren: true, ColumnExists: true, RenameDestination: " Exact Destination ", RenameDescendantsKnown: true}},
		{name: "known local", rows: [][]driver.Value{{false, false, false, false, true, false, false, false, false, int64(0), nil}}, want: &spec.PGColumnTraversal{ColumnExists: true}},
		{name: "inherited attribute", rows: [][]driver.Value{{true, false, false, false, true, false, true, false, false, int64(0), nil}}, want: &spec.PGColumnTraversal{Inherited: true, ColumnExists: true, StoredGenerated: true}},
		{name: "attached partitioned key", rows: [][]driver.Value{{false, true, true, false, true, false, false, false, false, int64(0), nil}}, want: &spec.PGColumnTraversal{DirectPartitionKey: true, AttachedChildren: true, ColumnExists: true}},
		{name: "attached partition inherits nullability", action: "drop_not_null", rows: [][]driver.Value{{true, false, false, true, true, false, false, true, false, int64(0), nil}}, want: &spec.PGColumnTraversal{Inherited: true, Partition: true, ColumnExists: true, ParentNotNull: true}},
		{name: "partitioned table has nullable descendant", action: "set_not_null", only: true, rows: [][]driver.Value{{false, false, true, false, true, false, false, false, true, int64(0), nil}}, want: &spec.PGColumnTraversal{AttachedChildren: true, ColumnExists: true, NullableDescendant: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.rows {
				tc.rows[i] = append(tc.rows[i], tc.want.ValidatedNotNullCheck, tc.want.IdentityChangeBlocked)
			}
			db, log := openTestDB(t, map[string]testQueryResult{"a.attinhcount": {columns: []string{"inherited", "partition_key", "children", "partition", "present", "system", "generated", "parent_not_null", "nullable_descendant", "rename_destination_kind", "rename_conflict", "validated_not_null_check", "identity_change_blocked"}, rows: tc.rows}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Type: "column_traversal", Schema: " Exact Schema ", Name: " Exact Column ", Qualifiers: map[string]string{"table": " Exact Table ", "action": tc.action}}
			if tc.action == "rename_column" {
				req.Qualifiers["destination"] = " Exact Destination "
			}
			if tc.only {
				req.Qualifiers["only"] = "true"
			}
			result, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (result == nil) != (tc.want == nil) {
				t.Fatalf("fact=%+v err=%v", result, err)
			}
			query := log.Queries()
			if len(query) != 1 || strings.Contains(query[0], req.Name) || !strings.Contains(query[0], "not h.inhdetachpending") || !strings.Contains(query[0], "any(pt.partattrs)") {
				t.Fatal(query)
			}
			if result == nil {
				return
			}
			want := *tc.want
			want.Schema, want.Table, want.Column = req.Schema, req.Qualifiers["table"], req.Name
			if result.PGColumnTraversal == nil || *result.PGColumnTraversal != want {
				t.Fatalf("fact=%+v want=%+v", result.PGColumnTraversal, want)
			}
			encoded, err := json.Marshal(result)
			if err != nil || strings.Contains(string(encoded), "PGColumnTraversal") || strings.Contains(string(encoded), "ValidatedNotNullCheck") || strings.Contains(string(encoded), "IdentityChangeBlocked") || strings.Contains(string(encoded), req.Name) {
				t.Fatalf("private result=%s err=%v", encoded, err)
			}
		})
	}
}
