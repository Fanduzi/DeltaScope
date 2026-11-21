package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgMaterializedViewOptionsRule struct{ level rule.Level }

func newPGMaterializedViewOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgMaterializedViewOptionsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgMaterializedViewOptionsRule) ID() string { return ruleIDPGMaterializedViewOptionsValid }

func (r pgMaterializedViewOptionsRule) AppliesTo(s spec.Statement) bool {
	return spec.PGMaterializedViewOptionsApplies(s)
}

func assessPGMaterializedViewOptions(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGMaterializedViewOptionsLookup(s)
	if lookup == nil {
		return "", "materialized_view.target_identity"
	}
	state := s.Metadata.PGDropViewState
	if state == nil || state.Target.Table.Schema != lookup.Schema || state.Target.Table.Name != lookup.Name {
		return "", "materialized_view.storage_state"
	}
	if state.Kind == "" {
		if s.DDL.PGTableIfExists {
			return "", ""
		}
		return "materialized_view_options_target_absent", ""
	}
	if state.Kind != "m" {
		return "materialized_view_options_target_wrong_kind", ""
	}

	a := s.DDL.Alter[0]
	if a.Action == "reset_options" {
		for _, opt := range a.PGTableReloptions {
			if opt.HasValue {
				return "materialized_view_option_reset_has_value", ""
			}
		}
		return "", ""
	}
	for _, opt := range a.PGTableReloptions {
		if opt.Namespace != "" && opt.Namespace != "toast" {
			return "materialized_view_option_namespace", ""
		}
	}
	if !state.Heap {
		return "", "materialized_view.pg_heap"
	}
	for _, namespace := range []string{"", "toast"} {
		if namespace == "toast" && !state.HasToast {
			continue
		}
		reason, missing := assessPGMaterializedViewStorageOptions(a.PGTableReloptions, namespace)
		if reason != "" {
			return reason, ""
		}
		if missing != "" {
			fact = missing
		}
	}
	return "", fact
}

func assessPGMaterializedViewStorageOptions(options []spec.PGTableReloption, namespace string) (reason, fact string) {
	seen := make(map[string]bool, 2)
	for _, opt := range options {
		if opt.Namespace != namespace {
			continue
		}
		switch {
		case namespace == "" && opt.Name == "fillfactor":
			if !opt.HasValue {
				return "materialized_view_option_value_required", ""
			}
			if opt.Value != spec.PGTableReloptionInteger {
				fact = "materialized_view.storage_options"
			} else if opt.Integer < 10 || opt.Integer > 100 {
				return "materialized_view_option_out_of_range", ""
			}
		case namespace == "toast" && opt.Name == "fillfactor", opt.Name == "security_barrier", opt.Name == "security_invoker", opt.Name == "check_option":
			return "materialized_view_option_unrecognized", ""
		case opt.Name == "autovacuum_enabled":
			if opt.Value == spec.PGTableReloptionInvalidBoolean {
				return "materialized_view_option_invalid_boolean", ""
			}
			if opt.Value != spec.PGTableReloptionBoolean {
				fact = "materialized_view.storage_options"
			}
		default:
			fact = "materialized_view.storage_options"
			continue
		}
		if seen[opt.Name] {
			return "materialized_view_option_duplicate", ""
		}
		seen[opt.Name] = true
	}
	return "", fact
}

func (r pgMaterializedViewOptionsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGMaterializedViewOptions(s)
	message := "PostgreSQL rejects the materialized view storage option declaration"
	switch reason {
	case "":
		return nil, nil
	case "materialized_view_options_target_absent":
		message = "The PostgreSQL ALTER MATERIALIZED VIEW target does not exist"
	case "materialized_view_options_target_wrong_kind":
		message = "The PostgreSQL ALTER MATERIALIZED VIEW target has the wrong relation kind"
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgMaterializedViewOptionsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGMaterializedViewOptions(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_materialized_view_options", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
