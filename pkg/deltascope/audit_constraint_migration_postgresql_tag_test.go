//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditPGConstraintMigration(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(cfg, []byte("rules:\n  ddl.table.foreign_key.forbid:\n    enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ sql, id, verdict string }{
		{"ALTER TABLE app.t ADD PRIMARY KEY(id)", "ddl.pg.alter.add_primary_key.concurrent_index.warn", "review"},
		{"ALTER TABLE app.t ADD CONSTRAINT pk PRIMARY KEY USING INDEX private_index", "", "review"},
		{"ALTER TABLE app.t ADD CONSTRAINT uq UNIQUE USING INDEX private_index", "", "review"},
		{"ALTER TABLE app.t ADD CONSTRAINT fk FOREIGN KEY(id) REFERENCES private_parent(private_column)", "ddl.pg.alter.add_foreign_key.not_valid.require", "review"},
	} {
		r, err := Audit(context.Background(), Request{SQL: tc.sql, Dialect: DialectPostgreSQL, ConfigPath: cfg})
		if err != nil || string(r.Verdict) != tc.verdict {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		found := false
		for _, s := range r.Statements {
			if tc.id == "" {
				if len(s.Findings) != 0 || len(s.EvidenceGaps) != 1 {
					t.Fatalf("unexpected reuse result: %+v", s)
				}
				gap := s.EvidenceGaps[0]
				if gap.RuleID != "ddl.pg.alter.constraint_using_index.valid" || gap.ReasonCode != "unproven_constraint_using_index" || len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "instance.version" {
					t.Fatalf("unexpected attachment gap: %+v", gap)
				}
			}
			for _, f := range s.Findings {
				if f.RuleID != tc.id {
					continue
				}
				found = true
				if f.Level != "warning" || f.Metadata["schema"] != "app" {
					t.Fatalf("finding=%+v", f)
				}
				raw, err := json.Marshal(f)
				if err != nil {
					t.Fatal(err)
				}
				for _, marker := range []string{"private_parent", "private_column", "private_index"} {
					if strings.Contains(string(raw), marker) {
						t.Fatalf("finding leaked unrelated payload: %s", raw)
					}
				}
			}
		}
		if found != (tc.id != "") {
			t.Fatalf("missing warning %q", tc.id)
		}
	}
	r, err := Audit(context.Background(), Request{SQL: "ALTER TABLE t ADD PRIMARY KEY(id);ALTER TABLE t ALTER CONSTRAINT c NOT ENFORCED", Dialect: DialectPostgreSQL})
	if err == nil || len(r.Diagnostics) == 0 || len(r.Statements) == 0 || len(r.Statements[0].Findings) != 1 || r.Statements[0].Findings[0].RuleID != "ddl.pg.alter.add_primary_key.concurrent_index.warn" {
		t.Fatalf("lost partial-result warning: %+v %v", r, err)
	}
}
