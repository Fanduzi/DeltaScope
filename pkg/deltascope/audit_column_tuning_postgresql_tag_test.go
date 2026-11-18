//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditPGColumnTuningContract(t *testing.T) {
	for _, tc := range []struct {
		sql, id, verdict string
		unknown          bool
	}{
		{"ALTER COLUMN id SET STATISTICS -2", "ddl.pg.alter.column.statistics.target.valid", "reject", false},
		{"ALTER COLUMN id SET STATISTICS 10001", "ddl.pg.alter.column.statistics.clamp.warn", "review", false},
		{"ALTER COLUMN id SET(n_distinct=-1.1)", "ddl.pg.alter.column.options.valid", "reject", false},
		{"ALTER COLUMN id SET(n_distinct=1e-999)", "", "review", false},
		{"DROP COLUMN IF EXISTS missing", "", "pass", false},
		{"DROP COLUMN missing", "ddl.alter.drop_column.exists.require", "reject", false},
		{"DROP COLUMN IF EXISTS missing", "ddl.pg.alter.drop_column.advisory", "review", true},
	} {
		catalog := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}
		if tc.unknown {
			catalog = nil
		}
		p := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGColumns: catalog}}
		r, err := Audit(context.Background(), Request{SQL: "ALTER TABLE app.t " + tc.sql, Dialect: DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if err != nil || string(r.Verdict) != tc.verdict || len(r.Statements) != 1 {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		found := false
		for _, f := range r.Statements[0].Findings {
			if f.RuleID == tc.id {
				found = true
			}
			b, _ := json.Marshal(f)
			for _, payload := range []string{"PGColumns", "10001", "-1.1"} {
				if strings.Contains(string(b), payload) {
					t.Fatalf("payload leaked: %s", b)
				}
			}
			if tc.id == "" && (strings.HasPrefix(f.RuleID, "ddl.pg.alter.column.") || f.RuleID == "ddl.pg.alter.drop_column.advisory") {
				t.Fatal("no-op finding", f)
			}
		}
		if found != (tc.id != "") || (tc.unknown || tc.id == "" && tc.verdict == "review") && (r.Coverage.Status != "unverified" || len(r.Statements[0].EvidenceGaps) != 1) {
			t.Fatalf("result=%+v", r)
		}
	}
}
