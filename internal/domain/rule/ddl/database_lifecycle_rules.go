// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for MySQL/TiDB database/schema lifecycle DDL operations
// output: findings for database/schema creation and drop risks on MySQL and TiDB
// pos: MySQL/TiDB database lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type databaseLifecycleRule struct {
	id        string
	level     rule.Level
	operation spec.DDLOperation
	message   string
}

func newDatabaseCreateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return databaseLifecycleRule{
		id:        ruleIDDatabaseCreateNotice,
		level:     configuredLevel(cfg, rule.LevelNotice),
		operation: spec.DDLOperationCreateSchema,
		message:   "database/schema %q creation creates a new logical namespace and should be reviewed",
	}, nil
}

func newDatabaseDropWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return databaseLifecycleRule{
		id:        ruleIDDatabaseDropWarn,
		level:     configuredLevel(cfg, rule.LevelWarning),
		operation: spec.DDLOperationDropSchema,
		message:   "database/schema %q drop removes all contained objects and should be reviewed",
	}, nil
}

func (r databaseLifecycleRule) ID() string { return r.id }

func (r databaseLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.ObjectType == "database"
}

func (r databaseLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Metadata: map[string]any{
			"operation":   string(r.operation),
			"object_type": "database",
			"object_name": objectName,
		},
	}}, nil
}
