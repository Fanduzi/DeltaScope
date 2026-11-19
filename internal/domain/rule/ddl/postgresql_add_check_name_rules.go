package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgAddCheckNameRule struct{ level rule.Level }

func newPGAddCheckNameRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAddCheckNameRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgAddCheckNameRule) ID() string { return ruleIDPGAlterAddCheckNameAvailable }

func (r pgAddCheckNameRule) AppliesTo(s spec.Statement) bool {
	return pgNamedAddCheck(s)
}

func pgNamedAddCheck(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != spec.DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return false
	}
	a := s.DDL.Alter[0]
	return a.Action == "add_constraint" && a.Options["constraint_type"] == "check" && a.Name != ""
}

func assessPGAddCheckName(s spec.Statement) (occupied bool, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return false, "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return false, ""
	}
	if !spec.PGOrdinaryTableBound(s) {
		return false, "target_table.pg_constraint_scope"
	}
	member, known := pgConstraintTargetState(s, 0)
	if !known {
		return false, "target_table.pg_constraints"
	}
	return member != nil, ""
}

func (r pgAddCheckNameRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	if occupied, _ := assessPGAddCheckName(s); !occupied {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: "The PostgreSQL table already has a constraint with the requested CHECK name",
		Metadata: map[string]any{
			"action": "add_constraint", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"constraint": s.DDL.Alter[0].Name, "reason_code": "constraint_name_occupied",
		},
	}}, nil
}

func (r pgAddCheckNameRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGAddCheckName(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_check_constraint_name", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
