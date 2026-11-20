package ddl

import (
	"context"
	"slices"

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
	// PostgreSQL resolves every action-column identity before checking the subset.
	for _, name := range target.DeleteSetColumns {
		member, known := s.Metadata.TargetTable.PGColumns.FindColumn(name)
		switch {
		case !known:
			return "", "target_table.pg_columns", ""
		case member == nil:
			return "foreign_key_delete_set_column_absent", "", name
		case member.System:
			return "foreign_key_delete_set_system_column", "", name
		}
	}
	for _, name := range target.DeleteSetColumns {
		if !slices.Contains(target.Columns, name) {
			return "foreign_key_delete_set_column_outside_key", "", name
		}
	}
	referenced := make(map[string]struct{}, len(target.ReferencedColumns))
	duplicate := ""
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
		if _, repeated := referenced[name]; repeated && duplicate == "" {
			duplicate = name
		}
		referenced[name] = struct{}{}
	}
	if duplicate != "" {
		return "foreign_key_reference_column_duplicate", "", duplicate
	}
	reason, fact = assessPGForeignKeyKey(target, referenced, s.Metadata.PGForeignKeyKeys)
	if reason != "" || fact != "" {
		return reason, fact, ""
	}
	wanted := spec.PGForeignKeyEqualityTarget{Source: *s.DDL.Table, Reference: target.Reference, Columns: target.Columns, ReferencedColumns: target.ReferencedColumns}
	observed := s.Metadata.PGForeignKeyEquality
	if observed == nil || !observed.Target.Matches(wanted) || observed.IncompatiblePosition < 0 || observed.IncompatiblePosition > len(target.Columns) {
		return "", "foreign_key.pg_equality_compatibility", ""
	}
	if observed.IncompatiblePosition != 0 {
		return "foreign_key_column_types_incompatible", "", target.Columns[observed.IncompatiblePosition-1]
	}
	return "", "", ""
}

func assessPGForeignKeyKey(target *spec.PGForeignKeySelection, referenced map[string]struct{}, keys *spec.PGForeignKeyKeys) (reason, fact string) {
	keyFact := "referenced_table.unique_key"
	if len(referenced) == 0 {
		keyFact = "referenced_table.primary_key"
	}
	if keys == nil || keys.Schema != target.Reference.Schema || keys.Table != target.Reference.Name {
		return "", keyFact
	}
	width, primaryCount := 0, 0
	deferred := false
	for _, key := range keys.Keys {
		if len(referenced) == 0 {
			if !key.Primary {
				continue
			}
			primaryCount++
		} else if !matchesPGForeignKeyColumns(referenced, key.Columns) {
			continue
		}
		if len(key.Columns) == 0 {
			return "", keyFact
		}
		if key.Immediate {
			width = len(key.Columns)
		} else {
			deferred = true
		}
	}
	if primaryCount > 1 {
		return "", keyFact
	}
	if width == 0 {
		if deferred {
			return "foreign_key_reference_key_deferrable", ""
		}
		if len(referenced) == 0 {
			return "foreign_key_reference_primary_key_absent", ""
		}
		return "foreign_key_reference_unique_key_absent", ""
	}
	if len(target.Columns) != width {
		return "foreign_key_column_count_mismatch", ""
	}
	return "", ""
}

func matchesPGForeignKeyColumns(referenced map[string]struct{}, columns []string) bool {
	if len(columns) != len(referenced) {
		return false
	}
	seen := make(map[string]struct{}, len(columns))
	for _, name := range columns {
		if _, wanted := referenced[name]; !wanted {
			return false
		}
		seen[name] = struct{}{}
	}
	return len(seen) == len(referenced)
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
	case "foreign_key_delete_set_column_absent":
		message = "A declared ON DELETE SET column does not exist on the PostgreSQL source table"
	case "foreign_key_delete_set_system_column":
		message = "PostgreSQL cannot use a system column in an ON DELETE SET column list"
	case "foreign_key_delete_set_column_outside_key":
		message = "PostgreSQL requires each ON DELETE SET column to be part of the foreign key"
	case "foreign_key_reference_column_duplicate":
		message = "PostgreSQL foreign keys cannot repeat a referenced column"
	case "foreign_key_reference_primary_key_absent":
		message = "PostgreSQL requires a referenced primary key when the reference column list is omitted"
	case "foreign_key_reference_unique_key_absent":
		message = "No eligible PostgreSQL unique index matches the referenced columns"
	case "foreign_key_reference_key_deferrable":
		message = "PostgreSQL requires a nondeferrable referenced key"
	case "foreign_key_column_types_incompatible":
		message = "The PostgreSQL foreign key column types cannot use the selected referenced key equality operators"
	case "foreign_key_column_count_mismatch":
		message = "The PostgreSQL foreign key and referenced key have different numbers of columns"
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
