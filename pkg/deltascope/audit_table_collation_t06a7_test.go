// Package deltascope verifies the T06-A7 table-collation upgrade at the
// public SDK seam — the declared table COLLATE now returns a normal audit
// result instead of ErrUnsupportedStatement, and a policy rejection stays a
// populated Result rather than an SDK error.
// input: CREATE TABLE statements with declared table COLLATE through Audit
// output: complete/pass under all-off, reject with one pinned blocker under the required profile, ErrUnsupportedStatement gone
// pos: public SDK contract test for issue #85 T06-A7
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

const t06a7TableCollateRuleID = "ddl.table.collation.allowlist"

// t06A7SDKPolicy writes a policy enabling the named rules (or none) so each
// representative binds its exact profile.
func t06A7SDKPolicy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if params, keep := enabled[entry.RuleID]; keep {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestT06A7TableCollateAllOffSDK is the S4 representative: the declared table
// COLLATE under an all-off policy returns a populated complete/pass Result —
// no ErrUnsupportedStatement, no unsupported entries.
func TestT06A7TableCollateAllOffSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;",
		Dialect:    DialectMySQL,
		ConfigPath: t06A7SDKPolicy(t, map[string]string{}),
	})
	if err != nil {
		t.Fatalf("audit: %v (previously ErrUnsupportedStatement)", err)
	}
	if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", result.Unsupported)
	}
}

// TestT06A7TableCollateDeniedSDK is the policy representative: a disallowed
// table collation produces exactly one blocker as ordinary result data.
func TestT06A7TableCollateDeniedSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;",
		Dialect: DialectMySQL,
		ConfigPath: t06A7SDKPolicy(t, map[string]string{
			t06a7TableCollateRuleID: "      values: [utf8mb4_bin]\n      require_explicit: false\n",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	findings := result.Statements[0].Findings
	if len(findings) != 1 || findings[0].RuleID != t06a7TableCollateRuleID || findings[0].Level != "blocker" {
		t.Fatalf("findings = %#v, want one %s blocker", findings, t06a7TableCollateRuleID)
	}
	if findings[0].Metadata["option"] != "collate" || findings[0].Metadata["actual"] != "utf8mb4_general_ci" {
		t.Fatalf("metadata = %#v, want collate/actual", findings[0].Metadata)
	}
}

// TestT06A7CoexistingUnsupportedSDK pins that a remaining unaudited aspect
// still returns the documented ErrUnsupportedStatement contract.
func TestT06A7CoexistingUnsupportedSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin COMPRESSION='zlib';",
		Dialect:    DialectMySQL,
		ConfigPath: t06A7SDKPolicy(t, map[string]string{}),
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement for the remaining aspect", err)
	}
	found := false
	for _, u := range result.Unsupported {
		if u.Feature == "create_table.option.compression" {
			found = true
		}
		if u.Feature == "create_table.option.collate" {
			t.Fatalf("collate reappeared as unsupported: %#v", u)
		}
	}
	if !found {
		t.Fatalf("unsupported = %#v, want compression aspect", result.Unsupported)
	}
}
