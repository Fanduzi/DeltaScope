// Package cli verifies the T04-B/#83 target-version CLI contract.
// input: audit invocations with --target-version under the isolated key-length policy
// output: exit-2 input errors, canonical version projection, and bounded version evidence gaps
// pos: CLI transport contract tests for version evidence (issue #83 T04-B slice)
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

const t04bKeyLengthRuleID = "ddl.index.key_length.max_bytes.require"
const t04bGoldenSQL = "CREATE TABLE t (c VARCHAR(255) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

func writeT04BIsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04b-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04bKeyLengthRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

// t04bSmallIndexSQL keeps a ≤767-byte utf8mb4 index: complete under every
// candidate page-size bound even without instance facts.
const t04bSmallIndexSQL = "CREATE TABLE t (c VARCHAR(191) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

func runT04BAudit(t *testing.T, configPath string, extraArgs ...string) (map[string]any, int, string) {
	t.Helper()
	return runT04BAuditSQL(t, configPath, t04bGoldenSQL, extraArgs...)
}

func runT04BAuditSQL(t *testing.T, configPath, sql string, extraArgs ...string) (map[string]any, int, string) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	args := []string{
		"audit", "--sql", sql, "--dialect", "mysql",
		"--format", "json", "--config", configPath,
	}
	args = append(args, extraArgs...)
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	var decoded map[string]any
	if code == exitOK || code == exitAudit {
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("unmarshal: %v\noutput=%s\nstderr=%s", err, stdout.String(), stderr.String())
		}
	}
	return decoded, code, stderr.String()
}

func t04bStatementGap(decoded map[string]any) map[string]any {
	stmts, _ := decoded["statements"].([]any)
	if len(stmts) != 1 {
		return nil
	}
	first, _ := stmts[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		return nil
	}
	gap, _ := gaps[0].(map[string]any)
	return gap
}

// Missing target_version produces the bounded gap through the real CLI path;
// a strict target_version completes and projects the canonical identity.
func TestAuditCommandT04BTargetVersionLifecycle(t *testing.T) {
	configPath := writeT04BIsolatedPolicy(t)

	t.Run("absent version yields missing_target_version gap", func(t *testing.T) {
		decoded, code, _ := runT04BAudit(t, configPath, "--fail-on", "warning")
		if code != exitAudit {
			t.Fatalf("expected exit %d, got %d", exitAudit, code)
		}
		gap := t04bStatementGap(decoded)
		if gap == nil || gap["reason_code"] != "missing_target_version" || gap["rule_id"] != t04bKeyLengthRuleID {
			t.Fatalf("gap = %#v", gap)
		}
		facts, _ := gap["required_facts"].([]any)
		if len(facts) != 1 || facts[0] != "target.version" {
			t.Fatalf("required_facts = %#v", gap["required_facts"])
		}
		if coverage, _ := decoded["coverage"].(map[string]any); coverage["status"] != "unverified" {
			t.Fatalf("expected unverified, got %#v", decoded["coverage"])
		}
	})

	t.Run("valid version completes and projects identity", func(t *testing.T) {
		decoded, code, _ := runT04BAuditSQL(t, configPath, t04bSmallIndexSQL, "--target-version", "8.4.10")
		if code != exitOK {
			t.Fatalf("expected exit %d, got %d", exitOK, code)
		}
		if decoded["verdict"] != "pass" {
			t.Fatalf("expected pass, got %#v", decoded["verdict"])
		}
		version, _ := decoded["version"].(map[string]any)
		if version["product"] != "mysql" || version["version"] != "8.4.10" || version["source"] != "target" || version["validated_range"] != true {
			t.Fatalf("version = %#v", version)
		}
	})

	t.Run("valid version with unknown page size yields instance fact gap", func(t *testing.T) {
		decoded, code, _ := runT04BAudit(t, configPath, "--target-version", "8.4.10")
		if code != exitOK {
			t.Fatalf("expected exit %d, got %d", exitOK, code)
		}
		gap := t04bStatementGap(decoded)
		if gap == nil || gap["reason_code"] != "missing_instance_fact" || gap["rule_id"] != t04bKeyLengthRuleID {
			t.Fatalf("gap = %#v", gap)
		}
		facts, _ := gap["required_facts"].([]any)
		if len(facts) != 1 || facts[0] != "instance.innodb_page_size" {
			t.Fatalf("required_facts = %#v", gap["required_facts"])
		}
		if coverage, _ := decoded["coverage"].(map[string]any); coverage["status"] != "unverified" {
			t.Fatalf("expected unverified, got %#v", decoded["coverage"])
		}
	})

	t.Run("v prefix canonicalizes", func(t *testing.T) {
		decoded, code, _ := runT04BAudit(t, configPath, "--target-version", "v8.0.46")
		if code != exitOK {
			t.Fatalf("expected exit %d, got %d", exitOK, code)
		}
		version, _ := decoded["version"].(map[string]any)
		if version["version"] != "8.0.46" {
			t.Fatalf("expected canonical 8.0.46, got %#v", version)
		}
	})

	t.Run("out-of-range yields range gap", func(t *testing.T) {
		decoded, code, _ := runT04BAudit(t, configPath, "--target-version", "8.3.0")
		if code != exitOK {
			t.Fatalf("expected exit %d, got %d", exitOK, code)
		}
		gap := t04bStatementGap(decoded)
		if gap == nil || gap["reason_code"] != "target_version_out_of_validated_range" {
			t.Fatalf("gap = %#v", gap)
		}
		if version, _ := decoded["version"].(map[string]any); version["validated_range"] != false {
			t.Fatalf("expected validated_range=false, got %#v", version)
		}
	})
}

// The missing-version gap carries warning-equivalent fail-on weight exactly
// like metadata gaps: warning/notice exit 1, blocker/none exit 0.
func TestAuditCommandT04BMissingVersionFailOnMatrix(t *testing.T) {
	configPath := writeT04BIsolatedPolicy(t)

	cases := map[string]int{
		"warning": exitAudit,
		"notice":  exitAudit,
		"blocker": exitOK,
		"none":    exitOK,
	}
	for threshold, want := range cases {
		t.Run(threshold, func(t *testing.T) {
			decoded, code, _ := runT04BAudit(t, configPath, "--fail-on", threshold)
			if code != want {
				t.Fatalf("fail-on %s: expected exit %d, got %d", threshold, want, code)
			}
			if t04bStatementGap(decoded) == nil {
				t.Fatalf("fail-on %s: expected missing_target_version gap", threshold)
			}
			if decoded["verdict"] != "review" {
				t.Fatalf("fail-on %s: expected review verdict, got %#v", threshold, decoded["verdict"])
			}
		})
	}
}

// Malformed target_version is an input error: exit 2, no audit payload.
func TestAuditCommandT04BMalformedTargetVersionExitsUser(t *testing.T) {
	configPath := writeT04BIsolatedPolicy(t)

	for _, raw := range []string{"8.4", "8.4.x", "8.4.10-extra", "8.0.11-TiDB-v8.5.0"} {
		t.Run(raw, func(t *testing.T) {
			decoded, code, stderrText := runT04BAudit(t, configPath, "--target-version", raw)
			if code != exitUser {
				t.Fatalf("expected exit %d for malformed %q, got %d", exitUser, raw, code)
			}
			if decoded != nil {
				t.Fatalf("expected no audit payload for malformed input, got %#v", decoded)
			}
			if !strings.Contains(stderrText, "target_version") {
				t.Fatalf("expected target_version error text, got %q", stderrText)
			}
		})
	}
}

// A malformed target_version is validated before any connection lookup or
// open: even an unreachable host/port must lose to the input error.
func TestAuditCommandT04BPreflightPrecedesConnection(t *testing.T) {
	configPath := writeT04BIsolatedPolicy(t)

	decoded, code, stderrText := runT04BAudit(t, configPath,
		"--target-version", "8.4",
		"--host", "127.0.0.1", "--port", "1", "--user", "root", "--schema", "app",
	)
	if code != exitUser {
		t.Fatalf("expected exit %d for malformed target before connect, got %d", exitUser, code)
	}
	if decoded != nil {
		t.Fatalf("expected no audit payload, got %#v", decoded)
	}
	if !strings.Contains(stderrText, "target_version") {
		t.Fatalf("expected target_version error before connection, got %q", stderrText)
	}
	if strings.Contains(stderrText, "connection") || strings.Contains(stderrText, "refused") {
		t.Fatalf("connection must not be attempted before target_version preflight, got %q", stderrText)
	}
}
