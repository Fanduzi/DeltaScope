//go:build !postgresql

package audit

func parsePostgreSQL(_ string) (ParsedSQL, error) {
	return ParsedSQL{}, &PostgreSQLCapabilityBoundaryError{Message: "PostgreSQL support requires a PG-capable DeltaScope build; rebuild with -tags postgresql or use a PostgreSQL-capable DeltaScope binary"}
}
