package ddl

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgSubscriptionChangeRule struct{ level rule.Level }

func newPGSubscriptionChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgSubscriptionChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgSubscriptionChangeRule) ID() string { return ruleIDPGSubscriptionChangeValid }

func (r pgSubscriptionChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil && s.DDL.PGSubscriptionChange != nil
}

func assessPGSubscriptionChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGSubscriptionChangeLookup(s) == nil {
		return "", "subscription.identity"
	}
	d := s.DDL.PGSubscriptionChange
	state := s.Metadata.PGSubscriptionChange
	if state == nil || state.Name != d.Name || state.RenameTo != d.RenameTo || state.Owner != d.Owner {
		return "", "subscription.catalog"
	}
	if d.Action == "set_owner" && !state.OwnerExists {
		return "subscription_owner_absent", ""
	}
	if !state.Exists {
		if d.Action == "drop" && d.IfExists {
			return "", ""
		}
		return "subscription_absent", ""
	}
	switch d.Action {
	case "rename":
		if state.DestinationOccupied {
			return "subscription_destination_occupied", ""
		}
	case "set_owner", "disable":
	case "enable":
		if !state.SlotPresent {
			return "subscription_slot_required", ""
		}
	case "drop":
		if state.SlotPresent {
			return "", "subscription.publisher_slot_drop"
		}
	case "connection":
		return "", "subscription.connection_validation"
	case "refresh", "set_publication", "add_publication", "drop_publication", "set_options", "skip":
		return assessPGSubscriptionOptions(d, state)
	default:
		return "", "subscription.action"
	}
	return "", ""
}

func pgReplicationBoolean(option spec.PGReplicationOption) (bool, bool) {
	if option.Bare {
		return true, true
	}
	if option.Integer {
		return option.Text == "1", option.Text == "0" || option.Text == "1"
	}
	switch strings.ToLower(option.Text) {
	case "true", "on":
		return true, true
	case "false", "off":
		return false, true
	}
	return false, false
}

func assessPGSubscriptionOptions(d *spec.PGSubscriptionChange, state *spec.PGSubscriptionChangeState) (reason, fact string) {
	if d.Action == "refresh" && !state.Enabled {
		return "subscription_disabled_refresh", ""
	}
	allowed := map[string][]string{
		"refresh": {"copy_data"}, "set_publication": {"refresh", "copy_data"},
		"add_publication": {"refresh", "copy_data"}, "drop_publication": {"refresh", "copy_data"},
		"set_options": {"slot_name", "synchronous_commit", "binary", "streaming", "disable_on_error", "password_required", "run_as_owner", "failover", "origin"},
		"skip":        {"lsn"},
	}
	seen := make(map[string]bool, len(d.Options))
	refresh, copyData, removeSlot := true, true, false
	for _, option := range d.Options {
		if !slices.Contains(allowed[d.Action], option.Name) {
			return "subscription_option_unknown", ""
		}
		if seen[option.Name] {
			return "subscription_option_duplicate", ""
		}
		seen[option.Name] = true
		if !option.Known && !option.Bare {
			return "", "subscription.option_value"
		}
		switch option.Name {
		case "lsn":
			if option.Text == "none" {
				continue
			}
			hi, lo, ok := strings.Cut(option.Text, "/")
			a, aErr := strconv.ParseUint(hi, 16, 32)
			b, bErr := strconv.ParseUint(lo, 16, 32)
			if !ok || len(hi) > 8 || len(lo) > 8 || aErr != nil || bErr != nil || a == 0 && b == 0 {
				return "subscription_lsn_invalid", ""
			}
			fact = "subscription.replication_origin_progress"
		case "slot_name":
			if option.Text == "none" {
				removeSlot = true
			} else if option.Text == "" || len(option.Text) > 63 || strings.ContainsFunc(option.Text, func(c rune) bool {
				letter := c >= 'a' && c <= 'z'
				digit := c >= '0' && c <= '9'
				return !letter && !digit && c != '_'
			}) {
				return "subscription_slot_invalid", ""
			}
		case "origin":
			if !strings.EqualFold(option.Text, "none") && !strings.EqualFold(option.Text, "any") {
				return "subscription_option_value_invalid", ""
			}
		case "synchronous_commit":
			if !slices.Contains([]string{"local", "remote_write", "remote_apply", "on", "off", "true", "false", "yes", "no", "1", "0"}, strings.ToLower(option.Text)) {
				return "subscription_option_value_invalid", ""
			}
		default:
			value, valid := pgReplicationBoolean(option)
			parallel := option.Name == "streaming" && strings.EqualFold(option.Text, "parallel")
			if !valid && !parallel {
				return "subscription_option_value_invalid", ""
			}
			switch option.Name {
			case "refresh":
				refresh = value
			case "copy_data":
				copyData = value
			}
		}
	}
	if d.Action == "skip" {
		return "", fact
	}
	if d.Action == "set_options" {
		if removeSlot && state.Enabled {
			return "subscription_enabled_slot_clear", ""
		}
		if seen["failover"] {
			if !state.SlotPresent {
				return "subscription_slot_required", ""
			}
			if state.Enabled {
				return "subscription_enabled_failover", ""
			}
			return "", "subscription.publisher_slot_alteration"
		}
		return "", ""
	}
	if d.Action != "refresh" {
		if reason := assessSubscriptionPublications(d, state.Publications); reason != "" {
			return reason, ""
		}
	}
	if refresh {
		if !state.Enabled {
			return "subscription_disabled_refresh", ""
		}
		if state.TwoPhaseEnabled && copyData {
			return "subscription_two_phase_copy", ""
		}
		return "", "subscription.publisher_refresh"
	}
	return "", ""
}

func assessSubscriptionPublications(d *spec.PGSubscriptionChange, prior []string) string {
	for index, name := range d.Publications {
		if slices.Contains(d.Publications[:index], name) {
			return "subscription_publication_duplicate"
		}
	}
	switch d.Action {
	case "add_publication":
		for _, name := range d.Publications {
			if slices.Contains(prior, name) {
				return "subscription_publication_exists"
			}
		}
	case "drop_publication":
		for _, name := range d.Publications {
			if !slices.Contains(prior, name) {
				return "subscription_publication_absent"
			}
		}
		if len(d.Publications) == len(prior) {
			return "subscription_publications_empty"
		}
	}
	return ""
}

func (r pgSubscriptionChangeRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGSubscriptionChange(s)
	messages := map[string]string{
		"subscription_owner_absent":          "The requested PostgreSQL subscription owner does not exist",
		"subscription_absent":                "The requested PostgreSQL subscription does not exist in the current database",
		"subscription_destination_occupied":  "A PostgreSQL subscription occupies the requested name",
		"subscription_slot_required":         "The PostgreSQL subscription requires a slot name for this action",
		"subscription_slot_invalid":          "The PostgreSQL subscription slot name is invalid",
		"subscription_enabled_slot_clear":    "PostgreSQL requires a disabled subscription to clear its slot name",
		"subscription_enabled_failover":      "PostgreSQL requires a disabled subscription to change failover",
		"subscription_disabled_refresh":      "PostgreSQL requires an enabled subscription to refresh publications",
		"subscription_two_phase_copy":        "PostgreSQL disallows refresh with copy_data while two-phase replication is enabled",
		"subscription_publication_duplicate": "The PostgreSQL subscription statement repeats a publication name",
		"subscription_publication_exists":    "The PostgreSQL subscription already contains a requested publication",
		"subscription_publication_absent":    "The PostgreSQL subscription lacks a requested publication",
		"subscription_publications_empty":    "PostgreSQL requires the subscription to retain at least one publication",
		"subscription_option_unknown":        "PostgreSQL does not recognize an option for this subscription action",
		"subscription_option_duplicate":      "The PostgreSQL subscription statement repeats an option",
		"subscription_option_value_invalid":  "A PostgreSQL subscription option value is invalid",
		"subscription_lsn_invalid":           "The PostgreSQL subscription SKIP location is invalid",
	}
	if message := messages[reason]; message != "" {
		return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
			"name": s.DDL.PGSubscriptionChange.Name, "reason_code": reason,
		}}}, nil
	}
	return nil, nil
}

func (r pgSubscriptionChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGSubscriptionChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_subscription_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
