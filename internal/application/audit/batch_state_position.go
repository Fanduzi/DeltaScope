// Package audit resolves the declared FIRST|AFTER column position against a
// known column list and applies the bounded move/insert.
// input: a normalized spec.AlterPosition fact and an ordered column slice
// output: a resolved insertion index or the failed-premise marker
// pos: T08 column-position successor semantics shared by single-action
// publications and the bounded multi-action successor; the validation list is
// the dialect-owned premise set (MySQL accumulating, TiDB frozen pre-state)
// while the applied list is the evolving working shape
// note: if this file changes, update this header and module README.md.
package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// positionResolveIndex returns the index AFTER resolves to inside validation
// columns, or -1 when the referenced column is absent. `self` names a column
// identity the position must not point at (the moved or added column's own
// final name is always out — FIRST carries no reference and reports -2).
func positionResolveIndex(columns []spec.Column, position *spec.AlterPosition, self string) (index int, selfRef bool) {
	if position == nil || position.Kind != "after" {
		return -2, false
	}
	after := strings.TrimSpace(position.After)
	if after == "" {
		return -1, false
	}
	if self != "" && strings.EqualFold(after, self) {
		return -1, true
	}
	for i := range columns {
		if strings.EqualFold(columns[i].Name, after) {
			return i, true
		}
	}
	return -1, false
}

// insertColumnAt returns columns with column inserted at index. An out of
// range index appends — callers bound the index before calling.
func insertColumnAt(columns []spec.Column, column spec.Column, index int) []spec.Column {
	if index < 0 || index > len(columns) {
		index = len(columns)
	}
	out := make([]spec.Column, 0, len(columns)+1)
	out = append(out, columns[:index]...)
	out = append(out, column)
	out = append(out, columns[index:]...)
	return out
}

// removeColumnAt returns columns without the element at index.
func removeColumnAt(columns []spec.Column, index int) []spec.Column {
	out := make([]spec.Column, 0, len(columns)-1)
	out = append(out, columns[:index]...)
	out = append(out, columns[index+1:]...)
	return out
}

// applyColumnPosition moves the element at fromIndex inside columns to the
// position FIRST|AFTER declares. The AFTER reference resolves against the
// list without the moving element, matching native move semantics: MODIFY b
// AFTER c on (a,b,c) yields (a,c,b). The caller must have already validated
// the reference against the dialect's premise set — this helper only reports
// whether the same reference resolves on the applied list, so a TiDB-shape
// reference that a sibling consumed fails the apply instead of landing wrong.
func applyColumnPosition(columns []spec.Column, fromIndex int, position *spec.AlterPosition, selfName string) ([]spec.Column, bool) {
	if position == nil {
		return columns, true
	}
	if position.Kind == "first" {
		moved := columns[fromIndex]
		rest := removeColumnAt(columns, fromIndex)
		return insertColumnAt(rest, moved, 0), true
	}
	if position.Kind != "after" {
		return nil, false
	}
	moved := columns[fromIndex]
	rest := removeColumnAt(columns, fromIndex)
	idx, ok := positionResolveIndex(rest, position, selfName)
	if !ok || idx < 0 {
		return nil, false
	}
	return insertColumnAt(rest, moved, idx+1), true
}
