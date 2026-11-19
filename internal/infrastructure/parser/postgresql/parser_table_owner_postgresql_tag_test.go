//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableOwnerRoleSpec(t *testing.T) {
	for _, tc := range []struct{ clause, kind, name string }{
		{`" Exact Owner "`, "named", " Exact Owner "},
		{`"current_user"`, "named", "current_user"},
		{`CURRENT_USER`, "current_user", ""},
		{`CURRENT_ROLE`, "current_role", ""},
		{`SESSION_USER`, "session_user", ""},
		{`PUBLIC`, "public", ""},
	} {
		t.Run(tc.clause, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, `ALTER TABLE app.t OWNER TO `+tc.clause)
			want := spec.PGTableOwnerTarget{Kind: tc.kind, Name: tc.name}
			if got := spec.PGTableOwnerSelection(s); got != want {
				t.Fatalf("selection=%+v want=%+v", got, want)
			}
			b, err := json.Marshal(s.DDL.Alter[0])
			if err != nil || strings.Contains(string(b), "PGTableOwner") || tc.name != "" && strings.Contains(string(b), tc.name) {
				t.Fatalf("private selection leaked: %s %v", b, err)
			}
		})
	}
}
