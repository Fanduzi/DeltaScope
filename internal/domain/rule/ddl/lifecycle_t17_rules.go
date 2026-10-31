// T17 database/schema lifecycle rules consume the ordered schema binding
// (Metadata.SchemaTarget): schema-existence premises for CREATE/ALTER/DROP
// DATABASE|SCHEMA synonyms and conditional clauses, the schema-name
// denylist for protected business databases, and the policy-level system
// schema protection the engines apply inconsistently (MySQL ≥8.0 refuses
// `mysql` natively while 5.7 and TiDB accept it — the rule answers
// uniformly). A nil SchemaTarget is the unvouched contract: no finding,
// one schema.existence gap.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// gapReasonUnknownSchemaState marks statements whose bound schema state the
// ordered batch view could not establish — no provider resolver, a stale
// provider answer after an in-batch touch, or a batch-internal unvouched
// outcome. It is the schema-namespace analogue of gapReasonUnknownTableState.
const gapReasonUnknownSchemaState = "unknown_schema_state"

// schemaStatementName resolves the schema identity the statement binds —
// its own ObjectName, or the request's trusted schema on the name-less
// ALTER DATABASE form (empty when the request carries none).
func schemaStatementName(statement spec.Statement) string {
	if statement.DDL == nil {
		return ""
	}
	if name := strings.ToLower(strings.TrimSpace(statement.DDL.ObjectName)); name != "" {
		return name
	}
	if statement.DDL.AlterDefaultSchema && statement.Metadata != nil {
		return strings.ToLower(strings.TrimSpace(statement.Metadata.Schema))
	}
	return ""
}

// schemaStatementBound reports whether the statement is a schema-scoped DDL
// on the database object type (CREATE/ALTER/DROP DATABASE|SCHEMA).
func schemaStatementBound(statement spec.Statement, ops ...spec.DDLOperation) bool {
	if statement.DDL == nil || statement.DDL.ObjectType != "database" {
		return false
	}
	for _, op := range ops {
		if statement.DDL.Operation == op {
			return true
		}
	}
	return false
}

// schemaExistenceRule adjudicates a schema-scoped statement against its
// ordered binding: create.exists.forbid fires on proven-present without IF
// NOT EXISTS (native 1007), alter/drop.exists.require fire on
// proven-absent without IF EXISTS (3503/1049/err-1 / 1008 by anchor).
// Conditional clauses arm the matching premise only — a proven match is a
// no-op, not a violation.
type schemaExistenceRule struct {
	ruleID       string
	level        rule.Level
	operation    spec.DDLOperation
	requireExist bool // true: fires when target proven-absent; false: fires when proven-present
	ifClause     string
	action       string
}

func newSchemaExistenceRule(ruleID string, operation spec.DDLOperation, requireExist bool, ifClause, action string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return schemaExistenceRule{
		ruleID:       ruleID,
		level:        configuredLevel(cfg, fallbackLevel),
		operation:    operation,
		requireExist: requireExist,
		ifClause:     ifClause,
		action:       action,
	}, nil
}

func (r schemaExistenceRule) ID() string { return r.ruleID }

func (r schemaExistenceRule) AppliesTo(statement spec.Statement) bool {
	return orderedGapDialect(statement.Dialect) &&
		statement.Kind == spec.KindDDL &&
		schemaStatementBound(statement, r.operation)
}

func (r schemaExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target := statement.MetadataSchemaTarget()
	if target == nil {
		return nil, nil
	}
	conditional := statement.DDL.Options[r.ifClause] == "true"
	name := schemaStatementName(statement)
	switch {
	case r.requireExist && !target.Exists && !conditional:
		return []rule.Finding{{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("%s target schema %q does not exist", r.action, name),
			Suggestion: "verify the schema name, create it first, or arm the IF EXISTS clause intentionally",
			Metadata:   map[string]any{"schema": name, "operation": string(r.operation), "exists": false},
		}}, nil
	case !r.requireExist && target.Exists && !conditional:
		return []rule.Finding{{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("%s target schema %q already exists", r.action, name),
			Suggestion: "rename the schema, drop it first, or arm the IF NOT EXISTS clause intentionally",
			Metadata:   map[string]any{"schema": name, "operation": string(r.operation), "exists": true},
		}}, nil
	}
	return nil, nil
}

func (r schemaExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	if statement.MetadataSchemaTarget() != nil {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonUnknownSchemaState,
		RequiredFacts: []string{"schema.existence"},
	}}
}

// databaseDenylistRule blocks schema-scoped statements whose bound schema
// name is policy-listed — the business-database protection list, keyed on
// the name the statement actually touches (a name-less ALTER resolves the
// request schema before matching).
type databaseDenylistRule struct {
	ruleID  string
	level   rule.Level
	schemas map[string]struct{}
}

func newDatabaseDenylistRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	schemas, err := normalizedStringSetParam(ruleID, cfg, "schemas", nil)
	if err != nil {
		return nil, err
	}
	return databaseDenylistRule{
		ruleID:  ruleID,
		level:   configuredLevel(cfg, fallbackLevel),
		schemas: schemas,
	}, nil
}

func (r databaseDenylistRule) ID() string { return r.ruleID }

func (r databaseDenylistRule) AppliesTo(statement spec.Statement) bool {
	return orderedGapDialect(statement.Dialect) && statement.Kind == spec.KindDDL &&
		schemaStatementBound(statement, spec.DDLOperationCreateSchema, spec.DDLOperationAlterSchema, spec.DDLOperationDropSchema)
}

func (r databaseDenylistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := schemaStatementName(statement)
	if name == "" {
		return nil, nil
	}
	if _, listed := r.schemas[name]; !listed {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("schema %q is blocked by the database denylist policy", name),
		Suggestion: "run the change against an allowed schema or relax the denylist policy intentionally",
		Metadata:   map[string]any{"schema": name, "operation": string(statement.DDL.Operation)},
	}}, nil
}

// schemaEncryptionApplicabilityRule reports the version boundary of the
// schema-level DEFAULT ENCRYPTION option: MySQL before 8.0.16 refuses it
// natively (probed 1064 on 5.7.44), TiDB accepts and ignores it (probed
// rc0 + warning 1064 — no SCHEMATA column exists). The ordered-state layer
// models the same boundary for batch transitions; this rule is the
// statement-visible answer. An unproven instance version leaves the
// applicability question open — EvidenceGaps owns it.
type schemaEncryptionApplicabilityRule struct {
	ruleID string
	level  rule.Level
}

func newSchemaEncryptionApplicabilityRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return schemaEncryptionApplicabilityRule{ruleID: ruleID, level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r schemaEncryptionApplicabilityRule) ID() string { return r.ruleID }

func (r schemaEncryptionApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	return orderedGapDialect(statement.Dialect) && statement.Kind == spec.KindDDL &&
		schemaStatementBound(statement, spec.DDLOperationCreateSchema, spec.DDLOperationAlterSchema) &&
		statement.DDL.Options["encryption"] != ""
}

func (r schemaEncryptionApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	version := statementVersionIdentity(statement)
	name := schemaStatementName(statement)
	if statement.Dialect == spec.DialectTiDB {
		// TiDB parses the clause and ignores it for every storage engine —
		// an informational answer, never the configured blocker level. The
		// answer does not depend on the instance version.
		metadata := map[string]any{"schema": name, "operation": string(statement.DDL.Operation), "ignored": true}
		if version != nil {
			metadata["version"] = version.Version
		}
		return []rule.Finding{{
			RuleID:     r.ruleID,
			Level:      rule.LevelNotice,
			Message:    fmt.Sprintf("DEFAULT ENCRYPTION on schema %q is parsed but ignored by TiDB storage engines", name),
			Suggestion: "drop the clause or treat it as a no-op; TiDB stores no schema encryption attribute",
			Metadata:   metadata,
		}}, nil
	}
	if version == nil {
		return nil, nil
	}
	if version.Major < 8 || (version.Major == 8 && version.Minor == 0 && version.Patch < 16) {
		return []rule.Finding{{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("DEFAULT ENCRYPTION requires MySQL 8.0.16 or newer; schema %q on %s refuses with error 1064", name, version.Version),
			Suggestion: "drop the ENCRYPTION clause or retarget the audit at MySQL 8.0.16 or newer",
			Metadata:   map[string]any{"schema": name, "operation": string(statement.DDL.Operation), "version": version.Version, "minimum_supported_version": "8.0.16"},
		}}, nil
	}
	return nil, nil
}

func (r schemaEncryptionApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || statement.Dialect != spec.DialectMySQL {
		return nil
	}
	if statementVersionIdentity(statement) != nil {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingTargetVersion,
		RequiredFacts: []string{"target.version"},
	}}
}

// reservedSystemSchemas are the engine-managed namespaces a lifecycle
// statement must not alter or drop — the policy-level system protection.
// MySQL ≥8.0 refuses `mysql` natively (3552) and rejects writes under
// information_schema (1044); MySQL 5.7 and TiDB accept several of these
// (probed: ALTER DATABASE mysql succeeds on both), so the rule answers
// uniformly rather than mirroring engine leniency. metrics_schema is
// TiDB-only and included unconditionally.
var reservedSystemSchemas = map[string]struct{}{
	"information_schema": {},
	"mysql":              {},
	"performance_schema": {},
	"sys":                {},
	"metrics_schema":     {},
}

// databaseSystemModifyRule blocks ALTER/DROP against reserved system
// schemas — CREATE of a system name is a duplicate-existence question the
// exists rules already own (every reserved schema is born present).
type databaseSystemModifyRule struct {
	ruleID string
	level  rule.Level
}

func newDatabaseSystemModifyRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return databaseSystemModifyRule{ruleID: ruleID, level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r databaseSystemModifyRule) ID() string { return r.ruleID }

func (r databaseSystemModifyRule) AppliesTo(statement spec.Statement) bool {
	return orderedGapDialect(statement.Dialect) && statement.Kind == spec.KindDDL &&
		schemaStatementBound(statement, spec.DDLOperationAlterSchema, spec.DDLOperationDropSchema)
}

func (r databaseSystemModifyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := schemaStatementName(statement)
	if _, reserved := reservedSystemSchemas[name]; !reserved {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("schema %q is a reserved system schema; %s on it is refused by policy", name, statement.DDL.Operation),
		Suggestion: "do not alter or drop engine-managed system schemas; scope the change to a business schema",
		Metadata:   map[string]any{"schema": name, "operation": string(statement.DDL.Operation), "system": true},
	}}, nil
}
