package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGOperatorFamilyMemberAssessment(t *testing.T) {
	target := spec.PGOperatorFamilyTarget{Schema: "app", Name: "family", Method: "btree"}
	integer := spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}
	member := spec.PGOperatorFamilyMember{Number: 1, Left: integer, Right: integer, Operator: spec.Table{Schema: "pg_catalog", Name: "<"}}
	for _, tc := range []struct {
		name, reason, fact string
		change             func(*spec.PGOperatorFamilyMemberState)
	}{
		{name: "available slot"},
		{name: "family absence precedes bounds", reason: "operator_family_absent", change: func(s *spec.PGOperatorFamilyMemberState) { s.FamilyExists, s.Member.Number = false, 6 }},
		{name: "invalid strategy", reason: "operator_family_strategy_out_of_range", change: func(s *spec.PGOperatorFamilyMemberState) { s.Member.Number = 6 }},
		{name: "nonboolean operator precedes duplicate", reason: "operator_family_operator_result", change: func(s *spec.PGOperatorFamilyMemberState) { s.OperatorBoolean, s.SlotExists = false, true }},
		{name: "qualified operator unresolved", fact: "operator_family.operator_identity", change: func(s *spec.PGOperatorFamilyMemberState) { s.Member.Operator.Schema = "" }},
		{name: "protected drop slot", reason: "operator_family_slot_protected", change: func(s *spec.PGOperatorFamilyMemberState) {
			s.Member.Drop, s.SlotExists, s.SlotProtected = true, true, true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := spec.PGOperatorFamilyMemberState{Target: target, Member: member, SchemaExists: true, MethodKind: "i", FamilyExists: true,
				BTree: true, OperandTypesRead: true, OperandTypesOK: true, OperatorExists: true, OperatorBoolean: true}
			if tc.change != nil {
				tc.change(&state)
			}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterOperatorFamily, PGOperatorFamily: &target, PGOperatorFamilyMember: &state.Member},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGOperatorFamilyMember: &state},
			}
			if reason, fact := assessPGOperatorFamilyMember(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("reason %q, fact %q, want %q, %q", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
