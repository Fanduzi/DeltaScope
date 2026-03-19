// Package jsonrender renders audit results as machine-oriented JSON.
// input: internal report results from the audit application flow
// output: stable JSON bytes for machine consumption
// pos: infrastructure output adapter for structured renderer output
// note: if this file changes, update this header and module README.md.
package jsonrender

import (
	"encoding/json"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
)

// Render formats an audit result into indented JSON.
func Render(result report.Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
