package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTextSearchChangeRule struct{ level rule.Level }

func newPGTextSearchChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTextSearchChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTextSearchChangeRule) ID() string { return ruleIDPGTextSearchChangeValid }

func (r pgTextSearchChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil && s.DDL.PGTextSearchChange != nil &&
		(s.DDL.Operation == spec.DDLOperationAlterTextSearchConfiguration || s.DDL.Operation == spec.DDLOperationAlterTextSearchDictionary)
}

func assessPGTextSearchChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGTextSearchChangeLookup(s) == nil {
		return "", "text_search.target_identity"
	}
	d := s.DDL.PGTextSearchChange
	state := s.Metadata.PGTextSearchChange
	if state == nil || state.Target != d.Target || state.Kind != d.Kind || state.Action != d.Action || state.Destination != d.Destination || state.Owner != d.Owner {
		return "", "text_search.catalog"
	}
	if d.Action == "set_owner" && !state.OwnerExists {
		if d.Owner.Kind == "named" || d.Owner.Kind == "public" {
			return "text_search_owner_absent", ""
		}
		return "", "text_search.owner_identity"
	}
	if !state.SourceSchema || !state.SourceExists {
		return "text_search_source_absent", ""
	}
	switch d.Action {
	case "set_owner":
		return "", ""
	case "set_schema":
		switch {
		case !state.DestinationSchema:
			return "text_search_destination_schema_absent", ""
		case d.Destination == d.Target.Table.Schema:
			return "", ""
		case state.DestinationForbidden:
			return "text_search_destination_forbidden", ""
		}
		fallthrough
	case "rename":
		if state.DestinationOccupied {
			return "text_search_destination_occupied", ""
		}
	case "set_dictionary_options":
		return assessPGTextSearchDictionary(d, state)
	case "add_mapping", "alter_mapping", "replace_mapping", "drop_mapping":
		return assessPGTextSearchMapping(d, state)
	default:
		return "", "text_search.action"
	}
	return "", ""
}

func assessPGTextSearchDictionary(d *spec.PGTextSearchChange, state *spec.PGTextSearchChangeState) (reason, fact string) {
	if !state.SimpleTemplate {
		return "", "text_search.dictionary_options"
	}
	for i, option := range d.Options {
		if !option.Present || slices.ContainsFunc(d.Options[i+1:], func(later spec.PGTextSearchDictionaryOption) bool { return later.Name == option.Name }) {
			continue
		}
		switch option.Name {
		case "stopwords":
			fact = "text_search.dictionary_options"
		case "accept":
			if !option.BooleanKnown {
				fact = "text_search.dictionary_options"
			} else if !option.BooleanValid {
				return "text_search_dictionary_boolean_invalid", ""
			}
		default:
			return "text_search_dictionary_option_unknown", ""
		}
	}
	return "", fact
}

func assessPGTextSearchMapping(d *spec.PGTextSearchChange, state *spec.PGTextSearchChangeState) (reason, fact string) {
	mapped := false
	if len(d.Tokens) != 0 {
		if !state.BuiltinParser || len(state.Mappings) == 0 {
			return "", "text_search.parser_tokens"
		}
		for _, token := range d.Tokens {
			i := slices.IndexFunc(state.Mappings, func(m spec.PGTextSearchMapping) bool { return m.Token == token })
			if i < 0 {
				return "text_search_token_absent", ""
			}
			mapped = mapped || state.Mappings[i].Mapped
		}
	}
	if !slices.Equal(d.Dictionaries, state.Dictionaries) || len(d.Dictionaries) != len(state.DictionaryExists) {
		return "", "text_search.dictionaries"
	}
	for i, dict := range d.Dictionaries {
		if !(spec.PGDestructiveTableTarget{Table: dict}).Qualified() || dict.Schema == "pg_toast_temp" {
			return "", "text_search.dictionary_identity"
		}
		if !state.DictionaryExists[i] {
			return "text_search_dictionary_absent", ""
		}
	}
	if d.Action == "add_mapping" && mapped {
		return "text_search_mapping_exists", ""
	}
	if d.Action == "drop_mapping" && !d.IfExists {
		for _, token := range d.Tokens {
			if !slices.ContainsFunc(state.Mappings, func(m spec.PGTextSearchMapping) bool { return m.Token == token && m.Mapped }) {
				return "text_search_mapping_absent", ""
			}
		}
	}
	return "", ""
}

func (r pgTextSearchChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTextSearchChange(s)
	messages := map[string]string{
		"text_search_dictionary_boolean_invalid": "The PostgreSQL simple dictionary ACCEPT option requires a Boolean value",
		"text_search_dictionary_option_unknown":  "The PostgreSQL simple dictionary does not recognize this option",
		"text_search_owner_absent":               "The requested PostgreSQL text search owner does not exist",
		"text_search_source_absent":              "The requested PostgreSQL text search object does not exist",
		"text_search_destination_schema_absent":  "The PostgreSQL text search destination schema does not exist",
		"text_search_destination_forbidden":      "PostgreSQL disallows this text search namespace change",
		"text_search_destination_occupied":       "A PostgreSQL text search object of the same kind occupies the destination",
		"text_search_token_absent":               "The PostgreSQL text search parser has no token of the requested name",
		"text_search_dictionary_absent":          "A requested PostgreSQL text search dictionary does not exist",
		"text_search_mapping_exists":             "A PostgreSQL text search token already has a mapping",
		"text_search_mapping_absent":             "A requested PostgreSQL text search mapping does not exist",
	}
	message := messages[reason]
	if message == "" {
		return nil, nil
	}
	d := s.DDL.PGTextSearchChange
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": d.Target.Table.Schema, "name": d.Target.Table.Name, "kind": d.Kind, "reason_code": reason,
	}}}, nil
}

func (r pgTextSearchChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTextSearchChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_text_search_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
