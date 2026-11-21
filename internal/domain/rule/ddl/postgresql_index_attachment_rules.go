package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgIndexAttachmentRule struct{ level rule.Level }

func newPGIndexAttachmentRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgIndexAttachmentRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgIndexAttachmentRule) ID() string { return ruleIDPGIndexAttachmentValid }

func (r pgIndexAttachmentRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil && s.DDL.PGIndexAttachment != nil
}

func assessPGIndexAttachment(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGIndexAttachmentLookup(s) == nil {
		return "", "index.attachment_identity"
	}
	state := s.Metadata.PGIndexAttachment
	if state == nil || state.Target != *s.DDL.PGIndexAttachment {
		return "", "index.attachment_state"
	}
	switch {
	case state.ParentKind == "":
		return "index_attachment_parent_absent", ""
	case state.ParentKind != "I":
		return "index_attachment_parent_wrong_kind", ""
	case state.ChildKind == "":
		return "index_attachment_child_absent", ""
	case state.ChildKind != "I" && state.ChildKind != "i":
		return "index_attachment_child_wrong_kind", ""
	case state.SameParent:
		return "", ""
	case state.PartitionInUse:
		return "index_attachment_partition_in_use", ""
	case state.OtherParent:
		return "index_attachment_child_already_attached", ""
	case state.DetachPending:
		return "", "index.partition_visibility"
	case !state.TablePartition:
		return "index_attachment_child_table_not_partition", ""
	case state.Definition == "match":
		return "", ""
	case state.Definition == "mismatch":
		return "index_attachment_definition_mismatch", ""
	case state.Definition == "constraint_absent":
		return "index_attachment_child_constraint_absent", ""
	default:
		return "", "index.definition_compatibility"
	}
}

func (r pgIndexAttachmentRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGIndexAttachment(s)
	if reason == "" {
		return nil, nil
	}
	target := s.DDL.PGIndexAttachment.Parent.Table
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects the index partition attachment", Metadata: map[string]any{
		"schema": target.Schema, "index": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgIndexAttachmentRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGIndexAttachment(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_index_attachment", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
