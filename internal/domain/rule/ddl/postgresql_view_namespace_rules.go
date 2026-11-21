package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgViewNamespaceRule struct{ level rule.Level }

func newPGViewNamespaceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgViewNamespaceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgViewNamespaceRule) ID() string { return ruleIDPGViewNamespaceValid }

func (r pgViewNamespaceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.PGViewNamespaceSource != nil
}

func assessPGViewNamespace(s spec.Statement) (reason, missing string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	lookup := spec.PGViewNamespaceLookup(s)
	if lookup == nil {
		return "", "view.target_identity"
	}
	state := s.Metadata.PGTableNamespace
	if state == nil || !state.SourceKindKnown || state.Schema != lookup.Schema || state.Table != lookup.Qualifiers["table"] ||
		state.Action != lookup.Qualifiers["action"] || state.Destination != lookup.Name {
		return "", "view.pg_namespace_destination"
	}
	if state.SourceKind == "" {
		if s.DDL.PGViewNamespaceSource.IfExists {
			return "", ""
		}
		return "view_namespace_source_absent", ""
	}
	if state.SourceKind != lookup.Qualifiers["kind"] {
		return "view_namespace_source_wrong_kind", ""
	}
	reason, missing = assessPGNamespaceDestination(state)
	if missing != "" {
		return "", "view.pg_namespace_destination"
	}
	return reason, ""
}

func (r pgViewNamespaceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGViewNamespace(s)
	var message string
	switch reason {
	case "view_namespace_source_absent":
		message = "The PostgreSQL view target does not exist"
	case "view_namespace_source_wrong_kind":
		message = "The PostgreSQL view target has the wrong relation kind"
	case "destination_schema_absent":
		message = "The requested PostgreSQL destination schema does not exist"
	case "destination_relation_conflict":
		message = "The PostgreSQL view's destination relation name is already in use"
	case "destination_row_type_conflict":
		message = "The PostgreSQL view's destination row-type name is already in use"
	case "destination_array_type_conflict":
		message = "The PostgreSQL view's associated array-type name is already in use in the destination schema"
	case "destination_index_conflict":
		message = "The destination schema contains a relation with the name of an attached index"
	case "destination_owned_sequence_conflict":
		message = "The destination schema contains a relation with the name of an owned sequence"
	default:
		return nil, nil
	}
	source := s.DDL.PGViewNamespaceSource
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": source.Schema, "view": source.Name, "reason_code": reason,
	}}}, nil
}

func (r pgViewNamespaceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGViewNamespace(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_view_namespace", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
