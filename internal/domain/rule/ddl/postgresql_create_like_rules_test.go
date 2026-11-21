package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateLikeAssessment(t *testing.T) {
	dest := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "copy"}}
	source := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "src", Name: " Exact "}}
	for _, tc := range []struct {
		name, kind, reason, gap  string
		exists, guard, duplicate bool
		options                  uint32
	}{
		{name: "guard skips missing source", exists: true, guard: true},
		{name: "missing source before duplicate target", exists: true, reason: "create_like_source_absent"},
		{name: "sequence source", kind: "S", reason: "create_like_source_wrong_kind"},
		{name: "valid view source", kind: "v"},
		{name: "duplicate column remains a refusal", kind: "r", duplicate: true, reason: "create_like_column_duplicate"},
		{name: "option needs more evidence", kind: "r", options: 8, gap: "create_like.copied_options"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Table: &dest.Table,
					PGCreateLike: []spec.PGCreateLikeSource{{Target: source, Options: tc.options}}, Options: map[string]string{}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateLike: &spec.PGCreateSources{Target: dest,
					SchemaExists: true, Exists: tc.exists, Sources: []spec.PGCreateSource{{Target: source, Kind: tc.kind, Columns: []string{" Exact "}}}}},
			}
			if tc.guard {
				s.DDL.Options["if_not_exists"] = "true"
			}
			if tc.duplicate {
				s.DDL.Columns = []spec.Column{{Name: " Exact "}}
			}
			if reason, gap := assessPGCreateLike(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment=(%q,%q), want (%q,%q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
