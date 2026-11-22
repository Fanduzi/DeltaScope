package ddl

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mysqlStoredObjectRule struct{ level rule.Level }

func newMySQLStoredObjectRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlStoredObjectRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r mysqlStoredObjectRule) ID() string { return ruleIDMySQLStoredObjectValid }

func (r mysqlStoredObjectRule) AppliesTo(s spec.Statement) bool {
	return spec.MySQLStoredObjectLookup(s) != nil
}

func assessMySQLStoredObject(s spec.Statement) (reason, fact string) {
	lookup := spec.MySQLStoredObjectLookup(s)
	if lookup == nil || s.Metadata == nil || s.Metadata.MySQLStoredObject == nil {
		return "", "stored_object.identity"
	}
	observed := s.Metadata.MySQLStoredObject
	target := spec.MySQLStoredObjectTarget{Kind: lookup.Qualifiers["kind"], Name: spec.Table{Schema: lookup.Schema, Name: lookup.Name},
		Related: spec.Table{Schema: lookup.Qualifiers["related_schema"], Name: lookup.Qualifiers["related_name"]}}
	if observed.Target != target {
		return "", "stored_object.identity"
	}
	switch s.DDL.Operation {
	case spec.DDLOperationCreateTrigger:
		d := s.DDL.Trigger
		if observed.SourceSeen {
			return "trigger_name_occupied", ""
		}
		if d.Order != nil {
			if !observed.RelatedSeen {
				return "", "trigger.ordering_sibling"
			}
			if !strings.EqualFold(observed.SiblingTable.Name, d.CatalogTable.Name) || observed.Event != d.Event || observed.Timing != d.Timing {
				return "trigger_order_group_mismatch", ""
			}
			if observed.SiblingTable != d.CatalogTable {
				return "", "trigger.ordering_sibling"
			}
		}
		return "", "stored_object.presence"
	case spec.DDLOperationCreateEvent:
		if observed.SourceSeen {
			return "event_name_occupied", ""
		}
		return "", "stored_object.presence"
	case spec.DDLOperationAlterEvent:
		if !observed.SourceSeen {
			return "", "stored_object.presence"
		}
		if target.Related.Name != "" {
			if target.Name.Schema == target.Related.Schema && strings.EqualFold(target.Name.Name, target.Related.Name) &&
				!strings.ContainsFunc(target.Name.Name+target.Related.Name, func(c rune) bool { return c > 127 }) {
				return "event_rename_same_identity", ""
			}
			if observed.RelatedSeen {
				return "event_destination_occupied", ""
			}
			return "", "event.destination_presence"
		}
	case spec.DDLOperationDropEvent, spec.DDLOperationDropTrigger:
		if !observed.SourceSeen {
			return "", "stored_object.presence"
		}
	}
	return "", ""
}

func (r mysqlStoredObjectRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessMySQLStoredObject(s)
	message := map[string]string{
		"trigger_name_occupied":        "A MySQL trigger already occupies the target name",
		"trigger_order_group_mismatch": "The MySQL trigger ordering target belongs to a different table, timing or event",
		"event_name_occupied":          "A MySQL event already occupies the target name",
		"event_rename_same_identity":   "MySQL cannot rename an event to its current identity",
		"event_destination_occupied":   "A MySQL event already occupies the rename destination",
	}[reason]
	if message == "" {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"object_type": s.DDL.ObjectType, "reason_code": reason,
	}}}, nil
}

func (r mysqlStoredObjectRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessMySQLStoredObject(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_mysql_stored_object", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
