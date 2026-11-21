package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGRefreshMaterializedViews(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		if s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Operation != spec.DDLOperationRefreshMaterializedView ||
			s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		target := s.DDL.PGRefreshTarget
		if !target.Qualified() || target.Name != s.DDL.ObjectName {
			continue
		}
		lookup := spec.ObjectLookupRequest{Type: "refresh_materialized_view", Schema: target.Schema, Name: target.Name}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGRefreshState != nil && object.PGRefreshState.Target == target {
			facts := *object.PGRefreshState
			s.Metadata.PGRefreshState = &facts
		}
	}
	return nil
}
