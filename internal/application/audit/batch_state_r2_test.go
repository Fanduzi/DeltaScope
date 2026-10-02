// Package audit verifies the T05-A1-R2 residual fixes for the ordered
// schema-state path.
// input: MySQL/TiDB batches whose member collections are unknown
// (conditional-create derivations, partial provider shapes) plus
// PostgreSQL batches under every metadata request shape
// output: failing-then-fixed assertions pinning per-collection evidence
// gaps on every member-existence rule and the MySQL/TiDB-only dialect
// boundary of the new gap projection
// pos: application-layer rework regressions for issue #84/T05-A1-R2
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// R2-1: every member-existence rule in the shared alterObjectExistenceRule
// class must report its own unknown-collection gap — a derived-incomplete
// table (conditional CREATE on unknown existence) cannot count as
// "checked". Each case enables exactly one rule.
func TestBatchStateR2UnknownMemberCollectionsEmitGaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		ruleID    string
		alterSQL  string
		wantFacts []string
	}{
		{name: "drop index", ruleID: "ddl.alter.drop_index.exists.require",
			alterSQL:  "ALTER TABLE t DROP INDEX ix",
			wantFacts: []string{"target_table.indexes"}},
		{name: "rename index", ruleID: "ddl.alter.rename_index.exists.require",
			alterSQL:  "ALTER TABLE t RENAME INDEX ix TO ix2",
			wantFacts: []string{"target_table.indexes"}},
		{name: "add index forbid", ruleID: "ddl.alter.add_index.exists.forbid",
			alterSQL:  "ALTER TABLE t ADD INDEX ix (id)",
			wantFacts: []string{"target_table.indexes"}},
		{name: "drop column", ruleID: "ddl.alter.drop_column.exists.require",
			alterSQL:  "ALTER TABLE t DROP COLUMN c",
			wantFacts: []string{"target_table.columns"}},
		{name: "modify column", ruleID: "ddl.alter.modify_column.exists.require",
			alterSQL:  "ALTER TABLE t MODIFY COLUMN c BIGINT",
			wantFacts: []string{"target_table.columns"}},
		{name: "change column", ruleID: "ddl.alter.change_column.exists.require",
			alterSQL:  "ALTER TABLE t CHANGE COLUMN c c2 BIGINT",
			wantFacts: []string{"target_table.columns"}},
		{name: "rename column", ruleID: "ddl.alter.rename_column.exists.require",
			alterSQL:  "ALTER TABLE t RENAME COLUMN c TO c2",
			wantFacts: []string{"target_table.columns"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     "CREATE TABLE IF NOT EXISTS t (id INT PRIMARY KEY); " + tc.alterSQL + ";",
				Dialect: spec.DialectMySQL,
				Schema:  "app",
				ConfigPath: t05PolicyPath(t, map[string]string{
					tc.ruleID: "",
				}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements[1].Findings) != 0 {
				t.Fatalf("unknown member state must not fabricate findings, got %+v", result.Statements[1].Findings)
			}
			gaps := t05GapsByRule(result, 1, tc.ruleID)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("%s must report exactly one unknown_table_state gap, got %+v", tc.ruleID, result.Statements[1].EvidenceGaps)
			}
			if len(gaps[0].RequiredFacts) != len(tc.wantFacts) {
				t.Fatalf("required_facts must be %v, got %+v", tc.wantFacts, gaps[0].RequiredFacts)
			}
			for i, fact := range tc.wantFacts {
				if gaps[0].RequiredFacts[i] != fact {
					t.Fatalf("required_facts must be %v in order, got %+v", tc.wantFacts, gaps[0].RequiredFacts)
				}
			}
			if result.Statements[1].Coverage.Status != report.CoverageUnverified {
				t.Fatalf("statement 1 coverage must be unverified, got %+v", result.Statements[1].Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("verdict must floor to review, got %+v", result.Verdict)
			}
		})
	}
}

// R2-1: when the table's own existence is unknown the member rules must pin
// both facts — columns keep [columns, existence] and indexes use the frozen
// [existence, indexes] order.
func TestBatchStateR2UnknownTableStateMemberGapFacts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		ruleID    string
		alterSQL  string
		wantFacts []string
	}{
		{name: "drop index", ruleID: "ddl.alter.drop_index.exists.require",
			alterSQL:  "ALTER TABLE t DROP INDEX ix",
			wantFacts: []string{"target_table.existence", "target_table.indexes"}},
		{name: "drop column", ruleID: "ddl.alter.drop_column.exists.require",
			alterSQL:  "ALTER TABLE t DROP COLUMN c",
			wantFacts: []string{"target_table.columns", "target_table.existence"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tc.alterSQL + ";",
				Dialect: spec.DialectMySQL,
				Schema:  "app",
				ConfigPath: t05PolicyPath(t, map[string]string{
					tc.ruleID: "",
				}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			gaps := t05GapsByRule(result, 0, tc.ruleID)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("%s must report one unknown_table_state gap, got %+v", tc.ruleID, result.Statements[0].EvidenceGaps)
			}
			if len(gaps[0].RequiredFacts) != len(tc.wantFacts) {
				t.Fatalf("required_facts must be %v, got %+v", tc.wantFacts, gaps[0].RequiredFacts)
			}
			for i, fact := range tc.wantFacts {
				if gaps[0].RequiredFacts[i] != fact {
					t.Fatalf("required_facts must be %v in order, got %+v", tc.wantFacts, gaps[0].RequiredFacts)
				}
			}
		})
	}
}

// R2-1 controls: a provider-loaded empty index set is a definite answer and
// still produces the real missing-index finding; a provider that knows the
// target index produces neither finding nor gap. "Unknown" never reads as
// "checked", and "known empty" never reads as "unknown".
func TestBatchStateR2KnownIndexSetKeepsRealAnswers(t *testing.T) {
	t.Parallel()
	t.Run("known empty produces finding", func(t *testing.T) {
		t.Parallel()
		provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
			"app.t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "app", Name: "t"},
				Columns: []spec.Column{{Name: "id", Type: "int"}},
				Indexes: []spec.Index{},
			},
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "ALTER TABLE t DROP INDEX ix;",
			Dialect: spec.DialectMySQL,
			Schema:  "app",
			ConfigPath: t05PolicyPath(t, map[string]string{
				"ddl.alter.drop_index.exists.require": "",
			}),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		findings := t05FindingsByRule(result, 0, "ddl.alter.drop_index.exists.require")
		if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
			t.Fatalf("known-empty index set must produce the missing-index blocker, got %+v", result.Statements[0].Findings)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("a definite empty set is not a gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("known member clean", func(t *testing.T) {
		t.Parallel()
		provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
			"app.t": {
				Exists:  true,
				Table:   &spec.Table{Schema: "app", Name: "t"},
				Columns: []spec.Column{{Name: "id", Type: "int"}},
				Indexes: []spec.Index{{Name: "ix", Columns: []string{"id"}}},
			},
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "ALTER TABLE t DROP INDEX ix;",
			Dialect: spec.DialectMySQL,
			Schema:  "app",
			ConfigPath: t05PolicyPath(t, map[string]string{
				"ddl.alter.drop_index.exists.require": "",
			}),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("a known index must be clean, findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
}
