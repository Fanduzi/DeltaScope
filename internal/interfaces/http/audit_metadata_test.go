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
	"time"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
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

func TestExecuteAuditRequestReturnsOfflineContext(t *testing.T) {
	var captured deltascope.Request

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "delete from users",
		Dialect: deltascope.DialectMySQL,
	}, "", func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
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
	}, MetadataConfig{})
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
	}, MetadataConfig{})
	if err == nil {
		t.Fatalf("expected config error")
	}
	status, code := mapAuditError(err)
	if status != 500 || code != "config_invalid" {
		t.Fatalf("expected config_invalid, got status=%d code=%s err=%v", status, code, err)
	}
}

func TestExecuteAuditRequestUsesConfigSnapshotForMetadataAwareAudit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if err := os.WriteFile(configPath, []byte("rules:\n  select:\n    require_where: nope\n"), 0o600); err != nil {
			t.Fatalf("mutate config: %v", err)
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

	var capturedConfigPath string
	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "secret",
			Schema:   "app",
		},
	}, configPath, func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		capturedConfigPath = request.ConfigPath
		if capturedConfigPath == configPath {
			t.Fatalf("expected metadata-aware audit to use config snapshot path")
		}
		if _, err := loadHTTPPolicy(capturedConfigPath); err != nil {
			t.Fatalf("expected snapshot config to remain loadable, got %v", err)
		}
		if _, err := loadHTTPPolicy(configPath); err == nil {
			t.Fatalf("expected original config mutation to make source path invalid")
		}
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
	if capturedConfigPath == "" {
		t.Fatalf("expected captured config snapshot path")
	}
	if _, err := os.Stat(capturedConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected config snapshot cleanup, got err=%v", err)
	}
}

func TestExecuteAuditRequestValidatesSnapshotInsteadOfSourcePolicyPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previousPrepare := prepareHTTPMetadataAudit
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
	t.Cleanup(func() { prepareHTTPMetadataAudit = previousPrepare })

	previousLoad := loadHTTPPolicy
	validatedSourcePath := false
	loadHTTPPolicy = func(path string) (domainpolicy.Policy, error) {
		if path == configPath {
			validatedSourcePath = true
			if err := os.WriteFile(configPath, []byte("rules:\n  select:\n    require_where: nope\n"), 0o600); err != nil {
				t.Fatalf("mutate config during validation: %v", err)
			}
			return domainpolicy.Default(), nil
		}
		return previousLoad(path)
	}
	t.Cleanup(func() { loadHTTPPolicy = previousLoad })

	var capturedConfigPath string
	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "secret",
			Schema:   "app",
		},
	}, configPath, func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		capturedConfigPath = request.ConfigPath
		if _, err := previousLoad(capturedConfigPath); err != nil {
			t.Fatalf("expected snapshot config to stay valid, got %v", err)
		}
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if validatedSourcePath {
		t.Fatalf("expected snapshot validation to avoid source policy path")
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
	if capturedConfigPath == "" {
		t.Fatalf("expected captured config snapshot path")
	}
}

func TestExecuteAuditRequestPassesConnectionConnectTimeout(t *testing.T) {
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

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:           "127.0.0.1",
			Port:           3306,
			User:           "root",
			Password:       "secret",
			Schema:         "app",
			ConnectTimeout: "5s",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 5*time.Second {
		t.Fatalf("expected ConnectTimeout=5s, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestExecuteAuditRequestAcceptsZeroConnectionConnectTimeoutAsDefault(t *testing.T) {
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

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:           "127.0.0.1",
			Port:           3306,
			User:           "root",
			Password:       "secret",
			Schema:         "app",
			ConnectTimeout: "0s",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 0 {
		t.Fatalf("expected ConnectTimeout=0 for 0s, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestExecuteAuditRequestRejectsInvalidConnectionConnectTimeout(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatal("expected prepare to not be called for invalid timeout")
		return nil, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:           "127.0.0.1",
			User:           "root",
			Password:       "secret",
			ConnectTimeout: "not-a-duration",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		t.Fatal("auditFn should not be called")
		return deltascope.Result{}, nil
	}, MetadataConfig{})
	if err == nil {
		t.Fatal("expected error for invalid connect_timeout")
	}
	status, code := mapAuditError(err)
	if status != 400 {
		t.Fatalf("expected 400 status, got %d code=%s err=%v", status, code, err)
	}
}

func TestExecuteAuditRequestRejectsNegativeConnectionConnectTimeout(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatal("expected prepare to not be called for negative timeout")
		return nil, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host:           "127.0.0.1",
			User:           "root",
			Password:       "secret",
			ConnectTimeout: "-5s",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		t.Fatal("auditFn should not be called")
		return deltascope.Result{}, nil
	}, MetadataConfig{})
	if err == nil {
		t.Fatal("expected error for negative connect_timeout")
	}
	status, code := mapAuditError(err)
	if status != 400 {
		t.Fatalf("expected 400 status, got %d code=%s err=%v", status, code, err)
	}
}

func TestHTTPRuntimeMetadataTimeoutUsedWhenRequestOmitsTimeout(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client: client, Dialect: spec.DialectMySQL, Schema: "app",
			DialectSource: "detected", SchemaSource: "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret", Schema: "app",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 7*time.Second {
		t.Fatalf("expected ConnectTimeout=7s from runtime default, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestHTTPRequestConnectTimeoutOverridesRuntimeDefault(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client: client, Dialect: spec.DialectMySQL, Schema: "app",
			DialectSource: "detected", SchemaSource: "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret",
			Schema: "app", ConnectTimeout: "3s",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 3*time.Second {
		t.Fatalf("expected ConnectTimeout=3s from request override, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestHTTPRequestZeroConnectTimeoutDoesNotOverrideRuntimeDefault(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client: client, Dialect: spec.DialectMySQL, Schema: "app",
			DialectSource: "detected", SchemaSource: "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret",
			Schema: "app", ConnectTimeout: "0s",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 7*time.Second {
		t.Fatalf("expected ConnectTimeout=7s from runtime default (0s should not override), got %v", capturedConfig.ConnectTimeout)
	}
}

func TestHTTPRuntimeMetadataTimeoutUnsetKeepsZeroDefault(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{}
	var capturedConfig auditmeta.ConnectionConfig
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		capturedConfig = request.Connection
		return &auditmeta.PreparedAudit{
			Client: client, Dialect: spec.DialectMySQL, Schema: "app",
			DialectSource: "detected", SchemaSource: "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret", Schema: "app",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}, MetadataConfig{})
	if err != nil {
		t.Fatalf("execute audit request: %v", err)
	}
	if capturedConfig.ConnectTimeout != 0 {
		t.Fatalf("expected ConnectTimeout=0 when no runtime default and no request timeout, got %v", capturedConfig.ConnectTimeout)
	}
}

func TestHTTPRejectsInvalidRequestConnectTimeoutStillBeforeOpen(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		t.Fatal("expected prepare to not be called for invalid timeout")
		return nil, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL: "delete from users",
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1", User: "root", Password: "secret",
			ConnectTimeout: "not-a-duration",
		},
	}, "", func(_ context.Context, _ deltascope.Request) (deltascope.Result, error) {
		t.Fatal("auditFn should not be called")
		return deltascope.Result{}, nil
	}, MetadataConfig{ConnectTimeout: 7 * time.Second})
	if err == nil {
		t.Fatal("expected error for invalid connect_timeout")
	}
	status, code := mapAuditError(err)
	if status != 400 {
		t.Fatalf("expected 400 status, got %d code=%s err=%v", status, code, err)
	}
}
