//go:build !postgresql

package audit

import "fmt"

func parsePostgreSQL(_ string) (ParsedSQL, error) {
	return ParsedSQL{}, fmt.Errorf("PostgreSQL support requires the PG-capable build; install deltascope-pg or use the Go library with the postgresql build tag")
}
