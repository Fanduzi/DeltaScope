package ddl

import (
	"context"
	"regexp"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var pgConstraintTargetVersionPattern = regexp.MustCompile(`^PostgreSQL 17\.\d+(\s|$)`)

type pgConstraintTargetRule struct {
	id    string
	level rule.Level
}

func newPGValidateConstraintTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintTargetRule{id: ruleIDPGAlterValidateConstraintTargetRequire, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func newPGConstraintDeferrabilityTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintTargetRule{id: ruleIDPGAlterConstraintDeferrabilityTargetRequire, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func newPGDropConstraintTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintTargetRule{id: ruleIDPGAlterDropConstraintTargetRequire, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgConstraintTargetRule) ID() string { return r.id }

func (r pgConstraintTargetRule) matches(action string) bool {
	switch r.id {
	case ruleIDPGAlterValidateConstraintTargetRequire:
		return action == "validate_constraint"
	case ruleIDPGAlterConstraintDeferrabilityTargetRequire:
		return action == "alter_constraint_deferrable" || action == "alter_constraint_initially_deferred"
	case ruleIDPGAlterDropConstraintTargetRequire:
		return action == "drop_constraint"
	default:
		return false
	}
}

func (r pgConstraintTargetRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	for _, a := range s.DDL.Alter {
		if r.matches(a.Action) {
			return true
		}
	}
	return false
}

// pgConstraintTargetState binds a catalog to the exact authored identity.
// A preceding ALTER action is unapplied too: never invent its post-state.
func pgConstraintTargetState(s spec.Statement, index int) (*spec.PGConstraintState, bool) {
	if s.DDL == nil || s.DDL.Table == nil || index != 0 || index >= len(s.DDL.Alter) || s.Metadata == nil || s.Metadata.TargetTable == nil {
		return nil, false
	}
	m, table := s.Metadata, s.DDL.Table
	snapshot := m.TargetTable
	schema := table.Schema
	if schema == "" {
		schema = m.Schema
	}
	if table.Name == "" || schema == "" || m.Schema != schema || snapshot.Schema != schema || !snapshot.Exists || snapshot.Table == nil || snapshot.Table.Name != table.Name {
		return nil, false
	}
	return snapshot.FindPGConstraint(s.DDL.Alter[index].Name)
}

// The shared VersionIdentity contract intentionally excludes PostgreSQL;
// use the existing observed server banner, never a caller target override.
func pgConstraintTargetVersionKnown(s spec.Statement) bool {
	return s.Metadata != nil && s.Metadata.Instance != nil && pgConstraintTargetVersionPattern.MatchString(s.Metadata.Instance.Version)
}

func (r pgConstraintTargetRule) assess(s spec.Statement, index int) (*spec.PGConstraintState, string, string) {
	member, known := pgConstraintTargetState(s, index)
	if !known {
		return nil, "", "target_table.pg_constraints"
	}
	if !pgConstraintTargetVersionKnown(s) {
		return nil, "", "instance.version"
	}
	if member == nil {
		if r.id == ruleIDPGAlterDropConstraintTargetRequire {
			switch s.DDL.Alter[index].Options["if_exists"] {
			case "true":
				return nil, "", ""
			case "false":
			default:
				return nil, "", "constraint.if_exists"
			}
		}
		return nil, "constraint_absent", ""
	}
	if r.id == ruleIDPGAlterValidateConstraintTargetRequire && member.Type != "c" && member.Type != "f" || r.id == ruleIDPGAlterConstraintDeferrabilityTargetRequire && member.Type != "f" {
		return member, "constraint_kind_mismatch", ""
	}
	return member, "", ""
}

func (r pgConstraintTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	var gaps []rule.EvidenceGap
	seen := make(map[string]bool)
	for i, a := range s.DDL.Alter {
		if !r.matches(a.Action) {
			continue
		}
		_, _, fact := r.assess(s, i)
		if fact != "" && !seen[fact] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_constraint_target", RequiredFacts: []string{fact}})
			seen[fact] = true
		}
	}
	return gaps
}

func (r pgConstraintTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	var findings []rule.Finding
	for i, a := range s.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !r.matches(a.Action) {
			continue
		}
		member, reason, _ := r.assess(s, i)
		if reason == "" {
			continue
		}
		message := "PostgreSQL constraint target does not exist"
		if reason == "constraint_kind_mismatch" {
			message = "PostgreSQL constraint target has an unsupported kind for this action"
		}
		metadata := map[string]any{"action": a.Action, "table": s.DDL.Table.Name, "schema": s.Metadata.TargetTable.Schema, "constraint": a.Name, "target_exists": member != nil, "reason_code": reason}
		if member != nil {
			metadata["target_constraint_kind"] = member.Type
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message,
			Suggestion: "verify the exact table/schema/constraint identity and action: VALIDATE accepts FK/CHECK, ALTER CONSTRAINT deferrability accepts FK; DROP IF EXISTS guards only absence, not dependency or data legality",
			Metadata:   metadata})
	}
	return findings, nil
}
