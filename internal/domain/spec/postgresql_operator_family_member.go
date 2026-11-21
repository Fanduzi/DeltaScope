package spec

// PGOperatorFamilyMember identifies one ordinary search-operator slot.
// Empty operand or operator identities mean their names need more evidence.
type PGOperatorFamilyMember struct {
	Number   int32
	Drop     bool
	Left     PGColumnTypeTarget
	Right    PGColumnTypeTarget
	Operator Table
}

// PGOperatorFamilyMemberState binds a member observation to the exact slot.
type PGOperatorFamilyMemberState struct {
	Target           PGOperatorFamilyTarget
	Member           PGOperatorFamilyMember
	SchemaExists     bool
	MethodKind       string
	FamilyExists     bool
	BTree            bool
	OperandTypesRead bool
	OperandTypesOK   bool
	OperatorExists   bool
	OperatorBoolean  bool
	SlotExists       bool
	SlotProtected    bool
}

// PGOperatorFamilyMemberLookup binds the qualified family and member identities.
func PGOperatorFamilyMemberLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterOperatorFamily || s.DDL.PGOperatorFamily == nil || s.DDL.PGOperatorFamilyMember == nil {
		return nil
	}
	target := s.DDL.PGOperatorFamily
	if !(PGDestructiveTableTarget{Table: Table{Schema: target.Schema, Name: target.Name}}).Qualified() || target.Method == "" {
		return nil
	}
	member := s.DDL.PGOperatorFamilyMember
	return &ObjectLookupRequest{Type: "pg_operator_family_member", Schema: target.Schema, Name: target.Name,
		Qualifiers: map[string]string{"method": target.Method}, PGOperatorFamilyMember: member}
}
