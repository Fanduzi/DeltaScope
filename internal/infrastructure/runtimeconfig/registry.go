// Package runtimeconfig loads non-policy process settings for server and MCP operations.
// input: optional YAML runtime config path
// output: structured runtime Config for logging and metadata settings
// pos: infrastructure adapter for runtime configuration loading
// note: if this file changes, update this header and module README.md.
package runtimeconfig

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	ErrConnectionNotFound  = errors.New("connection not found")
	ErrPurposeNotAllowed   = errors.New("purpose not allowed for this connection")
	ErrPrincipalNotAllowed = errors.New("principal not allowed for this connection")
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Registry struct {
	connections map[string]ConnectionConfig
	apiKeys     map[string]APIKeyConfig
	authEnabled bool
}

func ValidateAndBuildRegistry(cfg Config) (*Registry, error) {
	if err := validateConnections(cfg); err != nil {
		return nil, err
	}
	if err := validateAPIKeys(cfg); err != nil {
		return nil, err
	}
	if err := resolveSecrets(cfg); err != nil {
		return nil, err
	}

	connMap := make(map[string]ConnectionConfig, len(cfg.Metadata.Connections))
	for _, c := range cfg.Metadata.Connections {
		connMap[c.ID] = c
	}
	keyMap := make(map[string]APIKeyConfig, len(cfg.HTTP.Auth.Keys))
	for _, k := range cfg.HTTP.Auth.Keys {
		keyMap[k.ID] = k
	}

	return &Registry{
		connections: connMap,
		apiKeys:     keyMap,
		authEnabled: cfg.HTTP.Auth.Enabled,
	}, nil
}

func (r *Registry) Authorize(principalID, connectionID, purpose string) error {
	conn, ok := r.connections[connectionID]
	if !ok {
		return ErrConnectionNotFound
	}
	if !containsString(conn.Purposes, purpose) {
		return ErrPurposeNotAllowed
	}
	if r.authEnabled && !containsString(conn.AllowedAPIKeyIDs, principalID) {
		return ErrPrincipalNotAllowed
	}
	return nil
}

func (r *Registry) LookupConnection(connectionID string) (ConnectionConfig, bool) {
	c, ok := r.connections[connectionID]
	return c, ok
}

func (r *Registry) ResolveAPIKey(rawKey string) (string, bool) {
	if rawKey == "" {
		return "", false
	}
	for _, k := range r.apiKeys {
		if subtle.ConstantTimeCompare([]byte(rawKey), []byte(k.resolvedSecret)) == 1 {
			return k.ID, true
		}
	}
	return "", false
}

func (r *Registry) String() string {
	connIDs := make([]string, 0, len(r.connections))
	for id := range r.connections {
		connIDs = append(connIDs, id)
	}
	keyIDs := make([]string, 0, len(r.apiKeys))
	for id := range r.apiKeys {
		keyIDs = append(keyIDs, id)
	}
	return fmt.Sprintf("Registry{connections: %v, apiKeys: %v, authEnabled: %v}", connIDs, keyIDs, r.authEnabled)
}

func validateConnections(cfg Config) error {
	seen := make(map[string]bool)
	for _, c := range cfg.Metadata.Connections {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			return fmt.Errorf("connection: id is required")
		}
		if !idPattern.MatchString(id) {
			return fmt.Errorf("connection %q: id must be alphanumeric, hyphens, or underscores only", id)
		}
		if seen[id] {
			return fmt.Errorf("connection %q: duplicate id", id)
		}
		seen[id] = true

		dialect := strings.ToLower(strings.TrimSpace(c.Dialect))
		if dialect != "mysql" && dialect != "tidb" && dialect != "postgresql" {
			return fmt.Errorf("connection %q: dialect must be mysql, tidb, or postgresql", id)
		}

		hasHost := strings.TrimSpace(c.Host) != ""
		hasSocket := strings.TrimSpace(c.Socket) != ""
		if hasHost && hasSocket {
			return fmt.Errorf("connection %q: host and socket are mutually exclusive", id)
		}
		if !hasHost && !hasSocket {
			return fmt.Errorf("connection %q: host or socket is required", id)
		}

		if hasHost && (c.Port < 1 || c.Port > 65535) {
			return fmt.Errorf("connection %q: port must be 1-65535", id)
		}
		if !hasHost && c.Port != 0 {
			return fmt.Errorf("connection %q: port requires host", id)
		}

		if strings.TrimSpace(c.User) == "" {
			return fmt.Errorf("connection %q: user is required", id)
		}

		if dialect == "postgresql" && strings.TrimSpace(c.Database) == "" {
			return fmt.Errorf("connection %q: database is required for postgresql", id)
		}

		hasPasswordEnv := strings.TrimSpace(c.PasswordEnv) != ""
		hasPasswordFile := strings.TrimSpace(c.PasswordFile) != ""
		if hasPasswordEnv && hasPasswordFile {
			return fmt.Errorf("connection %q: password_env and password_file are mutually exclusive", id)
		}
		if !hasPasswordEnv && !hasPasswordFile {
			return fmt.Errorf("connection %q: password_env or password_file is required", id)
		}

		if strings.TrimSpace(c.ConnectTimeout) != "" {
			d, err := time.ParseDuration(strings.TrimSpace(c.ConnectTimeout))
			if err != nil {
				return fmt.Errorf("connection %q: connect_timeout must be a valid duration", id)
			}
			if d <= 0 {
				return fmt.Errorf("connection %q: connect_timeout must be positive", id)
			}
		}

		tlsMode := strings.ToLower(strings.TrimSpace(c.TLSMode))
		if tlsMode == "" {
			tlsMode = "disabled"
		}
		if tlsMode != "disabled" && tlsMode != "enabled" {
			return fmt.Errorf("connection %q: tls_mode must be disabled or enabled", id)
		}
		if tlsMode == "enabled" && !hasHost {
			return fmt.Errorf("connection %q: tls_mode enabled requires host (not socket)", id)
		}

		seenPurpose := make(map[string]bool)
		for _, p := range c.Purposes {
			if p != "audit" && p != "query_access" {
				return fmt.Errorf("connection %q: purpose must be audit or query_access", id)
			}
			if seenPurpose[p] {
				return fmt.Errorf("connection %q: duplicate purpose %q", id, p)
			}
			seenPurpose[p] = true
		}
	}
	return nil
}

func validateAPIKeys(cfg Config) error {
	seen := make(map[string]bool)
	for _, k := range cfg.HTTP.Auth.Keys {
		id := strings.TrimSpace(k.ID)
		if id == "" {
			return fmt.Errorf("api key: id is required")
		}
		if seen[id] {
			return fmt.Errorf("api key %q: duplicate id", id)
		}
		seen[id] = true

		hasSecretEnv := strings.TrimSpace(k.SecretEnv) != ""
		hasSecretFile := strings.TrimSpace(k.SecretFile) != ""
		if hasSecretEnv && hasSecretFile {
			return fmt.Errorf("api key %q: secret_env and secret_file are mutually exclusive", id)
		}
		if !hasSecretEnv && !hasSecretFile {
			return fmt.Errorf("api key %q: secret_env or secret_file is required", id)
		}
	}

	if cfg.HTTP.Auth.Enabled && len(cfg.HTTP.Auth.Keys) == 0 {
		return fmt.Errorf("auth enabled but no api keys configured")
	}

	keyIDs := make(map[string]bool)
	for _, k := range cfg.HTTP.Auth.Keys {
		keyIDs[k.ID] = true
	}
	for _, c := range cfg.Metadata.Connections {
		for _, ref := range c.AllowedAPIKeyIDs {
			if cfg.HTTP.Auth.Enabled && !keyIDs[ref] {
				return fmt.Errorf("connection %q: allowed_api_key_id %q references unknown api key", c.ID, ref)
			}
		}
	}
	return nil
}

func resolveSecrets(cfg Config) error {
	for i := range cfg.Metadata.Connections {
		c := &cfg.Metadata.Connections[i]
		pw, err := resolveSecretSource(strings.TrimSpace(c.PasswordEnv), strings.TrimSpace(c.PasswordFile))
		if err != nil {
			return fmt.Errorf("connection %q: %w", c.ID, err)
		}
		c.resolvedPassword = pw
	}
	for i := range cfg.HTTP.Auth.Keys {
		k := &cfg.HTTP.Auth.Keys[i]
		secret, err := resolveSecretSource(strings.TrimSpace(k.SecretEnv), strings.TrimSpace(k.SecretFile))
		if err != nil {
			return fmt.Errorf("api key %q: %w", k.ID, err)
		}
		k.resolvedSecret = secret
	}
	return nil
}

func resolveSecretSource(envKey, filePath string) (string, error) {
	if envKey != "" {
		val, ok := os.LookupEnv(envKey)
		if !ok || val == "" {
			return "", fmt.Errorf("secret source: environment variable is not set")
		}
		return val, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("secret source: unable to read file")
	}
	return strings.TrimSpace(string(data)), nil
}

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
