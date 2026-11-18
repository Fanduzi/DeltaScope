//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGAlterTableGuardFacts(t *testing.T) {
	for _, tc := range []struct {
		sql                     string
		tableGuard, columnGuard bool
	}{
		{`ALTER TABLE app.t ADD COLUMN c int`, false, false},
		{`ALTER TABLE app.t ADD COLUMN IF NOT EXISTS c int`, false, true},
		{`ALTER TABLE IF EXISTS app.t ADD COLUMN c int`, true, false},
		{`ALTER TABLE IF EXISTS app.t ADD COLUMN IF NOT EXISTS c int`, true, true},
		{`ALTER TABLE IF EXISTS app.t RENAME COLUMN " " TO " C "`, true, false},
		{`ALTER TABLE app.t RENAME COLUMN c TO d`, false, false},
		{`ALTER TABLE IF EXISTS app.t RENAME TO renamed_t`, true, false},
		{`ALTER TABLE IF EXISTS app.t SET SCHEMA dst`, true, false},
		{`ALTER TABLE IF EXISTS app.t ALTER COLUMN c TYPE bigint`, true, false},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, tc.sql)
			if s.DDL == nil || len(s.DDL.Alter) != 1 || s.DDL.PGTableIfExists != tc.tableGuard || s.DDL.Alter[0].PGAddColumnIfNotExists != tc.columnGuard {
				t.Fatalf("guard mismatch: %+v", s.DDL)
			}
			encoded, err := json.Marshal(s.DDL)
			if err != nil || strings.Contains(strings.ToLower(string(encoded)), "if_exists") || strings.Contains(strings.ToLower(string(encoded)), "ifnotexists") || strings.Contains(strings.ToLower(string(encoded)), "pgtableifexists") {
				t.Fatalf("guard leaked: %s, %v", encoded, err)
			}
		})
	}
	ddl := extractPostgreSQLStatement(t, `ALTER TABLE IF EXISTS app." t " RENAME COLUMN " " TO " C "`).DDL
	a := ddl.Alter[0]
	if ddl.Table.Name != " t " || a.Name != " " || a.Column.OldName != " " || a.Column.Definition.Name != " C " {
		t.Fatalf("decoded names changed: %+v, %+v", ddl.Table, a)
	}
	qualified := extractPostgreSQLStatement(t, `ALTER TABLE IF EXISTS db.app." " ADD COLUMN c int`).DDL
	if qualified == nil || qualified.Table == nil || qualified.Table.Name != " " || !qualified.PGTableCatalogQualified || !qualified.PGTableIfExists {
		t.Fatalf("qualification or exact name lost: %+v", qualified)
	}
	encoded, err := json.Marshal(qualified)
	if err != nil || strings.Contains(string(encoded), "PGTableCatalogQualified") {
		t.Fatalf("qualification leaked: %s %v", encoded, err)
	}
}
