// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL REFRESH MATERIALIZED VIEW operations
// output: findings for non-concurrent refresh and WITH NO DATA refresh risks
// pos: PostgreSQL-specific materialized view refresh rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgRefreshMaterializedViewConcurrentlyRule struct {
	level rule.Level
}

func newRefreshMaterializedViewConcurrentlyWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgRefreshMaterializedViewConcurrentlyRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r pgRefreshMaterializedViewConcurrentlyRule) ID() string {
	return ruleIDPGRefreshMaterializedViewConcurrentlyWarn
}

func (r pgRefreshMaterializedViewConcurrentlyRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationRefreshMaterializedView
}

func (r pgRefreshMaterializedViewConcurrentlyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if statement.DDL.Options["concurrently"] == "true" {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := "REFRESH MATERIALIZED VIEW without CONCURRENTLY may block reads"
	if objectName != "" {
		message = fmt.Sprintf("REFRESH MATERIALIZED VIEW %q without CONCURRENTLY may block reads on PostgreSQL", objectName)
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        "Non-concurrent REFRESH MATERIALIZED VIEW takes an exclusive lock that blocks all reads from the materialized view until the refresh completes.",
			Risk:       "Queries, dashboards, or ETL steps that read from this materialized view will be blocked for the duration of the refresh.",
			Suggestion: "Use REFRESH MATERIALIZED VIEW CONCURRENTLY when the materialized view has a suitable unique index, or run during a maintenance window. DeltaScope does not verify unique-index requirements offline.",
		},
		Metadata: map[string]any{
			"operation":    string(statement.DDL.Operation),
			"object_type":  statement.DDL.ObjectType,
			"object":       objectName,
			"concurrently": statement.DDL.Options["concurrently"],
			"with_no_data": statement.DDL.Options["with_no_data"],
		},
	}}, nil
}

type pgRefreshMaterializedViewNoDataRule struct {
	level rule.Level
}

func newRefreshMaterializedViewNoDataNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgRefreshMaterializedViewNoDataRule{
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r pgRefreshMaterializedViewNoDataRule) ID() string {
	return ruleIDPGRefreshMaterializedViewNoDataNotice
}

func (r pgRefreshMaterializedViewNoDataRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationRefreshMaterializedView
}

func (r pgRefreshMaterializedViewNoDataRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if statement.DDL.Options["with_no_data"] != "true" {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := "REFRESH MATERIALIZED VIEW with NO DATA leaves the materialized view unpopulated"
	if objectName != "" {
		message = fmt.Sprintf("REFRESH MATERIALIZED VIEW %q WITH NO DATA leaves the materialized view unpopulated on PostgreSQL", objectName)
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        "REFRESH MATERIALIZED VIEW ... WITH NO DATA empties the materialized view content without repopulating it, making it unscannable until a subsequent data refresh.",
			Risk:       "Downstream queries that read from this materialized view will fail or return no rows until a REFRESH ... WITH DATA is run.",
			Suggestion: "Confirm that downstream consumers tolerate the materialized view being unavailable for reads. Schedule a follow-up REFRESH MATERIALIZED VIEW ... WITH DATA to repopulate it.",
		},
		Metadata: map[string]any{
			"operation":    string(statement.DDL.Operation),
			"object_type":  statement.DDL.ObjectType,
			"object":       objectName,
			"concurrently": statement.DDL.Options["concurrently"],
			"with_no_data": statement.DDL.Options["with_no_data"],
		},
	}}, nil
}
