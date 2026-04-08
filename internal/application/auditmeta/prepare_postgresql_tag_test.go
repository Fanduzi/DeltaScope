//go:build postgresql

// Package auditmeta verifies shared metadata-aware audit preparation for PostgreSQL.
// input: metadata-aware audit requests with PostgreSQL dialect and fake metadata clients
// output: focused coverage for PostgreSQL-specific schema inference, fallback open, and client contracts
// pos: application-layer preparation tests gated behind the postgresql build tag
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

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
