// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mysqlTidbLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	objectType string
	message    string
	suggestion string
	// adjudicate, when set, lets a lifecycle notice consume the shared
	// ordered-state judgment instead of the unconditional single finding:
	// the hook returns (findings, true) to own the verdict or (nil, false)
	// to fall back to the generic path (T21-R1 B1 — sequence notices).
	adjudicate func(level rule.Level, statement spec.Statement) ([]rule.Finding, bool)
}

func newMySQLTiDBLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, objectType string, message string, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlTidbLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		objectType: objectType,
		message:    message,
		suggestion: suggestion,
	}, nil
}

func (r mysqlTidbLifecycleRule) ID() string { return r.id }

func (r mysqlTidbLifecycleRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil {
		return false
	}
	if statement.DDL.Operation != r.operation && (r.operation != spec.DDLOperationCreateIndex || !appliesToAlterActions(statement, "add_index")) {
		return false
	}
	if r.objectType != "" && statement.DDL.ObjectType != r.objectType {
		return false
	}
	return true
}

func (r mysqlTidbLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.adjudicate != nil {
		if findings, handled := r.adjudicate(r.level, statement); handled {
			return findings, nil
		}
	}
	objectName := statement.DDL.ObjectName
	message := r.message
	if strings.Contains(r.message, "%q") {
		message = fmt.Sprintf(r.message, lifecycleMessageObjectName(statement))
	}
	metadata := map[string]any{
		"operation": string(r.operation),
	}
	if statement.DDL.Operation == spec.DDLOperationAlterTable {
		if indexName := firstAlterIndexName(statement.DDL, "add_index"); indexName != "" {
			metadata["action"] = "add_index"
			metadata["index"] = indexName
		}
	}
	if r.objectType != "" {
		metadata["object_type"] = r.objectType
	}
	if objectName != "" {
		metadata["object_name"] = objectName
	}
	if statement.DDL.Table != nil {
		metadata["table"] = statement.DDL.Table.Name
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    message,
		Suggestion: r.suggestion,
		Metadata:   metadata,
	}}, nil
}

func lifecycleMessageObjectName(statement spec.Statement) string {
	ddl := statement.DDL
	if ddl.ObjectName != "" {
		return ddl.ObjectName
	}

	switch ddl.Operation {
	case spec.DDLOperationRenameTable:
		if ddl.Table != nil && ddl.Table.Name != "" {
			return qualifiedTableName(ddl.Table.Schema, ddl.Table.Name)
		}
	case spec.DDLOperationCreateIndex:
		return firstAlterIndexName(ddl, "")
	case spec.DDLOperationAlterTable:
		return firstAlterIndexName(ddl, "add_index")
	case spec.DDLOperationDropIndex:
		for _, alter := range ddl.Alter {
			if alter.Index != nil && alter.Index.OldName != "" {
				return alter.Index.OldName
			}
		}
	}
	return ""
}

func firstAlterIndexName(ddl *spec.DDL, action string) string {
	for _, alter := range ddl.Alter {
		if action != "" && alter.Action != action {
			continue
		}
		if index, ok := alterIndexDefinition(alter); ok && index.Name != "" {
			return index.Name
		}
	}
	return ""
}

// Constructor functions for each rule.

func newRenameTableNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDRenameTableNotice, rule.LevelNotice, spec.DDLOperationRenameTable, "",
		"RENAME TABLE %q changes table identifiers",
		"verify all references to the old table name are updated before applying",
		cfg,
	)
}

func newCreateIndexNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateIndexNotice, rule.LevelNotice, spec.DDLOperationCreateIndex, "",
		"CREATE INDEX %q adds a new index to a table",
		"review index naming conventions and column selection before applying",
		cfg,
	)
}

func newDropIndexNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropIndexNotice, rule.LevelNotice, spec.DDLOperationDropIndex, "",
		"DROP INDEX %q removes an existing index from a table",
		"verify no queries depend on this index for performance before dropping",
		cfg,
	)
}

func newAlterDatabaseNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterDatabaseNotice, rule.LevelNotice, spec.DDLOperationAlterSchema, "database",
		"ALTER DATABASE %q changes database-level settings",
		"review charset and collation changes for application compatibility",
		cfg,
	)
}

func newMySQLTiDBCreateProcedureNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateProcedureNotice, rule.LevelNotice, spec.DDLOperationCreateProcedure, "procedure",
		"CREATE PROCEDURE %q registers a new stored procedure",
		"review procedure logic and security implications before deploying",
		cfg,
	)
}

func newMySQLTiDBAlterProcedureNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterProcedureNotice, rule.LevelNotice, spec.DDLOperationAlterProcedure, "procedure",
		"ALTER PROCEDURE %q changes stored procedure characteristics",
		"review characteristic changes such as SQL SECURITY before applying",
		cfg,
	)
}

func newDropProcedureNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropProcedureNotice, rule.LevelNotice, spec.DDLOperationDropProcedure, "procedure",
		"DROP PROCEDURE %q removes a stored procedure",
		"verify no application code calls this procedure before dropping",
		cfg,
	)
}

func newMySQLTiDBCreateTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateTriggerNotice, rule.LevelNotice, spec.DDLOperationCreateTrigger, "trigger",
		"CREATE TRIGGER %q registers a new trigger",
		"review trigger timing, event, and body effects before deploying",
		cfg,
	)
}

func newMySQLTiDBDropTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropTriggerNotice, rule.LevelNotice, spec.DDLOperationDropTrigger, "trigger",
		"DROP TRIGGER %q removes a trigger",
		"verify no application flow depends on this trigger before dropping",
		cfg,
	)
}

func newMySQLTiDBCreateEventNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateEventNotice, rule.LevelNotice, spec.DDLOperationCreateEvent, "event",
		"CREATE EVENT %q registers a new scheduled event",
		"review the schedule, enablement, and body effects before deploying",
		cfg,
	)
}

func newMySQLTiDBAlterEventNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterEventNotice, rule.LevelNotice, spec.DDLOperationAlterEvent, "event",
		"ALTER EVENT %q changes a scheduled event",
		"review the changed schedule, enablement, or body effects before deploying",
		cfg,
	)
}

func newMySQLTiDBDropEventNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropEventNotice, rule.LevelNotice, spec.DDLOperationDropEvent, "event",
		"DROP EVENT %q removes a scheduled event",
		"verify no workload depends on this event before dropping",
		cfg,
	)
}

func newMySQLTiDBCreateFunctionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateFunctionNotice, rule.LevelNotice, spec.DDLOperationCreateFunction, "function",
		"CREATE FUNCTION %q registers a new stored function",
		"review function logic and security implications before deploying",
		cfg,
	)
}

func newMySQLTiDBAlterFunctionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterFunctionNotice, rule.LevelNotice, spec.DDLOperationAlterFunction, "function",
		"ALTER FUNCTION %q changes stored function characteristics",
		"review characteristic changes such as SQL SECURITY before applying",
		cfg,
	)
}

func newMySQLTiDBDropFunctionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropFunctionNotice, rule.LevelNotice, spec.DDLOperationDropFunction, "function",
		"DROP FUNCTION %q removes a stored function",
		"verify no application code calls this function before dropping",
		cfg,
	)
}

func newCreateUserNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateUserNotice, rule.LevelNotice, spec.DDLOperationCreateUser, "user",
		"CREATE USER %q creates a new database user",
		"follow principle of least privilege when granting permissions to this user",
		cfg,
	)
}

func newAlterUserNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterUserNotice, rule.LevelNotice, spec.DDLOperationAlterUser, "user",
		"ALTER USER %q modifies an existing database user",
		"review authentication and privilege changes before applying",
		cfg,
	)
}

func newDropUserNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropUserNotice, rule.LevelNotice, spec.DDLOperationDropUser, "user",
		"DROP USER %q removes a database user",
		"verify no applications use this user account before dropping",
		cfg,
	)
}

func newCreateRoleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateRoleNotice, rule.LevelNotice, spec.DDLOperationCreateRole, "role",
		"CREATE ROLE %q creates a new database role",
		"define appropriate privileges for this role before assigning to users",
		cfg,
	)
}

func newDropRoleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropRoleNotice, rule.LevelNotice, spec.DDLOperationDropRole, "role",
		"DROP ROLE %q removes a database role",
		"verify no users are currently assigned this role before dropping",
		cfg,
	)
}

func newGrantNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDGrantNotice, rule.LevelNotice, spec.DDLOperationGrant, "",
		"GRANT assigns privileges to a user or role",
		"review granted privileges follow the principle of least privilege",
		cfg,
	)
}

func newRevokeNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDRevokeNotice, rule.LevelNotice, spec.DDLOperationRevoke, "",
		"REVOKE removes privileges from a user or role",
		"verify no applications depend on the revoked privileges",
		cfg,
	)
}

func newCreateResourceGroupNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDCreateResourceGroupNotice, rule.LevelNotice, spec.DDLOperationCreateResourceGroup, "resource_group",
		"CREATE RESOURCE GROUP %q defines a new resource group",
		"review quota and burst settings before sessions bind to the group",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = resourceGroupNoticeAdjudicator("create", n.message, n.suggestion)
	return n, nil
}

func newAlterResourceGroupNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDAlterResourceGroupNotice, rule.LevelNotice, spec.DDLOperationAlterResourceGroup, "resource_group",
		"ALTER RESOURCE GROUP %q modifies an existing resource group",
		"review quota and priority changes for impact on bound workloads",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = resourceGroupNoticeAdjudicator("alter", n.message, n.suggestion)
	return n, nil
}

func newDropResourceGroupNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDDropResourceGroupNotice, rule.LevelNotice, spec.DDLOperationDropResourceGroup, "resource_group",
		"DROP RESOURCE GROUP %q removes a resource group",
		"verify no sessions are using this resource group before dropping",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = resourceGroupNoticeAdjudicator("drop", n.message, n.suggestion)
	return n, nil
}

func newCreatePlacementPolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDCreatePlacementPolicyNotice, rule.LevelNotice, spec.DDLOperationCreatePlacementPolicy, "placement_policy",
		"CREATE PLACEMENT POLICY %q defines a new placement policy",
		"review placement rules and region configuration before deploying",
		cfg,
	)
}

func newAlterPlacementPolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterPlacementPolicyNotice, rule.LevelNotice, spec.DDLOperationAlterPlacementPolicy, "placement_policy",
		"ALTER PLACEMENT POLICY %q modifies an existing placement policy",
		"review placement changes for impact on data locality before applying",
		cfg,
	)
}

func newDropPlacementPolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropPlacementPolicyNotice, rule.LevelNotice, spec.DDLOperationDropPlacementPolicy, "placement_policy",
		"DROP PLACEMENT POLICY %q removes a placement policy",
		"verify no tables reference this placement policy before dropping",
		cfg,
	)
}

func newCreateSequenceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDCreateSequenceNotice, rule.LevelNotice, spec.DDLOperationCreateSequence, "sequence",
		"CREATE SEQUENCE %q declares a new sequence generator",
		"review sequence options and initial values before deploying",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = sequenceNoticeAdjudicator("create", n.message, n.suggestion)
	return n, nil
}

func newAlterSequenceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDAlterSequenceNotice, rule.LevelNotice, spec.DDLOperationAlterSequence, "sequence",
		"ALTER SEQUENCE %q declares changes to a sequence generator",
		"review sequence changes for impact on auto-generated values",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = sequenceNoticeAdjudicator("alter", n.message, n.suggestion)
	return n, nil
}

func newDropSequenceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newMySQLTiDBLifecycleRule(
		ruleIDDropSequenceNotice, rule.LevelNotice, spec.DDLOperationDropSequence, "sequence",
		"DROP SEQUENCE %q declares removal of a sequence generator",
		"verify no applications or default values reference this sequence before dropping",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	n := base.(mysqlTidbLifecycleRule)
	n.adjudicate = sequenceNoticeAdjudicator("drop", n.message, n.suggestion)
	return n, nil
}
