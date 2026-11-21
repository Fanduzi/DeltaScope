//go:build postgresql

package postgresql

import "testing"

func TestPGCreateExpressionRefusalsUseAST(t *testing.T) {
	for _, tc := range []struct{ sql, reason string }{
		{`CREATE TABLE app.t (id int, v int DEFAULT id)`, "create_default_disallowed_reference"},
		{`CREATE TABLE app.t (id int DEFAULT (SELECT 1))`, "create_default_disallowed_reference"},
		{`CREATE TABLE app.t (id int, CHECK (id > (SELECT 1)))`, "create_check_subquery"},
		{`CREATE TABLE app.t (id int GENERATED ALWAYS AS ((SELECT 1)) STORED)`, "create_generated_subquery"},
		{`CREATE TABLE app.t (id int GENERATED ALWAYS AS (id) STORED)`, "create_generated_references_generated"},
		{`CREATE TABLE app.t (id int DEFAULT (ROW(1, true)::app.pair).x)`, ""},
		{`CREATE TABLE app.t (id int DEFAULT (ARRAY[1, 2])[1])`, ""},
		{`CREATE TABLE app.t (id int, a int GENERATED ALWAYS AS (id + 1) STORED)`, ""},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, tc.sql)
			if s.DDL == nil {
				t.Fatalf("DDL missing: %+v", s)
			}
			var reason string
			if refusal := s.DDL.PGCreateExpression; refusal != nil {
				reason = refusal.Reason
			}
			if reason != tc.reason {
				t.Fatalf("refusal = %q, want %q", reason, tc.reason)
			}
		})
	}
}
