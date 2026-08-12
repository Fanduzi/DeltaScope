//go:build !postgresql

// Package deltascope reports PostgreSQL capability as not linked when built
// without the postgresql tag; the unified online entry fails closed for an
// observed PostgreSQL target with ErrOnlineQueryAccessCapabilityUnsupported.
// This is source-build compatibility only and is not a separate official
// product edition.
// input: none (build-tag capability leaf)
// output: queryAccessPostgreSQLCapabilityLinked() = false
// pos: no-postgresql-tag private capability leaf for the unified online entry
// note: if this file changes, update this header and module README.md.
package deltascope

// queryAccessPostgreSQLCapabilityLinked reports that PostgreSQL 17 proof is
// not linked in this source build, so the unified online entry fails closed
// for an observed PostgreSQL target.
func queryAccessPostgreSQLCapabilityLinked() bool { return false }
