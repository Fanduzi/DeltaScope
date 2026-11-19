package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTablePersistence(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		action := spec.PGTablePersistenceTarget(s)
		if action == "" || !spec.PGExactTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: table.Name, Type: "table_persistence",
			Qualifiers: map[string]string{"action": action},
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTablePersistence == nil {
			continue
		}
		facts := *object.PGTablePersistence
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Logged == (action == "set_logged") {
			s.Metadata.PGTablePersistence = &facts
		}
	}
	return nil
}
