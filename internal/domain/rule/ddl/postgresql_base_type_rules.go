package ddl

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgBaseTypeRule struct{ level rule.Level }

func newPGBaseTypeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgBaseTypeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgBaseTypeRule) ID() string { return ruleIDPGBaseTypeValid }

func (r pgBaseTypeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.PGBaseType != nil
}

func assessPGBaseType(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGBaseType
	if spec.PGTypeDeclarationLookup(s) == nil {
		return "", "type.base_namespace"
	}
	state := s.Metadata.PGEnum
	if state == nil || state.Target != decl.Target {
		return "", "type.base_catalog"
	}
	if !state.SchemaExists {
		return "base_type_schema_absent", ""
	}
	switch decl.Action {
	case "create_shell":
		if state.NameOccupied || state.TypeExists && !state.Defined {
			return "base_type_name_occupied", ""
		}
		return "", ""
	case "create_base":
		if state.NameOccupied {
			return "base_type_name_occupied", ""
		}
		if !state.TypeExists || state.Defined {
			return "base_type_requires_shell", ""
		}
		seen := make(map[string]bool, len(decl.Options))
		for _, option := range decl.Options {
			name := option.Name
			if name == "analyse" {
				name = "analyze"
			}
			switch name {
			case "like", "internallength", "input", "output", "receive", "send", "typmod_in", "typmod_out", "analyze", "subscript",
				"category", "preferred", "delimiter", "element", "default", "passedbyvalue", "alignment", "storage", "collatable":
				if seen[name] {
					return "base_type_option_duplicate", ""
				}
				seen[name] = true
			}
		}
		if !seen["input"] || !seen["output"] {
			return "base_type_io_required", ""
		}
		return "", "type.base_io_and_representation"
	case "set_properties":
		return assessPGBaseTypeProperties(decl, state)
	default:
		return "", "type.base_action"
	}
}

func assessPGBaseTypeProperties(decl *spec.PGBaseTypeDeclaration, state *spec.PGEnumState) (reason, fact string) {
	if !state.TypeExists || !state.Defined {
		return "base_type_source_unresolved", ""
	}
	for _, option := range decl.Options {
		switch option.Name {
		case "storage":
			if !option.ValueKnown || state.Storage == "" {
				return "", "type.base_storage"
			}
			switch strings.ToLower(option.Storage) {
			case "plain":
				if state.Storage != "p" {
					return "base_type_cannot_change_to_plain", ""
				}
			case "external", "extended", "main":
				if !state.VariableLength {
					return "base_type_fixed_storage", ""
				}
			default:
				return "base_type_storage_unknown", ""
			}
		case "receive", "send", "typmod_in", "typmod_out", "analyze", "subscript":
			fact = "type.base_support_functions"
		default:
			return "base_type_property_inapplicable", ""
		}
	}
	if state.TypeKind != "b" || state.TrueArray {
		return "base_type_property_wrong_kind", ""
	}
	return "", fact
}

func (r pgBaseTypeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGBaseType(s)
	messages := map[string]string{
		"base_type_schema_absent":          "The PostgreSQL type schema does not exist",
		"base_type_name_occupied":          "The PostgreSQL type name is occupied",
		"base_type_requires_shell":         "PostgreSQL requires a shell type before completing the base type",
		"base_type_option_duplicate":       "A PostgreSQL base type option was specified more than once",
		"base_type_io_required":            "PostgreSQL requires input and output functions for a base type",
		"base_type_source_unresolved":      "The PostgreSQL type could not be resolved to a defined type",
		"base_type_cannot_change_to_plain": "PostgreSQL cannot change this type's storage to PLAIN",
		"base_type_fixed_storage":          "PostgreSQL fixed-size types require PLAIN storage",
		"base_type_storage_unknown":        "PostgreSQL does not recognize this type storage strategy",
		"base_type_property_inapplicable":  "PostgreSQL does not allow setting this type property",
		"base_type_property_wrong_kind":    "PostgreSQL requires a non-array base type for this property change",
	}
	message := messages[reason]
	if message == "" {
		return nil, nil
	}
	target := s.DDL.PGBaseType.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgBaseTypeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGBaseType(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_base_type", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
