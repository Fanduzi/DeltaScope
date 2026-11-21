package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPartitionConstraintAssessment(t *testing.T) {
	target := spec.Table{Schema: "app", Name: "events"}
	part := spec.PGPartitionConstraintKey{Name: " Exact ", CollationOK: true, EqualityRead: true, EqualityOK: true}
	for _, tt := range []struct {
		name, reason, gap string
		key               spec.PGPartitionConstraintKey
		members           []string
	}{
		{name: "exact partition member", key: part, members: []string{" Exact "}},
		{name: "partition is include only", key: part, members: []string{"payload"}, reason: "constraint_partition_column_missing"},
		{name: "expression slot", key: spec.PGPartitionConstraintKey{Expression: true}, members: []string{"payload"}, reason: "constraint_partition_expression"},
		{name: "missing column comes first", key: part, members: []string{"absent"}, reason: "constraint_column_absent"},
		{name: "collation mismatch", key: spec.PGPartitionConstraintKey{Name: " Exact "}, members: []string{" Exact "}, reason: "constraint_partition_comparison_mismatch"},
		{name: "unknown default equality", key: spec.PGPartitionConstraintKey{Name: " Exact ", CollationOK: true}, members: []string{" Exact "}, gap: "constraint.partition_equality"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &target, Alter: []spec.Alter{{
					Action: "add_constraint", Options: map[string]string{"constraint_type": "unique", "using_index": "false"},
					PGKeyColumns: tt.members, PGIncludeColumns: []string{" Exact "},
				}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Exists: true, Schema: target.Schema, PGRelation: spec.PGRelationState{Name: target.Name, Kind: "p"}},
					PGPartitionConstraint: &spec.PGPartitionConstraintState{Target: target, Columns: []spec.PGColumnState{{Name: " Exact "}, {Name: "payload"}},
						PartitionKeys: []spec.PGPartitionConstraintKey{tt.key}},
				},
			}
			if reason, gap, _ := assessPGConstraintColumns(s); reason != tt.reason || gap != tt.gap {
				t.Fatalf("got (%q,%q), want (%q,%q)", reason, gap, tt.reason, tt.gap)
			}
		})
	}
}
