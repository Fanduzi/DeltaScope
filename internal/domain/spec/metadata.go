// Package spec defines normalized statement specifications for rule evaluation.
// input: optional metadata-aware audit facts such as instance variables and target-table snapshots
// output: parser-neutral metadata structures and lookup helpers for future rules
// pos: domain metadata model shared by offline and metadata-aware audit paths
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// Metadata carries optional non-SQL facts for one statement evaluation.
type Metadata struct {
	Schema      string           `json:"schema,omitempty"`
	Instance    *InstanceFacts   `json:"instance,omitempty"`
	TargetTable *TableSnapshot   `json:"target_table,omitempty"`
	Objects     []ObjectSnapshot `json:"objects,omitempty"`
}

// InstanceFacts are normalized server-level facts that influence audit behavior.
type InstanceFacts struct {
	Version                   string `json:"version,omitempty"`
	DefaultCharset            string `json:"default_charset,omitempty"`
	InnoDBLargePrefixEnabled  bool   `json:"innodb_large_prefix_enabled,omitempty"`
	InnoDBDefaultRowFormat    string `json:"innodb_default_row_format,omitempty"`
	InnoDBAdaptiveHashEnabled bool   `json:"innodb_adaptive_hash_enabled,omitempty"`
}

// TableSnapshot is the current metadata-backed shape of a target table.
type TableSnapshot struct {
	Schema      string            `json:"schema,omitempty"`
	Exists      bool              `json:"exists"`
	Table       *Table            `json:"table,omitempty"`
	Columns     []Column          `json:"columns,omitempty"`
	PrimaryKey  *Index            `json:"primary_key,omitempty"`
	Indexes     []Index           `json:"indexes,omitempty"`
	Constraints []Constraint      `json:"constraints,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
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
