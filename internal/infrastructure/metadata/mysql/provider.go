// Package mysqlmeta implements metadata-aware audit adapters over the MySQL protocol.
// input: sql.DB access plus connection configs, version/schema/table lookup requests, and MySQL/TiDB metadata queries
// output: normalized instance facts with explicit known bits (innodb_page_size, 5.7 large-prefix, TiDB max-index-length via all-rows SHOW CONFIG verification — read failures propagate as provider errors, only absent/ambiguous values stay unknown), dialect detection, schema discovery, and table snapshots with preserved index cardinality and stored-representation default facts (COLUMN_DEFAULT IS NOT NULL → HasDefault with verbatim DefaultValue bytes; NULL rows and literal "null" text never set DefaultIsNull on this path) for application-level audit enrichment
// pos: infrastructure metadata adapter between database/sql and domain metadata specs
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// DefaultConnectTimeout is the default timeout for initial metadata connection.
const DefaultConnectTimeout = 5 * time.Second

// ConnectionConfig describes one MySQL-compatible metadata connection.
type ConnectionConfig struct {
	Host           string
	Port           int
	Socket         string
	User           string
	Password       string
	Database       string
	ConnectTimeout time.Duration
	TLSMode        string         // "disabled" (default) or "enabled"
	CACert         *x509.CertPool // pre-parsed CA pool; only used when tls_mode=enabled
}

// connectTimeout returns the configured timeout, defaulting to DefaultConnectTimeout when zero.
func (c ConnectionConfig) connectTimeout() time.Duration {
	if c.ConnectTimeout <= 0 {
		return DefaultConnectTimeout
	}
	return c.ConnectTimeout
}

// Network reports the driver network name for the connection.
func (c ConnectionConfig) Network() string {
	if strings.TrimSpace(c.Socket) != "" {
		return "unix"
	}
	return "tcp"
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
		port = 3306
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// mysqlConfig builds a go-sql-driver/mysql Config from this connection config.
// When TLS is enabled, the TLS config is set directly on the driver config
// rather than using RegisterTLSConfig, avoiding global state accumulation.
func (c ConnectionConfig) mysqlConfig() *gomysql.Config {
	cfg := gomysql.NewConfig()
	cfg.Net = c.Network()
	cfg.Addr = c.Address()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Collation = "utf8mb4_general_ci"
	cfg.Timeout = c.connectTimeout()
	cfg.InterpolateParams = true

	if strings.TrimSpace(c.Database) != "" {
		cfg.DBName = strings.TrimSpace(c.Database)
	}

	tlsMode := strings.ToLower(strings.TrimSpace(c.TLSMode))
	if tlsMode == "enabled" {
		host := strings.TrimSpace(c.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		tlsCfg := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}
		if c.CACert != nil {
			tlsCfg.RootCAs = c.CACert
		}
		cfg.TLS = tlsCfg
	}

	return cfg
}

func (c ConnectionConfig) DSN() string {
	return c.mysqlConfig().FormatDSN()
}

// OpenDBContext connects to a MySQL-compatible database for metadata reads,
// respecting the caller's context for cancellation and applying the configured
// connect timeout as a deadline. The caller is responsible for closing the
// returned *sql.DB.
func OpenDBContext(ctx context.Context, config ConnectionConfig) (*sql.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	connector, err := gomysql.NewConnector(config.mysqlConfig())
	if err != nil {
		return nil, fmt.Errorf("create metadata connector: %w", err)
	}
	db := sql.OpenDB(connector)

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

// OpenDB connects to a MySQL-compatible database for metadata reads using a
// background context. The caller is responsible for closing the returned *sql.DB.
func OpenDB(config ConnectionConfig) (*sql.DB, error) {
	return OpenDBContext(context.Background(), config)
}

// Provider loads metadata facts through a MySQL-compatible SQL connection.
type Provider struct {
	db *sql.DB
}

// NewProvider builds a metadata provider on top of an existing SQL handle.
func NewProvider(db *sql.DB) *Provider {
	return &Provider{db: db}
}

// DetectDialect reads server version information and classifies the SQL dialect.
func (p *Provider) DetectDialect(ctx context.Context) (spec.Dialect, error) {
	var version string
	if err := p.db.QueryRowContext(ctx, `select version()`).Scan(&version); err != nil {
		return "", fmt.Errorf("query server version: %w", err)
	}
	return detectDialectFromVersion(version), nil
}

// FindSchemasForTable lists schemas that currently contain the named table.
func (p *Provider) FindSchemasForTable(ctx context.Context, table string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `
		select table_schema
		from information_schema.tables
		where table_name = ?
		order by table_schema
	`, table)
	if err != nil {
		return nil, fmt.Errorf("query schemas for table %s: %w", table, err)
	}
	defer rows.Close()

	schemas := make([]string, 0)
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, fmt.Errorf("scan schema for table %s: %w", table, err)
		}
		schemas = append(schemas, schema)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schemas for table %s: %w", table, err)
	}
	return schemas, nil
}

var _ appaudit.MetadataProvider = (*Provider)(nil)

// LoadInstanceFacts reads server-level variables that influence audit behavior.
// Facts that cannot be observed stay unknown (Known=false) rather than being
// back-filled with defaults — rules downgrade to bounded evidence gaps on them.
func (p *Provider) LoadInstanceFacts(ctx context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	rows, err := p.db.QueryContext(ctx, `
		show variables where Variable_name in
		('version','character_set_database','innodb_large_prefix','innodb_default_row_format','innodb_adaptive_hash_index','innodb_page_size')
	`)
	if err != nil {
		return nil, fmt.Errorf("query instance facts: %w", err)
	}
	defer rows.Close()

	facts := &spec.InstanceFacts{}
	for rows.Next() {
		var name string
		var value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan instance fact: %w", err)
		}
		switch strings.ToLower(name) {
		case "version":
			facts.Version = value
		case "character_set_database":
			facts.DefaultCharset = value
		case "innodb_large_prefix":
			facts.InnoDBLargePrefixEnabled = normalizeOnOff(value)
			facts.InnoDBLargePrefixKnown = true
		case "innodb_default_row_format":
			facts.InnoDBDefaultRowFormat = strings.ToLower(value)
		case "innodb_adaptive_hash_index":
			facts.InnoDBAdaptiveHashEnabled = normalizeOnOff(value)
		case "innodb_page_size":
			if parsed, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil && parsed > 0 {
				facts.InnoDBPageSizeBytes = parsed
				facts.InnoDBPageSizeKnown = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance facts: %w", err)
	}

	// TiDB's index-length bound is the tidb-server max-index-length config,
	// not a system variable. SHOW CONFIG is the supported read path. An
	// actual read failure propagates as a provider error; only a clean read
	// with no usable or no consistent value leaves the fact unknown so the
	// rule reports a bounded gap instead of assuming the 3072 default.
	if strings.Contains(strings.ToLower(facts.Version), "-tidb-") {
		value, known, cfgErr := p.loadTiDBMaxIndexLength(ctx)
		if cfgErr != nil {
			return nil, fmt.Errorf("load tidb max-index-length: %w", cfgErr)
		}
		if known {
			facts.TiDBMaxIndexLengthBytes = value
			facts.TiDBMaxIndexLengthKnown = true
		}
	}
	return facts, nil
}

// loadTiDBMaxIndexLength reads the effective tidb-server max-index-length
// configuration via SHOW CONFIG and verifies every matching row — one row
// per tidb-server instance — never trusting the first row alone. Query,
// scan, and iteration failures return an error (provider errors never
// degrade into gaps); an unparsable or non-positive value marks the read
// unverifiable but the stream is still drained, so a later failure still
// surfaces as an error. A clean read ends in zero rows, an invalid value,
// or disagreeing instances → ok=false: the fact stays unknown rather than
// fabricating a bound.
func (p *Provider) loadTiDBMaxIndexLength(ctx context.Context) (value int, ok bool, err error) {
	rows, err := p.db.QueryContext(ctx, `
		show config where Type = 'tidb' and Name = 'max-index-length'
	`)
	if err != nil {
		return 0, false, fmt.Errorf("query tidb max-index-length: %w", err)
	}
	defer rows.Close()

	consistent := true
	for rows.Next() {
		var typ, instance, name, raw string
		if err := rows.Scan(&typ, &instance, &name, &raw); err != nil {
			return 0, false, fmt.Errorf("scan tidb max-index-length: %w", err)
		}
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(raw))
		if parseErr != nil || parsed <= 0 {
			// An unusable value marks the read unverifiable but the stream
			// is still drained to its end: a later scan or iteration
			// failure remains a provider error, never an unknown fact.
			consistent = false
			continue
		}
		if !ok {
			value, ok = parsed, true
		} else if parsed != value {
			consistent = false
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, fmt.Errorf("iterate tidb max-index-length: %w", err)
	}
	if !consistent {
		return 0, false, nil
	}
	return value, ok, nil
}

// LoadTableSnapshot reads one target table shape from information_schema.
func (p *Provider) LoadTableSnapshot(ctx context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	snapshot := &spec.TableSnapshot{
		Schema: schema,
		Exists: false,
	}

	tableRow := p.db.QueryRowContext(ctx, `
		select engine, table_collation, table_comment, auto_increment, row_format, table_rows
		from information_schema.tables
		where table_schema = ? and table_name = ?
	`, schema, table)

	var engine sql.NullString
	var collation sql.NullString
	var comment sql.NullString
	var autoIncrement sql.NullInt64
	var rowFormat sql.NullString
	var tableRows sql.NullInt64
	if err := tableRow.Scan(&engine, &collation, &comment, &autoIncrement, &rowFormat, &tableRows); err != nil {
		if err == sql.ErrNoRows {
			return snapshot, nil
		}
		return nil, fmt.Errorf("query table snapshot: %w", err)
	}

	snapshot.Exists = true
	snapshot.Table = &spec.Table{Name: table, Comment: comment.String}
	snapshot.Options = map[string]string{}
	if engine.Valid {
		snapshot.Options["engine"] = engine.String
	}
	if rowFormat.Valid {
		snapshot.Options["row_format"] = strings.ToUpper(rowFormat.String)
	}
	if autoIncrement.Valid {
		snapshot.Options["auto_increment"] = strconv.FormatInt(autoIncrement.Int64, 10)
	}
	if tableRows.Valid {
		snapshot.Options["table_rows"] = strconv.FormatInt(tableRows.Int64, 10)
	}
	if collation.Valid {
		snapshot.Options["collation"] = collation.String
		snapshot.Options["charset"] = charsetFromCollation(collation.String)
	}

	if err := p.loadColumns(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s columns: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if err := p.loadIndexes(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s indexes: %w", snapshot.Schema, snapshot.Table.Name, err)
	}

	return snapshot, nil
}

func (p *Provider) loadColumns(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select column_name, column_type, character_set_name, collation_name, column_comment,
		       column_default, is_nullable, extra
		from information_schema.columns
		where table_schema = ? and table_name = ?
		order by ordinal_position
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, columnType, isNullable, extra string
		var charset, collation, comment, defaultValue sql.NullString
		if err := rows.Scan(&name, &columnType, &charset, &collation, &comment, &defaultValue, &isNullable, &extra); err != nil {
			return fmt.Errorf("scan table column: %w", err)
		}
		baseType, length, unsigned := parseColumnType(columnType)
		column := spec.Column{
			Name:          name,
			Type:          baseType,
			Length:        length,
			Charset:       charset.String,
			Collation:     collation.String,
			Comment:       comment.String,
			Unsigned:      unsigned,
			NotNull:       strings.EqualFold(isNullable, "NO"),
			AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
			HasDefault:    defaultValue.Valid,
			DefaultValue:  defaultValue.String,
		}
		if defaultValue.Valid && strings.EqualFold(defaultValue.String, "current_timestamp") {
			column.DefaultIsCurrentTimestamp = true
		}
		// A non-NULL COLUMN_DEFAULT is always a stored representation, never
		// proof of a SQL NULL default: the text "null" is the string literal
		// 'null', while a real DEFAULT NULL surfaces as a NULL row. A NULL
		// column_default also cannot distinguish an omitted DEFAULT clause
		// from an explicit DEFAULT NULL, so neither sets DefaultIsNull here.
		if strings.Contains(strings.ToLower(extra), "on update current_timestamp") {
			column.OnUpdateCurrentTimestamp = true
		}
		snapshot.Columns = append(snapshot.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table columns: %w", err)
	}
	return nil
}

func (p *Provider) loadIndexes(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select index_name, non_unique, index_type, column_name, cardinality
		from information_schema.statistics
		where table_schema = ? and table_name = ?
		order by index_name, seq_in_index
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table indexes: %w", err)
	}
	defer rows.Close()

	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)

	for rows.Next() {
		var name, indexType, columnName string
		var nonUnique int
		var cardinality sql.NullInt64
		if err := rows.Scan(&name, &nonUnique, &indexType, &columnName, &cardinality); err != nil {
			return fmt.Errorf("scan table index: %w", err)
		}

		var cardinalityValue *int64
		if cardinality.Valid {
			cardinalityValue = ptrInt64(cardinality.Int64)
		}
		accumulateIndexRow(indexes, &order, name, nonUnique, indexType, columnName, cardinalityValue)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table indexes: %w", err)
	}

	for _, name := range order {
		index := indexes[name]
		if index.Kind == spec.IndexKindPrimary {
			snapshot.PrimaryKey = index
			continue
		}
		snapshot.Indexes = append(snapshot.Indexes, *index)
	}
	return nil
}

func accumulateIndexRow(indexes map[string]*spec.Index, order *[]string, name string, nonUnique int, indexType string, columnName string, cardinality *int64) {
	index, ok := indexes[name]
	if !ok {
		index = &spec.Index{
			Name: name,
			Kind: classifyIndex(name, nonUnique, indexType),
		}
		indexes[name] = index
		*order = append(*order, name)
	}

	index.Columns = append(index.Columns, columnName)
	if cardinality == nil {
		return
	}
	if index.Cardinality == nil || *cardinality > *index.Cardinality {
		index.Cardinality = ptrInt64(*cardinality)
	}
}

func normalizeOnOff(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "yes", "true":
		return true
	default:
		return false
	}
}

func charsetFromCollation(collation string) string {
	if idx := strings.Index(collation, "_"); idx > 0 {
		return collation[:idx]
	}
	return ""
}

func parseColumnType(columnType string) (baseType string, length int, unsigned bool) {
	lower := strings.ToLower(strings.TrimSpace(columnType))
	unsigned = strings.Contains(lower, " unsigned")
	if idx := strings.Index(lower, "("); idx >= 0 {
		baseType = strings.TrimSpace(lower[:idx])
		end := strings.Index(lower[idx+1:], ")")
		if end >= 0 {
			part := lower[idx+1 : idx+1+end]
			if comma := strings.Index(part, ","); comma >= 0 {
				part = part[:comma]
			}
			if parsed, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				length = parsed
			}
		}
	} else {
		baseType = strings.Fields(lower)[0]
	}
	return baseType, length, unsigned
}

func classifyIndex(name string, nonUnique int, indexType string) spec.IndexKind {
	if strings.EqualFold(name, "primary") {
		return spec.IndexKindPrimary
	}
	if strings.EqualFold(indexType, "fulltext") {
		return spec.IndexKindFulltext
	}
	if nonUnique == 0 {
		return spec.IndexKindUnique
	}
	return spec.IndexKindSecondary
}

func detectDialectFromVersion(version string) spec.Dialect {
	if strings.Contains(strings.ToLower(version), "tidb") {
		return spec.DialectTiDB
	}
	return spec.DialectMySQL
}

func ptrInt64(value int64) *int64 {
	return &value
}
