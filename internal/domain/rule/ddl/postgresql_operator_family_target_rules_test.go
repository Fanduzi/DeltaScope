package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGOperatorFamilyTargetAssessment(t *testing.T) {
	target := spec.PGOperatorFamilyTarget{Schema: " Exact Schema ", Name: " Exact Family ", Method: "btree"}
	for _, tc := range []struct {
		name, method, reason string
		schema, exists       bool
	}{
		{name: "missing schema precedes method", reason: "operator_family_schema_absent"},
		{name: "missing method", schema: true, reason: "operator_family_method_absent"},
		{name: "table method", schema: true, method: "t", reason: "operator_family_method_wrong_kind"},
		{name: "occupied for this method", schema: true, method: "i", exists: true, reason: "operator_family_already_exists"},
		{name: "available for this method", schema: true, method: "i"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateOperatorFamily, PGOperatorFamily: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGOperatorFamily: &spec.PGOperatorFamilyState{
					Target: target, SchemaExists: tc.schema, MethodKind: tc.method, Exists: tc.exists,
				}},
			}
			if reason, fact := assessPGOperatorFamilyTarget(s); reason != tc.reason || fact != "" {
				t.Fatalf("got %q/%q, want %q", reason, fact, tc.reason)
			}
		})
	}
}
