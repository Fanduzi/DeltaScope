package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type ddlParserErrorFeasibility string

const (
	parserUpgradeCandidate          ddlParserErrorFeasibility = "parser_upgrade_candidate"
	boundedFallbackCandidate        ddlParserErrorFeasibility = "bounded_fallback_candidate"
	productUnsupportedOrInapplicable ddlParserErrorFeasibility = "product_unsupported_or_inapplicable"
	unsafeFallbackDefer            ddlParserErrorFeasibility = "unsafe_fallback_defer"
	needsResearch                  ddlParserErrorFeasibility = "needs_research"
)

type ddlParserErrorFeasibilityCase struct {
	Dialect  spec.Dialect
	Family   string
	Name     string
	SQL      string
	Expected ddlParserErrorFeasibility
	Reason   string
}

func TestDDLParserErrorFeasibilityCensus(t *testing.T) {
	t.Parallel()

	cases := []ddlParserErrorFeasibilityCase{}
	if len(cases) != 0 {
		t.Fatalf("Task 1 skeleton should not enumerate parser-error cases yet")
	}
}
