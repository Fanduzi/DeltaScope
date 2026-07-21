// Package runtimeconfig loads non-policy process settings for server and MCP operations.
// input: optional YAML runtime config path
// output: structured runtime Config for logging and metadata settings
// pos: infrastructure adapter for runtime configuration loading
// note: if this file changes, update this header and module README.md.
package runtimeconfig

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config holds runtime (non-policy) process settings.
type Config struct {
	Logging  LoggingConfig  `yaml:"logging"`
	Metadata MetadataConfig `yaml:"metadata"`
	HTTP     HTTPConfig     `yaml:"http"`
}

// LoggingConfig controls structured logging behavior.
type LoggingConfig struct {
	Level  string       `yaml:"level"`
	Format string       `yaml:"format"`
	Output string       `yaml:"output"`
	File   string       `yaml:"file"`
	Rotate RotateConfig `yaml:"rotate"`
}

// RotateConfig controls log file rotation settings.
// Pointer fields distinguish "unset" from "explicit zero".
type RotateConfig struct {
	Enabled    *bool `yaml:"enabled"`
	MaxSizeMB  *int  `yaml:"max_size_mb"`
	MaxBackups *int  `yaml:"max_backups"`
	MaxAgeDays *int  `yaml:"max_age_days"`
	Compress   *bool `yaml:"compress"`
}

// MetadataConfig controls metadata connection behavior.
type MetadataConfig struct {
	ConnectTimeout string             `yaml:"connect_timeout"`
	Connections    []ConnectionConfig `yaml:"connections"`
}

// ConnectionConfig describes one named metadata connection in the runtime config.
type ConnectionConfig struct {
	ID               string   `yaml:"id"`
	Dialect          string   `yaml:"dialect"` // "mysql", "tidb", "postgresql"
	Host             string   `yaml:"host"`
	Port             int      `yaml:"port"`
	Socket           string   `yaml:"socket"`
	Database         string   `yaml:"database"`
	Schema           string   `yaml:"schema"`
	User             string   `yaml:"user"`
	PasswordEnv      string   `yaml:"password_env"`
	PasswordFile     string   `yaml:"password_file"`
	ConnectTimeout   string   `yaml:"connect_timeout"`
	TLSMode          string   `yaml:"tls_mode"`    // "disabled" (default), "enabled"
	TLSCAFile        string   `yaml:"tls_ca_file"` // PEM file for private CA; only used when tls_mode=enabled
	Purposes         []string `yaml:"purposes"`    // "audit", "query_access"
	AllowedAPIKeyIDs []string `yaml:"allowed_api_key_ids"`

	// resolvedPassword is set at startup and never serialized.
	resolvedPassword string `yaml:"-"`
	// resolvedCACert is the parsed CA certificate pool from tls_ca_file.
	resolvedCACert *x509.CertPool `yaml:"-"`
}

// ResolvedPassword returns the password resolved from the configured secret source at startup.
// It never appears in serialized output.
func (c ConnectionConfig) ResolvedPassword() string {
	return c.resolvedPassword
}

// ResolvedCACert returns the parsed CA certificate pool from tls_ca_file at startup.
// Returns nil when no CA file is configured or tls_mode is disabled.
// It never appears in serialized output.
func (c ConnectionConfig) ResolvedCACert() *x509.CertPool {
	return c.resolvedCACert
}

// HTTPConfig controls HTTP service settings.
type HTTPConfig struct {
	Auth AuthConfig `yaml:"auth"`
}

// AuthConfig controls API-key authentication.
type AuthConfig struct {
	Enabled bool           `yaml:"enabled"`
	Keys    []APIKeyConfig `yaml:"keys"`
}

// APIKeyConfig describes one API key identity.
type APIKeyConfig struct {
	ID         string `yaml:"id"`
	SecretEnv  string `yaml:"secret_env"`
	SecretFile string `yaml:"secret_file"`

	// resolvedSecret is set at startup and never serialized.
	resolvedSecret string `yaml:"-"`
}

// Load reads a runtime config YAML file. An empty path returns a zero Config.
// Unknown YAML fields are rejected. Read and parse errors include the path.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read runtime config %q: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse runtime config %q: %w", path, os.ErrInvalid)
	}

	return cfg, nil
}

// ParseConnectTimeout parses a duration string for metadata connect timeout.
// Empty string and zero duration return (0, false, nil) indicating "use default".
// Positive durations return (duration, true, nil).
// Negative or invalid durations return an error.
func ParseConnectTimeout(raw string) (time.Duration, bool, error) {
	if raw == "" {
		return 0, false, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, fmt.Errorf("metadata connect_timeout %q: %w", raw, err)
	}

	if d < 0 {
		return 0, false, fmt.Errorf("metadata connect_timeout %q: must not be negative", raw)
	}

	if d == 0 {
		return 0, false, nil
	}

	return d, true, nil
}
