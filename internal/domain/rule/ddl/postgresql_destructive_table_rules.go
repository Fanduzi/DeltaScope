package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgDestructiveTableTargetResult(s spec.Statement, i int) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	declaration := s.DDL.PGDestructiveTable
	if declaration == nil || i >= len(declaration.Targets) {
		return "", "target_table.pg_identity"
	}
	target := declaration.Targets[i]
	if !target.Qualified() {
		return "", "target_table.pg_identity"
	}
	if i >= len(s.Metadata.PGDestructiveTableRelations) || s.Metadata.PGDestructiveTableRelations[i].Schema != target.Table.Schema || s.Metadata.PGDestructiveTableRelations[i].Relation.Name != target.Table.Name {
		return "", "target_table.pg_relation"
	}
	kind := s.Metadata.PGDestructiveTableRelations[i].Relation.Kind
	if kind == "" {
		if s.DDL.Operation == spec.DDLOperationDropTable && declaration.IfExists {
			return "", ""
		}
		return "relation_absent", ""
	}
	if s.DDL.Operation == spec.DDLOperationTruncateTable {
		if kind == "p" && target.Only {
			for _, prior := range declaration.Targets[:i] {
				if prior.Only {
					continue
				}
				facts := s.Metadata.PGDestructiveReferences
				if facts == nil || !facts.Matches(declaration.Targets) {
					return "", "target_table.pg_truncate_membership"
				}
				if facts.IneligibleOnlyPosition != i+1 {
					return "", ""
				}
				break
			}
			return "partition_root_only_inapplicable", ""
		}
		if kind == "f" {
			return "", "target_table.pg_fdw_truncate"
		}
	}
	if kind != "r" && kind != "p" {
		return "relation_wrong_kind", ""
	}
	return "", ""
}

func (r tableOperationExistenceRule) evaluatePGDestructiveTable(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if s.DDL.PGDestructiveTable == nil {
		return nil, nil
	}
	var findings []rule.Finding
	seen := make(map[[3]string]bool)
	for i, target := range s.DDL.PGDestructiveTable.Targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reason, _ := pgDestructiveTableTargetResult(s, i)
		if reason == "" {
			continue
		}
		key := [3]string{target.Table.Schema, target.Table.Name, reason}
		if seen[key] {
			continue
		}
		seen[key] = true
		message := fmt.Sprintf("%s target %q does not exist in the target schema", r.label, target.Table.Name)
		switch reason {
		case "relation_wrong_kind":
			message = fmt.Sprintf("%s cannot operate on the observed relation named %q", r.label, target.Table.Name)
		case "partition_root_only_inapplicable":
			message = fmt.Sprintf("PostgreSQL cannot truncate only the partitioned table %q", target.Table.Name)
		}
		findings = append(findings, rule.Finding{RuleID: r.ruleID, Level: r.level, Message: message,
			Metadata: map[string]any{"table": target.Table.Name, "schema": target.Table.Schema,
				"operation": string(s.DDL.Operation), "reason_code": reason, "exists": reason != "relation_absent"},
		})
	}
	if len(findings) == 0 {
		if position, _ := pgDropReferenceResult(s); position > 0 {
			target := s.DDL.PGDestructiveTable.Targets[position-1].Table
			findings = append(findings, rule.Finding{RuleID: r.ruleID, Level: r.level,
				Message:    "A PostgreSQL view, materialized view, foreign key or column type depends on the selected DROP TABLE target",
				Suggestion: "Review the dependent objects before dropping the table with RESTRICT",
				Metadata: map[string]any{"table": target.Name, "schema": target.Schema, "operation": string(s.DDL.Operation),
					"reason_code": "drop_table_has_direct_dependency", "exists": true},
			})
		}
	}
	return findings, nil
}

func pgDropReferenceResult(s spec.Statement) (position int, missing string) {
	declaration := s.DDL.PGDestructiveTable
	if s.DDL.Operation != spec.DDLOperationDropTable || declaration == nil || declaration.Cascade || s.Unsupported != nil {
		return 0, ""
	}
	anyPresent := false
	for i := range declaration.Targets {
		reason, fact := pgDestructiveTableTargetResult(s, i)
		if reason != "" || fact != "" {
			return 0, ""
		}
		anyPresent = anyPresent || s.Metadata.PGDestructiveTableRelations[i].Relation.Kind != ""
	}
	if !anyPresent {
		return 0, ""
	}
	targets := spec.PGDropReferenceTargets(s)
	if len(targets) == 0 || s.Metadata.PGDestructiveReferences == nil || !s.Metadata.PGDestructiveReferences.Matches(targets) {
		return 0, "target_table.pg_drop_references"
	}
	return s.Metadata.PGDestructiveReferences.BlockedPosition, ""
}

func pgDestructiveTableEvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	declaration := s.DDL.PGDestructiveTable
	n := 1
	if declaration != nil && len(declaration.Targets) > 0 {
		n = len(declaration.Targets)
	}
	var gaps []rule.EvidenceGap
	seen := make(map[string]bool)
	for i := 0; i < n; i++ {
		_, fact := pgDestructiveTableTargetResult(s, i)
		if fact != "" && !seen[fact] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_pg_destructive_table_target", RequiredFacts: []string{fact}})
			seen[fact] = true
		}
	}
	if _, missing := pgDropReferenceResult(s); missing != "" {
		gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_pg_destructive_table_target", RequiredFacts: []string{missing}})
	}
	return gaps
}
