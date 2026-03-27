// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: audit tool connection parameters, local connection config files, and password lookup sources
// output: normalized metadata-aware connection settings for MCP audit requests
// pos: MCP connection resolution layer between tool inputs and metadata provider wiring
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// MetadataSource identifies how one metadata-aware audit connection was selected.
type MetadataSource string

const (
	// MetadataSourceNone indicates the request stayed offline.
	MetadataSourceNone MetadataSource = "none"
	// MetadataSourceConnectionRef indicates the request used a named connection reference.
	MetadataSourceConnectionRef MetadataSource = "connection_ref"
	// MetadataSourceDirect indicates the request used direct inline connection settings.
	MetadataSourceDirect MetadataSource = "direct"
)

// DefaultConnectionsPath is the default local path used to resolve MCP connection_ref names.
const DefaultConnectionsPath = "~/.config/deltascope/connections.yaml"

// AuditSQLParams describes the MCP-facing audit_sql request contract.
type AuditSQLParams struct {
	SQL           string           `json:"sql"`
	Dialect       string           `json:"dialect,omitempty"`
	ConfigPath    string           `json:"config_path,omitempty"`
	ConnectionRef string           `json:"connection_ref,omitempty"`
	Connection    *ConnectionInput `json:"connection,omitempty"`
}

// ConnectionInput describes one direct or referenced metadata-aware connection.
type ConnectionInput struct {
	Host         string `json:"host,omitempty" yaml:"host"`
	Port         int    `json:"port,omitempty" yaml:"port"`
	Socket       string `json:"socket,omitempty" yaml:"socket"`
	User         string `json:"user,omitempty" yaml:"user"`
	Schema       string `json:"schema,omitempty" yaml:"schema"`
	Dialect      string `json:"dialect,omitempty" yaml:"dialect"`
	Password     string `json:"password,omitempty" yaml:"password"`
	PasswordEnv  string `json:"password_env,omitempty" yaml:"password_env"`
	PasswordFile string `json:"password_file,omitempty" yaml:"password_file"`
}

// ResolvedConnection is the normalized metadata-aware connection used by MCP audit flows.
type ResolvedConnection struct {
	Enabled  bool
	Source   MetadataSource
	RefName  string
	RefPath  string
	Host     string
	Port     int
	Socket   string
	User     string
	Schema   string
	Dialect  string
	Password string
}

// ResolveConnectionOptions configures connection resolution dependencies.
type ResolveConnectionOptions struct {
	ConnectionsPath string
	LookupEnv       func(string) (string, bool)
	ReadFile        func(string) ([]byte, error)
}

type connectionConfigFile struct {
	Connections map[string]ConnectionInput `yaml:"connections"`
}

// ResolveAuditConnection validates and normalizes the MCP audit_sql connection inputs.
func ResolveAuditConnection(params AuditSQLParams, options ResolveConnectionOptions) (ResolvedConnection, error) {
	if strings.TrimSpace(params.ConnectionRef) != "" && params.Connection != nil {
		return ResolvedConnection{}, errors.New("connection_ref and connection are mutually exclusive")
	}
	if strings.TrimSpace(params.ConnectionRef) == "" && params.Connection == nil {
		return ResolvedConnection{Source: MetadataSourceNone}, nil
	}

	var input ConnectionInput
	var source MetadataSource
	switch {
	case strings.TrimSpace(params.ConnectionRef) != "":
		refName := strings.TrimSpace(params.ConnectionRef)
		configPath := strings.TrimSpace(options.ConnectionsPath)
		if configPath == "" {
			configPath = DefaultConnectionsPath
		}
		config, err := loadConnectionConfig(refName, options)
		if err != nil {
			return ResolvedConnection{}, err
		}
		input = config
		source = MetadataSourceConnectionRef
		return buildResolvedConnection(input, source, refName, configPath, options)
	default:
		input = *params.Connection
		source = MetadataSourceDirect
	}
	return buildResolvedConnection(input, source, "", "", options)
}

func buildResolvedConnection(input ConnectionInput, source MetadataSource, refName string, refPath string, options ResolveConnectionOptions) (ResolvedConnection, error) {
	if err := validateConnectionInput(input); err != nil {
		return ResolvedConnection{}, err
	}

	password, err := resolvePassword(input, options)
	if err != nil {
		return ResolvedConnection{}, err
	}

	return ResolvedConnection{
		Enabled:  true,
		Source:   source,
		RefName:  refName,
		RefPath:  refPath,
		Host:     strings.TrimSpace(input.Host),
		Port:     input.Port,
		Socket:   strings.TrimSpace(input.Socket),
		User:     strings.TrimSpace(input.User),
		Schema:   strings.TrimSpace(input.Schema),
		Dialect:  strings.TrimSpace(input.Dialect),
		Password: password,
	}, nil
}

func validateConnectionInput(input ConnectionInput) error {
	if strings.TrimSpace(input.Host) == "" &&
		input.Port == 0 &&
		strings.TrimSpace(input.Socket) == "" &&
		strings.TrimSpace(input.User) == "" &&
		strings.TrimSpace(input.Schema) == "" &&
		strings.TrimSpace(input.Dialect) == "" {
		return errors.New("connection must include at least one non-password field")
	}
	if strings.TrimSpace(input.Socket) == "" && (strings.TrimSpace(input.Host) == "" || strings.TrimSpace(input.User) == "") {
		return errors.New("connection must include host/user, socket/user, or connection_ref")
	}
	if strings.TrimSpace(input.Socket) != "" && strings.TrimSpace(input.User) == "" {
		return errors.New("connection must include host/user, socket/user, or connection_ref")
	}
	if strings.TrimSpace(input.Socket) != "" && (strings.TrimSpace(input.Host) != "" || input.Port != 0) {
		return errors.New("connection socket cannot be combined with host/port TCP options")
	}
	return nil
}

func loadConnectionConfig(name string, options ResolveConnectionOptions) (ConnectionInput, error) {
	path := strings.TrimSpace(options.ConnectionsPath)
	if path == "" {
		path = DefaultConnectionsPath
	}
	data, err := readConnectionFile(path, options)
	if err != nil {
		return ConnectionInput{}, err
	}

	var file connectionConfigFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return ConnectionInput{}, fmt.Errorf("connection_ref config: %w", err)
	}

	connection, ok := file.Connections[strings.TrimSpace(name)]
	if !ok {
		return ConnectionInput{}, fmt.Errorf("unknown connection_ref %q", name)
	}
	return connection, nil
}

func readConnectionFile(path string, options ResolveConnectionOptions) ([]byte, error) {
	readFile := options.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	expanded, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	data, err := readFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read connections config: %w", err)
	}
	return data, nil
}

func resolvePassword(input ConnectionInput, options ResolveConnectionOptions) (string, error) {
	count := 0
	if strings.TrimSpace(input.Password) != "" {
		count++
	}
	if strings.TrimSpace(input.PasswordEnv) != "" {
		count++
	}
	if strings.TrimSpace(input.PasswordFile) != "" {
		count++
	}
	if count > 1 {
		return "", errors.New("connection password sources are mutually exclusive")
	}

	switch {
	case strings.TrimSpace(input.Password) != "":
		return input.Password, nil
	case strings.TrimSpace(input.PasswordEnv) != "":
		lookup := options.LookupEnv
		if lookup == nil {
			lookup = os.LookupEnv
		}
		value, ok := lookup(strings.TrimSpace(input.PasswordEnv))
		if !ok {
			return "", fmt.Errorf("password env %q is not set", strings.TrimSpace(input.PasswordEnv))
		}
		return value, nil
	case strings.TrimSpace(input.PasswordFile) != "":
		data, err := readPasswordFile(input.PasswordFile, options)
		if err != nil {
			return "", fmt.Errorf("read password file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", nil
	}
}

func readPasswordFile(path string, options ResolveConnectionOptions) ([]byte, error) {
	readFile := options.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	expanded, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	data, err := readFile(expanded)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
