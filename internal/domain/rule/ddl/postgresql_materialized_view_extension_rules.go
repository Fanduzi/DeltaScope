package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgMaterializedViewExtensionRule struct{ level rule.Level }

func newPGMaterializedViewExtensionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgMaterializedViewExtensionRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgMaterializedViewExtensionRule) ID() string { return ruleIDPGMaterializedViewExtensionValid }

func (r pgMaterializedViewExtensionRule) AppliesTo(s spec.Statement) bool {
	return spec.PGMaterializedViewExtensionApplies(s)
}

func assessPGMaterializedViewExtension(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGMaterializedViewExtensionLookup(s)
	if lookup == nil {
		return "", "materialized_view.target_identity"
	}
	state := s.Metadata.PGMaterializedViewExtension
	if state == nil || state.Schema != lookup.Schema || state.View != lookup.Name || state.Extension != lookup.Qualifiers["extension"] {
		return "", "materialized_view.extension_state"
	}
	switch {
	case state.Kind == "":
		return "materialized_view_extension_target_absent", ""
	case state.Kind != "m":
		return "materialized_view_extension_target_wrong_kind", ""
	case !state.ExtensionExists:
		return "materialized_view_extension_absent", ""
	default:
		return "", ""
	}
}

func (r pgMaterializedViewExtensionRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGMaterializedViewExtension(s)
	var message string
	switch reason {
	case "materialized_view_extension_target_absent":
		message = "The PostgreSQL materialized view does not exist"
	case "materialized_view_extension_target_wrong_kind":
		message = "The PostgreSQL extension dependency target is not a materialized view"
	case "materialized_view_extension_absent":
		message = "The requested PostgreSQL extension is not installed"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgMaterializedViewExtensionRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGMaterializedViewExtension(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_materialized_view_extension", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
