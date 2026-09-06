// Package connresolve turns caller connection input into a ready-to-open configuration.
// input: host, socket, user, password sources, TLS files, timeout, dialect, and catalog hints
// output: a Config ready for auditmeta.Prepare or online.OpenSession, or an Error with FailureClass
// pos: Transport Connection Resolution entry that stops before open
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CodeSocketTCPConflict    = "socket_tcp_conflict"
	CodeTLSMode              = "tls_mode"
	CodeTLSCARequiresEnabled = "tls_ca_requires_enabled"
	CodeTLSRequiresHost      = "tls_requires_host"
	CodeTLSRequiresUser      = "tls_requires_user"
	CodeTLSNoSocket          = "tls_no_socket"
	CodeTLSCAPath            = "tls_ca_path"
	CodeTLSCARead            = "tls_ca_read"
	CodeTLSCAPEM             = "tls_ca_pem"
	CodeTimeout              = "timeout"
	CodePasswordSource       = "password_source"
	CodePasswordLookup       = "password_lookup"
	CodeCatalogConflict      = "catalog_conflict"
)

// Request is caller connection input before open.
type Request struct {
	Host            string
	Port            int
	PortExplicit    bool
	User            string
	Socket          string
	Database        string
	Schema          string
	RequestedSchema string
	Dialect         string
	DialectExplicit bool
	Password        string
	PasswordEnv     string
	PasswordFile    string
	ConnectTimeout  string
	TLSMode         string
	TLSCAFile       string
}

// Options substitutes env and file access in tests.
type Options struct {
	LookupEnv func(string) (string, bool)
	ReadFile  func(string) ([]byte, error)
}

// Config is ready-to-open connection configuration. It does not hold a live handle.
type Config struct {
	Host           string
	Port           int
	User           string
	Socket         string
	Database       string
	Schema         string
	Qualifier      string
	Dialect        string
	Password       string
	ConnectTimeout time.Duration
	TLSMode        string
	CACert         *x509.CertPool
}

// Error is a Transport Connection Resolution failure with a Connection Failure Class.
type Error struct {
	Class FailureClass
	Code  string
	msg   string
	err   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.msg
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Resolve validates input, resolves password and TLS, applies the PostgreSQL
// omitted-port default, and binds MySQL/TiDB catalog aliases. It does not open.
func Resolve(req Request, opts Options) (Config, error) {
	cfg := Config{
		Host:     strings.TrimSpace(req.Host),
		Port:     req.Port,
		User:     strings.TrimSpace(req.User),
		Socket:   strings.TrimSpace(req.Socket),
		Database: strings.TrimSpace(req.Database),
		Schema:   strings.TrimSpace(req.Schema),
		Dialect:  strings.TrimSpace(req.Dialect),
	}
	if req.DialectExplicit {
		cfg.Port = ApplyPostgreSQLDefaultPort(cfg.Dialect, req.Port, req.PortExplicit)
	}

	if cfg.Socket != "" && (cfg.Host != "" || req.PortExplicit) {
		return Config{}, newResolveError(ClassValidation, CodeSocketTCPConflict, "connection socket cannot be combined with host/port TCP options", nil)
	}

	timeout, err := parseTimeout(req.ConnectTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ConnectTimeout = timeout

	tlsMode, cert, err := configureTLS(req, cfg, opts)
	if err != nil {
		return Config{}, err
	}
	cfg.TLSMode = tlsMode
	cfg.CACert = cert

	password, err := resolvePassword(req, opts)
	if err != nil {
		return Config{}, err
	}
	cfg.Password = password

	catalog, qualifier, err := BindMySQLTiDBCatalog(cfg.Dialect, cfg.Database, cfg.Schema, strings.TrimSpace(req.RequestedSchema))
	if err != nil {
		return Config{}, newResolveError(ClassValidation, CodeCatalogConflict, ErrCatalogConflict.Error(), err)
	}
	if mysqlCompatibleDialect(cfg.Dialect) {
		cfg.Database = catalog
		cfg.Schema = catalog
		cfg.Qualifier = qualifier
	} else {
		cfg.Qualifier = strings.TrimSpace(req.RequestedSchema)
	}
	return cfg, nil
}

func mysqlCompatibleDialect(dialect string) bool {
	d := strings.ToLower(strings.TrimSpace(dialect))
	return d == "mysql" || d == "tidb"
}

func parseTimeout(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, newResolveError(ClassValidation, CodeTimeout, "invalid connect timeout", err)
	}
	if d < 0 {
		return 0, newResolveError(ClassValidation, CodeTimeout, "connect timeout must be a non-negative duration such as 5s", nil)
	}
	return d, nil
}

func configureTLS(req Request, cfg Config, opts Options) (string, *x509.CertPool, error) {
	tlsMode := strings.TrimSpace(req.TLSMode)
	if tlsMode == "" {
		tlsMode = "disabled"
	}
	if tlsMode != "disabled" && tlsMode != "enabled" {
		return "", nil, newResolveError(ClassValidation, CodeTLSMode, "tls mode must be disabled or enabled", nil)
	}
	caFile := strings.TrimSpace(req.TLSCAFile)
	if caFile != "" && tlsMode != "enabled" {
		return "", nil, newResolveError(ClassValidation, CodeTLSCARequiresEnabled, "tls ca file requires tls mode enabled", nil)
	}
	if tlsMode == "enabled" {
		if cfg.Host == "" {
			return "", nil, newResolveError(ClassValidation, CodeTLSRequiresHost, "tls mode enabled requires host", nil)
		}
		if cfg.User == "" {
			return "", nil, newResolveError(ClassValidation, CodeTLSRequiresUser, "tls mode enabled requires user", nil)
		}
		if cfg.Socket != "" {
			return "", nil, newResolveError(ClassValidation, CodeTLSNoSocket, "tls mode enabled cannot be used with socket", nil)
		}
	}
	if caFile == "" {
		return tlsMode, nil, nil
	}
	expanded, err := expandHome(caFile)
	if err != nil {
		return "", nil, newResolveError(ClassValidation, CodeTLSCAPath, "invalid TLS CA file path", err)
	}
	readFile := opts.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	pemBytes, err := readFile(expanded)
	if err != nil {
		return "", nil, newResolveError(ClassValidation, CodeTLSCARead, "cannot read TLS CA file", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return "", nil, newResolveError(ClassValidation, CodeTLSCAPEM, "invalid TLS CA certificate", nil)
	}
	return tlsMode, pool, nil
}

func resolvePassword(req Request, opts Options) (string, error) {
	count := 0
	if strings.TrimSpace(req.Password) != "" {
		count++
	}
	if strings.TrimSpace(req.PasswordEnv) != "" {
		count++
	}
	if strings.TrimSpace(req.PasswordFile) != "" {
		count++
	}
	if count > 1 {
		return "", newResolveError(ClassValidation, CodePasswordSource, "password sources are mutually exclusive", nil)
	}
	switch {
	case strings.TrimSpace(req.Password) != "":
		return req.Password, nil
	case strings.TrimSpace(req.PasswordEnv) != "":
		lookup := opts.LookupEnv
		if lookup == nil {
			lookup = os.LookupEnv
		}
		key := strings.TrimSpace(req.PasswordEnv)
		value, ok := lookup(key)
		if !ok {
			return "", newResolveError(ClassValidation, CodePasswordLookup, fmt.Sprintf("password env %q is not set", key), nil)
		}
		return value, nil
	case strings.TrimSpace(req.PasswordFile) != "":
		readFile := opts.ReadFile
		if readFile == nil {
			readFile = os.ReadFile
		}
		expanded, err := expandHome(req.PasswordFile)
		if err != nil {
			return "", newResolveError(ClassValidation, CodePasswordLookup, "read password file", err)
		}
		data, err := readFile(expanded)
		if err != nil {
			return "", newResolveError(ClassValidation, CodePasswordLookup, "read password file", err)
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", nil
	}
}

func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func newResolveError(class FailureClass, code, message string, err error) error {
	return &Error{Class: class, Code: code, msg: message, err: err}
}
