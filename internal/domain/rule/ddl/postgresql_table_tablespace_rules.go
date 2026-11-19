package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTableTablespaceRule struct{ level rule.Level }

func newPGTableTablespaceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableTablespaceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTableTablespaceRule) ID() string { return ruleIDPGAlterTableTablespaceTargetValid }

func (r pgTableTablespaceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterTable && appliesToAlterActions(s, "set_tablespace")
}

func boundPGTableTablespace(s spec.Statement) *spec.PGTableTablespace {
	name := spec.PGTableTablespaceTarget(s)
	if name == "" || !spec.PGOrdinaryTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
		return nil
	}
	facts, table := s.Metadata.PGTableTablespace, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Name != name {
		return nil
	}
	return facts
}

func assessPGTableTablespace(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	facts := boundPGTableTablespace(s)
	if facts == nil {
		return "", "target_table.pg_tablespace"
	}
	switch {
	case !facts.Exists:
		return "tablespace_absent", ""
	case facts.Global:
		return "tablespace_global_inapplicable", ""
	case !facts.SameKnown:
		return "", "target_table.pg_tablespace"
	}
	return "", ""
}

func (r pgTableTablespaceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTableTablespace(s)
	if reason == "" {
		return nil, nil
	}
	message := "The requested PostgreSQL tablespace does not exist"
	if reason == "tablespace_global_inapplicable" {
		message = "PostgreSQL reserves pg_global for shared relations"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: "Choose an existing tablespace that accepts ordinary tables",
		Metadata:   map[string]any{"action": "set_tablespace", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason},
	}}, nil
}

func (r pgTableTablespaceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTableTablespace(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_tablespace", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
