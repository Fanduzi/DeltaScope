// Package catalog defines explanation-oriented metadata for shipped audit rules.
// input: built-in policy defaults, rule IDs, and catalog template heuristics for shipped rules
// output: stable rule catalog entries for CLI discovery, search, and rule-detail rendering
// pos: explanation-oriented rule metadata layer above execution-only rule registration
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"fmt"
	"sort"
	"strings"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Entry describes one shipped rule in the CLI-facing catalog.
type Entry struct {
	RuleID          string
	Summary         string
	Description     string
	StatementKinds  []string
	DefaultEnabled  bool
	DefaultLevel    rule.Level
	DefaultParams   map[string]any
	MetadataAware   bool
	TriggerExample  string
	ValidExample    string
	ConfigExample   string
	RemediationHint string
	SearchText      string
}

var entries = buildEntries()

// All returns all shipped catalog entries in stable order.
func All() []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}

// Lookup returns the catalog entry for one rule ID.
func Lookup(ruleID string) (Entry, bool) {
	for _, entry := range entries {
		if entry.RuleID == ruleID {
			return entry, true
		}
	}
	return Entry{}, false
}

// Search returns entries whose IDs, summaries, descriptions, or kinds match the query.
func Search(query string) []Entry {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return All()
	}

	results := make([]Entry, 0)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.SearchText), needle) {
			results = append(results, entry)
		}
	}
	return results
}

func buildEntries() []Entry {
	defaults := domainpolicy.Default()
	ids := make([]string, 0, len(defaults.Rules))
	for ruleID := range defaults.Rules {
		ids = append(ids, ruleID)
	}
	sort.Strings(ids)

	items := make([]Entry, 0, len(ids))
	for _, ruleID := range ids {
		policy := defaults.Rules[ruleID]
		entry := Entry{
			RuleID:          ruleID,
			Summary:         summaryForRule(ruleID),
			Description:     descriptionForRule(ruleID, policy),
			StatementKinds:  statementKindsForRule(ruleID),
			DefaultEnabled:  policy.Enabled,
			DefaultLevel:    policy.Level,
			DefaultParams:   cloneParams(policy.Params),
			MetadataAware:   metadataAwareRuleIDs[ruleID],
			TriggerExample:  triggerExampleForRule(ruleID),
			ValidExample:    validExampleForRule(ruleID),
			ConfigExample:   configExampleForRule(ruleID, policy),
			RemediationHint: remediationForRule(ruleID),
		}
		entry.SearchText = strings.Join([]string{
			entry.RuleID,
			entry.Summary,
			entry.Description,
			strings.Join(entry.StatementKinds, " "),
		}, " ")
		items = append(items, entry)
	}
	return items
}

func cloneParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func summaryForRule(ruleID string) string {
	switch {
	case strings.HasSuffix(ruleID, ".require"):
		return humanize(ruleID, "Require")
	case strings.HasSuffix(ruleID, ".forbid"):
		return humanize(ruleID, "Forbid")
	case strings.Contains(ruleID, ".allowlist"):
		return humanize(ruleID, "Restrict")
	case strings.Contains(ruleID, ".max_length"):
		return humanize(ruleID, "Limit")
	case strings.Contains(ruleID, ".max_count"):
		return humanize(ruleID, "Limit")
	case strings.Contains(ruleID, ".max_bytes"):
		return humanize(ruleID, "Limit")
	case strings.HasSuffix(ruleID, ".warn"):
		return humanize(ruleID, "Warn on")
	default:
		return humanize(ruleID, "Check")
	}
}

func descriptionForRule(ruleID string, policy domainpolicy.RulePolicy) string {
	base := summaryForRule(ruleID)
	scope := strings.Join(statementKindsForRule(ruleID), ", ")
	metadata := "offline-safe"
	if metadataAwareRuleIDs[ruleID] {
		metadata = "metadata-aware"
	}
	return fmt.Sprintf("%s. Default level is %s, enabled=%t, scope=%s, and the shipped policy treats it as a %s rule.", base, policy.Level, policy.Enabled, scope, metadata)
}

func statementKindsForRule(ruleID string) []string {
	switch {
	case strings.HasPrefix(ruleID, "ddl."):
		if strings.Contains(ruleID, ".merge.") {
			return []string{"ddl", "batch"}
		}
		return []string{"ddl"}
	case strings.HasPrefix(ruleID, "dml."):
		return []string{"dml"}
	default:
		return []string{"unknown"}
	}
}

func triggerExampleForRule(ruleID string) string {
	switch {
	case ruleID == "dml.where.require":
		return "DELETE FROM users;"
	case strings.HasPrefix(ruleID, "dml."):
		return "UPDATE users SET status = 'disabled';"
	case strings.Contains(ruleID, ".drop."):
		return "ALTER TABLE users DROP COLUMN email;"
	case strings.Contains(ruleID, ".rename."):
		return "ALTER TABLE users RENAME COLUMN nickname TO display_name;"
	default:
		return "CREATE TABLE users (id bigint);"
	}
}

func validExampleForRule(ruleID string) string {
	switch {
	case ruleID == "dml.where.require":
		return "DELETE FROM users WHERE id = 1;"
	case strings.HasPrefix(ruleID, "dml."):
		return "UPDATE users SET status = 'disabled' WHERE id = 1;"
	case strings.Contains(ruleID, "comment.require"):
		return "CREATE TABLE users (id bigint PRIMARY KEY) COMMENT='application users';"
	case strings.Contains(ruleID, "primary_key.require"):
		return "CREATE TABLE users (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id));"
	case strings.Contains(ruleID, ".drop."):
		return "ALTER TABLE users MODIFY COLUMN email varchar(255) NOT NULL;"
	default:
		return "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"
	}
}

func configExampleForRule(ruleID string, policy domainpolicy.RulePolicy) string {
	lines := []string{
		"rules:",
		fmt.Sprintf("  %s:", ruleID),
		fmt.Sprintf("    enabled: %t", policy.Enabled),
		fmt.Sprintf("    level: %s", policy.Level),
	}
	if len(policy.Params) == 0 {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "    params:")
	keys := make([]string, 0, len(policy.Params))
	for key := range policy.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("      %s: %s", key, formatYAMLScalar(policy.Params[key])))
	}
	return strings.Join(lines, "\n")
}

func remediationForRule(ruleID string) string {
	switch {
	case strings.HasSuffix(ruleID, ".require"):
		return "Add the required clause, option, or object explicitly so the rule no longer has to infer intent."
	case strings.HasSuffix(ruleID, ".forbid"):
		return "Rewrite the statement to avoid the forbidden construct, or relax the policy if the team accepts the risk."
	case strings.Contains(ruleID, ".allowlist"):
		return "Use one of the shipped allowlisted values or update the policy allowlist deliberately."
	case strings.Contains(ruleID, ".max_"):
		return "Reduce the size/count below the configured limit or raise the threshold explicitly in policy."
	case strings.HasSuffix(ruleID, ".warn"):
		return "Review the metadata-backed caution and proceed only if the operational trade-off is acceptable."
	default:
		return "Review the default params and align the statement or policy with the intended governance rule."
	}
}

func humanize(ruleID string, verb string) string {
	replacer := strings.NewReplacer(".", " ", "_", " ")
	text := replacer.Replace(ruleID)
	text = strings.ReplaceAll(text, "ddl ", "DDL ")
	text = strings.ReplaceAll(text, "dml ", "DML ")
	return strings.TrimSpace(fmt.Sprintf("%s %s", verb, text))
}

func formatYAMLScalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return fmt.Sprintf("[%s]", strings.Join(typed, ", "))
	default:
		return fmt.Sprintf("%v", typed)
	}
}

var metadataAwareRuleIDs = map[string]bool{
	"ddl.alter.change_column.compatibility.require": true,
	"ddl.alter.change_column.exists.require":        true,
	"ddl.alter.drop_column.exists.require":          true,
	"ddl.alter.drop_index.exists.require":           true,
	"ddl.alter.drop_primary_key.exists.require":     true,
	"ddl.alter.modify_column.compatibility.require": true,
	"ddl.alter.modify_column.exists.require":        true,
	"ddl.alter.rename_column.exists.require":        true,
	"ddl.alter.rename_index.exists.require":         true,
	"ddl.table.drop.adaptive_hash.warn":             true,
	"ddl.table.drop.exists.require":                 true,
	"ddl.table.drop.rows.max_count":                 true,
	"ddl.table.exists.alter.require":                true,
	"ddl.table.exists.create.forbid":                true,
	"ddl.table.row_size.max_bytes.require":          true,
	"ddl.table.truncate.adaptive_hash.warn":         true,
	"ddl.table.truncate.exists.require":             true,
	"ddl.table.truncate.rows.max_count":             true,
	"dml.table.denylist.forbid":                     true,
	"ddl.table.denylist.forbid":                     true,
	"ddl.alter.add_column.exists.forbid":            true,
	"ddl.alter.add_index.exists.forbid":             true,
	"ddl.index.key_length.max_bytes.require":        true,
}
