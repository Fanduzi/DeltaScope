// Package deltascope exposes the unified online query access capability seam.
// input: identity-derived online capability target
// output: single private routing definition for which capabilities are linked and routable in this build
// pos: private unified online capability routing seam shared by constructor and analysis entry
// note: if this file changes, update this header and module README.md.
package deltascope

import online "github.com/Fanduzi/DeltaScope/internal/application/online"

// queryAccessOnlineCapabilityLinked is the single private routing definition
// for the unified online entry. It reports whether an identity-derived
// capability target is linked and routable in this build. MySQL 5.7/8.0/8.4
// and TiDB 8.5 always route; PostgreSQL 17 routing depends on the
// postgresql build tag and is delegated to queryAccessPostgreSQLCapabilityLinked.
// Any target not admitted here fails closed with
// ErrOnlineQueryAccessCapabilityUnsupported.
func queryAccessOnlineCapabilityLinked(target online.CapabilityTarget) bool {
	switch target {
	case online.TargetMySQL57, online.TargetMySQL80, online.TargetMySQL84, online.TargetTiDB85:
		return true
	case online.TargetPG17:
		return queryAccessPostgreSQLCapabilityLinked()
	default:
		return false
	}
}
