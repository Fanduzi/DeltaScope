//go:build !postgresql

// Package postgresql tests the PostgreSQL query access stub.
// input: any SQL text
// output: ErrPostgreSQLNotAvailable
// pos: infrastructure stub test for non-PostgreSQL builds
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"errors"
	"testing"
)

func TestQueryAccessExtractor_StubReturnsError(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	_, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id FROM users", "postgresql", "")
	if err == nil {
		t.Fatal("expected error from stub, got nil")
	}
	if !errors.Is(err, ErrPostgreSQLNotAvailable) {
		t.Errorf("expected ErrPostgreSQLNotAvailable, got %v", err)
	}
}
