// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with typed index metadata and per-rule policy values
// output: index-governance findings for count, naming, and duplicate-index concerns
// pos: DDL rule implementations for create-table index governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type indexTotalMaxCountRule struct {
	limit int
	level rule.Level
}

func newIndexTotalMaxCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDIndexTotalMaxCount, cfg, "limit", 12)
	if err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("rule %s param %q must be >= 0, got %d", ruleIDIndexTotalMaxCount, "limit", limit)
	}
	return indexTotalMaxCountRule{limit: limit, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r indexTotalMaxCountRule) ID() string { return ruleIDIndexTotalMaxCount }

func (r indexTotalMaxCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableIndexes(statement)
}

func (r indexTotalMaxCountRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	actual := len(statement.DDL.Indexes)
	if actual <= r.limit {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table must not define more than %d non-primary indexes", r.limit),
		Suggestion: "remove redundant indexes or relax the policy limit intentionally",
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"limit":  r.limit,
			"actual": actual,
		},
	}}, nil
}

type indexColumnsMaxCountRule struct {
	limit int
	level rule.Level
}

func newIndexColumnsMaxCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDIndexColumnsMaxCount, cfg, "limit", 8)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDIndexColumnsMaxCount, "limit", limit)
	}
	return indexColumnsMaxCountRule{limit: limit, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r indexColumnsMaxCountRule) ID() string { return ruleIDIndexColumnsMaxCount }

func (r indexColumnsMaxCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableIndexes(statement)
}

func (r indexColumnsMaxCountRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		actual := len(index.Columns)
		if actual <= r.limit {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s index %q must not contain more than %d columns", indexKindLabel(index.Kind), index.Name, r.limit),
			Suggestion: fmt.Sprintf("reduce the indexed column count to %d or fewer", r.limit),
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"index":  index.Name,
				"kind":   index.Kind,
				"limit":  r.limit,
				"actual": actual,
			},
		})
	}
	return findings, nil
}

type indexPrefixRequiredRule struct {
	ruleID string
	kind   spec.IndexKind
	prefix string
	level  rule.Level
}

type indexSuffixRequiredRule struct {
	ruleID string
	kind   spec.IndexKind
	suffix string
	level  rule.Level
}

type indexContainsRequiredRule struct {
	ruleID   string
	kind     spec.IndexKind
	contains []string
	level    rule.Level
}

func newIndexPrefixRequiredRule(ruleID string, kind spec.IndexKind, fallbackPrefix string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	if !required {
		return indexPrefixRequiredRule{ruleID: ruleID, kind: kind}, nil
	}
	prefix, err := stringParam(ruleID, cfg, "prefix", fallbackPrefix)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(prefix) == "" {
		return nil, fmt.Errorf("rule %s param %q must not be empty", ruleID, "prefix")
	}
	return indexPrefixRequiredRule{
		ruleID: ruleID,
		kind:   kind,
		prefix: strings.ToLower(prefix),
		level:  configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r indexPrefixRequiredRule) ID() string { return r.ruleID }

func (r indexPrefixRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.prefix != "" && appliesToCreateTableIndexes(statement)
}

func (r indexPrefixRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != r.kind {
			continue
		}
		if strings.HasPrefix(strings.ToLower(index.Name), r.prefix) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s index %q must use prefix %q", indexKindLabel(index.Kind), index.Name, r.prefix),
			Suggestion: fmt.Sprintf("rename the index to start with %q", r.prefix),
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"index":  index.Name,
				"kind":   index.Kind,
				"prefix": r.prefix,
			},
		})
	}
	return findings, nil
}

func newIndexSuffixRequiredRule(ruleID string, kind spec.IndexKind, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if requirement.suffix == "" {
		return indexSuffixRequiredRule{ruleID: ruleID, kind: kind}, nil
	}
	return indexSuffixRequiredRule{
		ruleID: ruleID,
		kind:   kind,
		suffix: requirement.suffix,
		level:  configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r indexSuffixRequiredRule) ID() string { return r.ruleID }

func (r indexSuffixRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.suffix != "" && appliesToCreateTableIndexes(statement)
}

func (r indexSuffixRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != r.kind || strings.TrimSpace(index.Name) == "" {
			continue
		}
		if strings.HasSuffix(index.Name, r.suffix) {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("%s index %q must use suffix %q", indexKindLabel(index.Kind), index.Name, r.suffix),
			Suggestion: fmt.Sprintf("rename the index to end with %q", r.suffix),
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"index":  index.Name,
				"kind":   index.Kind,
				"suffix": r.suffix,
			},
		})
	}
	return findings, nil
}

func newIndexContainsRequiredRule(ruleID string, kind spec.IndexKind, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if len(requirement.contains) == 0 {
		return indexContainsRequiredRule{ruleID: ruleID, kind: kind}, nil
	}
	return indexContainsRequiredRule{
		ruleID:   ruleID,
		kind:     kind,
		contains: append([]string(nil), requirement.contains...),
		level:    configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r indexContainsRequiredRule) ID() string { return r.ruleID }

func (r indexContainsRequiredRule) AppliesTo(statement spec.Statement) bool {
	return len(r.contains) > 0 && appliesToCreateTableIndexes(statement)
}

func (r indexContainsRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != r.kind || strings.TrimSpace(index.Name) == "" {
			continue
		}
		matched := false
		for _, item := range r.contains {
			if strings.Contains(index.Name, item) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("%s index %q must contain one of: %s", indexKindLabel(index.Kind), index.Name, strings.Join(r.contains, ", ")),
			Suggestion: fmt.Sprintf("rename the index to include one of: %s", strings.Join(r.contains, ", ")),
			Metadata: map[string]any{
				"table":    statement.DDL.Table.Name,
				"index":    index.Name,
				"kind":     index.Kind,
				"contains": append([]string(nil), r.contains...),
			},
		})
	}
	return findings, nil
}

type duplicateIndexForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newDuplicateIndexForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDIndexDuplicateForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return duplicateIndexForbiddenRule{forbid: forbid, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r duplicateIndexForbiddenRule) ID() string { return ruleIDIndexDuplicateForbid }

func (r duplicateIndexForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTableIndexes(statement)
}

func (r duplicateIndexForbiddenRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	seen := make(map[string]string)
	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		signature := duplicateSignature(index)
		if first, ok := seen[signature]; ok {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("%s index %q duplicates index %q", indexKindLabel(index.Kind), index.Name, first),
				Suggestion: "keep one exact index definition and remove the duplicate",
				Metadata: map[string]any{
					"table":     statement.DDL.Table.Name,
					"index":     index.Name,
					"duplicate": first,
					"kind":      index.Kind,
					"columns":   append([]string(nil), index.Columns...),
					"signature": signature,
				},
			})
			continue
		}
		seen[signature] = index.Name
	}
	return findings, nil
}

func duplicateSignature(index spec.Index) string {
	return fmt.Sprintf("%s:%s", index.Kind, strings.Join(index.Columns, ","))
}

type redundantLeftPrefixIndexRule struct {
	forbid bool
	level  rule.Level
}

func newRedundantLeftPrefixIndexRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDIndexRedundantLeftPrefixForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return redundantLeftPrefixIndexRule{forbid: forbid, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r redundantLeftPrefixIndexRule) ID() string { return ruleIDIndexRedundantLeftPrefixForbid }

func (r redundantLeftPrefixIndexRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTableIndexes(statement)
}

func (r redundantLeftPrefixIndexRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for i := range statement.DDL.Indexes {
		shorter := statement.DDL.Indexes[i]
		if shorter.Kind != spec.IndexKindSecondary || len(shorter.Columns) == 0 {
			continue
		}
		for j := range statement.DDL.Indexes {
			if i == j {
				continue
			}
			longer := statement.DDL.Indexes[j]
			if longer.Kind != spec.IndexKindSecondary || len(longer.Columns) <= len(shorter.Columns) {
				continue
			}
			if !isLeftPrefix(shorter.Columns, longer.Columns) {
				continue
			}
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("secondary index %q is redundant because %q already covers its left-prefix columns", shorter.Name, longer.Name),
				Suggestion: "drop the shorter left-prefix index or keep it only with a documented justification",
				Metadata: map[string]any{
					"table":      statement.DDL.Table.Name,
					"index":      shorter.Name,
					"redundant":  longer.Name,
					"columns":    append([]string(nil), shorter.Columns...),
					"covering":   append([]string(nil), longer.Columns...),
					"heuristic":  "left_prefix",
					"index_kind": shorter.Kind,
				},
			})
			break
		}
	}
	return findings, nil
}

type redundantUniqueOverlapIndexRule struct {
	forbid bool
	level  rule.Level
}

func newRedundantUniqueOverlapIndexRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDIndexRedundantUniqueOverlapForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return redundantUniqueOverlapIndexRule{forbid: forbid, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r redundantUniqueOverlapIndexRule) ID() string { return ruleIDIndexRedundantUniqueOverlapForbid }

func (r redundantUniqueOverlapIndexRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTableIndexes(statement)
}

func (r redundantUniqueOverlapIndexRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	uniqueBySignature := make(map[string]string)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != spec.IndexKindUnique {
			continue
		}
		uniqueBySignature[strings.Join(index.Columns, ",")] = index.Name
	}

	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != spec.IndexKindSecondary {
			continue
		}
		if uniqueName, ok := uniqueBySignature[strings.Join(index.Columns, ",")]; ok {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("secondary index %q is redundant because unique index %q uses the same columns", index.Name, uniqueName),
				Suggestion: "drop the secondary index or keep it only with a documented justification",
				Metadata: map[string]any{
					"table":      statement.DDL.Table.Name,
					"index":      index.Name,
					"redundant":  uniqueName,
					"columns":    append([]string(nil), index.Columns...),
					"heuristic":  "unique_overlap",
					"index_kind": index.Kind,
				},
			})
		}
	}
	return findings, nil
}

func isLeftPrefix(shorter, longer []string) bool {
	if len(shorter) == 0 || len(shorter) >= len(longer) {
		return false
	}
	for idx := range shorter {
		if !strings.EqualFold(shorter[idx], longer[idx]) {
			return false
		}
	}
	return true
}
