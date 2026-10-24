// Package spec records how the product names index members a CREATE TABLE
// leaves unnamed, so ordered post-state keeps the stored identity a
// successor ALTER/DROP resolves against instead of a known-empty name.
// input: a declaration-ordered Index list exactly as the statement authored
// it (authored names verbatim, empty names unclaimed)
// output: the same list with every member's stored name filled — or a
// provable=false answer when any member's stored name cannot be derived
// under the native rule, meaning the name space is unvouched as a whole
// pos: derived-state naming beside primary_key_role.go; consumed by the
// batch-state create-table projection (issue #89 T10-R3)
// note: if this file changes, update this header and module README.md.
package spec

import (
	"fmt"
	"strings"
)

// StoredIndexNames applies the product's auto-naming rule to a CREATE
// TABLE's declared index list in declaration order. Native anchor: MySQL
// 8.0.46 mysql-test/r/invisible_indexes.result L274-285 — an unnamed index
// member takes its first column member's name, and a name already claimed
// (by an authored member or an earlier auto-name) gains a numeric suffix
// (_2, _3, …) until free. Index-name comparison is case-insensitive.
//
// The answer is all-or-nothing: provable=false when ANY member's name
// cannot be proven — an unnamed member whose first key part is not a plain
// column (an expression key part stores under a server-generated name the
// declaration cannot predict), an unnamed member with no members at all,
// or an authored name colliding with an already-claimed name (natively
// ER_DUP_KEY_NAME 1061 — the statement fails, so no stored shape exists to
// vouch for). Callers must then keep the collection's name space unknown
// rather than publish partially-guessed identities.
func StoredIndexNames(indexes []Index) (resolved []Index, provable bool) {
	used := make(map[string]struct{}, len(indexes))
	resolved = make([]Index, len(indexes))
	for i, index := range indexes {
		resolved[i] = index
		name := strings.TrimSpace(index.Name)
		if name != "" {
			// An authored name duplicating any earlier claim fails the
			// whole CREATE natively — nothing stored is provable.
			if _, taken := used[strings.ToLower(name)]; taken {
				return nil, false
			}
		} else {
			base, ok := autoNameBase(index)
			if !ok {
				return nil, false
			}
			name = base
			for suffix := 2; ; suffix++ {
				if _, taken := used[strings.ToLower(name)]; !taken {
					break
				}
				name = fmt.Sprintf("%s_%d", base, suffix)
			}
		}
		used[strings.ToLower(name)] = struct{}{}
		resolved[i].Name = name
	}
	return resolved, true
}

// autoNameBase returns the column name an unnamed index member stores
// under — its first member, whether that member arrives as a key part or
// only through the plain Columns projection. A non-column first member
// (expression, unmodeled) stores under a server-generated name the
// declaration cannot prove, so the base is unavailable.
func autoNameBase(index Index) (string, bool) {
	if len(index.KeyParts) > 0 {
		first := index.KeyParts[0]
		if first.Kind == IndexKeyPartColumn && strings.TrimSpace(first.Column) != "" {
			return first.Column, true
		}
		return "", false
	}
	if len(index.Columns) > 0 && strings.TrimSpace(index.Columns[0]) != "" {
		return index.Columns[0], true
	}
	return "", false
}
