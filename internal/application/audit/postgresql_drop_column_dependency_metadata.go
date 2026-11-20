package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGDropColumnDependencies(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, statement := range statements {
		name := spec.PGDropColumnDependencyTarget(statement)
		if name == "" {
			continue
		}
		table := statement.DDL.Table
		lookup := spec.ObjectLookupRequest{Type: "drop_column_dependency", Schema: table.Schema, Name: name,
			Qualifiers: map[string]string{"table": table.Name}}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGDropColumnDependency == nil {
			continue
		}
		fact := *object.PGDropColumnDependency
		if fact.Schema == table.Schema && fact.Table == table.Name && fact.Column == name {
			statement.Metadata.PGDropColumnDependency = &fact
		}
	}
	return nil
}
