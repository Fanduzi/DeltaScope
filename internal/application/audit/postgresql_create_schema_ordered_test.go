package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateSchemaWithOwnerWithholdsOrderedPresence(t *testing.T) {
	for _, target := range []spec.PGCreateSchemaTarget{
		{Name: "app", Owner: spec.PGTableOwnerTarget{Kind: "named", Name: "owner"}},
		{Owner: spec.PGTableOwnerTarget{Kind: "named", Name: "app"}},
		{Owner: spec.PGTableOwnerTarget{Kind: "current_user"}},
	} {
		state := newBatchState(spec.DialectPostgreSQL, "public", nil)
		state.schemaEntries["app"] = &batchSchemaEntry{state: schemaAbsent}
		stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateSchema, ObjectName: target.Name, PGCreateSchema: &target}}
		if err := state.applyCreateSchema(context.Background(), stmt); err != nil {
			t.Fatal(err)
		}
		if got := state.schemaEntries["app"].state; got != schemaUnknown {
			t.Fatalf("target %+v left schema presence %v, want unknown", target, got)
		}
	}
}
