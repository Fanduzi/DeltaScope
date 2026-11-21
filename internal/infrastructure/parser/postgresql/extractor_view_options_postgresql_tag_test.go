//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewOptionsPreserveAuthoredOrder(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE VIEW app.v WITH (security_barrier, security_invoker=' true ', security_invoker='of', check_option='LOCAL') AS SELECT 1`)
	want := []spec.PGViewOption{
		{Name: "security_barrier", ValueKnown: true, ValueValid: true},
		{Name: "security_invoker", ValueKnown: true},
		{Name: "security_invoker", ValueKnown: true, ValueValid: true},
		{Name: "check_option", ValueKnown: true, ValueValid: true},
	}
	if got := s.DDL.PGViewOptions; len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("member %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	}
	data, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(data), "PGViewOptions") || strings.Contains(string(data), "security_invoker") {
		t.Fatalf("option facts appear in public DDL: %s (%v)", data, err)
	}
}
