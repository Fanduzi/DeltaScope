package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCompositeRule struct{ level rule.Level }

func newPGCompositeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCompositeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCompositeRule) ID() string { return ruleIDPGCompositeValid }

func (r pgCompositeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		(s.DDL.Operation == spec.DDLOperationCreateType || s.DDL.Operation == spec.DDLOperationAlterType) && s.DDL.PGComposite != nil
}

func assessPGComposite(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGComposite
	if decl.Action == "" {
		return "", "composite_type.action_order"
	}
	if spec.PGCompositeLookup(s) == nil {
		return "", "composite_type.namespace"
	}
	state := s.Metadata.PGComposite
	if state == nil || state.Target != decl.Target {
		return "", "composite_type.catalog"
	}
	if !state.SchemaExists {
		return "composite_type_schema_absent", ""
	}
	if decl.Action == "create" {
		if state.TypeOccupied {
			return "composite_type_name_occupied", ""
		}
		seen := make(map[string]bool, len(decl.Attributes))
		for _, name := range decl.Attributes {
			if seen[name] {
				return "composite_attribute_duplicate", ""
			}
			seen[name] = true
		}
		if state.RelationKind != "" {
			return "composite_relation_occupied", ""
		}
		return "", ""
	}
	switch state.RelationKind {
	case "":
		return "composite_type_relation_absent", ""
	case "c":
	default:
		return "composite_type_wrong_relation_kind", ""
	}
	if state.Attributes == nil {
		return "", "composite_type.attributes"
	}
	found := slices.Contains(state.Attributes, decl.Name)
	switch {
	case decl.Action == "add_attribute":
		if found {
			return "composite_attribute_duplicate", ""
		}
	case !found:
		if !decl.IfExists {
			return "composite_attribute_absent", ""
		}
	case decl.Action == "rename_attribute" && slices.Contains(state.Attributes, decl.NewName):
		return "composite_attribute_duplicate", ""
	}
	return "", ""
}

func (r pgCompositeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGComposite(s)
	var message string
	switch reason {
	case "composite_type_schema_absent":
		message = "The PostgreSQL composite type schema does not exist"
	case "composite_type_name_occupied":
		message = "A PostgreSQL type already occupies the creation target"
	case "composite_relation_occupied":
		message = "A PostgreSQL relation already occupies the creation target"
	case "composite_type_relation_absent":
		message = "The PostgreSQL composite relation does not exist"
	case "composite_type_wrong_relation_kind":
		message = "PostgreSQL requires a standalone composite type for this attribute action"
	case "composite_attribute_duplicate":
		message = "The PostgreSQL composite type has a conflicting attribute name"
	case "composite_attribute_absent":
		message = "The requested PostgreSQL composite attribute does not exist"
	default:
		return nil, nil
	}
	target := s.DDL.PGComposite.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCompositeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGComposite(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_composite_type", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
