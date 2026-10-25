//go:build postgresql

// Package deltascope reports PostgreSQL capability as linked when built with
// the postgresql tag; the unified online entry routes PG17 through the shared
// trusted proof core.
package deltascope

// queryAccessPostgreSQLCapabilityLinked reports that PostgreSQL 17 proof is
// linked and routable in this build. Official DeltaScope binaries are built
// with the postgresql tag and remain PostgreSQL-capable.
func queryAccessPostgreSQLCapabilityLinked() bool { return true }
