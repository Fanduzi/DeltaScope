package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgConstraintColumnsRule struct{ level rule.Level }

func newPGConstraintColumnsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintColumnsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgConstraintColumnsRule) ID() string { return ruleIDPGConstraintColumnsValid }

func (r pgConstraintColumnsRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || !pgColumnIdentityAction(s, "add_constraint") || len(s.DDL.Alter) != 1 {
		return false
	}
	a := s.DDL.Alter[0]
	return a.Options["using_index"] == "false" && (a.Options["constraint_type"] == "primary_key" || a.Options["constraint_type"] == "unique")
}

func assessPGConstraintColumns(s spec.Statement) (reason, fact, column string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version", ""
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", "", ""
	}
	var columns []spec.PGColumnState
	var findColumn func(string) (*spec.PGColumnState, bool)
	var partition *spec.PGPartitionConstraintState
	if spec.PGOrdinaryTableBound(s) {
		catalog := s.Metadata.TargetTable.PGColumns
		if catalog != nil {
			columns, findColumn = catalog.Columns, catalog.FindColumn
		}
	} else if lookup := spec.PGPartitionConstraintLookup(s); lookup != nil {
		partition = s.Metadata.PGPartitionConstraint
		if partition != nil && partition.Target == *s.DDL.Table {
			columns, findColumn = partition.Columns, partition.FindColumn
		}
	}
	if findColumn == nil {
		return "", "target_table.pg_columns", ""
	}
	a := s.DDL.Alter[0]
	if len(a.PGKeyColumns) == 0 {
		return "", "constraint.key_columns", ""
	}
	seen := make(map[string]bool, len(a.PGKeyColumns))
	for _, name := range a.PGKeyColumns {
		if name == "" {
			return "", "constraint.key_columns", ""
		}
		if seen[name] {
			return "constraint_key_column_duplicate", "", name
		}
		seen[name] = true
	}
	for _, columns := range [][]string{a.PGKeyColumns, a.PGIncludeColumns} {
		for _, name := range columns {
			if name == "" {
				return "", "constraint.include_columns", ""
			}
			member, known := findColumn(name)
			if !known {
				return "", "target_table.pg_columns", ""
			}
			if member == nil {
				return "constraint_column_absent", "", name
			}
			if member.System {
				return "constraint_system_column_inapplicable", "", name
			}
		}
	}
	if a.Options["constraint_type"] == "primary_key" && slices.ContainsFunc(columns, func(column spec.PGColumnState) bool {
		return column.PrimaryKey
	}) {
		return "constraint_primary_key_already_exists", "", ""
	}
	if partition != nil {
		if len(partition.PartitionKeys) == 0 {
			return "", "constraint.partition_keys", ""
		}
		for _, key := range partition.PartitionKeys {
			switch {
			case key.Expression:
				return "constraint_partition_expression", "", ""
			case key.Name == "":
				return "", "constraint.partition_keys", ""
			case !seen[key.Name]:
				return "constraint_partition_column_missing", "", ""
			case !key.CollationOK || key.EqualityRead && !key.EqualityOK:
				return "constraint_partition_comparison_mismatch", "", ""
			case !key.EqualityRead:
				fact = "constraint.partition_equality"
			}
		}
	}
	return "", fact, ""
}

func (r pgConstraintColumnsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _, column := assessPGConstraintColumns(s)
	var message string
	switch reason {
	case "constraint_key_column_duplicate":
		message = "The same column appears more than once in the PostgreSQL constraint key"
	case "constraint_column_absent":
		message = "The PostgreSQL constraint references a column that does not exist"
	case "constraint_system_column_inapplicable":
		message = "PostgreSQL cannot create a constraint index on a system column"
	case "constraint_primary_key_already_exists":
		message = "The PostgreSQL table already has a primary key"
	case "constraint_partition_expression":
		message = "PostgreSQL rejects a primary or unique constraint on an expression partition key"
	case "constraint_partition_column_missing":
		message = "The PostgreSQL constraint key must include every partition key column"
	case "constraint_partition_comparison_mismatch":
		message = "The PostgreSQL constraint key must use the partition key's collation and equality operator"
	default:
		return nil, nil
	}
	metadata := map[string]any{
		"schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
		"constraint_type": s.DDL.Alter[0].Options["constraint_type"], "reason_code": reason,
	}
	if column != "" {
		metadata["column"] = column
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: metadata}}, nil
}

func (r pgConstraintColumnsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact, _ := assessPGConstraintColumns(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_constraint_columns", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
