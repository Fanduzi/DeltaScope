// Package postgresqlmeta implements metadata-aware audit adapters over PostgreSQL.
// input: connection configs for PostgreSQL metadata reads
// output: pgx stdlib database/sql handles for metadata providers
// pos: infrastructure connection opener for PostgreSQL metadata access
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// ConnectionConfig describes one PostgreSQL metadata connection.
type ConnectionConfig struct {
	Host     string
	Port     int
	Socket   string
	Database string
	User     string
	Password string
	SSLMode  string
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
	user := url.UserPassword(c.User, c.Password)
	database := "/" + url.PathEscape(c.DatabaseName())
	query := url.Values{}
	query.Set("sslmode", c.sslMode())

	if strings.TrimSpace(c.Socket) != "" {
		query.Set("host", c.Address())
		return (&url.URL{Scheme: "postgres", User: user, Path: database, RawQuery: query.Encode()}).String()
	}

	return (&url.URL{Scheme: "postgres", User: user, Host: c.Address(), Path: database, RawQuery: query.Encode()}).String()
}

// OpenDB connects to a PostgreSQL database for metadata reads.
// The caller is responsible for closing the returned *sql.DB.
func OpenDB(config ConnectionConfig) (*sql.DB, error) {
	connConfig, err := pgx.ParseConfig(config.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse metadata connection: %w", err)
	}
	db := stdlib.OpenDB(*connConfig)

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping metadata connection: %w", err)
	}

	return db, nil
}
