// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL function and procedure lifecycle DDL operations
// output: findings for PostgreSQL function/procedure lifecycle risks including SECURITY DEFINER and OR REPLACE
// pos: PostgreSQL-specific function/procedure lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// pgFunctionLifecycleRule handles CREATE FUNCTION / CREATE PROCEDURE lifecycle notices.
type pgFunctionLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func (r pgFunctionLifecycleRule) ID() string { return r.id }

func (r pgFunctionLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgFunctionLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: map[string]any{
			"operation":   string(r.operation),
			"object_type": r.object,
			"object_name": objectName,
		},
	}}, nil
}

// pgFunctionSecurityDefinerRule fires when a CREATE FUNCTION uses SECURITY DEFINER.
type pgFunctionSecurityDefinerRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func (r pgFunctionSecurityDefinerRule) ID() string { return r.id }

func (r pgFunctionSecurityDefinerRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Options["security_definer"] == "true"
}

func (r pgFunctionSecurityDefinerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: map[string]any{
			"operation":        string(r.operation),
			"object_type":      r.object,
			"object_name":      objectName,
			"security_definer": "true",
		},
	}}, nil
}

// pgCreateOrReplaceFunctionRule fires when CREATE OR REPLACE FUNCTION is used.
type pgCreateOrReplaceFunctionRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func (r pgCreateOrReplaceFunctionRule) ID() string { return r.id }

func (r pgCreateOrReplaceFunctionRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Options["replace"] == "true"
}

func (r pgCreateOrReplaceFunctionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: map[string]any{
			"operation":   string(r.operation),
			"object_type": r.object,
			"object_name": objectName,
			"replace":     "true",
		},
	}}, nil
}

// pgDropFunctionProcedureRule handles DROP FUNCTION / DROP PROCEDURE notices.
type pgDropFunctionProcedureRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func (r pgDropFunctionProcedureRule) ID() string { return r.id }

func (r pgDropFunctionProcedureRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgDropFunctionProcedureRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: map[string]any{
			"operation":   string(r.operation),
			"object_type": r.object,
			"object_name": objectName,
		},
	}}, nil
}

func newCreateFunctionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgFunctionLifecycleRule{
		id:         ruleIDPGCreateFunctionNotice,
		level:      configuredLevel(cfg, rule.LevelNotice),
		operation:  spec.DDLOperationCreateFunction,
		object:     "function",
		message:    "CREATE FUNCTION %q defines a new routine on PostgreSQL",
		why:        "Creating a function introduces a new database object that application code, triggers, or views may depend on.",
		risk:       "New functions change the database's executable surface area. Poorly written functions can cause performance degradation, security issues, or unexpected side effects in transaction management.",
		suggestion: "Review the function's volatility category (IMMUTABLE, STABLE, VOLATILE) and ensure parameter types match calling conventions. Verify that the function does not perform unsafe operations.",
	}, nil
}

func newCreateFunctionSecurityDefinerWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgFunctionSecurityDefinerRule{
		id:         ruleIDPGCreateFunctionSecurityDefinerWarn,
		level:      configuredLevel(cfg, rule.LevelWarning),
		operation:  spec.DDLOperationCreateFunction,
		object:     "function",
		message:    "CREATE FUNCTION %q with SECURITY DEFINER executes with elevated privileges on PostgreSQL",
		why:        "SECURITY DEFINER functions run with the privileges of the function owner rather than the calling user, bypassing the caller's normal permission boundaries.",
		risk:       "A SECURITY DEFINER function can perform operations the caller cannot do directly. If the function has SQL injection vulnerabilities or inadequate input validation, attackers can escalate privileges.",
		suggestion: "Prefer SECURITY INVOKER (the default). If SECURITY DEFINER is necessary, ensure all inputs are validated, use EXECUTE granted only to authorized roles, and audit the function body for privilege escalation paths.",
	}, nil
}

func newCreateOrReplaceFunctionAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateOrReplaceFunctionRule{
		id:         ruleIDPGCreateOrReplaceFunctionAdvisory,
		level:      configuredLevel(cfg, rule.LevelNotice),
		operation:  spec.DDLOperationCreateFunction,
		object:     "function",
		message:    "CREATE OR REPLACE FUNCTION %q silently replaces an existing function body on PostgreSQL",
		why:        "OR REPLACE silently overwrites the function body without dropping dependent objects, which can change behavior for all callers immediately.",
		risk:       "Existing callers will see the new behavior without any migration step. If the replacement changes parameter types, return types, or volatility, downstream consumers may experience runtime errors or incorrect results.",
		suggestion: "Consider explicit DROP + CREATE in a transaction to make the change visible in migration history. If using OR REPLACE, verify that parameter signatures and return types are unchanged.",
	}, nil
}

func newDropFunctionAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgDropFunctionProcedureRule{
		id:         ruleIDPGDropFunctionAdvisory,
		level:      configuredLevel(cfg, rule.LevelNotice),
		operation:  spec.DDLOperationDropFunction,
		object:     "function",
		message:    "DROP FUNCTION %q removes a routine that may have active dependents on PostgreSQL",
		why:        "Dropping a function that is referenced by views, triggers, other functions, or application code will cause those dependents to fail at runtime.",
		risk:       "Any view, trigger, CHECK constraint, or application query calling this function will raise an error. If CASCADE is used, dependent objects are silently destroyed.",
		suggestion: "Before dropping: 1) Query pg_depend to find all objects referencing this function. 2) Update or drop dependents first. 3) Deploy the drop during a maintenance window with rollback readiness.",
	}, nil
}

func newCreateProcedureNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgFunctionLifecycleRule{
		id:         ruleIDPGCreateProcedureNotice,
		level:      configuredLevel(cfg, rule.LevelNotice),
		operation:  spec.DDLOperationCreateProcedure,
		object:     "procedure",
		message:    "CREATE PROCEDURE %q defines a new routine on PostgreSQL",
		why:        "Creating a procedure introduces a new database object that can manage transactions (COMMIT/ROLLBACK) within its body, unlike functions.",
		risk:       "Procedures can control transaction flow, which introduces complexity in session management and can interact unexpectedly with application-level transaction management.",
		suggestion: "Review the procedure's transaction control logic (COMMIT/ROLLBACK within the body) and ensure it does not conflict with the application's transaction management. Prefer functions for read-only or compute-only operations.",
	}, nil
}

func newDropProcedureAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgDropFunctionProcedureRule{
		id:         ruleIDPGDropProcedureAdvisory,
		level:      configuredLevel(cfg, rule.LevelNotice),
		operation:  spec.DDLOperationDropProcedure,
		object:     "procedure",
		message:    "DROP PROCEDURE %q removes a routine that may have active dependents on PostgreSQL",
		why:        "Dropping a procedure that is called by application code or other routines will cause runtime failures.",
		risk:       "Any application or routine calling this procedure will raise an error. If CASCADE is used, dependent objects are silently destroyed.",
		suggestion: "Before dropping: 1) Search the codebase for CALL statements referencing this procedure. 2) Update or remove those references first. 3) Deploy the drop during a maintenance window with rollback readiness.",
	}, nil
}
