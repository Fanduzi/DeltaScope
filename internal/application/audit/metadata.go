// Package audit orchestrates audit use cases at the application layer.
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

// PlacementPolicyLoader optionally loads the TiDB placement-policy catalog:
// proven existence plus each policy's bounded bound-object set. A nil map
// return means the catalog was not loaded (unvouched); a non-nil empty map
// is the proven-empty answer. Errors propagate — a failed read is never
// cached as absence (T20/#99).
type PlacementPolicyLoader interface {
	LoadPlacementPolicies(ctx context.Context, dialect spec.Dialect) (map[string]*spec.PlacementPolicyFact, error)
}

// MySQLResourceGroupFactsLoader optionally loads the T24/#103
// ordinary-client premise bundle — the global read-only variables, the
// executing subject's effective grants (base + active roles), the
// proven bound-group occupancy set, and the feature-positive probe. It
// is invoked at most once per request and only while a batch names a
// MySQL resource group; the returned facts fill only still-unproven
// slots. Errors propagate — a failed read is never degraded into an
// absence claim.
type MySQLResourceGroupFactsLoader interface {
	LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error)
}

// MySQLServerDropPrivilegeLoader optionally supplies the DROP SERVER
// privilege observation — the verbatim CURRENT_USER() account and the
// SHOW GRANTS FOR CURRENT_USER() lines read on one provider connection
// (T26-SERVER-DROP-EXECUTION-01). It is invoked at most once per
// request, only under spec.PrivilegeAuditModeObservedAccountNewSession,
// and only while the batch names an audited identifier-form DROP
// SERVER. A nil return means the observation never ran; a failed read
// propagates under the same bounded error contract as the object seam
// — never degraded into an unproven grant, an evidence gap, or a
// cached answer.
type MySQLServerDropPrivilegeLoader interface {
	LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error)
}

// MySQLSRSDependencyReader optionally supplies the SRS ordered
// channel's deps-only read — the provider's Qo support gate plus the
// ST_GEOMETRY_COLUMNS witness read for one authored SRID; the
// existence query is never issued on this path
// (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01). Capability is
// judged on the wired provider itself: a plain ObjectResolver
// implementation does not imply it, and a request-level adapter or
// wrapper is never probed. A nil fact with a nil error is a real
// answer (no witness available, e.g. a proven-unsupported support
// gate); query/iteration/close errors propagate and are never degraded
// into empty facts.
type MySQLSRSDependencyReader interface {
	ResolveMySQLSRSDependencies(ctx context.Context, srid uint32, version *spec.VersionIdentity) (*spec.SRSDependencyFact, error)
}

// MetadataRequest describes one optional metadata-aware audit invocation.
type MetadataRequest struct {
	Schema   string
	Provider MetadataProvider
	// DeploymentEvidence is the caller-sourced, target-bound fact bundle
	// for premises no SQL channel observes (host VCPU count, priority
	// capability, feature support). See spec.DeploymentEvidence for the
	// binding contract — a mismatched or unsourced bundle is ignored.
	DeploymentEvidence *spec.DeploymentEvidence
	// TargetVersion is the caller-supplied version constraint (already
	// strict-parsed and product-stamped by the audit service). Offline it is
	// the statement's version fact; online it only bounds the observed
	// identity and can never override it.
	TargetVersion *spec.VersionIdentity
	// TargetIdentity is the caller-declared, non-secret target identity a
	// Target-bound DeploymentEvidence bundle matches against when no
	// provider-observed identity exists. It is a declarative guard, not
	// a verified credential (T24-R6).
	TargetIdentity string
	// PrivilegeAuditMode is the request's conditional
	// privilege-evidence mode — the only registered value is
	// spec.PrivilegeAuditModeObservedAccountNewSession; empty keeps the
	// default path where the DROP SERVER privilege premise stays owed
	// (T26-SERVER-DROP-EXECUTION-01).
	PrivilegeAuditMode spec.PrivilegeAuditMode
}

func enrichStatementsWithMetadata(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement, failures []parseFailure) ([]spec.Statement, error) {
	return enrichStatementsWithMetadataLimits(ctx, dialect, request, statements, failures, auditLimits{orderedStatements: defaultOrderedStatementLimit})
}

func enrichStatementsWithMetadataLimits(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement, failures []parseFailure, limits auditLimits) ([]spec.Statement, error) {
	if request == nil {
		if !orderedStateDialect(dialect) {
			return statements, nil
		}
		// No request at all still gets the ordered state pass: batch-local
		// derived facts satisfy table/column existence even when no provider,
		// schema, or version is configured.
		return enrichOrderedStatements(ctx, dialect, nil, statements, failures, nil, nil, limits)
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
					objSnapshot, oerr := resolveObjectSnapshot(ctx, request, dialect, lookup)
					if oerr != nil {
						return nil, oerr
					}
					if objSnapshot != nil {
						enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
					}
				}
			}
			return enriched, nil
		}
		return enrichOrderedStatements(ctx, dialect, request, statements, failures, nil, request.TargetVersion, limits)
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
		return enrichOrderedStatements(ctx, dialect, request, statements, failures, instanceFacts, resolvedVersion, limits)
	}

	snapshots := make(map[[2]string]*spec.TableSnapshot)
	enriched := make([]spec.Statement, len(statements))

	for i, statement := range statements {
		enriched[i] = statement
		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}

		tableName, err := metadataTargetTableName(ctx, dialect, request, statement)
		if err != nil {
			return nil, fmt.Errorf("resolve index owner: %w", err)
		}
		if tableName != "" && (metadataSchema != "" || statement.DDL == nil || !statement.DDL.PGTableIfExists) {
			key := [2]string{metadataSchema, tableName}
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
			objSnapshot, oerr := resolveObjectSnapshot(ctx, request, dialect, lookup)
			if oerr != nil {
				return nil, oerr
			}
			if objSnapshot != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: request.Schema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Objects = append(enriched[i].Metadata.Objects, *objSnapshot)
			}
		}
	}

	if dialect == spec.DialectPostgreSQL {
		freshPrefix := invalidateStalePGTargetCatalogs(enriched, failures)
		if err := enrichPGCheckIntegers(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGPartitionConstraints(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGOperatorChanges(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGTypeChanges(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCompositeTypes(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGEnums(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateSchemas(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCollationChange(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCollationRefresh(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGOperatorFamilies(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGIndexIncludes(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGIndexDestinations(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGIndexOptions(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGIndexAttachments(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateStorage(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateTyped(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateDestinations(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateColumnTypes(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateForeignKeys(ctx, request, enriched[:freshPrefix], snapshots); err != nil {
			return nil, err
		}
		if err := enrichPGCreateSources(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGCreateViewTargets(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGViewDefaults(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGMaterializedViewExtensions(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGViewOwners(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGViewNamespaces(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGDropViews(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGDestructiveTableRelations(ctx, request, enriched[:freshPrefix], snapshots); err != nil {
			return nil, err
		}
		if err := enrichPGRefreshMaterializedViews(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGBulkTablespaces(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGColumnTraversal(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGDropConstraintReferences(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
		if err := enrichPGForeignKeyReferences(ctx, request, enriched[:freshPrefix], snapshots); err != nil {
			return nil, err
		}
		if err := enrichPGConstraintUsingIndexes(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGReplicaIdentityIndexes(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGTableTablespaces(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGTableAccessMethods(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGTablePersistence(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGTableNamespaces(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGTableOwners(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGColumnTypes(ctx, request, enriched); err != nil {
			return nil, err
		}
		if err := enrichPGDropColumnDependencies(ctx, request, enriched[:freshPrefix]); err != nil {
			return nil, err
		}
	}
	return enriched, nil
}

// enrichOrderedStatements is the MySQL/TiDB enrichment path: one batch-local
// state table feeds every statement an immutable pre-state projection, then
// records the statement's conditional post-state. Provider facts load at most
// once per (schema, table); derived state never flows back into the provider
// and never aliases into statement metadata.
func enrichOrderedStatements(ctx context.Context, dialect spec.Dialect, request *MetadataRequest, statements []spec.Statement, failures []parseFailure, instanceFacts *spec.InstanceFacts, resolvedVersion *spec.VersionIdentity, limits auditLimits) ([]spec.Statement, error) {
	requestSchema := ""
	var provider MetadataProvider
	if request != nil {
		requestSchema = request.Schema
		provider = request.Provider
	}
	state := newBatchState(dialect, requestSchema, provider)
	state.resolvedVersion = resolvedVersion
	state.instanceFacts = instanceFacts
	return state.enrichStatements(ctx, request, statements, failures, instanceFacts, limits)
}

func (state *batchState) enrichStatements(ctx context.Context, request *MetadataRequest, statements []spec.Statement, failures []parseFailure, instanceFacts *spec.InstanceFacts, limits auditLimits) ([]spec.Statement, error) {
	dialect, resolvedVersion := state.dialect, state.resolvedVersion
	requestSchema := ""
	if request != nil {
		requestSchema = request.Schema
	}
	// Phase-2 qualification for the audited CREATE/ALTER SERVER
	// slices (T26-SERVER-CREATE-EXECUTION-01,
	// T26-SERVER-ALTER-EXECUTION-01): every shape-passing CREATE or
	// ALTER whose resolved version is unknown or outside the
	// registered MySQL 8.0.x proof domain takes the unaudited
	// boundary with gate:"version" — before the family loader gate
	// evaluates below, so a version-failed statement can never
	// trigger the observation. Conflicting version inputs (malformed
	// pin, observed product mismatch, pin/observed mismatch) exited
	// earlier as typed errors and never reach this gate.
	if dialect == spec.DialectMySQL {
		for i := range statements {
			if statements[i].DDL != nil && statements[i].DDL.Server != nil &&
				(statements[i].DDL.Server.Kind == spec.ServerKindCreate ||
					statements[i].DDL.Server.Kind == spec.ServerKindAlter) &&
				statements[i].Unsupported == nil &&
				!spec.MySQLServerDropPrivilegeDomain(resolvedVersion) {
				statements[i].Unsupported = &spec.UnsupportedDetail{
					Feature: string(statements[i].DDL.Operation),
					Reason:  spec.UnsupportedUnauditedReason,
					Metadata: map[string]any{
						"boundary": "unaudited",
						"object":   "server",
						"gate":     "version",
					},
				}
			}
			// The audited SRS drop slice (T26-SRS-DROP-EXECUTION-01)
			// shares this gate shape: every DROP whose OBSERVED version
			// is unknown or outside the registered MySQL 8.0.4+ domain
			// takes the unaudited boundary — a caller pin is a conflict
			// check only and never substitutes for observation
			// (frozen V1, ER1). The same version verdict now also
			// suppresses
			// the family's catalog reads inside bindMySQLSRS — one
			// request-level version fact controls both audit and read
			// admission (T26-SRS-VERSION-APPLICABILITY-01 frozen V1).
			// The CREATE family inherits the same catalog-domain gate
			// verbatim (T26-SRS-CREATE-PUBLICATION-01 frozen F-2). The
			// marker keeps gate:"version" and adds the four-way reason
			// subdivision plus the named applicability gap (frozen V4/
			// V11).
			if statements[i].DDL != nil && statements[i].DDL.SpatialReference != nil &&
				statements[i].Unsupported == nil &&
				!spec.MySQLSRSDropAuditedDomain(resolvedVersion) {
				statements[i].Unsupported = &spec.UnsupportedDetail{
					Feature: string(statements[i].DDL.Operation),
					Reason:  spec.UnsupportedUnauditedReason,
					Metadata: map[string]any{
						"boundary":       "unaudited",
						"object":         "spatial_reference_system",
						"gate":           "version",
						"version_reason": spec.MySQLSRSVersionGateReason(resolvedVersion, instanceFacts != nil),
						"gap_fact":       spec.SRSFactVersionApplicability,
					},
				}
			}
			// The ALTER INSTANCE gate is per form (D2): RELOAD TLS
			// needs MySQL 8.0+ while ROTATE INNODB MASTER KEY exists
			// from 5.7.11 — a proven-too-old or unresolvable version
			// gates the statement to the unaudited boundary, exactly
			// like the SRS family's catalog-domain gate.
			if statements[i].DDL != nil && statements[i].DDL.AlterInstance != nil &&
				statements[i].Unsupported == nil &&
				!spec.MySQLAlterInstanceAuditedDomain(resolvedVersion, statements[i].DDL.AlterInstance.Form) {
				metadata := map[string]any{
					"boundary": "unaudited",
					"object":   "instance",
					"gate":     "version",
				}
				if statements[i].DDL.AlterInstance.NoRollbackOnError {
					// The NO ROLLBACK ON ERROR member is recorded, not
					// merged silently into the plain subaction.
					metadata["no_rollback_on_error"] = true
				}
				statements[i].Unsupported = &spec.UnsupportedDetail{
					Feature:  statements[i].DDL.AlterInstance.Feature(),
					Reason:   spec.UnsupportedUnauditedReason,
					Metadata: metadata,
				}
			}
		}
	}
	enriched := make([]spec.Statement, len(statements))
	// The TiDB placement-policy catalog loads at most once per request and
	// only when a statement carries a placement fact — the proven existence
	// and bound-object sets the bind/drop adjudicators consume. Errors
	// propagate rather than caching a failed read as absence (T20/#99).
	if state.provider != nil && dialect == spec.DialectTiDB && statementsNeedPlacementCatalog(statements) {
		if loader, ok := state.provider.(PlacementPolicyLoader); ok {
			catalog, err := loader.LoadPlacementPolicies(ctx, dialect)
			if err != nil {
				return nil, fmt.Errorf("load placement policies: %w", err)
			}
			state.placementCatalog = catalog
		}
	}
	// The MySQL resource-group premise bundle loads at most once per
	// request and only while a statement names the family — the provider
	// read runs on the audited session's own identity, then any sourced
	// deployment evidence fills the slots still unproven. Neither channel
	// overwrites a proven answer (T24/#103).
	if dialect == spec.DialectMySQL && statementsNeedMySQLResourceGroupFacts(statements) {
		if state.provider != nil {
			if loader, ok := state.provider.(MySQLResourceGroupFactsLoader); ok {
				loaded, err := loader.LoadMySQLResourceGroupFacts(ctx, dialect)
				if err != nil {
					return nil, fmt.Errorf("load mysql resource-group facts: %w", err)
				}
				if instanceFacts == nil && loaded != nil {
					instanceFacts = &spec.InstanceFacts{}
					state.instanceFacts = instanceFacts
				}
				mergeMySQLResourceGroupFacts(instanceFacts, loaded)
			}
		}
		if request != nil && request.DeploymentEvidence != nil {
			if instanceFacts == nil {
				instanceFacts = &spec.InstanceFacts{}
				state.instanceFacts = instanceFacts
			}
			applyDeploymentEvidence(instanceFacts, request.DeploymentEvidence, resolvedVersion, request.TargetIdentity, request.Provider != nil)
		}
	}
	// The SERVER family privilege observation loads at most once per
	// request — under the request-selected conditional mode and only
	// while the batch names an audited identifier-form DROP SERVER, a
	// fully qualified CREATE SERVER (T26-SERVER-DROP-EXECUTION-01,
	// T26-SERVER-CREATE-EXECUTION-01), or a version-qualified audited
	// DROP SPATIAL REFERENCE SYSTEM (T26-SRS-DROP-PRIVILEGE-
	// EXECUTION-01). The raw account and grant lines stay request-level
	// immutable evidence; the positive-witness verdict binds principal,
	// domain version, and the token proof once, and every statement only
	// projects consumable/suppressed over the ordered scope — the SRS
	// family's projection is independent of every SERVER statement's
	// adjudication and marker. A failed read propagates under the same
	// bounded error contract as the object seam.
	if dialect == spec.DialectMySQL && request != nil &&
		request.PrivilegeAuditMode == spec.PrivilegeAuditModeObservedAccountNewSession &&
		(statementsNeedMySQLServerPrivilege(statements) || statementsNeedMySQLSRSPrivilege(statements) || statementsNeedMySQLAlterInstancePrivilege(statements)) {
		if loader, ok := state.provider.(MySQLServerDropPrivilegeLoader); state.provider != nil && ok {
			observation, err := loader.LoadMySQLServerDropPrivilege(ctx, dialect)
			if err != nil {
				return nil, serverPrivilegeReadError{cause: err}
			}
			state.serverDropPrivilege = observation
			state.serverDropPrivilegeWitness = spec.MySQLServerDropPrivilegeWitness(observation, resolvedVersion)
		}
	}
	// Object lookups resolve at most once per distinct (schema, type, name,
	// qualifiers) identity per request — the same object is never re-asked.
	objectCache := make(map[objectLookupKey]*spec.ObjectSnapshot)
	if request != nil {
		state.schemaResolver = func(ctx context.Context, name string) (*spec.ObjectSnapshot, error) {
			lookup := &spec.ObjectLookupRequest{Schema: requestSchema, Type: "schema", Name: name}
			cacheKey := newObjectLookupKey(lookup)
			if snapshot, ok := objectCache[cacheKey]; ok {
				return snapshot, nil
			}
			snapshot, err := resolveObjectSnapshot(ctx, request, dialect, lookup)
			if err != nil {
				// Propagate resolver failures — never cache a failed read
				// as an unavailable answer.
				return nil, err
			}
			objectCache[cacheKey] = snapshot
			return snapshot, nil
		}
		// Sequence and resource-group lookups share the same
		// once-per-request cache: the ordered binding and the
		// Metadata.Objects projection never ask the provider twice for the
		// same (schema, type, name) identity.
		cachedObjectResolver := func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
			cacheKey := newObjectLookupKey(&lookup)
			if snapshot, ok := objectCache[cacheKey]; ok {
				return snapshot, nil
			}
			snapshot, err := resolveObjectSnapshot(ctx, request, dialect, &lookup)
			if err != nil {
				return nil, err
			}
			objectCache[cacheKey] = snapshot
			return snapshot, nil
		}
		state.sequenceResolver = cachedObjectResolver
		state.resourceGroupResolver = cachedObjectResolver
		state.mysqlResourceGroupResolver = cachedObjectResolver
		state.tablespaceResolver = cachedObjectResolver
		state.mysqlServerResolver = cachedObjectResolver
		state.mysqlSRSResolver = cachedObjectResolver
	}
	// The ordered channel's deps-only reader is a direct capability of
	// the request's own provider — re-judged every enrich call so a
	// reused batchState never inherits a stale capability. The
	// forwarder is deliberately uncached: every consuming statement
	// must see a fresh dependency witness, so objectCache is bypassed
	// (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01).
	state.srsDepsReader = nil
	if reader, ok := state.provider.(MySQLSRSDependencyReader); ok {
		resolve := reader.ResolveMySQLSRSDependencies
		state.srsDepsReader = func(ctx context.Context, srid uint32) (*spec.SRSDependencyFact, error) {
			fact, err := resolve(ctx, srid, resolvedVersion)
			if err != nil {
				return nil, metadataObjectReadError{cause: err}
			}
			return fact, nil
		}
	}

	consumed := 0
	for i, statement := range statements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		enriched[i] = statement
		if consumed >= limits.orderedStatements {
			state.contaminated = true
			enriched[i].ResourceLimit = &spec.AuditResourceLimit{Limit: limits.orderedStatements, Consumed: consumed}
			enriched[i].Metadata = nil
			if resolvedVersion != nil {
				enriched[i].Metadata = &spec.Metadata{Version: resolvedVersion}
			}
			if statement.DML != nil {
				dml := *statement.DML
				dml.Impact = nil
				enriched[i].DML = &dml
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			continue
		}
		consumed++

		for _, failure := range failures {
			if failureBefore(failure.Line, failure.Column, statement.Line, statement.Column) {
				state.contaminated = true
				break
			}
		}

		// An authored grant/role statement (or the matching boundary
		// feature) conservatively ends the privilege observation's
		// applicable range for every LATER statement — the grant the
		// conditional audit relied on may no longer hold under a new
		// session (T26-SERVER-DROP-EXECUTION-01).
		if serverPrivilegeScopeTerminator(statement) {
			state.serverPrivilegeScopeEnded = true
		}

		if statement.DDL != nil && statement.DDL.View != nil && statement.DDL.View.Definition != nil &&
			statement.DDL.View.Definition.HasCTE && dialect == spec.DialectMySQL &&
			resolvedVersion != nil && resolvedVersion.Major == 5 {
			enriched[i].Unsupported = &spec.UnsupportedDetail{
				Feature: "view_query_cte", Reason: spec.UnsupportedVendorBoundaryReason,
			}
			enriched[i].Metadata = &spec.Metadata{Schema: requestSchema, Instance: instanceFacts, Version: resolvedVersion}
			continue
		}

		metadataSchema := metadataTargetSchema(request, statement)
		metadata := &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}

		if statement.DDL != nil && len(statement.DDL.TableTargets()) > 0 {
			// Namespace-aware binding projects every authored identity —
			// a temp-scoped DROP reads the session slot, a plain DROP reads
			// temp-first, a CREATE destination reads its own namespace —
			// through the same once-per-key seam. TargetTable stays the
			// primary binding's snapshot for single-target consumers.
			bindings, source, sources, err := state.boundTargetsForStatement(ctx, statement)
			if err != nil {
				if statement.DDL.View != nil {
					return nil, viewStateReadError{cause: err}
				}
				return nil, fmt.Errorf("load table snapshot: %w", err)
			}
			metadata.TargetTables = bindings
			metadata.SourceTable = source
			metadata.SourceTables = sources
			if len(bindings) > 0 {
				metadata.TargetTable = bindings[0].Snapshot
			}
			if statement.DDL.View != nil {
				view, err := state.projectViewMetadata(ctx, statement, metadata.TargetTable, bindings, sources)
				if err != nil {
					return nil, viewStateReadError{cause: err}
				}
				metadata.View = view
			}
		} else {
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
		}

		// An EXCHANGE counterpart is a second named identity the action
		// touches: it must be preloaded through the same once-per-key seam
		// so a first touch after the exchange never revives provider-stale
		// statistics, and so the counterpart rules see bounded facts. The
		// qualified counterpart keeps its own schema qualifier through
		// preState's effective-schema resolution.
		if counterpart := exchangeCounterpartTarget(statement); counterpart != nil &&
			strings.TrimSpace(counterpart.Name) != "" {
			snapshot, err := state.preState(ctx, metadataSchema, *counterpart)
			if err != nil {
				return nil, fmt.Errorf("load table snapshot for %s: %w", counterpart.Name, err)
			}
			metadata.ExchangeTable = snapshot
		}

		// ExchangeTable participates in the mount: a statement carrying only
		// a counterpart fact (no schema, no version, no provider target)
		// still keeps its metadata so the counterpart rule sees it.
		if metadata.Schema != "" || metadata.Instance != nil || metadata.TargetTable != nil || metadata.View != nil ||
			metadata.ExchangeTable != nil || metadata.Version != nil {
			enriched[i].Metadata = metadata
		}

		// A schema-scoped statement binds its ordered schema pre-state —
		// present with proven attributes, absent, or nil when unvouched —
		// through the same once-per-schema seam its apply transition uses.
		if schemaSnapshot, serr := state.projectSchemaTarget(ctx, statement); serr != nil {
			return nil, serr
		} else if schemaSnapshot != nil {
			if enriched[i].Metadata == nil {
				enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
			}
			enriched[i].Metadata.SchemaTarget = schemaSnapshot
		}

		if request != nil {
			if lookup := planObjectLookup(requestSchema, statement); lookup != nil {
				cacheKey := newObjectLookupKey(lookup)
				objSnapshot, cached := objectCache[cacheKey]
				if !cached {
					var oerr error
					objSnapshot, oerr = resolveObjectSnapshot(ctx, request, dialect, lookup)
					if oerr != nil {
						return nil, oerr
					}
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

		// Placement-policy facts attach the registry's ordered pre-state —
		// the existence and bound-set answer this statement adjudicates
		// against (T20/#99).
		if targets := spec.CollectPlacementBindings(enriched[i].DDL); len(targets) != 0 || isPlacementPolicyLifecycle(enriched[i].DDL) {
			state.seedPlacementCatalog()
			facts := spec.AssessPlacementBindings(dialect, targets, state.placementPolicyExists)
			fillPlacementTargetSchema(facts, requestSchema)
			if isPlacementPolicyLifecycle(enriched[i].DDL) {
				name := enriched[i].DDL.ObjectName
				facts.Policy = &spec.PlacementPolicyPremise{
					Name:         name,
					Exists:       copyBoolPtr(state.placementPolicyExists(name)),
					BoundTargets: state.placementBoundSet(name),
				}
			}
			if enriched[i].Metadata == nil {
				enriched[i].Metadata = &spec.Metadata{Schema: requestSchema, Instance: instanceFacts, Version: resolvedVersion}
			}
			enriched[i].Metadata.Placement = facts
		}
		if projection := state.placementPoliciesProjection(); projection != nil {
			if enriched[i].Metadata == nil {
				enriched[i].Metadata = &spec.Metadata{Schema: requestSchema, Instance: instanceFacts, Version: resolvedVersion}
			}
			enriched[i].Metadata.PlacementPolicies = projection
		}

		// Sequence facts attach the registry's ordered pre-state — the
		// existence and definition answer this statement adjudicates against
		// (T21/#100).
		if enriched[i].DDL != nil && (enriched[i].DDL.Sequence != nil ||
			enriched[i].DDL.Operation == spec.DDLOperationRenameTable) {
			sequences, serr := state.sequenceTargetsForStatement(ctx, enriched[i])
			if serr != nil {
				return nil, serr
			}
			if sequences != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Sequences = sequences
			}
		}

		// Resource-group facts attach the registry's ordered pre-state —
		// the existence and definition answer this statement adjudicates
		// against. TiDB-only: other dialects bind nothing so the parser
		// vendor boundary holds the CPU-family surface apart (T22/#101).
		if enriched[i].DDL != nil && enriched[i].DDL.ResourceGroup != nil && dialect == spec.DialectTiDB {
			groups, gerr := state.bindResourceGroups(ctx, enriched[i])
			if gerr != nil {
				return nil, gerr
			}
			if groups != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.ResourceGroups = groups
			}
		}

		// The MySQL CPU resource-group family binds its own registry —
		// version premise and reserved groups are adjudicated inside the
		// resolver, never guessed (T24/#103).
		if enriched[i].DDL != nil && enriched[i].DDL.MySQLResourceGroup != nil && dialect == spec.DialectMySQL {
			groups, gerr := state.bindMySQLResourceGroups(ctx, enriched[i])
			if gerr != nil {
				return nil, gerr
			}
			if groups != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.MySQLResourceGroups = groups
			}
		}

		// The MySQL tablespace family binds its own ordered registry —
		// existence and occupancy premises come from the batch's proven
		// entries only, never guessed (T25/#104).
		if enriched[i].DDL != nil && enriched[i].DDL.Tablespace != nil && dialect == spec.DialectMySQL {
			states, tsErr := state.bindTablespaces(ctx, enriched[i])
			if tsErr != nil {
				return nil, tsErr
			}
			if states != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Tablespaces = states
			}
		}

		// The MySQL SERVER family binds its existence fact — the object
		// resolver reads the catalog only when the authored name is a
		// provable identifier; a string-literal spelling stays an
		// unissued read, never a guessed identity (T26/#105 provider
		// round).
		if enriched[i].DDL != nil && enriched[i].DDL.Server != nil && dialect == spec.DialectMySQL {
			server, serr := state.bindMySQLServer(ctx, enriched[i])
			if serr != nil {
				return nil, serr
			}
			if server != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.Server = server
			}
			// The audited identifier-form drop carries the shared
			// adjudication — one truth the rule's gap/message/output
			// and the ordered channel's post-state both read
			// (T26-SERVER-DROP-EXECUTION-01).
			if decl := enriched[i].DDL.Server; decl.Kind == spec.ServerKindDrop && decl.NameIdent && enriched[i].Unsupported == nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				adjudication := spec.AdjudicateServerDrop(enriched[i].Metadata.Server, state.serverDropPrivilegePremise(), decl.IfExists)
				enriched[i].Metadata.ServerDrop = &adjudication
			}
			// The audited identifier-form create carries its own
			// shared adjudication — an independent premise projection
			// and owed set, never a borrowed drop premise
			// (T26-SERVER-CREATE-EXECUTION-01). The whole-batch
			// contamination flag reaches the arm table directly; it is
			// not projected through UnreadReason.
			if decl := enriched[i].DDL.Server; decl.Kind == spec.ServerKindCreate && decl.NameIdent && enriched[i].Unsupported == nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				adjudication := spec.AdjudicateServerCreate(enriched[i].Metadata.Server, state.serverCreatePrivilegePremise(), decl.OptionsClean(), state.contaminated)
				enriched[i].Metadata.ServerCreate = &adjudication
			}
			// The audited identifier-form alter carries its own
			// shared adjudication — an independent premise projection
			// and owed set, never a borrowed drop/create premise
			// (T26-SERVER-ALTER-EXECUTION-01). The whole-batch
			// contamination flag reaches the arm table directly, same
			// as the siblings.
			if decl := enriched[i].DDL.Server; decl.Kind == spec.ServerKindAlter && decl.NameIdent && enriched[i].Unsupported == nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				adjudication := spec.AdjudicateServerAlter(enriched[i].Metadata.Server, state.serverAlterPrivilegePremise(), decl.OptionsClean(), state.contaminated)
				enriched[i].Metadata.ServerAlter = &adjudication
			}
		}

		// The MySQL spatial reference system family binds its fact set —
		// existence and column dependencies read on the provable SRID;
		// NAME/ORGANIZATION conflict reads stay unissued while their
		// identities remain authored spellings (T26/#105 provider
		// round).
		if enriched[i].DDL != nil && enriched[i].DDL.SpatialReference != nil && dialect == spec.DialectMySQL {
			srs, serr := state.bindMySQLSRS(ctx, enriched[i])
			if serr != nil {
				return nil, serr
			}
			if srs != nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				enriched[i].Metadata.SpatialReference = srs
			}
			// The audited qualified drop carries the shared
			// adjudication — one truth the rule's gap/message/output
			// and the own apply's stale flags both read
			// (T26-SRS-DROP-EXECUTION-01). The authored IF EXISTS guard
			// enters the adjudication and splits the proven-absence arm
			// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01).
			if decl := enriched[i].DDL.SpatialReference; decl.Kind == spec.SRSKindDrop && enriched[i].Unsupported == nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				// The SRS premise is this statement's own projection
				// of the shared observation — never a SERVER
				// adjudication or marker (T26-SRS-DROP-PRIVILEGE-
				// EXECUTION-01).
				adjudication := spec.AdjudicateSRSDrop(enriched[i].Metadata.SpatialReference, decl.SRID, state.srsDepsStale, state.srsDropPrivilegePremise(), decl.IfExists)
				enriched[i].Metadata.SRSDrop = &adjudication
			}
			// The audited create family carries its own shared
			// adjudication — the same one-truth contract as the drop
			// slice, with CREATE's own privilege premise projection
			// (T26-SRS-CREATE-PUBLICATION-01 frozen F-7: never the
			// drop premise's semantic identity).
			if decl := enriched[i].DDL.SpatialReference; (decl.Kind == spec.SRSKindCreate || decl.Kind == spec.SRSKindCreateOrReplace) && enriched[i].Unsupported == nil {
				if enriched[i].Metadata == nil {
					enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
				}
				adjudication := spec.AdjudicateSRSCreate(enriched[i].Metadata.SpatialReference, decl, state.srsCreatePrivilegePremise())
				enriched[i].Metadata.SRSCreate = &adjudication
			}
		}
		// The audited ALTER INSTANCE family carries its own shared
		// adjudication — the same one-truth contract as the SRS slices,
		// with the statement's own privilege premise projection over the
		// shared observed-account evidence (frozen D3). The batch
		// contamination flag is read before this statement's own apply
		// marks the instance surface mutated.
		if enriched[i].DDL != nil && enriched[i].DDL.AlterInstance != nil && enriched[i].Unsupported == nil {
			if enriched[i].Metadata == nil {
				enriched[i].Metadata = &spec.Metadata{Schema: metadataSchema, Instance: instanceFacts, Version: resolvedVersion}
			}
			decl := enriched[i].DDL.AlterInstance
			adjudication := spec.AdjudicateAlterInstance(state.alterInstanceMutated, decl, state.alterInstancePrivilegePremise(decl.Form))
			enriched[i].Metadata.AlterInstance = &adjudication
		}

		if state.afterBindObserve != nil {
			state.afterBindObserve(i, &enriched[i])
		}
		state.applyStmtIndex = i
		if err := state.apply(ctx, enriched[i]); err != nil {
			return nil, err
		}
		if state.afterApplyObserve != nil {
			state.afterApplyObserve(i, &enriched[i])
		}
	}

	// A trailing parse failure after the last statement is still a real
	// batch failure record: it leaves no later statement to trip the
	// per-statement check, yet the batch's whole-effect stays
	// unknowable — consume it so a next call's bind reads the honest
	// contaminated flag (T26-SERVER-ALTER-EXECUTION-01 R2-C, AM-8.1).
	if len(failures) > 0 {
		state.contaminated = true
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return enriched, nil
}

// exchangeCounterpartTarget returns the authored EXCHANGE counterpart
// identity (schema qualifier preserved; the effective-schema fallback is
// applied inside preState), or nil when the statement carries no exchange
// action or the clause never named a table.
func exchangeCounterpartTarget(statement spec.Statement) *spec.Table {
	if statement.DDL == nil {
		return nil
	}
	for i := range statement.DDL.Alter {
		alter := &statement.DDL.Alter[i]
		if alter.Action == "exchange_partition" && alter.ExchangeTable != nil {
			return alter.ExchangeTable
		}
	}
	return nil
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
		case spec.DDLOperationCreateTable, spec.DDLOperationAlterTable, spec.DDLOperationDropTable, spec.DDLOperationTruncateTable,
			spec.DDLOperationSplitRegion, spec.DDLOperationCompactTable:
			if statement.Dialect == spec.DialectPostgreSQL {
				return statement.DDL.Table.Name
			}
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
	if statement.Dialect == spec.DialectPostgreSQL && statement.DDL != nil && statement.DDL.Table != nil && statement.DDL.Table.Schema != "" {
		return statement.DDL.Table.Schema
	}
	if statement.DML != nil {
		if targets := statement.DML.MutationTargetTables(); len(targets) > 0 {
			if schema := strings.TrimSpace(targets[0].Schema); schema != "" {
				return schema
			}
		}
	}
	if request == nil {
		return ""
	}
	if statement.Dialect == spec.DialectPostgreSQL && statement.DDL != nil {
		return request.Schema
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
	if _, kind := spec.PGViewOwnerSelection(statement); kind != "" {
		return nil // The exact view-owner read also checks source kind.
	}
	ddl := statement.DDL
	objType := objectTypeForOperation(ddl.Operation, ddl.ObjectType)
	if objType == "" {
		return nil
	}
	name := strings.TrimSpace(ddl.ObjectName)
	if name == "" && ddl.Operation == spec.DDLOperationAlterSchema && ddl.AlterDefaultSchema {
		// A name-less ALTER DATABASE binds the request's trusted schema —
		// the same identity the ordered state binds, never a guess.
		name = strings.TrimSpace(schema)
	}
	if name == "" {
		return nil
	}
	req := &spec.ObjectLookupRequest{
		Schema: schema,
		Type:   objType,
		Name:   name,
	}
	if ddl.MySQLResourceGroup != nil {
		// The MySQL CPU family resolves its own catalog object type —
		// never the TiDB RU resource_group lookup (T24/#103).
		req.Type = "mysql_resource_group"
	}
	// A qualified sequence target resolves under its authored schema — the
	// qualifier wins over the request schema everywhere it is read.
	if ddl.Sequence != nil {
		if authored := strings.TrimSpace(ddl.Sequence.Schema); authored != "" {
			req.Schema = authored
		}
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
	case spec.DDLOperationCreateResourceGroup, spec.DDLOperationAlterResourceGroup, spec.DDLOperationDropResourceGroup:
		return "resource_group"
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
	if key.objectType == "mysql_tablespace" {
		// Tablespace names are case-sensitive identifiers — folding the
		// cache key would alias TS1 and ts1 into one catalog answer.
		key.name = strings.TrimSpace(lookup.Name)
	}
	if key.objectType == "mysql_server" {
		// Server names are decoded identifiers — the verbatim decoded
		// spelling is the only provable cache identity: a fold or trim
		// would alias ` s1` and `s1` into one catalog answer, and the
		// bound state must echo the statement's own decoded name
		// (T26/#105).
		key.name = lookup.Name
	}
	if len(lookup.Qualifiers) > 0 {
		names := make([]string, 0, len(lookup.Qualifiers))
		for name := range lookup.Qualifiers {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		for _, name := range names {
			// Length-prefix each member — a qualifier value may carry
			// '=' or ';', so raw name=value; concatenation aliases
			// distinct qualifier sets into one cache identity (T26/#105).
			fmt.Fprintf(&b, "%d:%s=%d:%s;", len(name), name, len(lookup.Qualifiers[name]), lookup.Qualifiers[name])
		}
		key.qualifiers = b.String()
	}
	return key
}

// resolveObjectSnapshot calls the ObjectResolver if available, or returns
// an unavailable snapshot as fallback when no resolver exists. Resolver
// failures propagate — a live read error is never downgraded to Unavailable:
// that status means "no resolver could answer", not "the answer failed".
func resolveObjectSnapshot(ctx context.Context, request *MetadataRequest, dialect spec.Dialect, lookup *spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if request == nil || request.Provider == nil {
		return &spec.ObjectSnapshot{
			Schema: lookup.Schema,
			Type:   lookup.Type,
			Name:   lookup.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	resolver, ok := request.Provider.(ObjectResolver)
	if !ok {
		return &spec.ObjectSnapshot{
			Schema: lookup.Schema,
			Type:   lookup.Type,
			Name:   lookup.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	snapshot, err := resolver.ResolveObject(ctx, dialect, *lookup)
	if err != nil {
		return nil, metadataObjectReadError{cause: err}
	}
	return snapshot, nil
}

// metadataObjectReadError bounds the public text of one failed object
// read: Error() carries a fixed cause-free message — provider text,
// driver detail, and authored values never cross into the public error
// — while Unwrap keeps the original cause reachable for errors.Is/As
// and for the existing cancellation/timeout recognition on every
// adapter. The read still fails: it is never laundered into an unknown
// answer, an evidence gap, or a cached absence (T26/#105).
type metadataObjectReadError struct{ cause error }

func (metadataObjectReadError) Error() string   { return "metadata object read failed" }
func (e metadataObjectReadError) Unwrap() error { return e.cause }
