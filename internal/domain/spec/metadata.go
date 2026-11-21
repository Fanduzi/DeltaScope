// Package spec defines normalized statement specifications for rule evaluation.
package spec

import "strings"

// Metadata carries optional non-SQL facts for one statement evaluation.
type Metadata struct {
	PGPartitionConstraint       *PGPartitionConstraintState       `json:"-"`
	PGDropViewSet               *PGDropViewSetState               `json:"-"`
	PGCreateViewTarget          *PGCreateViewTarget               `json:"-"`
	PGMaterializedViewExtension *PGMaterializedViewExtensionState `json:"-"`
	PGViewDefault               *PGViewDefaultState               `json:"-"`
	PGDropViewState             *PGDropViewState                  `json:"-"`
	PGForeignKeyEquality        *PGForeignKeyEquality             `json:"-"`
	Schema                      string                            `json:"schema,omitempty"`
	Instance                    *InstanceFacts                    `json:"instance,omitempty"`
	Version                     *VersionIdentity                  `json:"version,omitempty"`
	TargetTable                 *TableSnapshot                    `json:"target_table,omitempty"`
	PGForeignKeyReference       *TableSnapshot                    `json:"-"`
	PGForeignKeyKeys            *PGForeignKeyKeys                 `json:"-"`
	PGDropConstraintReference   *PGDropConstraintReference        `json:"-"`
	PGDropColumnDependency      *PGDropColumnDependency           `json:"-"`
	PGColumnTraversal           *PGColumnTraversal                `json:"-"`
	PGReplicaIdentity           *PGReplicaIdentityIndex           `json:"-"`
	PGConstraintUsingIndex      *PGConstraintUsingIndex           `json:"-"`
	PGTableTablespace           *PGTableTablespace                `json:"-"`
	PGTableAccessMethod         *PGTableAccessMethod              `json:"-"`
	PGTablePersistence          *PGTablePersistence               `json:"-"`
	PGTableNamespace            *PGTableNamespace                 `json:"-"`
	PGTableOwner                *PGTableOwnerState                `json:"-"`
	PGColumnType                *PGColumnTypeState                `json:"-"`
	PGBulkTablespace            *PGBulkTablespaceState            `json:"-"`
	View                        *ViewMetadata                     `json:"-"`
	PGDestructiveReferences     *PGDestructiveReferences          `json:"-"`
	PGDestructiveTableRelations []PGDestructiveTableRelation      `json:"-"`
	PGRefreshState              *PGRefreshState                   `json:"-"`
	// ExchangeTable is the ordered pre-state projection of the
	// EXCHANGE PARTITION counterpart table — the second object identity
	// the action touches, resolved through the same once-per-key seam as
	// the primary target. nil means the counterpart was never resolved
	// (no exchange action on the statement, or the identity is unknown).
	ExchangeTable *TableSnapshot   `json:"exchange_table,omitempty"`
	Objects       []ObjectSnapshot `json:"objects,omitempty"`

	// TargetTables binds every identity in DDL.TableTargets() to its
	// ordered pre-state projection in source order — the namespace-aware
	// answer for multi-target statements (multi-table DROP, RENAME pairs).
	// A binding's Snapshot is nil when the target's state is unvouched;
	// Ambiguous marks a binding the model could not even decide (a name
	// that could hit either namespace).
	TargetTables []TargetBinding `json:"target_tables,omitempty"`
	// SourceTable is the CREATE TABLE ... LIKE source's ordered pre-state —
	// bound through the same temp-first namespace rule every unqualified
	// read uses. nil when the LIKE source is unvouched (or the statement
	// is not a LIKE create).
	SourceTable *TableSnapshot `json:"source_table,omitempty"`
	// SourceTables binds each CTAS SelectSources entry to its ordered
	// pre-state, aligned 1:1 in source order. A nil element marks an
	// unvouched source; an empty slice means the select named no base
	// table (literal select — no source premise at all).
	SourceTables []*TableSnapshot `json:"source_tables,omitempty"`
	// PlacementPolicies carries the proven placement-policy catalog on TiDB:
	// presence of a name key is the proven-existence answer, the whole map
	// being nil means the catalog was never loaded, and BoundTargets is the
	// proven inbound-reference set the 8241 drop refusal consults
	// (issue #99 T20).
	PlacementPolicies map[string]*PlacementPolicyFact `json:"-"`
	// PlacementTopology carries the proven topology-prerequisite facts for
	// a placement action — label universe plus observed scheduling state —
	// a separate evidence layer from the catalog. nil means topology was
	// never read; rules translate that absence into a structured
	// placement_policy.topology evidence gap instead of implying verified
	// deployability (issue #99 T20-R3).
	PlacementTopology *PlacementTopologyFacts `json:"-"`
	// Placement carries the per-statement adjudicated placement facts —
	// every collected authored binding's outcome, the aggregate premise,
	// the lifecycle target's own policy premise, and the enclosing atomic
	// mutation's static outcome. An owned projection built once per
	// statement before its transition; never live registry pointers
	// (issue #99 T20-R4).
	Placement *PlacementFacts `json:"-"`
	// SchemaTarget binds a schema-scoped statement (CREATE/ALTER/DROP
	// DATABASE|SCHEMA) to its ordered pre-state projection: confirmed
	// schema attributes when the identity is provably present, an empty
	// snapshot when provably absent, and nil when the schema's state is
	// unvouched (or the statement is not schema-scoped). A name-less
	// ALTER DATABASE binds the request's trusted schema — never a
	// connection guess.
	SchemaTarget *ObjectSnapshot `json:"schema_target,omitempty"`
	// Sequences binds each sequence identity the statement touches to its
	// ordered pre-state, keyed `schema.name` (effective schema, lowercase).
	// Values are owned projections — never live registry pointers. Runtime
	// facts (current/next value, persisted position) are absent by design;
	// rules that need them emit bounded evidence gaps (T21/#100).
	Sequences map[string]*SequenceState `json:"sequences,omitempty"`
	// ResourceGroups binds each TiDB resource-group identity the statement
	// touches to its ordered pre-state, keyed by lowercase group name —
	// resource groups are globally namespaced, never schema-qualified.
	// Values are owned projections — never live registry pointers. The
	// resource-control capability premise lives in Instance (T22/#101).
	ResourceGroups map[string]*ResourceGroupState `json:"resource_groups,omitempty"`
	// MySQLResourceGroups binds each MySQL resource-group identity the
	// statement touches to its ordered pre-state, keyed by lowercase
	// group name. The version/privilege/host-cpu premises live in
	// Instance and evidence gaps — never guessed (T24/#103).
	MySQLResourceGroups map[string]*MySQLResourceGroupState `json:"mysql_resource_groups,omitempty"`
	// Tablespaces binds each MySQL tablespace identity the statement
	// touches to its ordered pre-state, keyed by lowercase name — the
	// occupancy (table association) premise rides beside existence and
	// stays unknown unless proven (T25/#104).
	Tablespaces map[string]*TablespaceState `json:"tablespaces,omitempty"`
	// Server carries the bound MySQL SERVER existence fact for the one
	// server identity a SERVER-family statement names — the provider
	// object read projected into per-fact knownness (T26/#105 provider
	// round). Internal evidence only: the family names stay inside the
	// audit run, never serialized.
	Server *ServerState `json:"-"`
	// ServerDrop carries the shared per-statement adjudication for the
	// audited identifier-form DROP SERVER slice — the one truth the
	// rule's gap/message/output and the ordered channel's post-state
	// both read (T26-SERVER-DROP-EXECUTION-01). Nil outside that
	// slice: legacy consumers keep the existence fact alone.
	ServerDrop *ServerDropAdjudication `json:"-"`
	// ServerCreate carries the shared per-statement adjudication for
	// the audited identifier-form CREATE SERVER slice — the same
	// one-truth contract as ServerDrop
	// (T26-SERVER-CREATE-EXECUTION-01). Nil outside that slice.
	ServerCreate *ServerCreateAdjudication `json:"-"`
	// ServerAlter carries the shared per-statement adjudication for
	// the audited identifier-form ALTER SERVER slice — the same
	// one-truth contract as ServerDrop/ServerCreate
	// (T26-SERVER-ALTER-EXECUTION-01). Nil outside that slice.
	ServerAlter *ServerAlterAdjudication `json:"-"`
	// SpatialReference carries the bound MySQL SRS facts for the one
	// SRID identity a spatial reference system statement names —
	// existence plus the optional conflict and dependency witness
	// reads (T26/#105 provider round). Internal evidence only.
	SpatialReference *SpatialReferenceSystemState `json:"-"`
	// SRSDrop carries the shared per-statement adjudication for the
	// audited qualified DROP SPATIAL REFERENCE SYSTEM slice
	// (T26-SRS-DROP-EXECUTION-01). Nil outside that slice.
	SRSDrop *SRSDropAdjudication `json:"-"`
	// SRSCreate carries the shared per-statement adjudication for the
	// audited qualified CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM
	// slice (T26-SRS-CREATE-PUBLICATION-01 frozen contract). Nil
	// outside that slice.
	SRSCreate *SRSCreateAdjudication `json:"-"`
	// AlterInstance carries the shared per-statement adjudication for
	// the audited MySQL ALTER INSTANCE slice — RELOAD TLS (including
	// the no_rollback_on_error variant) and ROTATE INNODB MASTER KEY
	// (T26-ALTER-INSTANCE-PUBLICATION-01 frozen contract). Nil outside
	// that slice.
	AlterInstance *AlterInstanceAdjudication `json:"-"`
}

// MetadataSchemaTarget is the ordered schema binding accessor rules use —
// nil-safe over a missing metadata block or a non-schema statement.
func (s Statement) MetadataSchemaTarget() *ObjectSnapshot {
	if s.Metadata == nil {
		return nil
	}
	return s.Metadata.SchemaTarget
}

// TargetBinding is the namespace-aware pre-state answer for one named
// identity: which object the statement's reference binds and what was
// provable about it. Snapshot.Temporary marks the session-local temp
// namespace; Snapshot nil means unvouched; Ambiguous means the binding
// itself could not be decided (never conflated with a proven absence).
type TargetBinding struct {
	Table     Table          `json:"table"`
	Snapshot  *TableSnapshot `json:"snapshot,omitempty"`
	Ambiguous bool           `json:"ambiguous,omitempty"`
	NotView   bool           `json:"-"`
}

// InstanceFacts are normalized server-level facts that influence audit behavior.
// Numeric facts pair a value with an explicit Known flag: a zero value is never
// treated as evidence — unknown facts stay unknown so rules can emit bounded
// evidence gaps instead of fabricating a configuration.
type InstanceFacts struct {
	Version string `json:"version,omitempty"`
	// PG compression support is observed from the complete GUC enum, not its selected default.
	PGCompressionLZ4Supported bool `json:"-"`
	PGCompressionLZ4Known     bool `json:"-"`
	// On MySQL, DefaultCharset is character_set_database, not the server default.
	DefaultCharset string `json:"default_charset,omitempty"`
	// Empty server default fields mean unobserved.
	ServerCharset              string `json:"server_charset,omitempty"`
	ServerCollation            string `json:"server_collation,omitempty"`
	DefaultCollationForUTF8MB4 string `json:"default_collation_for_utf8mb4,omitempty"`
	InnoDBLargePrefixEnabled   bool   `json:"innodb_large_prefix_enabled,omitempty"`
	InnoDBLargePrefixKnown     bool   `json:"innodb_large_prefix_known,omitempty"`
	InnoDBDefaultRowFormat     string `json:"innodb_default_row_format,omitempty"`
	InnoDBAdaptiveHashEnabled  bool   `json:"innodb_adaptive_hash_enabled,omitempty"`
	InnoDBPageSizeBytes        int    `json:"innodb_page_size,omitempty"`
	InnoDBPageSizeKnown        bool   `json:"innodb_page_size_known,omitempty"`
	TiDBMaxIndexLengthBytes    int    `json:"tidb_max_index_length,omitempty"`
	TiDBMaxIndexLengthKnown    bool   `json:"tidb_max_index_length_known,omitempty"`

	// TiDBEnableClusteredIndex is the session-observed
	// tidb_enable_clustered_index value (on/off/int_only). A declared
	// TiDB primary key without CLUSTERED|NONCLUSTERED stores under this
	// session default — an empty value means the mode was not probed, so
	// the handle answer stays unknown rather than inheriting the
	// product's ON default as a guess. TiDB only; never populated on
	// MySQL.
	TiDBEnableClusteredIndex string `json:"tidb_enable_clustered_index,omitempty"`

	// TiDBAlterPrimaryKey is the observed alter-primary-key configuration
	// fact the int_only clustered mode defers to: int_only clusters a
	// single-int PK only while alter-primary-key is disabled. An
	// unobserved fact keeps the int_only handle answer unknown — the
	// Known bit separates "probed false" from "never probed" so a zero
	// value is never read as evidence. TiDB only.
	TiDBAlterPrimaryKey      bool `json:"tidb_alter_primary_key,omitempty"`
	TiDBAlterPrimaryKeyKnown bool `json:"tidb_alter_primary_key_known,omitempty"`

	// RestrictNonStandardKey is the observed
	// restrict_fk_on_non_standard_key value — MySQL 8.4+'s refusal of
	// foreign keys whose referenced columns are covered only by a
	// non-unique index (native 6125, verified 8.4.10 ON; the variable
	// does not exist on 5.7/8.0/TiDB, so an unobserved read resolves the
	// eligibility question by dialect and version instead of guessing a
	// default). The Known bit separates "probed off" from "never probed".
	RestrictNonStandardKey      bool `json:"restrict_non_standard_key,omitempty"`
	RestrictNonStandardKeyKnown bool `json:"restrict_non_standard_key_known,omitempty"`

	// TiDBEnableCheckConstraint is the observed
	// tidb_enable_check_constraint value — TiDB's CHECK runtime switch.
	// OFF means CHECK syntax parses but stores and enforces nothing; ON
	// means CHECK members land and enforce. The Known bit separates
	// "probed off" from "never probed" so an unobserved value resolves to
	// undecidable, never a guessed ON. TiDB only; MySQL never returns the
	// row.
	TiDBEnableCheckConstraint      bool `json:"tidb_enable_check_constraint,omitempty"`
	TiDBEnableCheckConstraintKnown bool `json:"tidb_enable_check_constraint_known,omitempty"`

	// TiDBEnableResourceControl is the observed
	// tidb_enable_resource_control value — TiDB's gate for the whole
	// resource-group surface (CREATE/ALTER/DROP RESOURCE GROUP and RU
	// accounting). OFF means resource-group DDL refuses with a
	// resource-control-disabled error; the Known bit separates "probed
	// off" from "never probed" so an unobserved value resolves to
	// undecidable, never a guessed ON. TiDB only; MySQL never returns
	// the row.
	TiDBEnableResourceControl      bool `json:"tidb_enable_resource_control,omitempty"`
	TiDBEnableResourceControlKnown bool `json:"tidb_enable_resource_control_known,omitempty"`

	// TiDBSplitRegionMaxNum is the observed tidb-server
	// split-region-max-num config — the upper bound on authored SPLIT
	// ... REGIONS counts (v8.5.0 planbuilder.go checks it against
	// config.GetGlobalConfig().SplitRegionMaxNum). Read through
	// SHOW CONFIG; every tidb-server instance row must parse and agree
	// for the fact to become Known. MySQL never consults it. TiDB
	// only (issue #102 T23-R8).
	TiDBSplitRegionMaxNum      int  `json:"tidb_split_region_max_num,omitempty"`
	TiDBSplitRegionMaxNumKnown bool `json:"tidb_split_region_max_num_known,omitempty"`

	// MySQLResourceGroupFeatureSupported is the deployment answer to
	// upstream's check_resource_group_support() — whether this mysqld
	// build/platform runs the resource-group feature at all. It is a
	// separate premise from the version gate: 8.0.3+ is the version the
	// syntax appeared on, while a deployment may still lack the feature.
	// MySQL only (issue #103 T24-R2).
	MySQLResourceGroupFeatureSupported bool `json:"mysql_resource_group_feature_supported,omitempty"`
	MySQLResourceGroupFeatureKnown     bool `json:"mysql_resource_group_feature_known,omitempty"`

	// MySQLResourceGroupPriorityCapable is the deployment answer to
	// upstream's thread_priority_available() — whether this platform
	// applies nonzero THREAD_PRIORITY. When proven off, an authored
	// nonzero priority is ignored with ER_ATTRIBUTE_IGNORED and the
	// effective priority stores zero; when unproven the effective value
	// stays withheld rather than guessed. MySQL only.
	MySQLResourceGroupPriorityCapable  bool `json:"mysql_resource_group_priority_capable,omitempty"`
	MySQLResourceGroupPriorityCapKnown bool `json:"mysql_resource_group_priority_cap_known,omitempty"`

	// MySQLHostVCPUs is the CPU count of the mysqld host the authored
	// VCPU endpoints adjudicate against — upstream's num_vcpus(). It is
	// never the audit-runner's core count; an unproven read keeps every
	// endpoint check undecidable. MySQL only.
	MySQLHostVCPUs      uint64 `json:"mysql_host_vcpus,omitempty"`
	MySQLHostVCPUsKnown bool   `json:"mysql_host_vcpus_known,omitempty"`

	// MySQLResourceGroupAdmin is the RESOURCE_GROUP_ADMIN grant the
	// lifecycle commands require — distinct from an ordinary catalog-read
	// privilege. MySQL only.
	MySQLResourceGroupAdmin      bool `json:"mysql_resource_group_admin,omitempty"`
	MySQLResourceGroupAdminKnown bool `json:"mysql_resource_group_admin_known,omitempty"`

	// MySQLResourceGroupBound carries per-group is_bound_to_threads()
	// answers keyed by the lowercase group name — DROP consults it for
	// ER_RESOURCE_GROUP_BUSY. A missing key is unproven binding state,
	// never a proven "unoccupied". MySQL only.
	MySQLResourceGroupBound map[string]bool `json:"mysql_resource_group_bound,omitempty"`

	// MySQLResourceGroupBoundComplete marks the Bound map as a proven
	// complete observation of the bound set — a complete successful read
	// of performance_schema.threads where the observing account's SELECT
	// privilege shows every row. With it set, a missing key is a proven
	// "unoccupied" answer; without it the map is partial evidence and a
	// missing key stays unproven. MySQL only.
	MySQLResourceGroupBoundComplete bool `json:"mysql_resource_group_bound_complete,omitempty"`

	// MySQLReadOnly and MySQLSuperReadOnly are the observed global
	// variables feeding upstream's opt_readonly/opt_super_readonly —
	// check_readonly() refuses every lifecycle command at execute for an
	// ordinary client when the gate closes. They are raw variable
	// answers, never a precomputed refusal. MySQL only (T24-R4).
	MySQLReadOnly           bool `json:"mysql_read_only,omitempty"`
	MySQLReadOnlyKnown      bool `json:"mysql_read_only_known,omitempty"`
	MySQLSuperReadOnly      bool `json:"mysql_super_read_only,omitempty"`
	MySQLSuperReadOnlyKnown bool `json:"mysql_super_read_only_known,omitempty"`

	// MySQLReadOnlyExempt is the observed answer for the executing
	// subject's read-only exemption — SUPER_ACL or a CONNECTION_ADMIN
	// grant in upstream's check_readonly. It is distinct from
	// RESOURCE_GROUP_ADMIN, which carries no read-only exemption.
	// MySQL only (T24-R4).
	MySQLReadOnlyExempt      bool `json:"mysql_read_only_exempt,omitempty"`
	MySQLReadOnlyExemptKnown bool `json:"mysql_read_only_exempt_known,omitempty"`

	// MySQLServerUUID is the observed @@server_uuid of the audited
	// connection — a non-secret, provider-observed target identity that
	// DeploymentEvidence.Target binds against online. MySQL only
	// (T24-R6).
	MySQLServerUUID      string `json:"mysql_server_uuid,omitempty"`
	MySQLServerUUIDKnown bool   `json:"mysql_server_uuid_known,omitempty"`
}

// DeploymentEvidence is a caller-sourced fact bundle for the premises
// no SQL channel can observe — host CPU count, priority capability, and
// feature support. The bundle binds to the audited target: Source must
// identify its provenance, Product must equal the dialect's product,
// and Version must equal the target's resolved version exactly; a
// bundle that fails any binding is ignored, never applied. Evidence
// only fills still-unproven slots — it never overwrites an online
// observation, carries no generic trust flag, is consumed per request
// rather than cached, and is never reused across instances. It carries
// only the facts T24 adjudication requires; public diagnostics never
// echo the bundle back.
type DeploymentEvidence struct {
	// Source identifies where the bundle came from (for example a
	// capacity report or provisioning record) — required, free-form.
	Source string `json:"source"`
	// Product must equal the audited dialect's product ("mysql").
	Product string `json:"product"`
	// Version must equal the resolved target version exactly — the
	// observed banner online or the pinned target version offline.
	Version string `json:"version"`
	// Target names the deployment instance the bundle describes (a
	// non-secret identity such as the server UUID). Required since R9:
	// the bundle applies only if the audited target's resolved identity
	// matches it exactly — the provider-observed @@server_uuid online
	// (a cleanly absent or empty observation refuses the bundle without
	// erroring; never the declared identity), else the request's
	// declared TargetIdentity when no provider is attached at all. A
	// missing, empty, mismatched, or unverifiable target ignores the
	// whole bundle, so one instance's evidence can never fill another
	// instance's gaps.
	Target string `json:"target,omitempty"`
	// HostVCPUs is the deployment's logical CPU count — a positive
	// value fills the host.vcpu_count premise.
	HostVCPUs *uint64 `json:"host_vcpus,omitempty"`
	// PriorityCapable answers whether the deployment platform schedules
	// thread priorities — fills the priority-capability premise.
	PriorityCapable *bool `json:"priority_capable,omitempty"`
	// FeatureSupported answers whether the deployment build carries the
	// resource-group feature — fills the feature premise.
	FeatureSupported *bool `json:"feature_supported,omitempty"`
}

// TTLConfig is the proven TTL configuration recorded by the catalog or a
// prospective ordered-state transition. Enabled uses a pointer so "member not
// observed" stays distinct from OFF; interval members likewise remain empty
// when the catalog did not prove them.
type TTLConfig struct {
	TimeColumn    string `json:"-"`
	IntervalValue string `json:"-"`
	IntervalUnit  string `json:"-"`
	Enabled       *bool  `json:"-"`
	JobInterval   string `json:"-"`
}

// PGRelationState is an exact schema-local pg_class observation, including
// relation kinds outside the table snapshot. A zero Name means unobserved.
// A nonempty Name with an empty Kind proves absence in TableSnapshot.Schema.
type PGRelationState struct {
	Name string
	Kind string
	// Heap is true only for an observed built-in heap access method.
	Heap bool
}

// TableSnapshot is the current metadata-backed shape of a target table.
// Member collections keep their source semantics: a nil Columns marks a
// withheld column set (unknown), while PrimaryKey/Indexes/Constraints treat
// nil as "loaded, absent/empty" — the *Unknown flags below override that
// convention when a projection cannot vouch for those collections.
type TableSnapshot struct {
	Schema       string            `json:"schema,omitempty"`
	Exists       bool              `json:"exists"`
	Table        *Table            `json:"table,omitempty"`
	RelationKind string            `json:"relation_kind,omitempty"`
	View         *ViewSnapshot     `json:"-"`
	Columns      []Column          `json:"columns,omitempty"`
	PrimaryKey   *Index            `json:"primary_key,omitempty"`
	Indexes      []Index           `json:"indexes,omitempty"`
	Constraints  []Constraint      `json:"constraints,omitempty"`
	Options      map[string]string `json:"options,omitempty"`

	// PGConstraints is the complete, target-bound PG constraint directory.
	// nil means unobserved/stale; non-nil empty means observed empty. It is
	// separate from legacy PK-only Constraints and never enters public JSON.
	PGConstraints []PGConstraintState `json:"-"`
	// PGColumns is a complete, exact-name generation-state directory; nil
	// means unobserved/stale, never absence. It carries no expressions.
	PGColumns  *PGColumnCatalog `json:"-"`
	PGRelation PGRelationState  `json:"-"`

	// TTL is the catalog/prospective TTL configuration when one is proven
	// present. nil means either proven absence (TTLKnown=true) or an
	// unobserved state (TTLKnown=false); never infer either from the other.
	TTL *TTLConfig `json:"-"`
	// TTLKnown reports whether the snapshot established the TTL existence
	// answer. TTLKnown=true + TTL=nil is the bounded "known no TTL" fact;
	// TTLKnown=false keeps the configuration unobserved, not absent.
	TTLKnown bool `json:"-"`
	// PlacementPolicy carries the proven bound placement-policy name;
	// PlacementPolicyKnown=true + "" is the bounded "known unbound" fact,
	// and Known=false keeps the binding unobserved rather than absent
	// (issue #99 T20).
	PlacementPolicy      string `json:"-"`
	PlacementPolicyKnown bool   `json:"-"`
	// PartitionPlacements carries the proven per-partition bound policy
	// names when the partition directory was loaded; nil means the member
	// bindings were never observed, never read as confirmed unbound
	// (issue #99 T20).
	// PartitionPlacements maps a member name to its EXPLICIT placement
	// override. nil = the whole member binding space is unobserved; an
	// absent key = that member's override is unknown; an empty string =
	// a known-empty override (omitted or DEFAULT — the member inherits
	// the table binding effectively). Never read a missing key as a
	// known empty override (T20-R4).
	PartitionPlacements map[string]string `json:"-"`

	// PrimaryKeyIsHandle marks a TiDB PKIsHandle table: the single-integer
	// clustered primary key is the row handle, so no operable PRIMARY index
	// object exists even though the catalog synthesizes a PRIMARY row for
	// display (ALTER INDEX `PRIMARY` answers 1176 there). A non-handle TiDB
	// primary key — NONCLUSTERED or clustered on a non-single-int shape —
	// is a real index object (VISIBLE succeeds, INVISIBLE reports 3522).
	// MySQL never sets it: PRIMARY is always an index object there.
	PrimaryKeyIsHandle bool `json:"-"`

	// PrimaryKeyHandleUnknown marks that the snapshot cannot prove the
	// TiDB handle answer either way — e.g. the TIDB_INDEXES clustered row
	// was absent or NULL on the provider path, or a batch-derived PK was
	// declared without CLUSTERED|NONCLUSTERED while the session's
	// tidb_enable_clustered_index mode is unobserved. Consumers must not
	// read PrimaryKeyIsHandle as a determined "real object" then — the
	// PRIMARY-object answer stays unresolved.
	PrimaryKeyHandleUnknown bool `json:"-"`

	// PrimaryKeyClustered marks a TiDB primary key stored clustered —
	// PKIsHandle single-int PKs and common-handle PKs alike. The fact
	// answers the DROP PRIMARY KEY premise (TiDB refuses the drop with
	// 8200 on every clustered primary, handle or common-handle); it is
	// deliberately separate from PrimaryKeyIsHandle, which answers the
	// PRIMARY-object addressability question instead. MySQL never sets
	// it: a MySQL primary key drops cleanly.
	PrimaryKeyClustered bool `json:"-"`

	// PrimaryKeyClusteredUnknown marks that the snapshot cannot prove the
	// clustered-storage answer either way — the same evidence gaps that
	// leave the handle answer unknown. Consumers must not guess drop
	// executability under it.
	PrimaryKeyClusteredUnknown bool `json:"-"`

	// PrimaryKeyUnknown/IndexesUnknown/ConstraintsUnknown mark member
	// collections the snapshot cannot vouch for (a derived-incomplete
	// projection). Rules must treat them as unknown — never as confirmed
	// absence. Internal audit markers; not part of serialized output.
	PrimaryKeyUnknown  bool `json:"-"`
	IndexesUnknown     bool `json:"-"`
	ConstraintsUnknown bool `json:"-"`

	// IndexOrderCatalog records that Indexes slice order is the catalog /
	// declaration order (a table wholly created in-batch). Provider-loaded
	// indexes arrive name-sorted, which is never the role-selection order,
	// so provider snapshots keep this false and multi-candidate primary-role
	// determinations degrade to a bounded gap instead of guessing Indexes[0].
	IndexOrderCatalog bool `json:"-"`

	// IndexNamesUnknown marks a vouched Indexes collection whose stored
	// index names cannot all be proven — a derived create-table shape
	// where StoredIndexNames could not resolve every member under the
	// product's auto-naming rule (expression first member, authored-name
	// collision), or the post-state of a name-keyed transition whose target
	// or outcome could not be proven under the prior name space. The flag
	// covers the member name space only: the PK object's stored name has
	// its own provability marker (PrimaryKey.NameUnknown), which the
	// transitions that may have moved it set separately. The members stay
	// known; name-based consumers must treat lookups as unresolved rather
	// than reading an unmatched name as confirmed absence. Internal audit
	// marker; not serialized.
	IndexNamesUnknown bool `json:"-"`

	// Partitioned reports whether the table is partitioned — the fact TiDB's
	// GLOBAL index option and the partition-action applicability checks are
	// gated on. nil means the fact was never loaded (offline statement or a
	// pre-expansion provider), not "not partitioned".
	Partitioned *bool `json:"-"`

	// PartitionType is the observed PARTITION_METHOD normalized lowercase
	// (`range`, `list`, `hash`, `key`, `linear hash`, `linear key`,
	// `range columns`, `list columns`). nil means the catalog probe never
	// answered — never read as "no type". The fact is vacuous on a
	// known-non-partitioned table.
	PartitionType *string `json:"-"`

	// PartitionNames is the observed partition-member name list in catalog
	// order. nil means unobserved; a non-nil empty slice is an
	// observed-empty set (a known-non-partitioned table).
	PartitionNames []string `json:"-"`

	// PartitionDescriptions carries the catalog PARTITION_DESCRIPTION
	// text per member, aligned 1:1 with PartitionNames (RANGE keeps the
	// authored bound text like `120`/`'abc'`/`MAXVALUE`, LIST the value
	// list `2,5`, HASH/KEY members carry empty text). nil means
	// unobserved — the fact lives only where names were observed.
	PartitionDescriptions []string `json:"-"`

	// PartitionDefs carries the member directory with boundary clauses in
	// comparable literal form — projected from declared definitions on
	// batch-derived shapes and from the catalog description text on
	// provider shapes, so bound comparisons read one field on either
	// side. nil when the member space was never observed; a member whose
	// bound could not project keeps a clause without rows (unproven —
	// never a claimed-empty boundary).
	PartitionDefs []PartitionDef `json:"-"`

	// Subpartitioned reports that the catalog showed leaf-level member
	// rows (SUBPARTITION_NAME non-NULL). On such tables the top-level
	// member list is not the member space, so PartitionNames stays
	// unobserved; the flag distinguishes that incomplete projection from
	// a simply-unanswered catalog probe.
	Subpartitioned bool `json:"-"`

	// Temporary marks a bound session-local temporary table — an object
	// in the per-session namespace that can shadow a same-named
	// persistent table. Provider snapshots never carry it; only
	// batch-derived bindings produce it.
	Temporary bool `json:"temporary,omitempty"`

	// GlobalTemporary marks a TiDB global-temporary catalog object — a
	// persistent-namespace definition whose rows are transaction-scoped.
	// It lives in the ordinary catalog namespace (plain DROP/ALTER/
	// TRUNCATE/RENAME apply), unlike Temporary's session object.
	GlobalTemporary bool `json:"global_temporary,omitempty"`

	// ColumnsTailUnknown marks that the Columns list is provable only up
	// to its declared prefix — a CTAS destination keeps the declared
	// column definitions but cannot vouch for select-appended members
	// beyond the tail. Consumers must not treat Columns as the complete
	// member set under this flag.
	ColumnsTailUnknown bool `json:"-"`

	// ReferencedBy carries inbound foreign-key edges the state can prove:
	// batch-derived children declaring REFERENCES <this table>, or a
	// provider's successful reverse-edge catalog read. A non-nil slice is
	// the proven answer (empty means none); nil means the set was never
	// loaded and says nothing about inbound references.
	ReferencedBy []Table `json:"-"`

	// ReferencedByUnknown marks that the snapshot cannot vouch for
	// inbound references at all — provider paths that do not read reverse
	// edges carry it, so "no recorded child" there is unknown, never
	// proven-empty. Consumers (TRUNCATE's 1701 premise and TiDB's TTL
	// prerequisite) must gap instead of reading an unvouched set as safe.
	ReferencedByUnknown bool `json:"-"`
}

// HasColumn reports whether the snapshot contains a column by name, case-insensitively.
func (s TableSnapshot) HasColumn(name string) bool {
	return s.FindColumn(name) != nil
}

// FindColumn returns the matching column by name, case-insensitively.
func (s TableSnapshot) FindColumn(name string) *Column {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range s.Columns {
		if strings.EqualFold(s.Columns[i].Name, name) {
			column := s.Columns[i]
			return &column
		}
	}
	return nil
}

// HasIndex reports whether the snapshot contains an index by name, case-insensitively.
func (s TableSnapshot) HasIndex(name string) bool {
	return s.FindIndex(name) != nil
}

// FindIndex returns the matching secondary/unique/fulltext index by name, case-insensitively.
func (s TableSnapshot) FindIndex(name string) *Index {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range s.Indexes {
		if strings.EqualFold(s.Indexes[i].Name, name) {
			index := s.Indexes[i]
			return &index
		}
	}
	return nil
}

// FindProvenIndex returns the member whose stored name is both provable
// (!NameUnknown) and equal-folds the authored name — the only member a
// name-keyed lookup may bind. Under IndexNamesUnknown a member kept as a
// fact may carry an unproven projected name; binding it would fabricate a
// hit the stored name space cannot vouch for. The result points into the
// receiver's index slice so identity comparisons (e.g. HolderIndex) work.
func (s TableSnapshot) FindProvenIndex(name string) *Index {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range s.Indexes {
		if s.Indexes[i].NameUnknown {
			continue
		}
		if strings.EqualFold(s.Indexes[i].Name, name) {
			return &s.Indexes[i]
		}
	}
	return nil
}

// HasUnprovenIndexName reports whether any member's stored name is
// unproven — the granular form of IndexNamesUnknown used by name-keyed
// transitions to tell a provable target from a name space that may hide
// the real stored name.
func (s TableSnapshot) HasUnprovenIndexName() bool {
	for i := range s.Indexes {
		if s.Indexes[i].NameUnknown {
			return true
		}
	}
	return false
}

// IndexNameSpaceUnvouched reports whether the member name space cannot vouch
// every stored name — the collection unread, the whole space unvouched, or
// one member's stored name unproven. Name-keyed consumers share this check
// so they cannot drift on what "the member space is vouched" means.
func (s TableSnapshot) IndexNameSpaceUnvouched() bool {
	return s.IndexesUnknown || s.IndexNamesUnknown || s.HasUnprovenIndexName()
}

// FindForeignKeyConstraint binds a name-keyed foreign-key lookup to a
// proven member of the loaded constraint set (case-insensitive like the
// catalog's foreign-key name space). It returns nil when no member carries
// the name; callers must treat nil under ConstraintsUnknown or an
// unproven member name as "unknown", not absence — a nil Constraints is
// itself a loaded empty set per the collection contract.
func (s TableSnapshot) FindForeignKeyConstraint(name string) *Constraint {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range s.Constraints {
		if s.Constraints[i].Type != "foreign_key" {
			continue
		}
		if s.Constraints[i].NameUnknown {
			continue
		}
		if strings.EqualFold(s.Constraints[i].Name, name) {
			return &s.Constraints[i]
		}
	}
	return nil
}

// FindCheckConstraint binds a name-keyed CHECK lookup to a proven member of
// the loaded constraint set — the check-namespace counterpart of
// FindForeignKeyConstraint. A nil result under ConstraintsUnknown or an
// unproven member name means "unknown", never confirmed absence.
func (s TableSnapshot) FindCheckConstraint(name string) *Constraint {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range s.Constraints {
		if s.Constraints[i].Type != "check" {
			continue
		}
		if s.Constraints[i].NameUnknown {
			continue
		}
		if strings.EqualFold(s.Constraints[i].Name, name) {
			return &s.Constraints[i]
		}
	}
	return nil
}

// HasUnprovenCheckName reports whether any CHECK member's stored name is
// unproven — the check arm of an unvouched constraint name space.
func (s TableSnapshot) HasUnprovenCheckName() bool {
	for i := range s.Constraints {
		if s.Constraints[i].Type == "check" && s.Constraints[i].NameUnknown {
			return true
		}
	}
	return false
}

// HasUnprovenForeignKeyName reports whether any foreign-key member's stored
// name is unproven — the granular form of an unvouched constraint name
// space used by name-keyed transitions to tell a provable target from a
// member that may hide the real stored name.
func (s TableSnapshot) HasUnprovenForeignKeyName() bool {
	for i := range s.Constraints {
		if s.Constraints[i].Type == "foreign_key" && s.Constraints[i].NameUnknown {
			return true
		}
	}
	return false
}

// CheckNameSpaceUnvouched reports whether the CHECK member name space
// cannot vouch every stored name — the collection flagged unvouched, or one
// check member's stored name unproven. The check namespace on MySQL/TiDB is
// separate from index names, so only check members participate.
func (s TableSnapshot) CheckNameSpaceUnvouched() bool {
	return s.ConstraintsUnknown || s.HasUnprovenCheckName()
}

// ForeignKeyNameSpaceUnvouched reports whether the foreign-key member name
// space cannot vouch every stored name — the collection flagged
// unvouched, or one member's stored name unproven. Per the collection
// contract at the top of this file a nil Constraints is a loaded empty
// set, never an unread one; providers and projections mark the unread
// case through ConstraintsUnknown. Name-keyed consumers share this check
// so "not found" can never read as confirmed absence.
func (s TableSnapshot) ForeignKeyNameSpaceUnvouched() bool {
	return s.ConstraintsUnknown || s.HasUnprovenForeignKeyName()
}

// HasPrimaryKey reports whether the snapshot currently has a primary key.
func (s TableSnapshot) HasPrimaryKey() bool {
	if s.PrimaryKey != nil {
		return true
	}
	for _, c := range s.Constraints {
		if c.Type == "primary_key" {
			return true
		}
	}
	return false
}

// MetadataStatus identifies the outcome of a metadata object lookup.
type MetadataStatus string

const (
	MetadataStatusConfirmed   MetadataStatus = "confirmed"
	MetadataStatusNotFound    MetadataStatus = "not_found"
	MetadataStatusUnavailable MetadataStatus = "unavailable"
	MetadataStatusAmbiguous   MetadataStatus = "ambiguous"
)

// MetadataUnread* are the stable read-not-issued reason tokens shared by
// provider fact projections: a fact whose read never fired names why,
// keeping "not loaded" distinct from "loaded, answer unknown" (T26/#105).
const (
	// MetadataUnreadIdentityUnproven marks a fact read skipped because
	// the identity it is keyed on is not provable — an undecoded
	// spelling is never a normalized catalog identity.
	MetadataUnreadIdentityUnproven = "identity_unproven"
	// MetadataUnreadVersionInapplicable marks a fact read skipped
	// because a proven server version makes the catalog inapplicable —
	// never issued on a surface that cannot hold the object.
	MetadataUnreadVersionInapplicable = "version_inapplicable"
	// MetadataUnreadVersionUnproven marks a fact read skipped because no
	// version observation ever produced a fact — the version premise is
	// missing, not contradicted (T26-SRS-VERSION-APPLICABILITY-01 V2).
	MetadataUnreadVersionUnproven = "version_unproven"
	// MetadataUnreadVersionOutOfSeries marks a fact read skipped because
	// a resolved version identity lies outside the milestone-validated
	// series — a known fact, not an unproven one (V2).
	MetadataUnreadVersionOutOfSeries = "version_out_of_series"
	// MetadataUnreadVersionObservationError marks a fact read skipped
	// because the observation channel answered but the banner carried no
	// usable version fact — distinct from both inapplicable and unproven
	// (V10).
	MetadataUnreadVersionObservationError = "version_observation_error"
	// MetadataUnreadBatchEffects marks a fact read suppressed because an
	// earlier batch statement left an undischarged unknown effect on the
	// identity — the ordered channel's unproven barrier — so the catalog
	// answer may be stale and is never promoted (T26-SERVER-DROP-01).
	MetadataUnreadBatchEffects = "batch_effects"
)

// ObjectSnapshot carries the metadata-validated state of one non-table database object.
type ObjectSnapshot struct {
	PGPartitionConstraint       *PGPartitionConstraintState       `json:"-"`
	PGDropViewSet               *PGDropViewSetState               `json:"-"`
	PGCreateViewTarget          *PGCreateViewTarget               `json:"-"`
	PGMaterializedViewExtension *PGMaterializedViewExtensionState `json:"-"`
	PGViewDefault               *PGViewDefaultState               `json:"-"`
	PGDropViewState             *PGDropViewState                  `json:"-"`
	PGForeignKeyEquality        *PGForeignKeyEquality             `json:"-"`
	Schema                      string                            `json:"schema,omitempty"`
	Type                        string                            `json:"type,omitempty"`
	Name                        string                            `json:"name,omitempty"`
	Status                      MetadataStatus                    `json:"status,omitempty"`
	Exists                      bool                              `json:"exists"`
	Attributes                  map[string]string                 `json:"attributes,omitempty"`
	AmbiguousCandidates         []string                          `json:"ambiguous_candidates,omitempty"`
	PGDestructiveReferences     *PGDestructiveReferences          `json:"-"`
	// Sequence carries the provider-proven sequence definition parameters
	// when the resolved object is a sequence. Internal evidence only —
	// definition parameters are never runtime issuance progress (T21/#100).
	Sequence                  *SequenceDefinition        `json:"-"`
	PGReplicaIdentity         *PGReplicaIdentityIndex    `json:"-"`
	PGConstraintUsingIndex    *PGConstraintUsingIndex    `json:"-"`
	PGTableTablespace         *PGTableTablespace         `json:"-"`
	PGTableAccessMethod       *PGTableAccessMethod       `json:"-"`
	PGTablePersistence        *PGTablePersistence        `json:"-"`
	PGTableNamespace          *PGTableNamespace          `json:"-"`
	PGTableOwner              *PGTableOwnerState         `json:"-"`
	PGColumnType              *PGColumnTypeState         `json:"-"`
	PGBulkTablespace          *PGBulkTablespaceState     `json:"-"`
	PGForeignKeyKeys          *PGForeignKeyKeys          `json:"-"`
	PGDropConstraintReference *PGDropConstraintReference `json:"-"`
	PGDropColumnDependency    *PGDropColumnDependency    `json:"-"`
	PGColumnTraversal         *PGColumnTraversal         `json:"-"`
	PGRefreshState            *PGRefreshState            `json:"-"`
	// SequenceReferences is the bounded inbound-reference read for a proven
	// sequence identity — the catalog's owner-slot/target edge list. A nil
	// slice means the read never happened — never a proven-empty set;
	// SequenceReferencesUnknown reports the same unvouched shape explicitly.
	SequenceReferences        []SequenceRefEdge `json:"-"`
	SequenceReferencesUnknown bool              `json:"-"`
	// ResourceGroup carries the provider-proven resource-group definition
	// when the resolved object is a TiDB resource group — the bounded
	// information_schema.resource_groups row shape (T22/#101).
	ResourceGroup *ResourceGroupDefinition `json:"-"`
	// MySQLResourceGroup carries the provider-proven MySQL resource-group
	// definition — the information_schema.RESOURCE_GROUPS row shape with
	// MySQL columns (T24/#103).
	MySQLResourceGroup *MySQLResourceGroupDefinition `json:"-"`
	// Tablespace carries the provider-proven MySQL tablespace state —
	// the InnoDB catalog row's kind plus the bounded bound-table
	// occupancy answer (T25/#104). A nil Occupied member means the
	// association read never happened — never a proven-empty set.
	Tablespace *TablespaceState `json:"-"`
	// Server carries the provider-read MySQL SERVER existence fact —
	// the bounded mysql.servers identity read (T26/#105 provider
	// round). Nil means the family was never bound (dialect-inapplicable
	// or never requested); Read=false means the read stayed unissued
	// (UnreadReason names why).
	Server *ServerState `json:"-"`
	// SpatialReference carries the provider-read MySQL SRS facts —
	// the ST_SPATIAL_REFERENCE_SYSTEMS identity read, the optional
	// NAME/ORGANIZATION conflict witness reads, and the
	// ST_GEOMETRY_COLUMNS dependency witnesses (T26/#105 provider
	// round). Nil sub-members mean their reads never happened — never
	// proven-empty sets.
	SpatialReference *SpatialReferenceSystemState `json:"-"`
}

// ObjectLookupRequest describes one object to resolve from live metadata.
type ObjectLookupRequest struct {
	PGForeignKeyEqualityTarget *PGForeignKeyEqualityTarget `json:"-"`
	Schema                     string
	Type                       string
	Name                       string
	Qualifiers                 map[string]string
	PGBulkTablespace           *PGBulkTablespaceTarget    `json:"-"`
	PGDestructiveTargets       []PGDestructiveTableTarget `json:"-"`
	PGDropIfExists             bool                       `json:"-"`
	// RequestVersion carries the request-level OBSERVED version identity
	// the caller already adjudicated for this object's family — the SRS
	// family binds it so the provider channel's own version query can
	// only act as a consistency witness, never a second adjudicator
	// (T26-SRS-VERSION-APPLICABILITY-01 frozen V1/R3 additive binding).
	// Other families leave it nil and keep their existing checks.
	RequestVersion *VersionIdentity
}

// Object-lookup qualifier keys for the T26/#105 SERVER/SRS fact reads.
// Every qualifier carries a provable identity only — the provider never
// interprets an undecoded authored spelling as one of these values.
const (
	// ObjectLookupQualifierNameForm marks how a mysql_server request
	// Name was produced; only the ObjectLookupNameFormIdentifier value
	// proves a complete decoded identifier — anything else (including
	// an absent qualifier) is an unprovable spelling.
	ObjectLookupQualifierNameForm = "name_form"
	// ObjectLookupNameFormIdentifier is the qualifier value proving
	// the request Name is a decoded identifier — the complete catalog
	// identity, not a string-literal spelling.
	ObjectLookupNameFormIdentifier = "identifier"
	// ObjectLookupQualifierSRSName carries a provable SRS NAME catalog
	// identity; when present the provider issues the NAME-conflict
	// witness read. Authored NAME spellings never populate it.
	ObjectLookupQualifierSRSName = "srs_name"
	// ObjectLookupQualifierSRSOrg carries the provable ORGANIZATION
	// member of the combined conflict identity; it fires only together
	// with ObjectLookupQualifierSRSOrgID — a partially-known pair never
	// issues a query.
	ObjectLookupQualifierSRSOrg = "srs_org"
	// ObjectLookupQualifierSRSOrgID carries the provable
	// ORGANIZATION_COORDSYS_ID member of the combined conflict
	// identity — decimal text the provider parses strictly.
	ObjectLookupQualifierSRSOrgID = "srs_org_id"
)

// IsConfirmed reports whether the object was found in live metadata.
func (o ObjectSnapshot) IsConfirmed() bool {
	return o.Status == MetadataStatusConfirmed
}

// IsNotFound reports whether the object was confirmed absent from live metadata.
func (o ObjectSnapshot) IsNotFound() bool {
	return o.Status == MetadataStatusNotFound
}

// IsUnavailable reports whether metadata lookup was not performed or not possible.
func (o ObjectSnapshot) IsUnavailable() bool {
	return o.Status == MetadataStatusUnavailable || o.Status == ""
}

// IsAmbiguous reports whether the object identity could not be uniquely resolved.
func (o ObjectSnapshot) IsAmbiguous() bool {
	return o.Status == MetadataStatusAmbiguous
}

// SafeAttributes returns only non-sensitive attributes from the snapshot.
// Keys matching known sensitive patterns are excluded.
func (o ObjectSnapshot) SafeAttributes() map[string]string {
	if o.Attributes == nil {
		return nil
	}
	safe := make(map[string]string, len(o.Attributes))
	for k, v := range o.Attributes {
		if isSensitiveAttributeKey(k) {
			continue
		}
		safe[k] = v
	}
	return safe
}

// FindObject returns the first ObjectSnapshot matching the given type and name,
// case-insensitively. Returns nil if no match is found.
func (m *Metadata) FindObject(objectType, name string) *ObjectSnapshot {
	if m == nil || len(m.Objects) == 0 {
		return nil
	}
	ot := strings.TrimSpace(strings.ToLower(objectType))
	n := strings.TrimSpace(strings.ToLower(name))
	if ot == "" || n == "" {
		return nil
	}
	for i := range m.Objects {
		if strings.EqualFold(m.Objects[i].Type, objectType) && strings.EqualFold(m.Objects[i].Name, name) {
			snapshot := m.Objects[i]
			return &snapshot
		}
	}
	return nil
}

// FindObjectsByType returns all ObjectSnapshots matching the given type, case-insensitively.
func (m *Metadata) FindObjectsByType(objectType string) []ObjectSnapshot {
	if m == nil || len(m.Objects) == 0 {
		return nil
	}
	var result []ObjectSnapshot
	for i := range m.Objects {
		if strings.EqualFold(m.Objects[i].Type, objectType) {
			result = append(result, m.Objects[i])
		}
	}
	return result
}

// sensitiveAttributeKeys lists attribute keys that must never be projected publicly.
var sensitiveAttributeKeys = map[string]bool{
	"password": true, "secret": true, "token": true, "api_key": true,
	"connection": true, "dsn": true, "connstr": true,
	"body": true, "definition": true, "comment": true, "label": true,
	"query": true, "action_sql": true, "options": true,
}

func isSensitiveAttributeKey(key string) bool {
	return sensitiveAttributeKeys[strings.ToLower(strings.TrimSpace(key))]
}
