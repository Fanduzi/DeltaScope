// Package postgresqlmeta implements metadata-aware audit adapters over PostgreSQL.
// input: sql.DB access plus version/schema/table lookup requests and PostgreSQL catalog queries
// output: normalized instance facts, dialect detection, schema discovery, table snapshots, and plan estimates for application-level audit enrichment
// pos: infrastructure metadata adapter between database/sql and domain metadata specs
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var explainRowsPattern = regexp.MustCompile(`rows=(\d+)`)

// Provider loads metadata facts through a PostgreSQL SQL connection.
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
		select distinct n.nspname as schema_name
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where c.relname = $1
		  and c.relkind in ('r','p','v','m','f')
		  and n.nspname not in ('pg_catalog', 'information_schema')
		order by n.nspname
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

// LoadInstanceFacts reads server-level settings that influence audit behavior.
func (p *Provider) LoadInstanceFacts(ctx context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	facts := &spec.InstanceFacts{}

	if err := p.db.QueryRowContext(ctx, `select version()`).Scan(&facts.Version); err != nil {
		return nil, fmt.Errorf("query server version: %w", err)
	}

	rows, err := p.db.QueryContext(ctx, `
		select name, setting
		from pg_settings
		where name in ('server_encoding','default_toast_compression','wal_compression')
	`)
	if err != nil {
		return nil, fmt.Errorf("query instance facts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan instance fact: %w", err)
		}
		switch strings.ToLower(name) {
		case "server_encoding":
			facts.DefaultCharset = value
		case "default_toast_compression":
			facts.InnoDBDefaultRowFormat = strings.ToLower(value)
		case "wal_compression":
			facts.InnoDBAdaptiveHashEnabled = normalizeYesNo(value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instance facts: %w", err)
	}
	return facts, nil
}

// LoadTableSnapshot reads one target table shape from PostgreSQL catalogs.
func (p *Provider) ResolveTableForIndex(ctx context.Context, _ spec.Dialect, schema string, index string) (string, error) {
	var tableName string
	err := p.db.QueryRowContext(ctx, `
		select tbl.relname as table_name
		from pg_catalog.pg_class idx
		join pg_catalog.pg_namespace n on n.oid = idx.relnamespace
		join pg_catalog.pg_index ind on ind.indexrelid = idx.oid
		join pg_catalog.pg_class tbl on tbl.oid = ind.indrelid
		where n.nspname = $1 and idx.relname = $2
		limit 1
	`, schema, index).Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query table for index %s: %w", index, err)
	}
	return tableName, nil
}

func (p *Provider) LoadTableSnapshot(ctx context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	snapshot := &spec.TableSnapshot{
		Schema: schema,
		Exists: false,
	}

	tableRow := p.db.QueryRowContext(ctx, `
		select c.relkind,
		       obj_description(c.oid, 'pg_class') as table_comment,
		       c.reltuples
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where n.nspname = $1 and c.relname = $2 and c.relkind in ('r','p','v','m','f')
	`, schema, table)

	var relkind string
	var comment sql.NullString
	var reltuples sql.NullFloat64
	if err := tableRow.Scan(&relkind, &comment, &reltuples); err != nil {
		if err == sql.ErrNoRows {
			return snapshot, nil
		}
		return nil, fmt.Errorf("query table snapshot: %w", err)
	}

	snapshot.Exists = true
	snapshot.Table = &spec.Table{Name: table, Comment: comment.String}
	snapshot.Options = map[string]string{"table_type": tableTypeFromRelkind(relkind)}
	if reltuples.Valid {
		snapshot.Options["table_rows"] = strconv.FormatInt(int64(reltuples.Float64), 10)
	}

	if err := p.loadColumns(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s columns: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if err := p.loadConstraints(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s constraints: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if err := p.loadIndexes(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s indexes: %w", snapshot.Schema, snapshot.Table.Name, err)
	}

	return snapshot, nil
}

// LoadPlanEstimate reads a conservative row estimate using plain EXPLAIN output.
func (p *Provider) LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	if statement.DML == nil {
		return nil, nil
	}
	switch statement.DML.Operation {
	case spec.DMLOperationUpdate, spec.DMLOperationDelete:
	default:
		return nil, nil
	}

	rawSQL := strings.TrimSpace(statement.RawSQL)
	if rawSQL == "" {
		return nil, nil
	}

	rows, err := p.db.QueryContext(ctx, "EXPLAIN "+rawSQL)
	if err != nil {
		return nil, fmt.Errorf("explain statement: %w", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("scan explain output: %w", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate explain output: %w", err)
	}

	estimatedRows, ok := parseExplainEstimatedRows(planLines)
	if !ok {
		return nil, nil
	}

	estimate := &spec.ImpactEstimate{
		EstimatedRows: ptrInt64(estimatedRows),
		Source:        spec.ImpactSourcePlan,
		Confidence:    spec.ImpactConfidenceMedium,
		ReasonCodes:   []string{"explain_rows"},
		Notes:         []string{"plain EXPLAIN planner rows estimate"},
	}
	if estimatedRows <= 1 {
		estimate.RiskLevel = spec.ImpactRiskLow
	} else if estimatedRows < 100 {
		estimate.RiskLevel = spec.ImpactRiskMedium
	} else {
		estimate.RiskLevel = spec.ImpactRiskHigh
	}
	return estimate, nil
}

func (p *Provider) loadColumns(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select a.attname as column_name,
		       format_type(a.atttypid, a.atttypmod) as data_type,
		       information_schema._pg_char_max_length(information_schema._pg_truetypid(a.*, t.*), information_schema._pg_truetypmod(a.*, t.*)) as character_maximum_length,
		       null::text as character_set_name,
		       coll.collname as collation_name,
		       a.attnotnull as is_not_null,
		       pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) as column_default,
		       (a.attidentity <> '') as is_identity,
		       col_description(a.attrelid, a.attnum) as column_comment
		from pg_catalog.pg_attribute a
		join pg_catalog.pg_class c on c.oid = a.attrelid
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		join pg_catalog.pg_type t on t.oid = a.atttypid
		left join pg_catalog.pg_attrdef ad on ad.adrelid = a.attrelid and ad.adnum = a.attnum
		left join pg_catalog.pg_collation coll on coll.oid = a.attcollation and a.attcollation <> t.typcollation
		where n.nspname = $1 and c.relname = $2 and a.attnum > 0 and not a.attisdropped
		order by a.attnum
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, dataType string
		var maximumLength sql.NullInt64
		var charset, collation, defaultValue, comment sql.NullString
		var isNotNull, isIdentity bool
		if err := rows.Scan(&name, &dataType, &maximumLength, &charset, &collation, &isNotNull, &defaultValue, &isIdentity, &comment); err != nil {
			return fmt.Errorf("scan table column: %w", err)
		}
		baseType, length, unsigned := parseDataType(dataType, maximumLength, defaultValue.String)
		column := spec.Column{
			Name:          name,
			Type:          baseType,
			Length:        length,
			Charset:       charset.String,
			Collation:     collation.String,
			Comment:       comment.String,
			Unsigned:      unsigned,
			NotNull:       isNotNull,
			AutoIncrement: isIdentity || strings.Contains(strings.ToLower(defaultValue.String), "nextval("),
			HasDefault:    defaultValue.Valid,
			DefaultValue:  defaultValue.String,
		}
		if defaultValue.Valid && strings.EqualFold(defaultValue.String, "current_timestamp") {
			column.DefaultIsCurrentTimestamp = true
		}
		if defaultValue.Valid && strings.EqualFold(defaultValue.String, "null") {
			column.DefaultIsNull = true
		}
		snapshot.Columns = append(snapshot.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table columns: %w", err)
	}
	return nil
}

func (p *Provider) loadConstraints(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select con.conname as constraint_name,
		       idx.relname as index_name,
		       att.attname as column_name,
		       idx.reltuples as index_reltuples
		from pg_catalog.pg_constraint con
		join pg_catalog.pg_class tbl on tbl.oid = con.conrelid
		join pg_catalog.pg_namespace n on n.oid = tbl.relnamespace
		left join pg_catalog.pg_class idx on idx.oid = con.conindid
		left join unnest(con.conkey) with ordinality as cols(attnum, ordinality) on true
		left join pg_catalog.pg_attribute att on att.attrelid = con.conrelid and att.attnum = cols.attnum
		where n.nspname = $1 and tbl.relname = $2 and con.contype = 'p'
		order by con.conname, cols.ordinality
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table constraints: %w", err)
	}
	defer rows.Close()

	constraintColumns := make(map[string][]string)
	constraintIndexName := make(map[string]string)
	constraintCardinality := make(map[string]*int64)
	order := make([]string, 0)

	for rows.Next() {
		var constraintName string
		var indexName sql.NullString
		var columnName sql.NullString
		var reltuples sql.NullFloat64
		if err := rows.Scan(&constraintName, &indexName, &columnName, &reltuples); err != nil {
			return fmt.Errorf("scan table constraint: %w", err)
		}
		if _, ok := constraintColumns[constraintName]; !ok {
			constraintColumns[constraintName] = nil
			order = append(order, constraintName)
		}
		if columnName.Valid {
			constraintColumns[constraintName] = append(constraintColumns[constraintName], columnName.String)
		}
		if indexName.Valid {
			constraintIndexName[constraintName] = indexName.String
		}
		if reltuples.Valid {
			value := int64(reltuples.Float64)
			constraintCardinality[constraintName] = &value
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table constraints: %w", err)
	}

	for _, name := range order {
		constraint := spec.Constraint{
			Type:    "primary_key",
			Name:    name,
			Columns: append([]string(nil), constraintColumns[name]...),
		}
		snapshot.Constraints = append(snapshot.Constraints, constraint)
		if snapshot.PrimaryKey == nil {
			snapshot.PrimaryKey = &spec.Index{
				Name:        constraintIndexName[name],
				Kind:        spec.IndexKindPrimary,
				Columns:     append([]string(nil), constraintColumns[name]...),
				Cardinality: cloneInt64Ptr(constraintCardinality[name]),
			}
		}
	}
	return nil
}

func (p *Provider) loadIndexes(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select idx.relname as index_name,
		       ind.indisunique as is_unique,
		       ind.indisprimary as is_primary,
		       pg_get_indexdef(idx.oid) as indexdef,
		       idx.reltuples as index_reltuples
		from pg_catalog.pg_index ind
		join pg_catalog.pg_class tbl on tbl.oid = ind.indrelid
		join pg_catalog.pg_namespace n on n.oid = tbl.relnamespace
		join pg_catalog.pg_class idx on idx.oid = ind.indexrelid
		where n.nspname = $1 and tbl.relname = $2
		order by idx.relname
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table indexes: %w", err)
	}
	defer rows.Close()

	indexByName := make(map[string]spec.Index)
	order := make([]string, 0)

	for rows.Next() {
		var name, definition string
		var isUnique, isPrimary bool
		var reltuples sql.NullFloat64
		if err := rows.Scan(&name, &isUnique, &isPrimary, &definition, &reltuples); err != nil {
			return fmt.Errorf("scan table index: %w", err)
		}

		index := spec.Index{
			Name:    name,
			Kind:    classifyIndex(isUnique, isPrimary),
			Columns: parseIndexColumns(definition),
		}
		if reltuples.Valid {
			index.Cardinality = ptrInt64(int64(reltuples.Float64))
		}
		indexByName[name] = index
		order = append(order, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table indexes: %w", err)
	}

	sort.Strings(order)
	for _, name := range order {
		index := indexByName[name]
		if index.Kind == spec.IndexKindPrimary {
			if snapshot.PrimaryKey == nil {
				copied := index
				snapshot.PrimaryKey = &copied
			} else {
				snapshot.PrimaryKey.Name = index.Name
				if len(snapshot.PrimaryKey.Columns) == 0 {
					snapshot.PrimaryKey.Columns = append([]string(nil), index.Columns...)
				}
				if snapshot.PrimaryKey.Cardinality == nil {
					snapshot.PrimaryKey.Cardinality = cloneInt64Ptr(index.Cardinality)
				}
			}
			continue
		}
		snapshot.Indexes = append(snapshot.Indexes, index)
	}
	return nil
}

func normalizeYesNo(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "yes", "true":
		return true
	default:
		return false
	}
}

func parseDataType(dataType string, maximumLength sql.NullInt64, defaultValue string) (baseType string, length int, unsigned bool) {
	normalized := strings.ToLower(strings.TrimSpace(dataType))
	switch {
	case strings.HasPrefix(normalized, "character varying"):
		baseType = "varchar"
	case strings.HasPrefix(normalized, "timestamp with time zone"):
		baseType = "timestamptz"
	case strings.Contains(normalized, "("):
		baseType = strings.TrimSpace(normalized[:strings.Index(normalized, "(")])
	default:
		baseType = normalized
	}
	if maximumLength.Valid {
		length = int(maximumLength.Int64)
	}
	return baseType, length, strings.Contains(strings.ToLower(defaultValue), "unsigned")
}

func classifyIndex(isUnique bool, isPrimary bool) spec.IndexKind {
	if isPrimary {
		return spec.IndexKindPrimary
	}
	if isUnique {
		return spec.IndexKindUnique
	}
	return spec.IndexKindSecondary
}

func parseIndexColumns(definition string) []string {
	start := strings.LastIndex(definition, "(")
	end := strings.LastIndex(definition, ")")
	if start < 0 || end <= start {
		return nil
	}
	parts := strings.Split(definition[start+1:end], ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		trimmed = strings.Trim(trimmed, `"`)
		if space := strings.Index(trimmed, " "); space >= 0 {
			trimmed = trimmed[:space]
		}
		if trimmed != "" {
			columns = append(columns, trimmed)
		}
	}
	return columns
}

func detectDialectFromVersion(version string) spec.Dialect {
	if strings.Contains(strings.ToLower(version), "postgresql") {
		return spec.DialectPostgreSQL
	}
	return spec.DialectUnknown
}

func tableTypeFromRelkind(relkind string) string {
	switch strings.TrimSpace(relkind) {
	case "r", "p", "f":
		return "BASE TABLE"
	case "v", "m":
		return "VIEW"
	default:
		return "UNKNOWN"
	}
}

func parseExplainEstimatedRows(lines []string) (int64, bool) {
	var maxRows int64
	var found bool
	for _, line := range lines {
		matches := explainRowsPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		rows, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			continue
		}
		if !found || rows > maxRows {
			maxRows = rows
			found = true
		}
	}
	return maxRows, found
}

func ptrInt64(value int64) *int64 {
	return &value
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
