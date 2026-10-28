// Package spec: partition-boundary catalog projection and comparison.
//
// The catalog stores PARTITION_DESCRIPTION as authored text (`120`,
// `'abc'`, `MAXVALUE`, `2,5`); the declaration side stores structured
// BoundLiteral rows. This file projects the catalog text into the same
// BoundLiteral vocabulary and provides the bounded comparisons the
// topology rules and ordered-state transitions consume — literal-domain
// equality and RANGE bound ordering. Expression/non-literal items never
// compare equal: comparison answers "unproven", not "same".
package spec

import (
	"math/big"
	"strings"
)

// partitionBoundClauseKind maps the normalized method vocabulary to its
// member boundary clause shape: "less_than" on the RANGE family, "in" on
// the LIST family, "" on HASH/KEY (members carry no clause).
func partitionBoundClauseKind(method string) string {
	switch method {
	case "range", "range columns":
		return "less_than"
	case "list", "list columns":
		return "in"
	default:
		return ""
	}
}

// CatalogPartitionDefs aligns a provider member directory (names in
// ordinal order, one PARTITION_DESCRIPTION text per name) into the
// comparable member-def list. nil names means an unobserved directory and
// stays nil. A member whose description is empty or unprojectable keeps a
// clause shell with no rows — an unproven bound, never a bound-free member
// (except HASH/KEY methods, which carry no clause by definition).
func CatalogPartitionDefs(names, descriptions []string, method string) []PartitionDef {
	if names == nil {
		return nil
	}
	defs := make([]PartitionDef, len(names))
	for i, name := range names {
		desc := ""
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		defs[i] = PartitionDef{Name: name, Bound: catalogPartitionBound(desc, method)}
	}
	return defs
}

// catalogPartitionBound projects one member's PARTITION_DESCRIPTION text
// into a comparable bound under the method's clause shape.
func catalogPartitionBound(desc, method string) *PartitionBound {
	trimmed := strings.TrimSpace(desc)
	kind := partitionBoundClauseKind(method)
	if kind == "" {
		return nil
	}
	bound := &PartitionBound{Kind: kind}
	if trimmed == "" {
		return bound
	}
	switch kind {
	case "less_than":
		if strings.EqualFold(trimmed, "maxvalue") {
			bound.MaxValue = true
			return bound
		}
		row, ok := catalogLiteralRow(trimmed)
		if ok {
			bound.Rows = [][]BoundLiteral{row}
		}
	case "in":
		if strings.EqualFold(trimmed, "default") {
			bound.Default = true
			return bound
		}
		rows, ok := catalogLiteralRows(trimmed)
		if ok {
			bound.Rows = rows
		}
	}
	return bound
}

// catalogLiteralRows projects LIST description text into value rows. The
// catalog flattens `VALUES IN (a,b)` to `a,b` (one row per item) and keeps
// `(...)` tuple grouping when present.
func catalogLiteralRows(desc string) ([][]BoundLiteral, bool) {
	parts := splitTopLevel(desc)
	rows := make([][]BoundLiteral, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && part[0] == '(' && part[len(part)-1] == ')' {
			row, ok := catalogLiteralRow(part[1 : len(part)-1])
			if !ok {
				return nil, false
			}
			rows = append(rows, row)
			continue
		}
		lit, ok := catalogLiteral(part)
		if !ok {
			return nil, false
		}
		rows = append(rows, []BoundLiteral{lit})
	}
	return rows, true
}

// catalogLiteralRow projects one comma-separated literal tuple — the
// single LESS THAN row or one parenthesized IN tuple.
func catalogLiteralRow(text string) ([]BoundLiteral, bool) {
	parts := splitTopLevel(text)
	row := make([]BoundLiteral, 0, len(parts))
	for _, part := range parts {
		lit, ok := catalogLiteral(part)
		if !ok {
			return nil, false
		}
		row = append(row, lit)
	}
	return row, true
}

// splitTopLevel splits description text on commas that sit outside parens
// and string quoting.
func splitTopLevel(text string) []string {
	parts := make([]string, 0, 4)
	depth := 0
	inString := false
	start := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			if inString && i+1 < len(text) && text[i+1] == '\'' {
				i++
				continue
			}
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString && depth > 0 {
				depth--
			}
		case ',':
			if !inString && depth == 0 {
				parts = append(parts, text[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, text[start:])
	return parts
}

// catalogLiteral projects one catalog literal token into a comparable
// BoundLiteral: NULL, MAXVALUE (a +∞ item inside a COLUMNS tuple), a
// quoted string (with ” unescaping), or a numeric token. Anything else —
// functions, placeholders, unbalanced quoting — is unprojectable
// (ok=false), never a fabricated literal.
func catalogLiteral(text string) (BoundLiteral, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return BoundLiteral{}, false
	}
	if strings.EqualFold(t, "null") {
		return BoundLiteral{Kind: PartitionBoundNull}, true
	}
	if strings.EqualFold(t, "maxvalue") {
		return BoundLiteral{Kind: PartitionBoundMaxValue}, true
	}
	if len(t) >= 2 && t[0] == '\'' && t[len(t)-1] == '\'' {
		return BoundLiteral{Kind: PartitionBoundString, Value: strings.ReplaceAll(t[1:len(t)-1], "''", "'")}, true
	}
	if isNumericBoundText(t) {
		return BoundLiteral{Kind: PartitionBoundNumeric, Value: t}, true
	}
	return BoundLiteral{}, false
}

// isNumericBoundText reports whether the token reads as a numeric literal
// (digits, optional sign, optional decimal point/exponent). big.Rat
// comparison does the real equality work later — this is only the kind
// classifier.
func isNumericBoundText(t string) bool {
	seen := false
	for i, r := range t {
		switch {
		case r >= '0' && r <= '9':
			seen = true
		case (r == '+' || r == '-') && i == 0:
		case (r == '.' || r == 'e' || r == 'E') && seen:
		default:
			return false
		}
	}
	return seen
}

// BoundLiteralEqual compares two bound items across the literal domains:
// numerics compare by value (big.Rat, float fallback for exponent forms),
// NULLs and MAXVALUE sentinels compare by kind, strings by decoded bytes
// when identical — byte-different strings stay unproven (ok=false), not
// proven-unequal, because the collation domain is unmodeled (a
// case-insensitive collation equates 'a' and 'A'; no product collation
// fact answers it here). Any expression-shaped or unprojectable side
// answers ok=false — equality is never claimed for non-literal content.
func BoundLiteralEqual(a, b BoundLiteral) (equal, ok bool) {
	if a.Kind == PartitionBoundExpression || b.Kind == PartitionBoundExpression ||
		a.Kind == "" || b.Kind == "" {
		return false, false
	}
	if a.Kind != b.Kind {
		return false, true
	}
	switch a.Kind {
	case PartitionBoundNull, PartitionBoundMaxValue:
		return true, true
	case PartitionBoundString:
		if a.Value == b.Value {
			return true, true
		}
		return false, false
	case PartitionBoundNumeric:
		return numericBoundEqual(a.Value, b.Value)
	}
	return false, false
}

// numericBoundEqual compares two numeric literal texts by value.
func numericBoundEqual(a, b string) (equal, ok bool) {
	ra, oka := new(big.Rat).SetString(a)
	rb, okb := new(big.Rat).SetString(b)
	if oka && okb {
		return ra.Cmp(rb) == 0, true
	}
	fa, ea := new(big.Float).SetString(a)
	fb, eb := new(big.Float).SetString(b)
	if ea && eb {
		return fa.Cmp(fb) == 0, true
	}
	return false, false
}

// BoundRowEqual compares two bound rows item-wise under BoundLiteralEqual:
// equal only when every item provably equals its counterpart; ok=false
// when any item pair is undecidable (so "not equal" is never confused
// with "unproven").
func BoundRowEqual(a, b []BoundLiteral) (equal, ok bool) {
	if len(a) != len(b) {
		return false, true
	}
	for i := range a {
		eq, itemOK := BoundLiteralEqual(a[i], b[i])
		if !itemOK {
			return false, false
		}
		if !eq {
			return false, true
		}
	}
	return true, true
}

// BoundRowCompare orders two bound rows: -1 a<b, 0 equal, +1 a>b.
// Item-wise numeric comparison with the first non-zero pair deciding;
// a MAXVALUE item is +∞ (bigger than every non-MAXVALUE item, equal to
// another MAXVALUE at the same position). String items prove only
// byte-equality — any byte difference answers ok=false because the
// collation domain is unmodeled, never a byte-order verdict. NULL items
// are incomparable (RANGE bounds never carry them), and any
// expression/unprojected pair answers ok=false.
func BoundRowCompare(a, b []BoundLiteral) (cmp int, ok bool) {
	if len(a) != len(b) {
		return 0, false
	}
	for i := range a {
		if a[i].Kind == PartitionBoundMaxValue || b[i].Kind == PartitionBoundMaxValue {
			if a[i].Kind == b[i].Kind {
				continue
			}
			if a[i].Kind == PartitionBoundMaxValue {
				return 1, true
			}
			return -1, true
		}
		if a[i].Kind == PartitionBoundExpression || b[i].Kind == PartitionBoundExpression ||
			a[i].Kind == "" || b[i].Kind == "" {
			return 0, false
		}
		if a[i].Kind != b[i].Kind {
			return 0, false
		}
		switch a[i].Kind {
		case PartitionBoundNull:
			return 0, false
		case PartitionBoundString:
			if a[i].Value != b[i].Value {
				return 0, false
			}
		case PartitionBoundNumeric:
			ra, oka := new(big.Rat).SetString(a[i].Value)
			rb, okb := new(big.Rat).SetString(b[i].Value)
			if !oka || !okb {
				return 0, false
			}
			if c := ra.Cmp(rb); c != 0 {
				return c, true
			}
		default:
			return 0, false
		}
	}
	return 0, true
}

// BoundRowLessThan reports whether row a provably precedes row b under
// BoundRowCompare — ok=false keeps "unknown order" distinct from "not
// less".
func BoundRowLessThan(a, b []BoundLiteral) (less, ok bool) {
	cmp, ok := BoundRowCompare(a, b)
	if !ok {
		return false, false
	}
	return cmp < 0, true
}

// LessThanBoundUpper returns the member's upper edge: its single less_than
// bound row, MaxValue as a distinct sentinel state, and ok=false when the
// bound is absent/unproven for ordering. A row composed entirely of
// MAXVALUE items normalizes to the scalar sentinel — the same +∞ tail a
// scalar `VALUES LESS THAN MAXVALUE` declares.
func LessThanBoundUpper(bound *PartitionBound) (row []BoundLiteral, maxValue, ok bool) {
	if bound == nil || bound.Kind != "less_than" {
		return nil, false, false
	}
	if bound.MaxValue {
		return nil, true, true
	}
	if len(bound.Rows) != 1 || len(bound.Rows[0]) == 0 {
		return nil, false, false
	}
	allMax := true
	for _, item := range bound.Rows[0] {
		if item.Kind == PartitionBoundExpression || item.Kind == "" {
			return nil, false, false
		}
		if item.Kind != PartitionBoundMaxValue {
			allMax = false
		}
	}
	if allMax {
		return nil, true, true
	}
	return bound.Rows[0], false, true
}

// BoundComparable reports whether every item in a member bound is a
// projected literal the bounded comparators can adjudicate: nil bound,
// unshaped clause kind, empty rows, and expression items are all
// uncomparable. MAXVALUE sentinels, NULLs, strings, and numerics are
// projected literals — comparability answers "the comparison may run",
// not that it always decides (string ordering still answers unproven).
func BoundComparable(bound *PartitionBound) bool {
	if bound == nil || bound.Kind == "" {
		return false
	}
	if bound.MaxValue || bound.Default {
		return true
	}
	if len(bound.Rows) == 0 {
		return false
	}
	for _, row := range bound.Rows {
		if len(row) == 0 {
			return false
		}
		for _, item := range row {
			switch item.Kind {
			case PartitionBoundNumeric, PartitionBoundString, PartitionBoundNull, PartitionBoundMaxValue:
			default:
				return false
			}
		}
	}
	return true
}

// InBoundsOverlap reports whether two IN bounds provably share a value
// row: (true,true) proven overlap, (false,true) proven disjoint,
// (false,false) unproven — an expression/unprojectable item pair, a
// missing clause, or an empty shell leaves the overlap undecidable.
func InBoundsOverlap(a, b *PartitionBound) (overlap, ok bool) {
	if a == nil || b == nil || a.Kind != "in" || b.Kind != "in" {
		return false, false
	}
	if a.Default || b.Default || len(a.Rows) == 0 || len(b.Rows) == 0 {
		return false, false
	}
	unproven := false
	for _, rowA := range a.Rows {
		for _, rowB := range b.Rows {
			equal, eqOK := BoundRowEqual(rowA, rowB)
			if eqOK && equal {
				return true, true
			}
			if !eqOK {
				unproven = true
			}
		}
	}
	return false, !unproven
}
