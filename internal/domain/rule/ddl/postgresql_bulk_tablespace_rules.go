package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgBulkTablespaceRule struct{ level rule.Level }

func newPGBulkTablespaceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgBulkTablespaceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgBulkTablespaceRule) ID() string { return ruleIDPGBulkTablespaceTargetsValid }

func (r pgBulkTablespaceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterTable && s.DDL.Table == nil && len(s.DDL.Alter) == 1 && s.DDL.Alter[0].Action == "move_all_tablespace"
}

func assessPGBulkTablespace(s spec.Statement) (reason, fact string, same bool) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version", false
	}
	target, observed := spec.PGBulkTablespaceSelection(s), s.Metadata.PGBulkTablespace
	const scope = "instance.pg_bulk_tablespace_targets"
	if target == nil || observed == nil || !target.Matches(observed.Target) || len(target.Roles) != len(observed.RoleExists) {
		return "", scope, false
	}
	// PostgreSQL resolves the filters before either tablespace lookup.
	for i, role := range target.Roles {
		if role.Kind == "public" {
			return "owner_filter_public_inapplicable", "", false
		}
		if !observed.RoleExists[i] {
			if role.Kind == "named" {
				return "owner_filter_role_absent", "", false
			}
			return "", scope, false
		}
	}
	switch {
	case !observed.SourceExists:
		return "source_tablespace_absent", "", false
	case !observed.DestinationExists:
		return "destination_tablespace_absent", "", false
	case observed.SourceGlobal || observed.DestinationGlobal:
		return "tablespace_global_inapplicable", "", false
	}
	return "", "", observed.Same
}

func (r pgBulkTablespaceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _, _ := assessPGBulkTablespace(s)
	var message string
	switch reason {
	case "owner_filter_public_inapplicable":
		message = "PostgreSQL does not permit PUBLIC in a bulk tablespace owner filter"
	case "owner_filter_role_absent":
		message = "A PostgreSQL bulk tablespace owner-filter role does not exist"
	case "source_tablespace_absent":
		message = "The PostgreSQL bulk move source tablespace does not exist"
	case "destination_tablespace_absent":
		message = "The PostgreSQL bulk move destination tablespace does not exist"
	case "tablespace_global_inapplicable":
		message = "PostgreSQL reserves pg_global for shared relations and rejects it in a bulk tablespace move"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"action": "move_all_tablespace", "reason_code": reason},
	}}, nil
}

func (r pgBulkTablespaceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact, _ := assessPGBulkTablespace(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_bulk_tablespace_targets", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
