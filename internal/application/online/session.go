// Package online provides the shared online session factory for SDK, CLI, and HTTP.
// input: SessionConfig with connection parameters, TLS mode, and expected dialect
// output: pinned *sql.Conn with validated ServerIdentity and CapabilityTarget
// pos: shared session lifecycle for online query access (open, pin, identify, close)
// note: if this file changes, update this header and module README.md.
package online

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// SessionConfig holds the connection parameters for opening an online session.
type SessionConfig struct {
	Host           string
	Port           int
	Socket         string
	User           string
	Password       string
	Database       string
	Schema         string
	Dialect        string
	ConnectTimeout time.Duration
	TLSMode        string          // "disabled" or "enabled"
	CACert         *x509.CertPool  // pre-parsed CA pool; only used when tls_mode=enabled
}

// Session holds the pinned connection and derived identity/metadata.
type Session struct {
	DB       *sql.DB
	Conn     *sql.Conn
	Identity *ServerIdentity
	Target   CapabilityTarget
	Close    func() error // idempotent, closes both Conn and DB
}

// OpenSession opens a database connection, pins it, captures identity, and returns a session.
// On any failure after opening, both DB and Conn are closed.
// The caller must call session.Close() when done.
func OpenSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	if ctx == nil {
		return nil, ErrIdentityUnavailable
	}

	dialect := strings.ToLower(strings.TrimSpace(cfg.Dialect))

	switch dialect {
	case "mysql", "tidb":
		return openMySQLSession(ctx, cfg)
	case "postgresql":
		return openPostgreSQLSession(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// openMySQLSession opens a MySQL/TiDB session.
func openMySQLSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	mysqlCfg, err := buildMySQLConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build config: %w", err)
	}

	connector, err := gomysql.NewConnector(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("create connector: %w", err)
	}
	db := sql.OpenDB(connector)

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("pin connection: %w", err)
	}

	identity, err := IdentifyFromConn(ctx, conn, cfg.Dialect)
	if err != nil {
		conn.Close()
		db.Close()
		return nil, err
	}

	closed := atomic.Bool{}
	session := &Session{
		DB:       db,
		Conn:     conn,
		Identity: identity,
		Target:   DeriveCapabilityTarget(identity),
		Close: func() error {
			if closed.Swap(true) {
				return nil
			}
			var firstErr error
			if cErr := conn.Close(); cErr != nil && firstErr == nil {
				firstErr = cErr
			}
			if dErr := db.Close(); dErr != nil && firstErr == nil {
				firstErr = dErr
			}
			return firstErr
		},
	}
	return session, nil
}

// openPostgreSQLSession opens a PostgreSQL session.
func openPostgreSQLSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	dsn, err := buildPostgreSQLDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dsn: %w", err)
	}

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse connection: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(cfg.TLSMode)) == "enabled" {
		host := strings.TrimSpace(cfg.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		tlsCfg := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}
		if cfg.CACert != nil {
			tlsCfg.RootCAs = cfg.CACert
		}
		connConfig.TLSConfig = tlsCfg
	}

	db := stdlib.OpenDB(*connConfig)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("pin connection: %w", err)
	}

	identity, err := IdentifyFromConn(ctx, conn, "postgresql")
	if err != nil {
		conn.Close()
		db.Close()
		return nil, err
	}

	closed := atomic.Bool{}
	session := &Session{
		DB:       db,
		Conn:     conn,
		Identity: identity,
		Target:   DeriveCapabilityTarget(identity),
		Close: func() error {
			if closed.Swap(true) {
				return nil
			}
			var firstErr error
			if cErr := conn.Close(); cErr != nil && firstErr == nil {
				firstErr = cErr
			}
			if dErr := db.Close(); dErr != nil && firstErr == nil {
				firstErr = dErr
			}
			return firstErr
		},
	}
	return session, nil
}

// IdentifyFromConn queries VERSION() on the pinned connection and parses the identity.
func IdentifyFromConn(ctx context.Context, conn *sql.Conn, expectedDialect string) (*ServerIdentity, error) {
	var rawVersion string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&rawVersion); err != nil {
		return nil, ErrIdentityUnavailable
	}
	identity, err := ParseServerIdentity(rawVersion, expectedDialect)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

// buildMySQLConfig constructs a MySQL driver config from the session config.
func buildMySQLConfig(cfg SessionConfig) (*gomysql.Config, error) {
	mysqlCfg := gomysql.NewConfig()

	if strings.TrimSpace(cfg.Socket) != "" {
		mysqlCfg.Net = "unix"
		mysqlCfg.Addr = strings.TrimSpace(cfg.Socket)
	} else {
		mysqlCfg.Net = "tcp"
		host := strings.TrimSpace(cfg.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		port := cfg.Port
		if port == 0 {
			port = 3306
		}
		mysqlCfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	}

	mysqlCfg.User = cfg.User
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Collation = "utf8mb4_general_ci"

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	mysqlCfg.Timeout = timeout
	mysqlCfg.Params = map[string]string{"interpolateParams": "true"}

	if cfg.Database != "" {
		mysqlCfg.DBName = cfg.Database
	}

	// TLS configuration - set directly on config to avoid global registry.
	tlsMode := strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if tlsMode == "enabled" {
		host := strings.TrimSpace(cfg.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		tlsCfg := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}
		if cfg.CACert != nil {
			tlsCfg.RootCAs = cfg.CACert
		}
		mysqlCfg.TLS = tlsCfg
	}

	return mysqlCfg, nil
}

// buildPostgreSQLDSN constructs a PostgreSQL connection string from the session config.
func buildPostgreSQLDSN(cfg SessionConfig) (string, error) {
	user := url.UserPassword(cfg.User, cfg.Password)
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	database := "postgres"
	if strings.TrimSpace(cfg.Database) != "" {
		database = strings.TrimSpace(cfg.Database)
	}

	query := url.Values{}

	// TLS configuration.
	tlsMode := strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if tlsMode == "enabled" {
		query.Set("sslmode", "verify-full")
	} else {
		query.Set("sslmode", "disable")
	}

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	query.Set("connect_timeout", strconv.Itoa(int(timeout.Seconds())))

	if strings.TrimSpace(cfg.Socket) != "" {
		query.Set("host", strings.TrimSpace(cfg.Socket))
		return (&url.URL{
			Scheme:   "postgres",
			User:     user,
			Path:     "/" + url.PathEscape(database),
			RawQuery: query.Encode(),
		}).String(), nil
	}

	return (&url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + url.PathEscape(database),
		RawQuery: query.Encode(),
	}).String(), nil
}
