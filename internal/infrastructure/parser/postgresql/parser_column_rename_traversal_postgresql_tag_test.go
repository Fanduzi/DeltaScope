//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnRenameOnlyTraversal(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		only bool
	}{
		{sql: `ALTER TABLE ONLY " Schema "."t" RENAME COLUMN " Old " TO " New "`, only: true},
		{sql: `ALTER TABLE " Schema "."t" RENAME COLUMN " Old " TO " New "`},
		{sql: `ALTER TABLE " Schema "."t" * RENAME COLUMN " Old " TO " New "`},
	} {
		s := extractPostgreSQLStatement(t, tc.sql)
		if s.DDL.Operation != spec.DDLOperationAlterTable || s.DDL.PGTableOnly != tc.only || s.DDL.Table.Schema != " Schema " || s.DDL.Alter[0].Name != " Old " {
			t.Fatalf("traversal=%+v", s.DDL)
		}
		encoded, err := json.Marshal(s.DDL)
		if err != nil || strings.Contains(string(encoded), "PGTableOnly") {
			t.Fatalf("traversal metadata appeared in JSON: %s (%v)", encoded, err)
		}
	}
}
