// Package queryaccess defines transport-neutral domain types for query access analysis.
// input: SQL statement references, relation references, column references, and usage contexts
// output: pure domain models for query access requirements, read classification, and admission decisions
// pos: domain model for the query access analysis foundation shared across CLI, HTTP, and MCP surfaces
// note: if this file changes, update this header and module README.md.
package queryaccess
