//go:build postgresql

package postgresql

import (
	"slices"
	"testing"
)

func TestPGEnumLabelPresence(t *testing.T) {
	created := extractPostgreSQLStatement(t, `CREATE TYPE app." Mood " AS ENUM (' a, b ', '', '')`)
	if created.DDL == nil || created.DDL.PGCreateEnum == nil ||
		created.DDL.PGCreateEnum.Target.Table.Name != " Mood " ||
		!slices.Equal(created.DDL.PGCreateEnum.Labels, []string{" a, b ", "", ""}) {
		t.Fatalf("CREATE ENUM facts: %+v", created.DDL)
	}
	for _, tc := range []struct {
		sql, action, placement string
		old, neighbor          bool
	}{
		{`ALTER TYPE app.t ADD VALUE ''`, "add_value", "", false, false},
		{`ALTER TYPE app.t ADD VALUE 'new' BEFORE ''`, "add_value", "before", false, true},
		{`ALTER TYPE app.t ADD VALUE 'new' AFTER ''`, "add_value", "after", false, true},
		{`ALTER TYPE app.t RENAME VALUE '' TO ''`, "rename_value", "", true, false},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		if s.DDL == nil || s.DDL.PGAlterEnum == nil {
			t.Fatalf("%s: ALTER ENUM facts missing", tc.sql)
		}
		decl := s.DDL.PGAlterEnum
		if !decl.Known || (decl.Old != nil) != tc.old || (decl.Neighbor != nil) != tc.neighbor ||
			decl.Old != nil && *decl.Old != "" || decl.Neighbor != nil && *decl.Neighbor != "" ||
			s.DDL.Options["action"] != tc.action || s.DDL.Options["placement"] != tc.placement {
			t.Fatalf("%s: unexpected label facts %+v and options %+v", tc.sql, decl, s.DDL.Options)
		}
	}
}
