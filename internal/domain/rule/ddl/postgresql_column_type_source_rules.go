package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnTypeSourceRule struct{ level rule.Level }

func newPGColumnTypeSourceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTypeSourceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgColumnTypeSourceRule) ID() string { return ruleIDPGAlterColumnTypeSourceValid }

func (r pgColumnTypeSourceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, "set_data_type")
}

func assessPGColumnTypeSource(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	if len(s.DDL.Alter) != 1 || !spec.PGOrdinaryTableBound(s) {
		return "", "target_table.pg_columns"
	}
	member, known := pgColumnTargetState(s, 0)
	switch {
	case !known:
		return "", "target_table.pg_columns"
	case member == nil:
		return "column_absent", ""
	case member.System:
		return "system_column_inapplicable", ""
	case member.Generated == "s" && s.DDL.Alter[0].Options["has_using"] == "true":
		return "generated_column_using_inapplicable", ""
	}
	return "", ""
}

func (r pgColumnTypeSourceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGColumnTypeSource(s)
	if reason == "" {
		return nil, nil
	}
	message := "The source column does not exist on the bound PostgreSQL table"
	switch reason {
	case "system_column_inapplicable":
		message = "PostgreSQL cannot change the type of a system column"
	case "generated_column_using_inapplicable":
		message = "PostgreSQL does not accept USING when changing a generated column's type"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: "Verify the exact user column and whether the type change accepts a USING clause",
		Metadata: map[string]any{"action": "set_data_type", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"column": s.DDL.Alter[0].Name, "reason_code": reason},
	}}, nil
}

func (r pgColumnTypeSourceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGColumnTypeSource(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_column_type_source", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
