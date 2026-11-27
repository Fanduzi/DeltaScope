package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTableAccessMethodRule struct{ level rule.Level }

func newPGTableAccessMethodRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableAccessMethodRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTableAccessMethodRule) ID() string { return ruleIDPGAlterTableAccessMethodTargetValid }

func (r pgTableAccessMethodRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterTable && appliesToAlterActions(s, "set_access_method")
}

func boundPGTableAccessMethod(s spec.Statement) *spec.PGTableAccessMethod {
	name, useDefault := spec.PGTableAccessMethodTarget(s)
	if name == "" || !(spec.PGOrdinaryTableBound(s) || !useDefault && spec.PGPartitionedTableBound(s)) || !spec.PG17Observed(s.Metadata.Instance) {
		return nil
	}
	facts, table := s.Metadata.PGTableAccessMethod, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Name != name || facts.Default != useDefault {
		return nil
	}
	if spec.PGPartitionedTableBound(s) && facts.Kind != "" && facts.Kind != "i" {
		return nil
	}
	return facts
}

func assessPGTableAccessMethod(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	facts := boundPGTableAccessMethod(s)
	if facts == nil {
		return "", "target_table.pg_access_method"
	}
	switch facts.Kind {
	case "":
		return "access_method_absent", ""
	case "i":
		return "access_method_wrong_kind", ""
	case "t":
		if facts.SameKnown {
			return "", ""
		}
	}
	return "", "target_table.pg_access_method"
}

func (r pgTableAccessMethodRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTableAccessMethod(s)
	if reason == "" {
		return nil, nil
	}
	message := "The requested PostgreSQL access method does not exist"
	if reason == "access_method_wrong_kind" {
		message = "PostgreSQL cannot use an index access method for a table"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: "Choose an existing table access method or validate the current default setting",
		Metadata:   map[string]any{"action": "set_access_method", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason},
	}}, nil
}

func (r pgTableAccessMethodRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTableAccessMethod(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_access_method", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
