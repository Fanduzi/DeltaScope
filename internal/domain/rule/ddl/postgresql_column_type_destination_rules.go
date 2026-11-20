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
	if reason, fact := spec.PGColumnTypeDestinationPremises(s); reason != "" || fact != "" {
		return reason, fact
	}
	if spec.PGColumnTypeOnlyParent(s) {
		return "column_type_requires_child_traversal", ""
	}
	return assessPGColumnTypeDependency(s)
}

func assessPGColumnTypeDependency(s spec.Statement) (reason, missing string) {
	column := spec.PGColumnDependencyTarget(s)
	facts, table := s.Metadata.PGDropColumnDependency, s.DDL.Table
	if column == "" || facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Column != column {
		return "", "column.pg_type_dependencies"
	}
	switch facts.Kind {
	case "view", "generated_column":
		return "column_type_has_direct_dependency", ""
	case "", "foreign_key":
		return "", ""
	}
	return "", "column.pg_type_dependencies"
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
	case "destination_character_type_modifier_invalid":
		message = "The PostgreSQL destination character type requires one length between 1 and 10485760"
	case "destination_numeric_type_modifier_invalid":
		message = "The PostgreSQL numeric destination requires precision from 1 to 1000 and optional scale from -1000 to 1000"
	case "destination_type_assignment_unavailable":
		message = "PostgreSQL has no assignment cast from the selected column type to the destination type"
	case "column_type_has_direct_dependency":
		message = "A PostgreSQL view, materialized view or generated column prevents changing the selected column type"
	case "column_type_requires_child_traversal":
		message = "PostgreSQL requires changing the column type in the child tables too"
	case "destination_collation_absent":
		message = "The qualified PostgreSQL collation is unavailable for this database encoding"
	case "destination_type_not_collatable":
		message = "The PostgreSQL destination type does not accept COLLATE"
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
