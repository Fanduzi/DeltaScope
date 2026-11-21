//go:build postgresql

package postgresql

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateViewOutputPreservesLiteralColumnOrder(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE OR REPLACE VIEW " Exact Schema "." Exact View " (" First ") AS SELECT 1 AS ignored, 2147483648 AS big, true AS enabled, 'a' COLLATE pg_catalog."C" AS v`)
	lookup := spec.PGCreateViewTargetLookup(s)
	if lookup == nil || lookup.Schema != " Exact Schema " || lookup.Name != " Exact View " {
		t.Fatalf("lookup=%+v", lookup)
	}
	output := s.DDL.PGCreateViewOutput
	want := []spec.PGViewOutputColumn{
		{Name: " First ", TypeOID: 23, Typmod: -1},
		{Name: "big", TypeOID: 20, Typmod: -1},
		{Name: "enabled", TypeOID: 16, Typmod: -1},
		{Name: "v", TypeOID: 25, Typmod: -1, Collation: 950},
	}
	if output == nil || output.AliasCount != 1 || !slices.Equal(output.Columns, want) {
		t.Fatalf("output=%+v want=%+v", output, want)
	}
	encoded, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(encoded), "TypeOID") || strings.Contains(string(encoded), " First ") {
		t.Fatalf("public DDL=%s (%v)", encoded, err)
	}
	for _, sql := range []string{
		`CREATE VIEW app.v AS SELECT * FROM app.t`,
		`CREATE VIEW app.v AS SELECT 'value' COLLATE "C" AS id`,
		`CREATE VIEW app.v AS SELECT 1 UNION SELECT 2`,
	} {
		s := extractPostgreSQLStatement(t, sql)
		if s.DDL.PGCreateViewOutput == nil || s.DDL.PGCreateViewOutput.Columns != nil {
			t.Fatalf("%q incorrectly proves output %+v", sql, s.DDL.PGCreateViewOutput)
		}
	}
}
