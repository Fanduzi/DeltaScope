// Package audit provides shared SQL-corpus test helpers.
package audit

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// corpusConfigPath writes an optional inline corpus config to a temporary file
// and returns the path used by AuditSQL.
func corpusConfigPath(t *testing.T, tc corpusExpected) string {
	t.Helper()
	if len(tc.Config) == 0 {
		return ""
	}

	raw, err := yaml.Marshal(tc.Config)
	if err != nil {
		t.Fatalf("marshal corpus config: %v", err)
	}

	path := filepath.Join(t.TempDir(), "deltascope-corpus.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write corpus config: %v", err)
	}
	return path
}

func corpusMetadataFields(tc corpusExpected) (string, MetadataProvider) {
	if tc.Metadata == nil {
		return "", nil
	}
	if tc.Metadata.Instance == nil && len(tc.Metadata.Tables) == 0 && len(tc.Metadata.IndexOwners) == 0 &&
		len(tc.Metadata.PlacementPolicies) == 0 && len(tc.Metadata.Sequences) == 0 && len(tc.Metadata.ResourceGroups) == 0 &&
		len(tc.Metadata.MySQLStoredObjects) == 0 && len(tc.Metadata.PGCheckNumericCasts) == 0 && len(tc.Metadata.MySQLResourceGroups) == 0 && len(tc.Metadata.Tablespaces) == 0 && len(tc.Metadata.SpatialReferences) == 0 && len(tc.Metadata.PGReplicaIndexes) == 0 && len(tc.Metadata.PGTableTablespaces) == 0 && len(tc.Metadata.PGTableAccessMethods) == 0 && len(tc.Metadata.PGTablePersistences) == 0 && len(tc.Metadata.PGTableNamespaces) == 0 && len(tc.Metadata.PGTableOwners) == 0 && len(tc.Metadata.PGBulkTablespaces) == 0 && len(tc.Metadata.PGColumnTypes) == 0 && len(tc.Metadata.PGConstraintIndexes) == 0 && len(tc.Metadata.PGForeignKeyDirectories) == 0 && len(tc.Metadata.PGDropConstraintReferences) == 0 && len(tc.Metadata.PGDropColumnDependencies) == 0 && len(tc.Metadata.PGColumnTraversals) == 0 && len(tc.Metadata.PGDestructiveReferences) == 0 && len(tc.Metadata.PGRefreshStates) == 0 && len(tc.Metadata.PGCreateStorage) == 0 && len(tc.Metadata.PGCreateTyped) == 0 && len(tc.Metadata.PGCreateCommits) == 0 && len(tc.Metadata.PGCreateLikes) == 0 && len(tc.Metadata.PGOperatorFamilies) == 0 && len(tc.Metadata.PGIndexIncludes) == 0 && len(tc.Metadata.PGReplicationTargets) == 0 && len(tc.Metadata.PGLanguages) == 0 && len(tc.Metadata.PGGrants) == 0 && len(tc.Metadata.PGIndexDestinations) == 0 && len(tc.Metadata.PGIndexOptions) == 0 && len(tc.Metadata.PGIndexAttachments) == 0 && len(tc.Metadata.PGOperatorChanges) == 0 && len(tc.Metadata.PGComposites) == 0 && len(tc.Metadata.PGEnums) == 0 && len(tc.Metadata.PGCollationChanges) == 0 && len(tc.Metadata.PGCollationRefreshes) == 0 && len(tc.Metadata.PGPolicies) == 0 && len(tc.Metadata.PGSequenceChanges) == 0 && len(tc.Metadata.PGForeignTables) == 0 && len(tc.Metadata.PGWrappers) == 0 && len(tc.Metadata.PGTextSearch) == 0 && len(tc.Metadata.PGCreateSchemas) == 0 && len(tc.Metadata.PGPartitionConstraints) == 0 && len(tc.Metadata.PGDropViewStates) == 0 && len(tc.Metadata.PGMaterializedViewExtensions) == 0 && len(tc.Metadata.PGViewDefaults) == 0 {
		return strings.TrimSpace(tc.Metadata.Schema), nil
	}
	return strings.TrimSpace(tc.Metadata.Schema), newCorpusMetadataProvider(tc.Metadata)
}

type corpusFixtureMetadataProvider struct {
	mysqlStoredObjects           []corpusMySQLStoredObject
	pgCheckNumericCasts          map[string]bool
	pgMaterializedViewExtensions []corpusPGMaterializedViewExtensionState
	pgViewDefaults               []corpusPGViewDefaultState
	pgCreateStorage              []corpusPGCreateStorage
	pgCreateTyped                []corpusPGCreateTyped
	pgCreateCommits              []corpusPGCreateCommit
	pgCreateLikes                []corpusPGCreateLike
	pgOperatorFamilies           []corpusPGOperatorFamily
	pgIndexIncludes              []corpusPGIndexInclude
	pgReplicationTargets         []corpusPGReplicationTarget
	pgLanguages                  []corpusPGLanguage
	pgGrants                     []corpusPGGrant
	pgIndexDestinations          []corpusPGIndexDestination
	pgIndexOptions               []corpusPGIndexOptions
	pgIndexAttachments           []corpusPGIndexAttachment
	pgCollationChanges           []corpusPGCollationChange
	pgCollationRefreshes         []corpusPGCollationRefresh
	pgOperatorChanges            []corpusPGOperatorChange
	pgComposites                 []corpusPGComposite
	pgEnums                      []corpusPGEnum
	pgPolicies                   []corpusPGPolicy
	pgSequenceChanges            []corpusPGSequenceChange
	pgForeignTables              []corpusPGForeignTable
	pgWrappers                   []corpusPGWrapper
	pgTextSearch                 []corpusPGTextSearch
	pgCreateSchemas              []corpusPGCreateSchema
	pgPartitionConstraints       []corpusPGPartitionConstraint
	pgDropViewStates             []corpusPGDropViewState
	instance                     *spec.InstanceFacts
	tables                       map[string]*spec.TableSnapshot
	indexOwners                  map[string]string
	pgTableOwners                []corpusPGTableOwner
	pgDropColumnDependencies     []corpusPGDropColumnDependency
	pgColumnTraversals           []corpusPGColumnTraversal
	pgDropConstraintReferences   []corpusPGDropConstraintReference
	pgDestructiveReferences      []corpusPGDestructiveReference
	pgRefreshStates              []corpusPGRefreshState
	pgForeignKeyDirectories      []corpusPGForeignKeyKeys
	pgConstraintIndexes          []corpusPGConstraintIndex
	pgColumnTypes                []corpusPGColumnType
	pgBulkTablespaces            []corpusPGBulkTablespace
	pgTableNamespaces            []corpusPGTableNamespace
	pgTablePersistences          []corpusPGTablePersistence
	pgTableAccessMethods         []corpusPGTableAccessMethod
	pgTableTablespaces           []corpusPGTableTablespace
	pgReplicaIndexes             []corpusPGReplicaIndex
	// placement nil means the catalog is unvouched; a non-nil map is the
	// proven policy set (empty map = proven-empty).
	placement map[string]*spec.PlacementPolicyFact
	// sequences nil means every sequence identity is unvouched; entries are
	// keyed `schema.name` and carry the catalog's existence answer plus —
	// when proven — definition parameters and the bounded reference read.
	sequences map[string]*spec.ObjectSnapshot
	// resourceGroups nil means every resource-group identity is unvouched;
	// entries are keyed `name` and carry the catalog's existence answer
	// plus — when proven — definition parameters (T22/#101).
	resourceGroups map[string]*spec.ObjectSnapshot
	// mysqlResourceGroups is the MySQL CPU resource-group catalog fact set
	// — keyed `name`, carrying the information_schema.RESOURCE_GROUPS
	// answer (T24/#103).
	mysqlResourceGroups map[string]*spec.ObjectSnapshot
	// tablespaces is the MySQL tablespace catalog fact set — keyed `name`,
	// carrying the INNODB_TABLESPACES existence answer plus the bounded
	// bound-table occupancy read (T25/#104).
	tablespaces map[string]*spec.ObjectSnapshot
	// spatialReferences is the MySQL SRS catalog fact set — keyed by the
	// decimal SRID, carrying the st_spatial_reference_systems existence
	// answer plus the bounded st_geometry_columns dependency witnesses
	// (T26-SRS-DROP-EXECUTION-01).
	spatialReferences map[string]*spec.ObjectSnapshot
	// spatialReferenceSeeds keeps the raw fixture objects so the
	// CREATE conflict reads can synthesize NAME / ORGANIZATION+
	// IDENTIFIED BY witness sets by scanning every seeded row against
	// the request qualifiers (T26-SRS-CREATE-PUBLICATION-01).
	spatialReferenceSeeds map[string]corpusSRSObject
}

func newCorpusMetadataProvider(metadata *corpusMetadata) *corpusFixtureMetadataProvider {
	provider := &corpusFixtureMetadataProvider{
		pgCheckNumericCasts:          metadata.PGCheckNumericCasts,
		tables:                       make(map[string]*spec.TableSnapshot, len(metadata.Tables)),
		indexOwners:                  make(map[string]string, len(metadata.IndexOwners)),
		pgTableOwners:                metadata.PGTableOwners,
		pgDropColumnDependencies:     metadata.PGDropColumnDependencies,
		pgColumnTraversals:           metadata.PGColumnTraversals,
		pgDropConstraintReferences:   metadata.PGDropConstraintReferences,
		pgDestructiveReferences:      metadata.PGDestructiveReferences,
		pgRefreshStates:              metadata.PGRefreshStates,
		pgCreateStorage:              metadata.PGCreateStorage,
		pgCreateTyped:                metadata.PGCreateTyped,
		pgCreateCommits:              metadata.PGCreateCommits,
		pgCreateLikes:                metadata.PGCreateLikes,
		pgOperatorFamilies:           metadata.PGOperatorFamilies,
		pgIndexIncludes:              metadata.PGIndexIncludes,
		pgReplicationTargets:         metadata.PGReplicationTargets,
		pgLanguages:                  metadata.PGLanguages,
		pgGrants:                     metadata.PGGrants,
		pgIndexDestinations:          metadata.PGIndexDestinations,
		pgIndexOptions:               metadata.PGIndexOptions,
		pgIndexAttachments:           metadata.PGIndexAttachments,
		pgCollationChanges:           metadata.PGCollationChanges,
		pgCollationRefreshes:         metadata.PGCollationRefreshes,
		pgOperatorChanges:            metadata.PGOperatorChanges,
		pgComposites:                 metadata.PGComposites,
		pgEnums:                      metadata.PGEnums,
		pgPolicies:                   metadata.PGPolicies,
		pgSequenceChanges:            metadata.PGSequenceChanges,
		pgForeignTables:              metadata.PGForeignTables,
		pgWrappers:                   metadata.PGWrappers,
		pgTextSearch:                 metadata.PGTextSearch,
		mysqlStoredObjects:           metadata.MySQLStoredObjects,
		pgCreateSchemas:              metadata.PGCreateSchemas,
		pgPartitionConstraints:       metadata.PGPartitionConstraints,
		pgDropViewStates:             metadata.PGDropViewStates,
		pgMaterializedViewExtensions: metadata.PGMaterializedViewExtensions,
		pgViewDefaults:               metadata.PGViewDefaults,
		pgForeignKeyDirectories:      metadata.PGForeignKeyDirectories,
		pgConstraintIndexes:          metadata.PGConstraintIndexes,
		pgColumnTypes:                metadata.PGColumnTypes,
		pgBulkTablespaces:            metadata.PGBulkTablespaces,
		pgTableNamespaces:            metadata.PGTableNamespaces,
		pgTablePersistences:          metadata.PGTablePersistences,
		pgTableAccessMethods:         metadata.PGTableAccessMethods,
		pgTableTablespaces:           metadata.PGTableTablespaces,
		pgReplicaIndexes:             metadata.PGReplicaIndexes,
	}
	if metadata.Instance != nil {
		provider.instance = &spec.InstanceFacts{
			Version:                            metadata.Instance.Version,
			PGCompressionLZ4Known:              metadata.Instance.PGCompressionLZ4Known,
			PGCompressionLZ4Supported:          metadata.Instance.PGCompressionLZ4Supported,
			DefaultCharset:                     metadata.Instance.DefaultCharset,
			InnoDBLargePrefixEnabled:           metadata.Instance.InnoDBLargePrefixEnabled,
			InnoDBLargePrefixKnown:             metadata.Instance.InnoDBLargePrefixKnown,
			InnoDBDefaultRowFormat:             metadata.Instance.InnoDBDefaultRowFormat,
			InnoDBAdaptiveHashEnabled:          metadata.Instance.InnoDBAdaptiveHashEnabled,
			TiDBEnableCheckConstraint:          metadata.Instance.TiDBEnableCheckConstraint,
			TiDBEnableCheckConstraintKnown:     metadata.Instance.TiDBEnableCheckConstraintKnown,
			TiDBEnableResourceControl:          metadata.Instance.TiDBEnableResourceControl,
			TiDBEnableResourceControlKnown:     metadata.Instance.TiDBEnableResourceControlKnown,
			TiDBEnableClusteredIndex:           metadata.Instance.TiDBEnableClusteredIndex,
			TiDBAlterPrimaryKey:                metadata.Instance.TiDBAlterPrimaryKey,
			TiDBAlterPrimaryKeyKnown:           metadata.Instance.TiDBAlterPrimaryKeyKnown,
			MySQLResourceGroupFeatureSupported: metadata.Instance.MySQLResourceGroupFeatureSupported,
			MySQLResourceGroupFeatureKnown:     metadata.Instance.MySQLResourceGroupFeatureKnown,
			MySQLResourceGroupPriorityCapable:  metadata.Instance.MySQLResourceGroupPriorityCapable,
			MySQLResourceGroupPriorityCapKnown: metadata.Instance.MySQLResourceGroupPriorityCapKnown,
			MySQLHostVCPUs:                     metadata.Instance.MySQLHostVCPUs,
			MySQLHostVCPUsKnown:                metadata.Instance.MySQLHostVCPUsKnown,
			MySQLResourceGroupAdmin:            metadata.Instance.MySQLResourceGroupAdmin,
			MySQLResourceGroupAdminKnown:       metadata.Instance.MySQLResourceGroupAdminKnown,
			MySQLResourceGroupBound:            metadata.Instance.MySQLResourceGroupBound,
			MySQLResourceGroupBoundComplete:    metadata.Instance.MySQLResourceGroupBoundComplete,
			MySQLReadOnly:                      metadata.Instance.MySQLReadOnly,
			MySQLReadOnlyKnown:                 metadata.Instance.MySQLReadOnlyKnown,
			MySQLSuperReadOnly:                 metadata.Instance.MySQLSuperReadOnly,
			MySQLSuperReadOnlyKnown:            metadata.Instance.MySQLSuperReadOnlyKnown,
			MySQLReadOnlyExempt:                metadata.Instance.MySQLReadOnlyExempt,
			MySQLReadOnlyExemptKnown:           metadata.Instance.MySQLReadOnlyExemptKnown,
		}
	}
	for tableName, snapshot := range metadata.Tables {
		converted := corpusTableSnapshotToSpec(tableName, metadata.Schema, snapshot)
		provider.tables[strings.ToLower(strings.TrimSpace(tableName))] = &converted
	}
	for indexName, tableName := range metadata.IndexOwners {
		provider.indexOwners[strings.ToLower(strings.TrimSpace(indexName))] = strings.TrimSpace(tableName)
	}
	if metadata.PlacementPolicies != nil {
		provider.placement = make(map[string]*spec.PlacementPolicyFact, len(metadata.PlacementPolicies))
		for name, policy := range metadata.PlacementPolicies {
			provider.placement[strings.ToLower(strings.TrimSpace(name))] = &spec.PlacementPolicyFact{
				PrimaryRegion: policy.PrimaryRegion,
				Regions:       policy.Regions,
				BoundTargets:  append([]string{}, policy.BoundTargets...),
			}
		}
	}
	if metadata.Sequences != nil {
		provider.sequences = make(map[string]*spec.ObjectSnapshot, len(metadata.Sequences))
		for key, object := range metadata.Sequences {
			provider.sequences[strings.ToLower(strings.TrimSpace(key))] = corpusSequenceToSnapshot(key, metadata.Schema, object)
		}
	}
	if metadata.ResourceGroups != nil {
		provider.resourceGroups = make(map[string]*spec.ObjectSnapshot, len(metadata.ResourceGroups))
		for key, object := range metadata.ResourceGroups {
			provider.resourceGroups[strings.ToLower(strings.TrimSpace(key))] = corpusResourceGroupToSnapshot(key, object)
		}
	}
	if metadata.MySQLResourceGroups != nil {
		provider.mysqlResourceGroups = make(map[string]*spec.ObjectSnapshot, len(metadata.MySQLResourceGroups))
		for key, object := range metadata.MySQLResourceGroups {
			provider.mysqlResourceGroups[strings.ToLower(strings.TrimSpace(key))] = corpusMySQLResourceGroupToSnapshot(key, object)
		}
	}
	if metadata.Tablespaces != nil {
		provider.tablespaces = make(map[string]*spec.ObjectSnapshot, len(metadata.Tablespaces))
		for key, object := range metadata.Tablespaces {
			// Tablespace names are case-sensitive — fixture keys stay verbatim.
			provider.tablespaces[strings.TrimSpace(key)] = corpusTablespaceToSnapshot(key, object)
		}
	}
	if metadata.SpatialReferences != nil {
		provider.spatialReferences = make(map[string]*spec.ObjectSnapshot, len(metadata.SpatialReferences))
		provider.spatialReferenceSeeds = make(map[string]corpusSRSObject, len(metadata.SpatialReferences))
		for key, object := range metadata.SpatialReferences {
			provider.spatialReferences[strings.TrimSpace(key)] = corpusSRSToSnapshot(key, object)
			provider.spatialReferenceSeeds[strings.TrimSpace(key)] = object
		}
	}
	return provider
}

// corpusSRSToSnapshot converts a fixture SRS entry into the
// st_spatial_reference_systems + st_geometry_columns answer the object
// resolver would return — the status word is the provenance answer;
// dependency witnesses bind only on a read row (T26-SRS-DROP-EXECUTION-01).
func corpusSRSToSnapshot(key string, object corpusSRSObject) *spec.ObjectSnapshot {
	snapshot := &spec.ObjectSnapshot{Type: "mysql_spatial_reference_system", Name: strings.TrimSpace(key), Status: spec.MetadataStatus(object.Status)}
	if object.Exists != nil {
		snapshot.Exists = *object.Exists
	} else {
		snapshot.Exists = object.Status == "confirmed"
	}
	srid, err := strconv.ParseUint(strings.TrimSpace(key), 10, 32)
	if err != nil {
		return snapshot
	}
	state := &spec.SpatialReferenceSystemState{SRID: uint32(srid), Read: true}
	state.Exists = snapshot.Exists
	if snapshot.Exists {
		state.Status = spec.MetadataStatusConfirmed
	}
	fact := &spec.SRSDependencyFact{}
	for _, dep := range object.Dependencies {
		fact.Columns = append(fact.Columns, spec.SRSDependentColumn{
			Schema: dep.Schema, Table: dep.Table, Column: dep.Column, SRID: dep.SRID,
		})
	}
	state.Dependencies = fact
	snapshot.SpatialReference = state
	return snapshot
}

// corpusTablespaceToSnapshot converts a fixture tablespace entry into the
// INNODB_TABLESPACES answer the object resolver would return — the status
// word is the provenance answer; kind and occupied members stay unproven
// when the fixture omits them.
func corpusTablespaceToSnapshot(key string, object corpusTablespaceObject) *spec.ObjectSnapshot {
	snapshot := &spec.ObjectSnapshot{Type: "mysql_tablespace", Name: strings.TrimSpace(key), Status: spec.MetadataStatus(object.Status)}
	if object.Exists != nil {
		snapshot.Exists = *object.Exists
	} else {
		snapshot.Exists = object.Status == "confirmed"
	}
	if object.Kind != "" || object.Occupied != nil {
		snapshot.Tablespace = &spec.TablespaceState{Kind: object.Kind, Occupied: object.Occupied}
	}
	return snapshot
}

// corpusMySQLResourceGroupToSnapshot converts a fixture MySQL
// resource-group entry into the information_schema.RESOURCE_GROUPS answer
// the object resolver would return — the status word is the provenance
// answer; omitted definition members stay unproven.
func corpusMySQLResourceGroupToSnapshot(key string, object corpusMySQLResourceGroupObject) *spec.ObjectSnapshot {
	snapshot := &spec.ObjectSnapshot{Type: "mysql_resource_group", Name: key, Status: spec.MetadataStatus(object.Status)}
	if object.Exists != nil {
		snapshot.Exists = *object.Exists
	} else {
		snapshot.Exists = object.Status == "confirmed"
	}
	if object.Definition != nil {
		snapshot.MySQLResourceGroup = &spec.MySQLResourceGroupDefinition{
			GroupType:      spec.MySQLResourceGroupType(object.Definition.GroupType),
			Enabled:        object.Definition.Enabled,
			VCPUIDs:        object.Definition.VCPUIDs,
			ThreadPriority: object.Definition.ThreadPriority,
		}
	}
	return snapshot
}

// corpusResourceGroupToSnapshot converts a fixture resource-group entry
// into the catalog answer the object resolver would return. The status
// word is the provenance answer; a missing or empty group name keeps the
// identity blank rather than fabricating one.
func corpusResourceGroupToSnapshot(key string, object corpusResourceGroupObject) *spec.ObjectSnapshot {
	snapshot := &spec.ObjectSnapshot{
		Name: strings.TrimSpace(key),
		Type: "resource_group",
	}
	switch strings.ToLower(strings.TrimSpace(object.Status)) {
	case "confirmed":
		snapshot.Status = spec.MetadataStatusConfirmed
		snapshot.Exists = true
	case "not_found":
		snapshot.Status = spec.MetadataStatusNotFound
	default:
		snapshot.Status = spec.MetadataStatusUnavailable
	}
	if object.Exists != nil {
		snapshot.Exists = *object.Exists
	}
	if object.Definition != nil {
		snapshot.ResourceGroup = &spec.ResourceGroupDefinition{
			RUPerSec:    object.Definition.RUPerSec,
			RUUnlimited: object.Definition.RUUnlimited,
			Priority:    object.Definition.Priority,
			Burstable:   object.Definition.Burstable,
			QueryLimit:  object.Definition.QueryLimit,
		}
		// Mirror the provider's catalog contract: QUERY_LIMIT text means a
		// configured rule; NULL on a complete RU-mode row is confirmed
		// absent; an incomplete row proves nothing (T22/#101).
		rowComplete := (snapshot.ResourceGroup.RUPerSec != nil || snapshot.ResourceGroup.RUUnlimited) &&
			snapshot.ResourceGroup.Priority != "" && snapshot.ResourceGroup.Burstable != nil
		switch {
		case object.Definition.QueryLimit != nil:
			snapshot.ResourceGroup.QueryLimitState = spec.ResourceGroupQueryLimitConfigured
		case rowComplete:
			snapshot.ResourceGroup.QueryLimitState = spec.ResourceGroupQueryLimitConfirmedAbsent
		default:
			snapshot.ResourceGroup.QueryLimitState = spec.ResourceGroupQueryLimitUnread
		}
	}
	return snapshot
}

// corpusSequenceToSnapshot converts a fixture sequence entry into the
// catalog answer the object resolver would return. A conflict_kind marks
// the identity bound to a non-sequence object (sequences share the table
// namespace); otherwise the status word is the provenance answer.
func corpusSequenceToSnapshot(key, defaultSchema string, object corpusSequenceObject) *spec.ObjectSnapshot {
	schema, name := defaultSchema, strings.TrimSpace(key)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		schema, name = strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+1:])
	}
	snapshot := &spec.ObjectSnapshot{
		Schema: schema,
		Name:   name,
		Type:   "sequence",
	}
	switch strings.ToLower(strings.TrimSpace(object.Status)) {
	case "confirmed":
		snapshot.Status = spec.MetadataStatusConfirmed
		snapshot.Exists = true
	case "not_found":
		snapshot.Status = spec.MetadataStatusNotFound
	default:
		snapshot.Status = spec.MetadataStatusUnavailable
	}
	if object.Exists != nil {
		snapshot.Exists = *object.Exists
	}
	if object.ConflictKind != "" {
		snapshot.Type = object.ConflictKind
	}
	if object.Definition != nil {
		snapshot.Sequence = &spec.SequenceDefinition{
			Start:      object.Definition.Start,
			Increment:  object.Definition.Increment,
			MinValue:   object.Definition.MinValue,
			MaxValue:   object.Definition.MaxValue,
			CacheValue: object.Definition.CacheValue,
			Cache:      object.Definition.Cache,
			Cycle:      object.Definition.Cycle,
		}
	}
	if object.References != nil {
		edges := make([]spec.SequenceRefEdge, 0, len(object.References))
		for _, ref := range object.References {
			edges = append(edges, spec.SequenceRefEdge{
				OwnerKind:    ref.OwnerKind,
				OwnerSchema:  ref.OwnerSchema,
				OwnerName:    ref.OwnerName,
				Slot:         ref.Slot,
				TargetSchema: snapshot.Schema,
				TargetName:   snapshot.Name,
			})
		}
		snapshot.SequenceReferences = edges
	}
	snapshot.SequenceReferencesUnknown = object.ReferencesUnknown
	return snapshot
}

func (p *corpusFixtureMetadataProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return p.instance, nil
}

func (p *corpusFixtureMetadataProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	snapshot, ok := p.tables[strings.ToLower(strings.TrimSpace(table))]
	if !ok {
		return nil, nil
	}
	if snapshot.Schema == "" {
		snapshot.Schema = schema
	}
	if snapshot.Table == nil {
		snapshot.Table = &spec.Table{Schema: snapshot.Schema, Name: table}
	}
	return snapshot, nil
}

func (p *corpusFixtureMetadataProvider) ResolveTableForIndex(_ context.Context, _ spec.Dialect, _ string, index string) (string, error) {
	return p.indexOwners[strings.ToLower(strings.TrimSpace(index))], nil
}

// LoadPlacementPolicies serves the fixture's proven policy catalog — nil
// keeps the catalog unvouched, never a fabricated empty set.
func (p *corpusFixtureMetadataProvider) LoadPlacementPolicies(context.Context, spec.Dialect) (map[string]*spec.PlacementPolicyFact, error) {
	return p.placement, nil
}

// srsConflictFacts attaches the CREATE family's qualified conflict
// reads to the resolved snapshot — the NAME and ORGANIZATION+
// IDENTIFIED BY witness sets scanned across every seeded row. The
// snapshot's SRS state is cloned first so per-request facts never
// mutate the shared fixture answer; a qualifier-less request (DROP)
// returns the snapshot untouched. A zero-match scan is a fact with
// empty MatchedSRIDs — read, zero rows — never a proven "no conflict".
func (p *corpusFixtureMetadataProvider) srsConflictFacts(snapshot *spec.ObjectSnapshot, request spec.ObjectLookupRequest) *spec.ObjectSnapshot {
	nameQualifier := strings.TrimSpace(request.Qualifiers[spec.ObjectLookupQualifierSRSName])
	orgQualifier := strings.TrimSpace(request.Qualifiers[spec.ObjectLookupQualifierSRSOrg])
	orgIDQualifier := strings.TrimSpace(request.Qualifiers[spec.ObjectLookupQualifierSRSOrgID])
	if nameQualifier == "" && (orgQualifier == "" || orgIDQualifier == "") {
		return snapshot
	}
	var orgID uint64
	var orgIDOK bool
	if orgQualifier != "" && orgIDQualifier != "" {
		orgID, orgIDOK = parseCorpusUint(orgIDQualifier)
	}
	var nameFact, orgFact *spec.SRSConflictFact
	if nameQualifier != "" {
		nameFact = &spec.SRSConflictFact{}
	}
	if orgQualifier != "" && orgIDQualifier != "" && orgIDOK {
		orgFact = &spec.SRSConflictFact{}
	}
	for key, object := range p.spatialReferenceSeeds {
		srid, ok := parseCorpusUint(key)
		if !ok {
			continue
		}
		if nameFact != nil && object.Name == nameQualifier {
			nameFact.MatchedSRIDs = append(nameFact.MatchedSRIDs, uint32(srid))
		}
		if orgFact != nil && object.Organization == orgQualifier &&
			object.OrganizationCoordsysID != nil && uint64(*object.OrganizationCoordsysID) == orgID {
			orgFact.MatchedSRIDs = append(orgFact.MatchedSRIDs, uint32(srid))
		}
	}
	if nameFact == nil && orgFact == nil {
		return snapshot
	}
	out := *snapshot
	state := &spec.SpatialReferenceSystemState{}
	if snapshot.SpatialReference != nil {
		*state = *snapshot.SpatialReference
	}
	if nameFact != nil {
		state.NameConflict = nameFact
	}
	if orgFact != nil {
		state.OrgConflict = orgFact
	}
	out.SpatialReference = state
	return &out
}

// parseCorpusUint parses one fixture key — the decimal SRID spellings
// the corpus uses.
func parseCorpusUint(text string) (uint64, bool) {
	n, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
	return n, err == nil
}

// ResolveObject serves the fixture's sequence object answers — keyed
// `schema.name`. An unkeyed identity or a nil sequence map is unvouched,
// never a silent absent; non-sequence lookups stay unavailable.
func (p *corpusFixtureMetadataProvider) ResolveObject(_ context.Context, _ spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if request.Type == "mysql_stored_object" {
		for _, seed := range p.mysqlStoredObjects {
			if request.Schema == seed.Schema && request.Name == seed.Name && request.Qualifiers["kind"] == seed.Kind {
				target := spec.MySQLStoredObjectTarget{Kind: seed.Kind, Name: spec.Table{Schema: seed.Schema, Name: seed.Name}}
				return &spec.ObjectSnapshot{MySQLStoredObject: &spec.MySQLStoredObjectState{Target: target, SourceSeen: true}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_check_integer_coercion" {
		if allowed, known := p.pgCheckNumericCasts[request.Name]; known {
			return &spec.ObjectSnapshot{PGCheckIntegerCoercion: &spec.PGCheckIntegerCoercion{Source: request.Name, Allowed: allowed}}, nil
		}
		return nil, nil
	}
	if request.Type == "materialized_view_extension" {
		for _, seed := range p.pgMaterializedViewExtensions {
			if seed.Schema == request.Schema && seed.View == request.Name && seed.Extension == request.Qualifiers["extension"] {
				return &spec.ObjectSnapshot{PGMaterializedViewExtension: &spec.PGMaterializedViewExtensionState{Schema: seed.Schema, View: seed.View, Extension: seed.Extension, Kind: seed.Kind, ExtensionExists: seed.ExtensionExists}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "view_column_default" {
		for _, entry := range p.pgViewDefaults {
			state := spec.PGViewDefaultState(entry)
			if state.MatchesViewDefault(&request) {
				return &spec.ObjectSnapshot{PGViewDefault: &state}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_create_storage" {
		for _, seed := range p.pgCreateStorage {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Method == request.Qualifiers["method"] {
				return &spec.ObjectSnapshot{PGCreateStorage: &spec.PGCreateStorageState{
					Target:       spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					SchemaExists: true, Method: seed.Method, MethodKind: seed.MethodKind,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_create_typed" {
		for _, seed := range p.pgCreateTyped {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.TypeSchema == request.Qualifiers["type_schema"] && seed.TypeName == request.Qualifiers["type_name"] {
				return &spec.ObjectSnapshot{PGCreateTyped: &spec.PGCreateTypedState{
					Target:       spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Type:         spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.TypeSchema, Name: seed.TypeName}},
					TargetSchema: true, TypeSchema: true, TypeExists: true, Defined: true,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_create_destination" {
		for _, seed := range p.pgCreateCommits {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}, SchemaExists: true,
				}}, nil
			}
		}
		return nil, nil
	}
	if (request.Type == "pg_create_like" || request.Type == "pg_create_inheritance") && len(request.PGDestructiveTargets) == 1 {
		for _, seed := range p.pgCreateLikes {
			target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}
			source := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.SourceSchema, Name: seed.SourceName}}
			if target.Table.Schema == request.Schema && target.Table.Name == request.Name && request.PGDestructiveTargets[0] == source {
				state := &spec.PGCreateSources{
					Target: target, SchemaExists: true, Sources: []spec.PGCreateSource{{Target: source}},
				}
				if request.Type == "pg_create_inheritance" {
					return &spec.ObjectSnapshot{PGCreateInheritance: state}, nil
				}
				return &spec.ObjectSnapshot{PGCreateLike: state}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_operator_family_member" && request.PGOperatorFamilyMember != nil {
		for _, seed := range p.pgOperatorFamilies {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Method == request.Qualifiers["method"] {
				return &spec.ObjectSnapshot{PGOperatorFamilyMember: &spec.PGOperatorFamilyMemberState{
					Target: spec.PGOperatorFamilyTarget{Schema: seed.Schema, Name: seed.Name, Method: seed.Method},
					Member: *request.PGOperatorFamilyMember, SchemaExists: seed.SchemaExists, MethodKind: seed.MethodKind,
					FamilyExists: seed.Exists, BTree: seed.Method == "btree", OperandTypesRead: true, OperandTypesOK: true,
					SlotExists: seed.MemberSlotExists,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_operator_family_change" && request.Qualifiers["action"] == "rename" {
		for _, seed := range p.pgOperatorFamilies {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Method == request.Qualifiers["method"] && seed.OccupiedRename == request.Qualifiers["destination"] {
				return &spec.ObjectSnapshot{PGOperatorFamilyChange: &spec.PGOperatorFamilyChangeState{
					Target:     spec.PGOperatorFamilyChange{Source: spec.PGOperatorFamilyTarget{Schema: seed.Schema, Name: seed.Name, Method: seed.Method}, Action: "rename", Destination: seed.OccupiedRename},
					MethodKind: seed.MethodKind, SourceSchemaExists: seed.SchemaExists, SourceExists: seed.Exists, DestinationOccupied: true,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_operator_family" {
		for _, seed := range p.pgOperatorFamilies {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Method == request.Qualifiers["method"] {
				return &spec.ObjectSnapshot{PGOperatorFamily: &spec.PGOperatorFamilyState{
					Target:       spec.PGOperatorFamilyTarget{Schema: seed.Schema, Name: seed.Name, Method: seed.Method},
					SchemaExists: seed.SchemaExists, MethodKind: seed.MethodKind, Exists: seed.Exists,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_index_include" {
		for _, seed := range p.pgIndexIncludes {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Method == request.Qualifiers["method"] {
				return &spec.ObjectSnapshot{PGIndexInclude: &spec.PGIndexIncludeState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Kind:   seed.Kind, Method: seed.Method, Columns: seed.Columns, System: seed.System,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_index_destination" && request.Qualifiers["action"] == "set_tablespace" {
		for _, seed := range p.pgIndexDestinations {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Tablespace == request.Qualifiers["destination"] {
				return &spec.ObjectSnapshot{PGIndexDestination: &spec.PGIndexDestinationState{
					Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Action: "set_tablespace", Destination: seed.Tablespace, Kind: "i",
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_index_options" {
		for _, seed := range p.pgIndexOptions {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGIndexOptions: &spec.PGIndexOptionsState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}, Kind: seed.Kind, Method: seed.Method,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_index_attachment" {
		for _, seed := range p.pgIndexAttachments {
			if seed.Schema == request.Schema && seed.Parent == request.Name && seed.ChildSchema == request.Qualifiers["child_schema"] && seed.Child == request.Qualifiers["child"] {
				return &spec.ObjectSnapshot{PGIndexAttachment: &spec.PGIndexAttachmentState{
					Target: spec.PGIndexAttachment{
						Parent: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Parent}},
						Child:  spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.ChildSchema, Name: seed.Child}},
					},
					ParentKind: "I", ChildKind: "i", TablePartition: true, Definition: seed.Definition,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_collation_change" && request.Qualifiers["action"] == "rename" {
		for _, seed := range p.pgCollationChanges {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Destination == request.Qualifiers["destination"] {
				return &spec.ObjectSnapshot{PGCollationChange: &spec.PGCollationChangeState{
					Target:      spec.PGCollationChangeTarget{Source: spec.PGColumnCollation{Schema: seed.Schema, Name: seed.Name}, Action: "rename", Destination: seed.Destination},
					SourceKnown: true, SourceExists: true, DestinationOccupied: true,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_collation_refresh" {
		for _, seed := range p.pgCollationRefreshes {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGCollationRefresh: &spec.PGCollationRefreshState{
					Target: spec.PGColumnCollation{Schema: seed.Schema, Name: seed.Name}, Known: true, Exists: seed.Exists, Default: seed.Default,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_foreign_table_change" && request.Qualifiers["action"] == "drop_column" {
		for _, seed := range p.pgForeignTables {
			if seed.Schema == request.Schema && seed.Name == request.Name && seed.Column == request.Qualifiers["column"] {
				return &spec.ObjectSnapshot{PGForeignTableChange: &spec.PGForeignTableChangeState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Action: "drop_column", Column: seed.Column, RelationKind: "f",
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_sequence_change" {
		for _, seed := range p.pgSequenceChanges {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGSequenceChange: &spec.PGSequenceChangeState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}, SchemaExists: true, Kind: seed.Kind,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_wrapper_change" {
		for _, seed := range p.pgWrappers {
			if seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGWrapperChange: &spec.PGWrapperChangeState{Name: seed.Name, Exists: true, Options: seed.Options}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_policy_change" {
		for _, seed := range p.pgPolicies {
			if seed.Schema == request.Schema && seed.Table == request.Name && seed.Name == request.Qualifiers["policy"] {
				return &spec.ObjectSnapshot{PGPolicyChange: &spec.PGPolicyChangeState{
					Table: spec.Table{Schema: seed.Schema, Name: seed.Table}, Name: seed.Name, RelationKind: "r", PolicyCommand: seed.Command,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_operator_change" && request.Qualifiers["action"] == "set_schema" && request.Qualifiers["unary"] == "false" {
		for _, seed := range p.pgOperatorChanges {
			if seed.Schema != request.Schema || seed.Name != request.Name || seed.Destination != request.Qualifiers["destination"] {
				continue
			}
			return &spec.ObjectSnapshot{PGOperatorChange: &spec.PGOperatorChangeState{
				Target: spec.PGOperatorChange{
					Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Action: "set_schema", Destination: seed.Destination, Signature: true,
					Left:  spec.PGColumnTypeTarget{Schema: request.Qualifiers["left_schema"], Name: request.Qualifiers["left_type"], Array: request.Qualifiers["left_array"] == "true"},
					Right: spec.PGColumnTypeTarget{Schema: request.Qualifiers["right_schema"], Name: request.Qualifiers["right_type"], Array: request.Qualifiers["right_array"] == "true"},
				}, OperandSchema: true, OperandTypes: true, SourceSchema: true, SourceExists: true, DestinationSchema: true, DestinationOccupied: true,
			}}, nil
		}
		return nil, nil
	}
	if request.Type == "pg_type_change" {
		for _, seed := range p.pgComposites {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGTypeChange: &spec.PGTypeChangeState{
					Target: spec.PGTypeChange{
						Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
						Action: request.Qualifiers["action"], Destination: request.Qualifiers["destination"],
					}, SourceExists: true, Defined: true, Composite: true, DestinationRelation: true,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_composite_type" {
		for _, seed := range p.pgComposites {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGComposite: &spec.PGCompositeState{
					Target:       spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					SchemaExists: true, RelationKind: "c", Attributes: seed.Attributes,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_create_enum" || request.Type == "pg_alter_enum" || request.Type == "pg_create_range" || request.Type == "pg_base_type" {
		for _, seed := range p.pgEnums {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{
					Target:       spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					SchemaExists: true, TypeExists: true, Defined: request.Type != "pg_base_type", TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: seed.Labels,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_text_search_change" && request.Qualifiers["kind"] == "dictionary" && request.Qualifiers["action"] == "set_dictionary_options" {
		for _, seed := range p.pgTextSearch {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGTextSearchChange: &spec.PGTextSearchChangeState{
					Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}},
					Kind:   "dictionary", Action: "set_dictionary_options", SourceSchema: true, SourceExists: true, SimpleTemplate: true,
				}}, nil
			}
		}
		return nil, nil
	}
	switch request.Type {
	case "pg_publication_change":
		for _, seed := range p.pgReplicationTargets {
			if seed.Kind == "publication" && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGPublicationChange: &spec.PGPublicationChangeState{Name: seed.Name, Exists: seed.Exists}}, nil
			}
		}
	case "pg_subscription_change":
		for _, seed := range p.pgReplicationTargets {
			if seed.Kind == "subscription" && seed.Name == request.Name {
				return &spec.ObjectSnapshot{PGSubscriptionChange: &spec.PGSubscriptionChangeState{Name: seed.Name, Exists: seed.Exists}}, nil
			}
		}
	}
	if request.Type == "pg_language_declaration" {
		for _, seed := range p.pgLanguages {
			target := spec.PGLanguageRoutine{Kind: "handler", Schema: seed.HandlerSchema, Name: seed.HandlerName}
			if seed.Name == request.Name && len(request.PGLanguageRoutines) == 1 && request.PGLanguageRoutines[0] == target {
				return &spec.ObjectSnapshot{PGLanguage: &spec.PGLanguageState{Name: seed.Name, Database: "postgres", Routines: []spec.PGLanguageRoutineState{{Target: target}}}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_grant_declaration" && request.PGGrant != nil {
		d := request.PGGrant
		for _, seed := range p.pgGrants {
			target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}
			grantee := spec.PGTableOwnerTarget{Kind: "named", Name: seed.Grantee}
			if d.ObjectKind == "table" && len(d.Targets) == 1 && d.Targets[0] == target && len(d.Grantees) == 1 && d.Grantees[0] == grantee {
				return &spec.ObjectSnapshot{PGGrant: &spec.PGGrantState{ObjectKind: "table",
					Targets:  []spec.PGGrantTargetState{{Target: target, Exists: true, Kind: "r", Columns: []string{"id"}}},
					Grantees: []spec.PGTableOwnerTarget{grantee}, GranteeExists: []bool{true}}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_create_schema" {
		for _, seed := range p.pgCreateSchemas {
			if seed.Name == request.Name && seed.Owner == request.Qualifiers["role_name"] && request.Qualifiers["role_kind"] == "named" {
				return &spec.ObjectSnapshot{PGCreateSchema: &spec.PGCreateSchemaState{
					Target:    spec.PGCreateSchemaTarget{Name: seed.Name, Owner: spec.PGTableOwnerTarget{Kind: "named", Name: seed.Owner}},
					NameKnown: true, Exists: seed.Exists, OwnerExists: seed.OwnerExists,
				}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "pg_partition_constraint" {
		for _, seed := range p.pgPartitionConstraints {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				state := &spec.PGPartitionConstraintState{Target: spec.Table{Schema: seed.Schema, Name: seed.Name},
					Columns:       make([]spec.PGColumnState, 0, len(seed.Columns)),
					PartitionKeys: []spec.PGPartitionConstraintKey{{Name: seed.Key, CollationOK: true, EqualityRead: true, EqualityOK: true}}}
				for _, name := range seed.Columns {
					state.Columns = append(state.Columns, spec.PGColumnState{Name: name})
				}
				return &spec.ObjectSnapshot{PGPartitionConstraint: state}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "drop_view_target" || request.Type == "alter_view_options" || request.Type == "materialized_view_options" {
		for _, seed := range p.pgDropViewStates {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: seed.Schema, Name: seed.Name}}
				return &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: target, Kind: seed.Kind, DependentView: seed.DependentView, DependentColumn: seed.DependentColumn, CheckOption: seed.CheckOption, Heap: seed.Heap, HasToast: seed.HasToast}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "drop_references" || request.Type == "truncate_references" {
		if len(request.PGDestructiveTargets) == 1 {
			target := request.PGDestructiveTargets[0].Table
			for _, seed := range p.pgDestructiveReferences {
				if seed.Operation == request.Type && seed.Schema == target.Schema && seed.Table == target.Name {
					return &spec.ObjectSnapshot{PGDestructiveReferences: &spec.PGDestructiveReferences{Targets: request.PGDestructiveTargets, BlockedPosition: seed.BlockedPosition}}, nil
				}
			}
		}
		return nil, nil
	}
	if request.Type == "refresh_materialized_view" {
		for _, seed := range p.pgRefreshStates {
			if seed.Schema == request.Schema && seed.Name == request.Name {
				state := spec.PGRefreshState{Target: spec.PGRefreshTarget{Schema: seed.Schema, Name: seed.Name}, Kind: seed.Kind, Populated: seed.Populated, UniqueIndex: seed.UniqueIndex}
				return &spec.ObjectSnapshot{PGRefreshState: &state}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "column_traversal" {
		for _, seed := range p.pgColumnTraversals {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Column == request.Name {
				return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: seed.Schema, Table: seed.Table, Column: seed.Column, Inherited: seed.Inherited, Partition: seed.Partition, ColumnExists: seed.ColumnExists, AttachedChildren: seed.AttachedChildren, ParentNotNull: seed.ParentNotNull, ValidatedNotNullCheck: seed.ValidatedNotNullCheck, IdentityChangeBlocked: seed.IdentityChangeBlocked, RenameDestination: seed.RenameDestination, RenameDescendantsKnown: seed.RenameDescendantsKnown, RenameDescendantConflict: seed.RenameDescendantConflict, TypeDescendantConflict: seed.TypeDescendantConflict}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "drop_column_dependency" {
		for _, seed := range p.pgDropColumnDependencies {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Column == request.Name {
				return &spec.ObjectSnapshot{PGDropColumnDependency: &spec.PGDropColumnDependency{Schema: seed.Schema, Table: seed.Table, Column: seed.Column, Kind: seed.Kind}}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "drop_constraint_reference" {
		for _, seed := range p.pgDropConstraintReferences {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Name == request.Name && seed.Kind == request.Qualifiers["kind"] {
				return &spec.ObjectSnapshot{PGDropConstraintReference: &spec.PGDropConstraintReference{Schema: seed.Schema, Table: seed.Table, Name: seed.Name, Kind: seed.Kind, Blocked: seed.Blocked}}, nil
			}
		}
		return nil, nil
	}

	if request.Type == "foreign_key_scalar_equality" {
		if target := request.PGForeignKeyEqualityTarget; target != nil {
			for _, seed := range p.pgForeignKeyDirectories {
				if seed.Schema == target.Reference.Schema && seed.Table == target.Reference.Name && seed.IncompatiblePosition != nil {
					return &spec.ObjectSnapshot{PGForeignKeyEquality: &spec.PGForeignKeyEquality{Target: *target, IncompatiblePosition: *seed.IncompatiblePosition}}, nil
				}
			}
		}
		return nil, nil
	}
	if request.Type == "foreign_key_reference_keys" {
		for _, seed := range p.pgForeignKeyDirectories {
			if seed.Schema == request.Schema && seed.Table == request.Name {
				facts := &spec.PGForeignKeyKeys{Schema: seed.Schema, Table: seed.Table}
				for _, key := range seed.Keys {
					facts.Keys = append(facts.Keys, spec.PGForeignKeyKey{Columns: key.Columns, Primary: key.Primary, Immediate: key.Immediate})
				}
				return &spec.ObjectSnapshot{PGForeignKeyKeys: facts}, nil
			}
		}
		return nil, nil
	}

	if request.Type == "constraint_using_index" {
		for _, seed := range p.pgConstraintIndexes {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Name == request.Name {
				facts := &spec.PGConstraintUsingIndex{Schema: seed.Schema, Table: seed.Table, Name: seed.Name, Kind: seed.Kind, IndexKnown: true, OnTable: true, Valid: seed.Valid, Unique: true, BTree: true, KeyDefaults: true, KeyDefaultsKnown: true}
				if seed.OpclassesDefault != nil {
					facts.OpclassesDefaultKnown, facts.OpclassesDefault = true, *seed.OpclassesDefault
				}
				return &spec.ObjectSnapshot{PGConstraintUsingIndex: facts}, nil
			}
		}
		return nil, nil
	}

	if request.Type == "column_type_destination" {
		target := spec.PGColumnTypeTarget{Schema: request.Schema, Name: request.Name, Array: request.Qualifiers["array"] == "true"}
		for _, seed := range p.pgColumnTypes {
			if seed.Schema == target.Schema && seed.Name == target.Name && seed.Array == target.Array {
				facts := &spec.PGColumnTypeState{Target: target, SchemaExists: seed.SchemaExists, BaseExists: seed.BaseExists, Resolved: seed.Resolved, Defined: seed.Defined, Pseudo: seed.Pseudo}
				if seed.Assignment != nil {
					facts.SourceSchema, facts.SourceTable, facts.SourceColumn = seed.SourceSchema, seed.SourceTable, seed.SourceColumn
					facts.AssignmentKnown, facts.Assignable = true, *seed.Assignment
				}
				return &spec.ObjectSnapshot{PGColumnType: facts}, nil
			}
		}
		return nil, nil
	}

	if request.Type == "bulk_tablespace" {
		target := request.PGBulkTablespace
		if target != nil && len(target.Roles) == 0 {
			for _, seed := range p.pgBulkTablespaces {
				if seed.Source == target.Source && seed.Destination == target.Destination {
					return &spec.ObjectSnapshot{PGBulkTablespace: &spec.PGBulkTablespaceState{
						Target: *target, SourceExists: seed.SourceExists, DestinationExists: seed.DestinationExists,
					}}, nil
				}
			}
		}
		return nil, nil
	}
	if request.Type == "table_owner" || request.Type == "view_owner" {
		for _, seed := range p.pgTableOwners {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Kind == request.Qualifiers["kind"] && seed.Name == request.Name {
				facts := &spec.PGTableOwnerState{Schema: seed.Schema, Table: seed.Table, Target: spec.PGTableOwnerTarget{Kind: seed.Kind, Name: seed.Name}, Exists: seed.Exists, Same: seed.Same, SourceKind: seed.SourceKind, SourceKindKnown: seed.SourceKindKnown}
				return &spec.ObjectSnapshot{PGTableOwner: facts}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "table_namespace" || request.Type == "view_namespace" {
		for _, seed := range p.pgTableNamespaces {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Action == request.Qualifiers["action"] && seed.Destination == request.Name {
				facts := spec.PGTableNamespace(seed)
				return &spec.ObjectSnapshot{PGTableNamespace: &facts}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "table_persistence" {
		for _, seed := range p.pgTablePersistences {
			if seed.Schema == request.Schema && seed.Table == request.Name && seed.Logged == (request.Qualifiers["action"] == "set_logged") {
				facts := spec.PGTablePersistence(seed)
				return &spec.ObjectSnapshot{PGTablePersistence: &facts}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "table_access_method" {
		for _, seed := range p.pgTableAccessMethods {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Name == request.Name && request.Qualifiers["is_default"] == strconv.FormatBool(seed.Default) {
				facts := &spec.PGTableAccessMethod{Schema: seed.Schema, Table: seed.Table, Name: seed.Name, Default: seed.Default, Kind: seed.Kind}
				if seed.Same != nil {
					facts.Same, facts.SameKnown = *seed.Same, true
				}
				return &spec.ObjectSnapshot{PGTableAccessMethod: facts}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "table_tablespace" {
		for _, seed := range p.pgTableTablespaces {
			if seed.Schema == request.Schema && seed.Table == request.Qualifiers["table"] && seed.Name == request.Name {
				facts := &spec.PGTableTablespace{Schema: seed.Schema, Table: seed.Table, Name: seed.Name, Exists: seed.Exists, Global: seed.Global}
				if seed.Same != nil {
					facts.Same, facts.SameKnown = *seed.Same, true
				}
				return &spec.ObjectSnapshot{PGTableTablespace: facts}, nil
			}
		}
		return nil, nil
	}
	if request.Type == "replica_identity_index" {
		for _, seed := range p.pgReplicaIndexes {
			if seed.Schema != request.Schema || seed.Table != request.Qualifiers["table"] || seed.Name != request.Name {
				continue
			}
			facts := &spec.PGReplicaIdentityIndex{Schema: seed.Schema, Table: seed.Table, Name: seed.Name, Kind: seed.Kind,
				IndexKnown: seed.Kind == "i" || seed.Kind == "I", OnTable: seed.OnTable, Unique: seed.Unique,
				Immediate: seed.Immediate, Expression: seed.Expression, Partial: seed.Partial, PartitionRoot: seed.PartitionRoot}
			if seed.CanUnique != nil {
				facts.CanUnique, facts.CanUniqueKnown = *seed.CanUnique, true
			}
			if seed.KeyValid != nil {
				facts.KeyAttributesValid, facts.KeyAttributesKnown = *seed.KeyValid, true
			}
			return &spec.ObjectSnapshot{PGReplicaIdentity: facts}, nil
		}
		return nil, nil
	}
	if request.Type == "mysql_spatial_reference_system" {
		var snapshot *spec.ObjectSnapshot
		if p.spatialReferences == nil {
			snapshot = &spec.ObjectSnapshot{Schema: request.Schema, Type: request.Type, Name: request.Name, Status: spec.MetadataStatusUnavailable}
		} else if seeded, ok := p.spatialReferences[strings.TrimSpace(request.Name)]; ok {
			snapshot = seeded
		} else {
			snapshot = &spec.ObjectSnapshot{Schema: request.Schema, Type: request.Type, Name: request.Name, Status: spec.MetadataStatusUnavailable}
		}
		return p.srsConflictFacts(snapshot, request), nil
	}
	if request.Type == "mysql_tablespace" {
		if p.tablespaces == nil {
			return &spec.ObjectSnapshot{Schema: request.Schema, Type: request.Type, Name: request.Name, Status: spec.MetadataStatusUnavailable}, nil
		}
		// Tablespace names are case-sensitive — the fixture key must
		// not fold authored identity.
		snapshot, ok := p.tablespaces[strings.TrimSpace(request.Name)]
		if !ok {
			return &spec.ObjectSnapshot{Schema: request.Schema, Type: request.Type, Name: request.Name, Status: spec.MetadataStatusUnavailable}, nil
		}
		return snapshot, nil
	}
	if request.Type == "mysql_resource_group" {
		if p.mysqlResourceGroups == nil {
			return &spec.ObjectSnapshot{
				Schema: request.Schema,
				Type:   request.Type,
				Name:   request.Name,
				Status: spec.MetadataStatusUnavailable,
			}, nil
		}
		key := strings.ToLower(strings.TrimSpace(request.Name))
		snapshot, ok := p.mysqlResourceGroups[key]
		if !ok {
			return &spec.ObjectSnapshot{
				Schema: request.Schema,
				Type:   request.Type,
				Name:   request.Name,
				Status: spec.MetadataStatusUnavailable,
			}, nil
		}
		return snapshot, nil
	}
	if request.Type == "resource_group" {
		if p.resourceGroups == nil {
			return &spec.ObjectSnapshot{
				Schema: request.Schema,
				Type:   request.Type,
				Name:   request.Name,
				Status: spec.MetadataStatusUnavailable,
			}, nil
		}
		key := strings.ToLower(strings.TrimSpace(request.Name))
		snapshot, ok := p.resourceGroups[key]
		if !ok {
			return &spec.ObjectSnapshot{
				Schema: request.Schema,
				Type:   request.Type,
				Name:   request.Name,
				Status: spec.MetadataStatusUnavailable,
			}, nil
		}
		return snapshot, nil
	}
	if request.Type != "sequence" || p.sequences == nil {
		return &spec.ObjectSnapshot{
			Schema: request.Schema,
			Type:   request.Type,
			Name:   request.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	key := strings.ToLower(strings.TrimSpace(request.Schema) + "." + strings.TrimSpace(request.Name))
	snapshot, ok := p.sequences[key]
	if !ok {
		return &spec.ObjectSnapshot{
			Schema: request.Schema,
			Type:   request.Type,
			Name:   request.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	return snapshot, nil
}

func corpusTableSnapshotToSpec(tableName, defaultSchema string, snapshot corpusTableSnapshot) spec.TableSnapshot {
	out := spec.TableSnapshot{Schema: firstNonEmpty(snapshot.Schema, defaultSchema),
		Exists:      snapshot.Exists,
		Columns:     make([]spec.Column, 0, len(snapshot.Columns)),
		Indexes:     make([]spec.Index, 0, len(snapshot.Indexes)),
		Constraints: make([]spec.Constraint, 0, len(snapshot.Constraints)),
		Options:     cloneStringMap(snapshot.Options),
	}
	// Cache-status fixture convention: pre-R7 fixtures predate the fact —
	// they all model ordinary never-cached tables, so an absent key is
	// completed as a proven "disable". A fixture that wants the
	// unobserved-status premise declares `cache_status: "unobserved"`,
	// which the converter strips back to absence before any consumer
	// reads it (issue #102 R7).
	if out.Options == nil {
		out.Options = map[string]string{}
	}
	if status, ok := out.Options["cache_status"]; ok && status == "unobserved" {
		delete(out.Options, "cache_status")
	} else if !ok {
		out.Options["cache_status"] = "disable"
	}
	if snapshot.Table != nil {
		out.Table = &spec.Table{
			Schema:  snapshot.Table.Schema,
			Name:    snapshot.Table.Name,
			Comment: snapshot.Table.Comment,
		}
	} else if strings.TrimSpace(tableName) != "" {
		out.Table = &spec.Table{Schema: out.Schema, Name: tableName}
	}
	for _, column := range snapshot.Columns {
		out.Columns = append(out.Columns, corpusColumnToSpec(column))
	}
	if snapshot.PrimaryKey != nil {
		index := corpusIndexToSpec(*snapshot.PrimaryKey)
		out.PrimaryKey = &index
	}
	for _, index := range snapshot.Indexes {
		out.Indexes = append(out.Indexes, corpusIndexToSpec(index))
	}
	for _, constraint := range snapshot.Constraints {
		out.Constraints = append(out.Constraints, corpusConstraintToSpec(constraint))
	}
	if snapshot.PGConstraints != nil {
		out.PGConstraints = make([]spec.PGConstraintState, 0, len(snapshot.PGConstraints))
		for _, member := range snapshot.PGConstraints {
			if member.Validated == nil {
				out.PGConstraints = nil
				break
			}
			state := spec.PGConstraintState{Name: member.Name, Type: member.Type, Validated: *member.Validated, DerivedForeignKey: member.DerivedForeignKey}
			if member.Inherited != nil {
				state.InheritanceKnown, state.Inherited = true, *member.Inherited
			}
			if member.PartitionChildren != nil {
				state.PartitionCheckHasChildrenKnown, state.PartitionCheckHasChildren = true, *member.PartitionChildren
			}
			if member.ValidationChildren != nil {
				state.CheckValidationChildrenKnown = true
				state.CheckValidationHasChildren = *member.ValidationChildren
			}
			out.PGConstraints = append(out.PGConstraints, state)
		}
	}
	out.PGRelation = spec.PGRelationState{Name: snapshot.PGRelation.Name, Kind: snapshot.PGRelation.Kind, Heap: snapshot.PGRelation.Heap}
	if cat := snapshot.PGColumns; cat != nil && cat.Ordinary != nil {
		out.PGColumns = &spec.PGColumnCatalog{Ordinary: *cat.Ordinary}
		for namespace, members := range [][]corpusPGColumnState{cat.Columns, cat.SystemColumns} {
			if members == nil {
				continue
			}
			states := make([]spec.PGColumnState, 0, len(members))
			for _, member := range members {
				if member.Name == "" || member.Identity == nil || member.Generated == nil || member.TypeWidth == nil || member.NotNull == nil || member.HasDefault == nil || member.PrimaryKey == nil || member.System != (namespace == 1) {
					states = nil
					break
				}
				state := spec.PGColumnState{Name: member.Name, Identity: *member.Identity, Generated: *member.Generated, TypeWidth: *member.TypeWidth, TypeStorage: member.TypeStorage, System: member.System, NotNull: *member.NotNull, HasDefault: *member.HasDefault, PrimaryKey: *member.PrimaryKey}
				if member.ReplicaIdentityKey != nil {
					state.ReplicaIdentityKeyKnown = true
					state.ReplicaIdentityKey = *member.ReplicaIdentityKey
				}
				if seq := member.IdentitySequence; seq != nil {
					state.IdentitySequence = &spec.PGIdentitySequenceState{TypeWidth: seq.TypeWidth, Definition: spec.SequenceDefinition{Start: seq.Start, Increment: seq.Increment, MinValue: seq.MinValue, MaxValue: seq.MaxValue, CacheValue: seq.CacheValue, Cycle: seq.Cycle}}
				}
				states = append(states, state)
			}
			if namespace == 0 {
				out.PGColumns.Columns = states
			} else {
				out.PGColumns.SystemColumns = states
			}
		}
	}
	out.PrimaryKeyUnknown = snapshot.PrimaryKeyUnknown
	out.ConstraintsUnknown = snapshot.ConstraintsUnknown
	out.TTLKnown = snapshot.TTLKnown
	out.PlacementPolicy = snapshot.PlacementPolicy
	out.PlacementPolicyKnown = snapshot.PlacementPolicyKnown
	out.PartitionPlacements = snapshot.PartitionPlacements
	if snapshot.ReferencedBy != nil {
		out.ReferencedBy = make([]spec.Table, 0, len(snapshot.ReferencedBy))
		for _, ref := range snapshot.ReferencedBy {
			out.ReferencedBy = append(out.ReferencedBy, spec.Table{Schema: ref.Schema, Name: ref.Name})
		}
	}
	out.ReferencedByUnknown = snapshot.ReferencedByUnknown
	if snapshot.TTL != nil {
		out.TTL = &spec.TTLConfig{
			TimeColumn:    snapshot.TTL.TimeColumn,
			IntervalValue: snapshot.TTL.IntervalValue,
			IntervalUnit:  snapshot.TTL.IntervalUnit,
			Enabled:       snapshot.TTL.Enabled,
			JobInterval:   snapshot.TTL.JobInterval,
		}
	}
	if snapshot.Partitioned != nil {
		partitioned := *snapshot.Partitioned
		out.Partitioned = &partitioned
	}
	if snapshot.PartitionType != "" {
		partitionType := snapshot.PartitionType
		out.PartitionType = &partitionType
	}
	if snapshot.PartitionNames != nil {
		out.PartitionNames = append([]string{}, snapshot.PartitionNames...)
	}
	if snapshot.PartitionDescriptions != nil {
		out.PartitionDescriptions = append([]string{}, snapshot.PartitionDescriptions...)
		out.PartitionDefs = spec.CatalogPartitionDefs(out.PartitionNames, out.PartitionDescriptions, snapshot.PartitionType)
	}
	return out
}

func corpusColumnToSpec(column corpusColumn) spec.Column {
	return spec.Column{
		Name:                      column.Name,
		Type:                      column.Type,
		Length:                    column.Length,
		Charset:                   column.Charset,
		Collation:                 column.Collation,
		Comment:                   column.Comment,
		Unsigned:                  column.Unsigned,
		NotNull:                   column.NotNull,
		AutoIncrement:             column.AutoIncrement,
		HasDefault:                column.HasDefault,
		DefaultValue:              column.DefaultValue,
		DefaultIsNull:             column.DefaultIsNull,
		DefaultIsCurrentTimestamp: column.DefaultIsCurrentTimestamp,
		OnUpdateCurrentTimestamp:  column.OnUpdateCurrentTimestamp,
		GeneratedWhen:             column.GeneratedWhen,
		IsIdentity:                column.IsIdentity,
		IdentityOptions:           column.IdentityOptions,
	}
}

func corpusIndexToSpec(index corpusIndex) spec.Index {
	return spec.Index{
		Name:        index.Name,
		Kind:        spec.IndexKind(index.Kind),
		Columns:     append([]string(nil), index.Columns...),
		Cardinality: index.Cardinality,
	}
}

func corpusConstraintToSpec(constraint corpusConstraint) spec.Constraint {
	return spec.Constraint{
		Type:              constraint.Type,
		Name:              constraint.Name,
		Columns:           append([]string(nil), constraint.Columns...),
		Enforced:          constraint.Enforced,
		ReferencedSchema:  constraint.ReferencedSchema,
		ReferencedTable:   constraint.ReferencedTable,
		ReferencedColumns: append([]string(nil), constraint.ReferencedColumns...),
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// corpusExtractStatement parses and extracts SQL for a given dialect,
// returning the first supported spec.Statement. If all statements are
// unsupported, it returns ok=false.
func corpusExtractStatement(t *testing.T, sql string, dialect spec.Dialect) (spec.Statement, bool) {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("semantic parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("semantic extract: %v", err)
	}
	for i := range statements {
		if statements[i].Unsupported == nil {
			return statements[i], true
		}
	}
	return spec.Statement{}, false
}

// corpusAssertOperation asserts that the operation on the extracted statement
// matches the expected value. It checks DDL.Operation for DDL statements and
// DML.Operation for DML statements.
func corpusAssertOperation(t *testing.T, stmt spec.Statement, expected string) {
	t.Helper()
	var actual string
	switch {
	case stmt.DDL != nil:
		actual = string(stmt.DDL.Operation)
	case stmt.DML != nil:
		actual = string(stmt.DML.Operation)
	default:
		t.Fatal("semantic operation: statement has neither DDL nor DML payload")
	}
	if actual != expected {
		t.Errorf("operation: expected %q, got %q", expected, actual)
	}
}

// corpusAssertConstraintFacts asserts that every expected constraint is present
// in the statement's DDL constraints. Each expected constraint can specify any
// combination of type, name, columns, referenced_schema, referenced_table, and referenced_columns.
// Only non-zero fields in the expected entry are checked.
func corpusAssertConstraintFacts(t *testing.T, stmt spec.Statement, expected []corpusFactConstraint) {
	t.Helper()
	if stmt.DDL == nil {
		t.Fatal("semantic facts: statement has no DDL payload")
	}
	actual := stmt.DDL.Constraints
	for _, exp := range expected {
		found := false
		for _, c := range actual {
			if constraintMatches(c, exp) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("facts.constraints: expected %+v not found in actual constraints %+v", exp, summarizeConstraints(actual))
		}
	}
}

// summarizeConstraints returns a lightweight summary for error messages.
func summarizeConstraints(constraints []spec.Constraint) []map[string]string {
	out := make([]map[string]string, 0, len(constraints))
	for _, c := range constraints {
		m := map[string]string{
			"type": c.Type, "name": c.Name,
			"referenced_schema": c.ReferencedSchema,
			"referenced_table":  c.ReferencedTable,
		}
		out = append(out, m)
	}
	return out
}

// constraintMatches checks whether an actual spec.Constraint matches the
// expected corpusFactConstraint. Only non-zero expected fields are compared.
func constraintMatches(actual spec.Constraint, exp corpusFactConstraint) bool {
	if exp.Type != "" && actual.Type != exp.Type {
		return false
	}
	if exp.Name != "" && actual.Name != exp.Name {
		return false
	}
	if len(exp.Columns) > 0 && !stringSlicesEqual(actual.Columns, exp.Columns) {
		return false
	}
	if exp.ReferencedSchema != "" && actual.ReferencedSchema != exp.ReferencedSchema {
		return false
	}
	if exp.ReferencedTable != "" && actual.ReferencedTable != exp.ReferencedTable {
		return false
	}
	if len(exp.ReferencedColumns) > 0 && !stringSlicesEqual(actual.ReferencedColumns, exp.ReferencedColumns) {
		return false
	}
	return true
}

// stringSlicesEqual compares two string slices for element equality ignoring order.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// corpusAssertUnsupportedMetadata asserts that each expected metadata entry is
// present on a matching unsupported detail in the result. Matching is by feature
// name; metadata values are compared with numeric type coercion (int/int32/int64/float64
// are treated as equivalent when their values match).
func corpusAssertUnsupportedMetadata(t *testing.T, actual []spec.UnsupportedDetail, expected []corpusUnsupported) {
	t.Helper()
	for _, exp := range expected {
		found := false
		for _, u := range actual {
			if u.Feature != exp.Feature {
				continue
			}
			if corpusMetadataMatches(t, exp.Metadata, u.Metadata) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unsupported.metadata: expected feature=%q metadata=%+v not matched", exp.Feature, exp.Metadata)
		}
	}
}

// corpusMetadataMatches checks whether all expected keys/values are present in
// actual. Numeric types are coerced: int, int32, int64, and float64 are compared
// by value. Other types use direct equality.
func corpusMetadataMatches(t *testing.T, expected, actual map[string]any) bool {
	t.Helper()
	for key, expVal := range expected {
		actVal, ok := actual[key]
		if !ok {
			return false
		}
		if !corpusValueEqual(expVal, actVal) {
			return false
		}
	}
	return true
}

// corpusValueEqual compares two values with numeric type coercion and
// recursive map[string]any support.
func corpusValueEqual(a, b any) bool {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	// Recursive map comparison for nested identity_options etc.
	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		if len(aMap) != len(bMap) {
			return false
		}
		for k, av := range aMap {
			bv, ok := bMap[k]
			if !ok || !corpusValueEqual(av, bv) {
				return false
			}
		}
		return true
	}
	return a == b
}

// toFloat64 converts numeric types to float64 for comparison.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// corpusAssertSemantic checks operation and facts against the parsed/extracted
// statement. This is the single entrypoint for all semantic (non-report) assertions.
// Only fields present in the expected YAML are checked.
func corpusAssertSemantic(t *testing.T, sql string, dialect spec.Dialect, tc corpusExpected) {
	t.Helper()
	stmt, ok := corpusExtractStatement(t, sql, dialect)
	if !ok {
		// All statements are unsupported — no semantic checks possible.
		return
	}
	if tc.Expect.Operation != "" {
		corpusAssertOperation(t, stmt, tc.Expect.Operation)
	}
	if tc.Facts != nil && len(tc.Facts.Constraints) > 0 {
		corpusAssertConstraintFacts(t, stmt, tc.Facts.Constraints)
	}
	if tc.Expect.Impact != nil {
		corpusAssertImpact(t, stmt, tc.Expect.Impact)
	}
}

func corpusAssertImpact(t *testing.T, stmt spec.Statement, expected *corpusImpact) {
	t.Helper()
	if stmt.DML == nil || stmt.DML.Impact == nil {
		t.Fatalf("semantic impact: expected DML impact, got %#v", stmt)
	}
	actual := stmt.DML.Impact
	if expected.EstimatedRows != nil {
		if actual.EstimatedRows == nil || *actual.EstimatedRows != *expected.EstimatedRows {
			t.Errorf("impact.estimated_rows: expected %v, got %#v", *expected.EstimatedRows, actual.EstimatedRows)
		}
	}
	if expected.EstimatedRatio != nil {
		if actual.EstimatedRatio == nil || *actual.EstimatedRatio != *expected.EstimatedRatio {
			t.Errorf("impact.estimated_ratio: expected %v, got %#v", *expected.EstimatedRatio, actual.EstimatedRatio)
		}
	}
	if expected.RiskLevel != "" && string(actual.RiskLevel) != expected.RiskLevel {
		t.Errorf("impact.risk_level: expected %q, got %q", expected.RiskLevel, actual.RiskLevel)
	}
	if expected.Confidence != "" && string(actual.Confidence) != expected.Confidence {
		t.Errorf("impact.confidence: expected %q, got %q", expected.Confidence, actual.Confidence)
	}
	if expected.Source != "" && string(actual.Source) != expected.Source {
		t.Errorf("impact.source: expected %q, got %q", expected.Source, actual.Source)
	}
	if expected.ReasonCodes != nil && !stringSlicesEqual(actual.ReasonCodes, expected.ReasonCodes) {
		t.Errorf("impact.reason_codes: expected %#v, got %#v", expected.ReasonCodes, actual.ReasonCodes)
	}
	if expected.Notes != nil && !stringSlicesEqual(actual.Notes, expected.Notes) {
		t.Errorf("impact.notes: expected %#v, got %#v", expected.Notes, actual.Notes)
	}
}

type corpusPostgreSQLAlterFactFile struct {
	Facts *struct {
		Alter []corpusPostgreSQLAlterFact `yaml:"alter"`
	} `yaml:"facts"`
}

type corpusPostgreSQLAlterFact struct {
	Action        string         `yaml:"action"`
	ColumnOldName string         `yaml:"column_old_name,omitempty"`
	Options       map[string]any `yaml:"options,omitempty"`
}

func corpusAssertPostgreSQLAlterFacts(t *testing.T, stmt spec.Statement, rawYAML []byte) {
	t.Helper()
	var expected corpusPostgreSQLAlterFactFile
	if err := yaml.Unmarshal(rawYAML, &expected); err != nil {
		t.Fatalf("parse PostgreSQL alter facts yaml: %v", err)
	}
	if expected.Facts == nil || len(expected.Facts.Alter) == 0 {
		return
	}
	if stmt.DDL == nil {
		t.Fatal("semantic facts: statement has no DDL payload")
	}
	for _, want := range expected.Facts.Alter {
		matched := false
		for i := range stmt.DDL.Alter {
			actual := stmt.DDL.Alter[i]
			if actual.Action != want.Action {
				continue
			}
			if want.ColumnOldName != "" {
				if actual.Column == nil || actual.Column.OldName != want.ColumnOldName {
					continue
				}
			}
			if len(want.Options) > 0 {
				if len(actual.Options) == 0 {
					continue
				}
				optionsMatch := true
				for key, expectedValue := range want.Options {
					actualValue, ok := actual.Options[key]
					if !ok || !corpusValueEqual(expectedValue, actualValue) {
						optionsMatch = false
						break
					}
				}
				if !optionsMatch {
					continue
				}
			}
			matched = true
			break
		}
		if !matched {
			t.Errorf("facts.alter: expected %+v not found in actual alter actions %+v", want, stmt.DDL.Alter)
		}
	}
}

// unsupportedFeatures returns a summary of unsupported detail features for error messages.
func unsupportedFeatures(details []spec.UnsupportedDetail) []string {
	features := make([]string, 0, len(details))
	for _, d := range details {
		features = append(features, d.Feature)
	}
	return features
}
