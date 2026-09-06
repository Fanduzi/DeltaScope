// Package connresolve turns caller connection input into a ready-to-open configuration.
// input: requested dialect, configured port, and whether the caller set the port explicitly
// output: 5432 when PostgreSQL is explicit and the port was omitted
// pos: shared PostgreSQL omitted-port default before open
// note: if this file changes, update this header and module README.md.
package connresolve

import "strings"

const postgresqlDefaultPort = 5432

// ApplyPostgreSQLDefaultPort returns 5432 when dialect is postgresql and the
// caller did not set a port. Other dialects keep the configured port.
func ApplyPostgreSQLDefaultPort(dialect string, port int, portExplicit bool) int {
	if portExplicit {
		return port
	}
	if strings.EqualFold(strings.TrimSpace(dialect), "postgresql") {
		return postgresqlDefaultPort
	}
	return port
}
