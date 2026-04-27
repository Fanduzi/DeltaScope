package audit

import (
	"strings"
)

// attachParsedStatementLocations populates Line and Column on each ParsedStatement
// by progressively matching RawSQL fragments against the original source buffer.
// Column is always 1 since SQL statements start at the beginning of a line.
func attachParsedStatementLocations(stmts []ParsedStatement, source string) {
	offset := 0
	for i := range stmts {
		raw := normalizeRawSQL(stmts[i].RawSQL)
		if raw == "" {
			continue
		}
		idx := indexFrom(source, raw, offset)
		if idx < 0 {
			continue
		}
		stmts[i].Line = lineAt(source, idx)
		stmts[i].Column = 1
		offset = idx + len(raw)
	}
}

// normalizeRawSQL strips leading/trailing whitespace so the source search works
// even when the parser adds/removes whitespace from the raw text.
func normalizeRawSQL(s string) string {
	return strings.TrimSpace(s)
}

// indexFrom returns the index of the first occurrence of substr in s[start:],
// adjusted to be relative to the start of s.
func indexFrom(s, substr string, start int) int {
	if start > len(s) {
		return -1
	}
	idx := strings.Index(s[start:], substr)
	if idx < 0 {
		return -1
	}
	return start + idx
}

// lineAt returns the 1-based line number at byte offset idx in source.
func lineAt(source string, idx int) int {
	if idx > len(source) {
		idx = len(source)
	}
	line := 1
	for i := 0; i < idx; i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}
