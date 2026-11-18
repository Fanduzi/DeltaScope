package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestInstanceFactsUseSystemNamespace(t *testing.T) {
	db, log := openTestDB(t, map[string]testQueryResult{
		"select version()":            {columns: []string{"version"}, rows: [][]driver.Value{{"search_path_shadow"}}},
		"select pg_catalog.version()": {columns: []string{"version"}, rows: [][]driver.Value{{"PostgreSQL 17.7"}}},
		"from pg_settings":            {columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", false}}},
		"from pg_catalog.pg_settings": {columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", true}}},
	})
	defer db.Close()
	p := NewProvider(db)
	dialect, detectErr := p.DetectDialect(context.Background())
	facts, factsErr := p.LoadInstanceFacts(context.Background(), spec.DialectPostgreSQL, "source_shadow")
	if detectErr != nil || factsErr != nil || dialect != spec.DialectPostgreSQL || facts == nil || facts.Version != "PostgreSQL 17.7" || !facts.PGCompressionLZ4Known || !facts.PGCompressionLZ4Supported {
		t.Fatalf("dialect=%s facts=%+v detectErr=%v factsErr=%v", dialect, facts, detectErr, factsErr)
	}
	queries := log.Queries()
	if len(queries) != 3 {
		t.Fatal(queries)
	}
	for _, q := range queries {
		if strings.Contains(q, "version()") && !strings.Contains(q, "pg_catalog.version()") {
			t.Fatal("search_path version lookup", q)
		}
		if strings.Contains(q, "pg_settings") && (!strings.Contains(q, "from pg_catalog.pg_settings") || strings.Count(q, "OPERATOR(pg_catalog.=)") != 3) {
			t.Fatal("view, enum membership or row comparison can use search_path", q)
		}
	}
}
