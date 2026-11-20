package ddl

import (
	"context"
	"fmt"
	"math"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnTuningRule struct {
	id    string
	level rule.Level
}

func newPGColumnStatisticsTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTuningRule{ruleIDPGColumnStatisticsTargetValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func newPGColumnStatisticsClampRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTuningRule{ruleIDPGColumnStatisticsClampWarn, configuredLevel(cfg, rule.LevelWarning)}, nil
}
func newPGColumnOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTuningRule{ruleIDPGColumnOptionsValid, configuredLevel(cfg, rule.LevelBlocker)}, nil
}
func (r pgColumnTuningRule) ID() string { return r.id }
func (r pgColumnTuningRule) matches(a spec.Alter) bool {
	if r.id == ruleIDPGColumnOptionsValid {
		return a.Action == "set_column_options" || a.Action == "reset_column_options"
	}
	if a.Action != "set_statistics" {
		return false
	}
	if r.id != ruleIDPGColumnStatisticsClampWarn {
		return true
	}
	f := a.PGColumnTuning
	return f == nil || f.Unmodeled || f.StatisticsDefault == (f.StatisticsValue != nil) || f.StatisticsValue != nil && *f.StatisticsValue > 10000
}
func (r pgColumnTuningRule) AppliesTo(s spec.Statement) bool {
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

func pgStatisticsDeclaration(a spec.Alter) (string, string) {
	f := a.PGColumnTuning
	if f == nil || f.Unmodeled || len(f.Options) != 0 || f.StatisticsDefault == (f.StatisticsValue != nil) {
		return "", "column.statistics_target"
	}
	if f.StatisticsValue != nil {
		v := *f.StatisticsValue
		if v < math.MinInt32 || v > math.MaxInt32 {
			return "", "column.statistics_target"
		}
		if v < -1 {
			return "statistics_target_below_default", ""
		}
	}
	return "", ""
}

func pgColumnOptionsDeclaration(a spec.Alter) (string, string, string) {
	f := a.PGColumnTuning
	if f == nil {
		return "", "column.options", ""
	}
	unknown := f.Unmodeled || len(f.Options) == 0 || f.StatisticsDefault || f.StatisticsValue != nil
	seen := map[string]bool{}
	for _, o := range f.Options {
		if a.Action == "reset_column_options" {
			if o.Name == "" || o.ValueKind != spec.PGColumnOptionAbsent || o.Numeric != nil {
				unknown = true
			}
			continue
		}
		if o.Namespace != "" || o.Name != "n_distinct" && o.Name != "n_distinct_inherited" {
			unknown = true
			continue
		}
		if seen[o.Name] {
			return "column_option_duplicate", "", o.Name
		}
		seen[o.Name] = true
		if o.ValueKind != spec.PGColumnOptionNumeric || o.Numeric == nil || math.IsNaN(*o.Numeric) || math.IsInf(*o.Numeric, 0) {
			unknown = true
			continue
		}
		if *o.Numeric < -1 {
			return "column_option_out_of_range", "", o.Name
		}
	}
	if unknown {
		return "", "column.options", ""
	}
	return "", "", ""
}

func (r pgColumnTuningRule) assess(s spec.Statement, index int) (string, string, string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version", ""
	}
	a := s.DDL.Alter[index]
	reason, missing, parameter := "", "", ""
	if r.id == ruleIDPGColumnOptionsValid {
		reason, missing, parameter = pgColumnOptionsDeclaration(a)
	} else {
		reason, missing = pgStatisticsDeclaration(a)
	}
	if reason != "" {
		if r.id == ruleIDPGColumnStatisticsClampWarn {
			return "", "", ""
		}
		return reason, "", parameter
	}
	member, known := pgColumnTargetState(s, index)
	if !known {
		return "", "target_table.pg_columns", ""
	}
	if member == nil || member.System {
		if r.id == ruleIDPGColumnStatisticsClampWarn {
			return "", "", ""
		}
		if member != nil {
			return "system_column_inapplicable", "", ""
		}
		return "column_absent", "", ""
	}
	if missing != "" {
		return "", missing, ""
	}
	if r.id == ruleIDPGColumnStatisticsClampWarn && a.PGColumnTuning.StatisticsValue != nil && *a.PGColumnTuning.StatisticsValue > 10000 {
		return "statistics_target_clamped", "", ""
	}
	return "", "", ""
}

func (r pgColumnTuningRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	var gaps []rule.EvidenceGap
	seen := map[string]bool{}
	for i, a := range s.DDL.Alter {
		if !r.matches(a) {
			continue
		}
		_, missing, _ := r.assess(s, i)
		if missing != "" && !seen[missing] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_column_tuning", RequiredFacts: []string{missing}})
			seen[missing] = true
		}
	}
	return gaps
}
func (r pgColumnTuningRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
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
		reason, _, parameter := r.assess(s, i)
		if reason == "" {
			continue
		}
		message := "PostgreSQL column statistics target has a proven invalid value, missing column or inapplicable system target"
		suggestion := "use DEFAULT or an integer target of at least -1 and verify the exact target column; values above the native maximum are clamped, not rejected"
		switch r.id {
		case ruleIDPGColumnOptionsValid:
			message = "PostgreSQL column options have a proven invalid value, duplicate key, missing column or inapplicable system target"
			suggestion = "verify the exact column, remove duplicate builtin keys and correct the n_distinct/n_distinct_inherited numeric range; unknown extension keys and opaque values are not guessed"
		case ruleIDPGColumnStatisticsClampWarn:
			message = "PostgreSQL lowers this statistics target to its native maximum"
			suggestion = "review whether the native statistics maximum meets the intended sampling target; the statement is not refused merely for exceeding it"
		}
		metadata := map[string]any{"action": a.Action, "table": s.DDL.Table.Name, "column": a.Name, "reason_code": reason}
		if s.DDL.Table.Schema != "" {
			metadata["schema"] = s.DDL.Table.Schema
		}
		if parameter != "" {
			metadata["parameter"] = parameter
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return findings, nil
}

// pgDropColumnTarget shares guard/target proof between the existing-ID blocker
// and advisory. Legacy case-folded columns cannot prove a PostgreSQL no-op.
func pgDropColumnTarget(s spec.Statement, index int) (bool, string, string) {
	if !pgConstraintTargetVersionKnown(s) {
		return false, "", "instance.version"
	}
	member, known := pgColumnTargetState(s, index)
	if !known {
		return false, "", "target_table.pg_columns"
	}
	if member != nil && member.System {
		return false, "system_column_inapplicable", ""
	}
	guard := s.DDL.Alter[index].Options["if_exists"]
	if guard != "true" && guard != "false" {
		return false, "", "column.if_exists"
	}
	if member != nil {
		reason, missing := pgDropColumnDependencyResult(s, index)
		return false, reason, missing
	}
	if guard == "true" {
		return true, "", ""
	}
	return false, "column_absent", ""
}

func pgDropColumnDependencyResult(s spec.Statement, index int) (reason, missing string) {
	table, catalog := s.DDL.Table, s.Metadata.TargetTable.PGColumns
	action := s.DDL.Alter[index]
	if len(s.DDL.Alter) != 1 || action.Options["cascade"] != "false" || s.DDL.PGTableCatalogQualified ||
		table.Schema == "" || table.Schema == "pg_temp" || catalog.Typed {
		return "", ""
	}
	fact := s.Metadata.PGDropColumnDependency
	if !spec.PGOrdinaryTableBound(s) || fact == nil || fact.Schema != table.Schema || fact.Table != table.Name || fact.Column != action.Name {
		return "", "column.pg_dependencies"
	}
	switch fact.Kind {
	case "view", "generated_column":
		return "column_has_direct_dependency", ""
	case "foreign_key":
		return "column_referenced_by_foreign_key", ""
	case "":
		return "", ""
	default:
		return "", "column.pg_dependencies"
	}
}

func (r alterObjectExistenceRule) evaluatePGDropColumn(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.Unsupported != nil || s.Kind != spec.KindDDL || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return nil, nil
	}
	var findings []rule.Finding
	for i, a := range s.DDL.Alter {
		if a.Action != "drop_column" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, reason, _ := pgDropColumnTarget(s, i)
		if reason == "" {
			continue
		}
		message := fmt.Sprintf("column %q does not exist on table %q", a.Name, s.DDL.Table.Name)
		suggestion := "Verify the exact column identity. IF EXISTS protects an absent column."
		metadata := map[string]any{"table": s.DDL.Table.Name, "action": a.Action, "name": a.Name, "exists": reason != "column_absent"}
		switch reason {
		case "column_has_direct_dependency":
			message = "A PostgreSQL view, materialized view or generated column depends on the selected column"
			suggestion = "Review the direct dependency before dropping the column with RESTRICT"
		case "column_referenced_by_foreign_key":
			message = "A PostgreSQL foreign key references the selected column"
			suggestion = "Review the foreign key before dropping the column with RESTRICT"
		case "system_column_inapplicable":
			message = "PostgreSQL does not allow dropping this observed system column"
			suggestion = "Select a user column for DROP COLUMN"
		}
		if reason != "column_absent" {
			metadata["reason_code"] = reason
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return findings, nil
}
func pgDropColumnEvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if s.Unsupported != nil || s.Kind != spec.KindDDL || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return nil
	}
	var gaps []rule.EvidenceGap
	seen := map[string]bool{}
	for i, a := range s.DDL.Alter {
		if a.Action != "drop_column" {
			continue
		}
		_, _, missing := pgDropColumnTarget(s, i)
		if missing != "" && !seen[missing] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_column_target", RequiredFacts: []string{missing}})
			seen[missing] = true
		}
	}
	return gaps
}
