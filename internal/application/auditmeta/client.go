// Package auditmeta prepares metadata-aware audit requests for multiple adapters.
// input: MySQL-compatible metadata connections and provider methods from infrastructure adapters
// output: shared client wrapper implementing the metadata preparation client contract
// pos: infrastructure-backed client bridge for shared metadata-aware audit preparation
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"database/sql"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

type mysqlClient struct {
	db       *sql.DB
	provider *mysqlmeta.Provider
}

// OpenClient opens a shared metadata client for the requested dialect.
func OpenClient(config ConnectionConfig) (Client, error) {
	return openMySQLClient(config)
}

func (c mysqlClient) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return c.provider.LoadInstanceFacts(ctx, dialect, schema)
}

func (c mysqlClient) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return c.provider.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (c mysqlClient) DetectDialect(ctx context.Context) (spec.Dialect, error) {
	return c.provider.DetectDialect(ctx)
}

func (c mysqlClient) FindSchemasForTable(ctx context.Context, table string) ([]string, error) {
	return c.provider.FindSchemasForTable(ctx, table)
}

func (c mysqlClient) Close() error {
	return c.db.Close()
}
