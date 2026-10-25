// Package metadata exposes shared helpers for metadata-aware and offline interface adapters.
package metadata

// ExistenceNotCheckedNote is the offline limitation on CLI, HTTP, and MCP
// context when the adapter did not attach a table snapshot.
const ExistenceNotCheckedNote = "existence not checked (no database connection)"

// OfflineExistenceUnproven lists schema facts the offline path did not prove.
func OfflineExistenceUnproven() []string {
	return []string{"column_exists", "table_exists"}
}
