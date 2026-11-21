//go:build postgresql

package postgresql

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestPGConstraintColumnsRetainExactMembers(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD PRIMARY KEY (" A,B ", " a,b ") INCLUDE (id, id)`)
	a := s.DDL.Alter[0]
	if !slices.Equal(a.PGKeyColumns, []string{" A,B ", " a,b "}) || !slices.Equal(a.PGIncludeColumns, []string{"id", "id"}) {
		t.Fatalf("key columns = %q, included columns = %q", a.PGKeyColumns, a.PGIncludeColumns)
	}
	data, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(data), "PGKeyColumns") || strings.Contains(string(data), "PGIncludeColumns") {
		t.Fatalf("public DDL = %s, error = %v", data, err)
	}
}
