// Package audit orchestrates audit use cases at the application layer.
// input: optional metadata providers plus parsed statement targets for enrichment
// output: metadata-enriched statements with resolved target schemas for rules that can use live instance or schema facts
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

// ObjectResolver optionally resolves non-table database objects from live metadata.
type ObjectResolver interface {
	ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)
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
			enriched[i].Metadata = &spec.Metadata{Schema: metadataTargetSchema(request, statement)}

			if lookup := planObjectLookup(request.Schema, statement); lookup != nil {
				objSnapshot := resolveObjectSnapshot(ctx, request, dialect, lookup)
				if objSnapshot != nil {
					enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
				}
			}
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
		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts}

		tableName, err := metadataTargetTableName(ctx, dialect, request, statement)
		if err != nil {
			return nil, fmt.Errorf("resolve index owner: %w", err)
		}
		if tableName != "" {
			key := strings.ToLower(metadataSchema) + "." + strings.ToLower(tableName)
			snapshot, ok := snapshots[key]
			if !ok {
				snapshot, err = request.Provider.LoadTableSnapshot(ctx, dialect, metadataSchema, tableName)
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

		// Object-level enrichment for non-table DDL objects.
		if lookup := planObjectLookup(request.Schema, statement); lookup != nil {
			objSnapshot := resolveObjectSnapshot(ctx, request, dialect, lookup)
			if objSnapshot != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: request.Schema, Instance: instanceFacts}
				}
				enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
			}
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

func metadataTargetSchema(request *MetadataRequest, statement spec.Statement) string {
	if (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) && statement.DML != nil && len(statement.DML.Tables) > 0 {
		if schema := strings.TrimSpace(statement.DML.Tables[0].Schema); schema != "" {
			return schema
		}
	}
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Schema)
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

// planObjectLookup returns an ObjectLookupRequest for the given statement,
// or nil if the statement does not target a resolvable non-table object.
func planObjectLookup(schema string, statement spec.Statement) *spec.ObjectLookupRequest {
	if statement.DDL == nil {
		return nil
	}
	ddl := statement.DDL
	objType := objectTypeForOperation(ddl.Operation, ddl.ObjectType)
	if objType == "" {
		return nil
	}
	name := strings.TrimSpace(ddl.ObjectName)
	if name == "" {
		return nil
	}
	req := &spec.ObjectLookupRequest{
		Schema: schema,
		Type:   objType,
		Name:   name,
	}
	if ddl.Options != nil {
		if table := strings.TrimSpace(ddl.Options["table"]); table != "" {
			if req.Qualifiers == nil {
				req.Qualifiers = make(map[string]string)
			}
			req.Qualifiers["table"] = table
		}
	}
	return req
}

// objectTypeForOperation maps a DDL operation and extracted object type to a
// metadata lookup type. Returns empty string for operations that don't target
// resolvable non-table objects (e.g. table operations handled by TableSnapshot).
func objectTypeForOperation(op spec.DDLOperation, extractedType string) string {
	switch op {
	case spec.DDLOperationCreateType, spec.DDLOperationAlterType, spec.DDLOperationDropType:
		return "type"
	case spec.DDLOperationCreateDomain, spec.DDLOperationAlterDomain, spec.DDLOperationDropDomain:
		return "domain"
	case spec.DDLOperationCreateExtension, spec.DDLOperationAlterExtension, spec.DDLOperationDropExtension:
		return "extension"
	case spec.DDLOperationCreatePublication, spec.DDLOperationAlterPublication, spec.DDLOperationDropPublication:
		return "publication"
	case spec.DDLOperationCreateSubscription, spec.DDLOperationAlterSubscription, spec.DDLOperationDropSubscription:
		return "subscription"
	case spec.DDLOperationCreateForeignTable, spec.DDLOperationAlterForeignTable, spec.DDLOperationDropForeignTable:
		return "foreign_table"
	case spec.DDLOperationCreateForeignServer, spec.DDLOperationAlterForeignServer, spec.DDLOperationDropForeignServer:
		return "foreign_server"
	case spec.DDLOperationCreateUserMapping, spec.DDLOperationAlterUserMapping, spec.DDLOperationDropUserMapping:
		return "user_mapping"
	case spec.DDLOperationCreateForeignDataWrapper, spec.DDLOperationAlterForeignDataWrapper, spec.DDLOperationDropForeignDataWrapper:
		return "foreign_data_wrapper"
	case spec.DDLOperationCreateEventTrigger, spec.DDLOperationAlterEventTrigger, spec.DDLOperationDropEventTrigger:
		return "event_trigger"
	case spec.DDLOperationCreateRule, spec.DDLOperationAlterRule, spec.DDLOperationDropRule:
		return "rule"
	case spec.DDLOperationCreateSchema, spec.DDLOperationAlterSchema, spec.DDLOperationDropSchema:
		return "schema"
	case spec.DDLOperationCreateSequence, spec.DDLOperationAlterSequence, spec.DDLOperationDropSequence:
		return "sequence"
	case spec.DDLOperationCreateMaterializedView, spec.DDLOperationDropMaterializedView, spec.DDLOperationRefreshMaterializedView, spec.DDLOperationAlterMaterializedView:
		return "materialized_view"
	case spec.DDLOperationCommentOn, spec.DDLOperationSecurityLabel:
		return extractedType
	default:
		return ""
	}
}

// resolveObjectSnapshot calls the ObjectResolver if available, or returns
// an unavailable snapshot as fallback. Never returns nil.
func resolveObjectSnapshot(ctx context.Context, request *MetadataRequest, dialect spec.Dialect, lookup *spec.ObjectLookupRequest) *spec.ObjectSnapshot {
	if request == nil || request.Provider == nil {
		return &spec.ObjectSnapshot{
			Schema: lookup.Schema,
			Type:   lookup.Type,
			Name:   lookup.Name,
			Status: spec.MetadataStatusUnavailable,
		}
	}
	resolver, ok := request.Provider.(ObjectResolver)
	if !ok {
		return &spec.ObjectSnapshot{
			Schema: lookup.Schema,
			Type:   lookup.Type,
			Name:   lookup.Name,
			Status: spec.MetadataStatusUnavailable,
		}
	}
	snapshot, err := resolver.ResolveObject(ctx, dialect, *lookup)
	if err != nil {
		return &spec.ObjectSnapshot{
			Schema: lookup.Schema,
			Type:   lookup.Type,
			Name:   lookup.Name,
			Status: spec.MetadataStatusUnavailable,
		}
	}
	return snapshot
}
