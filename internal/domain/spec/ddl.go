// Package spec defines normalized statement specifications for rule evaluation.
// input: DDL facts extracted from parser-specific AST adapters
// output: parser-neutral DDL specification components for rules
// pos: domain DDL specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DDL contains the structural metadata extracted from a DDL statement.
type DDL struct {
	Operation   DDLOperation `json:"operation,omitempty"`
	Table       *Table       `json:"table,omitempty"`
	Columns     []Column     `json:"columns,omitempty"`
	PrimaryKey  *Index       `json:"primary_key,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	// Alter also carries standalone DDL action payloads when no table object exists.
	Alter         []Alter           `json:"alter,omitempty"`
	Options       map[string]string `json:"options,omitempty"`
	HasReferTable bool              `json:"has_refer_table,omitempty"`
	HasSelect     bool              `json:"has_select,omitempty"`
	HasPartition  bool              `json:"has_partition,omitempty"`
	ObjectName    string            `json:"object_name,omitempty"`
	ObjectType    string            `json:"object_type,omitempty"`
}

// DDLOperation identifies the normalized DDL operation represented by a statement.
type DDLOperation string

// Supported DDL operations.
const (
	DDLOperationUnknown       DDLOperation = "unknown"
	DDLOperationCreateTable   DDLOperation = "create_table"
	DDLOperationCreateView    DDLOperation = "create_view"
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
	DefaultIsNull             bool           `json:"default_is_null,omitempty"`
	DefaultIsCurrentTimestamp bool           `json:"default_is_current_timestamp,omitempty"`
	OnUpdateCurrentTimestamp  bool           `json:"on_update_current_timestamp,omitempty"`
	GeneratedWhen             string         `json:"generated_when,omitempty"`
	IsIdentity                bool           `json:"is_identity,omitempty"`
	IdentityOptions           map[string]any `json:"identity_options,omitempty"`
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
	Action  string            `json:"action"`
	Name    string            `json:"name,omitempty"`
	Column  *AlterColumn      `json:"column,omitempty"`
	Index   *AlterIndex       `json:"index,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}
