package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGRelationProofFreshness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix spec.Statement
		failed []parseFailure
		stale  bool
	}{
		{name: "observed absence"},
		{name: "earlier implicit relation creation", prefix: spec.Statement{Kind: spec.KindDDL, Line: 1, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Table: &spec.Table{Schema: "app", Name: "other"}}}, stale: true},
		{name: "earlier parser error", failed: []parseFailure{{Line: 1, Column: 1}}, stale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &spec.TableSnapshot{Schema: "app", PGRelation: spec.PGRelationState{Name: "missing"}}
			target := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, Line: 2, Column: 1,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "missing"}, PGTableIfExists: true},
				Metadata: &spec.Metadata{Schema: "app", TargetTable: snapshot}}
			statements := []spec.Statement{target}
			if tc.prefix.DDL != nil {
				statements = append([]spec.Statement{tc.prefix}, statements...)
			}
			invalidateStalePGTargetCatalogs(statements, tc.failed)
			actual := statements[len(statements)-1].Metadata.TargetTable
			if tc.stale != (actual.PGRelation == (spec.PGRelationState{})) || snapshot.PGRelation.Name != "missing" {
				t.Fatalf("projected=%+v provider=%+v", actual.PGRelation, snapshot.PGRelation)
			}
			clone := cloneTableSnapshot(snapshot)
			clone.PGRelation.Kind = "r"
			if clone.PGRelation.Name != "missing" || snapshot.PGRelation.Kind != "" {
				t.Fatal("relation observation was lost or aliased")
			}
		})
	}
}

func TestPGMetadataTargetTableNameKeepsDecodedSpaces(t *testing.T) {
	s := spec.Statement{Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: " "}}}
	if got := targetTableName(s); got != " " {
		t.Fatalf("decoded table name changed: %q", got)
	}
}
