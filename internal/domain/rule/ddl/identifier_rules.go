// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs carrying table, column, and index names plus per-rule policy values
// output: identifier-pattern and reserved-keyword findings for create-table object names
// pos: DDL rule implementations for create-table identifier governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/pingcap/tidb/pkg/parser"
)

var (
	defaultIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	reservedKeywords         = buildReservedKeywordSet()
)

type identifierSubject struct {
	kind string
	name string
}

type identifierPatternRule struct {
	ruleID  string
	subject string
	level   rule.Level
	pattern *regexp.Regexp
	selects func(spec.Statement) []identifierSubject
}

func newIdentifierPatternRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	if !required {
		return identifierPatternRule{ruleID: ruleID}, nil
	}
	patternText, err := stringParam(ruleID, cfg, "pattern", defaultIdentifierPattern.String())
	if err != nil {
		return nil, err
	}
	pattern, err := regexp.Compile(patternText)
	if err != nil {
		return nil, fmt.Errorf("rule %s param %q must be a valid regexp: %w", ruleID, "pattern", err)
	}
	return identifierPatternRule{
		ruleID:  ruleID,
		subject: subject,
		level:   configuredLevel(cfg, fallbackLevel),
		pattern: pattern,
		selects: selects,
	}, nil
}

func (r identifierPatternRule) ID() string { return r.ruleID }

func (r identifierPatternRule) AppliesTo(statement spec.Statement) bool {
	return r.pattern != nil && appliesToCreateTable(statement)
}

func (r identifierPatternRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if r.pattern.MatchString(subject.name) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s name %q must match %q", r.subject, subject.name, r.pattern.String()),
			Suggestion: fmt.Sprintf("rename the %s using only letters, digits, and underscores", r.subject),
			Metadata: map[string]any{
				"table":   statement.DDL.Table.Name,
				"subject": subject.kind,
				"name":    subject.name,
				"pattern": r.pattern.String(),
			},
		})
	}
	return findings, nil
}

type identifierKeywordRule struct {
	ruleID  string
	subject string
	level   rule.Level
	forbid  bool
	selects func(spec.Statement) []identifierSubject
}

func newIdentifierKeywordRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return identifierKeywordRule{
		ruleID:  ruleID,
		subject: subject,
		level:   configuredLevel(cfg, fallbackLevel),
		forbid:  forbid,
		selects: selects,
	}, nil
}

func (r identifierKeywordRule) ID() string { return r.ruleID }

func (r identifierKeywordRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTable(statement)
}

func (r identifierKeywordRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if !isReservedKeyword(subject.name) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s name %q must not use a reserved keyword", r.subject, subject.name),
			Suggestion: fmt.Sprintf("rename the %s to a non-keyword identifier", r.subject),
			Metadata: map[string]any{
				"table":   statement.DDL.Table.Name,
				"subject": subject.kind,
				"name":    subject.name,
			},
		})
	}
	return findings, nil
}

func selectTableName(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil || statement.DDL.Table == nil {
		return nil
	}
	return []identifierSubject{{kind: "table", name: statement.DDL.Table.Name}}
}

func selectColumnNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil || len(statement.DDL.Columns) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0, len(statement.DDL.Columns))
	for _, column := range statement.DDL.Columns {
		subjects = append(subjects, identifierSubject{kind: "column", name: column.Name})
	}
	return subjects
}

func selectIndexNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0, len(statement.DDL.Indexes))
	for _, index := range statement.DDL.Indexes {
		subjects = append(subjects, identifierSubject{kind: "index", name: index.Name})
	}
	return subjects
}

func isReservedKeyword(name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(strings.Trim(name, "`")))
	if normalized == "" {
		return false
	}
	_, ok := reservedKeywords[normalized]
	return ok
}

func buildReservedKeywordSet() map[string]struct{} {
	keywords := make(map[string]struct{}, len(tidbparser.Keywords))
	for _, keyword := range tidbparser.Keywords {
		if !keyword.Reserved {
			continue
		}
		keywords[keyword.Word] = struct{}{}
	}
	return keywords
}
