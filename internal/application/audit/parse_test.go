// Package audit verifies application parsing boundaries.
// input: SQL text and application parsing entrypoints
// output: test coverage for application-owned parsed results
// pos: application audit boundary test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParseReturnsApplicationOwnedStatements(t *testing.T) {
	result, err := Parse("create table t1 (id bigint);", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resultType := reflect.TypeOf(result)
	if got, want := resultType.PkgPath(), "github.com/Fanduzi/DeltaScope/internal/application/audit"; got != want {
		t.Fatalf("expected application-owned parse result type from %q, got %q", want, got)
	}

	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 parsed statement, got %d", len(result.Statements))
	}

	stmtType := reflect.TypeOf(result.Statements[0])
	if _, ok := stmtType.FieldByName("RawSQL"); !ok {
		t.Fatalf("expected application statement to expose RawSQL, fields are %v", stmtType)
	}
	if _, ok := stmtType.FieldByName("Node"); ok {
		t.Fatalf("expected AST node to remain hidden from the application return contract")
	}
	if result.Statements[0].RawSQL == "" {
		t.Fatalf("expected RawSQL to be populated")
	}
}
