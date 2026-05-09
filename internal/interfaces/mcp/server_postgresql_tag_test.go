//go:build postgresql

// Package mcpapi verifies PostgreSQL-only MCP behavior under the PG-capable build.
// input: MCP audit_sql tool calls executed with dialect=postgresql against the PG-capable binary path
// output: focused coverage for PostgreSQL offline audit success and additive MCP context fields
// pos: tagged MCP adapter regression coverage for PostgreSQL surface support
// note: if this file changes, update this header and module README.md.
package mcpapi
