// Package metadata exposes shared helpers for metadata-aware and offline interface adapters.
// input: transport-layer connection inputs and offline-audit context needs
// output: connection helpers plus the shared offline existence caveat for CLI, HTTP, and MCP
// pos: shared interface-adapter helpers used when a transport did not attach a snapshot
// note: if this file changes, update this header and module README.md.
package metadata

// ExistenceNotCheckedNote is the offline limitation on CLI, HTTP, and MCP
// context when the adapter did not attach a table snapshot.
const ExistenceNotCheckedNote = "existence not checked (no database connection)"

// OfflineExistenceUnproven lists schema facts the offline path did not prove.
func OfflineExistenceUnproven() []string {
	return []string{"column_exists", "table_exists"}
}
