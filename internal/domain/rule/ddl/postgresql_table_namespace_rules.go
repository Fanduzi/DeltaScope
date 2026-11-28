package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTableNamespaceRule struct {
	id, action string
	level      rule.Level
}

func newPGRenameTableTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableNamespaceRule{ruleIDPGAlterRenameTableTargetValid, "rename_table", configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func newPGSetSchemaTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableNamespaceRule{ruleIDPGAlterSetSchemaTargetValid, "set_schema", configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTableNamespaceRule) ID() string { return r.id }

func (r pgTableNamespaceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, r.action)
}

func boundPGTableNamespace(s spec.Statement) *spec.PGTableNamespace {
	action, destination := spec.PGTableNamespaceTarget(s)
	partitionRoot := destination != "" && spec.PGPartitionedTableBound(s)
	if destination == "" || !spec.PGOrdinaryTableBound(s) && !partitionRoot || !spec.PG17Observed(s.Metadata.Instance) {
		return nil
	}
	facts, table := s.Metadata.PGTableNamespace, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Action != action || facts.Destination != destination {
		return nil
	}
	if partitionRoot {
		if !facts.SourceKindKnown || facts.SourceKind != "p" {
			return nil
		}
		// Only catalog-backed refusals close the partition-root evidence gap.
		reason, _ := assessPGNamespaceDestination(facts)
		switch reason {
		case "destination_schema_absent", "destination_relation_conflict", "destination_row_type_conflict", "destination_array_type_conflict", "destination_index_conflict", "destination_owned_sequence_conflict":
		default:
			return nil
		}
	}
	return facts
}

func assessPGTableNamespace(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	facts := boundPGTableNamespace(s)
	if facts == nil {
		return "", "target_table.pg_namespace_destination"
	}
	return assessPGNamespaceDestination(facts)
}

func assessPGNamespaceDestination(facts *spec.PGTableNamespace) (reason, fact string) {
	if facts.Action == "set_schema" {
		if !facts.SchemaExists {
			return "destination_schema_absent", ""
		}
		if facts.SameSchema {
			return "", ""
		}
	}
	if facts.RelationConflict {
		return "destination_relation_conflict", ""
	}
	if !facts.RowTypeConflictKnown {
		return "", "target_table.pg_namespace_destination"
	}
	if facts.RowTypeConflict {
		return "destination_row_type_conflict", ""
	}
	if facts.Action == "set_schema" {
		switch {
		case !facts.ArrayTypeConflictKnown:
			return "", "target_table.pg_namespace_companions"
		case facts.ArrayTypeConflict:
			return "destination_array_type_conflict", ""
		case facts.IndexConflict:
			return "destination_index_conflict", ""
		case facts.OwnedSequenceConflict:
			return "destination_owned_sequence_conflict", ""
		}
	}
	return "", ""
}

func (r pgTableNamespaceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTableNamespace(s)
	if reason == "" {
		return nil, nil
	}
	message := "The PostgreSQL table's destination relation name is already in use"
	suggestion := "Choose an existing schema and an available relation and row-type name"
	switch reason {
	case "destination_schema_absent":
		message = "The requested PostgreSQL destination schema does not exist"
	case "destination_row_type_conflict":
		message = "The PostgreSQL table's destination row-type name is already in use"
	case "destination_array_type_conflict":
		message = "The PostgreSQL table's associated array-type name is already in use in the destination schema"
		suggestion = "Review the conflicting type in the destination schema"
	case "destination_index_conflict":
		message = "An index on the PostgreSQL table has a name already in use in the destination schema"
		suggestion = "Review the index names and the destination schema"
	case "destination_owned_sequence_conflict":
		message = "A sequence owned by a PostgreSQL table column has a name already in use in the destination schema"
		suggestion = "Review the owned sequence names and the destination schema"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: suggestion,
		Metadata:   map[string]any{"action": r.action, "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason},
	}}, nil
}

func (r pgTableNamespaceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTableNamespace(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_namespace_destination", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
