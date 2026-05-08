// Package tidbparser verifies TiDB-backed parser adapter behavior.
// input: SQL text covering multi-statement parsing and parse failures
// output: test coverage for parser adapter parsing and error handling
// pos: infrastructure parser adapter test coverage
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"
)

func TestParserParsesMultiStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse(context.Background(), "create table t1 (id bigint); update t1 set id = 2 where id = 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	if result.Statements[0].Text() == "" || result.Statements[1].Text() == "" {
		t.Fatalf("expected parsed statement text to be populated")
	}
}

func TestParserReturnsErrorForInvalidSQL(t *testing.T) {
	parser := New()

	_, err := parser.Parse(context.Background(), "create table")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestWrapStatementsReturnsExtractorBackedResults(t *testing.T) {
	parser := New()

	result, err := parser.Parse(context.Background(), "create table t1 (id bigint); update t1 set id = 2 where id = 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 2 {
		t.Fatalf("expected 2 wrapped statements, got %d", len(wrapped))
	}
	if wrapped[0].RawSQL == "" || wrapped[1].RawSQL == "" {
		t.Fatalf("expected raw SQL to be preserved")
	}
	if wrapped[0].Extractor == nil || wrapped[1].Extractor == nil {
		t.Fatalf("expected wrapped statements to carry extractors")
	}
}
