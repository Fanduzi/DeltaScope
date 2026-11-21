package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgIndexIncludeRule struct{ level rule.Level }

func newPGIndexIncludeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgIndexIncludeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgIndexIncludeRule) ID() string { return ruleIDPGIndexIncludeValid }

func (r pgIndexIncludeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateIndex && len(s.DDL.Indexes) == 1 && len(s.DDL.Indexes[0].IncludedColumns) != 0
}

func assessPGIndexInclude(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGIndexIncludeLookup(s)
	if lookup == nil {
		return "", "index.target_identity"
	}
	state := s.Metadata.PGIndexInclude
	if state == nil || state.Target.Table.Schema != lookup.Schema || state.Target.Table.Name != lookup.Name {
		return "", "index.include_target"
	}
	switch state.Kind {
	case "":
		return "index_include_target_absent", ""
	case "p":
		return "", "index.partition_key"
	case "r", "m":
	default:
		return "index_include_target_wrong_kind", ""
	}
	if state.Method == "" || state.Method != lookup.Qualifiers["method"] {
		return "", "index.access_method"
	}
	if state.Method == "hash" || state.Method == "gin" {
		return "index_include_method_unsupported", ""
	}
	if state.Method != "btree" {
		return "", "index.access_method"
	}
	if state.Columns == nil || state.System == nil {
		return "", "index.include_columns"
	}
	for _, column := range s.DDL.Indexes[0].IncludedColumns {
		switch {
		case slices.Contains(state.Columns, column):
		case slices.Contains(state.System, column):
			return "index_include_system_column", ""
		default:
			return "index_include_column_absent", ""
		}
	}
	return "", ""
}

func (r pgIndexIncludeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGIndexInclude(s)
	if reason == "" {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects the index INCLUDE target or declaration", Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgIndexIncludeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGIndexInclude(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_index_include", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
