// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
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
	ruleID      string
	subject     string
	level       rule.Level
	selects     func(spec.Statement) []identifierSubject
	matchKind   string
	prefix      string
	suffix      string
	contains    []string
	suggestion  string
	appliesToFn func(spec.Statement) bool
}

type identifierPatternRule struct {
	ruleID      string
	subject     string
	level       rule.Level
	pattern     *regexp.Regexp
	selects     func(spec.Statement) []identifierSubject
	appliesToFn func(spec.Statement) bool
}

func newIdentifierPatternRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject, appliesToOverride ...func(spec.Statement) bool) (rule.StatementRule, error) {
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
		ruleID:      ruleID,
		subject:     subject,
		level:       configuredLevel(cfg, fallbackLevel),
		pattern:     pattern,
		selects:     selects,
		appliesToFn: firstAppliesTo(appliesToOverride),
	}, nil
}

func newNamingPrefixRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject, appliesToOverride ...func(spec.Statement) bool) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if requirement.prefix == "" {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:      ruleID,
		subject:     subject,
		level:       configuredLevel(cfg, fallbackLevel),
		selects:     selects,
		matchKind:   "prefix",
		prefix:      requirement.prefix,
		suggestion:  fmt.Sprintf("rename the %s to start with %q", subject, requirement.prefix),
		appliesToFn: firstAppliesTo(appliesToOverride),
	}, nil
}

func newNamingSuffixRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject, appliesToOverride ...func(spec.Statement) bool) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if requirement.suffix == "" {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:      ruleID,
		subject:     subject,
		level:       configuredLevel(cfg, fallbackLevel),
		selects:     selects,
		matchKind:   "suffix",
		suffix:      requirement.suffix,
		suggestion:  fmt.Sprintf("rename the %s to end with %q", subject, requirement.suffix),
		appliesToFn: firstAppliesTo(appliesToOverride),
	}, nil
}

func newNamingContainsRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject, appliesToOverride ...func(spec.Statement) bool) (rule.StatementRule, error) {
	requirement, err := namingRequirementParam(ruleID, cfg)
	if err != nil {
		return nil, err
	}
	if len(requirement.contains) == 0 {
		return namingRule{ruleID: ruleID}, nil
	}
	return namingRule{
		ruleID:      ruleID,
		subject:     subject,
		level:       configuredLevel(cfg, fallbackLevel),
		selects:     selects,
		matchKind:   "contains",
		contains:    append([]string(nil), requirement.contains...),
		suggestion:  fmt.Sprintf("rename the %s to include one of: %s", subject, strings.Join(requirement.contains, ", ")),
		appliesToFn: firstAppliesTo(appliesToOverride),
	}, nil
}

func (r identifierPatternRule) ID() string { return r.ruleID }

func firstAppliesTo(overrides []func(spec.Statement) bool) func(spec.Statement) bool {
	if len(overrides) > 0 {
		return overrides[0]
	}
	return nil
}

func (r namingRule) ID() string { return r.ruleID }

func (r identifierPatternRule) AppliesTo(statement spec.Statement) bool {
	fn := r.appliesToFn
	if fn == nil {
		fn = appliesToCreateTable
	}
	return r.pattern != nil && fn(statement)
}

func (r namingRule) AppliesTo(statement spec.Statement) bool {
	fn := r.appliesToFn
	if fn == nil {
		fn = appliesToCreateTable
	}
	if !fn(statement) {
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

func (r identifierPatternRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

func (r namingRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
	ruleID      string
	subject     string
	level       rule.Level
	forbid      bool
	selects     func(spec.Statement) []identifierSubject
	appliesToFn func(spec.Statement) bool
}

func newIdentifierKeywordRule(ruleID, subject string, fallbackLevel rule.Level, cfg policy.RulePolicy, selects func(spec.Statement) []identifierSubject, appliesToOverride ...func(spec.Statement) bool) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return identifierKeywordRule{
		ruleID:      ruleID,
		subject:     subject,
		level:       configuredLevel(cfg, fallbackLevel),
		forbid:      forbid,
		selects:     selects,
		appliesToFn: firstAppliesTo(appliesToOverride),
	}, nil
}

func (r identifierKeywordRule) ID() string { return r.ruleID }

func (r identifierKeywordRule) AppliesTo(statement spec.Statement) bool {
	fn := r.appliesToFn
	if fn == nil {
		fn = appliesToCreateTable
	}
	return r.forbid && fn(statement)
}

func (r identifierKeywordRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, subject := range r.selects(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
	indexes := declaredIndexes(statement)
	if len(indexes) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0, len(indexes))
	for _, index := range indexes {
		subjects = append(subjects, identifierSubject{kind: "index", name: index.Name})
	}
	return subjects
}

// appliesToCreateTableOrAlterConstraintNames lets the unique/primary-key
// constraint naming families see ALTER TABLE … ADD CONSTRAINT names in
// addition to their create-table subjects. Index-syntax adds (add_index)
// stay with the ddl.index naming family. PostgreSQL is excluded: its only
// ALTER-side index path is add_constraint and those names keep their pinned
// ddl.alter.add_index.* naming-family ownership there.
func appliesToCreateTableOrAlterConstraintNames(statement spec.Statement) bool {
	if appliesToCreateTable(statement) {
		return true
	}
	if statement.Dialect == spec.DialectPostgreSQL {
		return false
	}
	return len(matchingAlterActions(statement, "add_constraint")) > 0
}

func selectPrimaryKeyConstraintNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil {
		return nil
	}
	subjects := make([]identifierSubject, 0, 1)
	if statement.DDL.PrimaryKey != nil {
		name := strings.TrimSpace(statement.DDL.PrimaryKey.Name)
		if name != "" && !strings.EqualFold(name, "primary") {
			subjects = append(subjects, identifierSubject{kind: "constraint.primary_key", name: name})
		}
	}
	return append(subjects, alterConstraintNamingSubjects(statement, spec.IndexKindPrimary, "constraint.primary_key")...)
}

// alterConstraintNamingSubjects collects ALTER-side add_constraint names of
// the produced index kind on the MySQL-family dialects. On PostgreSQL the
// add_constraint path stays with the add_index naming family, so the
// constraint selectors must not double-judge those names there.
func alterConstraintNamingSubjects(statement spec.Statement, kind spec.IndexKind, subjectKind string) []identifierSubject {
	if statement.Dialect == spec.DialectPostgreSQL {
		return nil
	}
	subjects := make([]identifierSubject, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		name := alterConstraintSubjectName(alter, kind)
		if name == "" || (kind == spec.IndexKindPrimary && strings.EqualFold(name, "primary")) {
			continue
		}
		subjects = append(subjects, identifierSubject{kind: subjectKind, name: name})
	}
	return subjects
}

func selectUniqueConstraintNames(statement spec.Statement) []identifierSubject {
	if statement.DDL == nil {
		return nil
	}
	subjects := make([]identifierSubject, 0, len(statement.DDL.Indexes))
	for _, index := range statement.DDL.Indexes {
		if index.Kind != spec.IndexKindUnique || strings.TrimSpace(index.Name) == "" {
			continue
		}
		subjects = append(subjects, identifierSubject{kind: "constraint.unique_key", name: index.Name})
	}
	// ALTER TABLE … ADD CONSTRAINT <name> UNIQUE names join the constraint
	// naming family; the same name is never an index-naming subject.
	return append(subjects, alterConstraintNamingSubjects(statement, spec.IndexKindUnique, "constraint.unique_key")...)
}

func selectForeignKeyConstraintNames(statement spec.Statement) []identifierSubject {
	subjects := selectConstraintNamesByType(statement, "foreign_key", "constraint.foreign_key")
	if !appliesToAlterTable(statement) {
		return subjects
	}
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		switch {
		case alter.Constraint != nil && alter.Constraint.Type == "foreign_key":
			// MySQL/TiDB carry the declared name on the constraint payload;
			// a bare ADD FOREIGN KEY has no authored name to judge.
			if name := strings.TrimSpace(alter.Constraint.Name); name != "" {
				subjects = append(subjects, identifierSubject{kind: "constraint.foreign_key", name: name})
			}
		case alter.Options["constraint_type"] == "foreign_key":
			// PostgreSQL carries the constraint name on alter.Name.
			if name := strings.TrimSpace(alter.Name); name != "" {
				subjects = append(subjects, identifierSubject{kind: "constraint.foreign_key", name: name})
			}
		}
	}
	return subjects
}

func selectCheckConstraintNames(statement spec.Statement) []identifierSubject {
	subjects := selectConstraintNamesByType(statement, "check", "constraint.check")
	if !appliesToAlterTable(statement) {
		return subjects
	}
	seen := make(map[string]bool, len(subjects))
	for _, subject := range subjects {
		seen[strings.ToLower(subject.name)] = true
	}
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		var name string
		switch {
		case alter.Constraint != nil && alter.Constraint.Type == "check":
			// MySQL/TiDB carry the declared name on the constraint payload;
			// a bare ADD CHECK has no authored name to judge.
			name = strings.TrimSpace(alter.Constraint.Name)
		case alter.Options["constraint_type"] == "check":
			// PostgreSQL carries the constraint name on alter.Name.
			name = strings.TrimSpace(alter.Name)
		}
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		subjects = append(subjects, identifierSubject{kind: "constraint.check", name: name})
	}
	for _, alter := range statement.DDL.Alter {
		// In-column CHECK members (ADD/MODIFY/CHANGE COLUMN ... CONSTRAINT
		// x CHECK(...)) declare names through the same governance channel —
		// the extractor projects them onto alter.Constraints.
		for _, member := range alter.Constraints {
			if member.Type != "check" {
				continue
			}
			name := strings.TrimSpace(member.Name)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			subjects = append(subjects, identifierSubject{kind: "constraint.check", name: name})
		}
	}
	return subjects
}

func selectConstraintNamesByType(statement spec.Statement, constraintType, kind string) []identifierSubject {
	if statement.DDL == nil || len(statement.DDL.Constraints) == 0 {
		return nil
	}
	subjects := make([]identifierSubject, 0, len(statement.DDL.Constraints))
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
