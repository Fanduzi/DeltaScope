package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgConstraintUsingIndexRule struct{ level rule.Level }

func newPGConstraintUsingIndexRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintUsingIndexRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgConstraintUsingIndexRule) ID() string { return ruleIDPGConstraintUsingIndexValid }

func (r pgConstraintUsingIndexRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || !pgColumnIdentityAction(s, "add_constraint") {
		return false
	}
	for _, a := range s.DDL.Alter {
		kind := a.Options["constraint_type"]
		if a.Action == "add_constraint" && a.Options["using_index"] == "true" && (kind == "primary_key" || kind == "unique") {
			return true
		}
	}
	return false
}

func assessPGConstraintUsingIndex(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	const attachment = "target_table.pg_constraint_index"
	f := boundPGConstraintUsingIndex(s)
	if f == nil {
		return "", attachment
	}
	switch {
	case f.Kind == "":
		return "constraint_index_absent", ""
	case f.Kind != "i":
		return "constraint_index_wrong_kind", ""
	case !f.IndexKnown:
		return "", attachment
	case f.ConstraintOwned:
		return "constraint_index_already_owned", ""
	case !f.OnTable:
		return "constraint_index_wrong_table", ""
	case !f.Valid:
		return "constraint_index_invalid", ""
	case !f.Unique:
		return "constraint_index_not_unique", ""
	case f.Expression:
		return "constraint_index_has_expression", ""
	case f.Partial:
		return "constraint_index_partial", ""
	case !f.BTree:
		return "constraint_index_not_btree", ""
	case f.KeyDefaultsKnown && !f.KeyDefaults:
		return "constraint_index_nondefault_key", ""
	case f.OpclassesDefaultKnown && !f.OpclassesDefault:
		return "constraint_index_nondefault_opclass", ""
	case f.NullsNotDistinct && s.DDL.Alter[0].Options["constraint_type"] == "primary_key":
		return "constraint_index_nulls_not_distinct_primary", ""
	case !f.KeyDefaultsKnown:
		return "", "index.constraint_key_defaults"
	case !f.OpclassesDefaultKnown:
		return "", "index.constraint_default_opclasses"
	}
	return "", ""
}

func boundPGConstraintUsingIndex(s spec.Statement) *spec.PGConstraintUsingIndex {
	name := spec.PGConstraintUsingIndexTarget(s)
	if name == "" || !pgConstraintTargetVersionKnown(s) || !spec.PGOrdinaryTableBound(s) {
		return nil
	}
	f, table := s.Metadata.PGConstraintUsingIndex, s.DDL.Table
	if f == nil || f.Schema != table.Schema || f.Table != table.Name || f.Name != name {
		return nil
	}
	return f
}

func (r pgConstraintUsingIndexRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGConstraintUsingIndex(s)
	var message string
	switch reason {
	case "constraint_index_absent":
		message = "The selected PostgreSQL index does not exist in the table schema"
	case "constraint_index_wrong_kind":
		message = "The selected PostgreSQL relation is not an ordinary index"
	case "constraint_index_already_owned":
		message = "A PostgreSQL constraint already owns the selected index"
	case "constraint_index_wrong_table":
		message = "The selected PostgreSQL index belongs to a different table"
	case "constraint_index_invalid":
		message = "The selected PostgreSQL index is invalid"
	case "constraint_index_not_unique":
		message = "PostgreSQL requires a unique index for this attachment"
	case "constraint_index_has_expression":
		message = "PostgreSQL cannot attach this constraint to an expression index"
	case "constraint_index_partial":
		message = "PostgreSQL cannot attach this constraint to a partial index"
	case "constraint_index_not_btree":
		message = "PostgreSQL requires a btree index for this attachment"
	case "constraint_index_nondefault_key":
		message = "PostgreSQL requires default sorting, collation and options for the index key columns"
	case "constraint_index_nondefault_opclass":
		message = "PostgreSQL requires default operator classes on the selected index key columns"
	case "constraint_index_nulls_not_distinct_primary":
		message = "PostgreSQL cannot attach a NULLS NOT DISTINCT index as a primary key"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"action": "add_constraint", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"constraint_type": s.DDL.Alter[0].Options["constraint_type"], "reason_code": reason},
	}}, nil
}

func (r pgConstraintUsingIndexRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGConstraintUsingIndex(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_constraint_using_index", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
