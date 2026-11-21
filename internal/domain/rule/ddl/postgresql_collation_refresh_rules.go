package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCollationRefreshRule struct{ level rule.Level }

func newPGCollationRefreshRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCollationRefreshRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCollationRefreshRule) ID() string { return ruleIDPGCollationRefreshValid }

func (r pgCollationRefreshRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterCollation && s.DDL.PGCollationRefresh != nil
}

func assessPGCollationRefresh(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGCollationRefreshLookup(s) == nil {
		return "", "collation.target_identity"
	}
	state := s.Metadata.PGCollationRefresh
	if state == nil || !state.Known || state.Target != *s.DDL.PGCollationRefresh {
		return "", "collation.encoding_identity"
	}
	switch {
	case !state.Exists:
		return "collation_refresh_target_absent", ""
	case state.Default:
		return "collation_refresh_database_default", ""
	case !state.VersionKnown:
		return "", "collation.provider_version"
	case state.VersionIncompatible:
		return "collation_refresh_version_incompatible", ""
	default:
		return "", ""
	}
}

func (r pgCollationRefreshRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCollationRefresh(s)
	var message string
	switch reason {
	case "collation_refresh_target_absent":
		message = "The PostgreSQL collation does not exist for this database encoding"
	case "collation_refresh_database_default":
		message = "PostgreSQL refreshes the default collation version through ALTER DATABASE"
	case "collation_refresh_version_incompatible":
		message = "PostgreSQL rejects a collation refresh that changes whether its version is tracked"
	default:
		return nil, nil
	}
	target := s.DDL.PGCollationRefresh
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "collation": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCollationRefreshRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCollationRefresh(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_collation_refresh", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
