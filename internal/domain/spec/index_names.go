// Package spec records how the product names index members a CREATE TABLE
// leaves unnamed, so ordered post-state keeps the stored identity a
// successor ALTER/DROP resolves against instead of a known-empty name.
// input: a declaration-ordered Index list exactly as the statement authored
// it (authored names verbatim, empty names unclaimed)
// output: the same list with every member's stored name filled — or a
// provable=false answer when any member's stored name cannot be derived
// under the native rule, meaning the name space is unvouched as a whole
// pos: derived-state naming beside primary_key_role.go; consumed by the
// batch-state create-table projection (issue #89 T10-R3/R4)
// note: if this file changes, update this header and module README.md.
package spec

import (
	"fmt"
	"strings"
)

// indexNameLengthLimit is the identifier bound both anchors enforce on
// index names; a suffix that would push a derived name past it truncates
// natively, so the shape stays unprovable instead.
const indexNameLengthLimit = 64

// StoredIndexNames applies the product's auto-naming rule to a CREATE
// TABLE's declared index list. The rule is dialect-bound, anchored
// natively on MySQL 8.0.46 and TiDB v8.5.0:
//
//   - MySQL claims names in declaration order. `primary` is reserved for
//     the primary key, so a base colliding with it (or any earlier claim)
//     gains an ordered numeric suffix — an unnamed index on column
//     `primary` stores as primary_2, then primary_3 (sql_table.cc
//     make_unique_key_name). Any expression-bearing member takes the
//     `functional_index` base regardless of position.
//   - TiDB pre-occupies every authored name before deriving automatic
//     ones (pkg/ddl/create_table.go setEmptyConstraintName): an unnamed
//     member whose base an authored name claimed is suffixed, never a
//     native failure. `primary` is reserved the same way — an unnamed
//     index on column `primary` stores as primary_2. Any
//     expression-bearing member takes the `expression_index` base
//     regardless of position.
//
// The answer is all-or-nothing: provable=false when ANY member's name
// cannot be proven — an unnamed member whose member list cannot classify
// every key part (an unmodeled member, or no members at all), a derived
// candidate that would need truncating past the identifier limit, an
// authored `primary` name (natively refused on both anchors — ER_WRONG_NAME_FOR_INDEX 1280),
// or a MySQL sequential claim that collides (an authored name
// duplicating an already-claimed name fails the CREATE natively — no
// stored shape exists to vouch for). Callers must then keep the
// collection's name space unknown rather than publish partially-guessed
// identities.
func StoredIndexNames(dialect Dialect, indexes []Index) (resolved []Index, provable bool) {
	used := make(map[string]struct{}, len(indexes)+1)
	resolved = make([]Index, len(indexes))
	// `primary` is reserved on both anchors — the unnamed base colliding
	// with it suffixes (primary_2), and an authored claim fails natively.
	used["primary"] = struct{}{}
	if dialect == DialectTiDB {
		// Pass one: authored names pre-occupy the space before any
		// derivation. Duplicates between authored members still fail the
		// statement natively.
		for i, index := range indexes {
			resolved[i] = index
			name := strings.TrimSpace(index.Name)
			if name == "" {
				continue
			}
			lower := strings.ToLower(name)
			if _, taken := used[lower]; taken {
				return nil, false
			}
			used[lower] = struct{}{}
			resolved[i].Name = name
		}
	}
	for i, index := range indexes {
		name := strings.TrimSpace(index.Name)
		if dialect == DialectTiDB && name != "" {
			// Authored names were claimed verbatim in pass one — keep the
			// trimmed identity pass one already resolved.
			continue
		}
		resolved[i] = index
		if name != "" {
			// Sequential claim: colliding with an earlier claim — authored
			// or derived, including the reserved `primary` — fails the
			// whole CREATE natively (ER_DUP_KEY_NAME 1061).
			if _, taken := used[strings.ToLower(name)]; taken {
				return nil, false
			}
			used[strings.ToLower(name)] = struct{}{}
			resolved[i].Name = name
			continue
		}
		base, ok := autoNameBase(dialect, index)
		if !ok {
			return nil, false
		}
		name = base
		for suffix := 2; ; suffix++ {
			if len(name) > indexNameLengthLimit {
				// A suffix beyond the identifier limit truncates natively —
				// the stored string is unprovable rather than guessed.
				return nil, false
			}
			if _, taken := used[strings.ToLower(name)]; !taken {
				break
			}
			name = fmt.Sprintf("%s_%d", base, suffix)
		}
		used[strings.ToLower(name)] = struct{}{}
		resolved[i].Name = name
	}
	return resolved, true
}

// autoNameBase returns the base an unnamed index member derives its stored
// name from. Any expression member — in any position — selects the
// product's functional base (`functional_index` on MySQL,
// `expression_index` on TiDB); an all-column member derives from its
// first column. A member whose parts cannot all classify (unmodeled, or
// none recorded) withholds the base.
func autoNameBase(dialect Dialect, index Index) (string, bool) {
	expression, unmodeled := false, false
	if len(index.KeyParts) > 0 {
		for _, part := range index.KeyParts {
			switch part.Kind {
			case IndexKeyPartExpression:
				expression = true
			case IndexKeyPartUnmodeled:
				unmodeled = true
			}
		}
	} else {
		// A legacy projection without member rows still carries the
		// aggregate marks.
		expression = index.ExpressionCount > 0 || index.HasExpressionKeys
		unmodeled = index.UnmodeledKeyParts > 0
	}
	if unmodeled {
		return "", false
	}
	if expression {
		if dialect == DialectTiDB {
			return "expression_index", true
		}
		return "functional_index", true
	}
	var base string
	if len(index.KeyParts) > 0 {
		first := index.KeyParts[0]
		if first.Kind != IndexKeyPartColumn {
			return "", false
		}
		base = strings.TrimSpace(first.Column)
	} else if len(index.Columns) > 0 {
		base = strings.TrimSpace(index.Columns[0])
	}
	if base == "" {
		return "", false
	}
	return base, true
}
