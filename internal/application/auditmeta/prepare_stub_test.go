//go:build !postgresql

// Package auditmeta verifies shared metadata-aware audit preparation for non-PG builds.
// input: metadata-aware audit requests where PostgreSQL parsing hits the capability boundary stub
// output: coverage for the capability-boundary error unwrapping path in Prepare
// pos: these tests only compile without the postgresql build tag — the stub returns PostgreSQLCapabilityBoundaryError
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"errors"
	"strings"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPrepareReturnsCapabilityBoundaryErrorWithoutSchemaWrapperForExplicitPostgreSQL(t *testing.T) {
	t.Parallel()

	client := &fakeClient{detectDialect: spec.DialectPostgreSQL}
	_, err := Prepare(context.Background(), Request{
		SQL:              "select 1;",
		RequestedDialect: spec.DialectPostgreSQL,
		ExplicitDialect:  true,
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err == nil {
		t.Fatal("expected capability-boundary error")
	}
	var prepErr *Error
	if !errors.As(err, &prepErr) {
		t.Fatalf("expected typed prepare error, got %T", err)
	}
	if prepErr.Kind != ErrorInvalidSQL {
		t.Fatalf("expected invalid_sql kind, got %#v", prepErr)
	}
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if !errors.As(prepErr.Err, &capabilityErr) {
		t.Fatalf("expected typed capability-boundary cause, got %T", prepErr.Err)
	}
	if err.Error() != capabilityErr.Error() {
		t.Fatalf("expected unwrapped capability message, got %q want %q", err.Error(), capabilityErr.Error())
	}
	if strings.Contains(strings.ToLower(err.Error()), "resolve schema targets:") {
		t.Fatalf("did not expect schema wrapper, got %q", err.Error())
	}
	if !client.closed {
		t.Fatal("expected client to close on prepare failure")
	}
}
