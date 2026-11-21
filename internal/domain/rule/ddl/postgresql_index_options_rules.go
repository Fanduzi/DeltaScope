package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgIndexOptionsRule struct{ level rule.Level }

func newPGIndexOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgIndexOptionsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgIndexOptionsRule) ID() string { return ruleIDPGIndexOptionsValid }

func (r pgIndexOptionsRule) AppliesTo(s spec.Statement) bool {
	return spec.PGIndexOptionsAction(s) != ""
}

func assessPGIndexOptions(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	lookup := spec.PGIndexOptionsLookup(s)
	if lookup == nil {
		return "", "index.target_identity"
	}
	state := s.Metadata.PGIndexOptions
	if state == nil || state.Target.Table.Schema != lookup.Schema || state.Target.Table.Name != lookup.Name {
		return "", "index.storage_state"
	}
	if state.Kind == "" {
		if s.DDL.PGTableIfExists {
			return "", ""
		}
		return "index_options_target_absent", ""
	}
	if state.Kind != "i" {
		return "index_options_target_wrong_kind", ""
	}

	options := s.DDL.Alter[0].PGTableReloptions
	if spec.PGIndexOptionsAction(s) == "reset_options" {
		for _, option := range options {
			if option.HasValue {
				return "index_option_reset_has_value", ""
			}
		}
		if state.Method == "" {
			return "", "index.access_method"
		}
		return "", ""
	}
	for _, option := range options {
		if option.Namespace != "" && option.Namespace != "toast" {
			return "index_option_namespace", ""
		}
	}
	if state.Method != "btree" && state.Method != "hash" {
		return "", "index.access_method"
	}
	seen := make(map[string]bool, 2)
	for _, option := range options {
		if option.Namespace == "toast" {
			continue
		}
		switch option.Name {
		case "fillfactor":
			if !option.HasValue {
				return "index_option_value_required", ""
			}
			if option.Value != spec.PGTableReloptionInteger {
				fact = "index.storage_options"
			} else if option.Integer < 10 || option.Integer > 100 {
				return "index_option_out_of_range", ""
			}
		case "deduplicate_items":
			if state.Method != "btree" {
				return "index_option_unrecognized", ""
			}
			if option.Value == spec.PGTableReloptionInvalidBoolean {
				return "index_option_invalid_boolean", ""
			}
			if option.Value != spec.PGTableReloptionBoolean {
				fact = "index.storage_options"
			}
		case "autovacuum_enabled":
			return "index_option_unrecognized", ""
		default:
			fact = "index.storage_options"
			continue
		}
		if seen[option.Name] {
			return "index_option_duplicate", ""
		}
		seen[option.Name] = true
	}
	return "", fact
}

func (r pgIndexOptionsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGIndexOptions(s)
	if reason == "" {
		return nil, nil
	}
	message := "PostgreSQL rejects the index storage option declaration"
	switch reason {
	case "index_options_target_absent":
		message = "The PostgreSQL ALTER INDEX target does not exist"
	case "index_options_target_wrong_kind":
		message = "PostgreSQL requires an ordinary index for this storage option change"
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": s.DDL.Table.Schema, "index": s.DDL.Table.Name, "reason_code": reason,
	}}}, nil
}

func (r pgIndexOptionsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGIndexOptions(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_index_options", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
