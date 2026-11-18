//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditPGColumnFormatContract(t *testing.T) {
	for _, tc := range []struct{ clause, storage, id, verdict string }{
		{"ALTER COLUMN id SET STORAGE EXTERNAL", "p", "ddl.pg.alter.column.storage.valid", "reject"},
		{"ALTER COLUMN id SET STORAGE DEFAULT", "p", "", "pass"},
		{"ALTER COLUMN id SET COMPRESSION pglz", "p", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN id SET COMPRESSION pglz", "x", "", "pass"},
		{"ALTER COLUMN id SET COMPRESSION default", "p", "", "pass"},
		{`ALTER COLUMN id SET COMPRESSION "private_method_payload"`, "x", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN id SET COMPRESSION lz4", "x", "", "review"},
		{"ALTER COLUMN ctid SET STORAGE PLAIN", "p", "ddl.pg.alter.column.storage.valid", "reject"},
		{"ALTER COLUMN ctid SET COMPRESSION default", "p", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN ctid SET STATISTICS DEFAULT", "p", "ddl.pg.alter.column.statistics.target.valid", "reject"},
		{"DROP COLUMN IF EXISTS ctid", "p", "ddl.alter.drop_column.exists.require", "reject"},
	} {
		catalog := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", TypeStorage: tc.storage}}, SystemColumns: []PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}
		p := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGColumns: catalog}}
		r, err := Audit(context.Background(), Request{SQL: "ALTER TABLE app.t " + tc.clause, Dialect: DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if err != nil || string(r.Verdict) != tc.verdict || len(r.Statements) != 1 {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		found := false
		for _, f := range r.Statements[0].Findings {
			if f.RuleID == tc.id {
				found = true
			}
			if strings.Contains(tc.clause, "ctid") && f.RuleID == "ddl.pg.alter.drop_column.advisory" {
				t.Fatal("system deletion advice", f)
			}
			b, _ := json.Marshal(f.Metadata)
			if strings.Contains(string(b), "private_method_payload") || strings.Contains(string(b), "PGColumns") {
				t.Fatal("private facts leaked", string(b))
			}
			if tc.id == "" && strings.HasPrefix(f.RuleID, "ddl.pg.alter.column.") {
				t.Fatal("unexpected format finding", f)
			}
		}
		if found != (tc.id != "") || tc.verdict == "review" && (r.Coverage.Status != "unverified" || len(r.Statements[0].EvidenceGaps) != 1) {
			t.Fatal("shared verdict/gap lost", r)
		}
	}
}
