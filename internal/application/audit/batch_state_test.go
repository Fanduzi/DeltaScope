// Package audit verifies the T05-A1 ordered schema-state path.
// input: multi-statement MySQL/TiDB DDL batches against fake metadata providers
// output: per-statement pre-state isolation, conditional derivation, invalidation, evidence-gap, and provider-call assertions
// pos: application-layer contract tests for the first prospective migration path (issue #84/T05-A1)
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

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t05RuleCreateForbid       = "ddl.table.exists.create.forbid"
	t05RuleAlterRequire       = "ddl.table.exists.alter.require"
	t05RuleAddColumnForbid    = "ddl.alter.add_column.exists.forbid"
	t05RuleCreateIndexColumns = "ddl.create_index.columns.exists.require"
	t05RuleEngineAllowlist    = "ddl.table.engine.allowlist"
)

const t05FirstPathSQL = "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t (c);"

// t05PolicyPath writes a policy that disables every default rule except the
// listed ones, so assertions bind to the ordered-state seam rather than the
// full default rule surface.
func t05PolicyPath(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
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
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

func t05FourRulePolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:       "",
		t05RuleAlterRequire:       "",
		t05RuleAddColumnForbid:    "",
		t05RuleCreateIndexColumns: "      required: true\n",
	})
}

// t05AbsentProvider answers exists:false for every table the audit consults.
type t05AbsentProvider struct {
	calls []string
}

func (p *t05AbsentProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t05AbsentProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t05FindingsByRule(result report.Result, statementIndex int, ruleID string) []rule.Finding {
	out := make([]rule.Finding, 0)
	for _, finding := range result.Statements[statementIndex].Findings {
		if finding.RuleID == ruleID {
			out = append(out, finding)
		}
	}
	return out
}

func t05GapsByRule(result report.Result, statementIndex int, ruleID string) []rule.EvidenceGap {
	out := make([]rule.EvidenceGap, 0)
	for _, gap := range result.Statements[statementIndex].EvidenceGaps {
		if gap.RuleID == ruleID {
			out = append(out, gap)
		}
	}
	return out
}

// A: online first migration path — every statement reads the ordered state
// derived so far; the provider is consulted once per table.
func TestBatchStateOnlineFirstPathDerivesSequentialState(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result, err := AuditSQL(context.Background(), Request{
				SQL:              t05FirstPathSQL,
				Dialect:          dialect,
				Schema:           "app",
				ConfigPath:       t05FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if result.Verdict != report.VerdictPass {
				t.Fatalf("expected pass verdict, got %s (findings=%+v)", result.Verdict, result.Statements)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "app.t" {
				t.Fatalf("expected exactly one provider read for app.t, got %#v", provider.calls)
			}
			for i := range result.Statements {
				if len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d must have no findings, got %+v", i, result.Statements[i].Findings)
				}
				if len(result.Statements[i].EvidenceGaps) != 0 {
					t.Fatalf("statement %d must have no evidence gaps, got %+v", i, result.Statements[i].EvidenceGaps)
				}
				if result.Statements[i].Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %+v, want complete", i, result.Statements[i].Coverage)
				}
			}
		})
	}
}

// B: the standalone CREATE INDEX column check fires per missing plain column
// against the derived post-CREATE shape.
func TestBatchStateCreateIndexMissingColumnFindsDerivedShape(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE t (id INT PRIMARY KEY); CREATE INDEX idx_c ON t (missing_c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 1, t05RuleCreateIndexColumns)
	if len(findings) != 1 {
		t.Fatalf("expected one missing-column finding, got %+v", result.Statements[1].Findings)
	}
	finding := findings[0]
	if finding.Level != rule.LevelBlocker {
		t.Fatalf("expected blocker level, got %s", finding.Level)
	}
	if finding.Metadata["column"] != "missing_c" || finding.Metadata["index"] != "idx_c" || finding.Metadata["table"] != "t" || finding.Metadata["exists"] != false {
		t.Fatalf("finding metadata must carry index/column/table/exists:false, got %#v", finding.Metadata)
	}
}

// C: a repeated plain ADD COLUMN reads the derived post-state and fires the
// duplicate-column blocker on the third statement.
func TestBatchStateRepeatedAddColumnFiresOnDerivedState(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; ALTER TABLE t ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 2, t05RuleAddColumnForbid)
	if len(findings) != 1 {
		t.Fatalf("expected one duplicate-column blocker on statement 2, got %+v", result.Statements[2].Findings)
	}
	if findings[0].Metadata["name"] != "c" {
		t.Fatalf("expected duplicate column name c, got %#v", findings[0].Metadata)
	}
}

// D: an unaudited mutation invalidates only its own table — dependent checks
// on t degrade to unknown while an unrelated table u still derives cleanly.
func TestBatchStateUnauditedMutationInvalidatesOnlyItsTable(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE t (id INT PRIMARY KEY); CREATE TABLE u (id INT PRIMARY KEY);" +
			" ALTER TABLE t ADD COLUMN g INT GENERATED ALWAYS AS (id+1) STORED;" +
			" ALTER TABLE t ADD COLUMN c INT; ALTER TABLE u ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	// The generated-column aspect is recognized-but-unsupported evidence, so
	// the audit returns its partial result with ErrUnsupportedStatement.
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement for the unaudited generated column, got %v", err)
	}
	if result.Statements[2].Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("generated-column ALTER must stay coverage incomplete, got %+v", result.Statements[2].Coverage)
	}
	// Statement 3 depends on the invalidated t: both existence rules report
	// unknown state as gaps instead of findings.
	for _, ruleID := range []string{t05RuleAlterRequire, t05RuleAddColumnForbid} {
		gaps := t05GapsByRule(result, 3, ruleID)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("statement 3 rule %s must report one unknown_table_state gap, got %+v", ruleID, result.Statements[3].EvidenceGaps)
		}
	}
	if len(result.Statements[3].Findings) != 0 {
		t.Fatalf("invalidated table must produce gaps, not findings, got %+v", result.Statements[3].Findings)
	}
	if result.Statements[3].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("statement 3 coverage = %+v, want unverified", result.Statements[3].Coverage)
	}
	// Statement 4 targets u, untouched by t's invalidation.
	if len(result.Statements[4].Findings) != 0 || len(result.Statements[4].EvidenceGaps) != 0 {
		t.Fatalf("unrelated table u must analyze cleanly, findings=%+v gaps=%+v", result.Statements[4].Findings, result.Statements[4].EvidenceGaps)
	}
	if result.Statements[4].Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement 4 coverage = %+v, want complete", result.Statements[4].Coverage)
	}
	// Provider consulted once per touched table.
	sort.Strings(provider.calls)
	if strings.Join(provider.calls, ",") != "app.t,app.u" {
		t.Fatalf("expected one provider read per table, got %#v", provider.calls)
	}
}

// E: standalone CREATE INDEX on a confirmed-absent table emits one
// table_not_found blocker — not a missing-column list.
func TestBatchStateCreateIndexOnAbsentTableReportsTableNotFound(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE INDEX idx_c ON gone (a, b);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 0, t05RuleCreateIndexColumns)
	if len(findings) != 1 {
		t.Fatalf("expected one table_not_found finding, got %+v", result.Statements[0].Findings)
	}
	if findings[0].Metadata["reason"] != "table_not_found" || findings[0].Metadata["table"] != "gone" {
		t.Fatalf("expected table_not_found reason on table gone, got %#v", findings[0].Metadata)
	}
}

// F: offline — no provider at all — the first statement reports exactly one
// unknown_table_state gap while the derived statements stay complete.
func TestBatchStateOfflineFirstPathGapProfile(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        t05FirstPathSQL,
				Dialect:    dialect,
				Schema:     "app",
				ConfigPath: t05FourRulePolicy(t),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			gaps := t05GapsByRule(result, 0, t05RuleCreateForbid)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("statement 0 must carry exactly one unknown_table_state gap, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageUnverified {
				t.Fatalf("statement 0 coverage = %+v, want unverified", result.Statements[0].Coverage)
			}
			for i := 1; i < 3; i++ {
				if len(result.Statements[i].EvidenceGaps) != 0 || len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d must be gap/finding-free on derived state, got gaps=%+v findings=%+v", i, result.Statements[i].EvidenceGaps, result.Statements[i].Findings)
				}
				if result.Statements[i].Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %+v, want complete", i, result.Statements[i].Coverage)
				}
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("expected review verdict, got %s", result.Verdict)
			}
		})
	}
}

// A present table whose column set the provider did not supply is
// present-incomplete: member-existence rules report incomplete structure
// instead of fabricating missing members, while table-level facts stay usable.
func TestBatchStateIncompleteProviderShapeYieldsIncompleteStructureGap(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.t": {Exists: true, Table: &spec.Table{Schema: "app", Name: "t"}},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t (c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i := 0; i < 2; i++ {
		if len(result.Statements[i].Findings) != 0 {
			t.Fatalf("statement %d must not fabricate missing members, got %+v", i, result.Statements[i].Findings)
		}
	}
	gaps := t05GapsByRule(result, 0, t05RuleAddColumnForbid)
	if len(gaps) != 1 || gaps[0].ReasonCode != "incomplete_table_structure" {
		t.Fatalf("statement 0 add-column must report incomplete_table_structure, got %+v", result.Statements[0].EvidenceGaps)
	}
	gaps = t05GapsByRule(result, 1, t05RuleCreateIndexColumns)
	if len(gaps) != 1 || gaps[0].ReasonCode != "incomplete_table_structure" {
		t.Fatalf("statement 1 create-index must report incomplete_table_structure, got %+v", result.Statements[1].EvidenceGaps)
	}
}

// A mid-batch parse failure contaminates everything after it: later
// statements get unknown state even when an earlier statement derived facts.
func TestBatchStateParseFailureContaminatesLaterStatements(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE t (id INT PRIMARY KEY);\nSELECT * FROM WHERE;\nALTER TABLE t ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("expected parser-failure error, got result=%#v", result)
	}
	if len(result.Statements) < 2 {
		t.Fatalf("expected partial statements, got %#v", result.Statements)
	}
	last := len(result.Statements) - 1
	gaps := t05GapsByRule(result, last, t05RuleAlterRequire)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("post-failure ALTER must see unknown state, got %+v", result.Statements[last].EvidenceGaps)
	}
}

// TRUNCATE preserves structure but clears row statistics: a provider
// table_rows fact must not survive into the post-TRUNCATE state.
func TestBatchStateTruncateClearsRowStatistics(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.big": {
			Exists:  true,
			Table:   &spec.Table{Schema: "app", Name: "big"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
			Options: map[string]string{"table_rows": "1000"},
		},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "TRUNCATE TABLE big; TRUNCATE TABLE big;",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		ConfigPath: t05PolicyPath(t, map[string]string{
			"ddl.table.truncate.rows.max_count": "      limit: 100\n",
		}),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(t05FindingsByRule(result, 0, "ddl.table.truncate.rows.max_count")) != 1 {
		t.Fatalf("first TRUNCATE must see provider table_rows, got %+v", result.Statements[0].Findings)
	}
	if len(t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count")) != 0 {
		t.Fatalf("second TRUNCATE must not reuse cleared table_rows, got %+v", result.Statements[1].Findings)
	}
}

// G: a provider error aborts the audit — it must never degrade into an
// unknown-state evidence gap.
func TestBatchStateProviderErrorStaysAnError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("snapshot backend down")
	result, err := AuditSQL(context.Background(), Request{
		SQL:              t05FirstPathSQL,
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: &dmlTableMetadataProvider{snapshotErr: wantErr},
	})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected provider error %v, got result=%#v err=%v", wantErr, result, err)
	}
}

// H: a policy blocker does not erase a deterministic transition — the
// allowlist-blocked CREATE still leaves a known shape for the next ALTER.
func TestBatchStatePolicyBlockerKeepsDeterministicTransition(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE t (id INT PRIMARY KEY) ENGINE=MyISAM; ALTER TABLE t ADD COLUMN c INT;",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		ConfigPath: t05PolicyPath(t, map[string]string{
			t05RuleEngineAllowlist: "      values: [InnoDB]\n",
			t05RuleAlterRequire:    "",
			t05RuleAddColumnForbid: "",
		}),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(t05FindingsByRule(result, 0, t05RuleEngineAllowlist)) != 1 {
		t.Fatalf("expected the engine allowlist blocker on statement 0, got %+v", result.Statements[0].Findings)
	}
	if len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("statement 1 must read the derived post-CREATE state despite the blocker, findings=%+v gaps=%+v",
			result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
	if result.Statements[1].Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement 1 coverage = %+v, want complete", result.Statements[1].Coverage)
	}
}

// Provider snapshots are request-local: a second audit on the same provider
// object asks again, and entries never alias statement metadata.
func TestBatchStateProviderReadsAreRequestLocal(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	for run := 0; run < 2; run++ {
		result, err := AuditSQL(context.Background(), Request{
			SQL:              t05FirstPathSQL,
			Dialect:          spec.DialectMySQL,
			Schema:           "app",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit run %d: %v", run, err)
		}
		if result.Verdict != report.VerdictPass {
			t.Fatalf("run %d expected pass, got %s", run, result.Verdict)
		}
	}
	if len(provider.calls) != 2 {
		t.Fatalf("expected one provider call per request (2 total), got %#v", provider.calls)
	}
}
