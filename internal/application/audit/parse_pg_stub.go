//go:build !postgresql

package audit

import "context"

func parsePostgreSQL(_ context.Context, _ string) (ParsedSQL, error) {
	return ParsedSQL{}, &PostgreSQLCapabilityBoundaryError{Message: "PostgreSQL support requires a PG-capable DeltaScope build; rebuild with -tags postgresql or use a PostgreSQL-capable DeltaScope binary"}
}
