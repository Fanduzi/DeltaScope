// Package application provides shared application-level input normalization.
package application

import "strings"

// NormalizeSQLInput removes exactly one leading UTF-8 BOM from SQL input.
func NormalizeSQLInput(sql string) string {
	return strings.TrimPrefix(sql, "\uFEFF")
}
