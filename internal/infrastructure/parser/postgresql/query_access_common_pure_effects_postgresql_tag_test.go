//go:build postgresql

// Package postgresql characterizes PostgreSQL extraction of common pure-effect forms.
// input: schema-qualified aggregate and window SELECT forms
// output: unproven function reason and current partition/order usages
// pos: parser-level characterization; no admission promotion
package postgresql

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestCommonPureEffects_PostgreSQLParser_FunctionsUnproven(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	for _, sql := range []string{
		"SELECT COUNT(*) FROM public.users",
		"SELECT SUM(amount) FROM public.orders",
		"SELECT AVG(amount) FROM public.orders",
		"SELECT MIN(amount) FROM public.orders",
		"SELECT MAX(amount) FROM public.orders",
		"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM public.employees",
		"SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM public.employees",
		"SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM public.employees",
	} {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts, err := extractor.ExtractQueryAccess(context.Background(), sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want indeterminate", facts.ReadClassification)
			}
			if !hasParserReason(facts.ReasonCodes, string(domain.ReasonUnprovenFunctionEffect)) {
				t.Errorf("reason codes: got %v, want unproven function effect", facts.ReasonCodes)
			}
		})
	}
}

func TestCommonPureEffects_PostgreSQLParser_WindowUsages(t *testing.T) {
	t.Parallel()
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM public.employees", "postgresql", "public")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, want := range []struct {
		column string
		usage  string
	}{{"dept", string(domain.UsageWindow)}, {"id", string(domain.UsageOrdering)}} {
		found := false
		for _, reference := range facts.ColumnReferences {
			if reference.Column == want.column {
				found = true
				assertUsageContains(t, reference.Usages, want.usage)
			}
		}
		if !found {
			t.Errorf("column %q not found in %+v", want.column, facts.ColumnReferences)
		}
	}
}

func hasParserReason(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}
