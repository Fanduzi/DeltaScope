// Package audit verifies the T04-A/#83 metadata evidence-gap contract.
// input: audit requests over the isolated modify-column compatibility policy with missing, complete, or partial metadata
// output: coverage unverified plus bounded evidence gaps, never silent pass and never fabricated findings
// pos: application audit evidence-gap regression tests for issue #83 (T04-A slice)
// note: if this file changes, update this header and module README.md.
package audit

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
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// writeIsolatedPolicy renders a temporary policy that disables every cataloged
// rule except the listed overrides, proving the evidence-gap path activates only
// for an explicitly enabled rule.
func writeIsolatedPolicy(t *testing.T, overrides map[string]policy.RulePolicy) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "isolated-policy.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if override, ok := overrides[id]; ok {
			fmt.Fprintf(&builder, "  %s:\n    enabled: %t\n", strconv.Quote(id), override.Enabled)
			if override.Level != "" {
				fmt.Fprintf(&builder, "    level: %s\n", override.Level)
			}
			if len(override.Params) > 0 {
				builder.WriteString("    params:\n")
				keys := make([]string, 0, len(override.Params))
				for key := range override.Params {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					fmt.Fprintf(&builder, "      %s: %v\n", key, override.Params[key])
				}
			}
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t04TargetRule = "ddl.alter.modify_column.compatibility.require"

func writeT04TargetPolicy(t *testing.T) string {
	t.Helper()
	return writeIsolatedPolicy(t, map[string]policy.RulePolicy{
		t04TargetRule: {
			Enabled: true,
			Level:   "blocker",
			Params:  map[string]any{"required": true, "requires_metadata": true},
		},
	})
}

// TestAuditSQLT04MissingMetadataMarksUnverified is the frozen T04-A oracle for
// matrix case A: with the target rule enabled and requires_metadata, an
// applicable MODIFY COLUMN with no usable source-column facts must report
// coverage=unverified, verdict=review, exactly one bounded evidence gap, and
// zero findings — never a silent pass.
func TestAuditSQLT04MissingMetadataMarksUnverified(t *testing.T) {
	t.Parallel()
	configPath := writeT04TargetPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected successful audit without provider error, got %v", err)
	}

	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	statement := result.Statements[0]
	if statement.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("expected statement coverage unverified, got %q", statement.Coverage.Status)
	}
	if result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("expected aggregate coverage unverified, got %q", result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected review verdict floor for evidence gap, got %q", result.Verdict)
	}
	if len(statement.Findings) != 0 || result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("expected zero findings for unverified path, got %#v summary=%#v", statement.Findings, result.Summary)
	}
	if len(statement.EvidenceGaps) != 1 {
		t.Fatalf("expected exactly one evidence gap, got %#v", statement.EvidenceGaps)
	}
	gap := statement.EvidenceGaps[0]
	if gap.RuleID != t04TargetRule {
		t.Fatalf("expected gap rule %q, got %q", t04TargetRule, gap.RuleID)
	}
	if gap.ReasonCode != "missing_source_column" {
		t.Fatalf("expected reason missing_source_column, got %q", gap.ReasonCode)
	}
	if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "source_column.definition" {
		t.Fatalf("expected required facts [source_column.definition], got %#v", gap.RequiredFacts)
	}
	if len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("expected no unsupported/diagnostics, got %#v %#v", result.Unsupported, result.Diagnostics)
	}
}

// TestAuditSQLT04MissingMetadataGapShape pins the missing-fact boundary: absent
// providers, absent tables, absent columns, and partial snapshots each map to a
// fixed reason code and bounded required-fact identifiers.
func TestAuditSQLT04MissingMetadataGapShape(t *testing.T) {
	t.Parallel()
	configPath := writeT04TargetPolicy(t)

	cases := []struct {
		name       string
		provider   MetadataProvider
		schema     string
		wantReason string
		wantFacts  []string
	}{
		{
			name:       "provider returns nil snapshot",
			provider:   &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{}},
			schema:     "app",
			wantReason: "missing_source_column",
			wantFacts:  []string{"source_column.definition"},
		},
		{
			name: "snapshot exists but column absent",
			provider: &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {Exists: true, Table: &spec.Table{Name: "t"}},
			}},
			schema:     "app",
			wantReason: "missing_source_column",
			wantFacts:  []string{"source_column.definition"},
		},
		{
			name: "snapshot exists but column type missing",
			provider: &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {Exists: true, Table: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "c"}}},
			}},
			schema:     "app",
			wantReason: "incomplete_source_column",
			wantFacts:  []string{"source_column.length", "source_column.type"},
		},
		{
			name: "string source column length missing",
			provider: &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {Exists: true, Table: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "c", Type: "varchar"}}},
			}},
			schema:     "app",
			wantReason: "incomplete_source_column",
			wantFacts:  []string{"source_column.length"},
		},
		{
			name: "non-string source column without length stays evaluable",
			provider: &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {Exists: true, Table: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "c", Type: "int"}}},
			}},
			schema: "app",
		},
		{
			name: "confirmed absent table cannot verify source column",
			provider: &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {Exists: false},
			}},
			schema:     "app",
			wantReason: "missing_source_column",
			wantFacts:  []string{"source_column.definition"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:              "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
				Dialect:          spec.DialectMySQL,
				ConfigPath:       configPath,
				Schema:           tc.schema,
				MetadataProvider: tc.provider,
			})
			if err != nil {
				t.Fatalf("expected successful audit, got %v", err)
			}
			statement := result.Statements[0]
			if tc.wantReason == "" {
				if len(statement.EvidenceGaps) != 0 {
					t.Fatalf("expected no evidence gap, got %#v", statement.EvidenceGaps)
				}
				if statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("expected coverage complete, got %q", statement.Coverage.Status)
				}
				return
			}
			if len(statement.EvidenceGaps) != 1 {
				t.Fatalf("expected one evidence gap, got %#v", statement.EvidenceGaps)
			}
			gap := statement.EvidenceGaps[0]
			if gap.RuleID != t04TargetRule || gap.ReasonCode != tc.wantReason {
				t.Fatalf("gap = %#v, want rule=%q reason=%q", gap, t04TargetRule, tc.wantReason)
			}
			gotFacts := append([]string(nil), gap.RequiredFacts...)
			wantFacts := append([]string(nil), tc.wantFacts...)
			sort.Strings(gotFacts)
			sort.Strings(wantFacts)
			if strings.Join(gotFacts, ",") != strings.Join(wantFacts, ",") {
				t.Fatalf("required facts = %#v, want %#v", gap.RequiredFacts, tc.wantFacts)
			}
			if statement.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("expected unverified, got %q", statement.Coverage.Status)
			}
		})
	}
}

// TestAuditSQLT04CompleteMetadataCoversBothDirections covers matrix cases B and
// C: real source facts decide pass versus a single narrowing blocker.
func TestAuditSQLT04CompleteMetadataCoversBothDirections(t *testing.T) {
	t.Parallel()
	configPath := writeT04TargetPolicy(t)

	cases := []struct {
		name         string
		sourceLength int
		wantVerdict  report.Verdict
		wantBlockers int
	}{
		{name: "widen varchar10 to 20", sourceLength: 10, wantVerdict: report.VerdictPass, wantBlockers: 0},
		{name: "shrink varchar200 to 20", sourceLength: 200, wantVerdict: report.VerdictReject, wantBlockers: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
				"app.t": {
					Exists:  true,
					Table:   &spec.Table{Name: "t"},
					Columns: []spec.Column{{Name: "c", Type: "varchar", Length: tc.sourceLength}},
				},
			}}
			result, err := AuditSQL(context.Background(), Request{
				SQL:              "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
				Dialect:          spec.DialectMySQL,
				ConfigPath:       configPath,
				Schema:           "app",
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("expected successful audit, got %v", err)
			}
			statement := result.Statements[0]
			if len(statement.EvidenceGaps) != 0 {
				t.Fatalf("expected no evidence gaps with complete metadata, got %#v", statement.EvidenceGaps)
			}
			if statement.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("expected complete coverage, got statement=%q aggregate=%q", statement.Coverage.Status, result.Coverage.Status)
			}
			if result.Verdict != tc.wantVerdict {
				t.Fatalf("expected verdict %q, got %q", tc.wantVerdict, result.Verdict)
			}
			if result.Summary.Blockers != tc.wantBlockers {
				t.Fatalf("expected %d blockers, got %#v", tc.wantBlockers, result.Summary)
			}
			if tc.wantBlockers == 0 {
				if len(statement.Findings) != 0 {
					t.Fatalf("expected no findings for widening change, got %#v", statement.Findings)
				}
				return
			}
			if len(statement.Findings) != 1 {
				t.Fatalf("expected exactly one blocker finding, got %#v", statement.Findings)
			}
			finding := statement.Findings[0]
			if finding.RuleID != t04TargetRule || finding.Level != rule.LevelBlocker {
				t.Fatalf("finding = %#v, want %q blocker", finding, t04TargetRule)
			}
			if finding.StatementIndex != 0 || finding.Location == nil || finding.Location.Line != 1 {
				t.Fatalf("expected statement index 0 with source location, got %#v", finding)
			}
			if finding.Metadata["source_length"] != 200 || finding.Metadata["target_length"] != 20 {
				t.Fatalf("expected source_length=200 target_length=20, got %#v", finding.Metadata)
			}
		})
	}
}

// TestAuditSQLT04DisabledOrNonApplicableRulesStaySilent covers matrix case D:
// no gap is emitted when the target rule is disabled, non-applicable, or
// configured with required=false / requires_metadata unset.
func TestAuditSQLT04DisabledOrNonApplicableRulesStaySilent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		sql        string
		configPath func(t *testing.T) string
	}{
		{
			name: "rule disabled",
			sql:  "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			configPath: func(t *testing.T) string {
				return writeIsolatedPolicy(t, map[string]policy.RulePolicy{t04TargetRule: {Enabled: false}})
			},
		},
		{
			name:       "non-applicable action",
			sql:        "ALTER TABLE t ADD COLUMN c2 INT;",
			configPath: writeT04TargetPolicy,
		},
		{
			name: "required false stays silent",
			sql:  "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			configPath: func(t *testing.T) string {
				return writeIsolatedPolicy(t, map[string]policy.RulePolicy{
					t04TargetRule: {
						Enabled: true,
						Level:   "blocker",
						Params:  map[string]any{"required": false, "requires_metadata": true},
					},
				})
			},
		},
		{
			name: "requires_metadata unset keeps legacy silent path",
			sql:  "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			configPath: func(t *testing.T) string {
				return writeIsolatedPolicy(t, map[string]policy.RulePolicy{
					t04TargetRule: {
						Enabled: true,
						Level:   "blocker",
						Params:  map[string]any{"required": true},
					},
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    spec.DialectMySQL,
				ConfigPath: tc.configPath(t),
			})
			if err != nil {
				t.Fatalf("expected successful audit, got %v", err)
			}
			statement := result.Statements[0]
			if len(statement.EvidenceGaps) != 0 {
				t.Fatalf("expected no evidence gaps, got %#v", statement.EvidenceGaps)
			}
			if len(statement.Findings) != 0 {
				t.Fatalf("expected no findings, got %#v", statement.Findings)
			}
			if statement.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("expected complete coverage, got statement=%q aggregate=%q", statement.Coverage.Status, result.Coverage.Status)
			}
			if result.Verdict != report.VerdictPass {
				t.Fatalf("expected pass verdict, got %q", result.Verdict)
			}
		})
	}
}

// TestAuditSQLT04ProviderErrorsStayErrors covers matrix case E: provider errors
// keep the existing error contract and never degrade into evidence gaps.
func TestAuditSQLT04ProviderErrorsStayErrors(t *testing.T) {
	t.Parallel()
	configPath := writeT04TargetPolicy(t)
	wantErr := errors.New("snapshot store unreachable")

	cases := []struct {
		name     string
		provider MetadataProvider
		wantWrap string
	}{
		{
			name:     "instance facts error",
			provider: &t04SnapshotProvider{factsErr: wantErr},
			wantWrap: "load instance facts",
		},
		{
			name:     "table snapshot error",
			provider: &t04SnapshotProvider{snapshotErr: wantErr},
			wantWrap: "load table snapshot",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:              "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
				Dialect:          spec.DialectMySQL,
				ConfigPath:       configPath,
				Schema:           "app",
				MetadataProvider: tc.provider,
			})
			if err == nil || !errors.Is(err, wantErr) {
				t.Fatalf("expected wrapped provider error %v, got result=%#v err=%v", wantErr, result, err)
			}
			if !strings.Contains(err.Error(), tc.wantWrap) {
				t.Fatalf("expected %q wrap in %v", tc.wantWrap, err)
			}
		})
	}
}

// TestAuditSQLT04MixedResultsPreserveSemantics covers matrix case F: statement
// identity, unsupported precedence, parser-failure priority, and per-statement
// gap/finding attribution across independent targets.
func TestAuditSQLT04MixedResultsPreserveSemantics(t *testing.T) {
	t.Parallel()
	configPath := writeT04TargetPolicy(t)

	t.Run("unverified plus unsupported stays incomplete", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); CREATE SEQUENCE golden_seq START WITH 1;",
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
		})
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("expected 2 statements, got %#v", result.Statements)
		}
		if result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected first statement unverified, got %q", result.Statements[0].Coverage.Status)
		}
		if len(result.Statements[0].EvidenceGaps) != 1 {
			t.Fatalf("expected one evidence gap on first statement, got %#v", result.Statements[0].EvidenceGaps)
		}
		if result.Statements[1].Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expected second statement incomplete, got %q", result.Statements[1].Coverage.Status)
		}
		if result.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expected aggregate incomplete, got %q", result.Coverage.Status)
		}
		if len(result.Unsupported) != 1 {
			t.Fatalf("expected one unsupported entry, got %#v", result.Unsupported)
		}
	})

	t.Run("parser failure keeps partial result and priority", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); THIS IS NOT VALID SQL AT ALL;",
			Dialect:    spec.DialectMySQL,
			ConfigPath: configPath,
		})
		if err == nil {
			t.Fatalf("expected parser-failure error, got result=%#v", result)
		}
		if len(result.Diagnostics) == 0 {
			t.Fatalf("expected parser diagnostics, got %#v", result.Diagnostics)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected partial result with one statement, got %#v", result.Statements)
		}
		if result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected retained statement unverified, got %q", result.Statements[0].Coverage.Status)
		}
		if len(result.Statements[0].EvidenceGaps) != 1 {
			t.Fatalf("expected retained evidence gap, got %#v", result.Statements[0].EvidenceGaps)
		}
		if result.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expected aggregate incomplete from parser failure, got %q", result.Coverage.Status)
		}
	})

	t.Run("gap on one target never suppresses blocker on another", func(t *testing.T) {
		provider := &t04SnapshotProvider{snapshots: map[string]*spec.TableSnapshot{
			"app.t": {
				Exists:  true,
				Table:   &spec.Table{Name: "t"},
				Columns: []spec.Column{{Name: "c", Type: "varchar", Length: 200}},
			},
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); ALTER TABLE t2 MODIFY COLUMN c2 VARCHAR(20);",
			Dialect:          spec.DialectMySQL,
			ConfigPath:       configPath,
			Schema:           "app",
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("expected successful audit, got %v", err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("expected 2 statements, got %#v", result.Statements)
		}
		first := result.Statements[0]
		if first.Coverage.Status != report.CoverageComplete || len(first.Findings) != 1 || len(first.EvidenceGaps) != 0 {
			t.Fatalf("expected complete+blocker on first statement, got %#v", first)
		}
		second := result.Statements[1]
		if second.Coverage.Status != report.CoverageUnverified || len(second.EvidenceGaps) != 1 || len(second.Findings) != 0 {
			t.Fatalf("expected unverified+gap on second statement, got %#v", second)
		}
		if result.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("expected aggregate unverified, got %q", result.Coverage.Status)
		}
		if result.Verdict != report.VerdictReject {
			t.Fatalf("expected reject verdict (blocker beats gap floor), got %q", result.Verdict)
		}
	})
}

// t04SnapshotProvider is a controlled substitute for metadata-gap tests: it
// returns fixed snapshots keyed by schema.table and can inject provider errors.
// Real MySQL 8.4.10 metadata proofs live in the T04 golden cases, not here.
type t04SnapshotProvider struct {
	snapshots   map[string]*spec.TableSnapshot
	factsErr    error
	snapshotErr error
}

func (p *t04SnapshotProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	if p.factsErr != nil {
		return nil, p.factsErr
	}
	return &spec.InstanceFacts{}, nil
}

func (p *t04SnapshotProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	if p.snapshotErr != nil {
		return nil, p.snapshotErr
	}
	return p.snapshots[schema+"."+table], nil
}
