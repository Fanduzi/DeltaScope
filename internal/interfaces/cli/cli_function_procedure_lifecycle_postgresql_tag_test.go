//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLFunctionProcedureLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_function_notice",
			sql:         "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice"},
		},
		{
			name:        "create_function_security_definer_warn",
			sql:         "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice", "ddl.pg.create_function.security_definer.warn"},
		},
		{
			name:        "create_or_replace_function_advisory",
			sql:         "CREATE OR REPLACE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice", "ddl.pg.create_or_replace_function.advisory"},
		},
		{
			name:        "drop_function_advisory",
			sql:         "DROP FUNCTION log_change();",
			wantRuleIDs: []string{"ddl.pg.drop_function.advisory"},
		},
		{
			name:        "create_procedure_notice",
			sql:         "CREATE PROCEDURE reset_counter() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_procedure.notice"},
		},
		{
			name:        "drop_procedure_advisory",
			sql:         "DROP PROCEDURE reset_counter();",
			wantRuleIDs: []string{"ddl.pg.drop_procedure.advisory"},
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
