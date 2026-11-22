package spec

import "strings"

// RepeatedAlterAddColumn returns the first repeated name in an ADD-only ALTER list.
func RepeatedAlterAddColumn(ddl *DDL) string {
	if ddl == nil || ddl.Operation != DDLOperationAlterTable || len(ddl.Alter) < 2 {
		return ""
	}
	seen := make(map[string]bool, len(ddl.Alter))
	var repeated string
	for _, alter := range ddl.Alter {
		if alter.Action != "add_columns" || alter.Name == "" {
			return ""
		}
		key := strings.ToLower(alter.Name)
		if repeated == "" && seen[key] {
			repeated = alter.Name
		}
		seen[key] = true
	}
	return repeated
}
