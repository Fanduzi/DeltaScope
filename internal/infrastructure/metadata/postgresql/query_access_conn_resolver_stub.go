//go:build !postgresql

// Package postgresqlmeta provides a stub QueryAccessConnResolver when built without the postgresql tag.
// input: none (stub only)
// output: empty struct satisfying build constraints
// pos: infrastructure stub for non-PostgreSQL builds
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

// QueryAccessConnResolver is not available without postgresql build tag.
type QueryAccessConnResolver struct{}
