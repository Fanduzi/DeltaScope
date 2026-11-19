//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGBulkTablespaceRoleSpecs(t *testing.T) {
	s := extractStmt(t, `ALTER TABLE ALL IN TABLESPACE " Source " OWNED BY " Alias ", CURRENT_USER, CURRENT_ROLE, SESSION_USER, PUBLIC, "session_user", " Alias " SET TABLESPACE " Target " NOWAIT`)
	want := spec.PGBulkTablespaceTarget{Source: " Source ", Destination: " Target ", Nowait: true, Roles: []spec.PGTableOwnerTarget{
		{Kind: "named", Name: " Alias "}, {Kind: "current_user"}, {Kind: "current_role"}, {Kind: "session_user"},
		{Kind: "public"}, {Kind: "named", Name: "session_user"}, {Kind: "named", Name: " Alias "},
	}}
	if got := spec.PGBulkTablespaceSelection(s); got == nil || !got.Matches(want) {
		t.Fatalf("bulk selection = %+v", got)
	}
	b, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(b), " Alias ") || strings.Contains(string(b), "Nowait") || strings.Contains(string(b), "PGBulkTablespace") {
		t.Fatalf("private selection leaked: %s, %v", b, err)
	}
	plain := extractStmt(t, `ALTER TABLE ALL IN TABLESPACE " Source " SET TABLESPACE " Target "`)
	got := spec.PGBulkTablespaceSelection(plain)
	if got == nil || got.Nowait || len(got.Roles) != 0 {
		t.Fatalf("unfiltered bulk selection = %+v", got)
	}
}
