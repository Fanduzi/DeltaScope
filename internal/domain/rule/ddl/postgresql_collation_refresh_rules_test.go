package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCollationRefreshAssessment(t *testing.T) {
	target := spec.PGColumnCollation{Schema: " Exact Schema ", Name: " Exact Collation "}
	for _, tt := range []struct {
		name, reason, missing string
		state                 spec.PGCollationRefreshState
	}{
		{name: "catalog absence", state: spec.PGCollationRefreshState{Known: true}, reason: "collation_refresh_target_absent"},
		{name: "database default", state: spec.PGCollationRefreshState{Known: true, Exists: true, Default: true}, reason: "collation_refresh_database_default"},
		{name: "provider version unavailable", state: spec.PGCollationRefreshState{Known: true, Exists: true}, missing: "collation.provider_version"},
		{name: "encoding identity unknown", state: spec.PGCollationRefreshState{Exists: true}, missing: "collation.encoding_identity"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.state.Target = target
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterCollation, PGCollationRefresh: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCollationRefresh: &tt.state},
			}
			if reason, fact := assessPGCollationRefresh(s); reason != tt.reason || fact != tt.missing {
				t.Fatalf("got (%q,%q), want (%q,%q)", reason, fact, tt.reason, tt.missing)
			}
		})
	}
}
