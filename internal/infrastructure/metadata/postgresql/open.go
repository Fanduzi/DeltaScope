// Package postgresqlmeta implements metadata-aware audit adapters over PostgreSQL.
// input: connection configs for PostgreSQL metadata reads
// output: pgx stdlib database/sql handles for metadata providers
// pos: infrastructure connection opener for PostgreSQL metadata access
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// DefaultConnectTimeout is the default timeout for initial metadata connection.
const DefaultConnectTimeout = 5 * time.Second

// ConnectionConfig describes one PostgreSQL metadata connection.
type ConnectionConfig struct {
	Host           string
	Port           int
	Socket         string
	Database       string
	User           string
	Password       string
	SSLMode        string
	CACert         *x509.CertPool // pre-parsed CA pool; only used when sslmode requires verification
	ConnectTimeout time.Duration
}

// connectTimeout returns the configured timeout, defaulting to DefaultConnectTimeout when zero.
func (c ConnectionConfig) connectTimeout() time.Duration {
	if c.ConnectTimeout <= 0 {
		return DefaultConnectTimeout
	}
	return c.ConnectTimeout
}

// Address reports the driver address for the connection.
func (c ConnectionConfig) Address() string {
	if strings.TrimSpace(c.Socket) != "" {
		return strings.TrimSpace(c.Socket)
	}
	host := strings.TrimSpace(c.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// DatabaseName reports the configured database or the PostgreSQL default.
func (c ConnectionConfig) DatabaseName() string {
	if strings.TrimSpace(c.Database) == "" {
		return "postgres"
	}
	return strings.TrimSpace(c.Database)
}

func (c ConnectionConfig) sslMode() string {
	if strings.TrimSpace(c.SSLMode) == "" {
		return "disable"
	}
	return strings.TrimSpace(c.SSLMode)
}

// DSN formats the config for the pgx database/sql driver.
func (c ConnectionConfig) DSN() string {
	user := url.User(c.User)
	database := "/" + url.PathEscape(c.DatabaseName())
	query := url.Values{}
	query.Set("sslmode", c.sslMode())
	query.Set("connect_timeout", strconv.Itoa(int(math.Ceil(c.connectTimeout().Seconds()))))

	if strings.TrimSpace(c.Socket) != "" {
		query.Set("host", c.Address())
		return (&url.URL{Scheme: "postgres", User: user, Path: database, RawQuery: query.Encode()}).String()
	}

	return (&url.URL{Scheme: "postgres", User: user, Host: c.Address(), Path: database, RawQuery: query.Encode()}).String()
}

// OpenDBContext connects to a PostgreSQL database for metadata reads,
// respecting the caller's context for cancellation and applying the configured
// connect timeout as a deadline. The caller is responsible for closing the
// returned *sql.DB.
func OpenDBContext(ctx context.Context, config ConnectionConfig) (*sql.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	connConfig, err := pgx.ParseConfig(config.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse metadata connection: %w", err)
	}

	connConfig.Password = config.Password

	if strings.ToLower(strings.TrimSpace(config.SSLMode)) != "disable" && config.CACert != nil {
		host := strings.TrimSpace(config.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		connConfig.TLSConfig = &tls.Config{
			ServerName:         host,
			RootCAs:            config.CACert,
			InsecureSkipVerify: false,
		}
	}

	db := stdlib.OpenDB(*connConfig)

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	timeoutCtx, cancel := context.WithTimeout(ctx, config.connectTimeout())
	defer cancel()
	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping metadata connection: %w", err)
	}

	return db, nil
}

// OpenDB connects to a PostgreSQL database for metadata reads using a
// background context. The caller is responsible for closing the returned *sql.DB.
func OpenDB(config ConnectionConfig) (*sql.DB, error) {
	return OpenDBContext(context.Background(), config)
}
