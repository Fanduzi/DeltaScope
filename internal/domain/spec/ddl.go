// Package spec defines normalized statement specifications for rule evaluation.
// input: DDL facts extracted from parser-specific AST adapters
// output: parser-neutral DDL specification components for rules, including
// the comparable literal-default identity (LiteralDefaultFacts) sources agree
// on, the per-member index name provability marker (Index.NameUnknown), and
// the foreign-key constraint refer-action facts (OnDelete/OnUpdate) plus the
// member name provability marker (Constraint.NameUnknown)
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
	Name          string `json:"name"`
	Type          string `json:"type,omitempty"`
	Length        int    `json:"length,omitempty"`
	Charset       string `json:"charset,omitempty"`
	Collation     string `json:"collation,omitempty"`
	Comment       string `json:"comment,omitempty"`
	Unsigned      bool   `json:"unsigned,omitempty"`
	NotNull       bool   `json:"not_null,omitempty"`
	AutoIncrement bool   `json:"auto_increment,omitempty"`
	// HasDefault and DefaultValue mean different facts by source: on the
	// input-AST path HasDefault marks an explicit DEFAULT clause and
	// literals keep their quoting; on the MySQL/TiDB metadata path
	// HasDefault marks a non-NULL COLUMN_DEFAULT and DefaultValue is the
	// stored representation verbatim. Neither may be read as recovering
	// the original declaration, and DefaultIsNull is only asserted by the
	// AST path — a NULL COLUMN_DEFAULT cannot distinguish an omitted
	// clause from an explicit DEFAULT NULL, so the metadata path never
	// sets it.
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
	// Generated is the internal generated-column fact: nil means no generated
	// attribute; non-nil carries the effective storage (omitted storage is
	// VIRTUAL) and structured expression facts. GeneratedWhen stays the
	// PostgreSQL-side fact and is unrelated.
	Generated *GeneratedColumn `json:"-"`
	// DefaultExpression is the internal expression-default fact: nil means no
	// expression default; a non-nil fact with Known=false is unknown, not an
	// empty reference list.
	DefaultExpression *ExpressionFacts `json:"-"`
	// Visibility is the effective column visibility fact: "visible",
	// "invisible", or empty when the fact is withheld/unresolved. CREATE
	// omission resolves to visible; it is never catalog-declaration presence.
	Visibility string `json:"-"`
	// VisibilityExplicit records that a VISIBLE|INVISIBLE attribute was
	// written. It is declaration presence only, never inferred from catalog
	// normalization.
	VisibilityExplicit bool `json:"-"`
	// DefaultParenthesized records that the default clause carried an
	// authored `DEFAULT (<expr>)` paren wrapper. Internal fact only: TiDB's
	// create-side parens grammar accepts only a bare call (a parenthesized
	// literal is a determined negative there while legal MySQL), and both
	// products require parens for non-literal ALTER SET DEFAULT — the flag
	// is never evidence about the stored value itself.
	DefaultParenthesized bool `json:"-"`
	// DefaultLiteral is the comparable literal identity for a
	// literal-class default: the decoded value bytes plus a literal kind.
	// nil means the identity is unproven or the default is not literal —
	// an empty declared string is a proven empty value, never the unknown
	// sentinel. Source-shaped DefaultValue stays the display field; it is
	// never read as identity (T08-R1). Every source writes this fact with
	// the same decode contract — declaration AST yields decoded values,
	// catalog COLUMN_DEFAULT bytes are already decoded value bytes.
	DefaultLiteral *LiteralDefaultFacts `json:"-"`
}

// LiteralDefaultFacts is the recorded literal identity of a literal-class
// column default. It carries no SQL text, quoting, or expression content —
// only the decoded value bytes and the literal kind. A recorded fact is not
// automatically a *comparable* pair: kind marks the value domain, and the
// comparison additionally requires a proven shared encoding (T08-R2).
type LiteralDefaultFacts struct {
	// Kind names the literal's value domain: "string" or "numeric".
	// Kinds never coerce — a string literal and a numeric literal never
	// prove equality, they prove unproven.
	Kind string
	// Value is the decoded literal value bytes exactly as the source
	// stores them: 'x' → x, '''x''' → 'x', 'O''Reilly' → O'Reilly on both
	// sides; a DECIMAL catalog may carry scale-normalized text (1.00)
	// while the AST integer carries canonical digits — same kind does not
	// prove a shared encoding.
	Value string
}

// Literal kinds. Only kinds whose cross-source value domain is provable
// without type coercion are named; encoding comparability is the
// comparison's domain check, not implied here (T08-R2).
const (
	LiteralDefaultString  = "string"
	LiteralDefaultNumeric = "numeric"
)

// CloneLiteralDefaultFacts deep-copies a literal-identity fact. Both fields
// are value types, so a nil-safe struct copy is the whole clone.
func CloneLiteralDefaultFacts(in *LiteralDefaultFacts) *LiteralDefaultFacts {
	if in == nil {
		return nil
	}
	out := *in
	return &out
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

// IndexKeyPartKind classifies one index member row.
type IndexKeyPartKind string

// Supported member kinds. "column" binds a named table column; "expression"
// is a functional key part carrying structured expression identity facts;
// "unmodeled" is a member row the source could not classify (e.g. a named
// member with a NULL catalog collation).
const (
	IndexKeyPartColumn     IndexKeyPartKind = "column"
	IndexKeyPartExpression IndexKeyPartKind = "expression"
	IndexKeyPartUnmodeled  IndexKeyPartKind = "unmodeled"
)

// IndexKeyPart is one index member with its source semantics preserved:
// ordinal position inside the index, which column it binds or which
// expression identity it carries, the explicit prefix length when authored/
// cataloged, and the declared or cataloged sort direction. The aggregate
// marks (PrefixParts/DescParts/ExpressionCount/MultiValuedParts/
// UnmodeledKeyParts) are derived from this list — SetKeyParts keeps both in
// sync so a member position can never disagree with its count.
type IndexKeyPart struct {
	Ordinal      int              `json:"ordinal"`
	Kind         IndexKeyPartKind `json:"kind"`
	Column       string           `json:"column,omitempty"`
	PrefixLength *int             `json:"prefix_length,omitempty"`
	Descending   bool             `json:"descending,omitempty"`
	// DirectionUnknown marks a classified member (column or expression)
	// whose sort direction the source could not prove — e.g. a provider
	// catalog row with a NULL or unrecognized COLLATION. The member keeps
	// its kind and identity; only the direction answer is withheld, so the
	// index's key ordering stays unproven for comparisons.
	DirectionUnknown bool `json:"direction_unknown,omitempty"`
	// Expression carries the member's structured identity on expression
	// parts: fingerprint for sameness, References for the column
	// dependencies, Functions/TopForm for the governed shape. nil on
	// column/unmodeled parts.
	Expression *ExpressionFacts `json:"expression,omitempty"`
	// MultiValued marks the MySQL multi-valued member form
	// CAST(… AS … ARRAY); the member also has Kind=expression.
	MultiValued bool `json:"multi_valued,omitempty"`
	// MultiValuedCastType records the normalized cast-target family a
	// multi-valued member declares: signed, unsigned, char, binary, date,
	// time, datetime, decimal, double, float, json, year, vector — or ""
	// when the member is not multi-valued or the target could not be
	// classified. The member-legality check applies the product's allowed
	// cast family to it (MySQL and TiDB allow disjoint subsets).
	MultiValuedCastType string `json:"multi_valued_cast_type,omitempty"`
	// MultiValuedCastLengthSet records that the authored cast target
	// carried an explicit length — char and binary targets require one on
	// both products (the unspecified-length form answers 1235).
	MultiValuedCastLengthSet bool `json:"multi_valued_cast_length_set,omitempty"`
}

// SetKeyParts installs the member list and derives every aggregate mark from
// it, so count fields can never drift from the member facts.
func (i *Index) SetKeyParts(parts []IndexKeyPart) {
	i.KeyParts = parts
	i.ExpressionCount, i.PrefixParts, i.DescParts, i.MultiValuedParts, i.UnmodeledKeyParts, i.DirectionUnknownParts = 0, 0, 0, 0, 0, 0
	i.HasExpressionKeys = false
	for _, part := range parts {
		switch part.Kind {
		case IndexKeyPartExpression:
			i.ExpressionCount++
			i.HasExpressionKeys = true
			if part.MultiValued {
				i.MultiValuedParts++
			}
		case IndexKeyPartUnmodeled:
			i.UnmodeledKeyParts++
		}
		if part.PrefixLength != nil {
			i.PrefixParts++
		}
		if part.Descending {
			i.DescParts++
		}
		if part.DirectionUnknown {
			i.DirectionUnknownParts++
		}
	}
}

// CloneKeyParts deep-copies the member list; ExpressionFacts are reference
// counts and strings, so a slice copy per part is the whole clone.
func CloneKeyParts(in []IndexKeyPart) []IndexKeyPart {
	if in == nil {
		return nil
	}
	out := make([]IndexKeyPart, len(in))
	for i, part := range in {
		out[i] = part
		if part.PrefixLength != nil {
			length := *part.PrefixLength
			out[i].PrefixLength = &length
		}
		if part.Expression != nil {
			facts := *part.Expression
			facts.References = append([]ColumnReference(nil), part.Expression.References...)
			facts.Functions = append([]string(nil), part.Expression.Functions...)
			facts.Features = append([]string(nil), part.Expression.Features...)
			out[i].Expression = &facts
		}
	}
	return out
}

// Index describes an index declaration.
type Index struct {
	Name              string         `json:"name"`
	Kind              IndexKind      `json:"kind,omitempty"`
	Columns           []string       `json:"columns,omitempty"`
	KeyParts          []IndexKeyPart `json:"key_parts,omitempty"`
	Cardinality       *int64         `json:"cardinality,omitempty"`
	AccessMethod      string         `json:"access_method,omitempty"`
	IncludedColumns   []string       `json:"included_columns,omitempty"`
	HasPredicate      bool           `json:"has_predicate,omitempty"`
	HasExpressionKeys bool           `json:"has_expression_keys,omitempty"`
	ExpressionCount   int            `json:"expression_count,omitempty"`
	// PrefixParts/DescParts count column-prefix lengths and descending key
	// parts parsed on the index but not yet modeled for audit semantics.
	PrefixParts int `json:"prefix_parts,omitempty"`
	DescParts   int `json:"desc_parts,omitempty"`
	// MultiValuedParts counts CAST(… AS … ARRAY) key parts — the MySQL
	// multi-valued index member form. Those parts are also expression key
	// parts (ExpressionCount/HasExpressionKeys still count them); the
	// mark exists so coverage and version applicability can name the
	// multi-valued aspect without inspecting expression contents.
	MultiValuedParts int `json:"multi_valued_parts,omitempty"`
	// Visibility is the effective index visibility fact: "visible",
	// "invisible", or "" when no fact was established. Provider-loaded
	// members carry the catalog answer; declared members carry the authored
	// attribute or "" when none was written.
	Visibility string `json:"visibility,omitempty"`
	// VisibilityExplicit records that a VISIBLE|INVISIBLE attribute was
	// authored on a declared index path. Provider and post-state members
	// keep this false — state claims the effective fact, not the authored
	// form.
	VisibilityExplicit bool `json:"visibility_explicit,omitempty"`
	// UnmodeledKeyParts counts member rows the model cannot classify as a
	// plain ascending column part (e.g. a NULL or unrecognized COLLATION on
	// a provider-loaded row). Any nonzero count means the member definition
	// is not fully known and the index must not enter ordinary-key
	// comparisons.
	UnmodeledKeyParts int `json:"unmodeled_key_parts,omitempty"`
	// DirectionUnknownParts counts classified members (column or
	// expression kind) whose sort direction the source could not prove.
	// Any nonzero count means the index's key ordering is unproven, so it
	// must not enter ordinary-key comparisons either.
	DirectionUnknownParts int `json:"direction_unknown_parts,omitempty"`
	// Global marks the TiDB-only GLOBAL index modifier parsed on table-level
	// UNIQUE/PRIMARY KEY constraints and standalone CREATE INDEX statements.
	// MySQL has no such modifier, so it is a vendor boundary there.
	Global bool `json:"global,omitempty"`
	// PrimaryKeyType records a parsed `PRIMARY KEY … CLUSTERED|NONCLUSTERED`
	// option (TiDB syntax) on a primary-key declaration: "clustered",
	// "nonclustered", or "" when unwritten. It decides whether a declared
	// TiDB single-int PK becomes the row handle (no operable index object)
	// or a real PRIMARY index object.
	PrimaryKeyType string `json:"primary_key_type,omitempty"`
	// UnmodeledOptions lists bounded names of parsed IndexOption members the
	// normalized model does not keep (comment, key_block_size, index_type,
	// with_parser, primary_key_type, split_opt, secondary_engine_attr,
	// columnar_replica). Visibility graduated to Visibility/
	// VisibilityExplicit. Any name present means that parsed aspect is
	// unaudited.
	UnmodeledOptions []string `json:"unmodeled_options,omitempty"`
	// NameUnknown marks a member whose stored name the projection cannot
	// prove — set on every member when StoredIndexNames cannot resolve the
	// collection under the product's auto-naming rule (cross-member suffix
	// effects make per-member honesty all-or-nothing there), and cleared by
	// construction everywhere else: provider names are catalog facts and
	// members merged by an authored ADD/CREATE carry an authored name.
	// Name-keyed lookups must only bind members with a proven name.
	// Internal audit marker; not serialized.
	NameUnknown bool `json:"-"`
}

// Constraint describes a non-index table constraint worth preserving for later rules.
type Constraint struct {
	Type              string   `json:"type"`
	Name              string   `json:"name,omitempty"`
	Columns           []string `json:"columns,omitempty"`
	ReferencedSchema  string   `json:"referenced_schema,omitempty"`
	ReferencedTable   string   `json:"referenced_table,omitempty"`
	ReferencedColumns []string `json:"referenced_columns,omitempty"`
	// UnmodeledParts counts local key parts (foreign keys) that carry more
	// than a plain column name: expression parts, explicit prefix lengths
	// (including zero), and descending parts.
	UnmodeledParts int `json:"unmodeled_parts,omitempty"`
	// UnmodeledReferencedParts is the same count over the referenced
	// IndexPartSpecification list of a foreign key.
	UnmodeledReferencedParts int `json:"unmodeled_referenced_parts,omitempty"`
	// OnDelete/OnUpdate carry the declared referential actions as normalized
	// lowercase tokens (restrict, cascade, set_null, no_action, set_default);
	// an empty string means the clause was not declared.
	OnDelete string `json:"on_delete,omitempty"`
	OnUpdate string `json:"on_update,omitempty"`
	// UnmodeledReferActions counts parsed reference options a foreign key
	// carries that the normalized model does not keep: MATCH clauses. ON
	// DELETE and ON UPDATE are modeled facts in OnDelete/OnUpdate.
	UnmodeledReferActions int `json:"unmodeled_refer_actions,omitempty"`
	// NameUnknown marks a member whose stored name is unproven — a bare
	// foreign key added through ordered state, where the server derives a
	// catalog name the statement itself does not author. Name-keyed
	// lookups must never bind or disprove on the empty stored name.
	NameUnknown bool `json:"-"`
}

// AlterColumnChange describes statement-local column-change intent.
// These flags are parser-neutral hints about what the ALTER statement touches;
// they do not claim live-schema source truth on their own.
type AlterColumnChange struct {
	TouchesNullability   bool `json:"touches_nullability,omitempty"`
	TouchesDefault       bool `json:"touches_default,omitempty"`
	TouchesAutoIncrement bool `json:"touches_auto_increment,omitempty"`
	// DeclaresPrimaryKey records that this column definition itself writes
	// PRIMARY KEY. It is parser-owned presence, not an added-primary-key
	// post-state, and it stays out of public JSON.
	DeclaresPrimaryKey bool `json:"-"`
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

// AlterPosition carries a parsed FIRST|AFTER column-position clause as a
// normalized fact: Kind is "first" or "after" and After is the normalized
// relative column name (empty for "first").
type AlterPosition struct {
	Kind  string `json:"kind"`
	After string `json:"after,omitempty"`
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
	// HasColumnPosition records a parsed FIRST|AFTER column-position clause;
	// Position carries the normalized fact (non-nil exactly when
	// HasColumnPosition is true).
	HasColumnPosition bool           `json:"has_column_position,omitempty"`
	Position          *AlterPosition `json:"position,omitempty"`
	// UnextractedOptions names parsed table-option clauses that were dropped
	// during extraction; see DDL.UnextractedOptions.
	UnextractedOptions []string `json:"unextracted_options,omitempty"`
}
