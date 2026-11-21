//go:build postgresql

package postgresql

import (
	"slices"
	"testing"
)

func TestPGCreateKeysRetainEveryDeclaration(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE TABLE IF NOT EXISTS app.t (" X, " integer PRIMARY KEY, id integer, UNIQUE NULLS NOT DISTINCT (" X, ", id) INCLUDE (id, id), UNIQUE USING INDEX existing_idx, PRIMARY KEY(id))`)
	if s.DDL == nil || s.DDL.PGCreateKeys == nil {
		t.Fatalf("key declarations missing: %+v", s)
	}
	keys := s.DDL.PGCreateKeys
	if keys.Target.Table.Schema != "app" || keys.Target.Table.Name != "t" || !keys.IfNotExists || keys.Unbound || len(keys.Declarations) != 4 {
		t.Fatalf("unexpected key declarations: %+v", keys)
	}
	inline, unique, attachment, primary := keys.Declarations[0], keys.Declarations[1], keys.Declarations[2], keys.Declarations[3]
	if !inline.Primary || !slices.Equal(inline.Keys, []string{" X, "}) || !unique.NullsNotDistinct || !slices.Equal(unique.Keys, []string{" X, ", "id"}) ||
		!slices.Equal(unique.Include, []string{"id", "id"}) || !attachment.UsingIndex || !primary.Primary || !slices.Equal(primary.Keys, []string{"id"}) {
		t.Fatalf("unexpected PostgreSQL key facts: %+v", keys.Declarations)
	}
}
