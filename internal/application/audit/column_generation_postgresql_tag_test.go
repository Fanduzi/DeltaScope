//go:build postgresql

package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnGenerationPipeline(t *testing.T) {
	cycle := false
	v := func(n int64) *int64 { return &n }
	plain := spec.PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}
	identity := plain
	identity.Identity = "d"
	identity.IdentitySequence = &spec.PGIdentitySequenceState{TypeWidth: 16, Definition: spec.SequenceDefinition{Start: v(1), Increment: v(1), MinValue: v(1), MaxValue: v(10), CacheValue: v(1), Cycle: &cycle}}
	for _, tc := range []struct {
		sql         string
		member      spec.PGColumnState
		id, verdict string
		gap         bool
	}{
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY", plain, "", "pass", false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY(CYCLE)", plain, "ddl.pg.alter.identity.cycle.warn", "review", false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY", identity, "ddl.pg.alter.column.generation.transition.valid", "reject", false},
		{"ALTER TABLE app.t ALTER COLUMN id SET CACHE 0", identity, "ddl.pg.alter.identity.sequence_options.valid", "reject", false},
		{"ALTER TABLE app.t ALTER COLUMN id RESTART WITH 5", identity, "ddl.pg.alter.identity.restart.warn", "review", false},
		{"ALTER TABLE app.t ALTER COLUMN id SET START WITH 5", identity, "", "review", false},
		{"ALTER TABLE app.t ALTER COLUMN id SET NO CYCLE", identity, "", "review", false},
		{"ALTER TABLE app.t ALTER COLUMN id DROP IDENTITY IF EXISTS", plain, "", "review", false},
		{"ALTER TABLE app.t ALTER COLUMN id SET MAXVALUE 5", identity, "", "review", true},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{tc.member}}
			p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog}}
			before, _ := json.Marshal(catalog)
			r, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
			if err != nil || string(r.Verdict) != tc.verdict {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			found := false
			for _, f := range r.Statements[0].Findings {
				if f.RuleID == tc.id {
					found = true
				}
				if strings.HasPrefix(f.RuleID, "ddl.pg.alter.identity.") || f.RuleID == "ddl.pg.alter.column.generation.transition.valid" {
					if f.RuleID != tc.id {
						t.Fatalf("unexpected D5 finding %+v", f)
					}
				}
			}
			if found != (tc.id != "") || (len(r.Statements[0].EvidenceGaps) > 0) != tc.gap {
				t.Fatalf("result=%+v", r)
			}
			after, _ := json.Marshal(catalog)
			if !bytes.Equal(before, after) {
				t.Fatal("provider state changed")
			}
		})
	}
	p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{identity}}}}
	r, err := AuditSQL(context.Background(), Request{SQL: "ALTER TABLE app.t ALTER COLUMN id DROP IDENTITY;ALTER TABLE app.t ALTER COLUMN id SET GENERATED ALWAYS", Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
	if err != nil || len(r.Statements) != 2 || r.Coverage.Status != "unverified" || len(r.Statements[1].EvidenceGaps) == 0 || p.snapshot.PGColumns == nil {
		t.Fatalf("freshness: %+v %v", r, err)
	}
}

func TestCorpusPGColumnFactsDoNotGuessMissingValues(t *testing.T) {
	yes, no := true, false
	empty, id := "", "d"
	width := 64
	valid := corpusPGColumnState{Name: "id", Identity: &id, Generated: &empty, TypeWidth: &width, NotNull: &yes, HasDefault: &no, PrimaryKey: &no}
	for _, tc := range []struct {
		cat   *corpusPGColumnCatalog
		known bool
	}{
		{nil, false}, {&corpusPGColumnCatalog{Ordinary: &yes}, false},
		{&corpusPGColumnCatalog{Ordinary: &yes, Columns: []corpusPGColumnState{}}, true},
		{&corpusPGColumnCatalog{Ordinary: &yes, Columns: []corpusPGColumnState{valid}}, true},
		{&corpusPGColumnCatalog{Ordinary: &yes, Columns: []corpusPGColumnState{{Name: "id", Identity: &id}}}, false},
	} {
		s := corpusTableSnapshotToSpec("t", "app", corpusTableSnapshot{Exists: true, PGColumns: tc.cat})
		_, known := s.PGColumns.FindColumn("id")
		if known != tc.known {
			t.Fatalf("catalog=%+v", s.PGColumns)
		}
	}
}
