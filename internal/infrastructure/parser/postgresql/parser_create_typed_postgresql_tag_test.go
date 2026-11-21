//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateTypedRetainsExactSourceAndOptions(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE TABLE IF NOT EXISTS app.t OF " Type Schema "." Source Type " (" X " WITH OPTIONS DEFAULT true, x WITH OPTIONS DEFAULT 42)`)
	if s.DDL == nil || s.DDL.PGCreateTyped == nil {
		t.Fatalf("missing CREATE TABLE OF facts: %+v", s)
	}
	typed := s.DDL.PGCreateTyped
	if typed.Type.Table != (spec.Table{Schema: " Type Schema ", Name: " Source Type "}) || typed.Target.Table != (spec.Table{Schema: "app", Name: "t"}) || !typed.IfNotExists || typed.Temporary {
		t.Fatalf("unexpected typed table declaration: %+v", typed)
	}
	if len(s.DDL.Columns) != 2 || s.DDL.Columns[0].Name != " X " || s.DDL.Columns[1].Name != "x" {
		t.Fatalf("unexpected typed column options: %+v", s.DDL.Columns)
	}
	body, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(body), "PGCreateTyped") || strings.Contains(string(body), " Source Type ") {
		t.Fatalf("private type identity in DDL: %s (%v)", body, err)
	}
}
