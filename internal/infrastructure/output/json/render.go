// Package jsonrender renders audit results as machine-oriented JSON.
package jsonrender

import (
	"encoding/json"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
)

// Render formats an audit result into indented JSON.
func Render(result report.Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
