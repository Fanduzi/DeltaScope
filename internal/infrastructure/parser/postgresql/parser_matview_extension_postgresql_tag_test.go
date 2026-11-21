//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGMaterializedViewExtensionExtractsExactIdentifiers(t *testing.T) {
	for _, tc := range []struct{ clause, action string }{
		{"DEPENDS ON EXTENSION", "depends_on_extension"},
		{"NO DEPENDS ON EXTENSION", "no_depends_on_extension"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, `ALTER MATERIALIZED VIEW " Exact "." Report " `+tc.clause+` " Bound Extension "`)
			lookup := spec.PGMaterializedViewExtensionLookup(s)
			if lookup == nil || lookup.Schema != " Exact " || lookup.Name != " Report " || lookup.Qualifiers["extension"] != " Bound Extension " || s.DDL.Alter[0].Action != tc.action {
				t.Fatalf("ddl=%+v lookup=%+v", s.DDL, lookup)
			}
			encoded, err := json.Marshal(s.DDL)
			if err != nil || strings.Contains(string(encoded), "Bound Extension") {
				t.Fatalf("public DDL=%s (%v)", encoded, err)
			}
		})
	}
}
