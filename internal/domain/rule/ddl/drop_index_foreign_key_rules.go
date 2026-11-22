package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type dropIndexForeignKeyWarnRule struct{ level rule.Level }

func newDropIndexForeignKeyWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return dropIndexForeignKeyWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r dropIndexForeignKeyWarnRule) ID() string { return ruleIDDropIndexForeignKeyWarn }

func (r dropIndexForeignKeyWarnRule) AppliesTo(s spec.Statement) bool {
	return (s.Dialect == spec.DialectMySQL || s.Dialect == spec.DialectTiDB) && s.Kind == spec.KindDDL &&
		s.Unsupported == nil && s.DDL != nil && s.DDL.Operation == spec.DDLOperationDropIndex
}

func (r dropIndexForeignKeyWarnRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) || s.Metadata == nil || !s.Metadata.DropIndexForeignKeyDependency {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    "A verified foreign key depends on this index and has no eligible replacement",
		Suggestion: "Keep the index or provide an eligible replacement before dropping it",
	}}, nil
}
