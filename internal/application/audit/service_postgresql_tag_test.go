//go:build postgresql

package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLReturnsMixedSupportedAndUnsupportedPostgreSQLResults(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 supported statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected supported statement kind ddl, got %#v", result.Statements[0])
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Index != 1 {
		t.Fatalf("expected unsupported statement index 1, got %#v", result.Unsupported[0])
	}
}
