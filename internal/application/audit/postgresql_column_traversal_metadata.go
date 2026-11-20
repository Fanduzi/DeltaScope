package audit

import (
	"context"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGColumnTraversal(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name := spec.PGColumnTraversalTarget(s)
		if name == "" {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{Type: "column_traversal", Schema: table.Schema, Name: name, Qualifiers: map[string]string{"table": table.Name, "action": s.DDL.Alter[0].Action, "only": strconv.FormatBool(s.DDL.PGTableOnly)}}
		if destination := spec.PGColumnRenameDestination(s.DDL.Alter[0]); destination != "" {
			lookup.Qualifiers["destination"] = destination
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGColumnTraversal != nil {
			fact := *object.PGColumnTraversal
			if fact.Schema == table.Schema && fact.Table == table.Name && fact.Column == name {
				s.Metadata.PGColumnTraversal = &fact
			}
		}
	}
	return nil
}
