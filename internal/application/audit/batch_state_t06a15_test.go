// Package audit verifies the T06-A15 row-size field-level fact contract at
// the application seam.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A15RuleID         = "ddl.table.row_size.max_bytes.require"
	t06a15FactCharset    = "instance.default_charset"
	t06a15FactRowFormat  = "instance.innodb_default_row_format"
	t06a15BelowSQL       = "CREATE TABLE t (c VARCHAR(65532)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;"
	t06a15EqualSQL       = "CREATE TABLE t (c VARCHAR(65533)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;"
	t06a15AboveSQL       = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;"
	t06a15OtherEngineSQL = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=MyISAM DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;"
	t06a15OverrideSQL    = "CREATE TABLE t (c VARCHAR(16384) CHARACTER SET utf8mb4) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;"
	t06a15NoCharsetSQL   = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"
	t06a15NoFmtSafeSQL   = "CREATE TABLE t (c VARCHAR(65532)) ENGINE=InnoDB DEFAULT CHARSET=latin1;"
	t06a15NoFmtOverSQL   = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1;"
	t06a15NoBothSQL      = "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB;"
	t06a15ColOnlySQL     = "CREATE TABLE t (c VARCHAR(16384) CHARACTER SET utf8mb4) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"
)

// t06a15Provider returns the configured instance facts object verbatim —
// nil means nil — and confirms the table absent, exactly like the frozen
// baseline probe. calls records every consultation for the ledger.
type t06a15Provider struct {
	facts *spec.InstanceFacts
	err   error
	calls []string
}

func (p *t06a15Provider) LoadInstanceFacts(_ context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	p.calls = append(p.calls, "facts")
	if p.err != nil {
		return nil, p.err
	}
	return p.facts, nil
}

func (p *t06a15Provider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, "snapshot:"+schema+"."+table)
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t06a15Policy(t *testing.T, required bool) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t06A15RuleID: "      required: " + map[bool]string{true: "true", false: "false"}[required] + "\n",
	})
}

func t06a15Audit(t *testing.T, dialect spec.Dialect, sql string, provider *t06a15Provider) report.Result {
	t.Helper()
	var meta MetadataProvider
	if provider != nil {
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
	return result
}

func t06a15Gaps(result report.Result) []string {
	gaps := t05GapsByRule(result, 0, t06A15RuleID)
	if len(gaps) == 0 {
		return nil
	}
	if len(gaps) != 1 {
		return append([]string{"TOO_MANY_GAPS"}, gaps[0].RequiredFacts...)
	}
	return gaps[0].RequiredFacts
}

// The 12-role offline matrix (per dialect): declaration-complete roles
// evaluate without any instance object; missing-fact roles surface the
// missing_instance_fact gap instead of a silent skip.
func TestT06A15OfflineRoleMatrix(t *testing.T) {
	t.Parallel()
	type expectation struct {
		verdict    report.Verdict
		coverage   report.CoverageStatus
		findings   int
		estimated  int
		charset    string
		rowFormat  string
		facts      []string
		applicable int
	}
	roles := []struct {
		name   string
		sql    string
		expect expectation
	}{
		{"below", t06a15BelowSQL, expectation{report.VerdictPass, report.CoverageComplete, 0, 0, "", "", nil, 1}},
		{"equal", t06a15EqualSQL, expectation{report.VerdictPass, report.CoverageComplete, 0, 0, "", "", nil, 1}},
		{"above", t06a15AboveSQL, expectation{report.VerdictReject, report.CoverageComplete, 1, 65536, "latin1", "DYNAMIC", nil, 1}},
		{"column-override", t06a15OverrideSQL, expectation{report.VerdictReject, report.CoverageComplete, 1, 65538, "latin1", "DYNAMIC", nil, 1}},
		{"non-innodb-skip", t06a15OtherEngineSQL, expectation{report.VerdictPass, report.CoverageComplete, 0, 0, "", "", nil, 1}},
		{"missing-charset", t06a15NoCharsetSQL, expectation{report.VerdictReview, report.CoverageUnverified, 0, 0, "", "", []string{t06a15FactCharset}, 1}},
		{"missing-format-safe", t06a15NoFmtSafeSQL, expectation{report.VerdictReview, report.CoverageUnverified, 0, 0, "", "", []string{t06a15FactRowFormat}, 1}},
		{"missing-format-over", t06a15NoFmtOverSQL, expectation{report.VerdictReject, report.CoverageUnverified, 1, 65536, "latin1", "", []string{t06a15FactRowFormat}, 1}},
		{"missing-both", t06a15NoBothSQL, expectation{report.VerdictReview, report.CoverageUnverified, 0, 0, "", "", []string{t06a15FactCharset, t06a15FactRowFormat}, 1}},
		{"column-only-charset", t06a15ColOnlySQL, expectation{report.VerdictReject, report.CoverageComplete, 1, 65538, "", "DYNAMIC", nil, 1}},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, role := range roles {
			role := role
			t.Run(string(dialect)+"/"+role.name, func(t *testing.T) {
				t.Parallel()
				result := t06a15Audit(t, dialect, role.sql, nil)
				statement := result.Statements[0]
				if result.Verdict != role.expect.verdict {
					t.Fatalf("verdict = %s, want %s", result.Verdict, role.expect.verdict)
				}
				if result.Coverage.Status != role.expect.coverage || statement.Coverage.Status != role.expect.coverage {
					t.Fatalf("coverage = %s/%s, want %s", result.Coverage.Status, statement.Coverage.Status, role.expect.coverage)
				}
				rowFindings := t05FindingsByRule(result, 0, t06A15RuleID)
				if len(rowFindings) != role.expect.findings {
					t.Fatalf("findings = %+v, want %d", rowFindings, role.expect.findings)
				}
				if role.expect.findings == 1 {
					meta := rowFindings[0].Metadata
					if meta["estimated"] != role.expect.estimated ||
						meta["charset"] != role.expect.charset ||
						meta["row_format"] != role.expect.rowFormat ||
						meta["limit"] != 65535 {
						t.Fatalf("finding metadata = %+v", meta)
					}
					if rowFindings[0].Location == nil || rowFindings[0].Location.Line != 1 {
						t.Fatalf("location = %+v, want 1:1", rowFindings[0].Location)
					}
				}
				got := t06a15Gaps(result)
				if len(got) != len(role.expect.facts) {
					t.Fatalf("required_facts = %v, want %v", got, role.expect.facts)
				}
				for i, fact := range role.expect.facts {
					if got[i] != fact {
						t.Fatalf("required_facts = %v, want %v", got, role.expect.facts)
					}
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != role.expect.applicable {
					t.Fatalf("rule_summary = %+v, want loaded=1 applicable=%d", result.RuleSummary, role.expect.applicable)
				}
			})
		}
	}
}

// required=false and disabled keep their distinct loaded/applicable
// projections and never emit this rule's findings or gaps.
func TestT06A15NotRequiredAndDisabled(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect)+"/not-required", func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL: t06a15AboveSQL, Dialect: dialect, Schema: "golden",
				ConfigPath: t06a15Policy(t, false),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("verdict/coverage = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("expected no findings/gaps, got %+v", result.Statements[0])
			}
			if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 0 {
				t.Fatalf("rule_summary = %+v, want loaded=1 applicable=0", result.RuleSummary)
			}
		})
		t.Run(string(dialect)+"/disabled", func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL: t06a15AboveSQL, Dialect: dialect, Schema: "golden",
				ConfigPath: t05PolicyPath(t, map[string]string{}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("verdict/coverage = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if result.RuleSummary != nil {
				t.Fatalf("expected absent rule_summary, got %+v", result.RuleSummary)
			}
		})
	}
}

// The estimate is pointer-presence independent: nil, empty, and full instance
// objects produce identical rule results when the declarations are complete.
func TestT06A15InstancePointerShapesAgree(t *testing.T) {
	t.Parallel()
	shapes := map[string]*spec.InstanceFacts{
		"nil":   nil,
		"empty": {},
		"full":  {DefaultCharset: "latin1", InnoDBDefaultRowFormat: "DYNAMIC"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for name, facts := range shapes {
			name, facts := name, facts
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				t.Parallel()
				result := t06a15Audit(t, dialect, t06a15AboveSQL, &t06a15Provider{facts: facts})
				findings := t05FindingsByRule(result, 0, t06A15RuleID)
				if result.Verdict != report.VerdictReject || len(findings) != 1 ||
					findings[0].Metadata["estimated"] != 65536 {
					t.Fatalf("expected identical reject/65536, got %s %+v", result.Verdict, findings)
				}
				if gaps := t06a15Gaps(result); len(gaps) != 0 {
					t.Fatalf("expected no gaps, got %v", gaps)
				}
			})
		}
	}
}

// Per-field sufficiency: each missing instance field reports exactly itself —
// and only when the statement actually needs it — under nil and empty objects
// alike.
func TestT06A15FieldLevelInstanceMatrix(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, shape := range []string{"nil", "empty"} {
			shape := shape
			t.Run(string(dialect)+"/"+shape, func(t *testing.T) {
				t.Parallel()
				providerFor := func(facts *spec.InstanceFacts) *t06a15Provider {
					if shape == "nil" {
						return &t06a15Provider{facts: nil}
					}
					return &t06a15Provider{facts: facts}
				}
				columnsSQL := t06a15NoBothSQL
				// charset-only instance: charset need satisfied → row_format gap only.
				result := t06a15Audit(t, dialect, columnsSQL,
					providerFor(&spec.InstanceFacts{DefaultCharset: "latin1"}))
				if shape == "nil" {
					// nil object supplies nothing: both facts still missing.
					if got := t06a15Gaps(result); len(got) != 2 {
						t.Fatalf("nil facts = %v, want both missing", got)
					}
				} else if got := t06a15Gaps(result); len(got) != 1 || got[0] != t06a15FactRowFormat {
					t.Fatalf("charset-only facts = %v, want [row_format]", got)
				}

				// row_format-only instance: charset still missing.
				result = t06a15Audit(t, dialect, columnsSQL,
					providerFor(&spec.InstanceFacts{InnoDBDefaultRowFormat: "DYNAMIC"}))
				if shape == "nil" {
					if got := t06a15Gaps(result); len(got) != 2 {
						t.Fatalf("nil facts = %v, want both missing", got)
					}
				} else if got := t06a15Gaps(result); len(got) != 1 || got[0] != t06a15FactCharset {
					t.Fatalf("rowformat-only facts = %v, want [default_charset]", got)
				}

				// both fields present: fully resolved, finding computed.
				result = t06a15Audit(t, dialect, columnsSQL,
					providerFor(&spec.InstanceFacts{DefaultCharset: "latin1", InnoDBDefaultRowFormat: "DYNAMIC"}))
				if shape == "nil" {
					if got := t06a15Gaps(result); len(got) != 2 {
						t.Fatalf("nil facts = %v, want both missing", got)
					}
				} else {
					if result.Verdict != report.VerdictReject {
						t.Fatalf("both facts = %s, want reject", result.Verdict)
					}
					if got := t06a15Gaps(result); len(got) != 0 {
						t.Fatalf("both facts = gaps %v, want none", got)
					}
				}
			})
		}
	}
}

// Provider failures and context cancellation keep the error channel — they
// never become missing_instance_fact gaps.
func TestT06A15ProviderErrorAndCancelPropagate(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("t06a15 provider sentinel")
	_, err := AuditSQL(context.Background(), Request{
		SQL: t06a15AboveSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath:       t06a15Policy(t, true),
		MetadataProvider: &t06a15Provider{err: sentinel},
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("provider error must propagate with identity, got %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = AuditSQL(cancelled, Request{
		SQL: t06a15AboveSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath:       t06a15Policy(t, true),
		MetadataProvider: &t06a15Provider{facts: &spec.InstanceFacts{}},
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("context cancellation must propagate, got %v", err)
	}
}
