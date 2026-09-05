// Package policy defines audit policy configuration in domain terms.
// input: shipped rule identifiers and Default Policy values
// output: FK-forbid suppression facts used by registration and config status
// pos: domain helper that names Default Policy rules which are not Loaded
// note: if this file changes, update this header and module README.md.
package policy

// ForeignKeyForbidRuleID is the Default Policy rule that suppresses foreign-key
// naming governance while foreign keys themselves are forbidden.
const ForeignKeyForbidRuleID = "ddl.table.foreign_key.forbid"

// ForeignKeyNamingSuppressionReason is the config-status reason for FK naming
// rules that stay in Default Policy but are not Loaded.
const ForeignKeyNamingSuppressionReason = "fk_forbid"

// ForeignKeyNamingRuleIDs are Default Policy naming rules that registration
// does not Load while ForeignKeyForbidRuleID is enabled. They stay in the
// Rule Catalog and Default Policy; they are suppressed, not skipped.
var ForeignKeyNamingRuleIDs = []string{
	"ddl.constraint.foreign_key.name.prefix.require",
	"ddl.constraint.foreign_key.name.suffix.require",
	"ddl.constraint.foreign_key.name.contains.require",
}

// SuppressesForeignKeyNaming reports whether ruleID is a foreign-key naming
// rule that Default Policy (or the supplied policy) will not Load because
// foreign keys are forbidden.
func SuppressesForeignKeyNaming(cfg Policy, ruleID string) bool {
	if !isForeignKeyNamingRule(ruleID) {
		return false
	}
	forbidCfg, ok := cfg.Rules[ForeignKeyForbidRuleID]
	if !ok || !forbidCfg.Enabled {
		return false
	}
	value, exists := forbidCfg.Params["forbid"]
	if !exists {
		return true
	}
	forbid, ok := value.(bool)
	if !ok {
		return false
	}
	return forbid
}

func isForeignKeyNamingRule(ruleID string) bool {
	for _, id := range ForeignKeyNamingRuleIDs {
		if id == ruleID {
			return true
		}
	}
	return false
}
