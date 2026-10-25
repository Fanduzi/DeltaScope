//go:build !postgresql

// Package postgresqlmeta provides a stub QueryAccessConnResolver when built without the postgresql tag.
package postgresqlmeta

// QueryAccessConnResolver is not available without postgresql build tag.
type QueryAccessConnResolver struct{}
