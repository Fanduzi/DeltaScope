//go:build postgresql

package postgresql

import "testing"

func TestDropConstraintTargetDeclaration(t *testing.T) {
	for _, tc := range []struct{ sql, guard, cascade string }{
		{"ALTER TABLE app.t DROP CONSTRAINT ck", "false", "false"},
		{"ALTER TABLE app.t DROP CONSTRAINT ck RESTRICT", "false", "false"},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS ck", "true", "false"},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS \"CK\" CASCADE", "true", "true"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, tc.sql)
			if s.DDL == nil || len(s.DDL.Alter) != 1 {
				t.Fatalf("missing alter: %+v", s)
			}
			a := s.DDL.Alter[0]
			if a.Action != "drop_constraint" || a.Options["if_exists"] != tc.guard || a.Options["cascade"] != tc.cascade {
				t.Fatalf("alter=%+v", a)
			}
			if tc.cascade == "true" && a.Name != "CK" {
				t.Fatalf("quoted name=%q", a.Name)
			}
		})
	}
}
