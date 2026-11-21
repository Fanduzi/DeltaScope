package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgDropViewTargetRule struct{ level rule.Level }

func newPGDropViewTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgDropViewTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgDropViewTargetRule) ID() string { return ruleIDPGDropViewTargetValid }

func (r pgDropViewTargetRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationDropView
}

func assessPGDropViewTarget(s spec.Statement) (reason, missing string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if len(s.DDL.PGDropViewTargets) > 1 {
		if spec.PGDropViewSetLookup(s) == nil {
			return "", "view.target_identity"
		}
		state := s.Metadata.PGDropViewSet
		if state == nil || !state.Matches(s.DDL.PGDropViewTargets, s.DDL.Options["if_exists"] == "true", s.DDL.Options["cascade"] == "true") {
			return "", "view.pg_drop_state"
		}
		return state.Reason, ""
	}
	if spec.PGDropViewLookup(s) == nil {
		return "", "view.target_identity"
	}
	state := s.Metadata.PGDropViewState
	if state == nil || state.Target != *s.DDL.PGDropViewTarget {
		return "", "view.pg_drop_state"
	}
	switch {
	case state.Kind == "":
		if s.DDL.Options["if_exists"] != "true" {
			return "drop_view_absent", ""
		}
	case state.Kind != "v":
		return "drop_view_wrong_kind", ""
	case state.DependentView && s.DDL.Options["cascade"] != "true":
		return "drop_view_has_direct_dependency", ""
	case state.DependentColumn && s.DDL.Options["cascade"] != "true":
		return "drop_view_has_row_type_dependency", ""
	}
	return "", ""
}

func (r pgDropViewTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGDropViewTarget(s)
	var message string
	switch reason {
	case "drop_view_absent":
		message = "The PostgreSQL DROP VIEW target does not exist"
	case "drop_view_wrong_kind":
		message = "The PostgreSQL DROP VIEW target has the wrong relation kind"
	case "drop_view_has_direct_dependency":
		message = "A view or materialized view depends on the PostgreSQL DROP VIEW target"
	case "drop_view_has_row_type_dependency":
		message = "A column outside the drop selection depends on the PostgreSQL view row type"
	default:
		return nil, nil
	}
	target := s.DDL.PGDropViewTarget
	if len(s.DDL.PGDropViewTargets) > 1 {
		target = &s.DDL.PGDropViewTargets[s.Metadata.PGDropViewSet.Position-1]
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Table.Schema, "view": target.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgDropViewTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGDropViewTarget(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_drop_view_target", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
