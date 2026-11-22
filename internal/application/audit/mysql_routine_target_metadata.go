package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichMySQLRoutineTargets(ctx context.Context, request *MetadataRequest, statements []spec.Statement, failures []parseFailure) error {
	for _, statement := range statements {
		for _, failure := range failures {
			if failure.Line <= 0 || statement.Line <= 0 || failureBefore(failure.Line, failure.Column, statement.Line, statement.Column) {
				return nil
			}
		}
		if statement.Unsupported != nil || statement.ResourceLimit != nil || statement.Kind == spec.KindUnknown {
			return nil
		}
		lookup := spec.MySQLRoutineLookup(statement)
		if lookup != nil && statement.Metadata != nil {
			object, err := resolveObjectSnapshot(ctx, request, spec.DialectMySQL, lookup)
			if err != nil {
				return err
			}
			if object != nil && object.MySQLRoutine != nil {
				state := *object.MySQLRoutine
				statement.Metadata.MySQLRoutine = &state
			}
		}
		if statement.Kind == spec.KindDDL {
			return nil
		}
	}
	return nil
}
