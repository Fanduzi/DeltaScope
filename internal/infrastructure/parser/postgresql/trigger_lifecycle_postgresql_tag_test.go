//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGExtractorCreateTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if s.Unsupported != nil {
		t.Fatalf("expected supported, got feature=%q reason=%q", s.Unsupported.Feature, s.Unsupported.Reason)
	}
	if s.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
	if s.DDL.Operation != spec.DDLOperationCreateTrigger {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationCreateTrigger, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "trg_audit" {
		t.Fatalf("expected object name %q, got %q", "trg_audit", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "trigger" {
		t.Fatalf("expected object type %q, got %q", "trigger", s.DDL.ObjectType)
	}
	if s.DDL.Table == nil || s.DDL.Table.Name != "users" {
		t.Fatalf("expected table name %q, got %#v", "users", s.DDL.Table)
	}
	if s.DDL.Options["constraint"] != "" {
		t.Fatalf("expected no constraint option for regular trigger, got %q", s.DDL.Options["constraint"])
	}
}

func TestPGExtractorCreateConstraintTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "CREATE CONSTRAINT TRIGGER trg_fk AFTER INSERT ON orders FROM items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fk()")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if s.Unsupported != nil {
		t.Fatalf("expected supported, got feature=%q reason=%q", s.Unsupported.Feature, s.Unsupported.Reason)
	}
	if s.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
	if s.DDL.Operation != spec.DDLOperationCreateTrigger {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationCreateTrigger, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "trg_fk" {
		t.Fatalf("expected object name %q, got %q", "trg_fk", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "trigger" {
		t.Fatalf("expected object type %q, got %q", "trigger", s.DDL.ObjectType)
	}
	if s.DDL.Options["constraint"] != "true" {
		t.Fatalf("expected constraint=true, got %q", s.DDL.Options["constraint"])
	}
	if s.DDL.Table == nil || s.DDL.Table.Name != "orders" {
		t.Fatalf("expected table name %q, got %#v", "orders", s.DDL.Table)
	}
}

func TestPGExtractorDropTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "DROP TRIGGER trg_audit ON users")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if s.Unsupported != nil {
		t.Fatalf("expected supported, got feature=%q reason=%q", s.Unsupported.Feature, s.Unsupported.Reason)
	}
	if s.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
	if s.DDL.Operation != spec.DDLOperationDropTrigger {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationDropTrigger, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "trg_audit" {
		t.Fatalf("expected object name %q, got %q", "trg_audit", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "trigger" {
		t.Fatalf("expected object type %q, got %q", "trigger", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "" {
		t.Fatalf("expected no if_exists for plain DROP TRIGGER, got %q", s.DDL.Options["if_exists"])
	}
}

func TestPGExtractorDropTriggerIfExistsNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "DROP TRIGGER IF EXISTS trg_audit ON users")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if s.Unsupported != nil {
		t.Fatalf("expected supported, got feature=%q reason=%q", s.Unsupported.Feature, s.Unsupported.Reason)
	}
	if s.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
	if s.DDL.Operation != spec.DDLOperationDropTrigger {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationDropTrigger, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "trg_audit" {
		t.Fatalf("expected object name %q, got %q", "trg_audit", s.DDL.ObjectName)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Fatalf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
}
