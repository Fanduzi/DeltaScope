package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgAlterViewOptionsRule struct{ level rule.Level }

func newPGAlterViewOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterViewOptionsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgAlterViewOptionsRule) ID() string { return ruleIDPGAlterViewOptionsValid }

func (r pgAlterViewOptionsRule) AppliesTo(s spec.Statement) bool {
	return spec.PGAlterViewOptionsAction(s) != ""
}

func assessPGAlterViewOptions(s spec.Statement) (reason, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGAlterViewOptionsLookup(s)
	if lookup == nil {
		return "", "view.target_identity"
	}
	state := s.Metadata.PGDropViewState
	if state == nil || state.Target.Table.Schema != lookup.Schema || state.Target.Table.Name != lookup.Name {
		return "", "view.pg_option_state"
	}
	if state.Kind == "" {
		if s.DDL.PGTableIfExists {
			return "", ""
		}
		return "view_options_target_absent", ""
	}
	if state.Kind != "v" {
		return "view_options_target_wrong_kind", ""
	}
	reset := spec.PGAlterViewOptionsAction(s) == "reset_options"
	if !reset {
		if reason, missing := assessPGViewOptionDeclarations(s.DDL.PGViewOptions, false, true); reason != "" || missing != "" {
			return reason, missing
		}
	}
	checkOption := state.CheckOption
	for _, option := range s.DDL.PGViewOptions {
		if option.Name == "" {
			return "", "view.options"
		}
		if reset && option.HasValue {
			return "view_option_reset_has_value", ""
		}
		if option.Namespace == "" && option.Name == "check_option" {
			checkOption = !reset
		}
	}
	if checkOption {
		return "", "view.check_option_updatability"
	}
	return "", ""
}

func (r pgAlterViewOptionsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGAlterViewOptions(s)
	message := "PostgreSQL rejects the declared view option or value"
	switch reason {
	case "":
		return nil, nil
	case "view_options_target_absent":
		message = "The PostgreSQL ALTER VIEW target does not exist"
	case "view_options_target_wrong_kind":
		message = "The PostgreSQL ALTER VIEW target has the wrong relation kind"
	case "view_option_reset_has_value":
		message = "PostgreSQL ALTER VIEW RESET does not accept option values"
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgAlterViewOptionsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGAlterViewOptions(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_alter_view_options", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
