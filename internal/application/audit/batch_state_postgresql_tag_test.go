//go:build postgresql

// Package audit verifies that the T05 ordered schema-state machine is scoped
// to MySQL/TiDB only.
// input: PostgreSQL audit requests across every request/provider shape
// output: assertions that no request shape enters the ordered derivation
// path — PostgreSQL keeps its pre-T05 metadata enrichment unchanged
// pos: application-layer dialect-scope guard for issue #84/T05-A1-R1
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// C4: PostgreSQL with a request schema and no provider must not enter the
// ordered-state pass — the pre-T05 loop attaches schema/version metadata but
// never derives table snapshots.
func TestBatchStatePostgreSQLSchemaWithoutProviderStaysOnLegacyPath(t *testing.T) {
	t.Parallel()
	parsed, parseErr := parseSQL(context.Background(),
		"CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;", spec.DialectPostgreSQL)
	if parseErr != nil || len(parsed.Statements) != 2 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL,
		&MetadataRequest{Schema: "public"}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	for i, statement := range enriched {
		if statement.Metadata != nil && statement.Metadata.TargetTable != nil {
			t.Fatalf("PostgreSQL statement %d must not carry a derived target snapshot, got %+v", i, statement.Metadata.TargetTable)
		}
	}
	if enriched[1].Metadata == nil || enriched[1].Metadata.Schema != "public" {
		t.Fatalf("PostgreSQL metadata schema must stay attached, got %+v", enriched[1].Metadata)
	}
}

// C4: PostgreSQL with provider also keeps its per-statement enrichment —
// the ordered pass is never entered regardless of request shape.
func TestBatchStatePostgreSQLProviderStaysOnLegacyPath(t *testing.T) {
	t.Parallel()
	parsed, parseErr := parseSQL(context.Background(),
		"CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;", spec.DialectPostgreSQL)
	if parseErr != nil || len(parsed.Statements) != 2 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"public.t": {
			Exists:  true,
			Table:   &spec.Table{Schema: "public", Name: "t"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		},
	}}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL,
		&MetadataRequest{Schema: "public", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	// The legacy path attaches the provider snapshot to each statement
	// independently — no in-batch derivation, no cross-statement state.
	if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable == nil {
		t.Fatalf("PostgreSQL provider path still resolves table metadata, got %+v", enriched[1].Metadata)
	}
	if got := len(enriched[1].Metadata.TargetTable.Columns); got != 1 {
		t.Fatalf("PostgreSQL must read the provider snapshot verbatim (no derived c column), got %d columns", got)
	}
}
