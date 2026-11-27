package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTableOwnerRule struct{ level rule.Level }

func newPGTableOwnerTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableOwnerRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTableOwnerRule) ID() string { return ruleIDPGAlterTableOwnerTargetValid }

func (r pgTableOwnerRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, "change_owner")
}

func boundPGTableOwner(s spec.Statement) *spec.PGTableOwnerState {
	target := spec.PGTableOwnerSelection(s)
	if target.Kind == "" || !spec.PGOrdinaryTableBound(s) && !spec.PGPartitionedTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
		return nil
	}
	facts, table := s.Metadata.PGTableOwner, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Target != target {
		return nil
	}
	if spec.PGPartitionedTableBound(s) && target.Kind != "public" && (target.Kind != "named" || facts.Exists) {
		return nil
	}
	return facts
}

func assessPGTableOwner(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	facts := boundPGTableOwner(s)
	if facts == nil {
		return "", "target_table.pg_owner"
	}
	switch facts.Target.Kind {
	case "public":
		return "owner_public_inapplicable", ""
	case "named":
		if !facts.Exists {
			return "owner_role_absent", ""
		}
	default:
		if !facts.Exists {
			return "", "target_table.pg_owner"
		}
	}
	return "", ""
}

func (r pgTableOwnerRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTableOwner(s)
	if reason == "" {
		return nil, nil
	}
	message := "The PostgreSQL table owner role does not exist"
	if reason == "owner_public_inapplicable" {
		message = "PostgreSQL does not permit the PUBLIC pseudo-role as table owner"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: "Choose an existing owner role and verify the execution session separately",
		Metadata:   map[string]any{"action": "change_owner", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason},
	}}, nil
}

func (r pgTableOwnerRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTableOwner(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_owner_target", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
