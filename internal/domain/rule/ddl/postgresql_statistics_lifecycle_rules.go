package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgStatisticsLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGStatisticsLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgStatisticsLifecycleRule{
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

func (r pgStatisticsLifecycleRule) ID() string { return r.id }

func (r pgStatisticsLifecycleRule) AppliesTo(statement spec.Statement) bool {
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

func (r pgStatisticsLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"action", "new_name", "new_schema", "owner", "target_table", "if_exists", "cascade"} {
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

func newCreateStatisticsNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGStatisticsLifecycleRule(
		ruleIDPGCreateStatisticsNotice, rule.LevelNotice, spec.DDLOperationCreateStatistics, "",
		"PostgreSQL extended statistics %q created — query planner correlation tracking defined",
		"CREATE STATISTICS defines extended statistics objects that help the query planner estimate cardinality for correlated columns.",
		"Extended statistics affect query plan choices. Dropping or changing them may cause the planner to pick different execution paths.",
		"Verify that the statistics target columns and kinds match the intended workload patterns.",
		cfg,
	)
}

func newAlterStatisticsNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGStatisticsLifecycleRule(
		ruleIDPGAlterStatisticsNotice, rule.LevelNotice, spec.DDLOperationAlterStatistics, "",
		"PostgreSQL extended statistics %q altered — correlation tracking configuration changed",
		"ALTER STATISTICS changes the configuration of an existing extended statistics object, such as renaming it, changing its owner, moving it to a different schema, or adjusting its statistics target.",
		"Changing statistics configuration may affect query plan estimation and execution paths that depend on the extended statistics.",
		"Review the change to ensure planner estimation behavior remains correct for dependent queries.",
		cfg,
	)
}

func newDropStatisticsWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGStatisticsLifecycleRule(
		ruleIDPGDropStatisticsWarn, rule.LevelWarning, spec.DDLOperationDropStatistics, "",
		"PostgreSQL extended statistics %q dropped — correlation tracking removed",
		"DROP STATISTICS permanently removes an extended statistics object from the database.",
		"Dropping extended statistics may cause the query planner to produce suboptimal plans for queries whose cardinality estimates relied on the statistics object.",
		"Ensure no query plans depend on this statistics object before dropping.",
		cfg,
	)
}
