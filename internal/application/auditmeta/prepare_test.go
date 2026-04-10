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

func TestPrepareReturnsCapabilityBoundaryErrorWithoutSchemaWrapperForExplicitPostgreSQL(t *testing.T) {
	t.Parallel()

	client := &fakeClient{detectDialect: spec.DialectPostgreSQL}
	_, err := Prepare(context.Background(), Request{
		SQL:              "select 1;",
		RequestedDialect: spec.DialectPostgreSQL,
		ExplicitDialect:  true,
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err == nil {
		t.Fatal("expected capability-boundary error")
	}
	var prepErr *Error
	if !errors.As(err, &prepErr) {
		t.Fatalf("expected typed prepare error, got %T", err)
	}
	if prepErr.Kind != ErrorInvalidSQL {
		t.Fatalf("expected invalid_sql kind, got %#v", prepErr)
	}
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if !errors.As(prepErr.Err, &capabilityErr) {
		t.Fatalf("expected typed capability-boundary cause, got %T", prepErr.Err)
	}
	if err.Error() != capabilityErr.Error() {
		t.Fatalf("expected unwrapped capability message, got %q want %q", err.Error(), capabilityErr.Error())
	}
	if strings.Contains(strings.ToLower(err.Error()), "resolve schema targets:") {
		t.Fatalf("did not expect schema wrapper, got %q", err.Error())
	}
	if !client.closed {
		t.Fatal("expected client to close on prepare failure")
	}
}

func TestPrepareKeepsSchemaWrapperForNonCapabilityInvalidSQL(t *testing.T) {
	t.Parallel()

	client := &fakeClient{detectDialect: spec.DialectMySQL}
	_, err := Prepare(context.Background(), Request{
		SQL: "select from users where;",
		OpenClient: func(config ConnectionConfig) (Client, error) {
			return client, nil
		},
	})
	if err == nil {
		t.Fatal("expected invalid sql error")
	}
	var prepErr *Error
	if !errors.As(err, &prepErr) {
		t.Fatalf("expected typed prepare error, got %T", err)
	}
	if prepErr.Kind != ErrorInvalidSQL {
		t.Fatalf("expected invalid_sql kind, got %#v", prepErr)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "resolve schema targets:") {
		t.Fatalf("expected schema wrapper to remain, got %q", err.Error())
	}
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if errors.As(prepErr.Err, &capabilityErr) {
		t.Fatalf("did not expect capability-boundary cause, got %#v", capabilityErr)
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

var _ Client = (*fakeClient)(nil)
var _ appaudit.MetadataProvider = (*fakeClient)(nil)
