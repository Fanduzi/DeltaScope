//go:build postgresql

package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestUnsupportedStatementDiagnosticEvidencePostgreSQL(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected unsupported statement error")
	}
	if len(result.Unsupported) == 0 {
		t.Fatalf("expected unsupported details, got result=%#v", result)
	}

	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected unsupported diagnostic evidence, got result.Diagnostics empty")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Classification == "unsupported_statement" {
			found = true
			if d.Audited {
				t.Fatal("unsupported_statement diagnostic must mark audited=false")
			}
			if !strings.Contains(d.ActionHint, "review it manually") {
				t.Fatalf("expected manual-review action hint, got %q", d.ActionHint)
			}
			if d.Dialect != "postgresql" {
				t.Fatalf("expected dialect postgresql, got %q", d.Dialect)
			}
			if strings.Contains(d.Reason+d.ActionHint, "old_name") {
				t.Fatal("unsupported diagnostic leaked raw SQL column name")
			}
		}
	}
	if !found {
		t.Fatalf("expected unsupported_statement diagnostic, got classifications: %+v",
			classificationsOf(result.Diagnostics))
	}
}

func classificationsOf(ds []spec.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Classification
	}
	return out
}
