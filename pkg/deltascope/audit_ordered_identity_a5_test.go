// Package deltascope verifies the public column-identity contract (T05-A5).
// input: public audit requests under the isolated eleven-rule policy for
// CHANGE publication, a MySQL 5.7 RENAME rejection, a missing version, and a
// provider error
// output: nil-error pass, reject, and gap results, plus a wrapped provider error
// pos: public SDK contract tests for CHANGE and RENAME COLUMN post-state
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func writeT05A5IdentityPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.change_column.exists.require":        "",
		"ddl.alter.change_column.compatibility.require": "      required: true\n",
		"ddl.alter.rename_column.exists.require":        "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
		"ddl.alter.change_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.version.require":       "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	for id, params := range enabled {
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		if params != "" {
			builder.WriteString("    params:\n")
			builder.WriteString(params)
		}
	}
	path := filepath.Join(t.TempDir(), "t05-a5-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A5ChangeSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_c2 ON t(c2);"

func TestAuditT05A5ChangePass(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: false, Schema: "golden"}}
			result, err := Audit(context.Background(), Request{
				SQL:              t05A5ChangeSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       writeT05A5IdentityPolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 4 || result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete {
				t.Fatalf("aggregate = %s/%s statements %d", result.Coverage.Status, result.Verdict, len(result.Statements))
			}
			for i, statement := range result.Statements {
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %+v", i, statement)
				}
			}
			if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "t" {
				t.Fatalf("provider calls = %#v", provider.tableCalls)
			}
		})
	}
}

func TestAuditT05A5RenameVersionReject(t *testing.T) {
	t.Parallel()
	sql := strings.Replace(t05A5ChangeSQL, "CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL", "RENAME COLUMN c TO c2", 1)
	result, err := Audit(context.Background(), Request{
		SQL:           sql,
		Dialect:       DialectMySQL,
		Schema:        "golden",
		ConfigPath:    writeT05A5IdentityPolicy(t),
		TargetVersion: "5.7.44",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReject || result.Statements[1].Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate = %s/%s", result.Coverage.Status, result.Verdict)
	}
	findings := result.Statements[1].Findings
	if len(findings) != 1 || findings[0].RuleID != "ddl.alter.rename_column.version.require" || findings[0].Metadata["target_version"] != "5.7.44" || findings[0].Metadata["minimum_supported_version"] != "8.0.3" {
		t.Fatalf("finding = %#v", findings)
	}
	if len(result.Statements[2].EvidenceGaps) == 0 {
		t.Fatal("incompatible RENAME published c2")
	}
}

func TestAuditT05A5RenameMissingVersionGap(t *testing.T) {
	t.Parallel()
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT);\nALTER TABLE t RENAME COLUMN c TO c2;",
		Dialect:    DialectMySQL,
		Schema:     "golden",
		ConfigPath: writeT05A5IdentityPolicy(t),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReview || result.Coverage.Status != CoverageUnverified {
		t.Fatalf("aggregate = %s/%s", result.Coverage.Status, result.Verdict)
	}
	gaps := result.Statements[1].EvidenceGaps
	if len(result.Statements[1].Findings) != 0 || len(gaps) != 1 || gaps[0].RuleID != "ddl.alter.rename_column.version.require" || gaps[0].ReasonCode != "missing_target_version" {
		t.Fatalf("rename = findings %#v gaps %#v", result.Statements[1].Findings, gaps)
	}
}

func TestAuditT05A5ProviderError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("t05-a5 provider down")
	_, err := Audit(context.Background(), Request{
		SQL:              t05A5ChangeSQL,
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT05A5IdentityPolicy(t),
		MetadataProvider: &fakeMetadataProvider{err: sentinel},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("provider error = %v, want the sentinel", err)
	}
}
