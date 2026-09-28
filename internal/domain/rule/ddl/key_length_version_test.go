// Package ddl verifies the T04-B/#83 version-dependent key-length rule contract.
// input: create-table statements with explicit version identities and instance facts across MySQL 5.7/8.0/8.4 and TiDB 8.5
// output: bounded evidence gaps for missing/out-of-range/unknown facts and proven-bound findings, never silent defaults
// pos: version-fact resolution matrix for ddl.index.key_length.max_bytes.require
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newT04BKeyLengthRule(t *testing.T, required bool) rule.StatementRule {
	t.Helper()
	ruleUnderTest, err := newIndexKeyLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": required},
	})
	if err != nil {
		t.Fatalf("new key-length rule: %v", err)
	}
	return ruleUnderTest
}

// keyLengthStatement builds a CREATE TABLE whose single secondary index over a
// utf8mb4 varchar column is the version-dependent fact under test.
func keyLengthStatement(dialect spec.Dialect, varcharLength int, options map[string]string, version *spec.VersionIdentity, instance *spec.InstanceFacts) spec.Statement {
	if options == nil {
		options = map[string]string{"engine": "InnoDB", "charset": "utf8mb4"}
	}
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Options:   options,
			Columns: []spec.Column{
				{Name: "c", Type: "varchar", Length: varcharLength, Charset: "utf8mb4"},
			},
			Indexes: []spec.Index{
				{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
			},
		},
		Metadata: &spec.Metadata{Version: version, Instance: instance},
	}
}

// twoIndexStatement builds a CREATE TABLE where one index is ambiguous under
// an unknown page size (200 chars ≈ 800 bytes > 768 but ≤ 3072) and another is
// a proven violation under every candidate bound (800 chars ≈ 3200 bytes).
func twoIndexStatement(version *spec.VersionIdentity, instance *spec.InstanceFacts) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Options:   map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": "DYNAMIC"},
			Columns: []spec.Column{
				{Name: "a", Type: "varchar", Length: 200, Charset: "utf8mb4"},
				{Name: "b", Type: "varchar", Length: 800, Charset: "utf8mb4"},
			},
			Indexes: []spec.Index{
				{Name: "idx_a", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
				{Name: "idx_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}},
			},
		},
		Metadata: &spec.Metadata{Version: version, Instance: instance},
	}
}

func targetVersion(product, version string, major, minor, patch int, validated bool) *spec.VersionIdentity {
	return &spec.VersionIdentity{
		Product:        product,
		Version:        version,
		Major:          major,
		Minor:          minor,
		Patch:          patch,
		Source:         spec.VersionSourceTarget,
		ValidatedRange: validated,
	}
}

func pageSizeFacts(bytes int) *spec.InstanceFacts {
	return &spec.InstanceFacts{InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: bytes}
}

func tidbMaxIndexLengthFacts(bytes int) *spec.InstanceFacts {
	return &spec.InstanceFacts{TiDBMaxIndexLengthKnown: true, TiDBMaxIndexLengthBytes: bytes}
}

func evidenceGapCodes(t *testing.T, r rule.StatementRule, statement spec.Statement) (string, []string) {
	t.Helper()
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		t.Fatalf("key-length rule must implement EvidenceReporter")
	}
	gaps := reporter.EvidenceGaps(statement)
	if len(gaps) == 0 {
		return "", nil
	}
	if len(gaps) != 1 {
		t.Fatalf("expected at most one gap, got %#v", gaps)
	}
	return gaps[0].ReasonCode, gaps[0].RequiredFacts
}

// TestIndexKeyLengthVersionEvidenceMatrix locks the version/fact matrix: which
// evidence resolves the bound, which absence degrades to a bounded gap, and
// which bounds actually apply per validated anchor.
func TestIndexKeyLengthVersionEvidenceMatrix(t *testing.T) {
	t.Parallel()
	ruleUnderTest := newT04BKeyLengthRule(t, true)

	dynamicOpts := map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": "DYNAMIC"}

	cases := []struct {
		name         string
		statement    spec.Statement
		wantReason   string
		wantFacts    []string
		wantFindings int
		wantLimit    int
	}{
		{
			name:       "mysql missing version yields version gap not finding",
			statement:  keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts, nil, nil),
			wantReason: gapReasonMissingTargetVersion,
			wantFacts:  []string{factTargetVersion},
		},
		{
			name:         "mysql missing version still reports violation proven under every candidate bound",
			statement:    keyLengthStatement(spec.DialectMySQL, 800, dynamicOpts, nil, nil),
			wantReason:   gapReasonMissingTargetVersion,
			wantFacts:    []string{factTargetVersion},
			wantFindings: 1,
			wantLimit:    3072,
		},
		{
			name: "mysql 8.4 dynamic small index with unknown page size is an instance-fact gap",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), nil),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBPageSize},
		},
		{
			name: "mysql 8.4 dynamic oversized index exceeds every candidate page bound",
			statement: keyLengthStatement(spec.DialectMySQL, 800, dynamicOpts,
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), nil),
			wantFindings: 1,
			wantLimit:    3072,
		},
		{
			name: "mysql 8.4 dynamic small index with known 4KB page is a 768 blocker",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), pageSizeFacts(4096)),
			wantFindings: 1,
			wantLimit:    768,
		},
		{
			name: "mysql 8.4 dynamic small index with known 8KB page fits 1536",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), pageSizeFacts(8192)),
		},
		{
			name: "mysql 8.4 dynamic small index with known 16KB page fits 3072",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), pageSizeFacts(16384)),
		},
		{
			name: "mysql 8.0 dynamic oversized index with known 16KB page is a 3072 blocker",
			statement: keyLengthStatement(spec.DialectMySQL, 800, dynamicOpts,
				targetVersion("mysql", "8.0.46", 8, 0, 46, true), pageSizeFacts(16384)),
			wantFindings: 1,
			wantLimit:    3072,
		},
		{
			name: "mysql 5.7 dynamic without large_prefix or page facts reports both",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true), &spec.InstanceFacts{}),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBLargePrefixEnabled, factInstanceInnoDBPageSize},
		},
		{
			name: "mysql 5.7 dynamic large_prefix on without page fact is a page-size gap",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true),
				&spec.InstanceFacts{InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true}),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBPageSize},
		},
		{
			name: "mysql 5.7 dynamic large_prefix on 4KB page caps at 768",
			statement: keyLengthStatement(spec.DialectMySQL, 255, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true),
				&spec.InstanceFacts{InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true, InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: 4096}),
			wantFindings: 1,
			wantLimit:    768,
		},
		{
			name: "mysql 5.7 dynamic large_prefix on 16KB page oversized is a 3072 blocker",
			statement: keyLengthStatement(spec.DialectMySQL, 800, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true),
				&spec.InstanceFacts{InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: true, InnoDBPageSizeKnown: true, InnoDBPageSizeBytes: 16384}),
			wantFindings: 1,
			wantLimit:    3072,
		},
		{
			name: "mysql 5.7 dynamic large_prefix off caps at 767",
			statement: keyLengthStatement(spec.DialectMySQL, 200, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true),
				&spec.InstanceFacts{InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: false}),
			wantFindings: 1,
			wantLimit:    767,
		},
		{
			name: "mysql 5.7 dynamic large_prefix off small index passes",
			statement: keyLengthStatement(spec.DialectMySQL, 100, dynamicOpts,
				targetVersion("mysql", "5.7.44", 5, 7, 44, true),
				&spec.InstanceFacts{InnoDBLargePrefixKnown: true, InnoDBLargePrefixEnabled: false}),
		},
		{
			name: "mysql compact resolves 767 without any version fact",
			statement: keyLengthStatement(spec.DialectMySQL, 200,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": "COMPACT"}, nil, nil),
			wantFindings: 1,
			wantLimit:    767,
		},
		{
			name: "mysql out-of-range suppresses finding and reports range gap",
			statement: keyLengthStatement(spec.DialectMySQL, 800, dynamicOpts,
				targetVersion("mysql", "8.3.0", 8, 3, 0, false), nil),
			wantReason: gapReasonTargetVersionOutOfRange,
			wantFacts:  []string{factTargetVersionValidatedRange},
		},
		{
			name:       "tidb missing version yields version gap",
			statement:  keyLengthStatement(spec.DialectTiDB, 255, dynamicOpts, nil, nil),
			wantReason: gapReasonMissingTargetVersion,
			wantFacts:  []string{factTargetVersion},
		},
		{
			name: "tidb 8.5 unknown max-index-length leaves mid-range index unverified",
			statement: keyLengthStatement(spec.DialectTiDB, 800, dynamicOpts,
				targetVersion("tidb", "8.5.0", 8, 5, 0, true), nil),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceTiDBMaxIndexLength},
		},
		{
			name: "tidb 8.5 unknown max-index-length passes totals under every candidate",
			statement: keyLengthStatement(spec.DialectTiDB, 700, dynamicOpts,
				targetVersion("tidb", "8.5.0", 8, 5, 0, true), nil),
		},
		{
			name: "tidb 8.5 unknown max-index-length still proves totals over 12288",
			statement: keyLengthStatement(spec.DialectTiDB, 3100, dynamicOpts,
				targetVersion("tidb", "8.5.0", 8, 5, 0, true), nil),
			wantFindings: 1,
			wantLimit:    12288,
		},
		{
			name: "tidb 8.5 configured 3072 blocks the 3200-byte index",
			statement: keyLengthStatement(spec.DialectTiDB, 800, dynamicOpts,
				targetVersion("tidb", "8.5.0", 8, 5, 0, true), tidbMaxIndexLengthFacts(3072)),
			wantFindings: 1,
			wantLimit:    3072,
		},
		{
			name: "tidb 8.5 configured 12288 passes the 4000-byte index",
			statement: keyLengthStatement(spec.DialectTiDB, 1000, dynamicOpts,
				targetVersion("tidb", "8.5.0", 8, 5, 0, true), tidbMaxIndexLengthFacts(12288)),
		},
		{
			name: "tidb out-of-range suppresses finding",
			statement: keyLengthStatement(spec.DialectTiDB, 800, dynamicOpts,
				targetVersion("tidb", "8.4.0", 8, 4, 0, false), nil),
			wantReason: gapReasonTargetVersionOutOfRange,
			wantFacts:  []string{factTargetVersionValidatedRange},
		},
		{
			name: "mysql 8.4 without row-format evidence yields instance fact gap",
			statement: keyLengthStatement(spec.DialectMySQL, 255,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4"},
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), pageSizeFacts(16384)),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBDefaultRowFormat},
		},
		{
			name: "mysql 8.4 without row-format or page facts reports both",
			statement: keyLengthStatement(spec.DialectMySQL, 255,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4"},
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), &spec.InstanceFacts{}),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBDefaultRowFormat, factInstanceInnoDBPageSize},
		},
		{
			name: "mysql 5.7 unknown row format and large prefix reports all missing facts",
			statement: keyLengthStatement(spec.DialectMySQL, 255,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4"},
				targetVersion("mysql", "5.7.44", 5, 7, 44, true), &spec.InstanceFacts{}),
			wantReason: gapReasonMissingInstanceFact,
			wantFacts:  []string{factInstanceInnoDBDefaultRowFormat, factInstanceInnoDBLargePrefixEnabled, factInstanceInnoDBPageSize},
		},
		{
			name: "mysql 8.4 missing facts never suppress a proven blocker",
			statement: twoIndexStatement(
				targetVersion("mysql", "8.4.10", 8, 4, 10, true), &spec.InstanceFacts{}),
			wantReason:   gapReasonMissingInstanceFact,
			wantFacts:    []string{factInstanceInnoDBPageSize},
			wantFindings: 1,
			wantLimit:    3072,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, facts := evidenceGapCodes(t, ruleUnderTest, tc.statement)
			if reason != tc.wantReason {
				t.Fatalf("gap reason = %q, want %q", reason, tc.wantReason)
			}
			gotFacts := append([]string(nil), facts...)
			wantFacts := append([]string(nil), tc.wantFacts...)
			sort.Strings(gotFacts)
			sort.Strings(wantFacts)
			if strings.Join(gotFacts, ",") != strings.Join(wantFacts, ",") {
				t.Fatalf("gap facts = %#v, want %#v", facts, tc.wantFacts)
			}
			findings, err := ruleUnderTest.Evaluate(context.Background(), tc.statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != tc.wantFindings {
				t.Fatalf("findings = %#v, want %d", findings, tc.wantFindings)
			}
			if tc.wantFindings > 0 {
				if got := findings[0].Metadata["limit"]; got != tc.wantLimit {
					t.Fatalf("finding limit = %v, want %d", got, tc.wantLimit)
				}
			}
		})
	}
}

// TestIndexKeyLengthNoGapWhenNotApplicable proves the gap path is gated by the
// same applicability contract as the findings path.
func TestIndexKeyLengthNoGapWhenNotApplicable(t *testing.T) {
	t.Parallel()

	t.Run("required false stays silent", func(t *testing.T) {
		ruleUnderTest := newT04BKeyLengthRule(t, false)
		statement := keyLengthStatement(spec.DialectMySQL, 255, nil, nil, nil)
		if ruleUnderTest.AppliesTo(statement) {
			t.Fatalf("required=false must not apply")
		}
		if reason, _ := evidenceGapCodes(t, ruleUnderTest, statement); reason != "" {
			t.Fatalf("expected no gap, got %q", reason)
		}
	})

	t.Run("non index create stays silent", func(t *testing.T) {
		ruleUnderTest := newT04BKeyLengthRule(t, true)
		statement := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "t"},
			},
		}
		if reason, _ := evidenceGapCodes(t, ruleUnderTest, statement); reason != "" {
			t.Fatalf("expected no gap for non-index statement, got %q", reason)
		}
	})
}
