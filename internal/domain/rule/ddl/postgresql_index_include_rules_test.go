package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGIndexIncludeAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "t"}}
	for _, tc := range []struct {
		name, kind, method, reason, fact string
		included                         []string
	}{
		{name: "repeated exact output and key overlap", kind: "r", method: "btree", included: []string{" Exact ", " Exact "}},
		{name: "quoted spelling differs", kind: "r", method: "btree", included: []string{" exact "}, reason: "index_include_column_absent"},
		{name: "system attribute", kind: "r", method: "btree", included: []string{"ctid"}, reason: "index_include_system_column"},
		{name: "hash method", kind: "r", method: "hash", included: []string{"missing"}, reason: "index_include_method_unsupported"},
		{name: "partition premises", kind: "p", method: "btree", included: []string{" Exact "}, fact: "index.partition_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateIndex, Table: &target.Table,
					Indexes: []spec.Index{{AccessMethod: tc.method, Columns: []string{" Exact "}, IncludedColumns: tc.included}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					PGIndexInclude: &spec.PGIndexIncludeState{Target: target, Kind: tc.kind, Method: tc.method, Columns: []string{" Exact "}, System: []string{"ctid"}}},
			}
			if reason, fact := assessPGIndexInclude(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment=(%q,%q), want (%q,%q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
