//go:build postgresql

package postgresql

import (
	"slices"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPolicyChangeDeclaration(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER POLICY " Policy " ON " App "." Rows " TO " Role ", PUBLIC, ignored USING (" X " IS NULL) WITH CHECK (true)`)
	if s.DDL == nil || s.DDL.PGPolicyChange == nil {
		t.Fatal("policy declaration missing")
	}
	d := s.DDL.PGPolicyChange
	if d.Table.Table != (spec.Table{Schema: " App ", Name: " Rows "}) || d.Name != " Policy " || len(d.Roles) != 3 ||
		!slices.Equal(d.NamedPolicyRoles(), []string{" Role "}) || !d.Using.BooleanResult || d.Using.Column != " X " || !d.WithCheck.BooleanResult {
		t.Fatalf("unexpected policy facts: %+v", d)
	}
	r := extractPostgreSQLStatement(t, `ALTER POLICY " Before " ON app.rows RENAME TO " After "`)
	if r.DDL == nil || r.DDL.PGPolicyChange == nil || r.DDL.PGPolicyChange.Name != " Before " || r.DDL.PGPolicyChange.RenameTo != " After " {
		t.Fatalf("unexpected policy rename: %+v", r.DDL)
	}
}
