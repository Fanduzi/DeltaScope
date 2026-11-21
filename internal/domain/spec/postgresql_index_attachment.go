package spec

// PGIndexAttachment identifies the exact indexes named by ATTACH PARTITION.
type PGIndexAttachment struct {
	Parent PGDestructiveTableTarget
	Child  PGDestructiveTableTarget
}

// PGIndexAttachmentState contains only catalog facts used to check the attachment.
type PGIndexAttachmentState struct {
	Target         PGIndexAttachment
	ParentKind     string
	ChildKind      string
	SameParent     bool
	PartitionInUse bool
	OtherParent    bool
	TablePartition bool
	DetachPending  bool
	Definition     string // empty when definition compatibility is unobserved
}

// PGIndexAttachmentLookup binds qualified index names without resolving search_path.
func PGIndexAttachmentLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterIndex || s.DDL.PGIndexAttachment == nil {
		return nil
	}
	target := *s.DDL.PGIndexAttachment
	if !target.Parent.Qualified() || !target.Child.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_index_attachment", Schema: target.Parent.Table.Schema, Name: target.Parent.Table.Name,
		Qualifiers: map[string]string{"child_schema": target.Child.Table.Schema, "child": target.Child.Table.Name}}
}
