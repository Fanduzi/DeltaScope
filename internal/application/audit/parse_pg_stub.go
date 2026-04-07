//go:build !postgresql

package audit

import "fmt"

func parsePostgreSQL(_ string) (ParsedSQL, error) {
	return ParsedSQL{}, fmt.Errorf("PostgreSQL support requires a PG-capable DeltaScope build; rebuild with -tags postgresql or use a PostgreSQL-capable DeltaScope binary")
}
