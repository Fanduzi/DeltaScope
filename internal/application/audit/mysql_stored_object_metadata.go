package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichMySQLStoredObjects(ctx context.Context, request *MetadataRequest, statements []spec.Statement, failures []parseFailure) error {
	for _, statement := range statements {
		for _, failure := range failures {
			if failure.Line <= 0 || statement.Line <= 0 || failureBefore(failure.Line, failure.Column, statement.Line, statement.Column) {
				return nil
			}
		}
		if statement.Unsupported != nil || statement.ResourceLimit != nil || statement.Kind == spec.KindUnknown {
			return nil
		}
		lookup := spec.MySQLStoredObjectLookup(statement)
		if lookup != nil && statement.Metadata != nil {
			object, err := resolveObjectSnapshot(ctx, request, spec.DialectMySQL, lookup)
			if err != nil {
				return err
			}
			if object != nil && object.MySQLStoredObject != nil {
				state := *object.MySQLStoredObject
				statement.Metadata.MySQLStoredObject = &state
			}
		}
		if statement.Kind == spec.KindDDL {
			return nil
		}
	}
	return nil
}
