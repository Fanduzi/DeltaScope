//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLTablePrivilegeDCLRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "grant_select_notice",
			sql:         "GRANT SELECT ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice"},
		},
		{
			name:        "grant_all_duplicate",
			sql:         "GRANT ALL PRIVILEGES ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice", "ddl.pg.grant.table_privilege.all.warn"},
		},
		{
			name:        "revoke_all_cascade_duplicate",
			sql:         "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE;",
			wantRuleIDs: []string{"ddl.pg.revoke.table_privilege.notice", "ddl.pg.revoke.table_privilege.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, findings)
				}
			}
		})
	}
}
