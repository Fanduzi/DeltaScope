package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCollationRefresh(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCollationRefreshLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCollationRefresh != nil && object.PGCollationRefresh.Target == *s.DDL.PGCollationRefresh {
			state := *object.PGCollationRefresh
			s.Metadata.PGCollationRefresh = &state
		}
	}
	return nil
}
