//go:build postgresql

// Package audit verifies PostgreSQL parser-error and partial-recovery contracts.
// input: PostgreSQL SQL with parser-error boundaries and valid surrounding statements
// output: fail-closed diagnostics plus preserved valid PostgreSQL audit results and locations
// pos: PostgreSQL-tagged application audit parser recovery regression coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestDDLParserErrorUnsupportedContractPostgreSQL(t *testing.T) {
	t.Parallel()

	cases := []parserErrorUnsupportedContractCase{
		{
			Dialect:   spec.DialectPostgreSQL,
			Name:      "postgres_drop_subscription_with_options_not_audited",
			SQL:       "DROP SUBSCRIPTION sub WITH (drop_slot = true)",
			Forbidden: []string{"drop_slot"},
		},
		{
			Dialect:   spec.DialectPostgreSQL,
			Name:      "postgres_pg18_constraint_not_audited",
			SQL:       "ALTER TABLE users ADD CONSTRAINT users_email_nn NOT NULL email NOT VALID",
			Forbidden: []string{"users_email_nn", "NOT VALID"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			assertParserErrorUnsupportedContract(t, tc)
		})
	}
}

func TestAuditParserRecoveryPostgreSQLPreservesDollarQuotedStatementAndTrailingFinding(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE FUNCTION audit_marker() RETURNS void AS $$ BEGIN PERFORM 1; END; $$ LANGUAGE plpgsql;\n" +
			"CREATE INDEX idx_x ON;\n" +
			"DELETE FROM users;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected parser-error result to remain non-nil")
	}
	if len(result.Statements) != 2 || result.Summary.Statements != 2 {
		t.Fatalf("expected two audited PostgreSQL statements, got %#v", result.Statements)
	}
	if !strings.HasPrefix(result.Statements[0].NormalizedSQL, "CREATE FUNCTION audit_marker") {
		t.Fatalf("expected dollar-quoted function to remain one statement, got %#v", result.Statements[0])
	}
	if result.Statements[1].NormalizedSQL != "DELETE FROM users" {
		t.Fatalf("expected trailing DELETE to be preserved, got %#v", result.Statements[1])
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Column != 1 {
		t.Fatalf("expected one located PostgreSQL parser diagnostic, got %#v", result.Diagnostics)
	}
	foundDeleteFinding := false
	for _, finding := range result.Statements[1].Findings {
		foundDeleteFinding = foundDeleteFinding || finding.RuleID == "dml.where.require"
	}
	if !foundDeleteFinding {
		t.Fatalf("expected trailing DELETE finding, got %#v", result.Statements[1].Findings)
	}
	if strings.Contains(result.Diagnostics[0].Reason+result.Diagnostics[0].ActionHint, "idx_x") {
		t.Fatalf("parser diagnostic leaked SQL text: %#v", result.Diagnostics[0])
	}
}

func TestAuditParserRecoveryPostgreSQLDoesNotSplitEscapeString(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "INSERT INTO users(name) VALUES (E'a\\';b');\nCREATE INDEX idx_x ON;\nDELETE FROM users;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected parser-error sentinel")
	}
	if len(result.Statements) != 2 {
		t.Fatalf("expected escape-string INSERT and DELETE to remain intact, got %#v", result.Statements)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Line != 2 {
		t.Fatalf("expected only the line-2 statement to be unaudited, got %#v", result.Diagnostics)
	}
}

func TestAuditParserRecoveryReportsParserAndUnsupportedDiagnosticsTogether(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE users ADD COLUMN x INT;\nCREATE INDEX idx_x ON;\nSELECT 1;\nDELETE FROM users;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected fail-closed parser error")
	}
	if len(result.Statements) != 2 || len(result.Unsupported) != 1 {
		t.Fatalf("expected supported and unsupported results to survive, got %#v", result)
	}
	classifications := classificationsOf(result.Diagnostics)
	if !slices.Contains(classifications, "parser_error") || !slices.Contains(classifications, "unsupported_statement") {
		t.Fatalf("expected both bounded diagnostics, got %#v", result.Diagnostics)
	}
}
