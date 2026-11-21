package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGMaterializedViewExtensionAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, extension string
		installed             bool
		reason, fact          string
	}{
		{name: "installed", kind: "m", extension: "Exact", installed: true},
		{name: "missing extension", kind: "m", extension: "Exact", reason: "materialized_view_extension_absent"},
		{name: "absent source first", extension: "Exact", reason: "materialized_view_extension_target_absent"},
		{name: "wrong source first", kind: "v", extension: "Exact", reason: "materialized_view_extension_target_wrong_kind"},
		{name: "case-distinct evidence", kind: "m", extension: "exact", installed: true, fact: "materialized_view.extension_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterMaterializedView, Table: &spec.Table{Schema: "app", Name: "mv"}, PGMaterializedViewExtension: "Exact"},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGMaterializedViewExtension: &spec.PGMaterializedViewExtensionState{
					Schema: "app", View: "mv", Extension: tc.extension, Kind: tc.kind, ExtensionExists: tc.installed}},
			}
			if reason, fact := assessPGMaterializedViewExtension(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment=(%q,%q), want (%q,%q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
