package ddl

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnFormatRule struct {
	id    string
	level rule.Level
}

func newPGColumnStorageRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnFormatRule{ruleIDPGColumnStorageValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func newPGColumnCompressionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnFormatRule{ruleIDPGColumnCompressionValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func (r pgColumnFormatRule) ID() string { return r.id }
func (r pgColumnFormatRule) matches(a spec.Alter) bool {
	return r.id == ruleIDPGColumnStorageValid && a.Action == "set_storage" || r.id == ruleIDPGColumnCompressionValid && a.Action == "set_compression"
}
func (r pgColumnFormatRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	for _, a := range s.DDL.Alter {
		if r.matches(a) {
			return true
		}
	}
	return false
}

func (r pgColumnFormatRule) assess(s spec.Statement, index int) (reason, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	a := s.DDL.Alter[index]
	method, declarationMissing := "", ""
	if r.id == ruleIDPGColumnStorageValid {
		if a.Options["has_storage_setting"] != "true" || a.Options["storage_kind"] == "" {
			declarationMissing = "column.storage_strategy"
		} else {
			for _, candidate := range []string{"plain", "external", "extended", "main", "default"} {
				if strings.EqualFold(a.Options["storage_kind"], candidate) {
					method = candidate
					break
				}
			}
			if method == "" {
				return "storage_mode_invalid", ""
			}
		}
	} else {
		if a.PGCompressionMethod == nil || *a.PGCompressionMethod == "" {
			declarationMissing = "column.compression_method"
		} else {
			method = *a.PGCompressionMethod
			switch method {
			case "pglz", "lz4", "default":
			default:
				return "compression_method_invalid", ""
			}
		}
	}
	member, known := pgColumnTargetState(s, index)
	if !known {
		return "", "target_table.pg_columns"
	}
	if member == nil {
		return "column_absent", ""
	}
	if member.System {
		return "system_column_inapplicable", ""
	}
	if declarationMissing != "" {
		return "", declarationMissing
	}
	if r.id == ruleIDPGColumnCompressionValid && method == "default" || r.id == ruleIDPGColumnStorageValid && method == "plain" {
		return "", ""
	}
	switch member.TypeStorage {
	case "p":
		if r.id != ruleIDPGColumnStorageValid || method != "default" {
			return "type_not_toastable", ""
		}
	case "e", "m", "x":
	default:
		return "", "column.type_storage"
	}
	if r.id == ruleIDPGColumnCompressionValid && method == "lz4" {
		if !s.Metadata.Instance.PGCompressionLZ4Known {
			return "", "instance.pg_compression_lz4"
		}
		if !s.Metadata.Instance.PGCompressionLZ4Supported {
			return "compression_method_unavailable", ""
		}
	}
	return "", ""
}

func (r pgColumnFormatRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	code := "unproven_column_storage"
	if r.id == ruleIDPGColumnCompressionValid {
		code = "unproven_column_compression"
	}
	var gaps []rule.EvidenceGap
	seen := map[string]bool{}
	for i, a := range s.DDL.Alter {
		if !r.matches(a) {
			continue
		}
		_, missing := r.assess(s, i)
		if missing != "" && !seen[missing] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: code, RequiredFacts: []string{missing}})
			seen[missing] = true
		}
	}
	return gaps
}
func (r pgColumnFormatRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	var findings []rule.Finding
	for i, a := range s.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !r.matches(a) {
			continue
		}
		reason, _ := r.assess(s, i)
		if reason == "" {
			continue
		}
		message := "PostgreSQL column storage has a proven invalid declaration or target/type precondition"
		suggestion := "verify the exact user column, storage mode and actual type storage strategy; this declaration does not immediately rewrite existing rows"
		if r.id == ruleIDPGColumnCompressionValid {
			message = "PostgreSQL column compression has a proven invalid declaration or target/type/build precondition"
			suggestion = "verify the exact user column, case-sensitive method, actual type and observed build capability; DEFAULT needs no compression-capable type, and this declaration does not recompress existing rows"
		}
		metadata := map[string]any{"action": a.Action, "table": s.DDL.Table.Name, "column": a.Name, "reason_code": reason}
		if s.DDL.Table.Schema != "" {
			metadata["schema"] = s.DDL.Table.Schema
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return findings, nil
}
