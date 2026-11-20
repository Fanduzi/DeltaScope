//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestConstraintUsingIndexExactName(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD CONSTRAINT checked UNIQUE USING INDEX " Exact.Index "`)
	if got := spec.PGConstraintUsingIndexTarget(s); got != " Exact.Index " {
		t.Fatalf("index selection = %q", got)
	}
	b, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(b), "Exact.Index") || strings.Contains(string(b), "PGUsingIndexName") {
		t.Fatalf("private selection leaked: %s %v", b, err)
	}
}

func TestAlterConstraintBuildSeam(t *testing.T) {
	for _, tc := range []struct {
		sql, kind, using string
	}{
		{"ALTER TABLE app.t ADD CONSTRAINT pk PRIMARY KEY(id)", "primary_key", "false"},
		{"ALTER TABLE app.t ADD PRIMARY KEY(id)", "primary_key", "false"},
		{"ALTER TABLE app.t ADD CONSTRAINT pk PRIMARY KEY USING INDEX pk_idx", "primary_key", "true"},
		{"ALTER TABLE app.t ADD PRIMARY KEY USING INDEX \"QuotedIdx\"", "primary_key", "true"},
		{"ALTER TABLE app.t ADD CONSTRAINT uq UNIQUE(id)", "unique", "false"},
		{"ALTER TABLE app.t ADD CONSTRAINT uq UNIQUE USING INDEX uq_idx", "unique", "true"},
		{"ALTER TABLE app.t ADD FOREIGN KEY(id) REFERENCES parent(id)", "foreign_key", ""},
		{"ALTER TABLE app.t ADD CHECK(id>0)", "check", ""},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, tc.sql)
			if s.DDL == nil || len(s.DDL.Alter) != 1 {
				t.Fatalf("missing alter: %+v", s)
			}
			a := s.DDL.Alter[0]
			if a.Action != "add_constraint" || a.Options["constraint_type"] != tc.kind || a.Options["using_index"] != tc.using {
				t.Fatalf("facts: %+v", a)
			}
			if _, present := a.Options["using_index"]; present != (tc.using != "") {
				t.Fatalf("presence lost: %+v", a.Options)
			}
		})
	}
}
