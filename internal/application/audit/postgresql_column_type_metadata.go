package audit

import (
	"context"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGColumnTypes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		if reason, known := spec.PGColumnTypeSource(s); !known || reason != "" {
			continue
		}
		target := s.DDL.Alter[0].PGColumnType
		if !target.Qualified() {
			continue
		}
		lookup := spec.ObjectLookupRequest{Type: "column_type_destination", Schema: target.Schema, Name: target.Name,
			Qualifiers: map[string]string{"array": strconv.FormatBool(target.Array)},
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGColumnType == nil || object.PGColumnType.Target != target {
			continue
		}
		facts := *object.PGColumnType
		s.Metadata.PGColumnType = &facts
	}
	return nil
}
