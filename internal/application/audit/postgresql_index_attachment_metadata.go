package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGIndexAttachments(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGIndexAttachmentLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGIndexAttachment != nil && object.PGIndexAttachment.Target == *s.DDL.PGIndexAttachment {
			state := *object.PGIndexAttachment
			s.Metadata.PGIndexAttachment = &state
		}
	}
	return nil
}
