package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGRangeAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "r"}}
	subtype := spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}
	for _, tc := range []struct {
		options          []string
		occupied, absent bool
		unknown, pseudo  bool
		reason, fact     string
	}{
		{options: []string{"unknown"}, occupied: true, reason: "range_name_occupied"},
		{options: []string{"unknown"}, absent: true, reason: "range_schema_absent"},
		{options: []string{"other"}, reason: "range_option_unknown"},
		{options: []string{"collation"}, reason: "range_subtype_required"},
		{options: []string{"subtype", "subtype"}, reason: "range_option_duplicate"},
		{options: []string{"subtype"}, unknown: true, fact: "create_type.range_subtype"},
		{options: []string{"subtype"}, pseudo: true, reason: "range_subtype_pseudo"},
		{options: []string{"subtype", "collation"}, reason: "range_subtype_not_collatable"},
		{options: []string{"subtype", "canonical"}, reason: "range_canonical_requires_shell"},
		{options: []string{"subtype"}, fact: "create_type.range_opclass"},
	} {
		var live *spec.PGColumnTypeState
		if !tc.unknown {
			live = &spec.PGColumnTypeState{Target: subtype, SchemaExists: true, Resolved: true, Defined: true, Pseudo: tc.pseudo}
		}
		s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateType, PGRange: &spec.PGRangeDeclaration{Target: target, Subtype: subtype, Options: tc.options}},
			Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGRangeSubtype: live,
				PGEnum: &spec.PGEnumState{Target: target, SchemaExists: !tc.absent, NameOccupied: tc.occupied}}}
		if reason, fact := assessPGRange(s); reason != tc.reason || fact != tc.fact {
			t.Fatalf("%+v: assessment = (%q, %q)", tc, reason, fact)
		}
	}
}
