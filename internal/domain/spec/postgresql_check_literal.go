package spec

// PGCheckIntegerCoercion binds a catalog observation of numeric literal-to-Boolean coercion.
type PGCheckIntegerCoercion struct {
	Allowed bool
	Source  string
}

// PGCheckIntegerLookup requests an observed cast for one numeric-literal CHECK.
func PGCheckIntegerLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || !PGOrdinaryTableBound(s) {
		return nil
	}
	source := s.DDL.Alter[0].PGCheckLiteral
	switch source {
	case "integer", "bigint", "numeric":
		return &ObjectLookupRequest{Type: "pg_check_integer_coercion", Name: source}
	default:
		return nil
	}
}
