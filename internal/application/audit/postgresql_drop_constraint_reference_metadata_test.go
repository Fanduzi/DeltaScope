package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropConstraintReferenceMetadataBinding(t *testing.T) {
	snapshot := &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
		PGConstraints: []spec.PGConstraintState{{Name: " exact key ", Type: "u"}}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}}
	statement := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{
		Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
		Alter: []spec.Alter{{Action: "drop_constraint", Name: " exact key ", Options: map[string]string{"cascade": "false"}}},
	}}
	secretError := errors.New("provider detail")
	for _, tc := range []struct {
		name                string
		alter               func(*spec.Statement)
		wantCall, wantFacts bool
		mismatch            bool
		providerErr         error
	}{
		{name: "exact", wantCall: true, wantFacts: true},
		{name: "misbound response", wantCall: true, mismatch: true},
		{name: "failed read", wantCall: true, providerErr: secretError},
		{name: "unqualified", alter: func(s *spec.Statement) { s.DDL.Table = &spec.Table{Name: "t"} }},
		{name: "stale", alter: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{name: "typed", alter: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Typed = true }},
		{name: "PG18", alter: func(s *spec.Statement) { s.Metadata.Instance.Version = "PostgreSQL 18.1" }},
		{name: "multiple actions", alter: func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "add_column"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ddl := statement, *statement.DDL
			s.DDL = &ddl
			s.Metadata = &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, TargetTable: cloneTableSnapshot(snapshot)}
			if tc.alter != nil {
				tc.alter(&s)
			}
			facts := &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: " exact key ", Kind: "u", Blocked: true}
			if tc.mismatch {
				facts.Name = "EXACT KEY"
			}
			called := false
			provider := replicaLookupProvider{lookup: func(req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
				called = true
				if req.Type != "drop_constraint_reference" || req.Schema != "app" || req.Name != " exact key " || req.Qualifiers["table"] != "t" || req.Qualifiers["kind"] != "u" {
					t.Fatalf("request=%+v", req)
				}
				return &spec.ObjectSnapshot{PGDropConstraintReference: facts}, tc.providerErr
			}}
			err := enrichPGDropConstraintReferences(context.Background(), &MetadataRequest{Provider: provider}, []spec.Statement{s})
			if !errors.Is(err, tc.providerErr) || err != nil && strings.Contains(err.Error(), secretError.Error()) {
				t.Fatalf("error=%v", err)
			}
			if called != tc.wantCall || (s.Metadata.PGDropConstraintReference != nil) != tc.wantFacts || s.Metadata.PGDropConstraintReference == facts {
				t.Fatalf("called=%v facts=%+v", called, s.Metadata.PGDropConstraintReference)
			}
			payload, err := json.Marshal(s.Metadata)
			if err != nil || strings.Contains(string(payload), facts.Name) || strings.Contains(string(payload), "PGDropConstraintReference") {
				t.Fatalf("private evidence: %s %v", payload, err)
			}
		})
	}
}
