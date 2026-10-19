// Package main records native and real-provider queries without connection credentials.
// input: a provisioned database/sql driver connector and fixed proof queries
// output: ordered query/exec/provider ledger entries across the real metadata provider
// pos: T07 supplemental proof-only driver instrumentation
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"database/sql/driver"
	"errors"
	"sync"
)

type ledgerEntry struct {
	Op    string `json:"op"`
	Query string `json:"query"`
}

type queryLedger struct {
	mu      sync.Mutex
	entries []ledgerEntry
}

func (l *queryLedger) add(op, query string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, ledgerEntry{op, query})
}

func (l *queryLedger) snapshot() []ledgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ledgerEntry(nil), l.entries...)
}

type ledgerConnector struct {
	inner  driver.Connector
	ledger *queryLedger
}

func (c *ledgerConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &ledgerConn{conn, c.ledger}, nil
}

func (c *ledgerConnector) Driver() driver.Driver { return c.inner.Driver() }

type ledgerConn struct {
	conn   driver.Conn
	ledger *queryLedger
}

func (c *ledgerConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &ledgerStmt{stmt, query, c.ledger}, nil
}

func (c *ledgerConn) Close() error              { return c.conn.Close() }
func (c *ledgerConn) Begin() (driver.Tx, error) { return c.conn.Begin() }

func (c *ledgerConn) Ping(ctx context.Context) error {
	c.ledger.add("ping", "")
	if pinger, ok := c.conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return errors.New("proof driver cannot verify ping")
}

func (c *ledgerConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.ledger.add("query", query)
	if querier, ok := c.conn.(driver.QueryerContext); ok {
		return querier.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *ledgerConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.ledger.add("exec", query)
	if execer, ok := c.conn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

type ledgerStmt struct {
	stmt   driver.Stmt
	query  string
	ledger *queryLedger
}

func (s *ledgerStmt) Close() error  { return s.stmt.Close() }
func (s *ledgerStmt) NumInput() int { return s.stmt.NumInput() }
func (s *ledgerStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.ledger.add("stmt-exec", s.query)
	return s.stmt.Exec(args)
}
func (s *ledgerStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.ledger.add("stmt-query", s.query)
	return s.stmt.Query(args)
}
