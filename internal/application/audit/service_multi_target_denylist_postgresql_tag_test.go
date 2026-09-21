//go:build postgresql

// Package audit verifies the application audit service behavior.
// input: PostgreSQL multi-target DROP statements naming distinct dotted qualified identities
// output: one denylist finding per distinct protected object without cross-identity dedup
// pos: application-layer regression coverage for multi-target denylist identity handling
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLDenylistKeepsDistinctPostgreSQLDottedIdentities(t *testing.T) {
	t.Parallel()

	// "a.b"."c" and "a"."b.c" are different objects; a flat "a.b.c"
	// dedup key would wrongly merge them into one finding.
	configPath := writeDenylistPolicy(t, "      tables: [c, b.c]\n")
	result, err := AuditSQL(context.Background(), Request{
		SQL:        `DROP TABLE "a.b"."c", "a"."b.c"`,
		Dialect:    spec.DialectPostgreSQL,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 top-level statement, got %d", len(result.Statements))
	}
	var denylist []rule.Finding
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.denylist.forbid" {
			denylist = append(denylist, finding)
		}
	}
	if len(denylist) != 2 {
		t.Fatalf("expected 2 findings for distinct dotted identities, got %#v", result.Statements[0].Findings)
	}
	if denylist[0].Metadata["schema"] != "a.b" || denylist[0].Metadata["table"] != "c" {
		t.Fatalf("expected first finding a.b/c, got %#v", denylist[0].Metadata)
	}
	if denylist[1].Metadata["schema"] != "a" || denylist[1].Metadata["table"] != "b.c" {
		t.Fatalf("expected second finding a/b.c, got %#v", denylist[1].Metadata)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
}
