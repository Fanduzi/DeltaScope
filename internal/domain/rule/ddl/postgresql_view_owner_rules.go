package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgViewOwnerRule struct{ level rule.Level }

func newPGViewOwnerRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgViewOwnerRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgViewOwnerRule) ID() string { return ruleIDPGViewOwnerValid }

func (r pgViewOwnerRule) AppliesTo(s spec.Statement) bool {
	_, kind := spec.PGViewOwnerSelection(s)
	return kind != ""
}

func assessPGViewOwner(s spec.Statement) (reason, missing string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	lookup := spec.PGViewOwnerLookup(s)
	if lookup == nil {
		return "", "view.target_identity"
	}
	state := s.Metadata.PGTableOwner
	if state == nil || !state.SourceKindKnown || state.Schema != lookup.Schema || state.Table != lookup.Qualifiers["table"] ||
		state.Target != s.DDL.Alter[0].PGTableOwner {
		return "", "view.pg_owner"
	}
	switch {
	case state.SourceKind == "":
		if s.DDL.PGTableIfExists {
			return "", ""
		}
		return "view_owner_source_absent", ""
	case state.SourceKind != lookup.Qualifiers["relation_kind"]:
		return "view_owner_source_wrong_kind", ""
	case state.Target.Kind == "public":
		return "owner_public_inapplicable", ""
	case state.Target.Kind == "named" && !state.Exists:
		return "owner_role_absent", ""
	case !state.Exists:
		return "", "view.pg_owner"
	}
	return "", ""
}

func (r pgViewOwnerRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGViewOwner(s)
	var message string
	switch reason {
	case "view_owner_source_absent":
		message = "The PostgreSQL view owner-change target does not exist"
	case "view_owner_source_wrong_kind":
		message = "The PostgreSQL view owner-change target has the wrong relation kind"
	case "owner_public_inapplicable":
		message = "PostgreSQL does not permit the PUBLIC pseudo-role as a view owner"
	case "owner_role_absent":
		message = "The PostgreSQL view owner role does not exist"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgViewOwnerRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGViewOwner(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_view_owner", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
