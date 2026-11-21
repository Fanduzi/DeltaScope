package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGViewNamespaces(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGViewNamespaceLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableNamespace == nil {
			continue
		}
		facts := *object.PGTableNamespace
		if facts.Schema == lookup.Schema && facts.Table == lookup.Qualifiers["table"] &&
			facts.Action == lookup.Qualifiers["action"] && facts.Destination == lookup.Name {
			s.Metadata.PGTableNamespace = &facts
		}
	}
	return nil
}
