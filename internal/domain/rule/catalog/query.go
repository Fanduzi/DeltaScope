// Package catalog defines explanation-oriented metadata for shipped audit rules.
// This file adds structured query support for filtering catalog entries.
// input: catalog entries and a structured query with dialect/level/kind/category/search/limit filters
// output: filtered result set with total count
// pos: query core for rule discoverability (v0.290.0 Task 3)
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"fmt"
	"strings"
)

// validDialects enumerates accepted dialect filter values.
var validDialects = map[string]bool{
	"mysql":      true,
	"tidb":       true,
	"postgresql": true,
	"common":     true,
}

// validLevels enumerates accepted level filter values.
var validLevels = map[string]bool{
	"blocker": true,
	"warning": true,
	"notice":  true,
}

// validKinds enumerates accepted kind filter values.
var validKinds = map[string]bool{
	"ddl": true,
	"dml": true,
}

// Query describes a structured filter over catalog entries.
type Query struct {
	Dialect  string // mysql, tidb, postgresql, common; empty means no filter
	Level    string // blocker, warning, notice; empty means no filter
	Kind     string // ddl, dml; empty means no filter
	Category string // case-insensitive substring match; empty means no filter
	Search   string // case-insensitive substring match across multiple fields; empty means no filter
	Limit    int    // max entries to return; 0 means no explicit limit
}

// Result holds the outcome of a catalog query.
type Result struct {
	Entries []Entry // filtered entries in deterministic order
	Total   int     // total matching entries before limit
}

// Validate checks that enum filters contain valid values and limit is non-negative.
// Returns an error describing the first invalid field, or nil if valid.
func (q Query) Validate() error {
	if q.Dialect != "" && !validDialects[q.Dialect] {
		return fmt.Errorf("invalid dialect %q: must be one of mysql, tidb, postgresql, common", q.Dialect)
	}
	if q.Level != "" && !validLevels[q.Level] {
		return fmt.Errorf("invalid level %q: must be one of blocker, warning, notice", q.Level)
	}
	if q.Kind != "" && !validKinds[q.Kind] {
		return fmt.Errorf("invalid kind %q: must be one of ddl, dml", q.Kind)
	}
	if q.Limit < 0 {
		return fmt.Errorf("invalid limit %d: must be non-negative", q.Limit)
	}
	return nil
}

// QueryEntries filters a slice of catalog entries according to the query.
// Entries are not reordered; deterministic input order is preserved.
// An empty result set returns Entries as a non-nil empty slice.
func QueryEntries(entries []Entry, q Query) (Result, error) {
	if err := q.Validate(); err != nil {
		return Result{}, err
	}

	matched := make([]Entry, 0, len(entries))
	searchNeedle := strings.ToLower(strings.TrimSpace(q.Search))
	categoryNeedle := strings.ToLower(strings.TrimSpace(q.Category))

	for _, entry := range entries {
		if !matchesDialect(entry, q.Dialect) {
			continue
		}
		if !matchesLevel(entry, q.Level) {
			continue
		}
		if !matchesKind(entry, q.Kind) {
			continue
		}
		if !matchesCategory(entry, categoryNeedle) {
			continue
		}
		if !matchesSearch(entry, searchNeedle) {
			continue
		}
		matched = append(matched, cloneEntry(entry))
	}

	total := len(matched)

	result := Result{
		Entries: matched,
		Total:   total,
	}

	// Apply limit if explicitly set.
	if q.Limit > 0 && total > q.Limit {
		// Keep the underlying slice capacity for efficiency but slice to limit.
		result.Entries = matched[:q.Limit]
	}

	// Guarantee non-nil empty slice.
	if result.Entries == nil {
		result.Entries = []Entry{}
	}

	return result, nil
}

// matchesDialect checks whether an entry covers the requested dialect.
func matchesDialect(entry Entry, dialect string) bool {
	if dialect == "" {
		return true
	}
	for _, d := range entry.Dialects {
		if d == dialect {
			return true
		}
	}
	return false
}

// matchesLevel checks whether an entry has the requested default level.
func matchesLevel(entry Entry, level string) bool {
	if level == "" {
		return true
	}
	return string(entry.DefaultLevel) == level
}

// matchesKind checks whether an entry includes the requested statement kind.
func matchesKind(entry Entry, kind string) bool {
	if kind == "" {
		return true
	}
	for _, k := range entry.StatementKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// matchesCategory checks whether an entry's category contains the needle
// as a case-insensitive substring.
func matchesCategory(entry Entry, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(entry.Category), needle)
}

// matchesSearch checks whether an entry contains the needle as a
// case-insensitive substring across rule_id, config_key, summary,
// why, risk, suggestion, category, and tags.
func matchesSearch(entry Entry, needle string) bool {
	if needle == "" {
		return true
	}

	// Build a searchable text from all covered fields.
	var b strings.Builder
	b.WriteString(entry.RuleID)
	b.WriteByte(' ')
	b.WriteString(entry.ConfigKey)
	b.WriteByte(' ')
	b.WriteString(entry.Summary)
	b.WriteByte(' ')
	b.WriteString(entry.Why)
	b.WriteByte(' ')
	b.WriteString(entry.Risk)
	b.WriteByte(' ')
	b.WriteString(entry.Suggestion)
	b.WriteByte(' ')
	b.WriteString(entry.Category)
	b.WriteByte(' ')
	for _, tag := range entry.Tags {
		b.WriteString(tag)
		b.WriteByte(' ')
	}

	return strings.Contains(strings.ToLower(b.String()), needle)
}
