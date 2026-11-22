package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mysqlRoutineTargetRule struct{ level rule.Level }

func newMySQLRoutineTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlRoutineTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r mysqlRoutineTargetRule) ID() string { return ruleIDMySQLRoutineTargetValid }

func (r mysqlRoutineTargetRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectMySQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Routine == nil {
		return false
	}
	if spec.MySQLRoutineLookup(s) == nil && !s.DDL.Routine.IfNotExists {
		return false
	}
	switch s.DDL.Operation {
	case spec.DDLOperationCreateFunction:
		return s.DDL.Routine.ReturnsType != ""
	case spec.DDLOperationCreateProcedure, spec.DDLOperationAlterProcedure, spec.DDLOperationAlterFunction,
		spec.DDLOperationDropProcedure, spec.DDLOperationDropFunction:
		return true
	}
	return false
}

func assessMySQLRoutineTarget(s spec.Statement) (reason, fact string) {
	d := s.DDL.Routine
	create := s.DDL.Operation == spec.DDLOperationCreateProcedure || s.DDL.Operation == spec.DDLOperationCreateFunction
	if create && d.IfNotExists {
		if s.Metadata == nil || s.Metadata.Version == nil || s.Metadata.Version.Product != spec.VersionProductMySQL || !s.Metadata.Version.ValidatedRange {
			return "", "target.version"
		}
		v := s.Metadata.Version
		if v.Major == 5 || v.Major == 8 && v.Minor == 0 && v.Patch < 29 {
			return "routine_create_guard_unavailable", ""
		}
	}
	if d.IfExists || create && d.IfNotExists {
		return "", ""
	}
	if s.Metadata == nil || s.Metadata.MySQLRoutine == nil || s.Metadata.MySQLRoutine.Name != d.CatalogTarget || s.Metadata.MySQLRoutine.Kind != d.Kind {
		return "", "routine.presence"
	}
	if create {
		return "routine_target_exists", ""
	}
	return "", ""
}

func (r mysqlRoutineTargetRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessMySQLRoutineTarget(s)
	message := map[string]string{
		"routine_create_guard_unavailable": "The target MySQL version does not support IF NOT EXISTS for stored routines",
		"routine_target_exists":            "A MySQL stored routine of this type already occupies the target name",
	}[reason]
	if message == "" {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Routine.Name.Schema, "name": s.DDL.Routine.Name.Name, "kind": string(s.DDL.Routine.Kind), "reason_code": reason,
	}}}, nil
}

func (r mysqlRoutineTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessMySQLRoutineTarget(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_mysql_routine_target", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
