// Package application provides shared application-level input normalization.
// input: SQL text supplied by interface and public API callers
// output: SQL text with one leading UTF-8 BOM treated as an encoding marker
// pos: shared SQL-input boundary before use-case validation and parsing
// note: if this file changes, update this header and module README.md.
package application

import "strings"

// NormalizeSQLInput removes exactly one leading UTF-8 BOM from SQL input.
func NormalizeSQLInput(sql string) string {
	return strings.TrimPrefix(sql, "\uFEFF")
}
