// Package ddl verifies the T06-A15-R1 declared-COMPRESSED row-format
// identity fix at the fact-resolution seam.
// input: create-table statements carrying explicit ROW_FORMAT=COMPRESSED
// declarations plus opposite-format controls across nil/empty/defaulted
// instance shapes under MySQL and TiDB
// output: the resolver preserving the declared COMPRESSED identity over any
// instance default, zero fabricated missing facts for known declarations,
// and the unchanged COMPACT/REDUNDANT/DEFAULT control behavior
// pos: DDL row-size fact-resolution regression for the R1 declared-format
// identity fix (issue #85)
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// A declared COMPRESSED keyword is a recognized row format: the resolver
// keeps the declared identity instead of treating it as an unresolved
// declaration that defers to the instance default or reports a gap.
func TestT06A15R1DeclaredCompressedResolvesVerbatim(t *testing.T) {
	t.Parallel()
	options := map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "COMPRESSED"}
	columns := []spec.Column{{Name: "c", Type: "varchar", Length: 65534}}
	shapes := map[string]*spec.InstanceFacts{
		"nil":       nil,
		"empty":     {},
		"dynamic":   {InnoDBDefaultRowFormat: "DYNAMIC"},
		"compact":   {InnoDBDefaultRowFormat: "COMPACT"},
		"redundant": {InnoDBDefaultRowFormat: "REDUNDANT"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for name, facts := range shapes {
			name, facts := name, facts
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				t.Parallel()
				statement := t06a15RowSizeStatement(dialect, options, columns, facts)
				resolution := resolveRowSizeFacts(statement)
				if !resolution.inScope {
					t.Fatal("expected statement in scope")
				}
				if !resolution.rowFormatKnown || resolution.rowFormat != "COMPRESSED" {
					t.Fatalf("declared COMPRESSED lost to %q (known=%v)", resolution.rowFormat, resolution.rowFormatKnown)
				}
				if len(resolution.missingFacts) != 0 {
					t.Fatalf("expected no missing facts, got %v", resolution.missingFacts)
				}
				ruleUnderTest := t06a15RowSizeRule(t, true)
				findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 1 || findings[0].Metadata["row_format"] != "COMPRESSED" {
					t.Fatalf("expected the over-limit finding carrying row_format=COMPRESSED, got %+v", findings)
				}
				t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
			})
		}
	}
}

// A compressed declaration must never enter the COMPACT or REDUNDANT
// payload branches: eleven utf8mb4 VARCHAR(192) columns estimate 8470 total
// bytes, but 8448 under the shared 768 cap — a wrong-format adoption would
// fabricate a finding the declared format never authorized.
func TestT06A15R1CompressedNeverEntersPayloadBranches(t *testing.T) {
	t.Parallel()
	columns := make([]spec.Column, 0, 11)
	for i := 0; i < 11; i++ {
		columns = append(columns, spec.Column{Name: "c", Type: "varchar", Length: 192, Charset: "utf8mb4"})
	}
	for _, format := range []string{"COMPRESSED", "DYNAMIC"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			ruleUnderTest := t06a15RowSizeRule(t, true)
			statement := t06a15RowSizeStatement(spec.DialectMySQL,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": format}, columns,
				&spec.InstanceFacts{InnoDBDefaultRowFormat: "COMPACT"})
			findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("%s must not enter the COMPACT/REDUNDANT branch, got %+v", format, findings)
			}
		})
	}
	// Opposite controls: real COMPACT/REDUNDANT declarations still hit their
	// existing bounds — the fix must not launder every declaration out of
	// the format branches.
	for _, tc := range []struct {
		format string
		limit  int
	}{
		{"COMPACT", 8126},
		{"REDUNDANT", 8000},
	} {
		t.Run("control-"+tc.format, func(t *testing.T) {
			ruleUnderTest := t06a15RowSizeRule(t, true)
			statement := t06a15RowSizeStatement(spec.DialectMySQL,
				map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": tc.format}, columns, nil)
			findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 || findings[0].Metadata["limit"] != tc.limit ||
				findings[0].Metadata["estimated"] != 8448 {
				t.Fatalf("expected the %s %d-byte payload finding (8448), got %+v", tc.format, tc.limit, findings)
			}
		})
	}
}
