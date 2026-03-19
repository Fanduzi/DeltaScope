// Package tidbparser verifies TiDB-backed parser adapter behavior.
// input: SQL text covering multi-statement parsing and parse failures
// output: test coverage for parser adapter classification and error handling
// pos: infrastructure parser adapter test coverage
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserParsesMultiStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create table t1 (id bigint); update t1 set id = 2 where id = 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Dialect != spec.DialectMySQL {
		t.Fatalf("expected dialect %q, got %q", spec.DialectMySQL, result.Dialect)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != spec.KindDDL {
		t.Fatalf("expected first statement kind %q, got %q", spec.KindDDL, result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != spec.KindDML {
		t.Fatalf("expected second statement kind %q, got %q", spec.KindDML, result.Statements[1].Kind)
	}
	if result.Statements[0].Text == "" || result.Statements[1].Text == "" {
		t.Fatalf("expected parsed statement text to be populated")
	}
}

func TestParserReturnsErrorForInvalidSQL(t *testing.T) {
	parser := New()

	_, err := parser.Parse("create table", spec.DialectMySQL)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
