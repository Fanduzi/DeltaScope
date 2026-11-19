package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnTypeDestinationRule struct{ level rule.Level }

func newPGColumnTypeDestinationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTypeDestinationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgColumnTypeDestinationRule) ID() string { return ruleIDPGAlterColumnTypeDestinationValid }

func (r pgColumnTypeDestinationRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, "set_data_type")
}

func assessPGColumnTypeDestination(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	sourceFailure, known := spec.PGColumnTypeSource(s)
	if !known {
		return "", "target_table.pg_columns"
	}
	if sourceFailure != "" {
		return "", ""
	}
	target := s.DDL.Alter[0].PGColumnType
	if !target.Qualified() {
		return "", "column.type_destination_identity"
	}
	observed := s.Metadata.PGColumnType
	if observed == nil || observed.Target != target {
		return "", "column.pg_type_destination"
	}
	switch {
	case !observed.SchemaExists:
		return "destination_type_schema_absent", ""
	case !observed.BaseExists:
		return "destination_type_absent", ""
	case target.Array && !observed.Resolved:
		return "destination_array_type_absent", ""
	case !observed.Resolved:
		return "", "column.pg_type_destination"
	case !observed.Defined:
		return "destination_type_undefined", ""
	case observed.Pseudo:
		return "destination_pseudo_type_inapplicable", ""
	}
	return "", ""
}

func (r pgColumnTypeDestinationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGColumnTypeDestination(s)
	var message string
	switch reason {
	case "destination_type_schema_absent":
		message = "The PostgreSQL destination type schema does not exist"
	case "destination_type_absent":
		message = "The qualified PostgreSQL destination type does not exist"
	case "destination_array_type_absent":
		message = "The PostgreSQL destination type has no associated array type"
	case "destination_type_undefined":
		message = "The PostgreSQL destination is an undefined shell type"
	case "destination_pseudo_type_inapplicable":
		message = "PostgreSQL does not permit this destination pseudo-type in a table column"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"action": "set_data_type", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"column": s.DDL.Alter[0].Name, "reason_code": reason},
	}}, nil
}

func (r pgColumnTypeDestinationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGColumnTypeDestination(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_column_type_destination", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
