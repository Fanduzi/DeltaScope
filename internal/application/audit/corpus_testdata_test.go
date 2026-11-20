// Package audit defines and validates shared SQL corpus expectations.
package audit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
)

// corpusExpected is the schema each .expected.yaml file must satisfy.
type corpusExpected struct {
	Name     string `yaml:"name"`
	Dialect  string `yaml:"dialect"`
	Category string `yaml:"category"`
	Expect   struct {
		ParseOK       *bool  `yaml:"parse_ok"`
		StatementKind string `yaml:"statement_kind"`
		Operation     string `yaml:"operation"`
		Unsupported   *struct {
			Count    *int                `yaml:"count"`
			Include  []string            `yaml:"include,omitempty"`
			Metadata []corpusUnsupported `yaml:"metadata,omitempty"`
		} `yaml:"unsupported"`
		Findings *struct {
			Include []string `yaml:"include"`
			Exclude []string `yaml:"exclude"`
		} `yaml:"findings"`
		Statements *struct {
			Count *int `yaml:"count"`
		} `yaml:"statements"`
		Diagnostics *struct {
			ParserErrorCount *int  `yaml:"parser_error_count"`
			Lines            []int `yaml:"lines"`
			Columns          []int `yaml:"columns"`
		} `yaml:"diagnostics"`
		Impact *corpusImpact `yaml:"impact,omitempty"`
	} `yaml:"expect"`
	Facts    *corpusFacts    `yaml:"facts"`
	Config   map[string]any  `yaml:"config,omitempty"`
	Metadata *corpusMetadata `yaml:"metadata,omitempty"`
}

func corpusHasPartialAssertions(tc corpusExpected) bool {
	return tc.Expect.Statements != nil || tc.Expect.Diagnostics != nil
}

// corpusExpectsUnsupported reports whether the fixture declares unsupported
// items and the audit returned the structured unsupported sentinel rather than
// a genuine parse failure.
func corpusExpectsUnsupported(tc corpusExpected, err error) bool {
	return tc.Expect.Unsupported != nil &&
		tc.Expect.Unsupported.Count != nil &&
		*tc.Expect.Unsupported.Count > 0 &&
		errors.Is(err, ErrUnsupportedStatement)
}

func corpusAssertPartialResult(t *testing.T, result report.Result, tc corpusExpected) {
	t.Helper()
	if tc.Expect.Statements != nil && tc.Expect.Statements.Count != nil && len(result.Statements) != *tc.Expect.Statements.Count {
		t.Errorf("statements.count: expected %d, got %d", *tc.Expect.Statements.Count, len(result.Statements))
	}
	if tc.Expect.Diagnostics == nil {
		return
	}
	parserDiagnostics := make([]struct{ line, column int }, 0)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Classification == "parser_error" {
			parserDiagnostics = append(parserDiagnostics, struct{ line, column int }{diagnostic.Line, diagnostic.Column})
		}
	}
	if tc.Expect.Diagnostics.ParserErrorCount != nil && len(parserDiagnostics) != *tc.Expect.Diagnostics.ParserErrorCount {
		t.Errorf("diagnostics.parser_error_count: expected %d, got %d", *tc.Expect.Diagnostics.ParserErrorCount, len(parserDiagnostics))
	}
	for i, line := range tc.Expect.Diagnostics.Lines {
		if i >= len(parserDiagnostics) || parserDiagnostics[i].line != line {
			t.Errorf("diagnostics.lines[%d]: expected %d, got %+v", i, line, parserDiagnostics)
		}
	}
	for i, column := range tc.Expect.Diagnostics.Columns {
		if i >= len(parserDiagnostics) || parserDiagnostics[i].column != column {
			t.Errorf("diagnostics.columns[%d]: expected %d, got %+v", i, column, parserDiagnostics)
		}
	}
}

type corpusImpact struct {
	EstimatedRows  *int64   `yaml:"estimated_rows,omitempty"`
	EstimatedRatio *float64 `yaml:"estimated_ratio,omitempty"`
	RiskLevel      string   `yaml:"risk_level,omitempty"`
	Confidence     string   `yaml:"confidence,omitempty"`
	Source         string   `yaml:"source,omitempty"`
	ReasonCodes    []string `yaml:"reason_codes,omitempty"`
	Notes          []string `yaml:"notes,omitempty"`
}

// corpusFacts carries expected structural facts for deeper assertions.
type corpusFacts struct {
	Constraints []corpusFactConstraint `yaml:"constraints"`
}

// corpusMetadata carries optional metadata-aware audit fixtures.
type corpusMetadata struct {
	Schema                     string                            `yaml:"schema,omitempty"`
	Instance                   *corpusInstanceFacts              `yaml:"instance,omitempty"`
	Tables                     map[string]corpusTableSnapshot    `yaml:"tables,omitempty"`
	IndexOwners                map[string]string                 `yaml:"index_owners,omitempty"`
	PGTableOwners              []corpusPGTableOwner              `yaml:"pg_table_owners,omitempty"`
	PGDropColumnDependencies   []corpusPGDropColumnDependency    `yaml:"pg_drop_column_dependencies,omitempty"`
	PGDropConstraintReferences []corpusPGDropConstraintReference `yaml:"pg_drop_constraint_references,omitempty"`
	PGForeignKeyDirectories    []corpusPGForeignKeyKeys          `yaml:"pg_foreign_key_directories,omitempty"`
	PGConstraintIndexes        []corpusPGConstraintIndex         `yaml:"pg_constraint_indexes,omitempty"`
	PGColumnTypes              []corpusPGColumnType              `yaml:"pg_column_types,omitempty"`
	PGBulkTablespaces          []corpusPGBulkTablespace          `yaml:"pg_bulk_tablespaces,omitempty"`
	PGTableNamespaces          []corpusPGTableNamespace          `yaml:"pg_table_namespaces,omitempty"`
	PGTablePersistences        []corpusPGTablePersistence        `yaml:"pg_table_persistences,omitempty"`
	PGTableAccessMethods       []corpusPGTableAccessMethod       `yaml:"pg_table_access_methods,omitempty"`
	PGTableTablespaces         []corpusPGTableTablespace         `yaml:"pg_table_tablespaces,omitempty"`
	PGReplicaIndexes           []corpusPGReplicaIndex            `yaml:"pg_replica_indexes,omitempty"`
	// PlacementPolicies is the proven TiDB policy catalog the provider
	// reads once: a non-nil map (even empty) is a vouched policy set;
	// bound_targets lists the proven inbound references the drop premise
	// consults. Omit the key to leave the catalog unvouched.
	PlacementPolicies map[string]corpusPlacementPolicy `yaml:"placement_policies,omitempty"`
	// Sequences is the proven sequence object set keyed `name` (bound under
	// metadata.schema): each entry carries the catalog's existence answer
	// and, when proven, the definition parameters. Omit the key to leave
	// every sequence identity unvouched (T21/#100).
	Sequences map[string]corpusSequenceObject `yaml:"sequences,omitempty"`
	// ResourceGroups is the proven TiDB resource-group object set keyed
	// `name`: each entry carries the catalog's existence answer and, when
	// proven, the definition parameters. Omit the key to leave every
	// resource-group identity unvouched (T22/#101).
	ResourceGroups map[string]corpusResourceGroupObject `yaml:"resource_groups,omitempty"`
	// MySQLResourceGroups is the proven MySQL CPU resource-group object
	// set keyed `name` — the information_schema.RESOURCE_GROUPS answer.
	// Omit the key to leave every MySQL resource-group identity unvouched
	// (T24/#103).
	MySQLResourceGroups map[string]corpusMySQLResourceGroupObject `yaml:"mysql_resource_groups,omitempty"`
	// Tablespaces is the proven MySQL tablespace object set keyed `name` —
	// the information_schema.INNODB_TABLESPACES existence answer plus the
	// bounded bound-table occupancy read (T25/#104). Omit the key to
	// leave every tablespace identity unvouched.
	Tablespaces map[string]corpusTablespaceObject `yaml:"tablespaces,omitempty"`
	// SpatialReferences is the proven MySQL SRS object set keyed by the
	// decimal SRID — the st_spatial_reference_systems existence answer
	// plus the bounded st_geometry_columns dependency witness set
	// (T26-SRS-DROP-EXECUTION-01). Omit the key to leave every SRS
	// identity unvouched.
	SpatialReferences map[string]corpusSRSObject `yaml:"spatial_references,omitempty"`
}

// corpusSRSObject is the catalog fact for one MySQL spatial reference
// system — status is the provenance answer; dependencies list the
// visible geometry-column witnesses the catalog returned for the SRID
// (a non-empty set is the positive witness the dependency refusal
// consumes); name/organization/organization_coordsys_id are the
// unique-key identities the CREATE conflict reads match against
// (T26-SRS-CREATE-PUBLICATION-01). Omit members to keep them unproven.
type corpusSRSObject struct {
	Status                 string                `yaml:"status"`
	Exists                 *bool                 `yaml:"exists,omitempty"`
	Name                   string                `yaml:"name,omitempty"`
	Organization           string                `yaml:"organization,omitempty"`
	OrganizationCoordsysID *uint32               `yaml:"organization_coordsys_id,omitempty"`
	Dependencies           []corpusSRSDependency `yaml:"dependencies,omitempty"`
}

// corpusSRSDependency is one visible dependent column witness.
type corpusSRSDependency struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Column string `yaml:"column"`
	SRID   uint32 `yaml:"srid"`
}

// corpusTablespaceObject is the catalog fact for one MySQL tablespace —
// status is the provenance answer; kind and occupied read only on a
// confirmed-existing object (omitted members stay unproven).
type corpusTablespaceObject struct {
	Status   string `yaml:"status"`
	Exists   *bool  `yaml:"exists,omitempty"`
	Kind     string `yaml:"kind,omitempty"`
	Occupied *bool  `yaml:"occupied,omitempty"`
}

// corpusResourceGroupObject is the catalog fact for one resource-group
// identity. Status is the provenance answer (confirmed | not_found |
// unavailable); definition is only read on a confirmed-existing object.
type corpusResourceGroupObject struct {
	Status     string                         `yaml:"status"`
	Exists     *bool                          `yaml:"exists,omitempty"`
	Definition *corpusResourceGroupDefinition `yaml:"definition,omitempty"`
}

// corpusResourceGroupDefinition mirrors the catalog definition parameters —
// omitted members stay unproven, never defaulted.
type corpusResourceGroupDefinition struct {
	RUPerSec    *uint64 `yaml:"ru_per_sec,omitempty"`
	RUUnlimited bool    `yaml:"ru_unlimited,omitempty"`
	Priority    string  `yaml:"priority,omitempty"`
	Burstable   *bool   `yaml:"burstable,omitempty"`
	QueryLimit  *string `yaml:"query_limit,omitempty"`
}

// corpusMySQLResourceGroupObject is the catalog fact for one MySQL
// resource-group identity — status is the provenance answer
// (confirmed | not_found | unavailable); definition is only read on a
// confirmed-existing object.
type corpusMySQLResourceGroupObject struct {
	Status     string                              `yaml:"status"`
	Exists     *bool                               `yaml:"exists,omitempty"`
	Definition *corpusMySQLResourceGroupDefinition `yaml:"definition,omitempty"`
}

// corpusMySQLResourceGroupDefinition mirrors the MySQL catalog definition
// columns — omitted members stay unproven, never defaulted.
type corpusMySQLResourceGroupDefinition struct {
	GroupType      string  `yaml:"group_type,omitempty"`
	Enabled        *bool   `yaml:"enabled,omitempty"`
	VCPUIDs        *string `yaml:"vcpu_ids,omitempty"`
	ThreadPriority *int64  `yaml:"thread_priority,omitempty"`
}

// corpusPlacementPolicy is the catalog fact for one proven policy.
type corpusPlacementPolicy struct {
	PrimaryRegion string   `yaml:"primary_region,omitempty"`
	Regions       string   `yaml:"regions,omitempty"`
	BoundTargets  []string `yaml:"bound_targets,omitempty"`
}

// corpusSequenceObject is the catalog fact for one sequence identity.
// Status is the provenance answer (confirmed | not_found | unavailable);
// definition and references are only read on a confirmed-existing object.
type corpusSequenceObject struct {
	Status            string                    `yaml:"status"`
	Exists            *bool                     `yaml:"exists,omitempty"`
	Definition        *corpusSequenceDefinition `yaml:"definition,omitempty"`
	References        []corpusSequenceRefEdge   `yaml:"references,omitempty"`
	ReferencesUnknown bool                      `yaml:"references_unknown,omitempty"`
	ConflictKind      string                    `yaml:"conflict_kind,omitempty"`
}

// corpusSequenceRefEdge is one vouched inbound reference: the owner-side
// identity and slot; the target is always the resolved sequence object.
type corpusSequenceRefEdge struct {
	OwnerKind   string `yaml:"owner_kind"`
	OwnerSchema string `yaml:"owner_schema,omitempty"`
	OwnerName   string `yaml:"owner_name"`
	Slot        string `yaml:"slot,omitempty"`
}

// corpusSequenceDefinition mirrors the catalog definition parameters —
// omitted members stay unproven, never defaulted.
type corpusSequenceDefinition struct {
	Start      *int64 `yaml:"start,omitempty"`
	Increment  *int64 `yaml:"increment,omitempty"`
	MinValue   *int64 `yaml:"min_value,omitempty"`
	MaxValue   *int64 `yaml:"max_value,omitempty"`
	CacheValue *int64 `yaml:"cache_value,omitempty"`
	Cache      *bool  `yaml:"cache,omitempty"`
	Cycle      *bool  `yaml:"cycle,omitempty"`
}

type corpusInstanceFacts struct {
	PGCompressionLZ4Known          bool   `yaml:"pg_compression_lz4_known,omitempty"`
	PGCompressionLZ4Supported      bool   `yaml:"pg_compression_lz4_supported,omitempty"`
	Version                        string `yaml:"version,omitempty"`
	DefaultCharset                 string `yaml:"default_charset,omitempty"`
	InnoDBLargePrefixEnabled       bool   `yaml:"innodb_large_prefix_enabled,omitempty"`
	InnoDBLargePrefixKnown         bool   `yaml:"innodb_large_prefix_known,omitempty"`
	InnoDBDefaultRowFormat         string `yaml:"innodb_default_row_format,omitempty"`
	InnoDBAdaptiveHashEnabled      bool   `yaml:"innodb_adaptive_hash_enabled,omitempty"`
	TiDBEnableCheckConstraint      bool   `yaml:"tidb_enable_check_constraint,omitempty"`
	TiDBEnableCheckConstraintKnown bool   `yaml:"tidb_enable_check_constraint_known,omitempty"`
	TiDBEnableResourceControl      bool   `yaml:"tidb_enable_resource_control,omitempty"`
	TiDBEnableResourceControlKnown bool   `yaml:"tidb_enable_resource_control_known,omitempty"`
	TiDBEnableClusteredIndex       string `yaml:"tidb_enable_clustered_index,omitempty"`
	TiDBAlterPrimaryKey            bool   `yaml:"tidb_alter_primary_key,omitempty"`
	TiDBAlterPrimaryKeyKnown       bool   `yaml:"tidb_alter_primary_key_known,omitempty"`
	// The T24/#103 MySQL resource-group premise facts — every one is a
	// separate known/answer pair so a fixture can prove a refusal, prove
	// a proceeding world, or leave the premise unproven to pin the gap.
	MySQLResourceGroupFeatureSupported bool            `yaml:"mysql_resource_group_feature_supported,omitempty"`
	MySQLResourceGroupFeatureKnown     bool            `yaml:"mysql_resource_group_feature_known,omitempty"`
	MySQLResourceGroupPriorityCapable  bool            `yaml:"mysql_resource_group_priority_capable,omitempty"`
	MySQLResourceGroupPriorityCapKnown bool            `yaml:"mysql_resource_group_priority_cap_known,omitempty"`
	MySQLHostVCPUs                     uint64          `yaml:"mysql_host_vcpus,omitempty"`
	MySQLHostVCPUsKnown                bool            `yaml:"mysql_host_vcpus_known,omitempty"`
	MySQLResourceGroupAdmin            bool            `yaml:"mysql_resource_group_admin,omitempty"`
	MySQLResourceGroupAdminKnown       bool            `yaml:"mysql_resource_group_admin_known,omitempty"`
	MySQLResourceGroupBound            map[string]bool `yaml:"mysql_resource_group_bound,omitempty"`
	MySQLResourceGroupBoundComplete    bool            `yaml:"mysql_resource_group_bound_complete,omitempty"`
	// The ordinary-client read-only gate premises (T24-R4): the global
	// variables and the subject's SUPER/CONNECTION_ADMIN exemption —
	// RESOURCE_GROUP_ADMIN alone never answers it.
	MySQLReadOnly            bool `yaml:"mysql_read_only,omitempty"`
	MySQLReadOnlyKnown       bool `yaml:"mysql_read_only_known,omitempty"`
	MySQLSuperReadOnly       bool `yaml:"mysql_super_read_only,omitempty"`
	MySQLSuperReadOnlyKnown  bool `yaml:"mysql_super_read_only_known,omitempty"`
	MySQLReadOnlyExempt      bool `yaml:"mysql_read_only_exempt,omitempty"`
	MySQLReadOnlyExemptKnown bool `yaml:"mysql_read_only_exempt_known,omitempty"`
}

type corpusPGDropColumnDependency struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Column string `yaml:"column"`
	Kind   string `yaml:"kind"`
}

type corpusPGDropConstraintReference struct {
	Schema  string `yaml:"schema"`
	Table   string `yaml:"table"`
	Name    string `yaml:"name"`
	Kind    string `yaml:"kind"`
	Blocked bool   `yaml:"blocked"`
}

type corpusPGForeignKeyKeys struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Keys   []struct {
		Columns   []string `yaml:"columns"`
		Primary   bool     `yaml:"primary"`
		Immediate bool     `yaml:"immediate"`
	} `yaml:"keys"`
}

type corpusPGConstraintIndex struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Name   string `yaml:"name"`
	Kind   string `yaml:"kind"`
	Valid  bool   `yaml:"valid"`
}

type corpusPGColumnType struct {
	Schema       string `yaml:"schema"`
	Name         string `yaml:"name"`
	Array        bool   `yaml:"array,omitempty"`
	SchemaExists bool   `yaml:"schema_exists"`
	BaseExists   bool   `yaml:"base_exists"`
	Resolved     bool   `yaml:"resolved"`
	Defined      bool   `yaml:"defined"`
	Pseudo       bool   `yaml:"pseudo"`
}

type corpusPGBulkTablespace struct {
	Source            string `yaml:"source"`
	Destination       string `yaml:"destination"`
	SourceExists      bool   `yaml:"source_exists"`
	DestinationExists bool   `yaml:"destination_exists"`
}

type corpusPGTableOwner struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Kind   string `yaml:"kind"`
	Name   string `yaml:"name,omitempty"`
	Exists bool   `yaml:"exists"`
	Same   bool   `yaml:"same,omitempty"`
}

type corpusPGTableNamespace struct {
	Schema               string `yaml:"schema"`
	Table                string `yaml:"table"`
	Action               string `yaml:"action"`
	Destination          string `yaml:"destination"`
	SchemaExists         bool   `yaml:"schema_exists,omitempty"`
	SameSchema           bool   `yaml:"same_schema,omitempty"`
	RelationConflict     bool   `yaml:"relation_conflict,omitempty"`
	RowTypeConflict      bool   `yaml:"row_type_conflict,omitempty"`
	RowTypeConflictKnown bool   `yaml:"row_type_conflict_known,omitempty"`
}

type corpusPGTablePersistence struct {
	Schema             string `yaml:"schema"`
	Table              string `yaml:"table"`
	Current            string `yaml:"current"`
	Logged             bool   `yaml:"logged,omitempty"`
	DirectPublication  bool   `yaml:"direct_publication,omitempty"`
	ForeignKeyConflict bool   `yaml:"foreign_key_conflict,omitempty"`
	ForeignKeyUnknown  bool   `yaml:"foreign_key_unknown,omitempty"`
}

type corpusPGTableAccessMethod struct {
	Schema  string `yaml:"schema"`
	Table   string `yaml:"table"`
	Name    string `yaml:"name"`
	Default bool   `yaml:"default,omitempty"`
	Kind    string `yaml:"kind"`
	Same    *bool  `yaml:"same,omitempty"`
}

type corpusPGTableTablespace struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Name   string `yaml:"name"`
	Exists bool   `yaml:"exists"`
	Global bool   `yaml:"global,omitempty"`
	Same   *bool  `yaml:"same,omitempty"`
}

type corpusPGReplicaIndex struct {
	Schema     string `yaml:"schema"`
	Table      string `yaml:"table"`
	Name       string `yaml:"name"`
	Kind       string `yaml:"kind"`
	OnTable    bool   `yaml:"on_table"`
	Unique     bool   `yaml:"unique"`
	Immediate  bool   `yaml:"immediate"`
	Expression bool   `yaml:"expression"`
	Partial    bool   `yaml:"partial"`
	CanUnique  *bool  `yaml:"can_unique,omitempty"`
	KeyValid   *bool  `yaml:"key_valid,omitempty"`
}

type corpusTableSnapshot struct {
	Schema             string             `yaml:"schema,omitempty"`
	Exists             bool               `yaml:"exists"`
	Table              *corpusTable       `yaml:"table,omitempty"`
	Columns            []corpusColumn     `yaml:"columns,omitempty"`
	PrimaryKey         *corpusIndex       `yaml:"primary_key,omitempty"`
	PrimaryKeyUnknown  bool               `yaml:"primary_key_unknown,omitempty"`
	Indexes            []corpusIndex      `yaml:"indexes,omitempty"`
	Constraints        []corpusConstraint `yaml:"constraints,omitempty"`
	ConstraintsUnknown bool               `yaml:"constraints_unknown,omitempty"`
	Options            map[string]string  `yaml:"options,omitempty"`
	TTL                *corpusTTLConfig   `yaml:"ttl,omitempty"`
	TTLKnown           bool               `yaml:"ttl_known,omitempty"`
	// ReferencedBy carries the provider's reverse-edge read: present-but-
	// empty means the catalog proved no inbound foreign keys, absent means
	// the read never happened and the bound shape stays unvouched.
	ReferencedBy        []corpusTable `yaml:"referenced_by,omitempty"`
	ReferencedByUnknown bool          `yaml:"referenced_by_unknown,omitempty"`
	Partitioned         *bool         `yaml:"partitioned,omitempty"`
	PartitionType       string        `yaml:"partition_type,omitempty"`
	PartitionNames      []string      `yaml:"partition_names,omitempty"`
	// PartitionDescriptions carries the catalog PARTITION_DESCRIPTION
	// text per member — the fixture describes exactly what
	// information_schema would show; the converter projects it through
	// spec.CatalogPartitionDefs so fixtures never hand-author the model.
	PartitionDescriptions []string `yaml:"partition_descriptions,omitempty"`
	// PlacementPolicy is the catalog TIDB_PLACEMENT_POLICY_NAME read for
	// the table; placement_policy_known marks the read as proven. A bound
	// member directory lands in partition_placements keyed by member name.
	PlacementPolicy      string            `yaml:"placement_policy,omitempty"`
	PlacementPolicyKnown bool              `yaml:"placement_policy_known,omitempty"`
	PartitionPlacements  map[string]string `yaml:"partition_placements,omitempty"`

	// nil is unobserved; an explicit empty array is a loaded empty directory.
	PGConstraints []corpusPGConstraint   `yaml:"pg_constraints,omitempty"`
	PGColumns     *corpusPGColumnCatalog `yaml:"pg_columns,omitempty"`
	PGRelation    corpusPGRelation       `yaml:"pg_relation,omitempty"`
}

type corpusPGRelation struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind,omitempty"`
	Heap bool   `yaml:"heap,omitempty"`
}

type corpusTable struct {
	Schema  string `yaml:"schema,omitempty"`
	Name    string `yaml:"name"`
	Comment string `yaml:"comment,omitempty"`
}

type corpusTTLConfig struct {
	TimeColumn    string `yaml:"time_column,omitempty"`
	IntervalValue string `yaml:"interval_value,omitempty"`
	IntervalUnit  string `yaml:"interval_unit,omitempty"`
	Enabled       *bool  `yaml:"enabled,omitempty"`
	JobInterval   string `yaml:"job_interval,omitempty"`
}

type corpusColumn struct {
	Name                      string         `yaml:"name"`
	Type                      string         `yaml:"type,omitempty"`
	Length                    int            `yaml:"length,omitempty"`
	Charset                   string         `yaml:"charset,omitempty"`
	Collation                 string         `yaml:"collation,omitempty"`
	Comment                   string         `yaml:"comment,omitempty"`
	Unsigned                  bool           `yaml:"unsigned,omitempty"`
	NotNull                   bool           `yaml:"not_null,omitempty"`
	AutoIncrement             bool           `yaml:"auto_increment,omitempty"`
	HasDefault                bool           `yaml:"has_default,omitempty"`
	DefaultValue              string         `yaml:"default_value,omitempty"`
	DefaultIsNull             bool           `yaml:"default_is_null,omitempty"`
	DefaultIsCurrentTimestamp bool           `yaml:"default_is_current_timestamp,omitempty"`
	OnUpdateCurrentTimestamp  bool           `yaml:"on_update_current_timestamp,omitempty"`
	GeneratedWhen             string         `yaml:"generated_when,omitempty"`
	IsIdentity                bool           `yaml:"is_identity,omitempty"`
	IdentityOptions           map[string]any `yaml:"identity_options,omitempty"`
}

type corpusIndex struct {
	Name        string   `yaml:"name"`
	Kind        string   `yaml:"kind,omitempty"`
	Columns     []string `yaml:"columns,omitempty"`
	Cardinality *int64   `yaml:"cardinality,omitempty"`
}

type corpusPGColumnCatalog struct {
	Ordinary      *bool                 `yaml:"ordinary"`
	Columns       []corpusPGColumnState `yaml:"columns"`
	SystemColumns []corpusPGColumnState `yaml:"system_columns,omitempty"`
}

type corpusPGColumnState struct {
	Name               string                         `yaml:"name"`
	Identity           *string                        `yaml:"identity"`
	Generated          *string                        `yaml:"generated"`
	TypeWidth          *int                           `yaml:"type_width"`
	TypeStorage        string                         `yaml:"type_storage,omitempty"`
	System             bool                           `yaml:"system,omitempty"`
	NotNull            *bool                          `yaml:"not_null"`
	HasDefault         *bool                          `yaml:"has_default"`
	PrimaryKey         *bool                          `yaml:"primary_key"`
	ReplicaIdentityKey *bool                          `yaml:"replica_identity_key,omitempty"`
	IdentitySequence   *corpusPGIdentitySequenceState `yaml:"identity_sequence,omitempty"`
}

type corpusPGIdentitySequenceState struct {
	TypeWidth  int    `yaml:"type_width"`
	Start      *int64 `yaml:"start"`
	Increment  *int64 `yaml:"increment"`
	MinValue   *int64 `yaml:"minvalue"`
	MaxValue   *int64 `yaml:"maxvalue"`
	CacheValue *int64 `yaml:"cache"`
	Cycle      *bool  `yaml:"cycle"`
}

type corpusPGConstraint struct {
	Name              string `yaml:"name"`
	Type              string `yaml:"type"`
	Inherited         *bool  `yaml:"inherited,omitempty"`
	PartitionChildren *bool  `yaml:"partition_children,omitempty"`
	Validated         *bool  `yaml:"validated"`
}

type corpusConstraint struct {
	Type              string   `yaml:"type"`
	Name              string   `yaml:"name,omitempty"`
	Columns           []string `yaml:"columns,omitempty"`
	Enforced          string   `yaml:"enforced,omitempty"`
	ReferencedSchema  string   `yaml:"referenced_schema,omitempty"`
	ReferencedTable   string   `yaml:"referenced_table,omitempty"`
	ReferencedColumns []string `yaml:"referenced_columns,omitempty"`
}

// corpusFactConstraint is one expected constraint fact.
type corpusFactConstraint struct {
	Type              string   `yaml:"type"`
	Name              string   `yaml:"name,omitempty"`
	Columns           []string `yaml:"columns,omitempty"`
	ReferencedSchema  string   `yaml:"referenced_schema,omitempty"`
	ReferencedTable   string   `yaml:"referenced_table,omitempty"`
	ReferencedColumns []string `yaml:"referenced_columns,omitempty"`
}

// corpusUnsupported is one expected unsupported-detail metadata entry.
type corpusUnsupported struct {
	Feature  string         `yaml:"feature"`
	Metadata map[string]any `yaml:"metadata,omitempty"`
}

var validDialects = map[string]bool{
	"mysql":      true,
	"tidb":       true,
	"postgresql": true,
}

var validCategories = map[string]bool{
	"ddl": true,
	"dml": true,
}

var validStatementKinds = map[string]bool{
	"ddl":     true,
	"dml":     true,
	"unknown": true,
}

// corpusExpectedFiles walks corpusRoot recursively and returns all
// .expected.yaml paths.
func corpusExpectedFiles(corpusRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(corpusRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".expected.yaml") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func TestSQLCorpusExpectedFilesAreWellFormed(t *testing.T) {
	t.Parallel()
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")

	entries, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found under testdata/sql-corpus; the corpus loader has nothing to validate")
	}

	for _, expPath := range entries {
		name := filepath.Base(expPath)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// 1. Sibling .sql must exist.
			sqlPath := expPath[:len(expPath)-len(".expected.yaml")] + ".sql"
			if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
				t.Fatalf("missing sibling .sql file: %s", sqlPath)
			}

			// 2. Parse the YAML.
			raw, err := os.ReadFile(expPath)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}
			var tc corpusExpected
			if err := yaml.Unmarshal(raw, &tc); err != nil {
				t.Fatalf("YAML parse error: %v", err)
			}

			// 3. Validate required top-level fields.
			if tc.Name == "" {
				t.Fatal("missing required field: name")
			}
			if !validDialects[tc.Dialect] {
				t.Fatalf("invalid dialect %q; must be one of mysql, tidb, postgresql", tc.Dialect)
			}
			if !validCategories[tc.Category] {
				t.Fatalf("invalid category %q; must be one of ddl, dml", tc.Category)
			}

			// 4. Validate unsupported fields if present.
			if tc.Expect.Unsupported != nil {
				if tc.Expect.Unsupported.Count != nil && *tc.Expect.Unsupported.Count < 0 {
					t.Fatalf("unsupported.count must be >= 0, got %d", *tc.Expect.Unsupported.Count)
				}
				for _, feat := range tc.Expect.Unsupported.Include {
					if feat == "" {
						t.Fatal("unsupported.include must not contain empty strings")
					}
				}
				for _, m := range tc.Expect.Unsupported.Metadata {
					if m.Feature == "" {
						t.Fatal("unsupported.metadata[].feature must not be empty")
					}
				}
			}
			if tc.Expect.Statements != nil && tc.Expect.Statements.Count != nil && *tc.Expect.Statements.Count < 0 {
				t.Fatalf("statements.count must be >= 0, got %d", *tc.Expect.Statements.Count)
			}
			if tc.Expect.Diagnostics != nil && tc.Expect.Diagnostics.ParserErrorCount != nil && *tc.Expect.Diagnostics.ParserErrorCount < 0 {
				t.Fatalf("diagnostics.parser_error_count must be >= 0, got %d", *tc.Expect.Diagnostics.ParserErrorCount)
			}

			// 5. Validate operation if present — must not be empty.
			if tc.Expect.Operation != "" { //nolint:staticcheck // non-empty is valid
				// non-empty is valid
			}

			// 6. Validate statement_kind if present — must be a known enum.
			if tc.Expect.StatementKind != "" {
				if !validStatementKinds[tc.Expect.StatementKind] {
					t.Fatalf("invalid statement_kind %q; must be one of ddl, dml, unknown", tc.Expect.StatementKind)
				}
			}

			// 7. Validate findings lists if present.
			if tc.Expect.Findings != nil {
				for _, id := range tc.Expect.Findings.Include {
					if id == "" {
						t.Fatal("findings.include must not contain empty strings")
					}
				}
				for _, id := range tc.Expect.Findings.Exclude {
					if id == "" {
						t.Fatal("findings.exclude must not contain empty strings")
					}
				}
			}

			if impact := tc.Expect.Impact; impact != nil {
				if impact.RiskLevel != "" && !map[string]bool{"low": true, "medium": true, "high": true, "unknown": true}[impact.RiskLevel] {
					t.Fatalf("invalid impact.risk_level %q", impact.RiskLevel)
				}
				if impact.Confidence != "" && !map[string]bool{"low": true, "medium": true, "high": true}[impact.Confidence] {
					t.Fatalf("invalid impact.confidence %q", impact.Confidence)
				}
				if impact.Source != "" && !map[string]bool{"shape": true, "metadata": true, "plan": true}[impact.Source] {
					t.Fatalf("invalid impact.source %q", impact.Source)
				}
				for _, code := range impact.ReasonCodes {
					if code == "" {
						t.Fatal("impact.reason_codes must not contain empty strings")
					}
				}
			}

			// 8. Validate facts.constraints if present.
			if tc.Facts != nil {
				for _, c := range tc.Facts.Constraints {
					if c.Type == "" {
						t.Fatal("facts.constraints[].type must not be empty")
					}
					for _, col := range c.Columns {
						if col == "" {
							t.Fatal("facts.constraints[].columns must not contain empty strings")
						}
					}
					for _, col := range c.ReferencedColumns {
						if col == "" {
							t.Fatal("facts.constraints[].referenced_columns must not contain empty strings")
						}
					}
				}
			}

			// 9. Validate optional inline config shape if present.
			if tc.Config != nil {
				if _, ok := tc.Config["rules"]; !ok {
					t.Fatal("config must contain rules when present")
				}
			}

			// 10. Validate optional metadata fixture shape if present.
			if tc.Metadata != nil {
				for tableName := range tc.Metadata.Tables {
					if tableName == "" {
						t.Fatal("metadata.tables must not contain an empty table key")
					}
				}
				for indexName, tableName := range tc.Metadata.IndexOwners {
					if indexName == "" || tableName == "" {
						t.Fatal("metadata.index_owners must not contain empty index or table names")
					}
				}
			}

			t.Logf("OK: %s (%s/%s)", tc.Name, tc.Dialect, tc.Category)
		})
	}
}

// TestSQLCorpusWalkerFindsActualCases is a sanity check that the walker
// discovers the sample fixture added in Task 1.
func TestSQLCorpusWalkerFindsActualCases(t *testing.T) {
	t.Parallel()
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")
	matches, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("walk found no .expected.yaml files under %s; check directory layout", corpusRoot)
	}
	t.Logf("found %d expected file(s)", len(matches))
}
