// Package report defines audit results, summaries, and verdict aggregation.
// This file adds a derived action summary that groups findings by rule for human reports.
// input: report.Result findings and rule catalog entries
// output: derived action summary items ordered by remediation priority
// pos: derived human-report helper; does not change Result JSON and does not run the audit
// note: if this file changes, update this header and module README.md.
package report

import (
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

// ActionSummaryOptions controls action summary derivation.
// A Limit of zero or less means the core performs no truncation.
type ActionSummaryOptions struct {
	Limit int
}

// ActionSummary is the derived, human-oriented grouping of audit findings by rule.
// It is derived from report.Result and does not change the Result JSON shape.
// It uses rule.Level (blocker, warning, notice) and never introduces a severity field.
type ActionSummary struct {
	Items      []ActionItem
	TotalItems int
}

// ActionItem describes one rule group in the action summary.
// It carries rule-level priority and counts only; it does not hold raw SQL or
// finding metadata, so it is safe to render in human reports.
type ActionItem struct {
	RuleID            string
	Level             rule.Level
	Count             int
	Summary           string
	Suggestion        string
	StatementIndexes  []int
	HasGlobalFindings bool
	ExplainCommand    string
}

// actionGroup accumulates findings for a single rule ID during derivation.
// It is internal and is never exposed on report.Result or in JSON output.
type actionGroup struct {
	level              rule.Level
	bestRank           int
	count              int
	statementIndexSet  map[int]struct{}
	hasGlobal          bool
	firstMessage       string
	hasMessage         bool
	fallbackSuggestion string
	hasFallbackSuggest bool
}

// BuildActionSummary derives an action summary from a report result and rule catalog entries.
//
// It groups statement findings and global findings by rule ID, derives each group's
// highest-priority level and total count, prefers catalog summary/suggestion text and
// falls back to finding text when a rule is absent from the catalog, and orders groups
// by remediation priority (blocker, warning, notice; then count descending; then rule ID ascending).
//
// BuildActionSummary is a pure derivation: it does not parse SQL, run the audit, inspect
// raw SQL, or mutate result or entries. Statement indexes are 1-based positions into
// result.Statements and are deduplicated within a group. An empty result returns a
// non-nil Items slice and TotalItems 0.
func BuildActionSummary(result Result, entries []catalog.Entry, options ActionSummaryOptions) ActionSummary {
	entryByID := make(map[string]catalog.Entry, len(entries))
	for _, entry := range entries {
		entryByID[entry.RuleID] = entry
	}

	groups := make(map[string]*actionGroup)
	order := make([]string, 0)

	// Statement findings use 1-based slice positions. Iterating in slice order keeps
	// first-finding fallback text and statement index discovery deterministic.
	for i := range result.Statements {
		statementIndex := i + 1
		for _, finding := range result.Statements[i].Findings {
			group := upsertActionGroup(groups, &order, finding.RuleID)
			group.recordFinding(finding)
			group.recordStatementIndex(statementIndex)
		}
	}

	// Global findings contribute to counts and levels and set HasGlobalFindings,
	// but they carry no statement index.
	for _, finding := range result.GlobalFindings {
		group := upsertActionGroup(groups, &order, finding.RuleID)
		group.recordFinding(finding)
		group.hasGlobal = true
	}

	items := make([]ActionItem, 0, len(order))
	for _, ruleID := range order {
		items = append(items, groups[ruleID].toActionItem(ruleID, entryByID))
	}

	sortActionItems(items)

	total := len(items)
	if options.Limit > 0 && total > options.Limit {
		items = items[:options.Limit]
	}

	if items == nil {
		items = []ActionItem{}
	}

	return ActionSummary{
		Items:      items,
		TotalItems: total,
	}
}

// upsertActionGroup returns the group for ruleID, creating and registering it on first use.
func upsertActionGroup(groups map[string]*actionGroup, order *[]string, ruleID string) *actionGroup {
	if group, ok := groups[ruleID]; ok {
		return group
	}
	group := &actionGroup{
		bestRank:          levelRankUnknown + 1,
		statementIndexSet: make(map[int]struct{}),
	}
	groups[ruleID] = group
	*order = append(*order, ruleID)
	return group
}

// recordFinding folds one finding into the group: bump the count, promote the level
// when a higher-priority level appears, and capture fallback message/suggestion text.
func (g *actionGroup) recordFinding(finding rule.Finding) {
	g.count++
	if rank := levelRank(finding.Level); rank < g.bestRank {
		g.bestRank = rank
		g.level = finding.Level
	}
	if !g.hasMessage {
		g.firstMessage = finding.Message
		g.hasMessage = true
	}
	if !g.hasFallbackSuggest && strings.TrimSpace(finding.Suggestion) != "" {
		g.fallbackSuggestion = finding.Suggestion
		g.hasFallbackSuggest = true
	}
}

// recordStatementIndex records a 1-based statement index, ignoring duplicates.
func (g *actionGroup) recordStatementIndex(index int) {
	g.statementIndexSet[index] = struct{}{}
}

// toActionItem freezes the accumulated group into an immutable ActionItem.
func (g *actionGroup) toActionItem(ruleID string, entryByID map[string]catalog.Entry) ActionItem {
	summary := g.firstMessage
	suggestion := g.fallbackSuggestion
	if entry, ok := entryByID[ruleID]; ok {
		if entry.Summary != "" {
			summary = entry.Summary
		}
		if entry.Suggestion != "" {
			suggestion = entry.Suggestion
		}
	}

	return ActionItem{
		RuleID:            ruleID,
		Level:             g.level,
		Count:             g.count,
		Summary:           summary,
		Suggestion:        suggestion,
		StatementIndexes:  sortedStatementIndexes(g.statementIndexSet),
		HasGlobalFindings: g.hasGlobal,
		ExplainCommand:    "deltascope rules explain " + ruleID,
	}
}

// sortedStatementIndexes returns the deduplicated statement indexes in ascending order.
// It returns nil for groups with no statement findings so global-only groups omit indexes.
func sortedStatementIndexes(set map[int]struct{}) []int {
	if len(set) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(set))
	for index := range set {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	return indexes
}

// sortActionItems orders items by level priority, then count descending, then rule ID ascending.
// The comparator is a strict total order because rule IDs are unique, so it is deterministic.
func sortActionItems(items []ActionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		ra, rb := levelRank(a.Level), levelRank(b.Level)
		if ra != rb {
			return ra < rb
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.RuleID < b.RuleID
	})
}

// levelRank maps a rule level to a priority rank where lower means more urgent.
// Unknown levels sort after notice so real blocker/warning/notice levels always win.
const (
	levelRankBlocker = 0
	levelRankWarning = 1
	levelRankNotice  = 2
	levelRankUnknown = 3
)

func levelRank(level rule.Level) int {
	switch level {
	case rule.LevelBlocker:
		return levelRankBlocker
	case rule.LevelWarning:
		return levelRankWarning
	case rule.LevelNotice:
		return levelRankNotice
	default:
		return levelRankUnknown
	}
}
