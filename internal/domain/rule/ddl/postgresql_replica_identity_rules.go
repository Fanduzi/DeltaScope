package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgReplicaIdentityIndexRule struct{ level rule.Level }

func newPGReplicaIdentityIndexRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgReplicaIdentityIndexRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgReplicaIdentityIndexRule) ID() string { return ruleIDPGAlterReplicaIdentityIndexValid }

func (r pgReplicaIdentityIndexRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Unsupported != nil || s.DDL == nil || s.DDL.Operation != spec.DDLOperationAlterTable || !appliesToAlterTable(s) {
		return false
	}
	for _, a := range s.DDL.Alter {
		if a.Action == "replica_identity" && a.Options["identity"] == "using_index" {
			return true
		}
	}
	return false
}

func assessPGReplicaIdentityIndex(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	name := spec.PGReplicaIdentityIndexTarget(s)
	partitionRoot := spec.PGPartitionedTableBound(s)
	if name == "" || !spec.PGOrdinaryTableBound(s) && !partitionRoot {
		return "", "target_table.pg_replica_identity_index"
	}
	facts := s.Metadata.PGReplicaIdentity
	table := s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Name != name || facts.PartitionRoot != partitionRoot {
		return "", "target_table.pg_replica_identity_index"
	}
	switch {
	case facts.Kind == "":
		return "index_absent", ""
	case facts.Kind != "i" && facts.Kind != "I":
		return "relation_not_index", ""
	case !facts.IndexKnown:
		return "", "target_table.pg_replica_identity_index"
	case !facts.OnTable:
		return "index_not_on_table", ""
	case !facts.Unique || !partitionRoot && facts.CanUniqueKnown && !facts.CanUnique:
		return "index_not_unique", ""
	case partitionRoot && (!facts.CanUniqueKnown || !facts.CanUnique):
		return "", "target_table.pg_replica_identity_index"
	case !facts.Immediate:
		return "index_not_immediate", ""
	case facts.Expression:
		return "index_expression", ""
	case facts.Partial:
		return "index_partial", ""
	case facts.KeyAttributesKnown && !facts.KeyAttributesValid:
		return "index_key_inapplicable", ""
	case partitionRoot || !facts.CanUniqueKnown || !facts.KeyAttributesKnown:
		return "", "target_table.pg_replica_identity_index"
	default:
		return "", ""
	}
}

func (r pgReplicaIdentityIndexRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGReplicaIdentityIndex(s)
	if reason == "" {
		return nil, nil
	}
	return []rule.Finding{{
		Level: r.level, Message: "PostgreSQL cannot attach this index as replica identity",
		Suggestion: "Verify the exact index belongs to the table and is unique, immediate and non-partial, with plain NOT NULL key columns",
		Metadata: map[string]any{"action": "replica_identity", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"index": spec.PGReplicaIdentityIndexTarget(s), "reason_code": reason},
	}}, nil
}

func (r pgReplicaIdentityIndexRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	if _, fact := assessPGReplicaIdentityIndex(s); fact != "" {
		return []rule.EvidenceGap{{ReasonCode: "unproven_replica_identity_index", RequiredFacts: []string{fact}}}
	}
	return nil
}
