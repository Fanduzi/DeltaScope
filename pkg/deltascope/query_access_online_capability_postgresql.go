//go:build postgresql

// Package deltascope reports PostgreSQL capability as linked when built with
// the postgresql tag; the unified online entry routes PG17 through the shared
// trusted proof core.
// input: none (build-tag capability leaf)
// output: queryAccessPostgreSQLCapabilityLinked() = true
// pos: postgresql-tagged private capability leaf for the unified online entry
// note: if this file changes, update this header and module README.md.
package deltascope

// queryAccessPostgreSQLCapabilityLinked reports that PostgreSQL 17 proof is
// linked and routable in this build. Official DeltaScope binaries are built
// with the postgresql tag and remain PostgreSQL-capable.
func queryAccessPostgreSQLCapabilityLinked() bool { return true }
