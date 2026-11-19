package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGBulkTablespaceFreshBinding(t *testing.T) {
	for _, tc := range []struct {
		name      string
		prior     spec.Kind
		failure   bool
		mismatch  bool
		wantRead  bool
		wantFacts bool
	}{
		{name: "first", wantRead: true, wantFacts: true},
		{name: "prior DML", prior: spec.KindDML, wantRead: true, wantFacts: true},
		{name: "prior DDL", prior: spec.KindDDL},
		{name: "prior unknown", prior: spec.KindUnknown},
		{name: "prior parser failure", failure: true},
		{name: "mismatched role", mismatch: true, wantRead: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGBulkTablespaceTarget{Source: " source ", Destination: "target", Roles: []spec.PGTableOwnerTarget{{Kind: "named", Name: " owner "}}}
			metadata := &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}}
			statement := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, Line: 2, Column: 1, Metadata: metadata,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Alter: []spec.Alter{{Action: "move_all_tablespace", PGBulkTablespace: &target}}}}
			statements := []spec.Statement{statement}
			if tc.prior != "" {
				statements = []spec.Statement{{Kind: tc.prior, Line: 1, Column: 1}, statement}
			}
			var failures []parseFailure
			if tc.failure {
				failures = []parseFailure{{Line: 1, Column: 1}}
			}
			reads := 0
			roles := []bool{true}
			provider := replicaLookupProvider{lookup: func(req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
				reads++
				if req.Type != "bulk_tablespace" || req.PGBulkTablespace == nil || !target.Matches(*req.PGBulkTablespace) {
					t.Fatalf("unbound request: %+v", req)
				}
				if tc.mismatch {
					req.PGBulkTablespace.Roles[0].Name = "different"
				}
				return &spec.ObjectSnapshot{PGBulkTablespace: &spec.PGBulkTablespaceState{Target: *req.PGBulkTablespace, RoleExists: roles}}, nil
			}}
			end := invalidateStalePGTargetCatalogs(statements, failures)
			if err := enrichPGBulkTablespaces(context.Background(), &MetadataRequest{Provider: provider}, statements[:end]); err != nil {
				t.Fatal(err)
			}
			if (reads == 1) != tc.wantRead || (metadata.PGBulkTablespace != nil) != tc.wantFacts || target.Roles[0].Name != " owner " {
				t.Fatalf("reads=%d facts=%+v target=%+v", reads, metadata.PGBulkTablespace, target)
			}
			if tc.wantFacts {
				roles[0] = false
				if !metadata.PGBulkTablespace.RoleExists[0] {
					t.Fatal("statement facts alias provider memory")
				}
			}
		})
	}
}
