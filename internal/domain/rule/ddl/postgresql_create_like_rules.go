package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateLikeRule struct{ level rule.Level }

func newPGCreateLikeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateLikeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateLikeRule) ID() string { return ruleIDPGCreateLikeValid }

func (r pgCreateLikeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && len(s.DDL.PGCreateLike) > 0
}

func assessPGCreateLike(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGCreateLikeLookup(s)
	if lookup == nil {
		return "", "create_like.target_identity"
	}
	state := s.Metadata.PGCreateLike
	if state == nil || state.Target.Table != *s.DDL.Table || len(state.Sources) != len(s.DDL.PGCreateLike) {
		return "", "create_like.catalog_state"
	}
	if !state.SchemaExists {
		return "create_like_schema_absent", ""
	}
	if state.Exists && s.DDL.Options["if_not_exists"] == "true" {
		return "", ""
	}
	for i, source := range s.DDL.PGCreateLike {
		got := state.Sources[i]
		if !source.Target.Qualified() || got.Target != source.Target {
			return "", "create_like.source_identity"
		}
		switch got.Kind {
		case "":
			return "create_like_source_absent", ""
		case "r", "v", "m", "c", "f", "p":
		default:
			return "create_like_source_wrong_kind", ""
		}
	}
	columns := make(map[string]bool)
	for _, c := range s.DDL.Columns {
		if columns[c.Name] {
			return "create_like_column_duplicate", ""
		}
		columns[c.Name] = true
	}
	for _, got := range state.Sources {
		if got.Columns == nil {
			return "", "create_like.source_columns"
		}
		for _, name := range got.Columns {
			if columns[name] {
				return "create_like_column_duplicate", ""
			}
			columns[name] = true
		}
	}
	if state.Exists {
		return "create_like_destination_occupied", ""
	}
	for _, source := range s.DDL.PGCreateLike {
		if source.Options != 0 {
			return "", "create_like.copied_options"
		}
	}
	return "", ""
}

func (r pgCreateLikeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateLike(s)
	var message string
	switch reason {
	case "create_like_schema_absent":
		message = "The PostgreSQL destination schema does not exist"
	case "create_like_source_absent":
		message = "A PostgreSQL CREATE TABLE LIKE source does not exist"
	case "create_like_source_wrong_kind":
		message = "A PostgreSQL CREATE TABLE LIKE source has an inapplicable relation kind"
	case "create_like_column_duplicate":
		message = "A column is declared more than once after PostgreSQL copies the LIKE sources"
	case "create_like_destination_occupied":
		message = "The PostgreSQL CREATE TABLE LIKE destination already exists"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateLikeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateLike(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_like", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
