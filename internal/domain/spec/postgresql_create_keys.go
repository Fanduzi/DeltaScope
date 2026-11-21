package spec

import "slices"

// PGCreateKeys preserves PostgreSQL key constraints in declaration order.
type PGCreateKeys struct {
	Target       PGDestructiveTableTarget
	IfNotExists  bool
	Unbound      bool
	Declarations []PGCreateKeyDeclaration
}

// PGCreateKeyDeclaration keeps key and INCLUDE members separate and in AST order.
type PGCreateKeyDeclaration struct {
	Primary, UsingIndex, NullsNotDistinct bool
	Keys, Include                         []string
}

// PGCreateKeyRefusal checks direct members before use of the prospective table.
func PGCreateKeyRefusal(ddl *DDL) (reason, fact string) {
	keys := ddl.PGCreateKeys
	if keys == nil || keys.Unbound {
		return "", ""
	}
	var primary, system bool
	for _, declaration := range keys.Declarations {
		if declaration.Primary && primary {
			return "create_keys_multiple_primary", ""
		}
		primary = primary || declaration.Primary
		if declaration.UsingIndex {
			return "create_keys_using_existing_index", ""
		}
		if len(declaration.Keys) == 0 {
			return "", "create_table.key_members"
		}
		seen := make(map[string]bool, len(declaration.Keys))
		for part, names := range [][]string{declaration.Keys, declaration.Include} {
			for _, name := range names {
				if name == "" {
					return "", "create_table.key_members"
				}
				if PGSystemColumnName(name) {
					system = true
				} else if !slices.ContainsFunc(ddl.Columns, func(c Column) bool { return c.Name == name }) {
					return "create_keys_column_absent", ""
				}
				if part == 0 {
					if seen[name] {
						return "create_keys_column_duplicate", ""
					}
					seen[name] = true
				}
			}
		}
	}
	if system {
		return "create_keys_system_column", ""
	}
	return "", ""
}
