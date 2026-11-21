package audit

import (
	"context"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateColumnTypes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	catalog := make(map[spec.PGColumnTypeTarget]*spec.PGColumnTypeState)
	for _, s := range statements {
		if s.DDL == nil || s.DDL.PGCreateColumns == nil || s.Metadata == nil {
			continue
		}
		decl := s.DDL.PGCreateColumns
		destination := s.Metadata.PGCreateDestination
		if destination == nil || destination.Target != decl.Target || !destination.SchemaExists || decl.IfNotExists && destination.RelationExists || decl.Refusal == "create_column_duplicate" {
			continue
		}
		s.Metadata.PGCreateColumnTypes = make([]*spec.PGColumnTypeState, len(decl.Types))
		for i, target := range decl.Types {
			state, cached := catalog[target]
			if !cached {
				lookup := &spec.ObjectLookupRequest{Type: "column_type_destination", Schema: target.Schema, Name: target.Name,
					Qualifiers: map[string]string{"array": strconv.FormatBool(target.Array)}}
				object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
				if err != nil {
					return err
				}
				if object != nil && object.PGColumnType != nil && object.PGColumnType.Target == target {
					copy := *object.PGColumnType
					state = &copy
				}
				catalog[target] = state
			}
			s.Metadata.PGCreateColumnTypes[i] = state
		}
	}
	return nil
}
