// Package audit verifies the T25 whole-table InnoDB tablespace path end to
// end through the public AuditSQL seam: the DISCARD blocker rejects under
// a complete InnoDB premise, IMPORT warns without firing the discard
// forbid, the InnoDB-only applicability gate separates proven native
// refusals (non-InnoDB engine, partitioned target) from owed evidence gaps
// (unobserved engine/partitioned), disabling the policy rule keeps the
// applicability boundary, and TiDB suppresses every governance finding at
// the vendor boundary. The ordered state drops the row-bearing statistics
// the data-file move can no longer vouch while proven refusals keep the
// exact pre-state.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t25RuleDiscardForbid        = "ddl.alter.discard_tablespace.forbid"
	t25RuleImportWarn           = "ddl.alter.import_tablespace.warn"
	t25RuleDiscardApplicability = "ddl.alter.discard_tablespace.applicability"
	t25RuleImportApplicability  = "ddl.alter.import_tablespace.applicability"
)

// t25Provider answers a proven InnoDB (or fixture-shaped) target table.
func t25Provider(engine string, partitioned *bool) *dmlTableMetadataProvider {
	return &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"golden.t": {
			Exists:      true,
			Table:       &spec.Table{Schema: "golden", Name: "t"},
			Partitioned: partitioned,
			Options:     map[string]string{"engine": engine},
		},
	}}
}

func t25Bool(value bool) *bool { return &value }

func t25Audit(t *testing.T, dialect spec.Dialect, sql string, provider *dmlTableMetadataProvider, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden",
		ConfigPath: policy, MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("%s %q: unexpected error %v", dialect, sql, err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("%s %q: statements = %d, want 1", dialect, sql, len(result.Statements))
	}
	return result
}

func t25AllRulesPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t25RuleDiscardForbid:        {level: "blocker", params: "      forbid: true\n"},
		t25RuleImportWarn:           {level: "warning"},
		t25RuleDiscardApplicability: {level: "blocker"},
		t25RuleImportApplicability:  {level: "blocker"},
	})
}

// The golden path: a complete InnoDB premise yields exactly the policy
// blocker, no evidence gaps, complete coverage, reject.
func TestT25DiscardTablespaceGoldenPath(t *testing.T) {
	t.Parallel()
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD TABLESPACE",
		t25Provider("innodb", t25Bool(false)), t25AllRulesPolicy(t))
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict = %s, want reject", result.Verdict)
	}
	statement := result.Statements[0]
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %+v, want exactly the discard blocker", statement.Findings)
	}
	finding := statement.Findings[0]
	if finding.RuleID != t25RuleDiscardForbid || finding.Level != "blocker" {
		t.Fatalf("finding = %+v, want %s blocker", finding, t25RuleDiscardForbid)
	}
	if finding.Metadata["action"] != "discard_tablespace" || finding.Metadata["table"] != "t" {
		t.Fatalf("finding metadata = %+v, want authored action and table", finding.Metadata)
	}
	if len(statement.EvidenceGaps) != 0 {
		t.Fatalf("complete premise must owe no gaps, got %+v", statement.EvidenceGaps)
	}
	if statement.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage = %s, want complete", statement.Coverage.Status)
	}
}

// IMPORT TABLESPACE warns on the unattested file and never fires the
// discard forbid — the two actions stay distinct.
func TestT25ImportTablespaceControl(t *testing.T) {
	t.Parallel()
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t IMPORT TABLESPACE",
		t25Provider("innodb", t25Bool(false)), t25AllRulesPolicy(t))
	statement := result.Statements[0]
	if len(statement.Findings) != 1 || statement.Findings[0].RuleID != t25RuleImportWarn ||
		statement.Findings[0].Level != "warning" {
		t.Fatalf("findings = %+v, want exactly the import warning", statement.Findings)
	}
	if len(statement.EvidenceGaps) != 0 {
		t.Fatalf("complete premise must owe no gaps, got %+v", statement.EvidenceGaps)
	}
}

// An ordinary ALTER never trips the tablespace rules.
func TestT25OrdinaryAlterControl(t *testing.T) {
	t.Parallel()
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD COLUMN c INT",
		t25Provider("innodb", t25Bool(false)), t25AllRulesPolicy(t))
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("ordinary alter findings = %+v, want none", result.Statements[0].Findings)
	}
}

// A proven MyISAM target emits the native refusal — a distinct rule ID
// from the policy forbid, which still fires on the authored form.
func TestT25NonInnoDBRefusal(t *testing.T) {
	t.Parallel()
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD TABLESPACE",
		t25Provider("myisam", t25Bool(false)), t25AllRulesPolicy(t))
	statement := result.Statements[0]
	ruleIDs := map[string]bool{}
	for _, f := range statement.Findings {
		ruleIDs[f.RuleID] = true
		if f.RuleID == t25RuleDiscardApplicability && f.Metadata["reason_code"] != "tablespace_engine_inapplicable" {
			t.Fatalf("refusal metadata = %+v, want engine_inapplicable", f.Metadata)
		}
	}
	if !ruleIDs[t25RuleDiscardForbid] || !ruleIDs[t25RuleDiscardApplicability] {
		t.Fatalf("findings = %+v, want forbid + applicability refusal together", statement.Findings)
	}
}

// An unproven engine owes the fact through an evidence gap — never a
// fabricated refusal and never a fabricated pass.
func TestT25UnknownEngineGap(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"golden.t": {
			Exists:      true,
			Table:       &spec.Table{Schema: "golden", Name: "t"},
			Partitioned: t25Bool(false),
			Options:     map[string]string{},
		},
	}}
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD TABLESPACE",
		provider, t25AllRulesPolicy(t))
	statement := result.Statements[0]
	found := false
	for _, gap := range statement.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.engine" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("gaps = %+v, want target_table.engine owed", statement.EvidenceGaps)
	}
	for _, f := range statement.Findings {
		if f.RuleID == t25RuleDiscardApplicability {
			t.Fatalf("unproven engine must not refuse, got %+v", f)
		}
	}
}

// forbid=false silences the policy blocker but the applicability boundary
// stays: a proven non-InnoDB target still refuses.
func TestT25ForbidOffKeepsApplicability(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{
		t25RuleDiscardForbid:        "      forbid: false\n",
		t25RuleDiscardApplicability: "",
	})
	result := t25Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD TABLESPACE",
		t25Provider("myisam", t25Bool(false)), policy)
	statement := result.Statements[0]
	if len(statement.Findings) != 1 || statement.Findings[0].RuleID != t25RuleDiscardApplicability {
		t.Fatalf("findings = %+v, want only the applicability refusal", statement.Findings)
	}
}

// TiDB refuses the whole-table forms at the vendor boundary: no governance
// finding applies even under a complete premise, and the statement keeps
// a bounded vendor aspect rather than silently passing.
func TestT25TiDBVendorBoundary(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL: "ALTER TABLE t DISCARD TABLESPACE", Dialect: spec.DialectTiDB, Schema: "golden",
		ConfigPath: t25AllRulesPolicy(t), MetadataProvider: t25Provider("innodb", t25Bool(false)),
	})
	if err == nil {
		t.Fatalf("vendor-boundary statement must surface ErrUnsupportedStatement, got nil")
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("TiDB findings = %+v, want none at the vendor boundary", result.Statements)
	}
	if len(result.Unsupported) != 1 ||
		result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason ||
		result.Unsupported[0].Feature != "alter_table.discard_tablespace" {
		t.Fatalf("unsupported = %+v, want the discard_tablespace vendor boundary", result.Unsupported)
	}
}

// DISCARD/IMPORT on a proven InnoDB premise drops the row-bearing
// statistics while keeping the structural shape; proven refusals keep the
// exact pre-state.
func TestT25TablespacePostState(t *testing.T) {
	t.Parallel()
	shape := func(engine string, partitioned *bool) *spec.TableSnapshot {
		cardinality := int64(7)
		return &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "t"},
			Columns:     []spec.Column{t10r2Col("id", "int", true, 0)},
			PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &cardinality},
			Partitioned: partitioned,
			Options: map[string]string{"engine": engine, "cache_status": "disable",
				"table_rows": "9", "data_length": "100", "avg_row_length": "8", "comment": "kept"},
		}
	}
	for _, sql := range []string{"ALTER TABLE t DISCARD TABLESPACE", "ALTER TABLE t IMPORT TABLESPACE"} {
		entry := t10r4Apply(t, spec.DialectMySQL, shape("innodb", t25Bool(false)), sql)
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			t.Fatalf("%s must keep the present entry, got %+v", sql, entry)
		}
		for _, key := range []string{"table_rows", "data_length", "avg_row_length"} {
			if _, ok := entry.shape.Options[key]; ok {
				t.Fatalf("%s must drop Options[%s]", sql, key)
			}
		}
		if entry.shape.Options["comment"] != "kept" || entry.shape.Options["engine"] != "innodb" {
			t.Fatalf("%s must keep non-statistic options, got %+v", sql, entry.shape.Options)
		}
		if len(entry.shape.Columns) != 1 || entry.shape.PrimaryKey == nil || entry.shape.PrimaryKey.Cardinality != nil {
			t.Fatalf("%s must keep the shape but drop cardinality, got %+v", sql, entry.shape)
		}
	}
	for _, tc := range []struct {
		name        string
		engine      string
		partitioned *bool
	}{
		{"myisam", "myisam", t25Bool(false)},
		{"partitioned", "innodb", t25Bool(true)},
	} {
		entry := t10r4Apply(t, spec.DialectMySQL, shape(tc.engine, tc.partitioned),
			"ALTER TABLE t DISCARD TABLESPACE")
		if entry == nil || entry.shape == nil || entry.shape.Options["table_rows"] != "9" {
			t.Fatalf("%s refusal must keep the exact pre-state, got %+v", tc.name, entry)
		}
	}
	entry := t10r4Apply(t, spec.DialectTiDB, shape("innodb", t25Bool(false)),
		"ALTER TABLE t DISCARD TABLESPACE")
	if entry == nil || entry.shape == nil || entry.shape.Options["table_rows"] != "9" {
		t.Fatalf("TiDB vendor refusal must keep the exact pre-state, got %+v", entry)
	}
}
