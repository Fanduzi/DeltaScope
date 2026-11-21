package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateInheritanceRule struct{ level rule.Level }

func newPGCreateInheritanceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateInheritanceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateInheritanceRule) ID() string { return ruleIDPGCreateInheritanceValid }

func (r pgCreateInheritanceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateInheritance != nil
}

func assessPGCreateInheritance(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGCreateInheritanceLookup(s) == nil {
		return "", "create_table.inheritance_namespace"
	}
	decl := s.DDL.PGCreateInheritance
	state := s.Metadata.PGCreateInheritance
	if state == nil || state.Target != decl.Target || len(state.Sources) != len(decl.Parents) {
		return "", "create_table.inheritance_catalog"
	}
	if !state.SchemaExists {
		return "create_inheritance_schema_absent", ""
	}
	if state.Exists && decl.IfNotExists {
		return "", ""
	}
	seen := make(map[spec.PGDestructiveTableTarget]bool, len(decl.Parents))
	for i, parent := range decl.Parents {
		source := state.Sources[i]
		if !parent.Qualified() || source.Target != parent {
			return "", "create_table.inheritance_parent_identity"
		}
		if source.Kind == "" {
			return "create_inheritance_parent_absent", ""
		}
		if seen[parent] {
			return "create_inheritance_parent_duplicate", ""
		}
		seen[parent] = true
	}
	for _, source := range state.Sources {
		if source.Kind != "r" && source.Kind != "f" {
			return "create_inheritance_parent_wrong_kind", ""
		}
		if source.Partition == nil {
			return "", "create_table.inheritance_parent_partition"
		}
		if *source.Partition {
			return "create_inheritance_parent_partition", ""
		}
	}
	return "", "create_table.inheritance_column_merge"
}

func (r pgCreateInheritanceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateInheritance(s)
	var message string
	switch reason {
	case "create_inheritance_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_inheritance_parent_absent":
		message = "A PostgreSQL inheritance parent does not exist"
	case "create_inheritance_parent_duplicate":
		message = "PostgreSQL cannot inherit from the same table more than once"
	case "create_inheritance_parent_wrong_kind":
		message = "A PostgreSQL inheritance parent has an inapplicable relation kind"
	case "create_inheritance_parent_partition":
		message = "PostgreSQL cannot use a partition as a regular inheritance parent"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateInheritance.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateInheritanceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateInheritance(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_inheritance", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
