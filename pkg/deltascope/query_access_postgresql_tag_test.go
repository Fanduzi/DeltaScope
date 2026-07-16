//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzePostgreSQLQueryAccess_PureEffectCandidatesRemainInternal(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT sum(amount) FROM public.orders",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("admission changed: classification=%q admission=%q", result.ReadClassification, result.Admission)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if raw := string(data); strings.Contains(raw, "effect_candidates") || strings.Contains(raw, "OperandColumnRefs") {
		t.Fatalf("candidate facts leaked into SDK JSON: %s", raw)
	}
}
