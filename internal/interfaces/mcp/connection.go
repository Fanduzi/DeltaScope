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
	"strings"

	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
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

type ConnectionInput = ifaceconn.ConnectionInput

// ResolvedConnection is the normalized metadata-aware connection used by MCP audit flows.
type ResolvedConnection struct {
	Enabled        bool
	Source         MetadataSource
	RefName        string
	RefPath        string
	Host           string
	Port           int
	Socket         string
	User           string
	Schema         string
	Dialect        string
	Password       string
	ConnectTimeout string
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

	switch {
	case strings.TrimSpace(params.ConnectionRef) != "":
		refName := strings.TrimSpace(params.ConnectionRef)
		configPath := strings.TrimSpace(options.ConnectionsPath)
		if configPath == "" {
			configPath = DefaultConnectionsPath
		}
		input, err := loadConnectionConfig(refName, options)
		if err != nil {
			return ResolvedConnection{}, err
		}
		return buildResolvedConnection(input, MetadataSourceConnectionRef, refName, configPath, options)
	default:
		return buildResolvedConnection(*params.Connection, MetadataSourceDirect, "", "", options)
	}
}

func buildResolvedConnection(input ConnectionInput, source MetadataSource, refName string, refPath string, options ResolveConnectionOptions) (ResolvedConnection, error) {
	if err := ifaceconn.ValidateConnectionInput(input); err != nil {
		return ResolvedConnection{}, err
	}

	password, err := ifaceconn.ResolvePassword(input, ifaceconn.ResolveConnectionOptions{
		LookupEnv: options.LookupEnv,
		ReadFile:  options.ReadFile,
	})
	if err != nil {
		return ResolvedConnection{}, err
	}

	return ResolvedConnection{
		Enabled:        true,
		Source:         source,
		RefName:        refName,
		RefPath:        refPath,
		Host:           strings.TrimSpace(input.Host),
		Port:           input.Port,
		Socket:         strings.TrimSpace(input.Socket),
		User:           strings.TrimSpace(input.User),
		Schema:         strings.TrimSpace(input.Schema),
		Dialect:        strings.TrimSpace(input.Dialect),
		Password:       password,
		ConnectTimeout: strings.TrimSpace(input.ConnectTimeout),
	}, nil
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
	expanded, err := ifaceconn.ExpandHome(path)
	if err != nil {
		return nil, err
	}
	data, err := readFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read connections config: %w", err)
	}
	return data, nil
}
