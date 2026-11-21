package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgRefreshMaterializedViewTargetRule struct{ level rule.Level }

func newPGRefreshMaterializedViewTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgRefreshMaterializedViewTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgRefreshMaterializedViewTargetRule) ID() string {
	return ruleIDPGRefreshMaterializedViewTargetValid
}

func (r pgRefreshMaterializedViewTargetRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationRefreshMaterializedView
}

func assessPGRefreshMaterializedViewTarget(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	t := s.DDL.PGRefreshTarget
	if !t.Qualified() || t.Name != s.DDL.ObjectName {
		return "", "materialized_view.target_identity"
	}
	observed := s.Metadata.PGRefreshState
	if observed == nil || observed.Target != t {
		return "", "materialized_view.pg_refresh_state"
	}
	switch {
	case observed.Kind == "":
		return "refresh_materialized_view_absent", ""
	case observed.Kind != "m":
		return "refresh_target_wrong_kind", ""
	}
	if s.DDL.Options["concurrently"] != "true" {
		return "", ""
	}
	switch {
	case !observed.Populated:
		return "refresh_concurrent_unpopulated", ""
	case s.DDL.Options["with_no_data"] == "true":
		return "refresh_concurrent_no_data", ""
	case !observed.UniqueIndex:
		return "refresh_concurrent_unique_index_absent", ""
	}
	return "", ""
}

func (r pgRefreshMaterializedViewTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGRefreshMaterializedViewTarget(s)
	var message string
	switch reason {
	case "refresh_materialized_view_absent":
		message = "The PostgreSQL materialized view does not exist"
	case "refresh_target_wrong_kind":
		message = "The PostgreSQL refresh target is not a materialized view"
	case "refresh_concurrent_unpopulated":
		message = "PostgreSQL concurrent refresh requires a populated materialized view"
	case "refresh_concurrent_no_data":
		message = "PostgreSQL rejects CONCURRENTLY WITH NO DATA"
	case "refresh_concurrent_unique_index_absent":
		message = "PostgreSQL concurrent refresh requires a usable unique index on plain columns"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"schema": s.DDL.PGRefreshTarget.Schema, "object": s.DDL.ObjectName, "reason_code": reason},
	}}, nil
}

func (r pgRefreshMaterializedViewTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGRefreshMaterializedViewTarget(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_materialized_view_refresh_target", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
