//go:build !postgresql

// Package deltascope reports PostgreSQL capability as not linked when built
// without the postgresql tag; the unified online entry fails closed for an
// observed PostgreSQL target with ErrOnlineQueryAccessCapabilityUnsupported.
// This is source-build compatibility only and is not a separate official
// product edition.
package deltascope

// queryAccessPostgreSQLCapabilityLinked reports that PostgreSQL 17 proof is
// not linked in this source build, so the unified online entry fails closed
// for an observed PostgreSQL target.
func queryAccessPostgreSQLCapabilityLinked() bool { return false }
