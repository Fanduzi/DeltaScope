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
