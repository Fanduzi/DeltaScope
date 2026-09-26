// Package spec defines normalized statement specifications for rule evaluation.
// input: DDL facts extracted from parser-specific AST adapters
// output: parser-neutral DDL specification components for rules
// pos: domain DDL specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DDL contains the structural metadata extracted from a DDL statement.
type DDL struct {
	Operation DDLOperation `json:"operation,omitempty"`
	Table     *Table       `json:"table,omitempty"`
	// Targets lists every table-level object identity the statement names, in
	// source order: each DROP TABLE/VIEW name, each TRUNCATE relation, each
	// RENAME TABLE source and destination pair-wise, and ALTER TABLE
	// RENAME TO destinations after the altered subject. Entries keep their
	// as-written qualifiers: an unqualified rename destination stays
	// unqualified here because it resolves to the current schema, not the
	// source table's schema, under MySQL/TiDB semantics. Table stays the
	// primary (first) target for single-target consumers; use TableTargets
	// to read the complete list.
	Targets     []Table      `json:"targets,omitempty"`
	Columns     []Column     `json:"columns,omitempty"`
	PrimaryKey  *Index       `json:"primary_key,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	// Alter also carries standalone DDL action payloads when no table object exists.
	Alter   []Alter           `json:"alter,omitempty"`
	Options map[string]string `json:"options,omitempty"`
	// UnextractedOptions names parsed table/statement options the extractor
	// recognized but did not model into Options. Presence marks an evidence
	// gap: the fact was parsed but is not auditable.
	UnextractedOptions []string `json:"unextracted_options,omitempty"`
	// OmittedTargets counts parsed statement targets the normalized model
	// collapsed or ignored entirely: extra names in DROP USER/ROLE/SEQUENCE
	// lists, extra CREATE/ALTER USER specs, and the whole user list on
	// GRANT/REVOKE. A nonzero value is an evidence gap, not a wildcard.
	OmittedTargets int  `json:"omitted_targets,omitempty"`
	HasReferTable  bool `json:"has_refer_table,omitempty"`
	HasSelect      bool `json:"has_select,omitempty"`
	HasPartition   bool `json:"has_partition,omitempty"`
	// TemporaryScope records a parsed temporary-table keyword on
	// CREATE/DROP TABLE. The scope changes object identity and lifetime, so a
	// non-empty value is a recognized-but-unaudited fact until temporary-table
	// semantics land (inventory owner T16).
	TemporaryScope TemporaryScope `json:"temporary_scope,omitempty"`
	// OnCommitDelete marks the transaction-scoped data attribute parsed on
	// GLOBAL TEMPORARY TABLE declarations.
	OnCommitDelete bool   `json:"on_commit_delete,omitempty"`
	ObjectName     string `json:"object_name,omitempty"`
	ObjectType     string `json:"object_type,omitempty"`
}

// TemporaryScope identifies the parsed temporary-table scope on CREATE and
// DROP TABLE statements.
type TemporaryScope string

const (
	// TemporaryScopeLocal marks the session-scoped local temporary form
	// (CREATE TEMPORARY TABLE / DROP TEMPORARY TABLE), supported by both
	// MySQL and TiDB.
	TemporaryScopeLocal TemporaryScope = "local"
	// TemporaryScopeGlobal marks the TiDB-only global temporary form whose
	// definition persists while data is transaction-scoped.
	TemporaryScopeGlobal TemporaryScope = "global"
)

// TableTargets returns every table-level object identity the statement names,
// in source order. Extractors that populate Targets report them all;
// otherwise the primary Table is the single target so consumers do not
// rediscover targets dropped during extraction.
func (d *DDL) TableTargets() []Table {
	if d == nil {
		return nil
	}
	if len(d.Targets) > 0 {
		return d.Targets
	}
	if d.Table != nil {
		return []Table{*d.Table}
	}
	return nil
}

// DDLOperation identifies the normalized DDL operation represented by a statement.
type DDLOperation string

// Supported DDL operations.
const (
	DDLOperationUnknown       DDLOperation = "unknown"
	DDLOperationCreateTable   DDLOperation = "create_table"
	DDLOperationCreateView    DDLOperation = "create_view"
	DDLOperationAlterView     DDLOperation = "alter_view"
	DDLOperationAlterTable    DDLOperation = "alter_table"
	DDLOperationDropTable     DDLOperation = "drop_table"
	DDLOperationDropIndex     DDLOperation = "drop_index"
	DDLOperationCreateIndex   DDLOperation = "create_index"
	DDLOperationDropView      DDLOperation = "drop_view"
	DDLOperationTruncateTable DDLOperation = "truncate_table"

	DDLOperationCreateSchema            DDLOperation = "create_schema"
	DDLOperationDropSchema              DDLOperation = "drop_schema"
	DDLOperationCreateSequence          DDLOperation = "create_sequence"
	DDLOperationAlterSequence           DDLOperation = "alter_sequence"
	DDLOperationDropSequence            DDLOperation = "drop_sequence"
	DDLOperationCreateMaterializedView  DDLOperation = "create_materialized_view"
	DDLOperationDropMaterializedView    DDLOperation = "drop_materialized_view"
	DDLOperationRefreshMaterializedView DDLOperation = "refresh_materialized_view"

	DDLOperationCreateType DDLOperation = "create_type"
	DDLOperationAlterType  DDLOperation = "alter_type"
	DDLOperationDropType   DDLOperation = "drop_type"

	DDLOperationCreateDomain DDLOperation = "create_domain"
	DDLOperationAlterDomain  DDLOperation = "alter_domain"
	DDLOperationDropDomain   DDLOperation = "drop_domain"

	DDLOperationCreateExtension DDLOperation = "create_extension"
	DDLOperationAlterExtension  DDLOperation = "alter_extension"
	DDLOperationDropExtension   DDLOperation = "drop_extension"

	DDLOperationGrantTable  DDLOperation = "grant_table"
	DDLOperationRevokeTable DDLOperation = "revoke_table"

	DDLOperationCreatePolicy DDLOperation = "create_policy"
	DDLOperationAlterPolicy  DDLOperation = "alter_policy"
	DDLOperationDropPolicy   DDLOperation = "drop_policy"

	DDLOperationCreateTrigger DDLOperation = "create_trigger"
	DDLOperationDropTrigger   DDLOperation = "drop_trigger"

	DDLOperationCreateFunction  DDLOperation = "create_function"
	DDLOperationDropFunction    DDLOperation = "drop_function"
	DDLOperationCreateProcedure DDLOperation = "create_procedure"
	DDLOperationDropProcedure   DDLOperation = "drop_procedure"

	DDLOperationAlterSchema DDLOperation = "alter_schema"

	// MySQL/TiDB extended DDL operations.
	DDLOperationRenameTable           DDLOperation = "rename_table"
	DDLOperationCreateUser            DDLOperation = "create_user"
	DDLOperationAlterUser             DDLOperation = "alter_user"
	DDLOperationDropUser              DDLOperation = "drop_user"
	DDLOperationCreateRole            DDLOperation = "create_role"
	DDLOperationDropRole              DDLOperation = "drop_role"
	DDLOperationGrant                 DDLOperation = "grant"
	DDLOperationRevoke                DDLOperation = "revoke"
	DDLOperationCreatePlacementPolicy DDLOperation = "create_placement_policy"
	DDLOperationAlterPlacementPolicy  DDLOperation = "alter_placement_policy"
	DDLOperationDropPlacementPolicy   DDLOperation = "drop_placement_policy"
	DDLOperationDropResourceGroup     DDLOperation = "drop_resource_group"

	DDLOperationAlterIndex            DDLOperation = "alter_index"
	DDLOperationAlterMaterializedView DDLOperation = "alter_materialized_view"

	DDLOperationCreatePublication DDLOperation = "create_publication"
	DDLOperationAlterPublication  DDLOperation = "alter_publication"
	DDLOperationDropPublication   DDLOperation = "drop_publication"

	DDLOperationCreateSubscription DDLOperation = "create_subscription"
	DDLOperationAlterSubscription  DDLOperation = "alter_subscription"
	DDLOperationDropSubscription   DDLOperation = "drop_subscription"

	DDLOperationCreateForeignTable DDLOperation = "create_foreign_table"
	DDLOperationAlterForeignTable  DDLOperation = "alter_foreign_table"
	DDLOperationDropForeignTable   DDLOperation = "drop_foreign_table"

	DDLOperationCreateForeignServer DDLOperation = "create_foreign_server"
	DDLOperationAlterForeignServer  DDLOperation = "alter_foreign_server"
	DDLOperationDropForeignServer   DDLOperation = "drop_foreign_server"

	DDLOperationCreateUserMapping DDLOperation = "create_user_mapping"
	DDLOperationAlterUserMapping  DDLOperation = "alter_user_mapping"
	DDLOperationDropUserMapping   DDLOperation = "drop_user_mapping"

	DDLOperationCreateForeignDataWrapper DDLOperation = "create_foreign_data_wrapper"
	DDLOperationAlterForeignDataWrapper  DDLOperation = "alter_foreign_data_wrapper"
	DDLOperationDropForeignDataWrapper   DDLOperation = "drop_foreign_data_wrapper"

	DDLOperationCommentOn     DDLOperation = "comment_on"
	DDLOperationSecurityLabel DDLOperation = "security_label"

	DDLOperationCreateEventTrigger DDLOperation = "create_event_trigger"
	DDLOperationAlterEventTrigger  DDLOperation = "alter_event_trigger"
	DDLOperationDropEventTrigger   DDLOperation = "drop_event_trigger"

	DDLOperationCreateRule DDLOperation = "create_rule"
	DDLOperationAlterRule  DDLOperation = "alter_rule"
	DDLOperationDropRule   DDLOperation = "drop_rule"

	DDLOperationCreateCollation DDLOperation = "create_collation"
	DDLOperationAlterCollation  DDLOperation = "alter_collation"
	DDLOperationDropCollation   DDLOperation = "drop_collation"

	DDLOperationCreateStatistics DDLOperation = "create_statistics"
	DDLOperationAlterStatistics  DDLOperation = "alter_statistics"
	DDLOperationDropStatistics   DDLOperation = "drop_statistics"

	DDLOperationCreateAggregate  DDLOperation = "create_aggregate"
	DDLOperationAlterAggregate   DDLOperation = "alter_aggregate"
	DDLOperationDropAggregate    DDLOperation = "drop_aggregate"
	DDLOperationCreateOperator   DDLOperation = "create_operator"
	DDLOperationAlterOperator    DDLOperation = "alter_operator"
	DDLOperationDropOperator     DDLOperation = "drop_operator"
	DDLOperationCreateConversion DDLOperation = "create_conversion"
	DDLOperationAlterConversion  DDLOperation = "alter_conversion"
	DDLOperationDropConversion   DDLOperation = "drop_conversion"

	DDLOperationCreateOperatorFamily DDLOperation = "create_operator_family"
	DDLOperationAlterOperatorFamily  DDLOperation = "alter_operator_family"
	DDLOperationDropOperatorFamily   DDLOperation = "drop_operator_family"
	DDLOperationCreateOperatorClass  DDLOperation = "create_operator_class"
	DDLOperationAlterOperatorClass   DDLOperation = "alter_operator_class"
	DDLOperationDropOperatorClass    DDLOperation = "drop_operator_class"

	DDLOperationCreateTextSearchConfiguration DDLOperation = "create_text_search_configuration"
	DDLOperationAlterTextSearchConfiguration  DDLOperation = "alter_text_search_configuration"
	DDLOperationDropTextSearchConfiguration   DDLOperation = "drop_text_search_configuration"
	DDLOperationCreateTextSearchDictionary    DDLOperation = "create_text_search_dictionary"
	DDLOperationAlterTextSearchDictionary     DDLOperation = "alter_text_search_dictionary"
	DDLOperationDropTextSearchDictionary      DDLOperation = "drop_text_search_dictionary"
	DDLOperationCreateTextSearchParser        DDLOperation = "create_text_search_parser"
	DDLOperationAlterTextSearchParser         DDLOperation = "alter_text_search_parser"
	DDLOperationDropTextSearchParser          DDLOperation = "drop_text_search_parser"
	DDLOperationCreateTextSearchTemplate      DDLOperation = "create_text_search_template"
	DDLOperationAlterTextSearchTemplate       DDLOperation = "alter_text_search_template"
	DDLOperationDropTextSearchTemplate        DDLOperation = "drop_text_search_template"

	DDLOperationCreateTransform    DDLOperation = "create_transform"
	DDLOperationCreateAccessMethod DDLOperation = "create_access_method"
	DDLOperationDropTransform      DDLOperation = "drop_transform"
	DDLOperationDropAccessMethod   DDLOperation = "drop_access_method"
	DDLOperationAlterLargeObject   DDLOperation = "alter_large_object"
)

// Table describes a table-level object.
type Table struct {
	Schema  string `json:"schema,omitempty"`
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// Column describes a table column.
type Column struct {
	Name                      string         `json:"name"`
	Type                      string         `json:"type,omitempty"`
	Length                    int            `json:"length,omitempty"`
	Charset                   string         `json:"charset,omitempty"`
	Collation                 string         `json:"collation,omitempty"`
	Comment                   string         `json:"comment,omitempty"`
	Unsigned                  bool           `json:"unsigned,omitempty"`
	NotNull                   bool           `json:"not_null,omitempty"`
	AutoIncrement             bool           `json:"auto_increment,omitempty"`
	HasDefault                bool           `json:"has_default,omitempty"`
	DefaultValue              string         `json:"default_value,omitempty"`
	DefaultKind               string         `json:"default_kind,omitempty"`
	DefaultIsNull             bool           `json:"default_is_null,omitempty"`
	DefaultIsCurrentTimestamp bool           `json:"default_is_current_timestamp,omitempty"`
	OnUpdateCurrentTimestamp  bool           `json:"on_update_current_timestamp,omitempty"`
	GeneratedWhen             string         `json:"generated_when,omitempty"`
	IsIdentity                bool           `json:"is_identity,omitempty"`
	IdentityOptions           map[string]any `json:"identity_options,omitempty"`
	// AutoRandom marks a parsed TiDB AUTO_RANDOM column attribute. MySQL has
	// no such feature, so it is a vendor boundary there (owner T23).
	AutoRandom bool `json:"auto_random,omitempty"`
	// UnextractedOptions names parsed column options the extractor recognized
	// but did not model; see DDL.UnextractedOptions.
	UnextractedOptions []string `json:"unextracted_options,omitempty"`
}

// IndexKind identifies the semantic class of an index declaration.
type IndexKind string

// Supported index kinds.
const (
	IndexKindUnknown   IndexKind = "unknown"
	IndexKindPrimary   IndexKind = "primary"
	IndexKindSecondary IndexKind = "secondary"
	IndexKindUnique    IndexKind = "unique"
	IndexKindFulltext  IndexKind = "fulltext"
	IndexKindSpatial   IndexKind = "spatial"
	IndexKindVector    IndexKind = "vector"
	IndexKindColumnar  IndexKind = "columnar"
)

// Index describes an index declaration.
type Index struct {
	Name              string    `json:"name"`
	Kind              IndexKind `json:"kind,omitempty"`
	Columns           []string  `json:"columns,omitempty"`
	Cardinality       *int64    `json:"cardinality,omitempty"`
	AccessMethod      string    `json:"access_method,omitempty"`
	IncludedColumns   []string  `json:"included_columns,omitempty"`
	HasPredicate      bool      `json:"has_predicate,omitempty"`
	HasExpressionKeys bool      `json:"has_expression_keys,omitempty"`
	ExpressionCount   int       `json:"expression_count,omitempty"`
	// PrefixParts/DescParts count column-prefix lengths and descending key
	// parts parsed on the index but not yet modeled for audit semantics.
	PrefixParts int `json:"prefix_parts,omitempty"`
	DescParts   int `json:"desc_parts,omitempty"`
	// Global marks the TiDB-only GLOBAL index modifier parsed on table-level
	// UNIQUE/PRIMARY KEY constraints and standalone CREATE INDEX statements.
	// MySQL has no such modifier, so it is a vendor boundary there.
	Global bool `json:"global,omitempty"`
}

// Constraint describes a non-index table constraint worth preserving for later rules.
type Constraint struct {
	Type              string   `json:"type"`
	Name              string   `json:"name,omitempty"`
	Columns           []string `json:"columns,omitempty"`
	ReferencedSchema  string   `json:"referenced_schema,omitempty"`
	ReferencedTable   string   `json:"referenced_table,omitempty"`
	ReferencedColumns []string `json:"referenced_columns,omitempty"`
}

// AlterColumnChange describes statement-local column-change intent.
// These flags are parser-neutral hints about what the ALTER statement touches;
// they do not claim live-schema source truth on their own.
type AlterColumnChange struct {
	TouchesNullability   bool `json:"touches_nullability,omitempty"`
	TouchesDefault       bool `json:"touches_default,omitempty"`
	TouchesAutoIncrement bool `json:"touches_auto_increment,omitempty"`
}

// AlterColumn describes a column-focused alter payload.
// OldName is only populated when the action targets an existing column name.
// Definition carries the target column shape after the action when available.
// Change carries parser-neutral statement-local relation facts for upcoming
// source-aware alter rules.
type AlterColumn struct {
	OldName    string             `json:"old_name,omitempty"`
	Definition *Column            `json:"definition,omitempty"`
	Change     *AlterColumnChange `json:"change,omitempty"`
}

// AlterIndex describes an index-focused alter payload.
// OldName is only populated when the action targets an existing index name.
// Definition carries the target index shape after the action when available.
type AlterIndex struct {
	OldName    string `json:"old_name,omitempty"`
	Definition *Index `json:"definition,omitempty"`
}

// Alter describes a normalized alter action.
// Name is the canonical subject identifier for downstream matching:
// existing-object actions use the pre-change name, pure additions use the
// created object's name, and table-option actions leave it empty.
type Alter struct {
	Action string       `json:"action"`
	Name   string       `json:"name,omitempty"`
	Column *AlterColumn `json:"column,omitempty"`
	Index  *AlterIndex  `json:"index,omitempty"`
	// Constraint carries the declared constraint payload for constraint-bearing
	// alter specs (ADD CONSTRAINT ...) so coverage and rules can see the
	// constraint type even when no index definition is produced.
	Constraint *Constraint       `json:"constraint,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	// HasColumnPosition records a parsed FIRST|AFTER column-position clause.
	// Positional column ordering is parsed but not audited, so the fact exists
	// to drive incomplete-coverage evidence.
	HasColumnPosition bool `json:"has_column_position,omitempty"`
	// UnextractedOptions names parsed table-option clauses that were dropped
	// during extraction; see DDL.UnextractedOptions.
	UnextractedOptions []string `json:"unextracted_options,omitempty"`
}
