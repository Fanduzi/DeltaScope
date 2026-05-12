// Package audit orchestrates audit use cases at the application layer.
// input: optional metadata providers plus parsed statement targets for enrichment
// output: metadata-enriched statements for rules that can use live instance or schema facts
// pos: application-layer bridge between provider-backed metadata and domain statements
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// MetadataProvider supplies optional instance and schema facts for one audit run.
type MetadataProvider interface {
	LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error)
	LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error)
}

// IndexOwnerResolver optionally resolves standalone index statements back to owning tables.
type IndexOwnerResolver interface {
	ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error)
}

// PlanEstimator optionally loads planner-backed DML impact estimates.
type PlanEstimator interface {
	LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error)
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
		return nil, fmt.Errorf("load instance facts: %w", err)
	}

	snapshots := make(map[string]*spec.TableSnapshot)
	enriched := make([]spec.Statement, len(statements))

	for i, statement := range statements {
		enriched[i] = statement
		metadata := &spec.Metadata{Schema: request.Schema, Instance: instanceFacts}

		tableName, err := metadataTargetTableName(ctx, dialect, request, statement)
		if err != nil {
			return nil, fmt.Errorf("resolve index owner: %w", err)
		}
		if tableName != "" {
			key := strings.ToLower(tableName)
			snapshot, ok := snapshots[key]
			if !ok {
				snapshot, err = request.Provider.LoadTableSnapshot(ctx, dialect, request.Schema, tableName)
				if err != nil {
					return nil, fmt.Errorf("load table snapshot for %s: %w", tableName, err)
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
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateTable, spec.DDLOperationAlterTable, spec.DDLOperationDropTable, spec.DDLOperationTruncateTable:
			return strings.TrimSpace(statement.DDL.Table.Name)
		default:
			return ""
		}
	}
	if statement.DML != nil && len(statement.DML.Tables) > 0 {
		return strings.TrimSpace(statement.DML.Tables[0].Name)
	}
	return ""
}

func metadataTargetTableName(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statement spec.Statement) (string, error) {
	tableName := targetTableName(statement)
	if tableName != "" {
		return tableName, nil
	}
	if request == nil || request.Provider == nil || statement.DDL == nil {
		return "", nil
	}
	resolver, ok := request.Provider.(IndexOwnerResolver)
	if !ok {
		return "", nil
	}
	// Standalone ALTER INDEX operations (PostgreSQL).
	if statement.DDL.Operation == spec.DDLOperationAlterIndex {
		indexName := strings.TrimSpace(statement.DDL.ObjectName)
		if indexName == "" {
			return "", nil
		}
		schema := indexOwnerSchema(request, statement.DDL.Options)
		if schema == "" {
			return "", nil
		}
		return resolver.ResolveTableForIndex(ctx, dialect, schema, indexName)
	}
	// Alter-based index operations (MySQL/TiDB).
	if len(statement.DDL.Alter) == 0 {
		return "", nil
	}
	for _, alter := range statement.DDL.Alter {
		switch alter.Action {
		case "rename_index", "drop_index":
			if strings.TrimSpace(alter.Name) == "" {
				continue
			}
			schema := indexStatementSchema(request, alter)
			if schema == "" {
				continue
			}
			return resolver.ResolveTableForIndex(ctx, dialect, schema, alter.Name)
		}
	}
	return "", nil
}

func indexOwnerSchema(request *MetadataRequest, options map[string]string) string {
	if options != nil {
		if schema := strings.TrimSpace(options["schema"]); schema != "" {
			return schema
		}
	}
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Schema)
}

func indexStatementSchema(request *MetadataRequest, alter spec.Alter) string {
	if alter.Options != nil {
		if schema := strings.TrimSpace(alter.Options["schema"]); schema != "" {
			return schema
		}
	}
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Schema)
}
