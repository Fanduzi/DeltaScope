// Package spec defines normalized statement specifications for rule evaluation.
package spec

import "strings"

// Metadata carries optional non-SQL facts for one statement evaluation.
type Metadata struct {
	Schema      string           `json:"schema,omitempty"`
	Instance    *InstanceFacts   `json:"instance,omitempty"`
	Version     *VersionIdentity `json:"version,omitempty"`
	TargetTable *TableSnapshot   `json:"target_table,omitempty"`
	Objects     []ObjectSnapshot `json:"objects,omitempty"`
}

// InstanceFacts are normalized server-level facts that influence audit behavior.
// Numeric facts pair a value with an explicit Known flag: a zero value is never
// treated as evidence — unknown facts stay unknown so rules can emit bounded
// evidence gaps instead of fabricating a configuration.
type InstanceFacts struct {
	Version                   string `json:"version,omitempty"`
	DefaultCharset            string `json:"default_charset,omitempty"`
	InnoDBLargePrefixEnabled  bool   `json:"innodb_large_prefix_enabled,omitempty"`
	InnoDBLargePrefixKnown    bool   `json:"innodb_large_prefix_known,omitempty"`
	InnoDBDefaultRowFormat    string `json:"innodb_default_row_format,omitempty"`
	InnoDBAdaptiveHashEnabled bool   `json:"innodb_adaptive_hash_enabled,omitempty"`
	InnoDBPageSizeBytes       int    `json:"innodb_page_size,omitempty"`
	InnoDBPageSizeKnown       bool   `json:"innodb_page_size_known,omitempty"`
	TiDBMaxIndexLengthBytes   int    `json:"tidb_max_index_length,omitempty"`
	TiDBMaxIndexLengthKnown   bool   `json:"tidb_max_index_length_known,omitempty"`

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
}

// TableSnapshot is the current metadata-backed shape of a target table.
// Member collections keep their source semantics: a nil Columns marks a
// withheld column set (unknown), while PrimaryKey/Indexes/Constraints treat
// nil as "loaded, absent/empty" — the *Unknown flags below override that
// convention when a projection cannot vouch for those collections.
type TableSnapshot struct {
	Schema      string            `json:"schema,omitempty"`
	Exists      bool              `json:"exists"`
	Table       *Table            `json:"table,omitempty"`
	Columns     []Column          `json:"columns,omitempty"`
	PrimaryKey  *Index            `json:"primary_key,omitempty"`
	Indexes     []Index           `json:"indexes,omitempty"`
	Constraints []Constraint      `json:"constraints,omitempty"`
	Options     map[string]string `json:"options,omitempty"`

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
	// GLOBAL index option is gated on. nil means the fact was never loaded
	// (offline statement or a pre-expansion provider), not "not
	// partitioned".
	Partitioned *bool `json:"-"`
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

// ObjectSnapshot carries the metadata-validated state of one non-table database object.
type ObjectSnapshot struct {
	Schema              string            `json:"schema,omitempty"`
	Type                string            `json:"type,omitempty"`
	Name                string            `json:"name,omitempty"`
	Status              MetadataStatus    `json:"status,omitempty"`
	Exists              bool              `json:"exists"`
	Attributes          map[string]string `json:"attributes,omitempty"`
	AmbiguousCandidates []string          `json:"ambiguous_candidates,omitempty"`
}

// ObjectLookupRequest describes one object to resolve from live metadata.
type ObjectLookupRequest struct {
	Schema     string
	Type       string
	Name       string
	Qualifiers map[string]string
}

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
