package ddl

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgPublicationChangeRule struct{ level rule.Level }

func newPGPublicationChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgPublicationChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgPublicationChangeRule) ID() string { return ruleIDPGPublicationChangeValid }

func (r pgPublicationChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil && s.DDL.PGPublicationChange != nil
}

func assessPGPublicationChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGPublicationChangeLookup(s) == nil {
		return "", "publication.identity"
	}
	d := s.DDL.PGPublicationChange
	state := s.Metadata.PGPublicationChange
	if state == nil || state.Name != d.Name || state.RenameTo != d.RenameTo || state.Owner != d.Owner {
		return "", "publication.catalog"
	}
	if d.Action == "set_owner" && !state.OwnerExists {
		return "publication_owner_absent", ""
	}
	if !state.Exists {
		return "publication_absent", ""
	}
	switch d.Action {
	case "set_owner":
	case "rename":
		if state.DestinationOccupied {
			return "publication_destination_occupied", ""
		}
	case "set_options":
		return assessPGPublicationOptions(d.Options, state)
	case "add_table", "drop_table", "set_table":
		return assessPGPublicationMembers(d, state)
	default:
		return "", "publication.action"
	}
	return "", ""
}

func assessPGPublicationMembers(d *spec.PGPublicationChange, state *spec.PGPublicationChangeState) (reason, fact string) {
	if len(state.Members) != len(d.Members) || len(d.Members) == 0 {
		return "", "publication.members"
	}
	hasSchema := false
	for i, member := range d.Members {
		observed := state.Members[i]
		if observed.Target != member {
			return "", "publication.members"
		}
		switch member.Kind {
		case "schema":
			if member.Schema == "pg_temp" {
				return "", "publication.schema_identity"
			}
			hasSchema = true
			if !observed.Exists {
				return "publication_schema_absent", ""
			}
		case "current_schema":
			return "", "publication.schema_identity"
		case "table":
		default:
			return "", "publication.member_identity"
		}
	}
	if state.AllTables {
		return "publication_all_tables", ""
	}
	for i, member := range d.Members {
		if member.Kind != "table" {
			continue
		}
		observed := state.Members[i]
		if !member.Table.Qualified() {
			return "", "publication.relation_identity"
		}
		if !observed.Exists {
			return "publication_relation_absent", ""
		}
		if observed.Inherits && !member.Table.Only {
			return "", "publication.inherited_members"
		}
	}
	for i, member := range d.Members {
		if member.Kind != "table" {
			continue
		}
		observed := state.Members[i]
		if d.Action == "drop_table" {
			switch {
			case member.HasColumns:
				return "publication_drop_column_list", ""
			case !observed.Member:
				return "publication_relation_not_member", ""
			case member.HasRowFilter:
				return "publication_drop_row_filter", ""
			}
			continue
		}
		if member.HasColumns || member.HasRowFilter {
			return "", "publication.relation_specification"
		}
		if !observed.Eligible {
			return "publication_relation_ineligible", ""
		}
		if d.Action == "add_table" && observed.Member {
			return "publication_relation_already_member", ""
		}
	}
	if hasSchema && d.Action == "add_table" && state.HasColumns {
		return "publication_schema_column_list_conflict", ""
	}
	for i, member := range d.Members {
		if member.Kind != "schema" {
			continue
		}
		observed := state.Members[i]
		switch {
		case d.Action == "drop_table" && !observed.Member:
			return "publication_schema_not_member", ""
		case d.Action == "add_table" && observed.Member:
			return "publication_schema_already_member", ""
		case d.Action != "drop_table" && !observed.Eligible:
			return "publication_schema_ineligible", ""
		}
	}
	return "", ""
}

func assessPGPublicationOptions(options []spec.PGReplicationOption, state *spec.PGPublicationChangeState) (reason, fact string) {
	seen := make(map[string]bool, len(options))
	withoutPartitionRoot := false
	for _, option := range options {
		if seen[option.Name] {
			return "publication_option_duplicate", ""
		}
		seen[option.Name] = true
		switch option.Name {
		case "publish":
			if !option.Known && !option.Bare || strings.ContainsFunc(option.Text, func(c rune) bool { return c > 127 }) {
				return "", "publication.publish_option"
			}
			if option.Bare || !validPGPublicationActions(option.Text) {
				return "publication_actions_invalid", ""
			}
		case "publish_via_partition_root":
			if !option.Known && !option.Bare {
				return "", "publication.partition_root_option"
			}
			value, valid := pgReplicationBoolean(option)
			if !valid {
				return "publication_boolean_invalid", ""
			}
			withoutPartitionRoot = !value
		default:
			return "publication_option_unknown", ""
		}
	}
	if withoutPartitionRoot && !state.AllTables && state.HasPartitionSpec {
		return "publication_partition_spec_conflict", ""
	}
	return "", ""
}

func validPGPublicationActions(value string) bool {
	const whitespace = " \t\n\r\f\v"
	value = strings.Trim(value, whitespace)
	if value == "" {
		return true
	}
	for {
		var action string
		if strings.HasPrefix(value, `"`) {
			end := strings.IndexByte(value[1:], '"')
			if end < 0 {
				return false
			}
			action, value = value[1:end+1], value[end+2:]
		} else {
			end := strings.IndexAny(value, ","+whitespace)
			if end < 0 {
				end = len(value)
			}
			action, value = strings.ToLower(value[:end]), value[end:]
		}
		switch action {
		case "insert", "update", "delete", "truncate":
		default:
			return false
		}
		value = strings.TrimLeft(value, whitespace)
		if value == "" {
			return true
		}
		if value[0] != ',' {
			return false
		}
		value = strings.TrimLeft(value[1:], whitespace)
	}
}

func (r pgPublicationChangeRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGPublicationChange(s)
	messages := map[string]string{
		"publication_absent":                      "The requested PostgreSQL publication does not exist",
		"publication_owner_absent":                "The requested PostgreSQL publication owner does not exist",
		"publication_destination_occupied":        "A PostgreSQL publication occupies the requested name",
		"publication_all_tables":                  "A FOR ALL TABLES publication disallows explicit member changes",
		"publication_schema_absent":               "A requested PostgreSQL publication schema does not exist",
		"publication_schema_not_member":           "A requested schema is absent from the PostgreSQL publication",
		"publication_schema_already_member":       "The PostgreSQL publication already contains a requested schema",
		"publication_schema_ineligible":           "A requested schema is ineligible for the PostgreSQL publication",
		"publication_schema_column_list_conflict": "PostgreSQL disallows adding schemas to a publication with column lists",
		"publication_relation_absent":             "A requested PostgreSQL publication relation does not exist",
		"publication_relation_not_member":         "A requested relation is absent from the PostgreSQL publication",
		"publication_relation_already_member":     "The PostgreSQL publication already contains a requested relation",
		"publication_relation_ineligible":         "A requested relation is ineligible for the PostgreSQL publication",
		"publication_drop_column_list":            "PostgreSQL disallows a column list when dropping a publication member",
		"publication_drop_row_filter":             "PostgreSQL disallows a row filter when dropping a publication member",
		"publication_option_unknown":              "PostgreSQL does not recognize a publication option",
		"publication_option_duplicate":            "The PostgreSQL publication statement repeats an option",
		"publication_actions_invalid":             "The PostgreSQL publication action list is invalid",
		"publication_boolean_invalid":             "A PostgreSQL publication Boolean option is invalid",
		"publication_partition_spec_conflict":     "PostgreSQL requires publication via the partition root for an existing root row filter or column list",
	}
	if message := messages[reason]; message != "" {
		return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
			"name": s.DDL.PGPublicationChange.Name, "reason_code": reason,
		}}}, nil
	}
	return nil, nil
}

func (r pgPublicationChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGPublicationChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_publication_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
