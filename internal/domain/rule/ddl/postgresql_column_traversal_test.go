package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGRecursiveColumnRenameDestinationBinding(t *testing.T) {
	for _, tc := range []struct {
		name, observed, sourceReason, destinationReason string
		kind                                            int
		conflict, gap                                   bool
	}{
		{name: "descendant blocks rename", observed: " Exact Destination ", conflict: true, sourceReason: "column_rename_descendant_conflict"},
		{name: "case mismatch withholds the observation", observed: " exact Destination ", conflict: true, gap: true},
		{name: "root has user name collision", observed: " Exact Destination ", kind: 1, destinationReason: "column_already_exists"},
		{name: "root has system name collision", observed: " Exact Destination ", kind: -1, destinationReason: "system_column_name_conflict"},
		{name: "no conflict in tree or root", observed: " Exact Destination "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("rename_column", &spec.PGColumnState{})
			s.DDL.Alter[0].Column = &spec.AlterColumn{Definition: &spec.Column{Name: " Exact Destination "}}
			s.Metadata.TargetTable.PGColumns.Ordinary = false
			s.Metadata.PGColumnTraversal = &spec.PGColumnTraversal{
				Schema: "app", Table: "t", Column: "id", ColumnExists: true, AttachedChildren: true,
				RenameDestination: tc.observed, RenameDescendantsKnown: true, RenameDescendantConflict: tc.conflict,
				RenameDestinationKind: tc.kind,
			}
			source, sourceGap := pgColumnIdentityTarget(s, 0, ruleIDAlterRenameColumnExistsRequire)
			destination, destinationGap := pgColumnIdentityTarget(s, 0, ruleIDAlterRenameColumnTargetExistsForbid)
			wantGap := ""
			if tc.gap {
				wantGap = "target_table.pg_columns"
			}
			if source != tc.sourceReason || destination != tc.destinationReason || sourceGap != wantGap || destinationGap != wantGap {
				t.Fatalf("source=%q/%q destination=%q/%q", source, sourceGap, destination, destinationGap)
			}
		})
	}
}

func TestPGColumnTraversalPremises(t *testing.T) {
	for _, tc := range []struct {
		label, action, kind, want                       string
		inherited, key, children, only, stale, misbound bool
	}{
		{label: "inherited before partition key", action: "drop_column", kind: "p", inherited: true, key: true, children: true, only: true, want: "column_inherited"},
		{label: "partition key before ONLY", action: "drop_column", kind: "p", key: true, children: true, only: true, want: "column_in_partition_key"},
		{label: "partition parent only", action: "drop_column", kind: "p", children: true, only: true, want: "column_partition_parent_only"},
		{label: "ordinary only drop", action: "drop_column", kind: "r", children: true, only: true},
		{label: "parent rename needs traversal", action: "rename_column", kind: "r", children: true, only: true, want: "column_rename_requires_child_traversal"},
		{label: "partition parent rename needs traversal", action: "rename_column", kind: "p", children: true, only: true, want: "column_rename_requires_child_traversal"},
		{label: "inherited rename", action: "rename_column", kind: "r", inherited: true, want: "column_rename_inherited"},
		{label: "stale catalog", action: "drop_column", kind: "p", key: true, stale: true},
		{label: "column identity mismatch", action: "rename_column", kind: "r", inherited: true, misbound: true},
	} {
		t.Run(tc.label, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, &spec.PGColumnState{})
			s.DDL.PGTableOnly = tc.only
			s.DDL.Alter[0].Options = map[string]string{"if_exists": "true", "cascade": "true"}
			s.Metadata.TargetTable.PGRelation.Kind = tc.kind
			s.Metadata.TargetTable.PGColumns.Ordinary = false
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			fact := &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Inherited: tc.inherited, DirectPartitionKey: tc.key, AttachedChildren: tc.children}
			if tc.misbound {
				fact.Column = "ID"
			}
			s.Metadata.PGColumnTraversal = fact
			var got string
			if tc.action == "rename_column" {
				got, _ = pgColumnIdentityTarget(s, 0, ruleIDAlterRenameColumnExistsRequire)
			} else {
				_, got, _ = pgDropColumnTarget(s, 0)
			}
			if got != tc.want {
				t.Fatalf("reason=%q want=%q", got, tc.want)
			}
		})
	}
}
