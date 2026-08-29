// Package cli verifies metadata-aware CLI audit wiring.
// input: audit command args plus fake metadata clients that simulate dialect, schema, and connection-option resolution
// output: focused coverage for metadata-mode connection setup, schema inference, dialect validation, and port defaults
// pos: interface-layer metadata-aware audit test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"crypto/x509"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataClient struct {
	options            auditConnectionOptions
	detectDialect      spec.Dialect
	detectErr          error
	schemasByTable     map[string][]string
	findSchemaCalls    []string
	findSchemaErr      error
	instanceCalls      []string
	indexCalls         []string
	indexSchemas       []string
	indexDialects      []spec.Dialect
	indexTable         string
	planCalls          int
	tableSnapshotCalls []struct {
		Schema string
		Table  string
	}
	snapshot       *spec.TableSnapshot
	objectSnapshot *spec.ObjectSnapshot
	objectCalls    []spec.ObjectLookupRequest
	closed         bool
}

func (f *fakeMetadataClient) LoadInstanceFacts(_ context.Context, _ spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	f.instanceCalls = append(f.instanceCalls, schema)
	return &spec.InstanceFacts{Version: "8.0.36", DefaultCharset: "utf8mb4"}, nil
}

func (f *fakeMetadataClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	f.tableSnapshotCalls = append(f.tableSnapshotCalls, struct {
		Schema string
		Table  string
	}{Schema: schema, Table: table})
	if f.snapshot != nil {
		return f.snapshot, nil
	}
	return &spec.TableSnapshot{Schema: schema, Exists: true, Table: &spec.Table{Name: table}}, nil
}

func (f *fakeMetadataClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return f.detectDialect, f.detectErr
}

func (f *fakeMetadataClient) FindSchemasForTable(_ context.Context, table string) ([]string, error) {
	if f.findSchemaErr != nil {
		return nil, f.findSchemaErr
	}
	f.findSchemaCalls = append(f.findSchemaCalls, strings.ToLower(table))
	return f.schemasByTable[strings.ToLower(table)], nil
}

func (f *fakeMetadataClient) ResolveTableForIndex(_ context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	f.indexCalls = append(f.indexCalls, index)
	f.indexDialects = append(f.indexDialects, dialect)
	f.indexSchemas = append(f.indexSchemas, schema)
	return f.indexTable, nil
}

func (f *fakeMetadataClient) Close() error {
	f.closed = true
	return nil
}

func (f *fakeMetadataClient) LoadPlanEstimate(context.Context, spec.Statement) (*spec.ImpactEstimate, error) {
	f.planCalls++
	rows := int64(7)
	ratio := 0.07
	return &spec.ImpactEstimate{
		EstimatedRows:  &rows,
		EstimatedRatio: &ratio,
		RiskLevel:      spec.ImpactRiskMedium,
		Confidence:     spec.ImpactConfidenceHigh,
		Source:         spec.ImpactSourcePlan,
		ReasonCodes:    []string{"planner_estimate"},
	}, nil
}

func (f *fakeMetadataClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	f.objectCalls = append(f.objectCalls, req)
	return f.objectSnapshot, nil
}

func TestAuditCommandResolvesMetadataPortByExplicitDialect(t *testing.T) {
	tests := []struct {
		name         string
		dialect      string
		detected     spec.Dialect
		port         string
		wantPort     int
		useFileInput bool
	}{
		{name: "explicit PostgreSQL omitted inline", dialect: "postgresql", detected: spec.DialectPostgreSQL, wantPort: 5432},
		{name: "explicit PostgreSQL port wins in file input", dialect: "postgresql", detected: spec.DialectPostgreSQL, port: "3306", wantPort: 3306, useFileInput: true},
		{name: "explicit MySQL omitted", dialect: "mysql", detected: spec.DialectMySQL, wantPort: 3306},
		{name: "explicit TiDB omitted", dialect: "tidb", detected: spec.DialectTiDB, wantPort: 3306},
		{name: "auto-detected omitted", detected: spec.DialectMySQL, wantPort: 3306},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := newMetadataClient
			client := &fakeMetadataClient{detectDialect: tt.detected}
			newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
				client.options = options
				return client, nil
			}
			t.Cleanup(func() { newMetadataClient = previous })

			args := []string{"audit", "--host", "127.0.0.1", "--user", "root", "--schema", "app"}
			if tt.useFileInput {
				path := t.TempDir() + "/migration.sql"
				if err := os.WriteFile(path, []byte("delete from users"), 0o600); err != nil {
					t.Fatalf("write SQL file: %v", err)
				}
				args = append(args, "--file", path)
			} else {
				args = append(args, "--sql", "delete from users")
			}
			if tt.dialect != "" {
				args = append(args, "--dialect", tt.dialect)
			}
			if tt.port != "" {
				args = append(args, "--port", tt.port)
			}

			Execute(context.Background(), args, strings.NewReader(""), &strings.Builder{}, &strings.Builder{})

			if client.options.Port != tt.wantPort {
				t.Fatalf("expected resolved port %d, got %d", tt.wantPort, client.options.Port)
			}
		})
	}
}

func TestAuditCommandUsesMetadataAwareProviderForTCPConnection(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--port", "3307", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if client.options.Host != "127.0.0.1" || client.options.Port != 3307 || client.options.User != "root" {
		t.Fatalf("unexpected connection options: %#v", client.options)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "app" {
		t.Fatalf("expected instance facts to load once for app schema, got %#v", client.instanceCalls)
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "app" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected users snapshot lookup in app schema, got %#v", client.tableSnapshotCalls)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
}

func TestAuditCommandUsesExplicitSchemaWithoutInference(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectMySQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--schema", "app"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "app" {
		t.Fatalf("expected explicit schema to flow to instance facts, got %#v", client.instanceCalls)
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "app" {
		t.Fatalf("expected explicit schema to flow to snapshots, got %#v", client.tableSnapshotCalls)
	}
}

func TestAuditCommandFailsWhenSchemaInferenceIsAmbiguous(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app", "archive"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "ambiguous") || !strings.Contains(stderr.String(), "--schema") {
		t.Fatalf("expected ambiguous schema guidance, got %q", stderr.String())
	}
}

func TestAuditCommandAllowsCreateTableWhenSchemaCannotBeInferred(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": nil},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id bigint)", "--host", "127.0.0.1", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1 for rule findings, got %d", code)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "" {
		t.Fatalf("expected create-table path to continue with empty schema, got %#v", client.instanceCalls)
	}
}

func TestAuditCommandRejectsDialectMismatchInMetadataMode(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectTiDB}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--dialect", "mysql"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "detected dialect") {
		t.Fatalf("expected dialect mismatch error, got %q", stderr.String())
	}
}

func TestAuditCommandUsesQualifiedSchemaWithoutInference(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app", "archive"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from app.users", "--host", "127.0.0.1", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if len(client.findSchemaCalls) != 0 {
		t.Fatalf("expected qualified schema to skip inference lookup, got %#v", client.findSchemaCalls)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "app" {
		t.Fatalf("expected qualified schema to flow to instance facts, got %#v", client.instanceCalls)
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "app" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected qualified schema to flow to snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
}

func TestAuditCommandPassesMetadataConnectTimeout(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--metadata-connect-timeout", "5s"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if client.options.ConnectTimeout != 5*time.Second {
		t.Fatalf("expected ConnectTimeout=5s, got %v", client.options.ConnectTimeout)
	}
}

func TestAuditCommandRejectsInvalidMetadataConnectTimeout(t *testing.T) {
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--metadata-connect-timeout", "not-a-duration"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid --metadata-connect-timeout") {
		t.Fatalf("expected invalid timeout error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsNegativeMetadataConnectTimeout(t *testing.T) {
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--metadata-connect-timeout", "-5s"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--metadata-connect-timeout must be a non-negative duration") {
		t.Fatalf("expected non-negative duration error, got %q", stderr.String())
	}
}

func TestAuditCommandAcceptsZeroMetadataConnectTimeoutAsDefault(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--metadata-connect-timeout", "0s"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if client.options.ConnectTimeout != 0 {
		t.Fatalf("expected ConnectTimeout=0 (unset), got %v", client.options.ConnectTimeout)
	}
}

func TestAuditCommandIgnoresMetadataConnectTimeoutWithoutConnection(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--dialect", "mysql", "--metadata-connect-timeout", "5s"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "delete") {
		t.Fatalf("expected normal offline audit output, got %q", stdout.String())
	}
}

func TestAuditCommandPassesTLSConfigurationToMySQLMetadataClient(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--tls-mode", "enabled"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}
	if client.options.TLSMode != "enabled" {
		t.Fatalf("expected TLSMode=enabled, got %q", client.options.TLSMode)
	}

	connection := toAuditMetaConnection(client.options, "", false)
	if connection.TLSMode != "enabled" {
		t.Fatalf("expected toAuditMetaConnection TLSMode=enabled, got %q", connection.TLSMode)
	}
}

func TestAuditCommandPassesTLSConfigurationToPostgreSQLMetadataClient(t *testing.T) {
	pool := x509.NewCertPool()
	options := auditConnectionOptions{
		Host:    "127.0.0.1",
		Port:    5432,
		User:    "root",
		TLSMode: "enabled",
		CACert:  pool,
	}

	connection := toAuditMetaConnection(options, spec.DialectPostgreSQL, true)
	if connection.TLSMode != "enabled" {
		t.Fatalf("expected toAuditMetaConnection TLSMode=enabled, got %q", connection.TLSMode)
	}
	if connection.CACert != pool {
		t.Fatalf("expected toAuditMetaConnection CACert to be the same pool instance")
	}
	if connection.Dialect != spec.DialectPostgreSQL {
		t.Fatalf("expected Dialect=postgresql, got %q", connection.Dialect)
	}
}

func TestToAuditMetaConnectionIncludesTLSFields(t *testing.T) {
	pool := x509.NewCertPool()
	options := auditConnectionOptions{
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		TLSMode: "enabled",
		CACert:  pool,
	}
	connection := toAuditMetaConnection(options, "", false)

	if connection.TLSMode != "enabled" {
		t.Fatalf("expected TLSMode=enabled, got %q", connection.TLSMode)
	}
	if connection.CACert != pool {
		t.Fatalf("expected CACert to be the same pool instance")
	}
}

func TestToAuditMetaConnectionOmitsTLSFieldsWhenDisabled(t *testing.T) {
	options := auditConnectionOptions{
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		TLSMode: "disabled",
	}
	connection := toAuditMetaConnection(options, "", false)

	if connection.TLSMode != "disabled" {
		t.Fatalf("expected TLSMode=disabled, got %q", connection.TLSMode)
	}
	if connection.CACert != nil {
		t.Fatalf("expected CACert=nil when disabled, got %v", connection.CACert)
	}
}

func TestOpenMetadataClientPassesTLSToMySQLProvider(t *testing.T) {
	options := auditConnectionOptions{
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		TLSMode: "disabled",
	}
	_, err := openMetadataClient(options)
	if err == nil {
		t.Fatalf("expected connection error without real server")
	}
}

func TestOpenMetadataClientPassesTLSToPostgreSQLProvider(t *testing.T) {
	options := auditConnectionOptions{
		Host:    "127.0.0.1",
		Port:    5432,
		User:    "root",
		Dialect: string(spec.DialectPostgreSQL),
		TLSMode: "disabled",
	}
	_, err := openMetadataClient(options)
	if err == nil {
		t.Fatalf("expected connection error without real server")
	}
}
