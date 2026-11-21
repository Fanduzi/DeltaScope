package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateCommitRule struct{ level rule.Level }

func newPGCreateCommitRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateCommitRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateCommitRule) ID() string { return ruleIDPGCreateCommitValid }

func (r pgCreateCommitRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateCommit != nil
}

func assessPGCreateCommit(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	target := s.DDL.PGCreateCommit
	if !target.Target.CatalogQualified && target.Persistence == "t" && target.Target.Table.Schema == "" {
		return "", ""
	}
	if spec.PGCreateDestinationLookup(s) == nil {
		return "", "create_table.commit_namespace"
	}
	state := s.Metadata.PGCreateDestination
	if state == nil || state.Target != target.Target {
		return "", "create_table.commit_destination"
	}
	switch {
	case !state.SchemaExists:
		return "create_commit_schema_absent", ""
	case target.Persistence == "t":
		return "create_commit_temporary_namespace_required", ""
	case target.IfNotExists && state.RelationExists:
		return "", ""
	case target.Action != "":
		return "create_commit_temporary_required", ""
	}
	return "", ""
}

func (r pgCreateCommitRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateCommit(s)
	var message string
	switch reason {
	case "create_commit_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_commit_temporary_namespace_required":
		message = "PostgreSQL cannot create a temporary table in this persistent schema"
	case "create_commit_temporary_required":
		message = "PostgreSQL requires a temporary table for ON COMMIT"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateCommit.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateCommitRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateCommit(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_commit", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
