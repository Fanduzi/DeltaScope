//go:build postgresql

package postgresql

import (
	"slices"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGSequenceChangeDeclaration(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE UNLOGGED SEQUENCE IF NOT EXISTS " App "." Seq " AS smallint INCREMENT -1 NO MINVALUE MAXVALUE -2 OWNED BY " App "." T "." X "`)
	if s.DDL == nil || s.DDL.PGSequenceChange == nil {
		t.Fatal("sequence declaration missing")
	}
	d := s.DDL.PGSequenceChange
	if d.Target.Table != (spec.Table{Schema: " App ", Name: " Seq "}) || !d.IfNotExists || d.Persistence != "u" || len(d.Options) != 5 ||
		d.Options[0].Width != 16 || d.Options[1].Value == nil || *d.Options[1].Value != -1 || d.Options[2].Value != nil || !d.Options[2].Known ||
		!slices.Equal(d.Ownership(), []string{" App ", " T ", " X "}) {
		t.Fatalf("unexpected sequence facts: %+v", d)
	}
	a := extractPostgreSQLStatement(t, `ALTER SEQUENCE IF EXISTS app.s RESTART OWNED BY NONE`)
	if a.DDL == nil || a.DDL.PGSequenceChange == nil || !a.DDL.PGSequenceChange.IfExists ||
		!slices.Equal(a.DDL.PGSequenceChange.Ownership(), []string{"none"}) || a.DDL.PGSequenceChange.Options[0].Value != nil {
		t.Fatalf("unexpected ALTER SEQUENCE facts: %+v", a.DDL)
	}
}
