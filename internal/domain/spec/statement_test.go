// Package spec defines normalized statement specifications for rule evaluation.
// input: statement kind and dialect domain scenarios
// output: coverage for typed statement metadata
// pos: domain specification test coverage
// note: if this file changes, update this header and module README.md.
package spec

import "testing"

func TestStatementKindAndDialectTypes(t *testing.T) {
	stmt := Statement{
		Kind:    KindDDL,
		Dialect: DialectMySQL,
	}

	if stmt.Kind != KindDDL {
		t.Fatalf("expected kind %q, got %q", KindDDL, stmt.Kind)
	}
	if stmt.Dialect != DialectMySQL {
		t.Fatalf("expected dialect %q, got %q", DialectMySQL, stmt.Dialect)
	}
}
