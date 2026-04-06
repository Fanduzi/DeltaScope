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

type namingRule struct {
	ruleID     string
	subject    string
	level      rule.Level
	selects    func(spec.Statement) []identifierSubject
	matchKind  string
	prefix     string
	suffix     string
	contains   []string
	suggestion string
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

func newNamingPrefixRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if requirement.prefix == "" {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:     ruleID,
		subject:    subject,
		level:      configuredLevel(cfg, fallbackLevel),
		selects:    selects,
		matchKind:  "prefix",
		prefix:     requirement.prefix,
		suggestion: fmt.Sprintf("rename the %s to start with %q", subject, requirement.prefix),
	}, nil
}

func newNamingSuffixRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if requirement.suffix == "" {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:     ruleID,
		subject:    subject,
		level:      configuredLevel(cfg, fallbackLevel),
		selects:    selects,
		matchKind:  "suffix",
		suffix:     requirement.suffix,
		suggestion: fmt.Sprintf("rename the %s to end with %q", subject, requirement.suffix),
	}, nil
}

func newNamingContainsRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if len(requirement.contains) == 0 {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:     ruleID,
		subject:    subject,
		level:      configuredLevel(cfg, fallbackLevel),
		selects:    selects,
		matchKind:  "contains",
		contains:   append([]string(nil), requirement.contains...),
		suggestion: fmt.Sprintf("rename the %s to include one of: %s", subject, strings.Join(requirement.contains, ", ")),
	}, nil
}

func (r identifierPatternRule) ID() string { return r.ruleID }

func (r namingRule) ID() string { return r.ruleID }

func (r identifierPatternRule) AppliesTo(statement spec.Statement) bool {
	return r.pattern != nil && appliesToCreateTable(statement)
}

func (r namingRule) AppliesTo(statement spec.Statement) bool {
	if !appliesToCreateTable(statement) {
		return false
	}
	switch r.matchKind {
	case "prefix":
		return r.prefix != ""
	case "suffix":
		return r.suffix != ""
	case "contains":
		return len(r.contains) > 0
	default:
		return false
	}
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

func (r namingRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if strings.TrimSpace(subject.name) == "" || r.matches(subject.name) {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    r.message(subject.name),
			Suggestion: r.suggestion,
			Metadata:   r.metadata(statement, subject),
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
		if !isReservedKeyword(r.ruleID, statement.Dialect, subject.name) {
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

func selectPrimaryKeyConstraintNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil || statement.DDL.PrimaryKey == nil {
		return nil
	}
	if strings.TrimSpace(statement.DDL.PrimaryKey.Name) == "" || strings.EqualFold(strings.TrimSpace(statement.DDL.PrimaryKey.Name), "primary") {
		return nil
	}
	return []identifierSubject{{kind: "constraint.primary_key", name: statement.DDL.PrimaryKey.Name}}
}

func selectUniqueConstraintNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0)
	for _, index := range statement.DDL.Indexes {
		if index.Kind != spec.IndexKindUnique || strings.TrimSpace(index.Name) == "" {
			continue
		}
		subjects = append(subjects, identifierSubject{kind: "constraint.unique_key", name: index.Name})
	}
	return subjects
}

func selectForeignKeyConstraintNames(statement spec.Statement) []identifierSubject {
	return selectConstraintNamesByType(statement, "foreign_key", "constraint.foreign_key")
}

func selectCheckConstraintNames(statement spec.Statement) []identifierSubject {
	return selectConstraintNamesByType(statement, "check", "constraint.check")
}

func selectConstraintNamesByType(statement spec.Statement, constraintType, kind string) []identifierSubject {
	if statement.DDL == nil || len(statement.DDL.Constraints) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0)
	for _, constraint := range statement.DDL.Constraints {
		if constraint.Type != constraintType || strings.TrimSpace(constraint.Name) == "" {
			continue
		}
		subjects = append(subjects, identifierSubject{kind: kind, name: constraint.Name})
	}
	return subjects
}

func (r namingRule) matches(name string) bool {
	switch r.matchKind {
	case "prefix":
		return strings.HasPrefix(name, r.prefix)
	case "suffix":
		return strings.HasSuffix(name, r.suffix)
	case "contains":
		for _, item := range r.contains {
			if strings.Contains(name, item) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func (r namingRule) message(name string) string {
	switch r.matchKind {
	case "prefix":
		return fmt.Sprintf("%s name %q must start with %q", r.subject, name, r.prefix)
	case "suffix":
		return fmt.Sprintf("%s name %q must end with %q", r.subject, name, r.suffix)
	case "contains":
		return fmt.Sprintf("%s name %q must contain one of: %s", r.subject, name, strings.Join(r.contains, ", "))
	default:
		return ""
	}
}

func (r namingRule) metadata(statement spec.Statement, subject identifierSubject) map[string]any {
	metadata := map[string]any{
		"table":   statement.DDL.Table.Name,
		"subject": subject.kind,
		"name":    subject.name,
	}
	switch r.matchKind {
	case "prefix":
		metadata["prefix"] = r.prefix
	case "suffix":
		metadata["suffix"] = r.suffix
	case "contains":
		metadata["contains"] = append([]string(nil), r.contains...)
	}
	return metadata
}

func isReservedKeyword(ruleID string, dialect spec.Dialect, name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(strings.Trim(name, "`\"")))
	if normalized == "" {
		return false
	}
	if _, ok := reservedKeywords[normalized]; ok {
		return true
	}
	if dialect != spec.DialectPostgreSQL {
		return false
	}
	if ruleID != ruleIDTableNameKeywordForbid && ruleID != ruleIDColumnNameKeywordForbid && ruleID != ruleIDIndexNameKeywordForbid {
		return false
	}
	_, ok := postgreSQLReservedKeywords[normalized]
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
