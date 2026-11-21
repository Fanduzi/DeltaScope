package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTableReloptionsRule struct{ level rule.Level }

func newPGTableReloptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTableReloptionsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTableReloptionsRule) ID() string { return ruleIDPGAlterTableReloptionsValid }

func (r pgTableReloptionsRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterTable && appliesToAlterActions(s, "set_reloptions", "reset_reloptions")
}

func assessPGTableReloptions(s spec.Statement) (reason, parameter, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", "", ""
	}
	if len(s.DDL.Alter) != 1 || !spec.PGOrdinaryTableBound(s) || !s.Metadata.TargetTable.PGRelation.Heap {
		return "", "", "target_table.pg_heap"
	}
	a := s.DDL.Alter[0]
	return assessPGTableReloptionDeclarations(a.PGTableReloptions, a.Action == "reset_reloptions")
}

func assessPGTableReloptionDeclarations(options []spec.PGTableReloption, reset bool) (reason, parameter, fact string) {
	unknown := len(options) == 0
	seen := make(map[string]bool, 2)
	for _, o := range options {
		known := o.Namespace == "" && (o.Name == "fillfactor" || o.Name == "autovacuum_enabled")
		if reset {
			if o.HasValue {
				if known {
					parameter = o.Name
				}
				return "table_reloption_reset_has_value", parameter, ""
			}
			unknown = unknown || o.Name == ""
			continue
		}
		if !known {
			unknown = true
			continue
		}
		if seen[o.Name] {
			return "table_reloption_duplicate", o.Name, ""
		}
		seen[o.Name] = true
		switch o.Name {
		case "fillfactor":
			if !o.HasValue {
				return "table_reloption_value_required", o.Name, ""
			}
			if o.Value != spec.PGTableReloptionInteger {
				unknown = true
				continue
			}
			if o.Integer < 10 || o.Integer > 100 {
				return "table_reloption_out_of_range", o.Name, ""
			}
		case "autovacuum_enabled":
			if o.Value == spec.PGTableReloptionInvalidBoolean {
				return "table_reloption_invalid_boolean", o.Name, ""
			}
			unknown = unknown || o.Value != spec.PGTableReloptionBoolean
		}
	}
	if unknown {
		return "", "", "table.reloptions"
	}
	return "", "", ""
}

func (r pgTableReloptionsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, parameter, _ := assessPGTableReloptions(s)
	if reason == "" {
		return nil, nil
	}
	metadata := map[string]any{"action": s.DDL.Alter[0].Action, "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name, "reason_code": reason}
	if parameter != "" {
		metadata["parameter"] = parameter
	}
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects this table storage option declaration",
		Suggestion: "Use one valid value for each SET parameter and omit values from RESET", Metadata: metadata}}, nil
}

func (r pgTableReloptionsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, _, fact := assessPGTableReloptions(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_table_reloptions", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
