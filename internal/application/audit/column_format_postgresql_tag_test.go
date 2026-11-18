//go:build postgresql

package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnFormatPipeline(t *testing.T) {
	for _, tc := range []struct {
		clause, storage, id, verdict    string
		known, supported, missingSystem bool
	}{
		{clause: "ALTER COLUMN id SET STORAGE EXTERNAL", storage: "p", id: "ddl.pg.alter.column.storage.valid", verdict: "reject"},
		{clause: "ALTER COLUMN id SET STORAGE PLAIN", storage: "p", verdict: "pass"},
		{clause: "ALTER COLUMN id SET STORAGE DEFAULT", storage: "p", verdict: "pass"},
		{clause: `ALTER COLUMN id SET STORAGE "ExTeRnAl"`, storage: "x", verdict: "pass"},
		{clause: "ALTER COLUMN id SET COMPRESSION pglz", storage: "p", id: "ddl.pg.alter.column.compression.valid", verdict: "reject"},
		{clause: "ALTER COLUMN id SET COMPRESSION pglz", storage: "x", verdict: "pass"},
		{clause: "ALTER COLUMN id SET COMPRESSION default", storage: "p", verdict: "pass"},
		{clause: `ALTER COLUMN id SET COMPRESSION "private_method_payload"`, storage: "x", id: "ddl.pg.alter.column.compression.valid", verdict: "reject"},
		{clause: "ALTER COLUMN id SET COMPRESSION lz4", storage: "x", verdict: "review"},
		{clause: "ALTER COLUMN id SET COMPRESSION lz4", storage: "x", verdict: "pass", known: true, supported: true},
		{clause: "ALTER COLUMN id SET COMPRESSION lz4", storage: "x", id: "ddl.pg.alter.column.compression.valid", verdict: "reject", known: true},
		{clause: "ALTER COLUMN ctid SET STORAGE PLAIN", id: "ddl.pg.alter.column.storage.valid", verdict: "reject"},
		{clause: "ALTER COLUMN ctid SET COMPRESSION default", id: "ddl.pg.alter.column.compression.valid", verdict: "reject"},
		{clause: "ALTER COLUMN ctid SET STATISTICS DEFAULT", id: "ddl.pg.alter.column.statistics.target.valid", verdict: "reject"},
		{clause: "ALTER COLUMN ctid SET(n_distinct=0.5)", id: "ddl.pg.alter.column.options.valid", verdict: "reject"},
		{clause: "DROP COLUMN IF EXISTS ctid", id: "ddl.alter.drop_column.exists.require", verdict: "reject"},
		{clause: "DROP COLUMN IF EXISTS ctid", verdict: "review", missingSystem: true},
	} {
		t.Run(tc.clause+"/"+tc.verdict, func(t *testing.T) {
			c := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id", TypeStorage: tc.storage}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}
			if tc.missingSystem {
				c.SystemColumns = nil
			}
			p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7", PGCompressionLZ4Known: tc.known, PGCompressionLZ4Supported: tc.supported}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: c}}
			r, err := AuditSQL(context.Background(), Request{SQL: "ALTER TABLE app.t " + tc.clause, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
			if err != nil || string(r.Verdict) != tc.verdict || len(r.Statements) != 1 {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			found := false
			for _, f := range r.Statements[0].Findings {
				if f.RuleID == tc.id {
					found = true
				}
				if strings.Contains(tc.clause, "ctid") && !tc.missingSystem && f.RuleID == "ddl.pg.alter.drop_column.advisory" {
					t.Fatal("inapplicable system deletion advice", f)
				}
				if strings.HasPrefix(f.RuleID, "ddl.pg.alter.column.") || f.RuleID == "ddl.alter.drop_column.exists.require" {
					if f.RuleID != tc.id {
						t.Fatal("unexpected bounded finding", f)
					}
					raw, _ := json.Marshal(f)
					if strings.Contains(string(raw), "private_method_payload") || strings.Contains(string(raw), "PGColumns") {
						t.Fatal("private fact leaked", string(raw))
					}
				}
			}
			if found != (tc.id != "") {
				t.Fatal("expected shared finding absent", r)
			}
			if tc.verdict == "review" && (r.Coverage.Status != "unverified" || len(r.Statements[0].EvidenceGaps) != 1) {
				t.Fatal("missing exact unknown proof", r)
			}
		})
	}
}

func TestPGColumnFormatPriorDDLAndParser(t *testing.T) {
	for _, sql := range []string{
		"ALTER TABLE app.t ADD COLUMN other_id int;ALTER TABLE app.t ALTER COLUMN id SET STORAGE EXTERNAL",
		"ALTER TABLE app.t ALTER COLUMN id SET STORAGE PLAIN, ALTER COLUMN id SET COMPRESSION pglz",
		"ALTER TABLE app.t ADD CONSTRAINT ck CHECK(id>0) NOT ENFORCED;ALTER TABLE app.t ALTER COLUMN id SET COMPRESSION pglz",
	} {
		c := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id", TypeStorage: "x"}}}
		p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: c}}
		r, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if (err != nil) != (len(r.Diagnostics) > 0) || len(r.Statements) == 0 {
			t.Fatal(r, err)
		}
		last := r.Statements[len(r.Statements)-1]
		if len(last.EvidenceGaps) == 0 || p.snapshot.PGColumns != c {
			t.Fatal("old type proof reused or provider mutated", r)
		}
		for _, f := range last.Findings {
			if f.RuleID == "ddl.pg.alter.column.storage.valid" || f.RuleID == "ddl.pg.alter.column.compression.valid" {
				t.Fatal("fabricated prospective type", f)
			}
		}
		if len(r.Diagnostics) > 0 && r.Coverage.Status != "incomplete" {
			t.Fatal("parser error promoted", r)
		}
	}
}
