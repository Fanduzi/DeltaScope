package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgSequenceChangeRule struct{ level rule.Level }

func newPGSequenceChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgSequenceChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgSequenceChangeRule) ID() string { return ruleIDPGSequenceChangeValid }

func (r pgSequenceChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.PGSequenceChange != nil
}

func assessPGSequenceChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGSequenceChange
	if spec.PGSequenceChangeLookup(s) == nil {
		return "", "sequence.target_identity"
	}
	state := s.Metadata.PGSequenceChange
	if state == nil || state.Target != decl.Target || state.RenameTo != decl.RenameTo || state.MoveTo != decl.MoveTo {
		return "", "sequence.target_catalog"
	}
	create := s.DDL.Operation == spec.DDLOperationCreateSequence
	if create && decl.IfNotExists {
		if !state.SchemaExists {
			return "sequence_schema_absent", ""
		}
		if decl.Persistence == "t" {
			return "sequence_temporary_namespace", ""
		}
		if state.Kind != "" {
			return "", ""
		}
	}
	if !create {
		if state.Kind == "" && decl.IfExists {
			return "", ""
		}
		switch state.Kind {
		case "":
			return "sequence_source_absent", ""
		case "S":
		default:
			return "sequence_source_wrong_kind", ""
		}
	}
	if decl.RenameTo != "" || decl.MoveTo != "" {
		return assessPGSequenceDestination(decl, state), ""
	}
	reason, fact = assessPGSequenceOptions(decl, state, create)
	if reason != "" || fact != "" {
		return reason, fact
	}
	if create {
		switch {
		case !state.SchemaExists:
			return "sequence_schema_absent", ""
		case decl.Persistence == "t":
			return "sequence_temporary_namespace", ""
		case state.Kind != "":
			return "sequence_destination_occupied", ""
		}
	}
	return assessPGSequenceOwnership(decl, state, create)
}

func assessPGSequenceDestination(decl *spec.PGSequenceChange, state *spec.PGSequenceChangeState) string {
	if decl.MoveTo != "" {
		switch {
		case state.Owned:
			return "sequence_move_owned"
		case !state.DestinationSchema:
			return "sequence_destination_schema_absent"
		case state.DestinationForbidden:
			return "sequence_destination_namespace_forbidden"
		case decl.MoveTo == decl.Target.Table.Schema:
			return ""
		}
	}
	if state.DestinationOccupied {
		return "sequence_destination_occupied"
	}
	return ""
}

func assessPGSequenceOwnership(decl *spec.PGSequenceChange, state *spec.PGSequenceChangeState, create bool) (reason, fact string) {
	owned := decl.Ownership()
	if len(owned) == 0 {
		return "", ""
	}
	if len(owned) == 1 && owned[0] != "none" {
		return "sequence_owned_by_invalid", ""
	}
	if len(owned) != 1 && len(owned) != 3 {
		return "", "sequence.ownership"
	}
	if create && len(owned) == 1 {
		return "", ""
	}
	owner := state.Ownership
	if owner == nil || owner.Sequence != decl.Target.Table {
		return "", "sequence.ownership"
	}
	if len(owned) == 3 {
		if owner.Table != (spec.Table{Schema: owned[0], Name: owned[1]}) || owner.Column != owned[2] {
			return "", "sequence.ownership"
		}
		switch owner.Kind {
		case "":
			return "sequence_owner_relation_absent", ""
		case "r", "f", "v", "p":
		default:
			return "sequence_owner_relation_kind", ""
		}
		switch {
		case !owner.OwnerMatches:
			return "sequence_owner_role_mismatch", ""
		case owner.Table.Schema != decl.Target.Table.Schema:
			return "sequence_owner_schema_mismatch", ""
		case !owner.ColumnExists:
			return "sequence_owner_column_absent", ""
		}
	} else if owner.Table != (spec.Table{}) || owner.Column != "" {
		return "", "sequence.ownership"
	}
	if !create && owner.IdentitySequence {
		return "sequence_identity_ownership", ""
	}
	return "", ""
}

func assessPGSequenceOptions(decl *spec.PGSequenceChange, state *spec.PGSequenceChangeState, create bool) (reason, fact string) {
	options := make(map[string]spec.PGSequenceChangeOption, len(decl.Options))
	for _, option := range decl.Options {
		if _, repeated := options[option.Name]; repeated {
			return "sequence_option_duplicate", ""
		}
		if option.Name == "sequence_name" {
			return "sequence_option_inapplicable", ""
		}
		options[option.Name] = option
	}
	width := state.Width
	var minValue, maxValue, start, increment int64
	if create {
		width, increment = 64, 1
	} else {
		d := state.Definition
		if width != 16 && width != 32 && width != 64 || d == nil || d.MinValue == nil || d.MaxValue == nil || d.Start == nil || d.Increment == nil {
			return "", "sequence.definition"
		}
		minValue, maxValue, start, increment = *d.MinValue, *d.MaxValue, *d.Start, *d.Increment
	}
	oldWidth, oldMin, oldMax := width, minValue, maxValue
	if option, ok := options["as"]; ok {
		if !option.Known {
			return "", "sequence.data_type"
		}
		if option.Width < 0 {
			return "sequence_type_invalid", ""
		}
		width = option.Width
	}
	if option, ok := options["increment"]; ok && option.Value != nil {
		increment = *option.Value
		if increment == 0 {
			return "sequence_increment_zero", ""
		}
	}
	for _, option := range decl.Options {
		if !option.Known {
			return "", "sequence.options"
		}
	}
	typeMin, typeMax := pgIdentityIntegerBounds(width)
	oldTypeMin, oldTypeMax := pgIdentityIntegerBounds(oldWidth)
	_, changeType := options["as"]
	resetMin, resetMax := changeType && oldMin == oldTypeMin, changeType && oldMax == oldTypeMax
	if option, ok := options["maxvalue"]; ok && option.Value != nil {
		maxValue = *option.Value
	} else if create || ok || resetMax {
		maxValue = typeMax
		if increment < 0 && !resetMax {
			maxValue = -1
		}
	}
	if option, ok := options["minvalue"]; ok && option.Value != nil {
		minValue = *option.Value
	} else if create || ok || resetMin {
		minValue = 1
		if increment < 0 || resetMin {
			minValue = typeMin
		}
	}
	if minValue < typeMin || maxValue > typeMax || minValue >= maxValue {
		return "sequence_bounds_invalid", ""
	}
	if option, ok := options["start"]; ok && option.Value != nil {
		start = *option.Value
	} else if create {
		start = minValue
		if increment < 0 {
			start = maxValue
		}
	}
	if start < minValue || start > maxValue {
		return "sequence_start_outside_bounds", ""
	}
	if option, ok := options["restart"]; ok {
		value := start
		if option.Value != nil {
			value = *option.Value
		}
		if value < minValue || value > maxValue {
			return "sequence_restart_outside_bounds", ""
		}
	} else if !create && (minValue > oldMin || maxValue < oldMax) {
		return "", "sequence.current_value"
	}
	if option := options["cache"]; option.Value != nil && *option.Value < 1 {
		return "sequence_cache_invalid", ""
	}
	return "", ""
}

func (r pgSequenceChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGSequenceChange(s)
	messages := map[string]string{
		"sequence_schema_absent":                   "The PostgreSQL sequence schema does not exist",
		"sequence_source_absent":                   "The PostgreSQL sequence does not exist",
		"sequence_source_wrong_kind":               "PostgreSQL requires a sequence for ALTER SEQUENCE",
		"sequence_destination_occupied":            "A PostgreSQL relation occupies the sequence creation target",
		"sequence_temporary_namespace":             "PostgreSQL requires a temporary namespace for this temporary sequence",
		"sequence_option_duplicate":                "A PostgreSQL sequence option was specified more than once",
		"sequence_option_inapplicable":             "PostgreSQL disallows this option on standalone sequences",
		"sequence_type_invalid":                    "PostgreSQL sequence data types must be smallint, integer or bigint",
		"sequence_increment_zero":                  "The PostgreSQL sequence increment must be nonzero",
		"sequence_bounds_invalid":                  "The PostgreSQL sequence bounds are invalid for this data type",
		"sequence_start_outside_bounds":            "The PostgreSQL sequence start is outside the declared bounds",
		"sequence_restart_outside_bounds":          "The PostgreSQL sequence restart is outside the declared bounds",
		"sequence_cache_invalid":                   "The PostgreSQL sequence cache must be positive",
		"sequence_owned_by_invalid":                "PostgreSQL requires OWNED BY table.column or OWNED BY NONE",
		"sequence_owner_relation_absent":           "The PostgreSQL sequence ownership relation does not exist",
		"sequence_owner_relation_kind":             "PostgreSQL does not allow this relation kind to own a sequence",
		"sequence_owner_role_mismatch":             "The PostgreSQL sequence and its ownership relation must have the same owner",
		"sequence_owner_schema_mismatch":           "The PostgreSQL sequence and its ownership relation must be in the same schema",
		"sequence_owner_column_absent":             "The PostgreSQL sequence ownership column does not exist",
		"sequence_identity_ownership":              "PostgreSQL does not allow changing the ownership link of an identity sequence",
		"sequence_move_owned":                      "PostgreSQL does not move an owned sequence independently of its table",
		"sequence_destination_schema_absent":       "The PostgreSQL sequence destination schema does not exist",
		"sequence_destination_namespace_forbidden": "PostgreSQL disallows this sequence namespace change",
	}
	message := messages[reason]
	if message == "" {
		return nil, nil
	}
	t := s.DDL.PGSequenceChange.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": t.Schema, "sequence": t.Name, "reason_code": reason,
	}}}, nil
}

func (r pgSequenceChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGSequenceChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_sequence_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
