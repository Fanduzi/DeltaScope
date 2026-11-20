package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeDependencyAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, reason, gap     string
		unobserved, misbound, using bool
	}{
		{name: "view", kind: "view", reason: "column_type_has_direct_dependency"},
		{name: "generated dependency with USING", kind: "generated_column", using: true, reason: "column_type_has_direct_dependency"},
		{name: "foreign key is outside this type premise", kind: "foreign_key"},
		{name: "observed empty"},
		{name: "unobserved", unobserved: true, gap: "column.pg_type_dependencies"},
		{name: "misbound", kind: "view", misbound: true, gap: "column.pg_type_dependencies"},
		{name: "unexpected provider value", kind: "private_dependent_identity", gap: "column.pg_type_dependencies"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_data_type", &spec.PGColumnState{})
			a := &s.DDL.Alter[0]
			a.PGColumnType = spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int8"}
			if tc.using {
				a.Options["has_using"] = "true"
			}
			s.Metadata.PGColumnType = &spec.PGColumnTypeState{Target: a.PGColumnType, SchemaExists: true, BaseExists: true,
				Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: a.Name}
			if !tc.unobserved {
				f := &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: a.Name, Kind: tc.kind}
				if tc.misbound {
					f.Column = "ID"
				}
				s.Metadata.PGDropColumnDependency = f
			}
			r := pgColumnTypeDestinationRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.reason != "") ||
				len(findings) == 1 && findings[0].Metadata["reason_code"] != tc.reason {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			gaps := r.EvidenceGaps(s)
			if len(gaps) > 1 || (len(gaps) == 1) != (tc.gap != "") ||
				len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			payload, err := json.Marshal(s.Metadata)
			if err != nil || strings.Contains(string(payload), "PGDropColumnDependency") || strings.Contains(string(payload), "private_dependent_identity") {
				t.Fatalf("private dependency encoded: %s (%v)", payload, err)
			}
		})
	}
}
