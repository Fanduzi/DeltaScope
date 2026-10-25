//go:build !postgresql

// Package queryaccess provides the PostgreSQL query access stub when built without the postgresql tag.
package queryaccess

import (
	"context"
	"errors"
)

// ErrPostgreSQLNotAvailable indicates PostgreSQL support was not compiled in.
var ErrPostgreSQLNotAvailable = errors.New("postgresql support requires build tag: go build -tags postgresql")

// AnalyzePostgreSQL returns ErrPostgreSQLNotAvailable when built without the postgresql tag.
func AnalyzePostgreSQL(_ context.Context, _ QueryAccessRequest) (QueryAccessResult, error) {
	return QueryAccessResult{}, ErrPostgreSQLNotAvailable
}
