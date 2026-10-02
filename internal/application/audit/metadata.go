// Package audit orchestrates audit use cases at the application layer.
// input: optional metadata providers, parsed statement MutationTargets, the validated target_version for observed-identity reconciliation, and parse-failure positions that contaminate later derived state
// output: metadata-enriched statements with resolved target schemas, per-statement pre-state snapshots from the request-local ordered batch state (provider facts or in-batch derivations for MySQL/TiDB — PostgreSQL keeps its original enrichment in every request shape), and a canonical Version identity for rules that can use live instance or schema facts
// pos: application-layer bridge between provider-backed metadata and domain statements, owning the ordered schema-state seam (batch_state.go) and the effective-identity resolver (explicit qualifiers win over the request schema)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"sort"
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
	// TargetVersion is the caller-supplied version constraint (already
	// strict-parsed and product-stamped by the audit service). Offline it is
	// the statement's version fact; online it only bounds the observed
	// identity and can never override it.
	TargetVersion *spec.VersionIdentity
}

func enrichStatementsWithMetadata(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement, failures []parseFailure) ([]spec.Statement, error) {
	if request == nil {
		if !orderedStateDialect(dialect) {
			return statements, nil
		}
		// No request at all still gets the ordered state pass: batch-local
		// derived facts satisfy table/column existence even when no provider,
		// schema, or version is configured.
		return enrichOrderedStatements(ctx, dialect, nil, statements, failures, nil, nil)
	}

	if request.Provider == nil {
		if !orderedStateDialect(dialect) {
			// PostgreSQL keeps its original enrichment: schema/version
			// attachment plus object lookups, never the ordered state pass.
			if strings.TrimSpace(request.Schema) == "" && request.TargetVersion == nil {
				return statements, nil
			}
			enriched := make([]spec.Statement, len(statements))
			for i, statement := range statements {
				enriched[i] = statement
				enriched[i].Metadata = &spec.Metadata{
					Schema:  metadataTargetSchema(request, statement),
					Version: request.TargetVersion,
				}

				if lookup := planObjectLookup(request.Schema, statement); lookup != nil {
					objSnapshot := resolveObjectSnapshot(ctx, request, dialect, lookup)
					if objSnapshot != nil {
						enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
					}
				}
			}
			return enriched, nil
		}
		return enrichOrderedStatements(ctx, dialect, request, statements, failures, nil, request.TargetVersion)
	}

	instanceFacts, err := request.Provider.LoadInstanceFacts(ctx, dialect, request.Schema)
	if err != nil {
		return nil, fmt.Errorf("load instance facts: %w", err)
	}

	// The observed banner is the only authoritative online version fact — its
	// product is derived from the banner itself, never injected from the
	// requested dialect. A dialect/observed product conflict and a
	// target/observed version conflict are both typed input mismatches; an
	// unparseable banner stays missing so version-dependent rules report a
	// bounded gap.
	observed := spec.ParseObservedVersion(instanceFactVersion(instanceFacts))
	if observed != nil && observed.Version != "" {
		if dialectProduct := spec.VersionProductForDialect(dialect); dialectProduct != "" && observed.Product != dialectProduct {
			return nil, fmt.Errorf("%w: dialect=%s observed_product=%s observed_version=%s", ErrDialectProductMismatch, dialect, observed.Product, observed.Version)
		}
		if request.TargetVersion != nil && request.TargetVersion.Version != observed.Version {
			return nil, fmt.Errorf("%w: target=%s observed=%s", ErrTargetVersionMismatch, request.TargetVersion.Version, observed.Version)
		}
	}
	resolvedVersion := observed

	if orderedStateDialect(dialect) {
		return enrichOrderedStatements(ctx, dialect, request, statements, failures, instanceFacts, resolvedVersion)
	}

	snapshots := make(map[string]*spec.TableSnapshot)
	enriched := make([]spec.Statement, len(statements))

	for i, statement := range statements {
		enriched[i] = statement
		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}

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

		if metadata.Schema != "" || metadata.Instance != nil || metadata.TargetTable != nil || metadata.Version != nil {
			enriched[i].Metadata = metadata
		}

		// Object-level enrichment for non-table DDL objects.
		if lookup := planObjectLookup(request.Schema, statement); lookup != nil {
			objSnapshot := resolveObjectSnapshot(ctx, request, dialect, lookup)
			if objSnapshot != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: request.Schema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
			}
		}
	}

	return enriched, nil
}

// enrichOrderedStatements is the MySQL/TiDB enrichment path: one batch-local
// state table feeds every statement an immutable pre-state projection, then
// records the statement's conditional post-state. Provider facts load at most
// once per (schema, table); derived state never flows back into the provider
// and never aliases into statement metadata.
func enrichOrderedStatements(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement, failures []parseFailure, instanceFacts *spec.InstanceFacts, resolvedVersion *spec.VersionIdentity) ([]spec.Statement, error) {
	requestSchema := ""
	var provider MetadataProvider
	if request != nil {
		requestSchema = request.Schema
		provider = request.Provider
	}
	state := newBatchState(dialect, requestSchema, provider)
	enriched := make([]spec.Statement, len(statements))
	// Object lookups resolve at most once per distinct (schema, type, name,
	// qualifiers) identity per request — the same object is never re-asked.
	objectCache := make(map[objectLookupKey]*spec.ObjectSnapshot)

	for i, statement := range statements {
		for _, failure := range failures {
			if failureBefore(failure.Line, failure.Column, statement.Line, statement.Column) {
				state.contaminated = true
				break
			}
		}

		enriched[i] = statement
		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}

		target, err := orderedTargetTable(ctx, dialect, request, statement, metadataSchema)
		if err != nil {
			return nil, fmt.Errorf("resolve index owner: %w", err)
		}
		if tableName := strings.TrimSpace(target.Name); tableName != "" {
			snapshot, err := state.preState(ctx, metadataSchema, target)
			if err != nil {
				return nil, fmt.Errorf("load table snapshot for %s: %w", tableName, err)
			}
			metadata.TargetTable = snapshot
		}

		if metadata.Schema != "" || metadata.Instance != nil || metadata.TargetTable != nil || metadata.Version != nil {
			enriched[i].Metadata = metadata
		}

		if request != nil {
			if lookup := planObjectLookup(requestSchema, statement); lookup != nil {
				cacheKey := newObjectLookupKey(lookup)
				objSnapshot, cached := objectCache[cacheKey]
				if !cached {
					objSnapshot = resolveObjectSnapshot(ctx, request, dialect, lookup)
					objectCache[cacheKey] = objSnapshot
				}
				if objSnapshot != nil {
					if enriched[i].Metadata == nil {
						enriched[i].Metadata = &spec.Metadata{Schema: requestSchema, Instance: instanceFacts, Version: resolvedVersion}
					}
					enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
				}
			}
		}

		if err := state.apply(ctx, statement); err != nil {
			return nil, err
		}
	}

	return enriched, nil
}

// orderedTargetTable resolves the effective table identity a statement's
// pre-state projection is about. Qualified names keep their qualifier — the
// explicit schema always wins over the request schema; index-owner resolver
// answers carry the schema they were resolved under, falling back to the
// statement's effective schema.
func orderedTargetTable(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statement spec.Statement, metadataSchema string) (spec.Table, error) {
	switch {
	case statement.DDL != nil && statement.DDL.Table != nil:
		return *statement.DDL.Table, nil
	case statement.DML != nil:
		if targets := statement.DML.MutationTargetTables(); len(targets) > 0 {
			return targets[0], nil
		}
		return spec.Table{}, nil
	case statement.DDL == nil:
		return spec.Table{}, nil
	}
	name, schema, err := indexOwnerTarget(ctx, dialect, request, statement)
	if err != nil {
		return spec.Table{}, err
	}
	if strings.TrimSpace(name) == "" {
		return spec.Table{}, nil
	}
	if schema == "" {
		schema = metadataSchema
	}
	return spec.Table{Schema: schema, Name: name}, nil
}

func instanceFactVersion(facts *spec.InstanceFacts) string {
	if facts == nil {
		return ""
	}
	return facts.Version
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
	if statement.DML != nil {
		if targets := statement.DML.MutationTargetTables(); len(targets) > 0 {
			return strings.TrimSpace(targets[0].Name)
		}
	}
	return ""
}

func metadataTargetSchema(request *MetadataRequest, statement spec.Statement) string {
	if (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) && statement.DML != nil {
		if targets := statement.DML.MutationTargetTables(); len(targets) > 0 {
			if schema := strings.TrimSpace(targets[0].Schema); schema != "" {
				return schema
			}
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
	name, _, err := indexOwnerTarget(ctx, dialect, request, statement)
	return name, err
}

// indexOwnerTarget resolves a statement's owning table through the optional
// IndexOwnerResolver — for statements that name an index but not its table
// (standalone ALTER INDEX, alter-based rename/drop index). It returns the
// resolved table name and the schema the resolution ran under.
func indexOwnerTarget(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statement spec.Statement) (string, string, error) {
	if request == nil || request.Provider == nil || statement.DDL == nil {
		return "", "", nil
	}
	resolver, ok := request.Provider.(IndexOwnerResolver)
	if !ok {
		return "", "", nil
	}
	// Standalone ALTER INDEX operations (PostgreSQL).
	if statement.DDL.Operation == spec.DDLOperationAlterIndex {
		indexName := strings.TrimSpace(statement.DDL.ObjectName)
		if indexName == "" {
			return "", "", nil
		}
		schema := indexOwnerSchema(request, statement.DDL.Options)
		if schema == "" {
			return "", "", nil
		}
		name, err := resolver.ResolveTableForIndex(ctx, dialect, schema, indexName)
		return name, schema, err
	}
	// Alter-based index operations (MySQL/TiDB).
	if len(statement.DDL.Alter) == 0 {
		return "", "", nil
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
			name, err := resolver.ResolveTableForIndex(ctx, dialect, schema, alter.Name)
			return name, schema, err
		}
	}
	return "", "", nil
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

// objectLookupKey is the structured identity of one object-resolution request
// inside an audit: schema, object type, name, and the canonical qualifiers
// list so qualifier-carrying lookups never collide with bare ones.
type objectLookupKey struct {
	schema     string
	objectType string
	name       string
	qualifiers string
}

func newObjectLookupKey(lookup *spec.ObjectLookupRequest) objectLookupKey {
	key := objectLookupKey{
		schema:     strings.ToLower(strings.TrimSpace(lookup.Schema)),
		objectType: strings.ToLower(strings.TrimSpace(lookup.Type)),
		name:       strings.ToLower(strings.TrimSpace(lookup.Name)),
	}
	if len(lookup.Qualifiers) > 0 {
		names := make([]string, 0, len(lookup.Qualifiers))
		for name := range lookup.Qualifiers {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		for _, name := range names {
			b.WriteString(name)
			b.WriteByte('=')
			b.WriteString(lookup.Qualifiers[name])
			b.WriteByte(';')
		}
		key.qualifiers = b.String()
	}
	return key
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
