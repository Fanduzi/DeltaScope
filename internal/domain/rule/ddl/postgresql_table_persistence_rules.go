package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTablePersistenceRule struct{ level rule.Level }

func newPGTablePersistenceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTablePersistenceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTablePersistenceRule) ID() string { return ruleIDPGAlterTablePersistenceValid }

func (r pgTablePersistenceRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterTable && appliesToAlterActions(s, "set_logged", "set_unlogged")
}

func boundPGTablePersistence(s spec.Statement) *spec.PGTablePersistence {
	action := spec.PGTablePersistenceTarget(s)
	partitionRoot := action == "set_unlogged" && spec.PGPartitionedTableBound(s)
	if action == "" || !spec.PGExactTableBound(s) && !partitionRoot || !spec.PG17Observed(s.Metadata.Instance) {
		return nil
	}
	facts, table := s.Metadata.PGTablePersistence, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Logged != (action == "set_logged") ||
		(facts.Current != "p" && facts.Current != "u") {
		return nil
	}
	if facts.PartitionRoot != partitionRoot || partitionRoot && (facts.Current != "p" || !facts.DirectPublication && !facts.ForeignKeyConflict) {
		return nil
	}
	return facts
}

func assessPGTablePersistence(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	facts := boundPGTablePersistence(s)
	if facts == nil {
		return "", "target_table.pg_persistence"
	}
	switch {
	case facts.Same():
		return "", ""
	case !facts.Logged && facts.DirectPublication:
		return "persistence_direct_publication", ""
	case facts.ForeignKeyConflict && facts.Logged:
		return "persistence_outbound_foreign_key", ""
	case facts.ForeignKeyConflict:
		return "persistence_inbound_foreign_key", ""
	case facts.ForeignKeyUnknown:
		return "", "target_table.pg_persistence"
	}
	return "", ""
}

func (r pgTablePersistenceRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTablePersistence(s)
	if reason == "" {
		return nil, nil
	}
	var message string
	switch reason {
	case "persistence_direct_publication":
		message = "PostgreSQL cannot make a directly published table unlogged"
	case "persistence_outbound_foreign_key":
		message = "PostgreSQL cannot make this table logged while it references a nonpermanent table"
	case "persistence_inbound_foreign_key":
		message = "PostgreSQL cannot make this table unlogged while another permanent table references it"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Suggestion: "Review the observed dependency before changing table persistence",
		Metadata:   map[string]any{"action": s.DDL.Alter[0].Action, "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason},
	}}, nil
}

func (r pgTablePersistenceRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTablePersistence(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_persistence", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
