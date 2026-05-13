// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL foreign object lifecycle DDL operations
// output: findings for PostgreSQL foreign table, server, user mapping, and FDW lifecycle events
// pos: PostgreSQL-specific foreign object lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgForeignObjectLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGForeignObjectLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, object, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgForeignObjectLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		object:     object,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgForeignObjectLifecycleRule) ID() string { return r.id }

func (r pgForeignObjectLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgForeignObjectLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"server", "has_options", "action", "if_exists", "cascade", "foreign_data_wrapper", "has_handler", "has_version", "user"} {
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

// Foreign table lifecycle rules.

func newCreateForeignTableNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGCreateForeignTableNotice, rule.LevelNotice, spec.DDLOperationCreateForeignTable,
		"foreign_table",
		"PostgreSQL foreign table %q created — review cross-database data access",
		"CREATE FOREIGN TABLE defines a table whose data resides on a remote server, accessed through a foreign data wrapper.",
		"Foreign tables expose data from external databases; changes to the remote schema or connection may silently break queries or expose unintended data.",
		"Confirm the remote server is trusted, the column mapping matches the remote schema, and access controls are appropriate.",
		cfg,
	)
}

func newAlterForeignTableNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGAlterForeignTableNotice, rule.LevelNotice, spec.DDLOperationAlterForeignTable,
		"foreign_table",
		"PostgreSQL foreign table %q altered — review foreign table configuration change",
		"ALTER FOREIGN TABLE modifies the definition of a foreign table, including column mappings and server options.",
		"Changes to foreign table options may affect how data is fetched from the remote server or how columns are mapped.",
		"Verify the remote schema still matches the foreign table definition after the change.",
		cfg,
	)
}

func newDropForeignTableWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGDropForeignTableWarn, rule.LevelWarning, spec.DDLOperationDropForeignTable,
		"foreign_table",
		"PostgreSQL foreign table %q dropped — cross-database access removed",
		"DROP FOREIGN TABLE removes the local foreign table definition, severing the link to the remote data source.",
		"Existing queries and views referencing this foreign table will fail. The remote data is not affected.",
		"Verify that no local queries, views, or application logic depends on this foreign table before dropping.",
		cfg,
	)
}

// Foreign server lifecycle rules.

func newCreateForeignServerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGCreateForeignServerNotice, rule.LevelNotice, spec.DDLOperationCreateForeignServer,
		"foreign_server",
		"PostgreSQL foreign server %q created — review remote database connection",
		"CREATE SERVER defines a connection to an external database through a foreign data wrapper.",
		"Foreign servers define how PostgreSQL connects to external data sources. Misconfigured servers may expose connection details or access unintended data.",
		"Confirm the foreign data wrapper, connection options, and user mappings are appropriate for the intended use.",
		cfg,
	)
}

func newAlterForeignServerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGAlterForeignServerNotice, rule.LevelNotice, spec.DDLOperationAlterForeignServer,
		"foreign_server",
		"PostgreSQL foreign server %q altered — review connection configuration change",
		"ALTER SERVER modifies the configuration of a foreign server, including connection options and version.",
		"Changes to server options may affect all foreign tables and user mappings that reference this server.",
		"Verify that existing foreign tables and user mappings still function correctly after the change.",
		cfg,
	)
}

func newDropForeignServerWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGDropForeignServerWarn, rule.LevelWarning, spec.DDLOperationDropForeignServer,
		"foreign_server",
		"PostgreSQL foreign server %q dropped — all dependent foreign objects invalidated",
		"DROP SERVER removes the server definition, invalidating all associated foreign tables and user mappings.",
		"All foreign tables and user mappings that depend on this server will become unusable. The remote data is not affected.",
		"Verify all dependent foreign tables and user mappings have been migrated or are no longer needed.",
		cfg,
	)
}

// User mapping lifecycle rules.

func newCreateUserMappingNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGCreateUserMappingNotice, rule.LevelNotice, spec.DDLOperationCreateUserMapping,
		"user_mapping",
		"PostgreSQL user mapping %q created — review remote authentication mapping",
		"CREATE USER MAPPING associates a local role with authentication credentials for a foreign server.",
		"User mappings control which local roles can access which foreign servers and with what credentials. Misconfigured mappings may grant unintended remote access.",
		"Confirm the mapped role, server, and authentication options are appropriate for the intended access pattern.",
		cfg,
	)
}

func newAlterUserMappingNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGAlterUserMappingNotice, rule.LevelNotice, spec.DDLOperationAlterUserMapping,
		"user_mapping",
		"PostgreSQL user mapping %q altered — review authentication mapping change",
		"ALTER USER MAPPING modifies the authentication options for an existing user-to-server mapping.",
		"Changes to user mapping options may alter how a local role authenticates to the remote server.",
		"Verify the updated mapping still provides appropriate access to the foreign server.",
		cfg,
	)
}

func newDropUserMappingWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGDropUserMappingWarn, rule.LevelWarning, spec.DDLOperationDropUserMapping,
		"user_mapping",
		"PostgreSQL user mapping %q dropped — remote access revoked for mapped role",
		"DROP USER MAPPING removes the authentication mapping, preventing the local role from accessing the foreign server.",
		"The mapped role will no longer be able to query foreign tables on the affected server.",
		"Verify the role no longer needs access to foreign tables through this server.",
		cfg,
	)
}

// Foreign data wrapper lifecycle rules.

func newCreateForeignDataWrapperNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGCreateForeignDataWrapperNotice, rule.LevelNotice, spec.DDLOperationCreateForeignDataWrapper,
		"foreign_data_wrapper",
		"PostgreSQL foreign data wrapper %q created — review external data access extension",
		"CREATE FOREIGN DATA WRAPPER registers a new FDW that provides access to external data sources.",
		"FDWs extend the database with the ability to connect to external systems. The handler and validator functions run with elevated privileges.",
		"Confirm the FDW handler and validator functions are from a trusted source and appropriate for production use.",
		cfg,
	)
}

func newAlterForeignDataWrapperNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGAlterForeignDataWrapperNotice, rule.LevelNotice, spec.DDLOperationAlterForeignDataWrapper,
		"foreign_data_wrapper",
		"PostgreSQL foreign data wrapper %q altered — review external data access configuration change",
		"ALTER FOREIGN DATA WRAPPER modifies the handler, validator, or options of an existing FDW.",
		"Changes to FDW configuration affect all servers and foreign tables that use this wrapper.",
		"Verify existing foreign servers and tables still function correctly after the FDW change.",
		cfg,
	)
}

func newDropForeignDataWrapperWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGForeignObjectLifecycleRule(
		ruleIDPGDropForeignDataWrapperWarn, rule.LevelWarning, spec.DDLOperationDropForeignDataWrapper,
		"foreign_data_wrapper",
		"PostgreSQL foreign data wrapper %q dropped — all dependent foreign objects invalidated",
		"DROP FOREIGN DATA WRAPPER removes the FDW, invalidating all associated servers, foreign tables, and user mappings.",
		"All foreign data access through this wrapper will fail. The remote data is not affected.",
		"Verify all dependent servers, foreign tables, and user mappings have been migrated or are no longer needed.",
		cfg,
	)
}
