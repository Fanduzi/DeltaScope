package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgForeignKeyTargetRule struct{ level rule.Level }

func newPGForeignKeyTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgForeignKeyTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgForeignKeyTargetRule) ID() string { return ruleIDPGForeignKeyTargetsValid }

func (r pgForeignKeyTargetRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, "add_constraint") &&
		len(s.DDL.Alter) == 1 && s.DDL.Alter[0].Options["constraint_type"] == "foreign_key"
}

func assessPGForeignKeyTargets(s spec.Statement) (reason, fact, column string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version", ""
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", "", ""
	}
	if !spec.PGOrdinaryTableBound(s) {
		return "", "target_table.pg_columns", ""
	}
	target := spec.PGForeignKeyTarget(s)
	if target == nil || !target.ReferenceQualified() || len(target.Columns) == 0 {
		return "", "foreign_key.target_identity", ""
	}
	ref := s.Metadata.PGForeignKeyReference
	if ref == nil || ref.Schema != target.Reference.Schema || ref.PGRelation.Name != target.Reference.Name {
		return "", "referenced_table.pg_relation", ""
	}
	switch ref.PGRelation.Kind {
	case "":
		if ref.Exists {
			return "", "referenced_table.pg_relation", ""
		}
		return "foreign_key_reference_absent", "", ""
	case "r", "p":
	default:
		return "foreign_key_reference_wrong_kind", "", ""
	}
	if ref.PGRelation.Kind != "r" || !ref.Exists || ref.PGColumns == nil || !ref.PGColumns.Ordinary || ref.PGColumns.Typed {
		return "", "referenced_table.pg_columns", ""
	}
	for _, name := range target.Columns {
		member, known := s.Metadata.TargetTable.PGColumns.FindColumn(name)
		if !known {
			return "", "target_table.pg_columns", ""
		}
		if member == nil {
			return "foreign_key_source_column_absent", "", name
		}
		if member.System {
			return "foreign_key_source_system_column", "", name
		}
	}
	if len(target.ReferencedColumns) == 0 {
		return "", "referenced_table.primary_key", ""
	}
	for _, name := range target.ReferencedColumns {
		member, known := ref.PGColumns.FindColumn(name)
		if !known {
			return "", "referenced_table.pg_columns", ""
		}
		if member == nil {
			return "foreign_key_reference_column_absent", "", name
		}
		if member.System {
			return "foreign_key_reference_system_column", "", name
		}
	}
	return "", "", ""
}

func (r pgForeignKeyTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _, column := assessPGForeignKeyTargets(s)
	var message string
	switch reason {
	case "foreign_key_reference_absent":
		message = "The PostgreSQL foreign key's referenced table does not exist"
	case "foreign_key_reference_wrong_kind":
		message = "PostgreSQL requires a table as the foreign key reference"
	case "foreign_key_source_column_absent":
		message = "A declared PostgreSQL foreign-key source column does not exist"
	case "foreign_key_reference_column_absent":
		message = "A declared PostgreSQL foreign-key reference column does not exist"
	case "foreign_key_source_system_column", "foreign_key_reference_system_column":
		message = "PostgreSQL cannot use system columns in foreign keys"
	default:
		return nil, nil
	}
	metadata := map[string]any{"action": "add_constraint", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason}
	if column != "" {
		metadata["column"] = column
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: metadata}}, nil
}

func (r pgForeignKeyTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact, _ := assessPGForeignKeyTargets(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_foreign_key_targets", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
