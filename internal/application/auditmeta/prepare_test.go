// Package auditmeta verifies shared metadata-aware audit preparation.
// input: metadata-aware audit requests plus fake metadata clients that simulate dialect and schema lookups
// output: focused coverage for shared connection setup, schema inference, and dialect validation
// pos: application-layer preparation tests shared by CLI and MCP adapters
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"errors"
	"strings"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeClient struct {
	connectionConfig  ConnectionConfig
	detectDialect     spec.Dialect
	detectErr         error
	schemasByTable    map[string][]string
	findSchemaErr     error
	findSchemaCalls   []string
	instanceCalls     []string
	tableSnapshotCall []struct {
		Schema string
		Table  string
	}
	closed bool
}

func (f *fakeClient) LoadInstanceFacts(_ context.Context, _ spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	f.instanceCalls = append(f.instanceCalls, schema)
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (f *fakeClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	f.tableSnapshotCall = append(f.tableSnapshotCall, struct {
		Schema string
		Table  string
	}{Schema: schema, Table: table})
	return &spec.TableSnapshot{Schema: schema, Exists: true}, nil
}

func (f *fakeClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return f.detectDialect, f.detectErr
}

func (f *fakeClient) FindSchemasForTable(_ context.Context, table string) ([]string, error) {
	if f.findSchemaErr != nil {
		return nil, f.findSchemaErr
	}
	f.findSchemaCalls = append(f.findSchemaCalls, strings.ToLower(table))
	return f.schemasByTable[strings.ToLower(table)], nil
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

func TestPrepareReturnsResolvedDialectAndSchema(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}

	prepared, err := Prepare(context.Background(), Request{
		SQL:        "delete from users",
		Connection: ConnectionConfig{Host: "127.0.0.1", Port: 3307, User: "root"},
		OpenClient: func(config ConnectionConfig) (Client, error) {
			client.connectionConfig = config
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if prepared.Dialect != spec.DialectMySQL {
		t.Fatalf("expected mysql dialect, got %q", prepared.Dialect)
	}
	if prepared.Schema != "app" {
		t.Fatalf("expected app schema, got %q", prepared.Schema)
	}
	if prepared.DialectSource != "detected" {
		t.Fatalf("expected detected dialect source, got %q", prepared.DialectSource)
	}
	if prepared.SchemaSource != "inferred" {
		t.Fatalf("expected inferred schema source, got %q", prepared.SchemaSource)
	}
	if client.connectionConfig.Host != "127.0.0.1" || client.connectionConfig.Port != 3307 || client.connectionConfig.User != "root" {
		t.Fatalf("unexpected connection config: %#v", client.connectionConfig)
	}
}

func TestPrepareUsesExplicitSchemaWithoutInference(t *testing.T) {
	t.Parallel()

	client := &fakeClient{detectDialect: spec.DialectMySQL}
	prepared, err := Prepare(context.Background(), Request{
		SQL:            "delete from users",
		ExplicitSchema: "app",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if prepared.Schema != "app" || prepared.SchemaSource != "request" {
		t.Fatalf("unexpected schema resolution: %#v", prepared)
	}
	if len(client.findSchemaCalls) != 0 {
		t.Fatalf("expected no schema inference calls, got %#v", client.findSchemaCalls)
	}
}

func TestPrepareFailsWhenSchemaInferenceIsAmbiguous(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app", "archive"}},
	}

	_, err := Prepare(context.Background(), Request{
		SQL:        "delete from users",
		SchemaHint: "connection.schema",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err == nil {
		t.Fatal("expected ambiguous schema error")
	}
	if !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "connection.schema") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !client.closed {
		t.Fatal("expected client to close on prepare failure")
	}
}

func TestPrepareAllowsCreateTableWithoutResolvableSchema(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": nil},
	}

	prepared, err := Prepare(context.Background(), Request{
		SQL: "create table users (id bigint)",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if prepared.Schema != "" || prepared.SchemaSource != "none" {
		t.Fatalf("unexpected schema resolution: %#v", prepared)
	}
}

func TestPrepareIgnoresCreateViewForSchemaInference(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		detectDialect:  spec.DialectPostgreSQL,
		schemasByTable: map[string][]string{"active_users": {"public"}},
	}

	prepared, err := Prepare(context.Background(), Request{
		SQL: "create view active_users as select id from users",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if prepared.Schema != "" || prepared.SchemaSource != "none" {
		t.Fatalf("unexpected schema resolution: %#v", prepared)
	}
	if len(client.findSchemaCalls) != 0 {
		t.Fatalf("expected no schema inference calls for create view, got %#v", client.findSchemaCalls)
	}
}

func TestPrepareIgnoresDropViewForSchemaInference(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		detectDialect:  spec.DialectPostgreSQL,
		schemasByTable: map[string][]string{"active_users": {"public"}},
	}

	prepared, err := Prepare(context.Background(), Request{
		SQL: "drop view active_users",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if prepared.Schema != "" || prepared.SchemaSource != "none" {
		t.Fatalf("unexpected schema resolution: %#v", prepared)
	}
	if len(client.findSchemaCalls) != 0 {
		t.Fatalf("expected no schema inference calls for drop view, got %#v", client.findSchemaCalls)
	}
}

func TestPrepareRejectsDialectMismatch(t *testing.T) {
	t.Parallel()

	client := &fakeClient{detectDialect: spec.DialectTiDB}
	_, err := Prepare(context.Background(), Request{
		SQL:              "delete from users",
		RequestedDialect: spec.DialectMySQL,
		ExplicitDialect:  true,
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err == nil {
		t.Fatal("expected dialect mismatch error")
	}
	if !strings.Contains(err.Error(), `detected dialect "tidb" does not match requested dialect "mysql"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !client.closed {
		t.Fatal("expected client to close on prepare failure")
	}
}

func TestPrepareReturnsOpenError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	_, err := Prepare(context.Background(), Request{
		SQL: "delete from users",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return nil, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped open error, got %v", err)
	}
	if !strings.Contains(err.Error(), "open metadata connection") {
		t.Fatalf("expected open metadata connection context, got %v", err)
	}
}

func TestPrepareFallsBackToPostgreSQLOpenWhenDialectNotExplicit(t *testing.T) {
	t.Parallel()

	mysqlErr := errors.New("mysql open failed")
	client := &fakeClient{detectDialect: spec.DialectPostgreSQL, schemasByTable: map[string][]string{"users": {"public"}}}
	calls := 0
	prepared, err := Prepare(context.Background(), Request{
		SQL:            "delete from users where id = 1",
		Connection:     ConnectionConfig{Host: "127.0.0.1", User: "root"},
		ExplicitSchema: "public",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			calls++
			if calls == 1 {
				if config.Dialect != "" {
					t.Fatalf("expected first open without dialect hint, got %#v", config)
				}
				return nil, mysqlErr
			}
			if config.Dialect != spec.DialectPostgreSQL {
				t.Fatalf("expected postgres fallback hint, got %#v", config)
			}
			client.connectionConfig = config
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if calls != 2 {
		t.Fatalf("expected two open attempts, got %d", calls)
	}
	if prepared.Dialect != spec.DialectPostgreSQL {
		t.Fatalf("expected postgres dialect, got %#v", prepared.Dialect)
	}
	if prepared.Schema != "public" {
		t.Fatalf("expected public schema, got %#v", prepared.Schema)
	}
}

func TestPrepareFallsBackToPostgreSQLOpenWhenDetectFailsOnInitialClient(t *testing.T) {
	t.Parallel()

	firstClient := &fakeClient{detectErr: errors.New("mysql detect failed")}
	fallbackClient := &fakeClient{detectDialect: spec.DialectPostgreSQL, schemasByTable: map[string][]string{"users": {"public"}}}
	calls := 0
	prepared, err := Prepare(context.Background(), Request{
		SQL:            "delete from users where id = 1",
		Connection:     ConnectionConfig{Host: "127.0.0.1", User: "root"},
		ExplicitSchema: "public",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			calls++
			switch calls {
			case 1:
				if config.Dialect != "" {
					t.Fatalf("expected first open without dialect hint, got %#v", config)
				}
				return firstClient, nil
			case 2:
				if config.Dialect != spec.DialectPostgreSQL {
					t.Fatalf("expected postgres fallback hint, got %#v", config)
				}
				return fallbackClient, nil
			default:
				t.Fatalf("unexpected extra open attempt: %d", calls)
				return nil, nil
			}
		},
	})
	if err != nil {
		t.Fatalf("prepare metadata audit: %v", err)
	}
	t.Cleanup(func() { _ = prepared.Client.Close() })

	if calls != 2 {
		t.Fatalf("expected two open attempts, got %d", calls)
	}
	if !firstClient.closed {
		t.Fatal("expected initial client to close before postgres fallback")
	}
	if prepared.Dialect != spec.DialectPostgreSQL {
		t.Fatalf("expected postgres dialect, got %#v", prepared.Dialect)
	}
}

func TestPostgreSQLClientImplementsIndexOwnerResolver(t *testing.T) {
	t.Parallel()

	resolver, ok := any(postgresqlClient{}).(interface {
		ResolveTableForIndex(context.Context, spec.Dialect, string, string) (string, error)
	})
	if !ok {
		t.Fatal("expected shared postgresql client to implement index-owner resolver")
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

var _ Client = (*fakeClient)(nil)
var _ appaudit.MetadataProvider = (*fakeClient)(nil)
