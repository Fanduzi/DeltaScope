// Package deltascope verifies the T06-A15 row-size field-level fact contract
// at the public SDK seam.
// input: MySQL CREATE TABLE audits under the isolated row-size profile —
// the over-limit declared input, the missing-charset input, and the
// determined-finding-plus-gap input, each audited without a provider
// output: an ordinary reject with the complete blocker identity, an
// ordinary review with the field-scoped missing_instance_fact gap, and a
// reject carrying both — all ordinary audit results, never transport-level
// errors
// pos: public SDK contract test for the frozen T06-A15 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

const t06A15RowSizeRule = "ddl.table.row_size.max_bytes.require"

// TestT06A15DeclaredOverLimitRejectSDK runs the declaration-complete
// over-limit input offline — the estimate needs no instance object and the
// blocker carries the complete metadata identity.
func TestT06A15DeclaredOverLimitRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, t06A15RowSizeRule, "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
		Dialect:    DialectMySQL,
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 1 {
		t.Fatalf("statements = %+v, want one statement with one finding", result.Statements)
	}
	finding := result.Statements[0].Findings[0]
	if finding.RuleID != t06A15RowSizeRule ||
		finding.Level != LevelBlocker ||
		finding.Message != "estimated row size exceeds 65535 bytes (65536)" ||
		finding.Metadata["charset"] != "latin1" ||
		finding.Metadata["row_format"] != "DYNAMIC" ||
		finding.Metadata["estimated"] != 65536 ||
		finding.Metadata["limit"] != 65535 {
		t.Fatalf("finding = %+v, want the over-limit blocker with the declared metadata", finding)
	}
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("evidence_gaps = %+v, want none on declared-complete input", result.Statements[0].EvidenceGaps)
	}
}

// TestT06A15MissingCharsetReviewSDK runs the missing-charset input offline —
// no finding can be determined, and the single field-scoped gap reports
// review/unverified.
func TestT06A15MissingCharsetReviewSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, t06A15RowSizeRule, "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;",
		Dialect:    DialectMySQL,
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if result.Verdict != VerdictReview || result.Coverage.Status != CoverageUnverified {
		t.Fatalf("aggregate = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("statements = %+v, want one statement with no findings", result.Statements)
	}
	gaps := result.Statements[0].EvidenceGaps
	if len(gaps) != 1 || gaps[0].RuleID != t06A15RowSizeRule ||
		gaps[0].ReasonCode != "missing_instance_fact" ||
		len(gaps[0].RequiredFacts) != 1 ||
		gaps[0].RequiredFacts[0] != "instance.default_charset" {
		t.Fatalf("evidence_gaps = %+v, want one default_charset missing_instance_fact gap", gaps)
	}
}

// TestT06A15MissingFormatOverRejectSDK runs the missing-row-format input
// that still proves the over-limit total — the determined blocker survives
// beside the field-scoped row-format gap.
func TestT06A15MissingFormatOverRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, t06A15RowSizeRule, "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1;",
		Dialect:    DialectMySQL,
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != CoverageUnverified {
		t.Fatalf("aggregate = %s/%s, want reject/unverified", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 1 {
		t.Fatalf("statements = %+v, want one statement with one finding", result.Statements)
	}
	finding := result.Statements[0].Findings[0]
	if finding.RuleID != t06A15RowSizeRule ||
		finding.Metadata["estimated"] != 65536 ||
		finding.Metadata["charset"] != "latin1" ||
		finding.Metadata["row_format"] != "" {
		t.Fatalf("finding = %+v, want the over-limit blocker with row_format=\"\"", finding)
	}
	gaps := result.Statements[0].EvidenceGaps
	if len(gaps) != 1 || gaps[0].ReasonCode != "missing_instance_fact" ||
		len(gaps[0].RequiredFacts) != 1 ||
		gaps[0].RequiredFacts[0] != "instance.innodb_default_row_format" {
		t.Fatalf("evidence_gaps = %+v, want one innodb_default_row_format gap beside the blocker", gaps)
	}
}
