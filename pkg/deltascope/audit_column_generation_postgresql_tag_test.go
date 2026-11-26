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
		{"ALTER TABLE app.t ALTER COLUMN id SET DEFAULT (SELECT 2)", "ddl.pg.alter.column.generation.transition.valid", "reject", PGColumnState{Name: "id"}, false},
		{"ALTER TABLE app.t ALTER COLUMN id SET DEFAULT 1", "ddl.pg.alter.column.generation.transition.valid", "reject", PGColumnState{Name: "id", TypeWidth: -1, BuiltinBool: true}, false},
		{"ALTER TABLE app.t ALTER COLUMN id SET DEFAULT $1", "ddl.pg.alter.column.generation.transition.valid", "reject", PGColumnState{Name: "id"}, false},
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
				if tc.sql == "ALTER TABLE app.t ALTER COLUMN id SET DEFAULT $1" && f.Metadata["reason_code"] != "default_parameter_inapplicable" {
					t.Fatalf("finding=%+v", f)
				}
				if tc.member.BuiltinBool && f.Metadata["reason_code"] != "default_literal_type_mismatch" {
					t.Fatalf("finding=%+v", f)
				}
			}
			if strings.HasPrefix(f.RuleID, "ddl.pg.alter.identity.") || f.RuleID == "ddl.pg.alter.column.generation.transition.valid" {
				b, _ := json.Marshal(f)
				if strings.Contains(string(b), "IdentitySequence") || strings.Contains(string(b), "PGColumns") || strings.Contains(string(b), "BuiltinBool") || strings.Contains(string(b), "DefaultLiteralType") || strings.Contains(string(b), "DefaultHasParameter") || strings.Contains(string(b), "$1") {
					t.Fatal("private facts leaked")
				}
			}
		}
		if found != (tc.id != "") || tc.unknown && (r.Coverage.Status != "unverified" || len(r.Statements[0].EvidenceGaps) == 0) {
			t.Fatalf("result=%+v", r)
		}
	}
}

func TestAuditPGCreateViewOptions(t *testing.T) {
	provider := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}}
	result, err := Audit(context.Background(), Request{SQL: `CREATE VIEW app.v WITH (security_invoker=' true ') AS SELECT 1`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || len(result.Statements) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.create_view.options.valid" && finding.Metadata["reason_code"] == "view_option_invalid_value" {
			return
		}
	}
	t.Fatalf("missing view option finding: %+v", result.Statements[0].Findings)
}
