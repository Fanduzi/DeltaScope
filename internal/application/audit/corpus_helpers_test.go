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
		len(tc.Metadata.MySQLResourceGroups) == 0 && len(tc.Metadata.Tablespaces) == 0 && len(tc.Metadata.SpatialReferences) == 0 {
		return strings.TrimSpace(tc.Metadata.Schema), nil
	}
	return strings.TrimSpace(tc.Metadata.Schema), newCorpusMetadataProvider(tc.Metadata)
}

type corpusFixtureMetadataProvider struct {
	instance    *spec.InstanceFacts
	tables      map[string]*spec.TableSnapshot
	indexOwners map[string]string
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
		tables:      make(map[string]*spec.TableSnapshot, len(metadata.Tables)),
		indexOwners: make(map[string]string, len(metadata.IndexOwners)),
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
			out.PGConstraints = append(out.PGConstraints, spec.PGConstraintState{Name: member.Name, Type: member.Type, Validated: *member.Validated})
		}
	}
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
