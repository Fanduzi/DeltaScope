// Package audit orchestrates audit use cases at the application layer.
// input: extracted domain statements and the registered rule engine
// output: aggregated report results with statement and global findings
// pos: application evaluation step between extraction and reporting
// note: if this file changes, update this header and module README.md.
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// EvaluateStatements applies registered rules and aggregates their findings into a report result.
func EvaluateStatements(registry *rule.Registry, statements []spec.Statement) (report.Result, error) {
	statementResults := make([]report.StatementResult, 0, len(statements))

	for idx, statement := range statements {
		findings, err := registry.EvaluateStatement(statement)
		if err != nil {
			return report.Result{}, err
		}

		for i := range findings {
			findings[i].StatementIndex = idx
			findings[i].StatementKind = statement.Kind.String()
		}
		findings = enrichFindings(findings, &statement)

		statementResults = append(statementResults, report.StatementResult{
			Index:         idx,
			Kind:          statement.Kind.String(),
			RawSQL:        statement.RawSQL,
			NormalizedSQL: statement.NormalizedSQL,
			Findings:      findings,
		})
	}

	globalFindings, err := registry.EvaluateGlobal(statements)
	if err != nil {
		return report.Result{}, err
	}
	globalFindings = enrichFindings(globalFindings, nil)

	return report.Aggregate(statementResults, globalFindings), nil
}
