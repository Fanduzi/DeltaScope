// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for MySQL/TiDB DDL lifecycle operations
// output: notice-level findings for MySQL/TiDB rename, index, ALTER-index, DCL, user, role, procedure, placement, sequence, and resource group operations
// pos: MySQL/TiDB lifecycle rule implementations for v0.200.0 normalized-silent coverage
// note: if this file changes, update this header and module README.md.
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
	if statement.DDL.Operation != r.operation && !(r.operation == spec.DDLOperationCreateIndex && appliesToAlterActions(statement, "add_index")) {
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
	objectName := statement.DDL.ObjectName
	message := r.message
	if strings.Contains(r.message, "%q") {
		message = fmt.Sprintf(r.message, lifecycleMessageObjectName(statement))
	}
	metadata := map[string]any{
		"operation": string(r.operation),
	}
	if statement.DDL.Operation == spec.DDLOperationAlterTable {
		for _, alter := range statement.DDL.Alter {
			if alter.Action != "add_index" {
				continue
			}
			metadata["action"] = alter.Action
			metadata["name"] = alter.Name
			break
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
		for _, alter := range ddl.Alter {
			if index, ok := alterIndexDefinition(alter); ok && index.Name != "" {
				return index.Name
			}
		}
	case spec.DDLOperationAlterTable:
		for _, alter := range ddl.Alter {
			if alter.Action != "add_index" {
				continue
			}
			if index, ok := alterIndexDefinition(alter); ok && index.Name != "" {
				return index.Name
			}
		}
	case spec.DDLOperationDropIndex:
		for _, alter := range ddl.Alter {
			if alter.Index != nil && alter.Index.OldName != "" {
				return alter.Index.OldName
			}
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

func newDropProcedureNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropProcedureNotice, rule.LevelNotice, spec.DDLOperationDropProcedure, "procedure",
		"DROP PROCEDURE %q removes a stored procedure",
		"verify no application code calls this procedure before dropping",
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

func newDropResourceGroupNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropResourceGroupNotice, rule.LevelNotice, spec.DDLOperationDropResourceGroup, "resource_group",
		"DROP RESOURCE GROUP %q removes a resource group",
		"verify no sessions are using this resource group before dropping",
		cfg,
	)
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
	return newMySQLTiDBLifecycleRule(
		ruleIDCreateSequenceNotice, rule.LevelNotice, spec.DDLOperationCreateSequence, "sequence",
		"CREATE SEQUENCE %q creates a new sequence generator",
		"review sequence options and initial values before deploying",
		cfg,
	)
}

func newAlterSequenceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDAlterSequenceNotice, rule.LevelNotice, spec.DDLOperationAlterSequence, "sequence",
		"ALTER SEQUENCE %q modifies an existing sequence generator",
		"review sequence changes for impact on auto-generated values",
		cfg,
	)
}

func newDropSequenceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newMySQLTiDBLifecycleRule(
		ruleIDDropSequenceNotice, rule.LevelNotice, spec.DDLOperationDropSequence, "sequence",
		"DROP SEQUENCE %q removes a sequence generator",
		"verify no applications or default values reference this sequence before dropping",
		cfg,
	)
}
