// Package cli exposes the command-line adapter for DeltaScope.
// input: metadata-aware audit connection options, SQL text, and metadata provider clients
// output: resolved dialect/schema context and provider wiring for metadata-aware CLI audits
// pos: CLI metadata-aware audit preparation between command flags and application requests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"database/sql"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
	postgresqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/postgresql"
)

type metadataClient = auditmeta.Client

var newMetadataClient = openMetadataClient

type auditRunContext struct {
	Mode          string `json:"mode,omitempty"`
	Dialect       string `json:"dialect,omitempty"`
	DialectSource string `json:"dialect_source,omitempty"`
	Schema        string `json:"schema,omitempty"`
	SchemaSource  string `json:"schema_source,omitempty"`
}

func openMetadataClient(options auditConnectionOptions) (metadataClient, error) {
	if options.Dialect == string(spec.DialectPostgreSQL) {
		db, err := postgresqlmeta.OpenDB(postgresqlmeta.ConnectionConfig{
			Host:           options.Host,
			Port:           options.Port,
			Socket:         options.Socket,
			User:           options.User,
			Password:       options.Password,
			ConnectTimeout: options.ConnectTimeout,
		})
		if err != nil {
			return nil, err
		}
		return postgresqlMetadataClient{
			db:       db,
			provider: postgresqlmeta.NewProvider(db),
		}, nil
	}

	db, err := mysqlmeta.OpenDB(mysqlmeta.ConnectionConfig{
		Host:           options.Host,
		Port:           options.Port,
		Socket:         options.Socket,
		User:           options.User,
		Password:       options.Password,
		ConnectTimeout: options.ConnectTimeout,
	})
	if err != nil {
		return nil, err
	}
	return mysqlMetadataClient{
		db:       db,
		provider: mysqlmeta.NewProvider(db),
	}, nil
}

type mysqlMetadataClient struct {
	db       *sql.DB
	provider *mysqlmeta.Provider
}

func (c mysqlMetadataClient) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return c.provider.LoadInstanceFacts(ctx, dialect, schema)
}

func (c mysqlMetadataClient) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return c.provider.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (c mysqlMetadataClient) DetectDialect(ctx context.Context) (spec.Dialect, error) {
	return c.provider.DetectDialect(ctx)
}

func (c mysqlMetadataClient) FindSchemasForTable(ctx context.Context, table string) ([]string, error) {
	return c.provider.FindSchemasForTable(ctx, table)
}

func (c mysqlMetadataClient) ResolveTableForIndex(context.Context, spec.Dialect, string, string) (string, error) {
	return "", nil
}

func (c mysqlMetadataClient) Close() error {
	return c.db.Close()
}

type postgresqlMetadataClient struct {
	db       *sql.DB
	provider *postgresqlmeta.Provider
}

func (c postgresqlMetadataClient) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return c.provider.LoadInstanceFacts(ctx, dialect, schema)
}

func (c postgresqlMetadataClient) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return c.provider.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (c postgresqlMetadataClient) DetectDialect(ctx context.Context) (spec.Dialect, error) {
	return c.provider.DetectDialect(ctx)
}

func (c postgresqlMetadataClient) FindSchemasForTable(ctx context.Context, table string) ([]string, error) {
	return c.provider.FindSchemasForTable(ctx, table)
}

func (c postgresqlMetadataClient) ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	return c.provider.ResolveTableForIndex(ctx, dialect, schema, index)
}

func (c postgresqlMetadataClient) LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	return c.provider.LoadPlanEstimate(ctx, statement)
}

func (c postgresqlMetadataClient) Close() error {
	return c.db.Close()
}

func prepareMetadataAudit(ctx context.Context, sqlText string, options auditConnectionOptions, requestedDialect spec.Dialect, explicitDialect bool) (metadataClient, spec.Dialect, string, *auditRunContext, error) {
	prepared, err := auditmeta.Prepare(ctx, auditmeta.Request{
		SQL:                  sqlText,
		Connection:           toAuditMetaConnection(options, requestedDialect, explicitDialect),
		RequestedDialect:     requestedDialect,
		ExplicitDialect:      explicitDialect,
		ExplicitSchema:       options.Schema,
		ExplicitSchemaSource: "flag",
		OpenClient: func(config auditmeta.ConnectionConfig) (auditmeta.Client, error) {
			return newMetadataClient(auditConnectionOptions{
				Host:           config.Host,
				Port:           config.Port,
				Socket:         config.Socket,
				User:           config.User,
				Password:       config.Password,
				Dialect:        string(config.Dialect),
				ConnectTimeout: config.ConnectTimeout,
			})
		},
	})
	if err != nil {
		return nil, "", "", nil, err
	}

	return prepared.Client, prepared.Dialect, prepared.Schema, &auditRunContext{
		Mode:          "metadata-aware",
		Dialect:       string(prepared.Dialect),
		DialectSource: prepared.DialectSource,
		Schema:        prepared.Schema,
		SchemaSource:  prepared.SchemaSource,
	}, nil
}

func toAuditMetaConnection(options auditConnectionOptions, requestedDialect spec.Dialect, explicitDialect bool) auditmeta.ConnectionConfig {
	connection := auditmeta.ConnectionConfig{
		Host:           options.Host,
		Port:           options.Port,
		Socket:         options.Socket,
		User:           options.User,
		Password:       options.Password,
		ConnectTimeout: options.ConnectTimeout,
	}
	if explicitDialect {
		connection.Dialect = requestedDialect
	}
	return connection
}
