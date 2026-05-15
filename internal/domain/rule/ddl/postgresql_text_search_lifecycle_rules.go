package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTextSearchLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGTextSearchLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTextSearchLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		action:     action,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgTextSearchLifecycleRule) ID() string { return r.id }

func (r pgTextSearchLifecycleRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectPostgreSQL ||
		statement.Kind != spec.KindDDL ||
		statement.DDL == nil ||
		statement.DDL.Operation != r.operation {
		return false
	}
	if r.action == "" {
		return true
	}
	return statement.DDL.Options["action"] == r.action
}

func (r pgTextSearchLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"action", "new_name", "new_schema", "owner", "if_exists", "cascade"} {
		if val, ok := statement.DDL.Options[key]; ok && val != "" {
			metadata[key] = val
		}
	}
	for k, v := range projectObjectMetadata(statement) {
		metadata[k] = v
	}

	return []rule.Finding{{
		RuleID:  r.id,
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: metadata,
	}}, nil
}

func newCreateTextSearchConfigurationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGCreateTextSearchConfigurationNotice, rule.LevelNotice, spec.DDLOperationCreateTextSearchConfiguration, "",
		"PostgreSQL text search configuration %q created",
		"CREATE TEXT SEARCH CONFIGURATION defines a new text search configuration for controlling how documents are parsed and processed.",
		"Text search configurations affect how full-text search indexes and queries behave. Removing or changing them can break search functionality.",
		"Verify that the configuration name and copy source match application text search requirements.",
		cfg,
	)
}

func newAlterTextSearchConfigurationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGAlterTextSearchConfigurationNotice, rule.LevelNotice, spec.DDLOperationAlterTextSearchConfiguration, "",
		"PostgreSQL text search configuration %q altered",
		"ALTER TEXT SEARCH CONFIGURATION changes a text search configuration, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing a text search configuration may affect full-text search behavior for queries that depend on it.",
		"Review the change to ensure full-text search behavior remains correct.",
		cfg,
	)
}

func newDropTextSearchConfigurationWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGDropTextSearchConfigurationWarn, rule.LevelWarning, spec.DDLOperationDropTextSearchConfiguration, "",
		"PostgreSQL text search configuration %q dropped",
		"DROP TEXT SEARCH CONFIGURATION permanently removes a text search configuration from the database.",
		"Dropping a text search configuration breaks any full-text search indexes or queries that depend on it.",
		"Ensure no full-text search operations depend on this configuration before dropping.",
		cfg,
	)
}

func newCreateTextSearchDictionaryNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGCreateTextSearchDictionaryNotice, rule.LevelNotice, spec.DDLOperationCreateTextSearchDictionary, "",
		"PostgreSQL text search dictionary %q created",
		"CREATE TEXT SEARCH DICTIONARY defines a new text search dictionary for controlling how tokens are processed during full-text search.",
		"Text search dictionaries affect how full-text search indexes and queries process tokens. Removing or changing them can break search functionality.",
		"Verify that the dictionary name and template match application text search requirements.",
		cfg,
	)
}

func newAlterTextSearchDictionaryNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGAlterTextSearchDictionaryNotice, rule.LevelNotice, spec.DDLOperationAlterTextSearchDictionary, "",
		"PostgreSQL text search dictionary %q altered",
		"ALTER TEXT SEARCH DICTIONARY changes a text search dictionary, such as renaming it, changing its owner, or modifying its options.",
		"Changing a text search dictionary may affect full-text search behavior for queries that depend on it.",
		"Review the change to ensure full-text search behavior remains correct.",
		cfg,
	)
}

func newDropTextSearchDictionaryWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGDropTextSearchDictionaryWarn, rule.LevelWarning, spec.DDLOperationDropTextSearchDictionary, "",
		"PostgreSQL text search dictionary %q dropped",
		"DROP TEXT SEARCH DICTIONARY permanently removes a text search dictionary from the database.",
		"Dropping a text search dictionary breaks any text search configurations that depend on it.",
		"Ensure no text search configurations depend on this dictionary before dropping.",
		cfg,
	)
}

func newCreateTextSearchParserNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGCreateTextSearchParserNotice, rule.LevelNotice, spec.DDLOperationCreateTextSearchParser, "",
		"PostgreSQL text search parser %q created",
		"CREATE TEXT SEARCH PARSER defines a new text search parser for splitting documents into tokens during full-text search.",
		"Text search parsers affect how documents are tokenized. Removing or changing them can break full-text search functionality.",
		"Verify that the parser name and function signatures match application text search requirements.",
		cfg,
	)
}

func newAlterTextSearchParserNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGAlterTextSearchParserNotice, rule.LevelNotice, spec.DDLOperationAlterTextSearchParser, "",
		"PostgreSQL text search parser %q altered",
		"ALTER TEXT SEARCH PARSER changes a text search parser, such as renaming it or moving it to a different schema.",
		"Changing a text search parser may affect full-text search behavior for configurations that depend on it.",
		"Review the change to ensure full-text search behavior remains correct.",
		cfg,
	)
}

func newDropTextSearchParserWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGDropTextSearchParserWarn, rule.LevelWarning, spec.DDLOperationDropTextSearchParser, "",
		"PostgreSQL text search parser %q dropped",
		"DROP TEXT SEARCH PARSER permanently removes a text search parser from the database.",
		"Dropping a text search parser breaks any text search configurations that depend on it.",
		"Ensure no text search configurations depend on this parser before dropping.",
		cfg,
	)
}

func newCreateTextSearchTemplateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGCreateTextSearchTemplateNotice, rule.LevelNotice, spec.DDLOperationCreateTextSearchTemplate, "",
		"PostgreSQL text search template %q created",
		"CREATE TEXT SEARCH TEMPLATE defines a new text search template for providing the implementation behind text search dictionaries.",
		"Text search templates define how dictionaries process tokens. Removing or changing them can break full-text search functionality.",
		"Verify that the template name and function signatures match application text search requirements.",
		cfg,
	)
}

func newAlterTextSearchTemplateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGAlterTextSearchTemplateNotice, rule.LevelNotice, spec.DDLOperationAlterTextSearchTemplate, "",
		"PostgreSQL text search template %q altered",
		"ALTER TEXT SEARCH TEMPLATE changes a text search template, such as renaming it or moving it to a different schema.",
		"Changing a text search template may affect full-text search behavior for dictionaries that depend on it.",
		"Review the change to ensure full-text search behavior remains correct.",
		cfg,
	)
}

func newDropTextSearchTemplateWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTextSearchLifecycleRule(
		ruleIDPGDropTextSearchTemplateWarn, rule.LevelWarning, spec.DDLOperationDropTextSearchTemplate, "",
		"PostgreSQL text search template %q dropped",
		"DROP TEXT SEARCH TEMPLATE permanently removes a text search template from the database.",
		"Dropping a text search template breaks any text search dictionaries that depend on it.",
		"Ensure no text search dictionaries depend on this template before dropping.",
		cfg,
	)
}
