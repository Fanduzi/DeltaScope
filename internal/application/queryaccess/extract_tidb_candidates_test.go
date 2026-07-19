package queryaccess_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

func TestExtractTiDBQueryAccess_MapsInternalCandidateClosureWithoutPublicLeak(t *testing.T) {
	t.Parallel()

	// Given a nested aggregate and a ranking window.
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT SUM(ABS(amount)), ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM orders",
		Dialect: "mysql",
	})
	// When the application seam maps parser facts.
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Then internal candidates retain closure facts without entering domain JSON.
	if len(result.EffectCandidates) != 3 {
		t.Fatalf("candidates: got %d, want 3", len(result.EffectCandidates))
	}
	if result.EffectCandidates[0].NamePath[0] != "sum" || result.EffectCandidates[1].NamePath[0] != "abs" {
		t.Errorf("nested candidates: %+v", result.EffectCandidates)
	}
	window := result.EffectCandidates[2]
	if !window.HasWindow || !window.HasWindowPartition || !window.HasWindowOrder {
		t.Errorf("window candidate: %+v", window)
	}
	publicJSON, err := json.Marshal(result.DomainResult)
	if err != nil {
		t.Fatalf("marshal domain result: %v", err)
	}
	if strings.Contains(string(publicJSON), "effect") || strings.Contains(string(publicJSON), "sum") {
		t.Errorf("public result leaked candidate data: %s", publicJSON)
	}
}
