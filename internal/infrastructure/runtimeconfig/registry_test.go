package runtimeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{
					ID:          "primary",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        3306,
					User:        "root",
					PasswordEnv: "TEST_DB_PASS",
					Purposes:    []string{"audit", "query_access"},
					TLSMode:     "disabled",
				},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "app-1", SecretEnv: "TEST_API_SECRET"},
				},
			},
		},
	}
}

func TestValidateAndBuildRegistryValidConfig(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	t.Setenv("TEST_API_SECRET", "apikey123")
	cfg := validConfig()
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	conn, ok := reg.LookupConnection("primary")
	if !ok {
		t.Fatal("expected primary connection")
	}
	if conn.ID != "primary" {
		t.Fatalf("expected primary, got %q", conn.ID)
	}
}

func TestValidateAndBuildRegistryDuplicateConnectionID(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "dup", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
				{ID: "dup", Dialect: "mysql", Host: "127.0.0.1", Port: 3307, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate id error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryInvalidDialect(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "sqlite", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "dialect") {
		t.Fatalf("expected dialect error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryBothHostAndSocket(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, Socket: "/var/run/mysqld.sock", User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected host/socket mutually exclusive error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryNeitherHostNorSocket(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "host or socket is required") {
		t.Fatalf("expected host/socket required error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryInvalidPort(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too_high", 65536},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Metadata: MetadataConfig{
					Connections: []ConnectionConfig{
						{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: tt.port, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
					},
				},
			}
			_, err := ValidateAndBuildRegistry(cfg)
			if err == nil || !strings.Contains(err.Error(), "port") {
				t.Fatalf("expected port error for %d, got: %v", tt.port, err)
			}
		})
	}
}

func TestValidateAndBuildRegistryMissingUser(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "user is required") {
		t.Fatalf("expected user required error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryMissingDatabaseForPostgreSQL(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "postgresql", Host: "127.0.0.1", Port: 5432, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "database is required for postgresql") {
		t.Fatalf("expected database required error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryBothPasswordSources(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", PasswordFile: "/tmp/pw", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected password source error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryNeitherPasswordSource(t *testing.T) {
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "password_env or password_file is required") {
		t.Fatalf("expected password source required error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryInvalidConnectTimeout(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", ConnectTimeout: "not-a-duration", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "connect_timeout") {
		t.Fatalf("expected connect_timeout error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryNegativeConnectTimeout(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", ConnectTimeout: "-5s", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "connect_timeout") {
		t.Fatalf("expected connect_timeout error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryInvalidTLSMode(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", TLSMode: "strict", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "tls_mode") {
		t.Fatalf("expected tls_mode error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryTLSEnabledWithSocket(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Socket: "/var/run/mysqld.sock", User: "root", PasswordEnv: "TEST_DB_PASS", TLSMode: "enabled", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "tls_mode enabled requires host") {
		t.Fatalf("expected tls/socket error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryInvalidPurpose(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"admin"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "purpose") {
		t.Fatalf("expected purpose error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryDuplicatePurposes(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit", "audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate purpose") {
		t.Fatalf("expected duplicate purpose error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryUnknownAllowedAPIKeyID(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	t.Setenv("TEST_API_SECRET", "secret")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}, AllowedAPIKeyIDs: []string{"nonexistent"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown api key") {
		t.Fatalf("expected unknown api key error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryAuthEnabledNoKeys(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{Enabled: true},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "no api keys configured") {
		t.Fatalf("expected no api keys error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryDuplicateAPIKeyID(t *testing.T) {
	t.Setenv("S1", "a")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "dup", SecretEnv: "S1"},
					{ID: "dup", SecretEnv: "S1"},
				},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate api key id error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryAPIKeyBothSecretSources(t *testing.T) {
	t.Setenv("S1", "a")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "k1", SecretEnv: "S1", SecretFile: "/tmp/s"},
				},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected secret source error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryAPIKeyNeitherSecretSource(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "k1"},
				},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "secret_env or secret_file is required") {
		t.Fatalf("expected secret source required error, got: %v", err)
	}
}

func TestValidateAndBuildRegistryMissingEnvVarForSecret(t *testing.T) {
	os.Unsetenv("NONEXISTENT_SECRET_12345")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "k1", SecretEnv: "NONEXISTENT_SECRET_12345"},
				},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if strings.Contains(err.Error(), "NONEXISTENT_SECRET_12345") {
		t.Fatalf("error must not contain env var name, got: %v", err)
	}
}

func TestValidateAndBuildRegistryMissingFileForSecret(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "k1", SecretFile: "/nonexistent/path/to/secret"},
				},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if strings.Contains(err.Error(), "/nonexistent/path/to/secret") {
		t.Fatalf("error must not contain file path, got: %v", err)
	}
}

func TestValidateAndBuildRegistryMissingEnvVarForPassword(t *testing.T) {
	os.Unsetenv("NONEXISTENT_DB_PASS_12345")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "NONEXISTENT_DB_PASS_12345", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if strings.Contains(err.Error(), "NONEXISTENT_DB_PASS_12345") {
		t.Fatalf("error must not contain env var name, got: %v", err)
	}
}

func TestValidateAndBuildRegistryPasswordFromFile(t *testing.T) {
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "dbpass.txt")
	if err := os.WriteFile(pwFile, []byte("  secret-pw  "), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_API_SECRET", "apikey")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordFile: pwFile, Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "k1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	conn, ok := reg.LookupConnection("c1")
	if !ok {
		t.Fatal("expected connection c1")
	}
	if conn.resolvedPassword != "secret-pw" {
		t.Fatalf("expected trimmed password, got %q", conn.resolvedPassword)
	}
}

func TestValidateAndBuildRegistryAllowedAPIKeyIDsIgnoredWhenAuthDisabled(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}, AllowedAPIKeyIDs: []string{"nonexistent"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{Enabled: false},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("expected success when auth disabled, got: %v", err)
	}
}

func TestAuthorizeAuthDisabledKnownConnectionValidPurpose(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{Auth: AuthConfig{Enabled: false}},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Authorize("", "c1", "audit"); err != nil {
		t.Fatalf("expected authorized, got: %v", err)
	}
}

func TestAuthorizeAuthDisabledUnknownConnection(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{Auth: AuthConfig{Enabled: false}},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Authorize("", "unknown", "audit"); !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("expected ErrConnectionNotFound, got: %v", err)
	}
}

func TestAuthorizeAuthDisabledWrongPurpose(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{Auth: AuthConfig{Enabled: false}},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Authorize("", "c1", "query_access"); !errors.Is(err, ErrPurposeNotAllowed) {
		t.Fatalf("expected ErrPurposeNotAllowed, got: %v", err)
	}
}

func TestAuthorizeAuthEnabledAllowedConnection(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	t.Setenv("TEST_API_SECRET", "apikey")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}, AllowedAPIKeyIDs: []string{"app-1"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Authorize("app-1", "c1", "audit"); err != nil {
		t.Fatalf("expected authorized, got: %v", err)
	}
}

func TestAuthorizeAuthEnabledDisallowedConnection(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	t.Setenv("TEST_API_SECRET", "apikey")
	t.Setenv("TEST_API_SECRET_2", "apikey2")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}, AllowedAPIKeyIDs: []string{"app-2"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys: []APIKeyConfig{
					{ID: "app-1", SecretEnv: "TEST_API_SECRET"},
					{ID: "app-2", SecretEnv: "TEST_API_SECRET_2"},
				},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Authorize("app-1", "c1", "audit"); !errors.Is(err, ErrPrincipalNotAllowed) {
		t.Fatalf("expected ErrPrincipalNotAllowed, got: %v", err)
	}
}

func TestResolveAPIKeyValidKey(t *testing.T) {
	t.Setenv("TEST_API_SECRET", "my-secret-key")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	id, ok := reg.ResolveAPIKey("my-secret-key")
	if !ok || id != "app-1" {
		t.Fatalf("expected app-1, got %q, ok=%v", id, ok)
	}
}

func TestResolveAPIKeyInvalidKey(t *testing.T) {
	t.Setenv("TEST_API_SECRET", "my-secret-key")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	id, ok := reg.ResolveAPIKey("wrong-key")
	if ok || id != "" {
		t.Fatalf("expected empty/false, got %q, ok=%v", id, ok)
	}
}

func TestResolveAPIKeyEmptyKey(t *testing.T) {
	t.Setenv("TEST_API_SECRET", "my-secret-key")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	id, ok := reg.ResolveAPIKey("")
	if ok || id != "" {
		t.Fatalf("expected empty/false for empty key, got %q, ok=%v", id, ok)
	}
}

func TestResolveAPIKeyConstantTimeComparison(t *testing.T) {
	t.Setenv("TEST_API_SECRET", "a]very-long-secret-key-that-varies-in-length")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "app-1", SecretEnv: "TEST_API_SECRET"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	id, ok := reg.ResolveAPIKey("a]very-long-secret-key-that-varies-in-length")
	if !ok || id != "app-1" {
		t.Fatalf("expected match, got %q, ok=%v", id, ok)
	}
	id, ok = reg.ResolveAPIKey("a]very-long-secret-key-that-varies-in-lengtX")
	if ok || id != "" {
		t.Fatalf("expected no match, got %q, ok=%v", id, ok)
	}
}

func TestNoLeakErrorMessagesNeverContainSecretValues(t *testing.T) {
	t.Setenv("LEAK_TEST_PASS", "super-secret-password-12345")
	t.Setenv("LEAK_TEST_SECRET", "api-key-secret-value-67890")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "LEAK_TEST_PASS", Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "k1", SecretEnv: "LEAK_TEST_SECRET"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		errMsg := err.Error()
		for _, forbidden := range []string{
			"super-secret-password-12345",
			"api-key-secret-value-67890",
			"LEAK_TEST_PASS",
			"LEAK_TEST_SECRET",
		} {
			if strings.Contains(errMsg, forbidden) {
				t.Errorf("error message must not contain %q, got: %s", forbidden, errMsg)
			}
		}
	}
}

func TestNoLeakRegistryNeverExposesResolvedSecrets(t *testing.T) {
	t.Setenv("SEC_PASS", "db-password")
	t.Setenv("SEC_KEY", "api-key-secret")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "SEC_PASS", Purposes: []string{"audit"}},
			},
		},
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{ID: "k1", SecretEnv: "SEC_KEY"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	regStr := reg.String()
	if strings.Contains(regStr, "db-password") {
		t.Error("registry String() must not expose resolved password")
	}
	if strings.Contains(regStr, "api-key-secret") {
		t.Error("registry String() must not expose resolved API key secret")
	}
}

func TestNoLeakMissingEnvErrorDoesNotContainEnvName(t *testing.T) {
	os.Unsetenv("MY_SECRET_ENV_VAR_NAME")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "MY_SECRET_ENV_VAR_NAME", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "MY_SECRET_ENV_VAR_NAME") {
		t.Errorf("error must not contain env var name: %s", errMsg)
	}
}

func TestNoLeakMissingFileErrorDoesNotContainFilePath(t *testing.T) {
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordFile: "/path/to/my/password.txt", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "/path/to/my/password.txt") {
		t.Errorf("error must not contain file path: %s", errMsg)
	}
}

func TestEmptyConnectionID(t *testing.T) {
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "P", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected id required error, got: %v", err)
	}
}

func TestInvalidConnectionIDFormat(t *testing.T) {
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "bad id!", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "P", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "alphanumeric") {
		t.Fatalf("expected id format error, got: %v", err)
	}
}

func TestEmptyAPIKeyID(t *testing.T) {
	t.Setenv("S1", "a")
	cfg := Config{
		HTTP: HTTPConfig{
			Auth: AuthConfig{
				Enabled: true,
				Keys:    []APIKeyConfig{{SecretEnv: "S1"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected id required error, got: %v", err)
	}
}

func TestLookupConnectionNotFound(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	_, ok := reg.LookupConnection("nonexistent")
	if ok {
		t.Fatal("expected false for nonexistent connection")
	}
}

func TestEmptyConfigBuildsEmptyRegistry(t *testing.T) {
	cfg := Config{}
	reg, err := ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("expected success for empty config, got: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestPortRequiresHost(t *testing.T) {
	t.Setenv("TEST_DB_PASS", "dbpass")
	cfg := Config{
		Metadata: MetadataConfig{
			Connections: []ConnectionConfig{
				{ID: "c1", Dialect: "mysql", Socket: "/var/run/mysqld.sock", Port: 3306, User: "root", PasswordEnv: "TEST_DB_PASS", Purposes: []string{"audit"}},
			},
		},
	}
	_, err := ValidateAndBuildRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "port requires host") {
		t.Fatalf("expected port requires host error, got: %v", err)
	}
}
