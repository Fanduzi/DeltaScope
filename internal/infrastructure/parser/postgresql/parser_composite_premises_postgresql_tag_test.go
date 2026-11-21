//go:build postgresql

package postgresql

import (
	"slices"
	"testing"
)

func TestPGCompositeDeclaration(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE TYPE " P "." Pair " AS (" X " int, " X " text)`)
	if s.DDL == nil || s.DDL.PGComposite == nil || !slices.Equal(s.DDL.PGComposite.Attributes, []string{" X ", " X "}) ||
		s.DDL.PGComposite.Target.Table.Schema != " P " || s.DDL.PGComposite.Target.Table.Name != " Pair " {
		t.Fatalf("missing exact CREATE TYPE facts: %+v", s.DDL)
	}
	for _, tc := range []struct {
		sql, action, name, newName string
		guard                      bool
	}{
		{`ALTER TYPE app.t ADD ATTRIBUTE " A " integer`, "add_attribute", " A ", "", false},
		{`ALTER TYPE app.t DROP ATTRIBUTE IF EXISTS " A "`, "drop_attribute", " A ", "", true},
		{`ALTER TYPE app.t ALTER ATTRIBUTE " A " TYPE integer`, "alter_attribute_type", " A ", "", false},
		{`ALTER TYPE app.t RENAME ATTRIBUTE " A " TO " New "`, "rename_attribute", " A ", " New ", false},
		{`ALTER TYPE app.t ADD ATTRIBUTE a int, DROP ATTRIBUTE a`, "", "a", "", false},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		if s.DDL == nil || s.DDL.PGComposite == nil {
			t.Fatalf("%s: missing composite declaration", tc.sql)
		}
		decl := s.DDL.PGComposite
		if decl.Target.Table.Schema != "app" || decl.Target.Table.Name != "t" || decl.Action != tc.action ||
			decl.Name != tc.name || decl.NewName != tc.newName || decl.IfExists != tc.guard {
			t.Fatalf("%s: %+v", tc.sql, decl)
		}
	}
}
