// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL extension lifecycle DDL operations
// output: findings for PostgreSQL extension creation, alteration, and drop risks
// pos: PostgreSQL-specific extension lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgExtensionLifecycleRule struct {
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

func newPGExtensionLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, optionKey, optionValue string, requireOption bool, object, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgExtensionLifecycleRule{
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

func (r pgExtensionLifecycleRule) ID() string { return r.id }

func (r pgExtensionLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgExtensionLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"if_not_exists", "schema", "version", "cascade", "if_exists", "action", "new_schema", "member_type", "member"} {
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

func newCreateExtensionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGCreateExtensionNotice, rule.LevelNotice, spec.DDLOperationCreateExtension,
		"", "", false, "extension",
		"PostgreSQL extension %q created — review server-side dependency change",
		"CREATE EXTENSION introduces server-side functionality that application code, indexes, or schema objects may depend on.",
		"Extensions change the available functions, operators, and index methods on the database server, affecting portability and upgrade paths.",
		"Confirm the extension is required and that target environments have it available before deployment.",
		cfg,
	)
}

func newCreateExtensionCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGCreateExtensionCascadeWarn, rule.LevelWarning, spec.DDLOperationCreateExtension,
		"cascade", "true", true, "extension",
		"CREATE EXTENSION %q CASCADE installs dependent extensions automatically",
		"CASCADE tells PostgreSQL to automatically install extensions that the target extension depends on.",
		"Transitive dependency extensions may introduce unexpected server-side changes or version conflicts across environments.",
		"Prefer explicit CREATE EXTENSION statements for each dependency. If CASCADE is intentional, review the full dependency tree.",
		cfg,
	)
}

func newAlterExtensionUpdateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGAlterExtensionUpdateNotice, rule.LevelNotice, spec.DDLOperationAlterExtension,
		"action", "update", true, "extension",
		"PostgreSQL extension %q updated — review version change impact",
		"ALTER EXTENSION UPDATE changes the extension version, which may alter available functions, operators, or behavior.",
		"Version upgrades can introduce breaking changes in function signatures, default behaviors, or deprecate previously stable features.",
		"Review the extension changelog and test application behavior against the new version before deployment.",
		cfg,
	)
}

func newAlterExtensionSetSchemaNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGAlterExtensionSetSchemaNotice, rule.LevelNotice, spec.DDLOperationAlterExtension,
		"action", "set_schema", true, "extension",
		"PostgreSQL extension %q moved to a different schema — qualified references may break",
		"ALTER EXTENSION SET SCHEMA changes the schema where the extension's objects reside.",
		"Existing qualified references to extension objects will fail if not updated to the new schema path.",
		"Update all qualified references to extension objects and verify that search_path settings still resolve them correctly.",
		cfg,
	)
}

func newDropExtensionAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGDropExtensionAdvisory, rule.LevelWarning, spec.DDLOperationDropExtension,
		"", "", false, "extension",
		"PostgreSQL extension %q dropped — dependent objects may break",
		"DROP EXTENSION removes server-side functionality that functions, indexes, or application code may depend on.",
		"Dependent objects using extension-provided types, functions, or operators will fail immediately after the extension is removed.",
		"Verify no schema objects or application code depend on this extension before dropping it.",
		cfg,
	)
}

func newDropExtensionCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGDropExtensionCascadeWarn, rule.LevelWarning, spec.DDLOperationDropExtension,
		"cascade", "true", true, "extension",
		"DROP EXTENSION %q CASCADE may remove dependent objects on PostgreSQL",
		"CASCADE tells PostgreSQL to drop objects that depend on the extension.",
		"Functions, indexes, views, tables, or constraints depending on the extension may be removed unexpectedly.",
		"Prefer RESTRICT during review. If CASCADE is intentional, enumerate dependent objects and confirm the blast radius.",
		cfg,
	)
}

func newAlterExtensionAddMemberNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGAlterExtensionAddMemberNotice, rule.LevelNotice, spec.DDLOperationAlterExtension,
		"action", "add_member", true, "extension",
		"PostgreSQL extension %q member added — review extension dependency change",
		"ALTER EXTENSION ADD member includes an existing database object into the extension, making it dependent on the extension.",
		"Objects added to an extension become owned by the extension and will be dropped if the extension is dropped.",
		"Confirm the object membership is intentional and that dependent applications account for the coupling.",
		cfg,
	)
}

func newAlterExtensionDropMemberWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGExtensionLifecycleRule(
		ruleIDPGAlterExtensionDropMemberWarn, rule.LevelWarning, spec.DDLOperationAlterExtension,
		"action", "drop_member", true, "extension",
		"PostgreSQL extension %q member removed — review decoupled object",
		"ALTER EXTENSION DROP member decouples an object from the extension, making it independent again.",
		"Removing a member may leave orphaned objects that were previously managed by the extension lifecycle.",
		"Verify that the decoupled object no longer depends on extension-provided types or functions.",
		cfg,
	)
}
