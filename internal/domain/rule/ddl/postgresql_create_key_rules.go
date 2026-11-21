package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateKeyRule struct{ level rule.Level }

func newPGCreateKeyRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateKeyRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateKeyRule) ID() string { return ruleIDPGCreateKeysValid }

func (r pgCreateKeyRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateKeys != nil
}

func assessPGCreateKeys(s spec.Statement) (reason, gap string) {
	keys := s.DDL.PGCreateKeys
	if keys.Unbound {
		return "", "create_table.key_namespace"
	}
	reason, gap = spec.PGCreateKeyRefusal(s.DDL)
	if reason == "" {
		return "", gap
	}
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if !keys.Target.Qualified() {
		return "", "create_table.key_namespace"
	}
	state := s.Metadata.PGCreateDestination
	if state == nil || state.Target != keys.Target {
		return "", "create_table.key_destination"
	}
	switch {
	case !state.SchemaExists:
		return "create_keys_schema_absent", ""
	case keys.IfNotExists && state.RelationExists:
		return "", ""
	case reason == "create_keys_system_column" && state.RelationExists:
		return "create_keys_destination_occupied", ""
	}
	return reason, ""
}

func (r pgCreateKeyRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateKeys(s)
	var message string
	switch reason {
	case "create_keys_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_keys_destination_occupied":
		message = "A PostgreSQL relation already occupies the creation target"
	case "create_keys_multiple_primary":
		message = "PostgreSQL permits only one primary key declaration per table"
	case "create_keys_using_existing_index":
		message = "PostgreSQL cannot use an existing index in CREATE TABLE"
	case "create_keys_column_absent":
		message = "The PostgreSQL constraint references a column that does not exist"
	case "create_keys_column_duplicate":
		message = "The same column appears more than once in the PostgreSQL constraint key"
	case "create_keys_system_column":
		message = "PostgreSQL cannot create a constraint index on a system column"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateKeys.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateKeyRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateKeys(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_keys", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
