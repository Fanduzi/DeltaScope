// Package spec defines normalized statement specifications for rule evaluation.
// input: optional metadata-aware audit facts such as instance variables and target-table snapshots
// output: parser-neutral metadata structures and lookup helpers for future rules
// pos: domain metadata model shared by offline and metadata-aware audit paths
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// Metadata carries optional non-SQL facts for one statement evaluation.
type Metadata struct {
	Schema      string         `json:"schema,omitempty"`
	Instance    *InstanceFacts `json:"instance,omitempty"`
	TargetTable *TableSnapshot `json:"target_table,omitempty"`
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
