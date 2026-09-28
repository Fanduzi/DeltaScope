// Package deltascope verifies the public target_version contract (T04-B/#83).
// input: public audit requests carrying TargetVersion under the isolated key-length policy
// output: typed input errors, canonical VersionIdentity projection, and bounded version evidence gaps
// pos: public SDK contract tests for version evidence
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
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t04bKeyLengthRuleID = "ddl.index.key_length.max_bytes.require"
const t04bGoldenSQL = "CREATE TABLE t (c VARCHAR(255) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

// t04bSmallIndexSQL keeps a ≤767-byte utf8mb4 index: complete under every
// candidate page-size bound even without instance facts.
const t04bSmallIndexSQL = "CREATE TABLE t (c VARCHAR(191) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

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

// The public request forwards TargetVersion to the shared parser: empty stays
// absent (bounded gap), a valid string completes and projects the canonical
// identity, and malformed input is the typed ErrInvalidTargetVersion.
func TestAuditT04BTargetVersionContract(t *testing.T) {
	t.Parallel()
	configPath := writeT04BIsolatedPolicy(t)

	t.Run("absent version yields bounded gap", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:        t04bGoldenSQL,
			Dialect:    DialectMySQL,
			ConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if result.Coverage.Status != CoverageUnverified || result.Verdict != VerdictReview {
			t.Fatalf("expected unverified/review, got %q/%q", result.Coverage.Status, result.Verdict)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_target_version" || gaps[0].RuleID != t04bKeyLengthRuleID {
			t.Fatalf("gaps = %#v", gaps)
		}
		if result.Version != nil {
			t.Fatalf("expected nil version, got %#v", result.Version)
		}
	})

	t.Run("valid version projects canonical identity", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:           t04bSmallIndexSQL,
			Dialect:       DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "v8.4.10",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if result.Coverage.Status != CoverageComplete || result.Verdict != VerdictPass {
			t.Fatalf("expected complete/pass, got %q/%q", result.Coverage.Status, result.Verdict)
		}
		if result.Version == nil {
			t.Fatalf("expected version identity")
		}
		if result.Version.Product != spec.VersionProductMySQL || result.Version.Version != "8.4.10" || result.Version.Source != spec.VersionSourceTarget || !result.Version.ValidatedRange {
			t.Fatalf("version = %#v", result.Version)
		}
	})

	t.Run("valid version with unknown page size degrades to instance fact gap", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:           t04bGoldenSQL,
			Dialect:       DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "8.4.10",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_instance_fact" || gaps[0].RuleID != t04bKeyLengthRuleID {
			t.Fatalf("gaps = %#v", gaps)
		}
		if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.innodb_page_size" {
			t.Fatalf("required_facts = %#v", gaps[0].RequiredFacts)
		}
	})

	t.Run("malformed version returns typed input error", func(t *testing.T) {
		for _, raw := range []string{"8.4", "8.4.10-extra", "8.0.11-TiDB-v8.5.0"} {
			result, err := Audit(context.Background(), Request{
				SQL:           t04bGoldenSQL,
				Dialect:       DialectMySQL,
				ConfigPath:    configPath,
				TargetVersion: raw,
			})
			if !errors.Is(err, ErrInvalidTargetVersion) {
				t.Fatalf("target %q: expected ErrInvalidTargetVersion, got result=%#v err=%v", raw, result, err)
			}
		}
	})

	t.Run("out-of-range version returns range gap", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:           t04bGoldenSQL,
			Dialect:       DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "8.3.0",
		})
		if err != nil {
			t.Fatalf("expected nil error for valid-but-unvalidated version, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "target_version_out_of_validated_range" {
			t.Fatalf("gaps = %#v", gaps)
		}
		if result.Version == nil || result.Version.ValidatedRange {
			t.Fatalf("expected unvalidated identity, got %#v", result.Version)
		}
	})

	t.Run("tidb valid target projects tidb product", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:           t04bGoldenSQL,
			Dialect:       DialectTiDB,
			ConfigPath:    configPath,
			TargetVersion: "8.5.0",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if result.Version == nil || result.Version.Product != spec.VersionProductTiDB || result.Version.Version != "8.5.0" {
			t.Fatalf("version = %#v", result.Version)
		}
	})
}

// TestAuditT04BProviderFailureContract pins the error/gap boundary at the
// public surface (issue #83 T04-B-R2): a provider that fails stays an error —
// it can never be laundered into an evidence gap — while a provider that
// succeeds but leaves a bound-relevant fact unknown yields
// unverified/review with a nil error and a missing_instance_fact gap.
func TestAuditT04BProviderFailureContract(t *testing.T) {
	t.Parallel()
	configPath := writeT04BIsolatedPolicy(t)
	tidbAmbiguousSQL := "CREATE TABLE t (c VARCHAR(1000) CHARACTER SET utf8mb4, KEY idx_c (c));"
	tidbFacts := &InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}

	t.Run("provider failure stays an error", func(t *testing.T) {
		wantErr := errors.New("tidb config read failed")
		result, err := Audit(context.Background(), Request{
			SQL:              tidbAmbiguousSQL,
			Dialect:          DialectTiDB,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: &fakeMetadataProvider{err: wantErr},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected provider error, got result=%#v err=%v", result, err)
		}
	})

	t.Run("successful read with unknown fact is unverified gap not error", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:              tidbAmbiguousSQL,
			Dialect:          DialectTiDB,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: &fakeMetadataProvider{instance: tidbFacts},
		})
		if err != nil {
			t.Fatalf("successful read with unknown fact must not error, got %v", err)
		}
		if result.Coverage.Status != CoverageUnverified || result.Verdict != VerdictReview {
			t.Fatalf("expected unverified/review, got %q/%q", result.Coverage.Status, result.Verdict)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_instance_fact" || gaps[0].RuleID != t04bKeyLengthRuleID {
			t.Fatalf("gaps = %#v", gaps)
		}
		if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.tidb_max_index_length" {
			t.Fatalf("required_facts = %#v", gaps[0].RequiredFacts)
		}
	})
}
