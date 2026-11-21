package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGIndexAttachmentAssessment(t *testing.T) {
	target := spec.PGIndexAttachment{
		Parent: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "parent"}},
		Child:  spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "child"}},
	}
	for _, tc := range []struct {
		name, reason, fact string
		state              spec.PGIndexAttachmentState
	}{
		{name: "missing parent first", reason: "index_attachment_parent_absent"},
		{name: "ordinary parent not eligible", state: spec.PGIndexAttachmentState{ParentKind: "i", ChildKind: "i"}, reason: "index_attachment_parent_wrong_kind"},
		{name: "missing child", state: spec.PGIndexAttachmentState{ParentKind: "I"}, reason: "index_attachment_child_absent"},
		{name: "already attached", state: spec.PGIndexAttachmentState{ParentKind: "I", ChildKind: "i", SameParent: true, PartitionInUse: true}},
		{name: "duplicate slot before other attachment", state: spec.PGIndexAttachmentState{ParentKind: "I", ChildKind: "i", PartitionInUse: true, OtherParent: true}, reason: "index_attachment_partition_in_use"},
		{name: "detach pending", state: spec.PGIndexAttachmentState{ParentKind: "I", ChildKind: "i", TablePartition: true, DetachPending: true}, fact: "index.partition_visibility"},
		{name: "must compare new attachment", state: spec.PGIndexAttachmentState{ParentKind: "I", ChildKind: "I", TablePartition: true}, fact: "index.definition_compatibility"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.state.Target = target
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterIndex, PGIndexAttachment: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGIndexAttachment: &tc.state},
			}
			if reason, fact := assessPGIndexAttachment(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("reason=%q fact=%q, expected %q and %q", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
