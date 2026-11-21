package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateViewOutputRule struct{ level rule.Level }

func newPGCreateViewOutputRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateViewOutputRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateViewOutputRule) ID() string { return ruleIDPGCreateViewOutputValid }

func (r pgCreateViewOutputRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != spec.DDLOperationCreateView || s.DDL.Table == nil || s.DDL.PGCreateViewOutput == nil {
		return false
	}
	output := s.DDL.PGCreateViewOutput
	return s.DDL.Table.Schema != "" || output.AliasCount != 0 || output.Unlogged || s.DDL.Options["replace"] == "true"
}

func assessPGCreateViewOutput(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	output := s.DDL.PGCreateViewOutput
	if output.Columns == nil {
		return "", "view.query_output"
	}
	if s.DDL.Options["check_option"] == "true" {
		return "", "view.check_option_updatability"
	}
	for _, opt := range s.DDL.PGViewOptions {
		if opt.Name == "check_option" {
			return "", "view.check_option_updatability"
		}
	}
	if output.AliasCount > len(output.Columns) {
		return "view_output_excess_aliases", ""
	}
	if output.Unlogged {
		return "view_output_unlogged", ""
	}
	lookup := spec.PGCreateViewTargetLookup(s)
	if lookup == nil {
		return "", "view.creation_namespace"
	}
	state := s.Metadata.PGCreateViewTarget
	if state == nil || state.Table.Schema != lookup.Schema || state.Table.Name != lookup.Name {
		return "", "view.output_target"
	}
	if !state.SchemaExists {
		return "view_output_schema_absent", ""
	}
	if output.Temporary {
		return "view_output_temporary_in_persistent_schema", ""
	}
	if state.Kind != "" {
		if s.DDL.Options["replace"] != "true" {
			return "view_output_target_exists", ""
		}
		if state.Kind != "v" {
			return "view_output_target_wrong_kind", ""
		}
		if state.Columns == nil {
			return "", "view.existing_output"
		}
		if len(output.Columns) < len(state.Columns) {
			return "view_output_removed_column", ""
		}
		for i, old := range state.Columns {
			column := output.Columns[i]
			switch {
			case column.Name != old.Name:
				return "view_output_column_name_changed", ""
			case column.TypeOID != old.TypeOID || column.Typmod != old.Typmod:
				return "view_output_column_type_changed", ""
			case column.Collation != old.Collation:
				return "view_output_column_collation_changed", ""
			}
		}
	}
	seen := make(map[string]struct{}, len(output.Columns))
	for _, column := range output.Columns {
		if _, duplicate := seen[column.Name]; duplicate {
			return "view_output_duplicate_column", ""
		}
		seen[column.Name] = struct{}{}
	}
	return "", ""
}

func (r pgCreateViewOutputRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateViewOutput(s)
	if reason == "" {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects the view target or output declaration", Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "view": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateViewOutputRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateViewOutput(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_view_output", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
