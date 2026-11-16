//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditPGGenerationRetentionAndPrivacy(t *testing.T) {
	for _, partial := range []bool{false, true} {
		sql := "ALTER TABLE app.t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY (CYCLE);"
		if partial {
			sql += "ALTER TABLE t ALTER CONSTRAINT ck NOT ENFORCED;"
		}
		r, err := Audit(context.Background(), Request{SQL: sql, Dialect: DialectPostgreSQL})
		if (err != nil) != partial || r.Verdict != "review" {
			t.Fatalf("partial=%t result=%+v err=%v", partial, r, err)
		}
		found := false
		for _, s := range r.Statements {
			for _, f := range s.Findings {
				if f.RuleID == "ddl.pg.column.identity.cycle.warn" {
					found = true
					if f.Metadata["schema"] != "app" {
						t.Fatalf("metadata=%+v", f)
					}
				}
			}
		}
		if !found || (partial && len(r.Diagnostics) == 0) {
			t.Fatal("lost warning or independent parser-error evidence")
		}
	}
	r, err := Audit(context.Background(), Request{SQL: "CREATE TABLE t(secret int DEFAULT 987654321 GENERATED ALWAYS AS (123456789) STORED)", Dialect: DialectPostgreSQL})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Statements[0].Findings {
		if f.RuleID == "ddl.pg.column.generation.declaration.valid" {
			found = true
			raw, _ := json.Marshal(f)
			for _, marker := range []string{"987654321", "123456789", "identity_options"} {
				if strings.Contains(string(raw), marker) {
					t.Fatalf("new finding leaked payload: %s", raw)
				}
			}
		}
	}
	if !found {
		t.Fatal("missing declaration finding")
	}
}
