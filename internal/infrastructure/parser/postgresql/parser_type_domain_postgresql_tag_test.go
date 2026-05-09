//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateTypeEnumNormalizes(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "CREATE TYPE color AS ENUM ('red', 'green', 'blue')")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported %#v", stmt.Unsupported)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationCreateType {
		t.Fatalf("expected create_type operation, got %#v", stmt.DDL)
	}
	if stmt.DDL.ObjectName != "color" {
		t.Fatalf("expected object_name color, got %q", stmt.DDL.ObjectName)
	}
	if stmt.DDL.ObjectType != "type" {
		t.Fatalf("expected object_type type, got %q", stmt.DDL.ObjectType)
	}
	if stmt.DDL.Options["type_kind"] != "enum" {
		t.Fatalf("expected type_kind enum, got %q", stmt.DDL.Options["type_kind"])
	}
	if stmt.DDL.Options["labels"] != "red,green,blue" {
		t.Fatalf("expected labels red,green,blue, got %q", stmt.DDL.Options["labels"])
	}
}

func TestExtractAlterTypeEnumAddValueNormalizes(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TYPE color ADD VALUE 'yellow'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported %#v", stmt.Unsupported)
	}
	if stmt.DDL.Operation != spec.DDLOperationAlterType {
		t.Fatalf("expected alter_type, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.Options["action"] != "add_value" {
		t.Fatalf("expected action add_value, got %q", stmt.DDL.Options["action"])
	}
	if stmt.DDL.Options["value"] != "yellow" {
		t.Fatalf("expected value yellow, got %q", stmt.DDL.Options["value"])
	}
	if _, ok := stmt.DDL.Options["placement"]; ok {
		t.Fatalf("expected no placement for basic add value, got %q", stmt.DDL.Options["placement"])
	}
}

func TestExtractAlterTypeEnumAddValueIfNotExists(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.Options["if_not_exists"] != "true" {
		t.Fatalf("expected if_not_exists true, got %q", stmt.DDL.Options["if_not_exists"])
	}
}

func TestExtractAlterTypeEnumAddValueBefore(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.Options["placement"] != "before" {
		t.Fatalf("expected placement before, got %q", stmt.DDL.Options["placement"])
	}
	if stmt.DDL.Options["neighbor"] != "green" {
		t.Fatalf("expected neighbor green, got %q", stmt.DDL.Options["neighbor"])
	}
}

func TestExtractAlterTypeEnumAddValueAfter(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.Options["placement"] != "after" {
		t.Fatalf("expected placement after, got %q", stmt.DDL.Options["placement"])
	}
	if stmt.DDL.Options["neighbor"] != "green" {
		t.Fatalf("expected neighbor green, got %q", stmt.DDL.Options["neighbor"])
	}
}

func TestExtractDropTypeNormalizes(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "DROP TYPE color")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported %#v", stmt.Unsupported)
	}
	if stmt.DDL.Operation != spec.DDLOperationDropType {
		t.Fatalf("expected drop_type, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.ObjectName != "color" {
		t.Fatalf("expected object_name color, got %q", stmt.DDL.ObjectName)
	}
	if stmt.DDL.ObjectType != "type" {
		t.Fatalf("expected object_type type, got %q", stmt.DDL.ObjectType)
	}
}

func TestExtractDropTypeIfExistsCascade(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "DROP TYPE IF EXISTS color CASCADE")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.Options["if_exists"] != "true" {
		t.Fatalf("expected if_exists true, got %q", stmt.DDL.Options["if_exists"])
	}
	if stmt.DDL.Options["cascade"] != "true" {
		t.Fatalf("expected cascade true, got %q", stmt.DDL.Options["cascade"])
	}
}

func TestExtractCompositeType(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "CREATE TYPE address AS (street text, city text)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected normalized, got unsupported: %s", stmt.Unsupported.Feature)
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
	if stmt.DDL.Operation != spec.DDLOperationCreateType {
		t.Fatalf("expected create_type, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.ObjectName != "address" {
		t.Fatalf("expected object name address, got %q", stmt.DDL.ObjectName)
	}
	if stmt.DDL.Options["type_kind"] != "composite" {
		t.Fatalf("expected type_kind=composite, got %q", stmt.DDL.Options["type_kind"])
	}
	if stmt.DDL.Options["attributes"] != "2" {
		t.Fatalf("expected attributes=2, got %q", stmt.DDL.Options["attributes"])
	}
	if stmt.DDL.Options["attribute_names"] != "street,city" {
		t.Fatalf("expected attribute_names=street,city, got %q", stmt.DDL.Options["attribute_names"])
	}
}

func TestExtractCreateDomain(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "CREATE DOMAIN email AS text CHECK (VALUE <> '')")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL == nil {
		t.Fatalf("expected normalized DDL, got nil")
	}
	if stmt.DDL.Operation != spec.DDLOperationCreateDomain {
		t.Fatalf("expected create_domain, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.ObjectName != "email" {
		t.Fatalf("expected object name email, got %q", stmt.DDL.ObjectName)
	}
	if stmt.DDL.ObjectType != "domain" {
		t.Fatalf("expected object type domain, got %q", stmt.DDL.ObjectType)
	}
	if stmt.DDL.Options["base_type"] != "text" {
		t.Fatalf("expected base_type text, got %q", stmt.DDL.Options["base_type"])
	}
	if stmt.DDL.Options["has_check"] != "true" {
		t.Fatalf("expected has_check true, got %q", stmt.DDL.Options["has_check"])
	}
}
