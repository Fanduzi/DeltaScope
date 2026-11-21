package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateStorage(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCreateStorageLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCreateStorage != nil {
			state := *object.PGCreateStorage
			s.Metadata.PGCreateStorage = &state
		}
	}
	return nil
}
