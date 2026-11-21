package spec

// PGCheckIntegerCoercion binds a catalog observation of integer-to-Boolean coercion.
type PGCheckIntegerCoercion struct {
	Allowed bool
}

// PGCheckIntegerLookup requests an observed cast for one integer-literal CHECK.
func PGCheckIntegerLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || s.DDL.Alter[0].PGCheckLiteral != "integer" || !PGOrdinaryTableBound(s) {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_check_integer_coercion"}
}
