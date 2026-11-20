package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnGenerationRule struct {
	id    string
	level rule.Level
}

func newPGColumnGenerationTransitionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnGenerationRule{ruleIDPGColumnGenerationTransitionValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func newPGAlterIdentityOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnGenerationRule{ruleIDPGAlterIdentityOptionsValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func newPGAlterIdentityRestartRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnGenerationRule{ruleIDPGAlterIdentityRestartWarn, configuredLevel(cfg, rule.LevelWarning)}, nil
}
func newPGAlterIdentityCycleRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnGenerationRule{ruleIDPGAlterIdentityCycleWarn, configuredLevel(cfg, rule.LevelWarning)}, nil
}
func (r pgColumnGenerationRule) ID() string { return r.id }

func (r pgColumnGenerationRule) matches(a spec.Alter) bool {
	if r.id == ruleIDPGColumnGenerationTransitionValid {
		switch a.Action {
		case "add_identity", "set_generated", "drop_identity", "drop_expression", "set_default", "drop_default", "set_not_null", "drop_not_null":
			return true
		}
		return false
	}
	if a.Action != "add_identity" && a.Action != "set_generated" {
		return false
	}
	if r.id == ruleIDPGAlterIdentityOptionsValid {
		return true
	}
	if r.id == ruleIDPGAlterIdentityRestartWarn && a.Action != "set_generated" {
		return false
	}
	f := a.PGColumnTransition
	if f == nil || f.Unmodeled {
		return true
	}
	for _, o := range f.Options {
		if r.id == ruleIDPGAlterIdentityCycleWarn && o.Kind == spec.SequenceOptionCycle || r.id == ruleIDPGAlterIdentityRestartWarn && (o.Kind == spec.SequenceOptionRestart || o.Kind == spec.SequenceOptionRestartWith) {
			return true
		}
	}
	return false
}
func (r pgColumnGenerationRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	for _, a := range s.DDL.Alter {
		if r.matches(a) {
			return true
		}
	}
	return false
}

func pgColumnTargetState(s spec.Statement, index int) (*spec.PGColumnState, bool) {
	if catalog := pgColumnTargetCatalog(s, index); catalog != nil {
		return catalog.FindColumn(s.DDL.Alter[index].Name)
	}
	return nil, false
}

func pgColumnTargetCatalog(s spec.Statement, index int) *spec.PGColumnCatalog {
	if s.DDL == nil || s.DDL.Table == nil || index != 0 || index >= len(s.DDL.Alter) || s.Metadata == nil || s.Metadata.TargetTable == nil {
		return nil
	}
	m, table := s.Metadata, s.DDL.Table
	snapshot := m.TargetTable
	schema := table.Schema
	if schema == "" {
		schema = m.Schema
	}
	if table.Name == "" || schema == "" || m.Schema != schema || snapshot.Schema != schema || !snapshot.Exists || snapshot.Table == nil || snapshot.Table.Name != table.Name {
		return nil
	}
	return snapshot.PGColumns
}

func pgColumnTransitionTarget(s spec.Statement, index int) (*spec.PGColumnState, string, string) {
	if !pgConstraintTargetVersionKnown(s) {
		return nil, "", "instance.version"
	}
	if index == 0 {
		if facts := spec.BoundPGColumnTraversal(s); facts != nil {
			switch s.DDL.Alter[0].Action {
			case "drop_expression":
				if s.DDL.PGTableOnly && facts.AttachedChildren {
					return nil, "drop_expression_requires_child_traversal", ""
				}
				if facts.Inherited {
					return nil, "drop_expression_inherited_column", ""
				}
			case "drop_not_null":
				if s.DDL.PGTableOnly && s.Metadata.TargetTable.PGRelation.Kind == "p" && facts.AttachedChildren {
					return nil, "drop_not_null_requires_partition_traversal", ""
				}
				if facts.Partition && facts.ParentNotNull {
					return nil, "not_null_required_by_partition_parent", ""
				}
			case "set_not_null":
				if s.DDL.PGTableOnly && s.Metadata.TargetTable.PGRelation.Kind == "p" && facts.NullableDescendant {
					return nil, "set_not_null_requires_partition_traversal", ""
				}
			}
		}
	}
	c, known := pgColumnTargetState(s, index)
	if !known {
		return nil, "", "target_table.pg_columns"
	}
	if c == nil {
		return nil, "column_absent", ""
	}
	if c.System {
		return c, "system_column_inapplicable", ""
	}
	a := s.DDL.Alter[index]
	switch a.Action {
	case "add_identity":
		switch {
		case c.Identity != "":
			return c, "column_already_identity", ""
		case c.Generated != "":
			return c, "column_already_generated", ""
		case c.TypeWidth == -1:
			return c, "identity_type_inapplicable", ""
		case c.TypeWidth != 16 && c.TypeWidth != 32 && c.TypeWidth != 64:
			return c, "", "column.identity_type"
		case !c.NotNull:
			return c, "identity_requires_not_null", ""
		case c.HasDefault:
			return c, "identity_default_conflict", ""
		}
	case "set_generated":
		if c.Identity == "" {
			return c, "identity_target_required", ""
		}
	case "drop_identity", "drop_expression":
		matches := a.Action == "drop_identity" && c.Identity != "" || a.Action == "drop_expression" && c.Generated == "s"
		if !matches {
			if a.PGColumnTransition == nil {
				return c, "", "column.generation_transition"
			}
			if !a.PGColumnTransition.IfExists {
				return c, "generation_target_kind_mismatch", ""
			}
		}
	case "set_default", "drop_default":
		if c.Identity != "" || c.Generated != "" {
			return c, "generation_default_conflict", ""
		}
	case "drop_not_null":
		if c.Identity != "" || c.PrimaryKey {
			return c, "not_null_required_by_identity_or_primary_key", ""
		}
		// A prior DDL can invalidate the relation while leaving its column list.
		schema := s.DDL.Table.Schema
		if len(s.DDL.Alter) == 1 && schema != "" && schema != "pg_temp" && !s.DDL.PGTableCatalogQualified && !s.Metadata.TargetTable.PGColumns.Typed {
			if !spec.PGOrdinaryTableBound(s) || !c.ReplicaIdentityKeyKnown {
				return c, "", "column.replica_identity_key"
			}
			if c.ReplicaIdentityKey {
				return c, "not_null_required_by_replica_identity", ""
			}
		}
	case "set_not_null":
		if !c.NotNull {
			return c, "", "column.nullability_data"
		}
	}
	return c, "", ""
}

func (r pgColumnGenerationRule) assess(s spec.Statement, index int) (string, string) {
	c, targetReason, targetMissing := pgColumnTransitionTarget(s, index)
	if r.id == ruleIDPGColumnGenerationTransitionValid {
		return targetReason, targetMissing
	}
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	a := s.DDL.Alter[index]
	reason, missing, cycle, restart := pgAlterIdentityOptions(a, c, targetReason, targetMissing)
	if r.id == ruleIDPGAlterIdentityOptionsValid {
		return reason, missing
	}
	if targetReason != "" || reason != "" {
		return "", ""
	}
	if missing != "" {
		return "", missing
	}
	if r.id == ruleIDPGAlterIdentityRestartWarn && restart {
		return "identity_restart", ""
	}
	if r.id == ruleIDPGAlterIdentityCycleWarn && cycle {
		return "identity_cycle", ""
	}
	return "", ""
}

func (r pgColumnGenerationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	var out []rule.EvidenceGap
	seen := map[string]bool{}
	code := "unproven_identity_sequence_options"
	if r.id == ruleIDPGColumnGenerationTransitionValid {
		code = "unproven_column_generation_target"
	}
	for i, a := range s.DDL.Alter {
		if !r.matches(a) {
			continue
		}
		_, fact := r.assess(s, i)
		if fact != "" && !seen[fact] {
			out = append(out, rule.EvidenceGap{ReasonCode: code, RequiredFacts: []string{fact}})
			seen[fact] = true
		}
	}
	return out
}

func (r pgColumnGenerationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	var out []rule.Finding
	for i, a := range s.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !r.matches(a) {
			continue
		}
		reason, _ := r.assess(s, i)
		if reason == "" {
			continue
		}
		message := "PostgreSQL column generation transition has a proven invalid target or precondition"
		suggestion := "verify the exact column identity, generation kind, integer type, NOT NULL and DEFAULT premises; target checks do not certify data, expressions or permissions"
		switch reason {
		case "drop_expression_requires_child_traversal":
			message = "PostgreSQL requires DROP EXPRESSION to include the child tables"
		case "drop_not_null_requires_partition_traversal":
			message = "PostgreSQL requires dropping NOT NULL on the partitioned parent and its partitions"
		case "not_null_required_by_partition_parent":
			message = "The PostgreSQL partition parent requires this column to remain NOT NULL"
		case "set_not_null_requires_partition_traversal":
			message = "A PostgreSQL descendant partition still allows NULL in this column"
		case "drop_expression_inherited_column":
			message = "PostgreSQL cannot drop a generation expression from an inherited column directly"
		}
		switch r.id {
		case ruleIDPGAlterIdentityOptionsValid:
			message = "PostgreSQL identity sequence options have a proven conflict or invalid value"
			suggestion = "correct duplicate options/modes and effective type, increment, cache and START/RESTART bounds; unknown current issuance state is not inferred"
		case ruleIDPGAlterIdentityRestartWarn:
			message = "identity RESTART can reuse values and blocks concurrent sequence issuance"
			suggestion = "review existing keys, sequence direction and bounds before RESTART; SET START alone changes only the recorded start, not issuance position"
		case ruleIDPGAlterIdentityCycleWarn:
			message = "identity sequence CYCLE can reuse values after reaching its limit"
			suggestion = "review value-reuse requirements; use NO CYCLE when wraparound is not intended"
		}
		if reason == "not_null_required_by_replica_identity" {
			message = "PostgreSQL requires NOT NULL on a replica identity index key column"
			suggestion = "Review the replica identity index before changing this column's nullability"
		}
		metadata := map[string]any{"action": a.Action, "table": s.DDL.Table.Name, "column": a.Name, "reason_code": reason}
		if s.DDL.Table.Schema != "" {
			metadata["schema"] = s.DDL.Table.Schema
		}
		out = append(out, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return out, nil
}
