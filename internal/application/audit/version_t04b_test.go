// Package audit verifies the T04-B/#83 target_version and observed-version evidence contract.
// input: audit requests carrying strict target_version, optional observed provider banners, and the isolated key-length policy
// output: version identity propagation, bounded evidence gaps, typed input errors, and preserved mixed-aggregation semantics
// pos: application audit version-evidence regression tests for issue #83 (T04-B slice)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t04bKeyLengthRule = "ddl.index.key_length.max_bytes.require"
const t04bPrimaryKeyRule = "ddl.table.primary_key.require"

const t04bDynamicSQL = "CREATE TABLE t (c VARCHAR(255) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

// t04bSmallIndexSQL keeps a ≤767-byte utf8mb4 index so the audit is complete
// under every candidate page-size bound even when instance facts are absent.
const t04bSmallIndexSQL = "CREATE TABLE t (c VARCHAR(191) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

func writeT04BPolicy(t *testing.T, extra map[string]policy.RulePolicy) string {
	t.Helper()
	overrides := map[string]policy.RulePolicy{
		t04bKeyLengthRule: {
			Enabled: true,
			Level:   "blocker",
			Params:  map[string]any{"required": true},
		},
	}
	for id, rp := range extra {
		overrides[id] = rp
	}
	return writeIsolatedPolicy(t, overrides)
}

// TestAuditSQLT04BOfflineTargetVersionMatrix covers the offline matrix: absent
// version is a bounded gap (never a silent default), a strict target_version is
// the statement's version fact, malformed input is a typed error, and a valid
// version outside the validated series degrades to the range gap.
func TestAuditSQLT04BOfflineTargetVersionMatrix(t *testing.T) {
	t.Parallel()
	configPath := writeT04BPolicy(t, nil)

	t.Run("missing target version yields gap review unverified", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL,
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		statement := result.Statements[0]
		if len(statement.Findings) != 0 {
			t.Fatalf("expected zero findings, got %#v", statement.Findings)
		}
		if len(statement.EvidenceGaps) != 1 {
			t.Fatalf("expected exactly one gap, got %#v", statement.EvidenceGaps)
		}
		gap := statement.EvidenceGaps[0]
		if gap.RuleID != t04bKeyLengthRule || gap.ReasonCode != "missing_target_version" {
			t.Fatalf("gap = %#v", gap)
		}
		if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "target.version" {
			t.Fatalf("required facts = %#v", gap.RequiredFacts)
		}
		if statement.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected unverified, got statement=%q aggregate=%q", statement.Coverage.Status, result.Coverage.Status)
		}
		if result.Verdict != report.VerdictReview {
			t.Fatalf("expected review, got %q", result.Verdict)
		}
		if result.Version != nil {
			t.Fatalf("expected nil version without facts, got %#v", result.Version)
		}
	})

	t.Run("valid target version completes when safe under every page bound", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bSmallIndexSQL,
			Dialect:       spec.DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "8.4.10",
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		statement := result.Statements[0]
		if len(statement.EvidenceGaps) != 0 || len(statement.Findings) != 0 {
			t.Fatalf("expected clean result, gaps=%#v findings=%#v", statement.EvidenceGaps, statement.Findings)
		}
		if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
			t.Fatalf("expected complete/pass, got %q/%q", result.Coverage.Status, result.Verdict)
		}
		if result.Version == nil {
			t.Fatalf("expected projected version identity")
		}
		if result.Version.Product != "mysql" || result.Version.Version != "8.4.10" || result.Version.Source != spec.VersionSourceTarget || !result.Version.ValidatedRange {
			t.Fatalf("version identity = %#v", result.Version)
		}
	})

	t.Run("valid target version with unknown page size yields instance fact gap", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "8.4.10",
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		statement := result.Statements[0]
		if len(statement.Findings) != 0 {
			t.Fatalf("expected no fabricated finding, got %#v", statement.Findings)
		}
		if len(statement.EvidenceGaps) != 1 {
			t.Fatalf("expected exactly one gap, got %#v", statement.EvidenceGaps)
		}
		gap := statement.EvidenceGaps[0]
		if gap.RuleID != t04bKeyLengthRule || gap.ReasonCode != "missing_instance_fact" {
			t.Fatalf("gap = %#v", gap)
		}
		if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "instance.innodb_page_size" {
			t.Fatalf("required facts = %#v", gap.RequiredFacts)
		}
		if result.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
			t.Fatalf("expected unverified/review, got %q/%q", result.Coverage.Status, result.Verdict)
		}
	})

	t.Run("v prefix target canonicalizes", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectMySQL,
			ConfigPath:    configPath,
			TargetVersion: "v8.0.46",
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if result.Version == nil || result.Version.Version != "8.0.46" {
			t.Fatalf("expected canonical 8.0.46, got %#v", result.Version)
		}
	})

	t.Run("malformed target versions are input errors", func(t *testing.T) {
		for _, raw := range []string{"8.4", "8.4.x", "8.4.10-extra", "8.0.11-TiDB-v8.5.0", "v"} {
			result, err := AuditSQL(context.Background(), Request{
				SQL:           t04bDynamicSQL,
				Dialect:       spec.DialectMySQL,
				ConfigPath:    configPath,
				TargetVersion: raw,
			})
			if !errors.Is(err, ErrInvalidTargetVersion) {
				t.Fatalf("target %q: expected ErrInvalidTargetVersion, got result=%#v err=%v", raw, result, err)
			}
			if len(result.Statements) != 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("target %q: expected empty result, got %#v", raw, result)
			}
		}
	})

	t.Run("valid out-of-range target degrades to range gap", func(t *testing.T) {
		for _, raw := range []string{"8.3.0", "9.0.0"} {
			result, err := AuditSQL(context.Background(), Request{
				SQL:           t04bDynamicSQL,
				Dialect:       spec.DialectMySQL,
				ConfigPath:    configPath,
				TargetVersion: raw,
			})
			if err != nil {
				t.Fatalf("target %q: expected successful audit, got %v", raw, err)
			}
			statement := result.Statements[0]
			if len(statement.EvidenceGaps) != 1 {
				t.Fatalf("target %q: expected one gap, got %#v", raw, statement.EvidenceGaps)
			}
			gap := statement.EvidenceGaps[0]
			if gap.ReasonCode != "target_version_out_of_validated_range" {
				t.Fatalf("target %q: gap = %#v", raw, gap)
			}
			if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "target.version.validated_range" {
				t.Fatalf("target %q: required facts = %#v", raw, gap.RequiredFacts)
			}
			if result.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
				t.Fatalf("target %q: expected unverified/review, got %q/%q", raw, result.Coverage.Status, result.Verdict)
			}
			if result.Version == nil || result.Version.ValidatedRange {
				t.Fatalf("target %q: expected unvalidated version identity, got %#v", raw, result.Version)
			}
		}
	})

	t.Run("tidb dialect missing version yields gap", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL,
			Dialect:    spec.DialectTiDB,
			ConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_target_version" {
			t.Fatalf("expected missing_target_version gap, got %#v", gaps)
		}
	})

	t.Run("tidb valid target completes", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectTiDB,
			ConfigPath:    configPath,
			TargetVersion: "8.5.0",
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if result.Coverage.Status != report.CoverageComplete || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("expected complete, got %q gaps=%#v", result.Coverage.Status, result.Statements[0].EvidenceGaps)
		}
		if result.Version == nil || result.Version.Product != "tidb" || result.Version.Version != "8.5.0" {
			t.Fatalf("version identity = %#v", result.Version)
		}
	})
}

// TestAuditSQLT04BMixedResultsPreserveSemantics covers the required mixed
// matrix: a version gap coexists with a proven blocker, unsupported stays
// incomplete, parser failure keeps its contract, and version-independent rules
// keep evaluating regardless of the version fact.
func TestAuditSQLT04BMixedResultsPreserveSemantics(t *testing.T) {
	t.Parallel()

	bothRules := writeT04BPolicy(t, map[string]policy.RulePolicy{
		t04bPrimaryKeyRule: {Enabled: true, Level: "blocker"},
	})

	t.Run("version gap plus deterministic blocker rejects unverified", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL,
			Dialect:    spec.DialectMySQL,
			ConfigPath: bothRules,
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		statement := result.Statements[0]
		if len(statement.EvidenceGaps) != 1 || statement.EvidenceGaps[0].ReasonCode != "missing_target_version" {
			t.Fatalf("expected one version gap, got %#v", statement.EvidenceGaps)
		}
		if len(statement.Findings) != 1 || statement.Findings[0].RuleID != t04bPrimaryKeyRule || statement.Findings[0].Level != rule.LevelBlocker {
			t.Fatalf("expected one primary-key blocker, got %#v", statement.Findings)
		}
		if result.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected aggregate unverified, got %q", result.Coverage.Status)
		}
		if result.Verdict != report.VerdictReject {
			t.Fatalf("expected reject, got %q", result.Verdict)
		}
	})

	t.Run("version gap plus unsupported stays incomplete", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL + " CREATE SEQUENCE golden_seq START WITH 1;",
			Dialect:    spec.DialectMySQL,
			ConfigPath: writeT04BPolicy(t, nil),
		})
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
		}
		if result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected first statement unverified, got %q", result.Statements[0].Coverage.Status)
		}
		if len(result.Statements[0].EvidenceGaps) != 1 {
			t.Fatalf("expected retained version gap, got %#v", result.Statements[0].EvidenceGaps)
		}
		if result.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expected aggregate incomplete, got %q", result.Coverage.Status)
		}
	})

	t.Run("valid statement plus parser failure keeps parser contract", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL + " THIS IS NOT VALID SQL AT ALL;",
			Dialect:    spec.DialectMySQL,
			ConfigPath: writeT04BPolicy(t, nil),
		})
		if err == nil {
			t.Fatalf("expected parser-failure error, got %#v", result)
		}
		if len(result.Diagnostics) == 0 {
			t.Fatalf("expected parser diagnostics, got %#v", result.Diagnostics)
		}
		if len(result.Statements) != 1 || result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected retained unverified statement, got %#v", result.Statements)
		}
		if result.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expected aggregate incomplete, got %q", result.Coverage.Status)
		}
	})

	t.Run("version independent rule fires without and out-of-range version", func(t *testing.T) {
		for _, target := range []string{"", "8.3.0"} {
			result, err := AuditSQL(context.Background(), Request{
				SQL:           t04bDynamicSQL,
				Dialect:       spec.DialectMySQL,
				ConfigPath:    bothRules,
				TargetVersion: target,
			})
			if err != nil {
				t.Fatalf("target %q: expected successful audit, got %v", target, err)
			}
			statement := result.Statements[0]
			found := false
			for _, f := range statement.Findings {
				if f.RuleID == t04bPrimaryKeyRule {
					found = true
				}
			}
			if !found {
				t.Fatalf("target %q: expected version-independent primary-key finding, got %#v", target, statement.Findings)
			}
		}
	})
}

// TestAuditSQLT04BObservedVersionReconciliation covers the online matrix: the
// provider banner is the only authoritative version fact, a caller
// target_version is a strict constraint on it, provider errors stay errors,
// and the TiDB compatibility prefix never becomes the version.
func TestAuditSQLT04BObservedVersionReconciliation(t *testing.T) {
	t.Parallel()
	configPath := writeT04BPolicy(t, nil)
	largePrefixOn := &spec.InstanceFacts{Version: "5.7.44", InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true, InnoDBDefaultRowFormat: "DYNAMIC", InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: 16384}
	mysql84Facts := &spec.InstanceFacts{Version: "8.4.10", InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true, InnoDBDefaultRowFormat: "DYNAMIC", InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: 16384}

	t.Run("observed only uses observed without missing gap", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: &t04SnapshotProvider{facts: mysql84Facts},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("expected no gap with observed version, got %#v", result.Statements[0].EvidenceGaps)
		}
		if result.Version == nil || result.Version.Source != spec.VersionSourceObserved || result.Version.Version != "8.4.10" || result.Version.Product != "mysql" {
			t.Fatalf("expected observed mysql/8.4.10, got %#v", result.Version)
		}
	})

	t.Run("mysql 8.4.10 observed plus equal target passes", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			TargetVersion:    "8.4.10",
			MetadataProvider: &t04SnapshotProvider{facts: mysql84Facts},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if result.Version == nil || result.Version.Source != spec.VersionSourceObserved || result.Version.Version != "8.4.10" {
			t.Fatalf("expected observed identity, got %#v", result.Version)
		}
	})

	t.Run("mysql 8.4.10 observed plus 8.0.46 target mismatches", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			TargetVersion:    "8.0.46",
			MetadataProvider: &t04SnapshotProvider{facts: mysql84Facts},
		})
		if !errors.Is(err, ErrTargetVersionMismatch) {
			t.Fatalf("expected ErrTargetVersionMismatch, got result=%#v err=%v", result, err)
		}
	})

	t.Run("tidb compatibility banner plus target 8.5.0 passes", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectTiDB,
			ConfigPath:    configPath,
			Schema:        "app",
			TargetVersion: "8.5.0",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "8.0.11-TiDB-v8.5.0", InnoDBDefaultRowFormat: "DYNAMIC", TiDBMaxIndexLengthKnown: true, TiDBMaxIndexLengthBytes: 3072,
			}},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if result.Version == nil || result.Version.Product != "tidb" || result.Version.Version != "8.5.0" {
			t.Fatalf("expected canonical tidb/8.5.0, got %#v", result.Version)
		}
	})

	t.Run("tidb banner plus target 8.0.11 compatibility prefix mismatches", func(t *testing.T) {
		_, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectTiDB,
			ConfigPath:    configPath,
			Schema:        "app",
			TargetVersion: "8.0.11",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "8.0.11-TiDB-v8.5.0", InnoDBDefaultRowFormat: "DYNAMIC",
			}},
		})
		if !errors.Is(err, ErrTargetVersionMismatch) {
			t.Fatalf("expected ErrTargetVersionMismatch, got %v", err)
		}
	})

	t.Run("mysql dialect plus tidb banner is a typed product mismatch", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectMySQL,
			ConfigPath:    configPath,
			Schema:        "app",
			TargetVersion: "8.5.0",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "8.0.11-TiDB-v8.5.0",
			}},
		})
		if !errors.Is(err, ErrDialectProductMismatch) {
			t.Fatalf("expected ErrDialectProductMismatch, got result=%#v err=%v", result, err)
		}
		if len(result.Statements) != 0 {
			t.Fatalf("product mismatch must not reach rule evaluation, got %#v", result.Statements)
		}
	})

	t.Run("tidb dialect plus mysql banner is a typed product mismatch", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectTiDB,
			ConfigPath:       configPath,
			Schema:           "app",
			TargetVersion:    "8.5.0",
			MetadataProvider: &t04SnapshotProvider{facts: mysql84Facts},
		})
		if !errors.Is(err, ErrDialectProductMismatch) {
			t.Fatalf("expected ErrDialectProductMismatch, got result=%#v err=%v", result, err)
		}
	})

	t.Run("same canonical version different product still mismatches", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           t04bDynamicSQL,
			Dialect:       spec.DialectTiDB,
			ConfigPath:    configPath,
			Schema:        "app",
			TargetVersion: "8.0.46",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "8.0.46", InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: 16384,
			}},
		})
		if !errors.Is(err, ErrDialectProductMismatch) {
			t.Fatalf("expected ErrDialectProductMismatch, got result=%#v err=%v", result, err)
		}
	})

	t.Run("no target still rejects a dialect/observed product conflict", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectTiDB,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: &t04SnapshotProvider{facts: mysql84Facts},
		})
		if !errors.Is(err, ErrDialectProductMismatch) {
			t.Fatalf("expected ErrDialectProductMismatch, got result=%#v err=%v", result, err)
		}
	})

	t.Run("provider error stays provider error even with target", func(t *testing.T) {
		wantErr := errors.New("connection refused")
		_, err := AuditSQL(context.Background(), Request{
			SQL:              t04bDynamicSQL,
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			TargetVersion:    "8.4.10",
			MetadataProvider: &t04SnapshotProvider{factsErr: wantErr},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected provider error, got %v", err)
		}
		if !strings.Contains(err.Error(), "load instance facts") {
			t.Fatalf("expected instance-facts wrap, got %v", err)
		}
	})

	t.Run("provider empty version yields bounded gap not fabricated fact", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL,
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
			Schema:     "app",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "", InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true, InnoDBDefaultRowFormat: "DYNAMIC",
			}},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_target_version" {
			t.Fatalf("expected missing_target_version gap, got %#v", gaps)
		}
		if result.Version != nil {
			t.Fatalf("expected nil version, got %#v", result.Version)
		}
	})

	t.Run("mysql 5.7 observed with unknown large prefix yields instance fact gap", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        t04bDynamicSQL,
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
			Schema:     "app",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "5.7.44", InnoDBLargePrefixKnown: false, InnoDBDefaultRowFormat: "DYNAMIC",
			}},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_instance_fact" {
			t.Fatalf("expected missing_instance_fact gap, got %#v", gaps)
		}
		found := false
		for _, f := range gaps[0].RequiredFacts {
			if f == "instance.innodb_large_prefix_enabled" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected instance.innodb_large_prefix_enabled fact, got %#v", gaps[0].RequiredFacts)
		}
	})

	t.Run("tidb observed with unknown max-index-length yields instance fact gap", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "CREATE TABLE t (c VARCHAR(1000) CHARACTER SET utf8mb4, KEY idx_c (c));",
			Dialect:    spec.DialectTiDB,
			ConfigPath: configPath,
			Schema:     "app",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "8.0.11-TiDB-v8.5.0",
			}},
		})
		if err != nil {
			t.Fatalf("successful read with unknown fact must not error, got %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].ReasonCode != "missing_instance_fact" {
			t.Fatalf("expected missing_instance_fact gap, got %#v", gaps)
		}
		if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.tidb_max_index_length" {
			t.Fatalf("expected instance.tidb_max_index_length fact, got %#v", gaps[0].RequiredFacts)
		}
		if result.Coverage.Status != "unverified" || result.Verdict != "review" {
			t.Fatalf("expected unverified/review, got %q/%q", result.Coverage.Status, result.Verdict)
		}
	})

	t.Run("mysql 5.7 observed large prefix off blocks 802-byte index", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "CREATE TABLE t (c VARCHAR(200) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;",
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
			Schema:     "app",
			MetadataProvider: &t04SnapshotProvider{facts: &spec.InstanceFacts{
				Version: "5.7.44", InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: false, InnoDBDefaultRowFormat: "DYNAMIC",
			}},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if len(result.Statements[0].Findings) != 1 {
			t.Fatalf("expected one 767 blocker, got %#v", result.Statements[0].Findings)
		}
		if result.Statements[0].Findings[0].Metadata["limit"] != 767 {
			t.Fatalf("expected limit 767, got %#v", result.Statements[0].Findings[0].Metadata)
		}
	})

	t.Run("mysql 5.7 observed large prefix on passes 802-byte index", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE t (c VARCHAR(200) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;",
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: &t04SnapshotProvider{facts: largePrefixOn},
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("expected clean pass, findings=%#v gaps=%#v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
}
