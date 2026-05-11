//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLAdvancedViewLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
		expectBlock bool
	}{
		{
			name:        "create_or_replace_view_advisory",
			sql:         "CREATE OR REPLACE VIEW active_users AS SELECT id FROM users WHERE active = true",
			wantRuleIDs: []string{"ddl.pg.create_or_replace_view.advisory"},
			expectBlock: true,
		},
		{
			name:        "create_temp_view_notice",
			sql:         "CREATE TEMP VIEW session_data AS SELECT id FROM temp_storage",
			wantRuleIDs: []string{"ddl.pg.create_temp_view.notice"},
			expectBlock: true,
		},
		{
			name:        "create_view_check_option_notice",
			sql:         "CREATE VIEW checked_view AS SELECT * FROM users WHERE active = true WITH CHECK OPTION",
			wantRuleIDs: []string{"ddl.pg.create_view.check_option.notice"},
			expectBlock: true,
		},
		{
			name:        "alter_view_rename_notice",
			sql:         "ALTER VIEW v_old RENAME TO v_new",
			wantRuleIDs: []string{"ddl.pg.alter_view.rename.notice"},
		},
		{
			name:        "alter_view_set_schema_notice",
			sql:         "ALTER VIEW v_stats SET SCHEMA archive",
			wantRuleIDs: []string{"ddl.pg.alter_view.set_schema.notice"},
		},
		{
			name:        "drop_view_cascade_warn",
			sql:         "DROP VIEW v_stats CASCADE",
			wantRuleIDs: []string{"ddl.pg.drop_view.cascade.warn"},
			expectBlock: true,
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
			if tt.expectBlock {
				if code != 1 {
					t.Fatalf("expected exit code 1 (blocker present), got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
				}
			} else {
				if code != 0 {
					t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
				}
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
