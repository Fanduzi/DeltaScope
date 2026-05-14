package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCollationLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGCollationLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCollationLifecycleRule{
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

func (r pgCollationLifecycleRule) ID() string { return r.id }

func (r pgCollationLifecycleRule) AppliesTo(statement spec.Statement) bool {
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

func (r pgCollationLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"action", "new_name", "new_schema", "if_exists", "cascade"} {
		if val, ok := statement.DDL.Options[key]; ok && val != "" {
			metadata[key] = val
		}
	}
	for k, v := range projectObjectMetadata(statement) {
		metadata[k] = v
	}

	return []rule.Finding{{
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

func newCreateCollationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGCollationLifecycleRule(
		ruleIDPGCreateCollationNotice, rule.LevelNotice, spec.DDLOperationCreateCollation, "",
		"PostgreSQL collation %q created — string sort ordering defined",
		"CREATE COLLATION defines a custom collation that controls how strings are sorted and compared in the database.",
		"Custom collations affect query results and index ordering. Applications may behave differently if the collation definition changes or is removed.",
		"Verify that the collation locale and provider are intentional and match application expectations.",
		cfg,
	)
}

func newAlterCollationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGCollationLifecycleRule(
		ruleIDPGAlterCollationNotice, rule.LevelNotice, spec.DDLOperationAlterCollation, "",
		"PostgreSQL collation %q altered — string sort ordering configuration changed",
		"ALTER COLLATION changes the configuration of an existing collation, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing collation configuration may affect query results, index ordering, and application behavior that depends on string comparison.",
		"Review the change to ensure string ordering semantics remain correct for dependent queries.",
		cfg,
	)
}

func newDropCollationWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGCollationLifecycleRule(
		ruleIDPGDropCollationWarn, rule.LevelWarning, spec.DDLOperationDropCollation, "",
		"PostgreSQL collation %q dropped — string sort ordering removed",
		"DROP COLLATION permanently removes a custom collation from the database.",
		"Dropping a collation invalidates any columns, indexes, or queries that reference it. Dependent objects will fail until the collation is restored or references are updated.",
		"Ensure no columns, indexes, or queries depend on this collation before dropping.",
		cfg,
	)
}
