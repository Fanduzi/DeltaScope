// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL publication and subscription lifecycle DDL operations
// output: findings for PostgreSQL replication object creation, alteration, and drop risks
// pos: PostgreSQL-specific publication/subscription lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgReplicationLifecycleRule struct {
	id            string
	level         rule.Level
	operation     spec.DDLOperation
	optionKey     string
	optionValue   string
	requireOption bool
	object        string
	message       string
	why           string
	risk          string
	suggestion    string
}

func newPGReplicationLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, optionKey, optionValue string, requireOption bool, object, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgReplicationLifecycleRule{
		id:            id,
		level:         configuredLevel(cfg, level),
		operation:     operation,
		optionKey:     optionKey,
		optionValue:   optionValue,
		requireOption: requireOption,
		object:        object,
		message:       message,
		why:           why,
		risk:          risk,
		suggestion:    suggestion,
	}, nil
}

func (r pgReplicationLifecycleRule) ID() string { return r.id }

func (r pgReplicationLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgReplicationLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.requireOption && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}
	if !r.requireOption && r.optionKey != "" && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"all_tables", "action", "has_connection", "if_exists", "cascade"} {
		if val := statement.DDL.Options[key]; val != "" {
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

func newCreatePublicationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGCreatePublicationNotice, rule.LevelNotice, spec.DDLOperationCreatePublication,
		"", "", false, "publication",
		"PostgreSQL publication %q created — review logical replication change",
		"CREATE PUBLICATION defines a publication for logical replication, controlling which table changes are replicated to subscribers.",
		"Publications change the replication topology and may expose data to downstream systems that were not previously receiving changes.",
		"Confirm the publication scope is intended and that downstream subscribers are prepared to receive the replicated data.",
		cfg,
	)
}

func newAlterPublicationNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGAlterPublicationNotice, rule.LevelNotice, spec.DDLOperationAlterPublication,
		"", "", false, "publication",
		"PostgreSQL publication %q altered — review replication scope change",
		"ALTER PUBLICATION changes which tables or schemas are included in the publication.",
		"Adding or removing tables from a publication affects which data changes are sent to subscribers.",
		"Verify that subscriber-side expectations still hold after the publication membership change.",
		cfg,
	)
}

func newDropPublicationWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGDropPublicationWarn, rule.LevelWarning, spec.DDLOperationDropPublication,
		"", "", false, "publication",
		"PostgreSQL publication %q dropped — subscribers will stop receiving changes",
		"DROP PUBLICATION removes the publication, causing all associated subscriptions to stop receiving changes.",
		"Existing subscriptions referencing this publication will enter an error state and replication will halt.",
		"Verify all subscribers have been redirected or are prepared for replication to stop before dropping.",
		cfg,
	)
}

func newCreateSubscriptionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGCreateSubscriptionNotice, rule.LevelNotice, spec.DDLOperationCreateSubscription,
		"", "", false, "subscription",
		"PostgreSQL subscription %q created — review logical replication setup",
		"CREATE SUBSCRIPTION establishes a replication connection from this database to a publisher.",
		"Subscriptions initiate data streaming from a remote publisher, which may affect local data, trigger events, and consume resources.",
		"Confirm the publisher connection details, the replication identity of source tables, and that local schema is prepared.",
		cfg,
	)
}

func newAlterSubscriptionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGAlterSubscriptionNotice, rule.LevelNotice, spec.DDLOperationAlterSubscription,
		"", "", false, "subscription",
		"PostgreSQL subscription %q altered — review replication configuration change",
		"ALTER SUBSCRIPTION modifies the replication connection, publication set, or run state of a subscription.",
		"Changes to subscription configuration may interrupt replication, change the data set being replicated, or alter connection parameters.",
		"Verify that the subscription change is compatible with the publisher and that replication will continue as expected.",
		cfg,
	)
}

func newAlterSubscriptionDisableWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGAlterSubscriptionDisableWarn, rule.LevelWarning, spec.DDLOperationAlterSubscription,
		"action", "disable", true, "subscription",
		"PostgreSQL subscription %q disabled — replication halted",
		"Disabling a subscription stops the replication worker, causing data changes on the publisher to accumulate without being applied locally.",
		"Prolonged disable may lead to replication lag, WAL accumulation on the publisher, and eventual slot or disk exhaustion.",
		"Re-enable the subscription promptly or drop it if no longer needed to prevent unbounded resource growth on the publisher.",
		cfg,
	)
}

func newDropSubscriptionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGReplicationLifecycleRule(
		ruleIDPGDropSubscriptionWarn, rule.LevelWarning, spec.DDLOperationDropSubscription,
		"", "", false, "subscription",
		"PostgreSQL subscription %q dropped — replication terminated",
		"DROP SUBSCRIPTION removes the subscription, terminating replication and cleaning up the replication slot by default.",
		"Data changes on the publisher will no longer be replicated. The replication slot is dropped unless retained explicitly.",
		"Verify that downstream dependencies on the replicated data have been migrated or are no longer needed.",
		cfg,
	)
}
