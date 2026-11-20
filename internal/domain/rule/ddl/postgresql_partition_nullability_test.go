package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPartitionNullabilityPremises(t *testing.T) {
	for _, tc := range []struct {
		name, action, kind, reason, gap                                     string
		only, partition, children, parentNotNull, nullableDescendant, stale bool
	}{
		{name: "partition child", action: "drop_not_null", kind: "r", partition: true, parentNotNull: true, reason: "not_null_required_by_partition_parent"},
		{name: "local partition constraint", action: "drop_not_null", kind: "r", partition: true, gap: "target_table.pg_columns"},
		{name: "ordinary child", action: "drop_not_null", kind: "r", parentNotNull: true, gap: "target_table.pg_columns"},
		{name: "only root drop", action: "drop_not_null", kind: "p", only: true, children: true, reason: "drop_not_null_requires_partition_traversal"},
		{name: "recursive root drop", action: "drop_not_null", kind: "p", children: true, gap: "target_table.pg_columns"},
		{name: "only root set with nullable leaf", action: "set_not_null", kind: "p", only: true, children: true, nullableDescendant: true, reason: "set_not_null_requires_partition_traversal"},
		{name: "only root set with marked leaves", action: "set_not_null", kind: "p", only: true, children: true, gap: "target_table.pg_columns"},
		{name: "recursive set", action: "set_not_null", kind: "p", children: true, nullableDescendant: true, gap: "target_table.pg_columns"},
		{name: "stale", action: "drop_not_null", kind: "p", only: true, children: true, stale: true, gap: "target_table.pg_columns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, &spec.PGColumnState{})
			s.DDL.PGTableOnly = tc.only
			s.Metadata.TargetTable.PGColumns.Ordinary = false
			s.Metadata.TargetTable.PGRelation.Kind = tc.kind
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			s.Metadata.PGColumnTraversal = &spec.PGColumnTraversal{
				Schema: "app", Table: "t", Column: "id", Partition: tc.partition,
				AttachedChildren: tc.children, ParentNotNull: tc.parentNotNull, NullableDescendant: tc.nullableDescendant,
			}
			_, reason, gap := pgColumnTransitionTarget(s, 0)
			if reason != tc.reason || gap != tc.gap {
				t.Fatalf("reason=%q gap=%q", reason, gap)
			}
		})
	}
}
