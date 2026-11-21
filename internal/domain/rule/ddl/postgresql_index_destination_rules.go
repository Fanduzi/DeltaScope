package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgIndexDestinationRule struct{ level rule.Level }

func newPGIndexDestinationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgIndexDestinationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgIndexDestinationRule) ID() string { return ruleIDPGIndexDestinationValid }

func (r pgIndexDestinationRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterIndex && s.DDL.PGIndexDestination != nil
}

func assessPGIndexDestination(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGIndexDestinationLookup(s) == nil {
		return "", "index.target_identity"
	}
	target, state := s.DDL.PGIndexDestination, s.Metadata.PGIndexDestination
	if state == nil || state.Source != target.Source || state.Action != target.Action || state.Destination != target.Destination {
		return "", "index.destination_state"
	}
	if state.Kind == "" {
		if target.IfExists {
			return "", ""
		}
		return "index_destination_source_absent", ""
	}
	if target.Action == "rename" {
		switch {
		case state.Kind == "c":
			return "index_destination_source_wrong_kind", ""
		case state.DestinationExists:
			return "index_rename_name_occupied", ""
		case state.HasRowType:
			return "", "index.rename_row_type"
		case state.ConstraintBacked:
			return "", "index.rename_constraint"
		}
		return "", ""
	}
	switch {
	case state.Kind != "i" && state.Kind != "I":
		return "index_destination_source_wrong_kind", ""
	case !state.DestinationExists:
		return "index_tablespace_absent", ""
	case state.SameTablespace:
		return "", ""
	case state.Mapped:
		return "index_tablespace_mapped_relation", ""
	case state.Global:
		return "index_tablespace_global_inapplicable", ""
	}
	return "", ""
}

func (r pgIndexDestinationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGIndexDestination(s)
	if reason == "" {
		return nil, nil
	}
	name := s.DDL.PGIndexDestination.Source.Table
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects this ALTER INDEX source or destination", Metadata: map[string]any{
		"schema": name.Schema, "relation": name.Name, "reason_code": reason,
	}}}, nil
}

func (r pgIndexDestinationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGIndexDestination(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_index_destination", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
