// Package audit verifies the T06-A15-R1 declared-COMPRESSED row-format
// identity fix through the real parser, extractor, and AuditSQL seam.
// input: four fixed CREATE TABLE inputs declaring ROW_FORMAT=COMPRESSED plus
// opposite-format controls, executed under the isolated row-size profile
// across offline/nil/empty/defaulted instance shapes and both dialects
// output: real Options["row_format"]=="COMPRESSED" extraction, the
// declaration-over-instance resolution contract in public verdicts and
// finding metadata, a charset-only missing_instance_fact projection, exact
// provider call ledgers, and input-fact immutability
// pos: application-layer integration regression for the A15-R1
// declared-format identity fix (issue #85)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06a15r1SmallSQL = "CREATE TABLE t (c INT) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=COMPRESSED;"
	t06a15r1OverSQL  = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=COMPRESSED;"
	t06a15r1NoCSQL   = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB ROW_FORMAT=COMPRESSED;"
)

// t06a15r1BranchSQL builds the fixed eleven-column S_BRANCH input in
// declaration order; the literal the CLI evidence runs use is byte-identical.
func t06a15r1BranchSQL(format string) string {
	columns := make([]string, 0, 11)
	for i := 1; i <= 11; i++ {
		columns = append(columns, fmt.Sprintf("c%02d VARCHAR(192)", i))
	}
	return "CREATE TABLE t (" + strings.Join(columns, ", ") +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=" + format + ";"
}

// t06a15r1Audit runs one real AuditSQL with the isolated row-size profile and
// the supplied instance facts object (nil provider value means the provider
// is absent entirely — the offline shape).
func t06a15r1Audit(t *testing.T, dialect spec.Dialect, sql string, facts *spec.InstanceFacts, withProvider bool) (report.Result, *t06a15Provider) {
	t.Helper()
	var provider *t06a15Provider
	var meta MetadataProvider
	if withProvider {
		provider = &t06a15Provider{facts: facts}
		meta = provider
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden",
		ConfigPath:       t06a15Policy(t, true),
		MetadataProvider: meta,
	})
	if err != nil {
		t.Fatalf("%s %q: unexpected error %v", dialect, sql, err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("%s %q: expected one statement, got %d", dialect, sql, len(result.Statements))
	}
	return result, provider
}

func t06a15r1ExpectLedger(t *testing.T, provider *t06a15Provider, before *spec.InstanceFacts) {
	t.Helper()
	if len(provider.calls) != 2 || provider.calls[0] != "facts" || provider.calls[1] != "snapshot:golden.t" {
		t.Fatalf("unexpected provider reads: %v", provider.calls)
	}
	if provider.facts != before {
		t.Fatal("provider facts pointer was replaced")
	}
}

// The real parser and extractor carry the declared keyword through:
// Options["row_format"] must equal "COMPRESSED" before any rule runs.
func TestT06A15R1CompressedOptionExtractedVerbatim(t *testing.T) {
	t.Parallel()
	inputs := map[string]string{
		"S_SMALL":      t06a15r1SmallSQL,
		"S_BRANCH":     t06a15r1BranchSQL("COMPRESSED"),
		"S_OVER":       t06a15r1OverSQL,
		"S_NO_CHARSET": t06a15r1NoCSQL,
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for name, sql := range inputs {
			name, sql := name, sql
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				t.Parallel()
				parsed, err := Parse(context.Background(), sql, dialect)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				extracted, err := Extract(context.Background(), parsed)
				if err != nil {
					t.Fatalf("extract: %v", err)
				}
				if len(extracted) != 1 || extracted[0].DDL == nil {
					t.Fatalf("expected one DDL statement, got %+v", extracted)
				}
				if got := extracted[0].DDL.Options["row_format"]; got != "COMPRESSED" {
					t.Fatalf("Options[row_format] = %q, want COMPRESSED", got)
				}
			})
		}
	}
}

// S_SMALL is declaration-complete: every instance shape must yield the same
// complete/pass with no finding and no gap, and the provider ledger shows the
// usual two reads with facts left untouched.
func TestT06A15R1SmallComputesUnderEveryInstanceShape(t *testing.T) {
	t.Parallel()
	shapes := map[string]*spec.InstanceFacts{
		"offline":   nil, // no provider at all
		"nil":       nil, // provider returns nil facts
		"empty":     {},
		"dynamic":   {InnoDBDefaultRowFormat: "DYNAMIC"},
		"compact":   {InnoDBDefaultRowFormat: "COMPACT"},
		"redundant": {InnoDBDefaultRowFormat: "REDUNDANT"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for shape, facts := range shapes {
			shape, facts := shape, facts
			t.Run(string(dialect)+"/"+shape, func(t *testing.T) {
				t.Parallel()
				result, provider := t06a15r1Audit(t, dialect, t06a15r1SmallSQL, facts, shape != "offline")
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("verdict/coverage = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("expected 0 findings/0 gaps, got %+v", statement)
				}
				if shape != "offline" {
					t06a15r1ExpectLedger(t, provider, facts)
				}
			})
		}
	}
}

// S_BRANCH guards the wrong-format adoption: an instance default of COMPACT
// or REDUNDANT must never pull the declared COMPRESSED input into the
// 768-byte-capped payload branch (11×768=8448, which would fabricate a
// blocker neither the declaration nor the model authorizes).
func TestT06A15R1BranchNeverEntersInstanceFormatBranch(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, format := range []string{"COMPACT", "REDUNDANT"} {
			format := format
			t.Run(string(dialect)+"/default-"+format, func(t *testing.T) {
				t.Parallel()
				facts := &spec.InstanceFacts{InnoDBDefaultRowFormat: format}
				result, provider := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("COMPRESSED"), facts, true)
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("verdict/coverage = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("declared COMPRESSED must not enter the %s branch, got %+v", format, statement)
				}
				t06a15r1ExpectLedger(t, provider, facts)
			})
		}
	}
}

// S_OVER keeps the determined finding: the 65535 total bound still fires
// under a declared COMPRESSED, carries the declared format in metadata, and
// needs no instance object — complete/reject with zero gaps.
func TestT06A15R1OverLimitFindingKeepsCompressedIdentity(t *testing.T) {
	t.Parallel()
	shapes := map[string]*spec.InstanceFacts{
		"offline": nil,
		"compact": {InnoDBDefaultRowFormat: "COMPACT"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for shape, facts := range shapes {
			shape, facts := shape, facts
			t.Run(string(dialect)+"/"+shape, func(t *testing.T) {
				t.Parallel()
				result, provider := t06a15r1Audit(t, dialect, t06a15r1OverSQL, facts, shape != "offline")
				if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("verdict/coverage = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
				}
				findings := t05FindingsByRule(result, 0, t06A15RuleID)
				if len(findings) != 1 {
					t.Fatalf("expected exactly one blocker, got %+v", findings)
				}
				meta := findings[0].Metadata
				if meta["estimated"] != 65536 || meta["limit"] != 65535 ||
					meta["charset"] != "latin1" || meta["row_format"] != "COMPRESSED" {
					t.Fatalf("finding metadata = %+v", meta)
				}
				if len(statement0EvidenceGaps(result)) != 0 {
					t.Fatalf("expected no gaps, got %+v", result.Statements[0].EvidenceGaps)
				}
				if shape != "offline" {
					t06a15r1ExpectLedger(t, provider, facts)
				}
			})
		}
	}
}

func statement0EvidenceGaps(result report.Result) []rule.EvidenceGap {
	return result.Statements[0].EvidenceGaps
}

// S_NO_CHARSET still reports the genuinely missing fact — and only that
// fact: the declared COMPRESSED must keep innodb_default_row_format out of
// required_facts. Supplying a constructed DefaultCharset resolves the width
// and flips the role to a complete reject with the declared format.
func TestT06A15R1MissingCharsetReportsOnlyCharset(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, shape := range []string{"offline", "nil", "empty"} {
			shape := shape
			t.Run(string(dialect)+"/"+shape, func(t *testing.T) {
				t.Parallel()
				facts := &spec.InstanceFacts{}
				result, _ := t06a15r1Audit(t, dialect, t06a15r1NoCSQL, facts, shape != "offline")
				if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
					t.Fatalf("verdict/coverage = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
				}
				if len(result.Statements[0].Findings) != 0 {
					t.Fatalf("expected no findings, got %+v", result.Statements[0].Findings)
				}
				gaps := t06a15Gaps(result)
				if len(gaps) != 1 || gaps[0] != t06a15FactCharset {
					t.Fatalf("required_facts = %v, want [instance.default_charset]", gaps)
				}
			})
		}
		t.Run(string(dialect)+"/charset-provided", func(t *testing.T) {
			facts := &spec.InstanceFacts{DefaultCharset: "latin1"}
			result, provider := t06a15r1Audit(t, dialect, t06a15r1NoCSQL, facts, true)
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("verdict/coverage = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			findings := t05FindingsByRule(result, 0, t06A15RuleID)
			if len(findings) != 1 || findings[0].Metadata["row_format"] != "COMPRESSED" {
				t.Fatalf("expected the over-limit finding carrying COMPRESSED, got %+v", findings)
			}
			if len(statement0EvidenceGaps(result)) != 0 {
				t.Fatalf("expected no gaps, got %+v", result.Statements[0].EvidenceGaps)
			}
			t06a15r1ExpectLedger(t, provider, facts)
		})
	}
}

// Opposite-format controls: the fix must not launder every declaration out
// of the format branches or weaken the DEFAULT/instance-default path.
func TestT06A15R1OppositeFormatControls(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect)+"/compact-declared", func(t *testing.T) {
			t.Parallel()
			result, _ := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("COMPACT"), nil, false)
			assertSingleFormatFinding(t, result, "COMPACT", 8126, 8448)
		})
		t.Run(string(dialect)+"/redundant-declared", func(t *testing.T) {
			t.Parallel()
			result, _ := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("REDUNDANT"), nil, false)
			assertSingleFormatFinding(t, result, "REDUNDANT", 8000, 8448)
		})
		t.Run(string(dialect)+"/dynamic-declared", func(t *testing.T) {
			t.Parallel()
			result, _ := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("DYNAMIC"), nil, false)
			if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 ||
				len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("DYNAMIC control: expected silent pass, got %s %+v", result.Verdict, result.Statements[0])
			}
		})
		t.Run(string(dialect)+"/default-declared-with-compact-instance", func(t *testing.T) {
			t.Parallel()
			result, _ := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("DEFAULT"),
				&spec.InstanceFacts{InnoDBDefaultRowFormat: "COMPACT"}, true)
			assertSingleFormatFinding(t, result, "COMPACT", 8126, 8448)
		})
		t.Run(string(dialect)+"/default-declared-no-instance", func(t *testing.T) {
			t.Parallel()
			result, _ := t06a15r1Audit(t, dialect, t06a15r1BranchSQL("DEFAULT"), nil, false)
			if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("verdict/coverage = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
			}
			gaps := t06a15Gaps(result)
			if len(gaps) != 1 || gaps[0] != t06a15FactRowFormat {
				t.Fatalf("required_facts = %v, want [instance.innodb_default_row_format]", gaps)
			}
		})
		t.Run(string(dialect)+"/omitted-no-instance", func(t *testing.T) {
			t.Parallel()
			// No row_format option at all — the same unresolved path.
			sql := "CREATE TABLE t (c VARCHAR(65532)) ENGINE=InnoDB DEFAULT CHARSET=latin1;"
			result, _ := t06a15r1Audit(t, dialect, sql, nil, false)
			if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("verdict/coverage = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
			}
			gaps := t06a15Gaps(result)
			if len(gaps) != 1 || gaps[0] != t06a15FactRowFormat {
				t.Fatalf("required_facts = %v, want [instance.innodb_default_row_format]", gaps)
			}
		})
	}
}

func assertSingleFormatFinding(t *testing.T, result report.Result, format string, limit, estimated int) {
	t.Helper()
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("verdict/coverage = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	findings := t05FindingsByRule(result, 0, t06A15RuleID)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one %s finding, got %+v", format, findings)
	}
	meta := findings[0].Metadata
	if meta["row_format"] != format || meta["limit"] != limit || meta["estimated"] != estimated {
		t.Fatalf("finding metadata = %+v, want row_format=%s limit=%d estimated=%d", meta, format, limit, estimated)
	}
	if len(statement0EvidenceGaps(result)) != 0 {
		t.Fatalf("expected no gaps, got %+v", result.Statements[0].EvidenceGaps)
	}
}
