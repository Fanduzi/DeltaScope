// Package audit orchestrates audit use cases at the application layer.
// input: extracted domain statements and the registered rule engine
// output: aggregated report results with statement/global findings and preserved statement-level impact estimates
// pos: application evaluation step between extraction/metadata refinement and reporting
// note: if this file changes, update this header and module README.md.
package audit

import (
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// EvaluateStatements applies registered rules and aggregates their findings into a report result.
func EvaluateStatements(registry *rule.Registry, statements []spec.Statement) (report.Result, error) {
	statementResults := make([]report.StatementResult, 0, len(statements))
	unsupported := make([]spec.UnsupportedDetail, 0)
	supportedStatements := make([]spec.Statement, 0, len(statements))

	appliedIDs := make(map[string]struct{})
	skippedDedup := make(map[string]rule.SkippedRule)

	for idx, statement := range statements {
		if statement.Unsupported != nil {
			item := *statement.Unsupported
			item.Index = idx
			if item.SQL == "" {
				item.SQL = statement.RawSQL
			}
			unsupported = append(unsupported, item)
			continue
		}

		eval, err := registry.EvaluateStatementDetailed(statement)
		if err != nil {
			return report.Result{}, err
		}

		findings := eval.Findings
		resultIndex := len(statementResults)
		for i := range findings {
			findings[i].StatementIndex = resultIndex
			findings[i].StatementKind = statement.Kind.String()
		}
		findings = enrichFindings(findings, &statement)

		for _, id := range eval.AppliedRuleIDs {
			appliedIDs[id] = struct{}{}
		}

		for _, s := range eval.Skipped {
			if _, seen := skippedDedup[s.RuleID]; !seen {
				skippedDedup[s.RuleID] = s
			}
		}

		statementResults = append(statementResults, report.StatementResult{
			Index:         resultIndex,
			Kind:          statement.Kind.String(),
			RawSQL:        statement.RawSQL,
			NormalizedSQL: statement.NormalizedSQL,
			Findings:      findings,
			Impact:        reportImpact(statement),
		})
		supportedStatements = append(supportedStatements, statement)
	}

	globalFindings, err := registry.EvaluateGlobal(supportedStatements)
	if err != nil {
		return report.Result{}, err
	}
	globalFindings = enrichFindings(globalFindings, nil)

	result := report.Aggregate(statementResults, globalFindings)
	result.Unsupported = unsupported

	loaded := registry.LoadedStatementRuleCount()
	if loaded > 0 {
		skipped := make([]rule.SkippedRule, 0, len(skippedDedup))
		for _, s := range skippedDedup {
			skipped = append(skipped, s)
		}
		sort.Slice(skipped, func(i, j int) bool {
			return skipped[i].RuleID < skipped[j].RuleID
		})
		result.RuleSummary = &report.RuleSummary{
			Loaded:     loaded,
			Applicable: len(appliedIDs),
			Skipped:    skipped,
		}
	}

	return result, nil
}

func reportImpact(statement spec.Statement) *report.Impact {
	if statement.DML == nil || statement.DML.Impact == nil {
		return nil
	}

	impact := statement.DML.Impact
	result := &report.Impact{
		EstimatedRows:  impact.EstimatedRows,
		EstimatedRatio: impact.EstimatedRatio,
		RiskLevel:      report.ImpactRisk(impact.RiskLevel),
		Confidence:     report.ImpactConfidence(impact.Confidence),
		Source:         report.ImpactSource(impact.Source),
	}
	if impact.ReasonCodes != nil {
		result.ReasonCodes = append([]string(nil), impact.ReasonCodes...)
	}
	if impact.Notes != nil {
		result.Notes = append([]string(nil), impact.Notes...)
	}
	return result
}
