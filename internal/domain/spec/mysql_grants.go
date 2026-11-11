package spec

import (
	"regexp"
	"strings"
)

// mysqlGlobalGrantPattern matches a privilege-scoped SHOW GRANTS line —
// "GRANT <privileges> ON <scope> TO <grantee> ...". Role-assignment
// lines (GRANT 'r'@'%' TO ...) carry no ON clause and never match.
var mysqlGlobalGrantPattern = regexp.MustCompile(`(?i)^GRANT\s+(.+?)\s+ON\s+(\S+)\s+TO\s+`)

// CollectMySQLGlobalPrivileges folds every global-scope (*.*) grant
// line into the set. Privileges parse as bounded, exact tokens — never
// substring matches, so RESOURCE_GROUP_USER never counts as
// RESOURCE_GROUP_ADMIN. This is the shared low-level mechanism used by
// the T24 effective-privilege bundle and the T26 DROP SERVER privilege
// observation; behavior is identical for both consumers.
func CollectMySQLGlobalPrivileges(set map[string]bool, lines []string) {
	for _, line := range lines {
		match := mysqlGlobalGrantPattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || match[2] != "*.*" {
			continue
		}
		for _, privilege := range strings.Split(match[1], ",") {
			privilege = strings.ToUpper(strings.TrimSpace(privilege))
			if privilege != "" {
				set[privilege] = true
			}
		}
	}
}
