package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCollationChangeRule struct{ level rule.Level }

func newPGCollationChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCollationChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCollationChangeRule) ID() string { return ruleIDPGCollationChangeValid }

func (r pgCollationChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterCollation && s.DDL.PGCollationChange != nil
}

func assessPGCollationChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGCollationChangeLookup(s) == nil {
		return "", "collation.target_identity"
	}
	state := s.Metadata.PGCollationChange
	if state == nil || state.Target != *s.DDL.PGCollationChange {
		return "", "collation.encoding_identity"
	}
	if state.Target.Action == "set_owner" {
		switch {
		case state.Target.Owner.Kind == "public":
			return "collation_change_owner_public", ""
		case state.Target.Owner.Kind == "named" && !state.OwnerExists:
			return "collation_change_owner_absent", ""
		case !state.OwnerExists:
			return "", "collation.owner_identity"
		}
	}
	if !state.SourceKnown {
		return "", "collation.encoding_identity"
	}
	if !state.SourceExists {
		return "collation_change_source_absent", ""
	}
	switch state.Target.Action {
	case "set_owner":
		return "", ""
	case "set_schema":
		switch {
		case !state.DestinationExists:
			return "collation_change_schema_absent", ""
		case state.SameNamespace:
			return "", ""
		case state.NamespaceUnsupported:
			return "collation_change_schema_inapplicable", ""
		case state.DestinationOccupied:
			return "collation_change_destination_occupied", ""
		}
	case "rename":
		if state.DestinationOccupied {
			return "collation_change_destination_occupied", ""
		}
	default:
		return "", "collation.change_action"
	}
	return "", ""
}

func (r pgCollationChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCollationChange(s)
	var message string
	switch reason {
	case "collation_change_source_absent":
		message = "The PostgreSQL collation does not exist for this database encoding"
	case "collation_change_owner_public":
		message = "PostgreSQL does not permit PUBLIC as a collation owner"
	case "collation_change_owner_absent":
		message = "The requested PostgreSQL collation owner does not exist"
	case "collation_change_schema_absent":
		message = "The requested PostgreSQL collation destination schema does not exist"
	case "collation_change_schema_inapplicable":
		message = "PostgreSQL disallows moving collations into or out of temporary or TOAST schemas"
	case "collation_change_destination_occupied":
		message = "The PostgreSQL collation destination name is already in use"
	default:
		return nil, nil
	}
	target := s.DDL.PGCollationChange.Source
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "collation": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCollationChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCollationChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_collation_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
