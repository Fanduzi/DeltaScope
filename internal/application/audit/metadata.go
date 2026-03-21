// Package audit orchestrates audit use cases at the application layer.
// input: optional metadata providers plus parsed statement targets for enrichment
// output: metadata-enriched statements for rules that can use live instance or schema facts
// pos: application-layer bridge between provider-backed metadata and domain statements
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// MetadataProvider supplies optional instance and schema facts for one audit run.
type MetadataProvider interface {
	LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error)
	LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error)
}

// MetadataRequest describes one optional metadata-aware audit invocation.
type MetadataRequest struct {
	Schema   string
	Provider MetadataProvider
}

func enrichStatementsWithMetadata(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement) ([]spec.Statement, error) {
	if request == nil {
		return statements, nil
	}

	if request.Provider == nil {
		if strings.TrimSpace(request.Schema) == "" {
			return statements, nil
		}
		enriched := make([]spec.Statement, len(statements))
		for i, statement := range statements {
			enriched[i] = statement
			enriched[i].Metadata = &spec.Metadata{Schema: request.Schema}
		}
		return enriched, nil
	}

	instanceFacts, err := request.Provider.LoadInstanceFacts(ctx, dialect, request.Schema)
	if err != nil {
		return nil, err
	}

	snapshots := make(map[string]*spec.TableSnapshot)
	enriched := make([]spec.Statement, len(statements))

	for i, statement := range statements {
		enriched[i] = statement
		metadata := &spec.Metadata{Schema: request.Schema, Instance: instanceFacts}

		tableName := targetTableName(statement)
		if tableName != "" {
			key := strings.ToLower(tableName)
			snapshot, ok := snapshots[key]
			if !ok {
				snapshot, err = request.Provider.LoadTableSnapshot(ctx, dialect, request.Schema, tableName)
				if err != nil {
					return nil, err
				}
				snapshots[key] = snapshot
			}
			metadata.TargetTable = snapshot
		}

		if metadata.Schema != "" || metadata.Instance != nil || metadata.TargetTable != nil {
			enriched[i].Metadata = metadata
		}
	}

	return enriched, nil
}

func targetTableName(statement spec.Statement) string {
	if statement.DDL != nil && statement.DDL.Table != nil {
		return strings.TrimSpace(statement.DDL.Table.Name)
	}
	if statement.DML != nil && len(statement.DML.Tables) > 0 {
		return strings.TrimSpace(statement.DML.Tables[0].Name)
	}
	return ""
}
