package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateSchemaTargetRule struct{ level rule.Level }

func newPGCreateSchemaTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateSchemaTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateSchemaTargetRule) ID() string { return ruleIDPGCreateSchemaTargetValid }

func (r pgCreateSchemaTargetRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateSchema && s.DDL.PGCreateSchema != nil
}

func assessPGCreateSchemaTarget(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGCreateSchemaLookup(s) == nil {
		return "", "schema.target_identity"
	}
	target := s.DDL.PGCreateSchema
	state := s.Metadata.PGCreateSchema
	if state == nil || state.Target != *target {
		return "", "schema.creation_state"
	}
	switch {
	case target.Owner.Kind == "public":
		return "schema_owner_public_inapplicable", ""
	case target.Owner.Kind == "named" && !state.OwnerExists:
		return "schema_owner_absent", ""
	case target.Owner.Kind != "" && !state.OwnerExists:
		return "", "schema.owner_identity"
	case !state.NameKnown:
		return "", "schema.target_identity"
	case state.Reserved && !state.SystemSchemaSettingKnown:
		return "", "schema.system_name_setting"
	case state.Reserved && !state.SystemSchemaAllowed:
		return "schema_name_reserved", ""
	case state.Exists && s.DDL.Options["if_not_exists"] != "true":
		return "schema_target_exists", ""
	}
	return "", ""
}

func (r pgCreateSchemaTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateSchemaTarget(s)
	var message string
	switch reason {
	case "schema_owner_public_inapplicable":
		message = "PostgreSQL does not permit the PUBLIC pseudo-role as a schema owner"
	case "schema_owner_absent":
		message = "The requested PostgreSQL schema owner does not exist"
	case "schema_name_reserved":
		message = "The requested PostgreSQL schema name uses the reserved pg_ prefix"
	case "schema_target_exists":
		message = "The requested PostgreSQL schema already exists"
	default:
		return nil, nil
	}
	metadata := map[string]any{"reason_code": reason}
	if name := s.DDL.PGCreateSchema.Name; name != "" {
		metadata["schema"] = name
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: metadata}}, nil
}

func (r pgCreateSchemaTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateSchemaTarget(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_schema", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
