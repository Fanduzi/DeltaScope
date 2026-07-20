// Package httpapi verifies metadata-aware HTTP audit execution wiring.
// input: HTTP audit requests, fake metadata preparation results, and stub public audit functions
// output: focused coverage for offline context emission and registry-based metadata-aware adapter behavior
// pos: interface-layer tests for HTTP metadata-aware audit execution
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type metadataAuditTestClient struct {
	closed         bool
	detectDialect  spec.Dialect
	tableCalls     []string
	indexCalls     []string
	indexSchemas   []string
	indexDialects  []spec.Dialect
	indexTable     string
	snapshot       *spec.TableSnapshot
	objectSnapshot *spec.ObjectSnapshot
	objectCalls    []spec.ObjectLookupRequest
}

func (c *metadataAuditTestClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (c *metadataAuditTestClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, _ string, table string) (*spec.TableSnapshot, error) {
	c.tableCalls = append(c.tableCalls, table)
	if c.snapshot != nil {
		return c.snapshot, nil
	}
	return &spec.TableSnapshot{Exists: true}, nil
}

func (c *metadataAuditTestClient) DetectDialect(context.Context) (spec.Dialect, error) {
	if c.detectDialect == "" {
		return spec.DialectMySQL, nil
	}
	return c.detectDialect, nil
}

func (c *metadataAuditTestClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return []string{"app"}, nil
}

func (c *metadataAuditTestClient) ResolveTableForIndex(_ context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	c.indexCalls = append(c.indexCalls, index)
	c.indexDialects = append(c.indexDialects, dialect)
	c.indexSchemas = append(c.indexSchemas, schema)
	return c.indexTable, nil
}

func (c *metadataAuditTestClient) Close() error {
	c.closed = true
	return nil
}

func (c *metadataAuditTestClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	c.objectCalls = append(c.objectCalls, req)
	return c.objectSnapshot, nil
}

func newTestRegistry(t *testing.T, connID string, opts ...func(*runtimeconfig.ConnectionConfig)) *runtimeconfig.Registry {
	t.Helper()
	t.Setenv("TEST_DB_PASSWORD", "secret")
	t.Setenv("TEST_API_KEY_SECRET", "test-key-value")

	conn := runtimeconfig.ConnectionConfig{
		ID:               connID,
		Dialect:          "mysql",
		Host:             "127.0.0.1",
		Port:             3306,
		User:             "root",
		PasswordEnv:      "TEST_DB_PASSWORD",
		Schema:           "app",
		Purposes:         []string{"audit"},
		AllowedAPIKeyIDs: []string{"default-key"},
	}
	for _, opt := range opts {
		opt(&conn)
	}

	cfg := runtimeconfig.Config{
		HTTP: runtimeconfig.HTTPConfig{
			Auth: runtimeconfig.AuthConfig{
				Enabled: true,
				Keys:    []runtimeconfig.APIKeyConfig{{ID: "default-key", SecretEnv: "TEST_API_KEY_SECRET"}},
			},
		},
		Metadata: runtimeconfig.MetadataConfig{
			Connections: []runtimeconfig.ConnectionConfig{conn},
		},
	}
	reg, err := runtimeconfig.ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	return reg
}

func newTestRegistryWithAuth(t *testing.T, connID string, keyID string, allowedKeyIDs []string) *runtimeconfig.Registry {
	t.Helper()
	t.Setenv("TEST_DB_PASSWORD", "secret")
	t.Setenv("TEST_API_KEY_SECRET", "test-key-value")
	t.Setenv("TEST_OTHER_KEY_SECRET", "other-key-value")

	cfg := runtimeconfig.Config{
		HTTP: runtimeconfig.HTTPConfig{
			Auth: runtimeconfig.AuthConfig{
				Enabled: true,
				Keys: []runtimeconfig.APIKeyConfig{
					{ID: keyID, SecretEnv: "TEST_API_KEY_SECRET"},
					{ID: "other-key", SecretEnv: "TEST_OTHER_KEY_SECRET"},
				},
			},
		},
		Metadata: runtimeconfig.MetadataConfig{
			Connections: []runtimeconfig.ConnectionConfig{
				{
					ID:               connID,
					Dialect:          "mysql",
					Host:             "127.0.0.1",
					Port:             3306,
					User:             "root",
					PasswordEnv:      "TEST_DB_PASSWORD",
					Schema:           "app",
					Purposes:         []string{"audit"},
					AllowedAPIKeyIDs: allowedKeyIDs,
				},
			},
		},
	}
	reg, err := runtimeconfig.ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	return reg
}

func TestExecuteAuditRequestReturnsOfflineContext(t *testing.T) {
	var captured deltascope.Request

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "delete from users",
		Dialect: deltascope.DialectMySQL,
	}, "", func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{}, nil, "")
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

func TestExecuteAuditRequestReturnsMetadataAwareContextForRegistryConnection(t *testing.T) {
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

	reg := newTestRegistry(t, "test-conn")

	var captured deltascope.Request
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		if request.MetadataProvider == nil {
			t.Fatalf("expected metadata provider")
		}
		if _, err := request.MetadataProvider.LoadInstanceFacts(ctx, request.Dialect, request.Schema); err != nil {
			t.Fatalf("load instance facts: %v", err)
		}
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{}, reg, "default-key")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if response.Context == nil {
		t.Fatalf("expected additive context")
	}
	if response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", response.Context.Mode)
	}
	if response.Context.MetadataSource != "registry" {
		t.Fatalf("expected registry metadata source, got %#v", response.Context.MetadataSource)
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

func TestExecuteAuditRequestReturnsConnectionNotFoundForMissingID(t *testing.T) {
	reg := newTestRegistry(t, "test-conn")

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "nonexistent",
	}, "", func(context.Context, deltascope.Request) (deltascope.Result, error) {
		t.Fatalf("auditFn should not be called for missing connection")
		return deltascope.Result{}, nil
	}, MetadataConfig{}, reg, "")
	if err == nil {
		t.Fatalf("expected error for missing connection")
	}
	if !errors.Is(err, online.ErrConnectionNotFound) {
		t.Fatalf("expected ErrConnectionNotFound, got %v", err)
	}
}

func TestExecuteAuditRequestReturnsConnectionNotFoundWhenRegistryNil(t *testing.T) {
	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(context.Context, deltascope.Request) (deltascope.Result, error) {
		t.Fatalf("auditFn should not be called when registry is nil")
		return deltascope.Result{}, nil
	}, MetadataConfig{}, nil, "")
	if err == nil {
		t.Fatalf("expected error when registry is nil")
	}
	if !errors.Is(err, online.ErrConnectionNotFound) {
		t.Fatalf("expected ErrConnectionNotFound, got %v", err)
	}
}

func TestExecuteAuditRequestReturnsNotAuthorizedWhenPrincipalNotAllowed(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatalf("prepare should not be called when principal is not authorized")
		return nil, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistryWithAuth(t, "test-conn", "key-1", []string{"other-key"})

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(context.Context, deltascope.Request) (deltascope.Result, error) {
		t.Fatalf("auditFn should not be called when unauthorized")
		return deltascope.Result{}, nil
	}, MetadataConfig{}, reg, "key-1")
	if err == nil {
		t.Fatalf("expected authorization error")
	}
	if !errors.Is(err, online.ErrPrincipalNotAllowed) {
		t.Fatalf("expected ErrPrincipalNotAllowed, got %v", err)
	}
}

func TestExecuteAuditRequestAllowsAuthorizedPrincipal(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistryWithAuth(t, "test-conn", "key-1", []string{"key-1"})

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{}, reg, "key-1")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if response.Context == nil {
		t.Fatalf("expected additive context")
	}
	if response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", response.Context.Mode)
	}
}

func TestExecuteAuditRequestPassesRegistryConnectTimeout(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistry(t, "test-conn", func(c *runtimeconfig.ConnectionConfig) {
		c.ConnectTimeout = "5s"
	})

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{}, reg, "default-key")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 5*time.Second {
		t.Fatalf("expected ConnectTimeout=5s, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestExecuteAuditRequestUsesRuntimeDefaultWhenRegistryTimeoutUnset(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistry(t, "test-conn")

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second}, reg, "default-key")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 7*time.Second {
		t.Fatalf("expected ConnectTimeout=7s from runtime default, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestExecuteAuditRequestRegistryTimeoutOverridesRuntimeDefault(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistry(t, "test-conn", func(c *runtimeconfig.ConnectionConfig) {
		c.ConnectTimeout = "3s"
	})

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second}, reg, "default-key")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 3*time.Second {
		t.Fatalf("expected ConnectTimeout=3s from registry override, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestExecuteAuditRequestRequestSchemaOverridesRegistrySchema(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedRequest auditmeta.Request
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedRequest = request
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectMySQL,
			Schema:        "custom_schema",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistry(t, "test-conn")

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:          "delete from users",
		ConnectionID: "test-conn",
		Schema:       "custom_schema",
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{}, reg, "default-key")
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedRequest.ExplicitSchema != "custom_schema" {
		t.Fatalf("expected explicit schema custom_schema, got %q", capturedRequest.ExplicitSchema)
	}
	if response.Context.Schema != "custom_schema" {
		t.Fatalf("expected schema custom_schema in context, got %q", response.Context.Schema)
	}
}

func TestMapRegistryAuthorizeErrorMapsAllCases(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{"connection_not_found", runtimeconfig.ErrConnectionNotFound, online.ErrConnectionNotFound},
		{"purpose_not_allowed", runtimeconfig.ErrPurposeNotAllowed, online.ErrPurposeNotAllowed},
		{"principal_not_allowed", runtimeconfig.ErrPrincipalNotAllowed, online.ErrPrincipalNotAllowed},
		{"passthrough", context.Canceled, context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRegistryAuthorizeError(tt.input)
			if !errors.Is(got, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
