// Package mysqlmeta implements metadata-aware audit adapters over the MySQL protocol.
// input: sql.DB access plus connection configs, version/schema/table lookup requests, and MySQL/TiDB metadata queries
// output: normalized instance facts, dialect detection, schema discovery, and table snapshots with preserved index cardinality for application-level audit enrichment
// pos: infrastructure metadata adapter between database/sql and domain metadata specs
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
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

// ConnectionConfig describes one MySQL-compatible metadata connection.
type ConnectionConfig struct {
	Host     string
	Port     int
	Socket   string
	User     string
	Password string
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

// DSN formats the config for the MySQL driver.
func (c ConnectionConfig) DSN() string {
	cfg := gomysql.NewConfig()
	cfg.Net = c.Network()
	cfg.Addr = c.Address()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Collation = "utf8mb4_general_ci"
	cfg.Params = map[string]string{"interpolateParams": "true"}
	return cfg.FormatDSN()
}

// OpenDB connects to a MySQL-compatible database for metadata reads.
// The caller is responsible for closing the returned *sql.DB.
func OpenDB(config ConnectionConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", config.DSN())
	if err != nil {
		return nil, fmt.Errorf("open metadata connection: %w", err)
	}

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
func (p *Provider) LoadInstanceFacts(ctx context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	rows, err := p.db.QueryContext(ctx, `
		show variables where Variable_name in
		('version','character_set_database','innodb_large_prefix','innodb_default_row_format','innodb_adaptive_hash_index')
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
		case "innodb_default_row_format":
			facts.InnoDBDefaultRowFormat = strings.ToLower(value)
		case "innodb_adaptive_hash_index":
			facts.InnoDBAdaptiveHashEnabled = normalizeOnOff(value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance facts: %w", err)
	}
	return facts, nil
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
		if defaultValue.Valid && strings.EqualFold(defaultValue.String, "null") {
			column.DefaultIsNull = true
		}
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
