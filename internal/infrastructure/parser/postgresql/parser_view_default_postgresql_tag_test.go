//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewDefaultRetainsExactTargetAndReferenceFact(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER VIEW IF EXISTS " Exact Schema "." Exact View " ALTER COLUMN " Exact Column " SET DEFAULT (SELECT 42)`)
	lookup := spec.PGViewDefaultLookup(s)
	if lookup == nil || lookup.Schema != " Exact Schema " || lookup.Name != " Exact View " || lookup.Qualifiers["column"] != " Exact Column " || !s.DDL.PGTableIfExists {
		t.Fatalf("statement=%+v lookup=%+v", s.DDL, lookup)
	}
	if got := s.DDL.Alter[0].PGColumnTransition; got == nil || !got.DefaultHasDisallowedReference {
		t.Fatalf("reference fact=%+v", got)
	}
	encoded, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(encoded), "PGColumnTransition") || strings.Contains(string(encoded), "SELECT 42") {
		t.Fatalf("public DDL=%s (%v)", encoded, err)
	}
}
