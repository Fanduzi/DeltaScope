package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropColumnDirectDependency(t *testing.T) {
	r := pgDropTestRule(t)
	for _, tc := range []struct {
		name        string
		fact        *spec.PGDropColumnDependency
		alter       func(*spec.Statement)
		reason, gap string
	}{
		{name: "known view", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "view"}, reason: "column_has_direct_dependency"},
		{name: "known generated expression", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "generated_column"}, reason: "column_has_direct_dependency"},
		{name: "absent direct blockers", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id"}},
		{name: "missing read", gap: "column.pg_dependencies"},
		{name: "mismatched case", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "ID", Kind: "view"}, gap: "column.pg_dependencies"},
		{name: "unrecognized kind", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "private_dependency_name"}, gap: "column.pg_dependencies"},
		{name: "stale relation", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "view"}, alter: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }, gap: "column.pg_dependencies"},
		{name: "CASCADE", fact: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "view"}, alter: func(s *spec.Statement) { s.DDL.Alter[0].Options["cascade"] = "true" }},
		{name: "guarded absent", alter: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("drop_column", &spec.PGColumnState{})
			s.DDL.Alter[0].Options = map[string]string{"if_exists": "true", "cascade": "false"}
			s.Metadata.PGDropColumnDependency = tc.fact
			if tc.alter != nil {
				tc.alter(&s)
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) == 0 && tc.reason != "" || len(findings) > 0 && (len(findings) != 1 || findings[0].Metadata["reason_code"] != tc.reason) {
				t.Fatalf("findings %+v", findings)
			}
			gaps := r.EvidenceGaps(s)
			if len(gaps) == 0 && tc.gap != "" || len(gaps) > 0 && (len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps %+v", gaps)
			}
			encoded, err := json.Marshal(s)
			if err != nil || strings.Contains(string(encoded), "PGDropColumnDependency") || strings.Contains(string(encoded), "private_dependency_name") {
				t.Fatal("private observation appeared in output")
			}
		})
	}
}
