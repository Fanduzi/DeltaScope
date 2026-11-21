package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgGrantDeclarationRule struct{ level rule.Level }

func newPGGrantDeclarationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgGrantDeclarationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgGrantDeclarationRule) ID() string { return ruleIDPGGrantDeclarationValid }

func (r pgGrantDeclarationRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil && s.DDL.PGGrant != nil
}

func assessPGGrant(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	d := s.DDL.PGGrant
	state := s.Metadata.PGGrant
	if state == nil || state.ObjectKind != d.ObjectKind || state.AllInSchema != d.AllInSchema || state.Grantor != d.Grantor ||
		len(state.Targets) != len(d.Targets) || len(state.GranteeExists) != len(d.Grantees) || !slices.Equal(state.Grantees, d.Grantees) {
		return "", "grant.catalog"
	}
	if d.Grantor.Kind != "" {
		if !d.Grantor.Valid() {
			return "", "grant.grantor_identity"
		}
		if !state.GrantorExists {
			return "grant_grantor_absent", ""
		}
		if !state.GrantorCurrent {
			return "grant_grantor_not_current_user", ""
		}
	}
	for i, target := range d.Targets {
		if state.Targets[i].Target != target || target.Table.Name == "" {
			return "", "grant.target_identity"
		}
		if d.AllInSchema || d.ObjectKind == "schema" {
			if target.Table.Name == "pg_temp" {
				return "", "grant.schema_identity"
			}
		} else if (d.ObjectKind == "table" || d.ObjectKind == "sequence") && !target.Qualified() {
			return "", "grant.relation_identity"
		}
		if !state.Targets[i].Exists {
			return "grant_target_absent", ""
		}
	}
	for i, role := range d.Grantees {
		if !role.Valid() {
			return "", "grant.grantee_identity"
		}
		if !state.GranteeExists[i] {
			return "grant_grantee_absent", ""
		}
	}
	return assessPGGrantPrivileges(d, state)
}

func assessPGGrantPrivileges(d *spec.PGGrantDeclaration, state *spec.PGGrantState) (reason, fact string) {
	tablePrivileges := []string{"insert", "select", "update", "delete", "truncate", "references", "trigger", "maintain", "usage"}
	columns := []string{"insert", "select", "update", "references"}
	allowed := map[string][]string{
		"table": tablePrivileges, "sequence": {"select", "update", "usage"}, "schema": {"usage", "create"},
		"database": {"create", "temporary", "temp", "connect"}, "tablespace": {"create"},
	}
	for _, privilege := range d.Privileges {
		if len(privilege.Columns) != 0 {
			if d.ObjectKind != "table" {
				return "grant_column_privilege_invalid", ""
			}
			continue
		}
		if privilege.Name != "rule" && !slices.Contains(allowed[d.ObjectKind], privilege.Name) {
			return "grant_object_privilege_invalid", ""
		}
	}
	if d.AllInSchema {
		return "", "grant.schema_members"
	}
	for _, target := range state.Targets {
		if target.Kind == "i" || target.Kind == "I" || target.Kind == "c" || d.ObjectKind == "sequence" && target.Kind != "S" {
			return "grant_relation_kind_invalid", ""
		}
		for _, privilege := range d.Privileges {
			if len(privilege.Columns) == 0 {
				if d.ObjectKind == "table" && target.Kind != "S" && privilege.Name == "usage" {
					return "grant_object_privilege_invalid", ""
				}
				continue
			}
			if privilege.Name != "" && privilege.Name != "rule" && !slices.Contains(columns, privilege.Name) {
				return "grant_column_privilege_invalid", ""
			}
			for _, column := range privilege.Columns {
				if !slices.Contains(target.Columns, column) {
					return "grant_column_absent", ""
				}
			}
		}
	}
	if d.Grant && d.GrantOption && slices.ContainsFunc(d.Grantees, func(role spec.PGTableOwnerTarget) bool { return role.Kind == "public" }) {
		return "", "grant.public_grant_option_effect"
	}
	return "", ""
}

func (r pgGrantDeclarationRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGGrant(s)
	messages := map[string]string{
		"grant_grantor_absent":           "The PostgreSQL grantor role does not exist",
		"grant_grantor_not_current_user": "PostgreSQL requires GRANTED BY to identify the executing role",
		"grant_grantee_absent":           "A PostgreSQL grantee role does not exist",
		"grant_target_absent":            "A PostgreSQL privilege target does not exist",
		"grant_relation_kind_invalid":    "A PostgreSQL relation has the wrong kind for the privilege declaration",
		"grant_object_privilege_invalid": "A PostgreSQL privilege is invalid for the target object kind",
		"grant_column_privilege_invalid": "A PostgreSQL privilege does not support a column list",
		"grant_column_absent":            "An authored PostgreSQL privilege column does not exist on a target",
	}
	if message := messages[reason]; message != "" {
		return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
			"object_kind": s.DDL.PGGrant.ObjectKind, "reason_code": reason,
		}}}, nil
	}
	return nil, nil
}

func (r pgGrantDeclarationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGGrant(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_grant_declaration", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
