//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditPGColumnGenerationContract(t *testing.T) {
	v := func(n int64) *int64 { return &n }
	cycle := false
	identity := PGColumnState{Name: "id", Identity: "d", TypeWidth: 64, NotNull: true, IdentitySequence: &PGIdentitySequenceState{TypeWidth: 16, Definition: SequenceDefinition{Start: v(1), Increment: v(1), MinValue: v(1), MaxValue: v(10), CacheValue: v(1), Cycle: &cycle}}}
	for _, tc := range []struct {
		sql, id, verdict string
		member           PGColumnState
		unknown          bool
	}{
		{"ALTER TABLE app.t ALTER COLUMN id DROP NOT NULL", "ddl.pg.alter.column.generation.transition.valid", "reject", PGColumnState{Name: "id", TypeWidth: 32, NotNull: true, ReplicaIdentityKeyKnown: true, ReplicaIdentityKey: true}, false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY", "ddl.pg.alter.column.generation.transition.valid", "reject", PGColumnState{Name: "id", Identity: "d", TypeWidth: 64, NotNull: true}, false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY(CACHE 0)", "ddl.pg.alter.identity.sequence_options.valid", "reject", PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY(CYCLE)", "ddl.pg.alter.identity.cycle.warn", "review", PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"ALTER TABLE app.t ALTER COLUMN id RESTART WITH 5", "ddl.pg.alter.identity.restart.warn", "review", identity, false},
		{"ALTER TABLE app.t ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY", "", "review", PGColumnState{}, true},
	} {
		catalog := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{tc.member}}
		if tc.unknown {
			catalog = nil
		}
		p := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGColumns: catalog, PGRelation: PGRelationState{Name: "t", Kind: "r"}}}
		r, err := Audit(context.Background(), Request{SQL: tc.sql, Dialect: DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if err != nil || string(r.Verdict) != tc.verdict {
			t.Fatalf("result=%+v %v", r, err)
		}
		found := false
		for _, f := range r.Statements[0].Findings {
			if f.RuleID == tc.id {
				found = true
			}
			if strings.HasPrefix(f.RuleID, "ddl.pg.alter.identity.") || f.RuleID == "ddl.pg.alter.column.generation.transition.valid" {
				b, _ := json.Marshal(f)
				if strings.Contains(string(b), "IdentitySequence") || strings.Contains(string(b), "PGColumns") {
					t.Fatal("private facts leaked")
				}
			}
		}
		if found != (tc.id != "") || tc.unknown && (r.Coverage.Status != "unverified" || len(r.Statements[0].EvidenceGaps) == 0) {
			t.Fatalf("result=%+v", r)
		}
	}
}
