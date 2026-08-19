// Package ddl verifies MySQL/TiDB ALTER TABLE action notice wording.
// input: synthetic alter-table statements for add/drop column notices
// output: coverage that offline-safe notices do not claim live-schema existence
// pos: domain DDL rule test coverage for MySQL/TiDB alter action notices
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAlterDropColumnNoticeDoesNotClaimColumnExists(t *testing.T) {
	t.Parallel()
	rule, err := newAlterDropColumnNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new drop column notice rule: %v", err)
	}

	statement := alterStatement(spec.Alter{Action: "drop_column", Name: "not_a_col"})
	statement.Dialect = spec.DialectMySQL

	findings, err := rule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	message := findings[0].Message
	if strings.Contains(message, "existing column") || strings.Contains(message, "removes an existing") {
		t.Fatalf("notice must not claim the column exists, got %q", message)
	}
	if !strings.Contains(message, `would drop column "not_a_col" if it exists`) {
		t.Fatalf("expected hypothetical drop wording, got %q", message)
	}
}

func TestAlterAddColumnNoticeKeepsSchemaExpansionWording(t *testing.T) {
	t.Parallel()
	rule, err := newAlterAddColumnNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new add column notice rule: %v", err)
	}

	statement := alterStatement(spec.Alter{Action: "add_columns", Name: "x"})
	statement.Dialect = spec.DialectMySQL

	findings, err := rule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if strings.Contains(findings[0].Message, "existing") {
		t.Fatalf("add-column notice must not claim the table exists, got %q", findings[0].Message)
	}
}

func TestFormatAlterActionNoticeSupportsNamedAndLegacyTemplates(t *testing.T) {
	t.Parallel()
	got := formatAlterActionNotice("ALTER TABLE %s DROP COLUMN would drop column %q if it exists", "users", "email")
	if got != `ALTER TABLE users DROP COLUMN would drop column "email" if it exists` {
		t.Fatalf("named template: got %q", got)
	}
	got = formatAlterActionNotice("ALTER TABLE %s ADD COLUMN adds a new column", "users", "x")
	if got != `ALTER TABLE users ADD COLUMN adds a new column for "x"` {
		t.Fatalf("legacy template: got %q", got)
	}
	got = formatAlterActionNotice("ALTER TABLE %s ADD CONSTRAINT adds a new constraint", "users", "")
	if got != "ALTER TABLE users ADD CONSTRAINT adds a new constraint" {
		t.Fatalf("nameless template: got %q", got)
	}
}
