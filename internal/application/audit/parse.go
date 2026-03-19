// Package audit orchestrates audit use cases at the application layer.
// input: SQL text, selected dialect, and infrastructure-backed parser adapters
// output: parsed statement bundles for later extraction and rule evaluation
// pos: application parsing entrypoint between interfaces and parser infrastructure
// note: if this file changes, update this header and module README.md.
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
)

// Parse delegates SQL parsing to the TiDB-backed parser adapter for supported v1 dialects.
func Parse(sql string, dialect spec.Dialect) (tidbparser.Result, error) {
	return tidbparser.New().Parse(sql, dialect)
}
