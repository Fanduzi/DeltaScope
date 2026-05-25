//go:build postgresql

package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPostgreSQLDDLConsolidatedCoverageCensus(t *testing.T) {
	t.Parallel()

	type sourceSummary struct {
		Source  string
		Summary ddlCensusSummary
	}

	var summaries []sourceSummary

	// 1. Representative DDL census.
	var representativeResults []ddlCensusResult
	for _, tc := range pgDDLCensusCases {
		if tc.DeferReason != "" {
			continue
		}
		res := runCensusCase(t, tc)
		representativeResults = append(representativeResults, adaptCensusResult(res))
	}
	s := summarizeDDLCensusResults("PostgreSQL", representativeResults)
	s.assertArithmetic(t)
	summaries = append(summaries, sourceSummary{Source: "representative", Summary: s})

	// 2. Completion census.
	completionResults := runBatchClassification(t, pgDDLCompletionCensusCases)
	s = summarizeDDLCensusResults("PostgreSQL", completionResults)
	s.assertArithmetic(t)
	summaries = append(summaries, sourceSummary{Source: "completion", Summary: s})

	// 3. Deep coverage census.
	deepResults := runBatchClassification(t, pgDDLDeepCoverageCensusCases)
	s = summarizeDDLCensusResults("PostgreSQL", deepResults)
	s.assertArithmetic(t)
	summaries = append(summaries, sourceSummary{Source: "deep", Summary: s})

	// 4. Long-tail census.
	longTailResults := runBatchClassification(t, pgDDLLongTailCensusCases)
	s = summarizeDDLCensusResults("PostgreSQL", longTailResults)
	s.assertArithmetic(t)
	summaries = append(summaries, sourceSummary{Source: "long_tail", Summary: s})

	// 5. ALTER TABLE residual census.
	var residualResults []ddlCensusResult
	for _, tc := range pgAlterTableResidualCases {
		res := classifyAlterTableResidualCase(t, tc)
		residualResults = append(residualResults, adaptResidualResult(res))
	}
	s = summarizeDDLCensusResults("PostgreSQL", residualResults)
	s.assertArithmetic(t)
	summaries = append(summaries, sourceSummary{Source: "alter_residual", Summary: s})

	// Consolidated summary.
	t.Log("")
	t.Log("=== PostgreSQL DDL Consolidated Census Summary ===")

	var cTotal, cFinding, cSilent, cUnsupported, cParserErr, cUnclassified int
	for _, ss := range summaries {
		t.Logf("  %-20s total=%d finding_covered=%d normalized_silent=%d unsupported_boundary=%d parser_error=%d unclassified=%d",
			ss.Source, ss.Summary.Total, ss.Summary.FindingCovered, ss.Summary.NormalizedSilent,
			ss.Summary.UnsupportedBoundary, ss.Summary.ParserError, ss.Summary.Unclassified)
		cTotal += ss.Summary.Total
		cFinding += ss.Summary.FindingCovered
		cSilent += ss.Summary.NormalizedSilent
		cUnsupported += ss.Summary.UnsupportedBoundary
		cParserErr += ss.Summary.ParserError
		cUnclassified += ss.Summary.Unclassified
	}

	t.Log("")
	t.Logf("  %-20s total=%d (tracked-case total, includes overlap between source lists)", "consolidated", cTotal)
	t.Logf("  %-20s finding_covered=%d normalized_silent=%d unsupported_boundary=%d parser_error=%d unclassified=%d",
		"", cFinding, cSilent, cUnsupported, cParserErr, cUnclassified)

	// Assert expected per-source facts.
	assertExpected := func(source string, got ddlCensusSummary, total, finding, silent, unsupported, parserErr int) {
		t.Helper()
		if got.Total != total {
			t.Errorf("%s: expected total=%d, got %d", source, total, got.Total)
		}
		if got.FindingCovered != finding {
			t.Errorf("%s: expected finding_covered=%d, got %d", source, finding, got.FindingCovered)
		}
		if got.NormalizedSilent != silent {
			t.Errorf("%s: expected normalized_silent=%d, got %d", source, silent, got.NormalizedSilent)
		}
		if got.UnsupportedBoundary != unsupported {
			t.Errorf("%s: expected unsupported_boundary=%d, got %d", source, unsupported, got.UnsupportedBoundary)
		}
		if got.ParserError != parserErr {
			t.Errorf("%s: expected parser_error=%d, got %d", source, parserErr, got.ParserError)
		}
	}

	assertExpected("representative", summaries[0].Summary, 92, 88, 4, 0, 0)
	assertExpected("completion", summaries[1].Summary, 31, 31, 0, 0, 0)
	assertExpected("deep", summaries[2].Summary, 39, 38, 0, 0, 1)
	assertExpected("long_tail", summaries[3].Summary, 57, 57, 0, 0, 0)
	assertExpected("alter_residual", summaries[4].Summary, 66, 60, 2, 0, 4)
}

func adaptCensusResult(r censusResult) ddlCensusResult {
	var class ddlCoverageClass
	switch r.Status {
	case statusFindingCovered:
		class = ddlCoverageFindingCovered
	case statusNormalizedSilent:
		class = ddlCoverageNormalizedSilent
	case statusUnsupportedExp:
		class = ddlCoverageUnsupportedBoundary
	case statusParserError:
		class = ddlCoverageParserError
	default:
		class = ddlCoverageClass("unclassified")
	}
	ids := make([]string, 0, len(r.StmtFindings)+len(r.GlobalFindings))
	ids = append(ids, r.StmtFindings...)
	ids = append(ids, r.GlobalFindings...)
	return ddlCensusResult{
		Name:           r.Name,
		Dialect:        "PostgreSQL",
		ParseOK:        r.ParseOK,
		Kind:           r.Kind,
		Unsupported:    r.Unsupported,
		UnsupportedFea: r.UnsupportedFeat,
		DDLOperation:   r.DDLOperation,
		FindingRuleIDs: ids,
		Classification: class,
	}
}

func adaptResidualResult(r alterTableResidualResult) ddlCensusResult {
	var class ddlCoverageClass
	switch r.Classification {
	case residualFindingCovered:
		class = ddlCoverageFindingCovered
	case residualNormalizedSilent:
		class = ddlCoverageNormalizedSilent
	case residualUnsupportedBound:
		class = ddlCoverageUnsupportedBoundary
	case residualParserError:
		class = ddlCoverageParserError
	default:
		class = ddlCoverageClass("unclassified")
	}
	return ddlCensusResult{
		Name:           r.Name,
		Dialect:        "PostgreSQL",
		Kind:           "ddl",
		Unsupported:    r.Classification == residualUnsupportedBound,
		UnsupportedFea: r.UnsupportedFeature,
		DDLOperation:   r.DDLOperation,
		FindingRuleIDs: r.FindingRuleIDs,
		Classification: class,
	}
}

func runBatchClassification(t *testing.T, cases []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}) []ddlCensusResult {
	t.Helper()
	results := make([]ddlCensusResult, 0, len(cases))
	for _, tc := range cases {
		res := classifyDDLCensusResult(t, tc.Name, tc.SQL, "PostgreSQL", spec.DialectPostgreSQL)
		results = append(results, res)
	}
	return results
}
