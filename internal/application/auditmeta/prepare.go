// Package auditmeta prepares metadata-aware audit requests for multiple adapters.
// input: audit SQL text, requested dialect/schema preferences, metadata connection configs, and metadata clients
// output: prepared metadata-aware audit context with opened client, resolved dialect, and resolved schema
// pos: shared application helper between transport adapters and the core audit service
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

// Client describes the metadata-aware capabilities required before audit execution.
type Client interface {
	appaudit.MetadataProvider
	DetectDialect(ctx context.Context) (spec.Dialect, error)
	FindSchemasForTable(ctx context.Context, table string) ([]string, error)
	Close() error
}

// ConnectionConfig describes one metadata-aware connection opening request.
type ConnectionConfig struct {
	Host     string
	Port     int
	Socket   string
	User     string
	Password string
}

// Request describes one shared metadata-aware audit preparation request.
type Request struct {
	SQL                  string
	Connection           ConnectionConfig
	RequestedDialect     spec.Dialect
	ExplicitDialect      bool
	ExplicitSchema       string
	ExplicitSchemaSource string
	SchemaHint           string
	OpenClient           func(ConnectionConfig) (Client, error)
}

// PreparedAudit captures the shared metadata-aware audit context needed by adapters.
type PreparedAudit struct {
	Client        Client
	Dialect       spec.Dialect
	Schema        string
	DialectSource string
	SchemaSource  string
}

// Prepare opens a metadata client, resolves dialect and schema, and returns prepared audit context.
func Prepare(ctx context.Context, request Request) (*PreparedAudit, error) {
	openClient := request.OpenClient
	if openClient == nil {
		openClient = openMySQLClient
	}

	client, err := openClient(request.Connection)
	if err != nil {
		return nil, newConnectionOpenError(err)
	}

	closeOnError := true
	defer func() {
		if closeOnError {
			_ = client.Close()
		}
	}()

	detectedDialect, err := client.DetectDialect(ctx)
	if err != nil {
		return nil, newDialectDetectError(err)
	}
	if request.ExplicitDialect && request.RequestedDialect != detectedDialect {
		return nil, newDialectMismatchError(string(detectedDialect), string(request.RequestedDialect))
	}

	schema, schemaSource, err := resolveSchema(ctx, client, request.SQL, detectedDialect, request.ExplicitSchema, explicitSchemaSource(request.ExplicitSchemaSource), schemaHint(request.SchemaHint))
	if err != nil {
		return nil, err
	}

	closeOnError = false
	return &PreparedAudit{
		Client:        client,
		Dialect:       detectedDialect,
		Schema:        schema,
		DialectSource: "detected",
		SchemaSource:  schemaSource,
	}, nil
}

func resolveSchema(ctx context.Context, client Client, sqlText string, dialect spec.Dialect, explicitSchema string, explicitSource string, hint string) (string, string, error) {
	if strings.TrimSpace(explicitSchema) != "" {
		return strings.TrimSpace(explicitSchema), explicitSource, nil
	}

	targets, err := collectTargetTables(sqlText, dialect)
	if err != nil {
		return "", "", newInvalidSQLError(err)
	}
	if len(targets) == 0 {
		return "", "none", nil
	}

	resolvedSchemas := make(map[string]struct{})
	for _, target := range targets {
		if target.Schema != "" {
			resolvedSchemas[target.Schema] = struct{}{}
			continue
		}
		schemas, err := client.FindSchemasForTable(ctx, target.Name)
		if err != nil {
			return "", "", newSchemaLookupFailedError(target.Name, err)
		}
		switch len(schemas) {
		case 0:
			if target.RequiresExisting {
				return "", "", newSchemaHintRequiredError(fmt.Sprintf("could not infer schema for table %q; set %s", target.Name, hint))
			}
		case 1:
			resolvedSchemas[schemas[0]] = struct{}{}
		default:
			return "", "", newSchemaHintRequiredError(fmt.Sprintf("schema inference for table %q is ambiguous; set %s", target.Name, hint))
		}
	}

	if len(resolvedSchemas) == 0 {
		return "", "none", nil
	}
	if len(resolvedSchemas) > 1 {
		schemas := make([]string, 0, len(resolvedSchemas))
		for schema := range resolvedSchemas {
			schemas = append(schemas, schema)
		}
		sort.Strings(schemas)
		return "", "", newSchemaHintRequiredError(fmt.Sprintf("resolved multiple schemas (%s); set %s", strings.Join(schemas, ", "), hint))
	}
	for schema := range resolvedSchemas {
		return schema, "inferred", nil
	}
	return "", "none", nil
}

func schemaHint(value string) string {
	if strings.TrimSpace(value) == "" {
		return "--schema"
	}
	return strings.TrimSpace(value)
}

func explicitSchemaSource(value string) string {
	if strings.TrimSpace(value) == "" {
		return "request"
	}
	return strings.TrimSpace(value)
}

func openMySQLClient(config ConnectionConfig) (Client, error) {
	db, err := mysqlmeta.OpenDB(mysqlmeta.ConnectionConfig{
		Host:     config.Host,
		Port:     config.Port,
		Socket:   config.Socket,
		User:     config.User,
		Password: config.Password,
	})
	if err != nil {
		return nil, err
	}
	return mysqlClient{
		db:       db,
		provider: mysqlmeta.NewProvider(db),
	}, nil
}
