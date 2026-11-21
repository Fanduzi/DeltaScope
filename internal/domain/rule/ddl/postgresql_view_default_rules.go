package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgViewDefaultRule struct{ level rule.Level }

func newPGViewDefaultRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgViewDefaultRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgViewDefaultRule) ID() string { return ruleIDPGViewDefaultValid }

func (r pgViewDefaultRule) AppliesTo(s spec.Statement) bool { return spec.PGViewDefaultAction(s) != "" }

func assessPGViewDefault(s spec.Statement) (reason, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGViewDefaultLookup(s)
	if lookup == nil {
		return "", "view.target_identity"
	}
	state := s.Metadata.PGViewDefault
	if state == nil || !state.MatchesViewDefault(lookup) {
		return "", "view.pg_default_target"
	}
	switch {
	case state.Kind == "":
		if s.DDL.PGTableIfExists {
			return "", ""
		}
		return "view_default_target_absent", ""
	case state.Kind != "v":
		return "view_default_target_wrong_kind", ""
	case !state.ColumnExists:
		return "view_default_column_absent", ""
	}
	a := s.DDL.Alter[0]
	if a.Action == "set_default" {
		if a.PGColumnTransition == nil {
			return "", "view.default_expression"
		}
		if a.PGColumnTransition.DefaultHasDisallowedReference {
			return "view_default_reference_inapplicable", ""
		}
	}
	return "", ""
}

func (r pgViewDefaultRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGViewDefault(s)
	var message string
	switch reason {
	case "view_default_target_absent":
		message = "The PostgreSQL ALTER VIEW target does not exist"
	case "view_default_target_wrong_kind":
		message = "The PostgreSQL ALTER VIEW target has the wrong relation kind"
	case "view_default_column_absent":
		message = "The PostgreSQL view has no user column with the requested name"
	case "view_default_reference_inapplicable":
		message = "A PostgreSQL view column default cannot reference columns or contain subqueries"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "column": s.DDL.Alter[0].Name, "reason_code": reason,
	}}}, nil
}

func (r pgViewDefaultRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGViewDefault(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_view_default", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
