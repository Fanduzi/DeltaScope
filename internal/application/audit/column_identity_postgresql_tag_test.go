//go:build postgresql

package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnIdentityPipeline(t *testing.T) {
	const (
		add         = "ddl.alter.add_column.exists.forbid"
		source      = "ddl.alter.rename_column.exists.require"
		destination = "ddl.alter.rename_column.target.exists.forbid"
		table       = "ddl.table.exists.alter.require"
	)
	for _, tc := range []struct {
		name, sql, ruleID, reason, fact string
		kind                            string
		present                         bool
	}{
		{name: "duplicate", sql: `ALTER TABLE app.t ADD COLUMN id int`, ruleID: add, reason: "column_already_exists", kind: "r", present: true},
		{name: "guard skips execution warning", sql: `ALTER TABLE app.t ADD COLUMN IF NOT EXISTS id int NOT NULL`, kind: "r", present: true},
		{name: "guard system", sql: `ALTER TABLE app.t ADD COLUMN IF NOT EXISTS ctid int`, ruleID: add, reason: "system_column_name_conflict", kind: "r", present: true},
		{name: "quote case", sql: `ALTER TABLE app.t RENAME COLUMN "ID" TO moved`, ruleID: source, reason: "column_absent", kind: "r", present: true},
		{name: "same name", sql: `ALTER TABLE app.t RENAME COLUMN id TO id`, ruleID: destination, reason: "column_already_exists", kind: "r", present: true},
		{name: "quote spaces", sql: `ALTER TABLE app.t RENAME COLUMN " " TO " ID "`, kind: "r", present: true},
		{name: "guard absent", sql: `ALTER TABLE IF EXISTS app.t ADD COLUMN id int`},
		{name: "missing legacy shape but sequence exists", sql: `ALTER TABLE IF EXISTS app.t ADD COLUMN id int`, ruleID: add, reason: "unproven_column_identity", fact: "target_table.pg_columns", kind: "S"},
		{name: "unqualified absence", sql: `ALTER TABLE IF EXISTS t ADD COLUMN id int`, ruleID: table, reason: "unproven_alter_table_relation", fact: "target_table.pg_relation"},
		{name: "stale absence", sql: `CREATE TABLE app.predecessor(id int); ALTER TABLE IF EXISTS app.t ADD COLUMN id int`, ruleID: table, reason: "unproven_alter_table_relation", fact: "target_table.pg_relation"},
		{name: "stale member", sql: `ALTER TABLE app.t ADD COLUMN c int; ALTER TABLE app.t RENAME COLUMN c TO moved`, ruleID: source, reason: "unproven_column_identity", fact: "target_table.pg_columns", kind: "r", present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &spec.TableSnapshot{Schema: "app", Exists: tc.present, PGRelation: spec.PGRelationState{Name: "t", Kind: tc.kind}}
			if tc.present {
				snapshot.Table = &spec.Table{Name: "t"}
				snapshot.PGColumns = &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}, {Name: " "}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true}}}
			}
			p := &fakeMetadataProvider{snapshot: snapshot, instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}}
			result, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
			if err != nil || len(result.Statements) == 0 {
				t.Fatalf("audit: %v, %+v", err, result)
			}
			s := result.Statements[len(result.Statements)-1]
			found := false
			for _, f := range s.Findings {
				if f.RuleID == tc.ruleID && tc.fact == "" && f.Metadata["reason_code"] == tc.reason {
					found = true
				}
				if f.RuleID == table || (tc.ruleID == "" && f.RuleID != "ddl.alter.rename_column.forbid") {
					t.Fatalf("unexpected finding: %+v", f)
				}
			}
			for _, gap := range s.EvidenceGaps {
				if gap.RuleID == tc.ruleID && gap.ReasonCode == tc.reason && len(gap.RequiredFacts) == 1 && gap.RequiredFacts[0] == tc.fact {
					found = true
				}
			}
			if tc.ruleID != "" && !found || tc.ruleID == "" && len(s.EvidenceGaps) != 0 {
				t.Fatalf("expected rule=%q reason=%q fact=%q, findings=%+v gaps=%+v", tc.ruleID, tc.reason, tc.fact, s.Findings, s.EvidenceGaps)
			}
			encoded, err := json.Marshal(s)
			if err != nil || strings.Contains(string(encoded), `"PGRelation"`) || strings.Contains(string(encoded), `"PGTableIfExists"`) || strings.Contains(string(encoded), `"PGAddColumnIfNotExists"`) {
				t.Fatalf("unexpected private field: %s, %v", encoded, err)
			}
		})
	}
}
