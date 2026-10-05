// Package deltascope verifies the T05-A7 ordered-state budget at the public SDK seam.
// input: a 1025-statement MySQL audit request through the public Audit entrypoint
// output: ErrUnsupportedStatement with the retained partial result and the exact bounded resource-limit projection
// pos: public SDK contract test for the shared ordered-state admission budget (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestT05A7AuditBudgetsOrderedStatements(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     strings.Repeat("SELECT 1;\n", 1025),
		Dialect: DialectMySQL,
		Schema:  "golden",
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	if len(result.Statements) != 1025 {
		t.Fatalf("statements = %d, want 1025 retained", len(result.Statements))
	}
	for i := 0; i < 1024; i++ {
		statement := result.Statements[i]
		if statement.Index != i || statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("admitted statement %d = %+v, want complete with no findings or gaps", i, statement)
		}
	}
	last := result.Statements[1024]
	if last.Index != 1024 || last.Kind != "unknown" || strings.TrimSpace(last.RawSQL) != "SELECT 1;" || last.NormalizedSQL == "" {
		t.Fatalf("blocked statement identity = %+v, want index 1024 kind unknown raw SELECT 1;", last)
	}
	if last.Coverage.Status != CoverageIncomplete || len(last.Findings) != 0 || len(last.EvidenceGaps) != 0 || last.Impact != nil {
		t.Fatalf("blocked statement = %+v, want incomplete with no findings, gaps, or impact", last)
	}
	if result.Verdict != VerdictReview || result.Coverage.Status != CoverageIncomplete {
		t.Fatalf("aggregate = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("global findings = %+v, want none", result.GlobalFindings)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("finding counters = %+v, want zero", result.Summary)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported entries = %d, want 1: %+v", len(result.Unsupported), result.Unsupported)
	}
	item := result.Unsupported[0]
	if item.Feature != "audit.resource_limit" || item.Reason != "ordered-state statement budget exhausted" {
		t.Fatalf("unsupported entry = %+v, want the audit.resource_limit budget projection", item)
	}
	if item.Index != 1024 || item.SQL != last.RawSQL {
		t.Fatalf("unsupported identity = index %d sql %q, want index 1024 bound to the blocked statement", item.Index, item.SQL)
	}
	wantMeta := map[string]any{
		"phase":    "ordered_state",
		"resource": "statements",
		"limit":    1024,
		"consumed": 1024,
		"line":     1025,
		"column":   1,
	}
	if !reflect.DeepEqual(item.Metadata, wantMeta) {
		t.Fatalf("resource metadata = %+v, want %+v", item.Metadata, wantMeta)
	}
}
