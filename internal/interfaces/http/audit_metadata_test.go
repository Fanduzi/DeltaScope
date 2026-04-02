// Package httpapi verifies metadata-aware HTTP audit execution wiring.
// input: HTTP audit requests, fake metadata preparation results, and stub public audit functions
// output: focused coverage for offline context emission and direct metadata-aware adapter behavior
// pos: interface-layer tests for HTTP metadata-aware audit execution
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type metadataAuditTestClient struct {
	closed bool
}

func (c *metadataAuditTestClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (c *metadataAuditTestClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return &spec.TableSnapshot{Exists: true}, nil
}

func (c *metadataAuditTestClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectMySQL, nil
}

func (c *metadataAuditTestClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return []string{"app"}, nil
}

func (c *metadataAuditTestClient) Close() error {
	c.closed = true
	return nil
}

func TestExecuteAuditRequestReturnsOfflineContext(t *testing.T) {
	var captured deltascope.Request

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "delete from users",
		Dialect: deltascope.DialectMySQL,
	}, "", func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if response.Context == nil {
		t.Fatalf("expected additive context")
	}
	if response.Context.Mode != "offline" {
		t.Fatalf("expected offline mode, got %#v", response.Context.Mode)
	}
	if response.Context.MetadataSource != "none" {
		t.Fatalf("expected metadata source none, got %#v", response.Context.MetadataSource)
	}
	if captured.MetadataProvider != nil {
		t.Fatalf("expected offline request to avoid metadata provider")
	}
}

func TestExecuteAuditRequestReturnsMetadataAwareContextForDirectConnection(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Host != "127.0.0.1" || request.Connection.User != "root" || request.Connection.Password != "secret" {
			t.Fatalf("unexpected connection config: %#v", request.Connection)
		}
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	var captured deltascope.Request
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "secret",
			Schema:   "app",
		},
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		if request.MetadataProvider == nil {
			t.Fatalf("expected metadata provider")
		}
		if _, err := request.MetadataProvider.LoadInstanceFacts(ctx, request.Dialect, request.Schema); err != nil {
			t.Fatalf("load instance facts: %v", err)
		}
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if response.Context == nil {
		t.Fatalf("expected additive context")
	}
	if response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", response.Context.Mode)
	}
	if response.Context.MetadataSource != "direct" {
		t.Fatalf("expected direct metadata source, got %#v", response.Context.MetadataSource)
	}
	if captured.Schema != "app" {
		t.Fatalf("expected schema to flow to public audit request, got %#v", captured.Schema)
	}
	if captured.Dialect != deltascope.DialectMySQL {
		t.Fatalf("expected mysql dialect, got %#v", captured.Dialect)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
}

func TestExecuteAuditRequestReturnsConfigErrorBeforeMetadataPreparation(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := NewHandler(configPath, "test-build"); err != nil {
		t.Fatalf("new handler: %v", err)
	}
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatalf("prepareHTTPMetadataAudit should not be called when config reload fails")
		return nil, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1",
			User: "root",
		},
	}, configPath, func(context.Context, deltascope.Request) (deltascope.Result, error) {
		t.Fatalf("auditFn should not be called when config reload fails")
		return deltascope.Result{}, nil
	})
	if err == nil {
		t.Fatalf("expected config error")
	}
	status, code := mapAuditError(err)
	if status != 500 || code != "config_invalid" {
		t.Fatalf("expected config_invalid, got status=%d code=%s err=%v", status, code, err)
	}
}
