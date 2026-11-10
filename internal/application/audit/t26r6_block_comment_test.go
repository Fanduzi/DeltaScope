package audit

// T26-R6 entry-layer assertions (issue #105): an unclosed block
// comment must reach the fixed parser_error diagnostic path instead of
// a successful T26 claim. Diagnostic path and statement retention are
// asserted at the audit entry itself; the dedicated marker must not
// appear in any product output field.

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT26R6_UnclosedBlockCommentDiagnosticPath(t *testing.T) {
	const marker = "t26-comment-marker"
	res, err := t26Audit(t, spec.DialectMySQL, "DROP SERVER s /* "+marker)
	if err == nil {
		t.Fatalf("unclosed block comment audited without error; res=%+v", res.Statements)
	}
	if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
		t.Fatalf("diagnostics = %+v, want parser_error on the fixed diagnostic path", res.Diagnostics)
	}
	if len(res.Statements) != 0 {
		t.Fatalf("statements = %d, want 0 — unclaimed input must not retain a statement", len(res.Statements))
	}
	if len(res.Unsupported) != 0 {
		t.Fatalf("unsupported = %+v, want none — no named boundary may fire on unclaimed input", res.Unsupported)
	}
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Reason, marker) || strings.Contains(d.ActionHint, marker) {
			t.Fatalf("diagnostic leaks marker: %+v", d)
		}
	}
	for _, s := range res.Statements {
		if strings.Contains(s.RawSQL, marker) || strings.Contains(s.NormalizedSQL, marker) {
			t.Fatalf("statement leaks marker: %+v", s)
		}
	}
}
