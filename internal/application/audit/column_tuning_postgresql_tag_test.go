//go:build postgresql

package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTuningPipeline(t *testing.T) {
	for _, tc := range []struct {
		sql, id, verdict string
		unknown, gap     bool
	}{
		{"ALTER COLUMN id SET STATISTICS -2", "ddl.pg.alter.column.statistics.target.valid", "reject", false, false},
		{"ALTER COLUMN id SET STATISTICS 10001", "ddl.pg.alter.column.statistics.clamp.warn", "review", false, false},
		{"ALTER COLUMN id SET STATISTICS DEFAULT", "", "pass", false, false},
		{"ALTER COLUMN id SET STATISTICS 0", "", "pass", false, false},
		{"ALTER COLUMN id SET(n_distinct=-1.1)", "ddl.pg.alter.column.options.valid", "reject", false, false},
		{"ALTER COLUMN id SET(n_distinct=1,n_distinct=2)", "ddl.pg.alter.column.options.valid", "reject", false, false},
		{"ALTER COLUMN id SET(n_distinct=-0.5)", "", "pass", false, false},
		{"ALTER COLUMN id SET(n_distinct=1e-999)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct=-1e-999)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct=1e-310)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct=4.9406564584124654e-324)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct=2.2250738585072014e-308)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct=0.0)", "", "pass", false, false},
		{"ALTER COLUMN id SET(n_distinct=0e-999)", "", "pass", false, false},
		{"ALTER COLUMN id SET(n_distinct=1e-300)", "", "pass", false, false},
		{"ALTER COLUMN id SET(private_unknown_key=1)", "", "review", false, true},
		{"ALTER COLUMN id SET(n_distinct='private_quoted_value')", "", "review", false, true},
		{"ALTER COLUMN id RESET(private_unknown_key)", "", "pass", false, false},
		{"DROP COLUMN missing", "ddl.alter.drop_column.exists.require", "reject", false, false},
		{"DROP COLUMN IF EXISTS missing", "", "pass", false, false},
		{"DROP COLUMN IF EXISTS id", "ddl.pg.alter.drop_column.advisory", "review", false, true},
		{`DROP COLUMN "ID"`, "ddl.alter.drop_column.exists.require", "reject", false, false},
		{"DROP COLUMN IF EXISTS missing", "ddl.pg.alter.drop_column.advisory", "review", true, true},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}
			if tc.unknown {
				catalog = nil
			}
			p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog}}
			r, err := AuditSQL(context.Background(), Request{SQL: "ALTER TABLE app.t " + tc.sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
			if err != nil || string(r.Verdict) != tc.verdict || len(r.Statements) != 1 {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			found := false
			for _, f := range r.Statements[0].Findings {
				if f.RuleID == tc.id {
					found = true
				}
				if strings.HasPrefix(f.RuleID, "ddl.pg.alter.column.statistics.") || f.RuleID == "ddl.pg.alter.column.options.valid" {
					if f.RuleID != tc.id {
						t.Fatalf("unexpected D6 finding %+v", f)
					}
				}
				raw, _ := json.Marshal(f)
				if strings.Contains(string(raw), "private_unknown_key") || strings.Contains(string(raw), "private_quoted_value") {
					t.Fatal("declaration payload leaked")
				}
			}
			if found != (tc.id != "") || (len(r.Statements[0].EvidenceGaps) > 0) != tc.gap {
				t.Fatalf("result=%+v", r)
			}
		})
	}
}

func TestPGColumnTuningPriorDDLAndParser(t *testing.T) {
	for _, tc := range []struct {
		sql         string
		diagnostics int
	}{
		{"ALTER TABLE app.t DROP COLUMN id;ALTER TABLE app.t DROP COLUMN IF EXISTS id", 0},
		{"ALTER TABLE app.t ADD COLUMN other_id int;ALTER TABLE app.t ALTER COLUMN id SET STATISTICS 10", 0},
		{"ALTER TABLE app.t DROP COLUMN id, DROP COLUMN IF EXISTS id", 0},
		{"ALTER TABLE app.t ADD CONSTRAINT ck CHECK(id>0) NOT ENFORCED;ALTER TABLE app.t ALTER COLUMN id SET STATISTICS 10", 1},
	} {
		catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}
		p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog}}
		r, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if (err != nil) != (tc.diagnostics > 0) || len(r.Diagnostics) != tc.diagnostics {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		last := r.Statements[len(r.Statements)-1]
		if len(last.EvidenceGaps) == 0 || p.snapshot.PGColumns != catalog {
			t.Fatalf("stale facts guessed or provider mutated: %+v", r)
		}
		for _, f := range last.Findings {
			if f.RuleID == "ddl.alter.drop_column.exists.require" || f.RuleID == "ddl.pg.alter.column.statistics.target.valid" {
				t.Fatalf("unapplied post-state fabricated: %+v", f)
			}
		}
		if tc.diagnostics == 1 && (r.Coverage.Status != "incomplete" || r.Diagnostics[0].Classification != DiagnosticParserError || r.Diagnostics[0].Audited) {
			t.Fatal("parser failure promoted", r.Coverage, r.Diagnostics)
		}
	}
}
