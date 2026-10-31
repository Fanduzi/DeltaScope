// Package mysqlmeta implements metadata-aware audit adapters over the MySQL protocol.
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
	"unicode"

	gomysql "github.com/go-sql-driver/mysql"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
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
		('version','character_set_database','innodb_large_prefix','innodb_default_row_format','innodb_adaptive_hash_index','innodb_page_size','tidb_enable_clustered_index','restrict_fk_on_non_standard_key','tidb_enable_check_constraint')
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
		case "tidb_enable_clustered_index":
			// The session-scoped mode governs unstated-type PK storage on
			// TiDB (v8.5.0 pkg/ddl/create_table.go L1582-1593). MySQL never
			// returns the row, so only the TiDB arm below consumes it.
			facts.TiDBEnableClusteredIndex = strings.ToLower(strings.TrimSpace(value))
		case "restrict_fk_on_non_standard_key":
			// MySQL 8.4+'s non-standard referenced-key refusal gate —
			// absent on 5.7/8.0/TiDB so an unobserved answer is resolved
			// by dialect and version downstream, never by a guessed value.
			facts.RestrictNonStandardKey = normalizeOnOff(value)
			facts.RestrictNonStandardKeyKnown = true
		case "tidb_enable_check_constraint":
			// TiDB's CHECK storage/enforcement switch (v8.5.0) — OFF means
			// CHECK syntax parses but no member is stored or enforced.
			// MySQL never returns the row; unobserved stays Known=false.
			facts.TiDBEnableCheckConstraint = normalizeOnOff(value)
			facts.TiDBEnableCheckConstraintKnown = true
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
		alterPK, alterKnown, alterErr := p.loadTiDBAlterPrimaryKey(ctx)
		if alterErr != nil {
			return nil, fmt.Errorf("load tidb alter-primary-key: %w", alterErr)
		}
		if alterKnown {
			facts.TiDBAlterPrimaryKey = alterPK
			facts.TiDBAlterPrimaryKeyKnown = true
		}
	} else {
		// A non-TiDB server never consults the clustered-index mode — an
		// unexpected row must not leak a TiDB fact onto MySQL facts.
		facts.TiDBEnableClusteredIndex = ""
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

// loadTiDBAlterPrimaryKey reads the effective tidb-server
// alter-primary-key configuration via SHOW CONFIG — the fact the int_only
// clustered mode defers to for a single-int PK (upstream: !AlterPrimaryKey
// && isSingleIntPK). Same contract as loadTiDBMaxIndexLength: query, scan,
// and iteration failures propagate as provider errors; every matching row
// (one per tidb-server instance) must parse and agree, otherwise the fact
// stays unknown so a declared unstated-type PK resolves to a bounded gap
// rather than a guessed handle answer.
func (p *Provider) loadTiDBAlterPrimaryKey(ctx context.Context) (value bool, ok bool, err error) {
	rows, err := p.db.QueryContext(ctx, `
		show config where Type = 'tidb' and Name = 'alter-primary-key'
	`)
	if err != nil {
		return false, false, fmt.Errorf("query tidb alter-primary-key: %w", err)
	}
	defer rows.Close()

	consistent := true
	for rows.Next() {
		var typ, instance, name, raw string
		if err := rows.Scan(&typ, &instance, &name, &raw); err != nil {
			return false, false, fmt.Errorf("scan tidb alter-primary-key: %w", err)
		}
		parsed, parseErr := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
		if parseErr != nil {
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
		return false, false, fmt.Errorf("iterate tidb alter-primary-key: %w", err)
	}
	if !consistent {
		return false, false, nil
	}
	return value, ok, nil
}

// LoadTableSnapshot reads one target table shape from information_schema.
func (p *Provider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
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
	if err := p.loadIndexes(ctx, snapshot, dialect); err != nil {
		return nil, fmt.Errorf("load table %s.%s indexes: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if err := p.loadForeignKeys(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s foreign keys: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if err := p.loadCheckConstraints(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s check constraints: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	// Partition facts are loaded on both products — the TiDB GLOBAL index
	// gate and the partition-action applicability checks both consume them.
	if err := p.loadPartitionFacts(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("load table %s.%s partition state: %w", snapshot.Schema, snapshot.Table.Name, err)
	}
	if dialect == spec.DialectTiDB {
		// PrimaryKeyIsHandle is TiDB-only — skip those probes on MySQL.
		if err := p.resolveTiDBPrimaryName(ctx, snapshot); err != nil {
			return nil, fmt.Errorf("load table %s.%s primary-key identity: %w", snapshot.Schema, snapshot.Table.Name, err)
		}
		if err := p.loadTiDBPrimaryKeyHandle(ctx, snapshot); err != nil {
			return nil, fmt.Errorf("load table %s.%s primary-key handle: %w", snapshot.Schema, snapshot.Table.Name, err)
		}
	}

	return snapshot, nil
}

// loadPartitionFacts records the observed partition catalog: whether the
// table is partitioned, the PARTITION_METHOD normalized lowercase
// (`range`, `hash`, `linear key`, `range columns`, ...), and the member
// names in ordinal order. A query the catalog cannot answer keeps every
// fact unknown (nil) — a bounded gap, not a guessed "not partitioned".
// A partitioned table's catalog answer always carries a method, so a
// nil PartitionType under partitioned=true is itself an unobserved fact.
func (p *Provider) loadPartitionFacts(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select partition_method, partition_name, subpartition_name, partition_description
		from information_schema.partitions
		where table_schema = ? and table_name = ?
			and partition_name is not null
		order by partition_ordinal_position
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			// An unanswerable catalog probe keeps every partition fact
			// unknown — a bounded gap, never a guessed "not partitioned".
			return nil
		}
		return fmt.Errorf("query partitions: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	descriptions := make([]string, 0)
	seen := make(map[string]bool)
	var method string
	subpartitioned := false
	for rows.Next() {
		var rawMethod, name, subName, description sql.NullString
		if err := rows.Scan(&rawMethod, &name, &subName, &description); err != nil {
			return fmt.Errorf("scan partition for table %s.%s: %w", snapshot.Schema, snapshot.Table.Name, err)
		}
		if method == "" && rawMethod.Valid {
			method = strings.ToLower(rawMethod.String)
		}
		if subName.Valid {
			// Leaf members live in SUBPARTITION_NAME — the top-level
			// projection is not the member space, so the directory is
			// incomplete: MySQL exchanges a leaf name natively.
			subpartitioned = true
		}
		if name.Valid && !seen[name.String] {
			seen[name.String] = true
			names = append(names, name.String)
			// PARTITION_DESCRIPTION is NULL on HASH/KEY members and on
			// unanswerable cells; the recorded text keeps alignment with
			// the name list ("" marks the member whose bound text the
			// catalog never stored).
			text := ""
			if description.Valid {
				text = description.String
			}
			descriptions = append(descriptions, text)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate partitions for table %s.%s: %w", snapshot.Schema, snapshot.Table.Name, err)
	}

	partitioned := len(names) > 0
	snapshot.Partitioned = &partitioned
	snapshot.Subpartitioned = subpartitioned
	if partitioned {
		if method != "" {
			snapshot.PartitionType = &method
		}
		if !subpartitioned {
			snapshot.PartitionNames = names
			snapshot.PartitionDescriptions = descriptions
			snapshot.PartitionDefs = spec.CatalogPartitionDefs(names, descriptions, method)
		}
		// A subpartitioned catalog is an incomplete projection: the
		// member directory stays unobserved (nil) so no rule can claim
		// a name was proven absent.
	} else {
		// No partition rows is an observed-empty member set on a
		// known-plain table — never confused with an unobserved set.
		snapshot.PartitionNames = []string{}
	}
	return nil
}

// resolveTiDBPrimaryName binds the PK identity to an index-object fold
// under the table's proven constraint presence
// (information_schema.table_constraints — a `PRIMARY KEY` row proves the
// constraint exists; verified on the TiDB v8.5.0 anchor that tidb_pk_type
// cannot answer this — it reports NONCLUSTERED both for PK-less tables and
// for real nonclustered primaries). Presence and attribution are judged
// separately: a proven constraint with zero `primary`-fold rows means the
// PK object was renamed (RENAME INDEX `PRIMARY` TO pk2 is legal on v8.5.0
// and moves the object's stored catalog name while the constraint stays
// PRIMARY), never that no PK exists — the constraint row and the name
// space answer different questions.
//
// Attribution resolves by unique-fold candidacy — a PK object always
// emits a NON_UNIQUE=0 STATISTICS fold (handle tables synthesize one),
// while ordinary members may be unique too, so every unique fold is a
// candidate:
//   - zero candidates means the constraint declares members the index
//     catalog does not carry — presence stays vouched, nothing is
//     attributable;
//   - exactly one candidate can only be the PK object — promote it;
//   - several candidates consult TIDB_INDEXES clustered first: only the
//     PK object's row can read YES, so exactly one clustered candidate
//     identifies it, and more than one contradicts the catalog's own
//     rule — withhold;
//   - with no clustered candidate, the constraint's ordered member
//     sequence (key_column_usage.ordinal_position) separates the object:
//     exactly one ordered match promotes — never the first read in
//     STATISTICS order, and never a same-set-different-order member;
//   - anything else withholds: the constraint still proves presence (a
//     name-unknown placeholder), the surviving candidates demote to kind
//     unknown so unique-overlap consumers never read a maybe-PK fold as
//     an ordinary UNIQUE member, and the handle/clustered answers stay
//     unproven unless every candidate already proved NONCLUSTERED.
//
// A proven-absent constraint (count 0) keeps every fold a secondary; a
// failed constraint or disambiguation probe is a provider failure and
// propagates — issue #79 requires actual query/connection errors to stay
// errors rather than degrade into withheld facts.
func (p *Provider) resolveTiDBPrimaryName(ctx context.Context, snapshot *spec.TableSnapshot) error {
	uniquePositions := make([]int, 0, 2)
	for i := range snapshot.Indexes {
		if snapshot.Indexes[i].Kind == spec.IndexKindUnique {
			uniquePositions = append(uniquePositions, i)
		}
	}
	var pkCount int
	err := p.db.QueryRowContext(ctx, `
		select count(*)
		from information_schema.table_constraints
		where table_schema = ? and table_name = ? and constraint_type = 'PRIMARY KEY'
	`, snapshot.Schema, snapshot.Table.Name).Scan(&pkCount)
	if err != nil {
		// count(*) can never be empty — a failed read is a provider
		// failure (issue #79: actual failures stay errors), not an
		// answerable absence. Context cancellation propagates the same.
		return fmt.Errorf("query table_constraints: %w", err)
	}
	if pkCount == 0 {
		// Proven PK-less: every fold is a secondary — members, not the
		// constraint object.
		return nil
	}
	if pkCount > 1 {
		// A constraint count beyond one is a catalog inconsistency the
		// binding cannot resolve — withhold all three primary facts and
		// demote every maybe-PK fold: no candidate may stay a proven
		// UNIQUE member while the constraint truth itself is unproven.
		for _, pos := range uniquePositions {
			snapshot.Indexes[pos].Kind = spec.IndexKindUnknown
		}
		snapshot.PrimaryKeyUnknown = true
		snapshot.PrimaryKeyHandleUnknown = true
		snapshot.PrimaryKeyClusteredUnknown = true
		return nil
	}
	// pkCount == 1 — the constraint proves presence; attribute the object.
	// Every unique fold is a candidate: a PK object always emits a
	// NON_UNIQUE=0 fold, a secondary may be unique too, and a member may
	// even own `primary` uniquely (a unique member `primary` beside a
	// renamed pk2 object is legal — the name alone cannot pick).
	switch len(uniquePositions) {
	case 0:
		// A proven PK with no NON_UNIQUE=0 fold is a catalog
		// inconsistency — presence stays vouched but nothing can be
		// attributed, and the stored shape answers nothing.
		p.withholdTiDBPrimaryAttribution(snapshot, nil, false)
		return nil
	case 1:
		// Exactly one unique fold can only be the PK object — the
		// constraint requires a unique fold and none other exists.
		p.promoteTiDBPrimary(snapshot, uniquePositions[0])
		return nil
	}
	// Multiple unique candidates — the clustered fact first: only the PK
	// object's TIDB_INDEXES row can read YES.
	clustered, err := p.loadTiDBClusteredRows(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("query tidb_indexes clustered: %w", err)
	}
	clusteredCount := 0
	clusteredPos := -1
	coverageComplete := true
	for _, pos := range uniquePositions {
		switch clustered[strings.ToLower(snapshot.Indexes[pos].Name)] {
		case tidbClusteredYes:
			clusteredCount++
			clusteredPos = pos
		case tidbClusteredNo:
		default:
			// A missing row or an unrecognized value is unproven
			// coverage — never a proven NO.
			coverageComplete = false
		}
	}
	if clusteredCount == 1 {
		p.promoteTiDBPrimary(snapshot, clusteredPos)
		snapshot.PrimaryKeyClustered = true
		return nil
	}
	if clusteredCount > 1 {
		// Two clustered candidates contradicts the catalog's own rule —
		// withhold the object identity.
		p.withholdTiDBPrimaryAttribution(snapshot, uniquePositions, false)
		return nil
	}
	// No clustered candidate — the PK object is nonclustered only while
	// every candidate's row is a recognized NO; an unproven row keeps the
	// storage facts unknown instead of publishing NONCLUSTERED. The
	// constraint's declared ordered member sequence still separates the
	// object from unique members carrying the same column set.
	pkColumns, err := p.loadTiDBPrimaryKeyColumns(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("query key_column_usage primary: %w", err)
	}
	matched := -1
	matches := 0
	for _, pos := range uniquePositions {
		if columnsSequenceEqualFold(snapshot.Indexes[pos].Columns, pkColumns) {
			matched = pos
			matches++
		}
	}
	if len(pkColumns) > 0 && matches == 1 {
		p.promoteTiDBPrimary(snapshot, matched)
		return nil
	}
	p.withholdTiDBPrimaryAttribution(snapshot, uniquePositions, coverageComplete)
	return nil
}

// columnsSequenceEqualFold compares two column-name lists as ordered
// sequences — the KCU ordinal_position list against a candidate fold's
// seq_in_index order. A UNIQUE (b,a) member is never the PRIMARY KEY
// (a,b) object: the same column set in a different order is a different
// index.
func columnsSequenceEqualFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

// loadTiDBPrimaryKeyColumns returns the PRIMARY KEY constraint's declared
// column names — the fact that separates the PK object fold from ordinary
// UNIQUE members when every candidate proves nonclustered.
func (p *Provider) loadTiDBPrimaryKeyColumns(ctx context.Context, snapshot *spec.TableSnapshot) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `
		select column_name
		from information_schema.key_column_usage
		where table_schema = ? and table_name = ? and constraint_name = 'PRIMARY'
		order by ordinal_position
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			// ErrNoRows from Query is the row-absent answer — no declared
			// member list — not a provider failure.
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

// promoteTiDBPrimary moves the candidate fold to snapshot.PrimaryKey —
// the constraint object's real index-member identity. The remaining folds
// keep their member slots.
func (p *Provider) promoteTiDBPrimary(snapshot *spec.TableSnapshot, pos int) {
	index := snapshot.Indexes[pos]
	index.Kind = spec.IndexKindPrimary
	snapshot.PrimaryKey = &index
	snapshot.Indexes = append(snapshot.Indexes[:pos], snapshot.Indexes[pos+1:]...)
}

// withholdTiDBPrimaryAttribution keeps the proven PK presence (a
// name-unknown placeholder) while unvouching the object identity: each
// surviving candidate demotes to kind unknown so member-comparison rules
// never consume a maybe-PK fold as a proven UNIQUE, and the
// handle/clustered answers stay unknown unless every candidate already
// proved nonclustered storage.
func (p *Provider) withholdTiDBPrimaryAttribution(snapshot *spec.TableSnapshot, candidates []int, provenNonClustered bool) {
	snapshot.PrimaryKey = &spec.Index{Kind: spec.IndexKindPrimary, NameUnknown: true}
	for _, pos := range candidates {
		snapshot.Indexes[pos].Kind = spec.IndexKindUnknown
	}
	if !provenNonClustered {
		snapshot.PrimaryKeyHandleUnknown = true
		snapshot.PrimaryKeyClusteredUnknown = true
	}
}

// tidbClusteredValue is the tri-state answer one TIDB_INDEXES.CLUSTERED
// row can give — a clean read that is missing the row or carries a
// NULL/unrecognized value is unproven coverage, never a proven NO.
type tidbClusteredValue int

const (
	tidbClusteredUnproven tidbClusteredValue = iota
	tidbClusteredNo
	tidbClusteredYes
)

// loadTiDBClusteredRows reads every index row's clustered flag from
// information_schema.TIDB_INDEXES — the catalog's own clustered-storage
// fact, keyed by stored index name (lowercased). Only the PK object's row
// can read YES, which is exactly what separates a common-handle object
// from ordinary UNIQUE members. Candidates absent from the map (or
// carrying a NULL/unrecognized value) answer unproven, not NO.
func (p *Provider) loadTiDBClusteredRows(ctx context.Context, snapshot *spec.TableSnapshot) (map[string]tidbClusteredValue, error) {
	rows, err := p.db.QueryContext(ctx, `
		select key_name, clustered
		from information_schema.tidb_indexes
		where table_schema = ? and table_name = ?
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			// A driver reporting ErrNoRows from Query means an empty
			// result — the row-absent answer, not a failure.
			return map[string]tidbClusteredValue{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	clustered := make(map[string]tidbClusteredValue)
	for rows.Next() {
		var name string
		var value sql.NullString
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		// Keys fold to lowercase for the case-insensitive name match —
		// identifier folds compare EqualFold everywhere else. Two
		// case-variant rows (a member `primary` beside a synthesized
		// `PRIMARY`) collapse to one key, so the fold merges YES over
		// unproven over NO, independent of read order: a YES overwrite can
		// only push attribution toward the clustered candidate or the
		// withhold arm, and an unproven row may never be covered by a
		// same-key NO — the fold's answer stays unproven until every row
		// under the key is a recognized NO.
		key := strings.ToLower(name)
		state := tidbClusteredUnproven
		if value.Valid {
			switch {
			case strings.EqualFold(value.String, "yes"):
				state = tidbClusteredYes
			case strings.EqualFold(value.String, "no"):
				state = tidbClusteredNo
			}
		}
		if prev, seen := clustered[key]; !seen || state == tidbClusteredYes ||
			(prev == tidbClusteredNo && state == tidbClusteredUnproven) {
			clustered[key] = state
		}
	}
	return clustered, rows.Err()
}

// loadTiDBPrimaryKeyHandle resolves the two independent TiDB primary facts
// from information_schema.TIDB_INDEXES.CLUSTERED: clustered storage (the
// DROP PRIMARY KEY 8200 refusal class — PKIsHandle and common-handle member
// shapes alike) and whether the PK is stored as PKIsHandle — the row-id
// promotion for a single-integer clustered PK that leaves no operable
// PRIMARY index object. STATISTICS synthesizes a PRIMARY member row even
// for handle tables, so neither fact can be told by index presence alone.
// A missing catalog row keeps both unknown — MySQL and PK-less TiDB tables
// never reach this probe.
func (p *Provider) loadTiDBPrimaryKeyHandle(ctx context.Context, snapshot *spec.TableSnapshot) error {
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKeyUnknown {
		// No PK or an unresolved PK identity — the `primary` catalog rows
		// may not be the constraint object's, so neither fact can be
		// answered from them.
		return nil
	}
	if snapshot.PrimaryKey.NameUnknown || strings.TrimSpace(snapshot.PrimaryKey.Name) == "" {
		// A present-but-unattributed PK (the constraint proved presence
		// while no fold could be picked) has no proven stored name to
		// probe — the attribution pass already set the handle/clustered
		// answers it could prove, so nothing more can settle here.
		return nil
	}
	// Probe by the PK object's actual stored name — a renamed object no
	// longer answers `primary`, and a case-insensitive match may return a
	// member `primary` row alongside the synthesized PRIMARY row, so the
	// clustered fact takes the YES row whenever one exists (only the PK
	// object's row can be clustered).
	rows, err := p.db.QueryContext(ctx, `
		select clustered
		from information_schema.tidb_indexes
		where table_schema = ? and table_name = ? and key_name = ?
	`, snapshot.Schema, snapshot.Table.Name, snapshot.PrimaryKey.Name)
	if err != nil {
		if err != sql.ErrNoRows {
			// A failed catalog read is a provider failure (issue #79) —
			// propagate rather than fabricating a withheld answer.
			// Context cancellation propagates identically.
			return fmt.Errorf("query tidb_indexes clustered: %w", err)
		}
		// ErrNoRows from Query is the row-absent answer — the stored
		// shape stays unproven both ways.
		snapshot.PrimaryKeyHandleUnknown = true
		snapshot.PrimaryKeyClusteredUnknown = true
		return nil
	}
	defer rows.Close()
	rowCount := 0
	clusteredYes := false
	unrecognized := false
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return fmt.Errorf("scan tidb_indexes clustered: %w", err)
		}
		rowCount++
		if !value.Valid {
			unrecognized = true
			continue
		}
		switch {
		case strings.EqualFold(value.String, "yes"):
			clusteredYes = true
		case strings.EqualFold(value.String, "no"):
		default:
			unrecognized = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tidb_indexes clustered: %w", err)
	}
	if rowCount == 0 {
		// STATISTICS vouched the object but TIDB_INDEXES lacks the row —
		// the stored shape is unproven either way, so the handle and
		// clustered answers both stay unknown instead of defaulting to a
		// real object.
		snapshot.PrimaryKeyHandleUnknown = true
		snapshot.PrimaryKeyClusteredUnknown = true
		return nil
	}
	if unrecognized && !clusteredYes {
		// A NULL or unrecognized CLUSTERED value is equally unproven.
		snapshot.PrimaryKeyHandleUnknown = true
		snapshot.PrimaryKeyClusteredUnknown = true
		return nil
	}
	if !clusteredYes {
		// CLUSTERED=no is a real nonclustered PRIMARY index object — a
		// determined answer on both facts.
		return nil
	}
	// CLUSTERED=yes is the catalog's own clustered-storage fact — it covers
	// PKIsHandle and common-handle shapes alike, which is exactly the class
	// DROP PRIMARY KEY refuses natively (8200).
	snapshot.PrimaryKeyClustered = true
	if len(snapshot.PrimaryKey.Columns) != 1 {
		// A clustered non-single-int PK is a common-handle real object.
		return nil
	}
	if snapshot.PrimaryKey.Columns[0] == "" {
		// The member list cannot prove the PK column — the handle answer
		// stays unknown (an expression/unmodeled PK member is already
		// excluded by the single-column guard above only when a name is
		// provable).
		snapshot.PrimaryKeyHandleUnknown = true
		return nil
	}
	if snapshot.Columns == nil {
		// A withheld column set cannot prove the member is integer.
		snapshot.PrimaryKeyHandleUnknown = true
		return nil
	}
	if integer := tidbHandleColumnInteger(snapshot, snapshot.PrimaryKey.Columns[0]); integer != nil {
		snapshot.PrimaryKeyIsHandle = *integer
	} else {
		// The PK member is not provable in the vouched column set — the
		// handle answer stays unknown.
		snapshot.PrimaryKeyHandleUnknown = true
	}
	return nil
}

// tidbHandleColumnInteger reports whether the PK column is an integer kind —
// the shape TiDB promotes to PKIsHandle under clustered storage. nil when
// the column is not provable in the loaded set.
func tidbHandleColumnInteger(snapshot *spec.TableSnapshot, column string) *bool {
	for i := range snapshot.Columns {
		if strings.EqualFold(snapshot.Columns[i].Name, column) {
			integer := false
			switch catalogBaseType(snapshot.Columns[i].Type) {
			case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
				integer = true
			}
			return &integer
		}
	}
	return nil
}

func (p *Provider) loadColumns(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select column_name, column_type, character_set_name, collation_name, column_comment,
		       column_default, is_nullable, extra, generation_expression
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
		var charset, collation, comment, defaultValue, generationExpr sql.NullString
		if err := rows.Scan(&name, &columnType, &charset, &collation, &comment, &defaultValue, &isNullable, &extra, &generationExpr); err != nil {
			return fmt.Errorf("scan table column: %w", err)
		}
		typeText, length, unsigned := parseColumnType(columnType)
		extraTokens := catalogExtraTokens(extra)
		column := spec.Column{
			Name:          name,
			Type:          typeText,
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
		// The CURRENT_TIMESTAMP marker is only meaningful on temporal
		// columns: on a VARCHAR row the stored bytes are the string literal
		// 'CURRENT_TIMESTAMP', not the temporal function. The catalog
		// cannot recover which declaration produced them, but the column
		// type is authoritative — a non-temporal base type never stores a
		// temporal function default.
		if defaultValue.Valid && strings.EqualFold(defaultValue.String, "current_timestamp") && temporalDefaultColumnType(typeText) {
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
		// T07: generation presence comes from the GENERATED EXTRA token or
		// non-empty GENERATION_EXPRESSION text, never from declaration
		// spelling. Storage comes only from unambiguous STORED/VIRTUAL
		// tokens — a missing or contradictory pair stays explicit unknown
		// storage. Absent or unparseable expression text stays an explicit
		// unknown expression fact rather than collapsing into a known-empty
		// one or a fake absent column.
		generationText := ""
		if generationExpr.Valid {
			generationText = strings.TrimSpace(generationExpr.String)
		}
		if extraTokens["generated"] || generationText != "" {
			generated := &spec.GeneratedColumn{Expression: spec.ExpressionFacts{}}
			switch {
			case extraTokens["stored"] && !extraTokens["virtual"]:
				generated.Storage = spec.GeneratedStorageStored
			case extraTokens["virtual"] && !extraTokens["stored"]:
				generated.Storage = spec.GeneratedStorageVirtual
			default:
				generated.Storage = spec.GeneratedStorageUnknown
			}
			if generationText != "" {
				generated.Expression = tidbparser.ParseCatalogExpressionFacts(generationText)
			}
			column.Generated = generated
		}
		// DEFAULT_GENERATED marks an expression default: keep the raw catalog
		// bytes in DefaultValue and carry parsed facts separately. A bare
		// temporal current_timestamp keeps the existing temporal contract
		// (DefaultIsCurrentTimestamp, no DefaultExpression). Undecodable or
		// absent expression text is an explicit unknown fact.
		if extraTokens["default_generated"] && !column.DefaultIsCurrentTimestamp {
			// Catalog-stored expression defaults are always parenthesized —
			// the flag mirrors the authored-syntax fact, not the value.
			column.DefaultParenthesized = true
			if defaultValue.Valid && strings.TrimSpace(defaultValue.String) != "" {
				column.DefaultExpression = ptrExpressionFacts(tidbparser.ParseCatalogExpressionFacts(defaultValue.String))
			} else {
				column.DefaultExpression = &spec.ExpressionFacts{}
			}
		}
		// A stored literal carries its decoded value bytes verbatim on all
		// anchors — no quoting or unescaping applies. The kind comes from
		// the column's own type family, never from guessing at the byte
		// content. The recorded bytes are the literal *identity fact*;
		// whether a pair shares a proven encoding is the comparison's
		// domain check (T08-R2) — scale-normalized catalog text like 1.00
		// is recorded, not normalized to the AST integer's 1.
		if defaultValue.Valid && !column.DefaultIsCurrentTimestamp && column.DefaultExpression == nil {
			column.DefaultLiteral = &spec.LiteralDefaultFacts{
				Kind:  catalogLiteralKind(typeText),
				Value: defaultValue.String,
			}
		}
		// EXTRA is complete for every catalog row: INVISIBLE is the only
		// invisibility marker, so its absence proves effective visibility.
		// VisibilityExplicit stays false — the catalog cannot prove the
		// declaration carried the keyword.
		if extraTokens["invisible"] {
			column.Visibility = spec.ColumnVisibilityInvisible
		} else {
			column.Visibility = spec.ColumnVisibilityVisible
		}
		snapshot.Columns = append(snapshot.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table columns: %w", err)
	}
	return nil
}

// catalogBaseType reduces a parsed column type to its base keyword:
// faithful forms like decimal(10,2) or enum('a','b') lose their parameter
// span, and attribute text after the base is ignored.
func catalogBaseType(typeText string) string {
	lower := strings.ToLower(strings.TrimSpace(typeText))
	if i := strings.IndexByte(lower, '('); i >= 0 {
		lower = lower[:i]
	}
	if fields := strings.Fields(lower); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// temporalDefaultColumnType reports the only column base types that can
// carry a CURRENT_TIMESTAMP default under any supported anchor — TIMESTAMP
// and DATETIME. Any other base type storing the bytes CURRENT_TIMESTAMP
// carries the plain string literal.
func temporalDefaultColumnType(typeText string) bool {
	switch catalogBaseType(typeText) {
	case "timestamp", "datetime":
		return true
	default:
		return false
	}
}

// catalogLiteralKind names the comparison domain for a stored literal from
// the column's own type family: numeric bases decode to numeric literals,
// every other family keeps string bytes. It never inspects the byte
// content — the family is the authoritative domain.
func catalogLiteralKind(typeText string) string {
	switch catalogBaseType(typeText) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint",
		"decimal", "numeric", "float", "double", "real", "year":
		return spec.LiteralDefaultNumeric
	default:
		return spec.LiteralDefaultString
	}
}

// catalogExtraTokens splits the EXTRA attribute string into lowercase word
// tokens so compound markers like "VIRTUAL GENERATED" and "DEFAULT_GENERATED"
// are matched exactly, never by substring bleed.
func catalogExtraTokens(extra string) map[string]bool {
	tokens := make(map[string]bool)
	for _, token := range strings.FieldsFunc(strings.ToLower(extra), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		tokens[token] = true
	}
	return tokens
}

func ptrExpressionFacts(facts spec.ExpressionFacts) *spec.ExpressionFacts {
	return &facts
}

func (p *Provider) loadIndexes(ctx context.Context, snapshot *spec.TableSnapshot, dialect spec.Dialect) error {
	// sub_part/collation are the bounded plain-key knownness facts: a prefix
	// length, a descending part, or a NULL column_name (expression key part)
	// means the index must not enter ordinary-key comparisons. Both columns
	// exist on every supported anchor; a server without them fails the query
	// and surfaces a load error rather than a guessed shape.
	//
	// IS_VISIBLE is version-gated: MySQL 5.7 has no such column (index
	// visibility landed in 8.0) while 8.0/8.4 and TiDB expose it. The
	// capability probe keeps 5.7 loading honestly — members there carry an
	// unknown Visibility instead of a guessed "visible" or a failed query.
	hasVisible, err := p.statisticsHasIsVisible(ctx)
	if err != nil {
		return err
	}
	visibleSelect := ""
	if hasVisible {
		visibleSelect = ", is_visible"
	}
	// EXPRESSION carries the catalog-stored functional key-part text (MySQL
	// 8.0.13+ and TiDB); MySQL 5.7 has no such column and functional parts
	// cannot exist there, so the probe keeps the row shape honest per
	// anchor instead of selecting a column that may not exist.
	hasExpression, err := p.statisticsHasExpression(ctx)
	if err != nil {
		return err
	}
	expressionSelect := ""
	if hasExpression {
		expressionSelect = ", expression"
	}
	rows, err := p.db.QueryContext(ctx, `
		select index_name, non_unique, index_type, column_name, cardinality,
			sub_part, collation`+visibleSelect+expressionSelect+`
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
		var name, indexType string
		var columnName, collation, visible, expression sql.NullString
		var nonUnique int
		var cardinality, subPart sql.NullInt64
		dest := []any{&name, &nonUnique, &indexType, &columnName, &cardinality, &subPart, &collation}
		if hasVisible {
			dest = append(dest, &visible)
		}
		if hasExpression {
			dest = append(dest, &expression)
		}
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan table index: %w", err)
		}

		var cardinalityValue *int64
		if cardinality.Valid {
			cardinalityValue = ptrInt64(cardinality.Int64)
		}
		accumulateIndexRow(indexes, &order, dialect, name, nonUnique, indexType,
			indexMemberRow{columnName: columnName, collation: collation, subPart: subPart, visible: visible, expression: expression},
			cardinalityValue, snapshot.Columns)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table indexes: %w", err)
	}

	for _, name := range order {
		index := indexes[name]
		index.SetKeyParts(index.KeyParts)
		if index.Kind == spec.IndexKindPrimary {
			snapshot.PrimaryKey = index
			continue
		}
		snapshot.Indexes = append(snapshot.Indexes, *index)
	}
	return nil
}

// loadForeignKeys loads the table's foreign-key members into
// snapshot.Constraints — the only constraint type the collection vouches
// for on the provider path (primary-key identity lives on PrimaryKey and
// unique keys on index members; statement-derived shapes may append
// primary_key members the parser itself proved). KEY_COLUMN_USAGE rows
// with a non-NULL referenced table are exactly the FK column pairs, in
// ORDINAL_POSITION order, so local and referenced columns fold into
// aligned ordered lists. REFERENTIAL_CONSTRAINTS supplies delete/update
// rules; a MATCH option other than NONE counts as an unmodeled refer
// action rather than silently dropping. A clean empty result is a proven
// empty set — never a withheld one — while query/scan failures propagate
// because a failed catalog read is not an answerable absence.
func (p *Provider) loadForeignKeys(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select k.constraint_name, k.column_name,
			k.referenced_table_schema, k.referenced_table_name, k.referenced_column_name,
			r.delete_rule, r.update_rule, r.match_option
		from information_schema.key_column_usage k
		left join information_schema.referential_constraints r
			on r.constraint_schema = k.constraint_schema
			and r.table_name = k.table_name
			and r.constraint_name = k.constraint_name
		where k.table_schema = ? and k.table_name = ?
			and k.referenced_table_name is not null
		order by k.constraint_name, k.ordinal_position
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table foreign keys: %w", err)
	}
	defer rows.Close()

	constraints := make([]spec.Constraint, 0)
	byName := make(map[string]int)
	for rows.Next() {
		var name, column, refSchema, refTable, refColumn string
		var deleteRule, updateRule, matchOption sql.NullString
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn,
			&deleteRule, &updateRule, &matchOption); err != nil {
			return fmt.Errorf("scan table foreign key: %w", err)
		}
		pos, seen := byName[name]
		if !seen {
			constraints = append(constraints, spec.Constraint{Type: "foreign_key", Name: name})
			pos = len(constraints) - 1
			byName[name] = pos
		}
		member := &constraints[pos]
		member.Columns = append(member.Columns, column)
		member.ReferencedSchema = refSchema
		member.ReferencedTable = refTable
		member.ReferencedColumns = append(member.ReferencedColumns, refColumn)
		if deleteRule.Valid {
			member.OnDelete = referActionName(deleteRule.String)
		}
		if updateRule.Valid {
			member.OnUpdate = referActionName(updateRule.String)
		}
		if matchOption.Valid && !strings.EqualFold(matchOption.String, "NONE") {
			member.UnmodeledReferActions++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table foreign keys: %w", err)
	}
	snapshot.Constraints = constraints
	return nil
}

// loadCheckConstraints loads the table's CHECK members into
// snapshot.Constraints after the foreign keys — name, enforcement state,
// and the parsed clause's structural facts (never the raw expression
// text). Two live catalog shapes exist, probed on the anchors:
//   - MySQL 8.0.16+: members sit in TABLE_CONSTRAINTS with ENFORCED; the
//     clause joins from CHECK_CONSTRAINTS on catalog+schema+name (that
//     table has no TABLE_NAME column on MySQL).
//   - TiDB v8.5.0: members sit only in TIDB_CHECK_CONSTRAINTS (which does
//     carry TABLE_NAME); TABLE_CONSTRAINTS stores no CHECK rows and no
//     catalog column exposes the enforcement bit — SHOW CREATE's
//     /*!80016 NOT ENFORCED */ marker is the only surface, so provider
//     members stay enforcement-unvouched on TiDB.
//
// MySQL 5.7 stores no CHECK members at all. A catalog without the
// relevant capability leaves member facts unproven rather than guessed.
// Query failures propagate — a failed catalog read is not an answerable
// absence.
func (p *Provider) loadCheckConstraints(ctx context.Context, snapshot *spec.TableSnapshot) error {
	hasTidb, err := p.tidbCheckConstraintsTableExists(ctx)
	if err != nil {
		return fmt.Errorf("probe tidb_check_constraints table: %w", err)
	}
	if hasTidb {
		return p.loadTiDBCheckConstraints(ctx, snapshot)
	}
	hasEnforced, err := p.tableConstraintsHasEnforced(ctx)
	if err != nil {
		return fmt.Errorf("probe table_constraints.enforced: %w", err)
	}
	hasClauses, err := p.checkConstraintsTableExists(ctx)
	if err != nil {
		return fmt.Errorf("probe check_constraints table: %w", err)
	}

	enforcedSel, clauseSel, join := "null", "null", ""
	if hasEnforced {
		enforcedSel = "tc.enforced"
	}
	if hasClauses {
		clauseSel = "cc.check_clause"
		join = `
		left join information_schema.check_constraints cc
			on cc.constraint_catalog = tc.constraint_catalog
			and cc.constraint_schema = tc.constraint_schema
			and cc.constraint_name = tc.constraint_name`
	}
	rows, err := p.db.QueryContext(ctx, `
		select tc.constraint_name, `+enforcedSel+`, `+clauseSel+`
		from information_schema.table_constraints tc`+join+`
		where tc.table_schema = ? and tc.table_name = ?
			and tc.constraint_type = 'CHECK'
		order by tc.constraint_name
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query table check constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var enforced, clause sql.NullString
		if err := rows.Scan(&name, &enforced, &clause); err != nil {
			return fmt.Errorf("scan table check constraint: %w", err)
		}
		member := spec.Constraint{Type: "check", Name: name}
		if enforced.Valid {
			member.Enforced = checkEnforcementName(enforced.String)
		}
		if clause.Valid {
			facts := tidbparser.ParseCatalogExpressionFacts(clause.String)
			member.Expression = &facts
			for _, ref := range facts.References {
				if !ref.Qualified() {
					member.Columns = append(member.Columns, ref.Name)
				}
			}
		}
		snapshot.Constraints = append(snapshot.Constraints, member)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table check constraints: %w", err)
	}

	return p.finishCheckConstraintLoad(ctx, snapshot)
}

// loadTiDBCheckConstraints loads CHECK members on the TiDB catalog shape:
// TIDB_CHECK_CONSTRAINTS carries TABLE_NAME and CHECK_CLAUSE but no
// enforcement column (verified on the v8.5.0 anchor — enforcement is only
// rendered inside SHOW CREATE output). Members therefore arrive with
// Enforced unvouched; ordered state transitions carry the flip knowledge.
func (p *Provider) loadTiDBCheckConstraints(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select constraint_name, check_clause
		from information_schema.tidb_check_constraints
		where constraint_schema = ? and table_name = ?
		order by constraint_name
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query tidb check constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return fmt.Errorf("scan tidb check constraint: %w", err)
		}
		member := spec.Constraint{Type: "check", Name: name}
		facts := tidbparser.ParseCatalogExpressionFacts(clause)
		member.Expression = &facts
		for _, ref := range facts.References {
			if !ref.Qualified() {
				member.Columns = append(member.Columns, ref.Name)
			}
		}
		snapshot.Constraints = append(snapshot.Constraints, member)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tidb check constraints: %w", err)
	}
	return p.finishCheckConstraintLoad(ctx, snapshot)
}

// finishCheckConstraintLoad applies the shared tail of
// loadCheckConstraints: the collection vouches for foreign keys, checks,
// and statement-proven primary_key members — primary keys live on
// PrimaryKey and unique keys on index members. Any other
// TABLE_CONSTRAINTS type is a member this projection cannot represent, so
// its presence unvouches the collection rather than letting a partial
// answer pose as complete.
func (p *Provider) finishCheckConstraintLoad(ctx context.Context, snapshot *spec.TableSnapshot) error {
	var unmodeled int
	err := p.db.QueryRowContext(ctx, `
		select count(*)
		from information_schema.table_constraints
		where table_schema = ? and table_name = ?
			and constraint_type not in ('PRIMARY KEY', 'UNIQUE', 'FOREIGN KEY', 'CHECK')
	`, snapshot.Schema, snapshot.Table.Name).Scan(&unmodeled)
	if err != nil {
		return fmt.Errorf("query table unmodeled constraint types: %w", err)
	}
	if unmodeled > 0 {
		snapshot.ConstraintsUnknown = true
	}
	return nil
}

// tidbCheckConstraintsTableExists reports whether this server's
// information_schema exposes the TiDB-specific TIDB_CHECK_CONSTRAINTS
// member table (the only catalog with TABLE_NAME attribution for checks
// on TiDB; absent on MySQL).
func (p *Provider) tidbCheckConstraintsTableExists(ctx context.Context) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`tables`"+`
		where table_schema = 'information_schema'
			and table_name = 'TIDB_CHECK_CONSTRAINTS'
	`).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return n > 0, nil
}

// checkEnforcementName normalizes the catalog ENFORCED token into the
// constraint vocabulary — an unrecognized value stays empty rather than a
// guessed fact.
func checkEnforcementName(token string) string {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "YES":
		return "enforced"
	case "NO":
		return "not_enforced"
	default:
		return ""
	}
}

// tableConstraintsHasEnforced reports whether this server's
// information_schema.TABLE_CONSTRAINTS exposes the ENFORCED column
// (MySQL 8.0.16+, TiDB; absent on MySQL 5.7). The probe queries the
// catalog itself so the answer reflects the connected server, not a
// version guess.
func (p *Provider) tableConstraintsHasEnforced(ctx context.Context) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`columns`"+`
		where table_schema = 'information_schema'
			and table_name = 'TABLE_CONSTRAINTS'
			and column_name = 'ENFORCED'
	`).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return n > 0, nil
}

// checkConstraintsTableExists reports whether this server's
// information_schema exposes the CHECK_CONSTRAINTS clause table (MySQL
// 8.0.16+, TiDB; absent on MySQL 5.7 where CHECK syntax parses but is
// ignored).
func (p *Provider) checkConstraintsTableExists(ctx context.Context) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`tables`"+`
		where table_schema = 'information_schema'
			and table_name = 'CHECK_CONSTRAINTS'
	`).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return n > 0, nil
}

// referActionName normalizes a catalog DELETE_RULE/UPDATE_RULE token into
// the same lowercase vocabulary parser-extracted referential actions use.
// An unrecognized token stays an empty string rather than a guessed fact.
func referActionName(rule string) string {
	switch strings.ToUpper(strings.TrimSpace(rule)) {
	case "RESTRICT":
		return "restrict"
	case "CASCADE":
		return "cascade"
	case "SET NULL":
		return "set_null"
	case "NO ACTION":
		return "no_action"
	case "SET DEFAULT":
		return "set_default"
	default:
		return ""
	}
}

// ResolveObject implements the ObjectResolver seam for non-table catalog
// objects. T17 resolves schema identities through
// information_schema.SCHEMATA: existence, default charset/collation, the
// DEFAULT_ENCRYPTION column (MySQL 8.0.16+, capability-probed), and TiDB's
// TIDB_PLACEMENT_POLICY_NAME binding. Every other object type reports
// unavailable — the resolver never fabricates coverage it does not query.
func (p *Provider) ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	snapshot := &spec.ObjectSnapshot{
		Schema: request.Schema,
		Type:   request.Type,
		Name:   request.Name,
		Status: spec.MetadataStatusUnavailable,
	}
	if request.Type != "schema" || strings.TrimSpace(request.Name) == "" {
		return snapshot, nil
	}
	return p.loadSchemaObject(ctx, dialect, request.Name, snapshot)
}

func (p *Provider) loadSchemaObject(ctx context.Context, dialect spec.Dialect, name string, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	row := p.db.QueryRowContext(ctx, `
		select schema_name, default_character_set_name, default_collation_name
		from information_schema.schemata
		where schema_name = ?
	`, name)
	var schemaName, charset, collation sql.NullString
	if err := row.Scan(&schemaName, &charset, &collation); err != nil {
		if err == sql.ErrNoRows {
			snapshot.Status = spec.MetadataStatusNotFound
			return snapshot, nil
		}
		return nil, fmt.Errorf("query schema snapshot %s: %w", name, err)
	}
	snapshot.Status = spec.MetadataStatusConfirmed
	snapshot.Exists = true
	snapshot.Schema = schemaName.String
	snapshot.Attributes = map[string]string{}
	if charset.Valid && charset.String != "" {
		snapshot.Attributes["charset"] = charset.String
	}
	if collation.Valid && collation.String != "" {
		snapshot.Attributes["collation"] = collation.String
	}
	if has, err := p.schemataHasColumn(ctx, "DEFAULT_ENCRYPTION"); err != nil {
		return nil, err
	} else if has {
		var encryption sql.NullString
		err = p.db.QueryRowContext(ctx, `
			select default_encryption
			from information_schema.schemata
			where schema_name = ?
		`, name).Scan(&encryption)
		if err != nil {
			return nil, fmt.Errorf("query schema encryption %s: %w", name, err)
		}
		if encryption.Valid && encryption.String != "" {
			snapshot.Attributes["encryption"] = strings.ToLower(encryption.String)
		}
	}
	if dialect == spec.DialectTiDB {
		if has, err := p.schemataHasColumn(ctx, "TIDB_PLACEMENT_POLICY_NAME"); err != nil {
			return nil, err
		} else if has {
			var policy sql.NullString
			err = p.db.QueryRowContext(ctx, `
				select tidb_placement_policy_name
				from information_schema.schemata
				where schema_name = ?
			`, name).Scan(&policy)
			if err != nil {
				return nil, fmt.Errorf("query schema placement policy %s: %w", name, err)
			}
			if policy.Valid && policy.String != "" {
				snapshot.Attributes["placement_policy"] = policy.String
			}
		}
	}
	return snapshot, nil
}

// schemataHasColumn reports whether this server's information_schema.SCHEMATA
// exposes the named column (DEFAULT_ENCRYPTION on MySQL 8.0.16+,
// TIDB_PLACEMENT_POLICY_NAME on TiDB; neither on MySQL 5.7). The answer comes
// from the catalog itself, not a version guess.
func (p *Provider) schemataHasColumn(ctx context.Context, column string) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`columns`"+`
		where table_schema = 'information_schema'
			and table_name = 'SCHEMATA'
			and column_name = ?
	`, column).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("probe schemata column %s: %w", column, err)
	}
	return n > 0, nil
}

// statisticsHasIsVisible reports whether this server's
// information_schema.STATISTICS exposes the IS_VISIBLE column (MySQL 8.0+,
// TiDB; absent on MySQL 5.7). The probe queries the catalog itself so the
// answer reflects the connected server, not a version guess.
func (p *Provider) statisticsHasIsVisible(ctx context.Context) (bool, error) {
	var n int
	// The backtick-quoted catalog table keeps this probe distinct from the
	// member-row SELECT against the same schema.
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`columns`"+`
		where table_schema = 'information_schema'
			and table_name = 'STATISTICS'
			and column_name = 'IS_VISIBLE'
	`).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("probe statistics is_visible: %w", err)
	}
	return n > 0, nil
}

// statisticsHasExpression reports whether this server's
// information_schema.STATISTICS exposes the EXPRESSION column (MySQL
// 8.0.13+, TiDB; absent on 5.7). Like the IS_VISIBLE probe, the answer
// comes from the catalog itself.
func (p *Provider) statisticsHasExpression(ctx context.Context) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`columns`"+`
		where table_schema = 'information_schema'
			and table_name = 'STATISTICS'
			and column_name = 'EXPRESSION'
	`).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("probe statistics expression: %w", err)
	}
	return n > 0, nil
}

// indexMemberRow carries one STATISTICS member row's plain-key knownness
// facts: which column it names, its sort direction, and its prefix length.
// A NULL column_name marks an expression key part; a NULL collation on a
// named member means the row is not classifiable as a plain ascending part.
// visible carries the index-level IS_VISIBLE fact (constant across the
// index's rows); a NULL/unsupported value leaves Visibility unknown.
// expression carries the catalog-stored functional key-part text when the
// server exposes it.
type indexMemberRow struct {
	columnName sql.NullString
	collation  sql.NullString
	subPart    sql.NullInt64
	visible    sql.NullString
	expression sql.NullString
}

func accumulateIndexRow(indexes map[string]*spec.Index, order *[]string, dialect spec.Dialect, name string, nonUnique int, indexType string, member indexMemberRow, cardinality *int64, columns []spec.Column) {
	index, ok := indexes[name]
	if !ok {
		index = &spec.Index{
			Name: name,
			Kind: classifyIndex(dialect, name, nonUnique, indexType),
		}
		indexes[name] = index
		*order = append(*order, name)
	}
	if index.Visibility == "" {
		switch {
		case strings.EqualFold(member.visible.String, "yes"):
			index.Visibility = spec.ColumnVisibilityVisible
		case member.visible.Valid && strings.EqualFold(member.visible.String, "no"):
			index.Visibility = spec.ColumnVisibilityInvisible
		}
	}

	// STATISTICS emits rows in seq_in_index order inside each index group,
	// so the accumulated slice position is the member ordinal.
	keyPart := spec.IndexKeyPart{Ordinal: len(index.KeyParts) + 1}
	switch {
	case !member.columnName.Valid:
		// A SQL NULL column_name is an expression key part — the catalog
		// stores no column name for it. The catalog EXPRESSION text
		// supplies the member identity and column dependencies when the
		// server exposes it.
		classifyExpressionMember(&keyPart, member)
	case strings.EqualFold(member.columnName.String, "NULL"):
		// TiDB emits the literal text 'NULL' for an expression member
		// while MySQL emits a SQL NULL — but a real column can carry the
		// backtick-quoted reserved name too (`` `NULL` INT ``), so the
		// literal alone cannot classify the row. Decide on verifiable
		// catalog facts: expression text present → expression member;
		// the loaded column set vouches the named column → plain column;
		// neither provable → unmodeled rather than a guessed either-way.
		switch {
		case member.expression.Valid && strings.TrimSpace(member.expression.String) != "":
			classifyExpressionMember(&keyPart, member)
		case catalogColumnExists(columns, member.columnName.String):
			classifyColumnMember(&keyPart, index, member)
		default:
			keyPart.Kind = spec.IndexKeyPartUnmodeled
		}
	default:
		classifyColumnMember(&keyPart, index, member)
	}
	index.KeyParts = append(index.KeyParts, keyPart)
	if cardinality == nil {
		return
	}
	if index.Cardinality == nil || *cardinality > *index.Cardinality {
		index.Cardinality = ptrInt64(*cardinality)
	}
}

// classifyExpressionMember marks the member an expression key part and
// attaches the catalog-stored functional text when the server exposes it —
// the EXPRESSION column is authoritative for identity and dependencies.
// The catalog records sort direction on functional key parts too (MySQL
// 8.0.46 functional_index.result L186-193 proves descending functional
// indexes are real objects), so the member reads the same collation fact
// the column branch does: 'D' keeps descending, 'A' stays ascending, and
// an unproven or unrecognized direction keeps the expression identity but
// marks DirectionUnknown — never demoted to unmodeled, which would lose
// the member's identity and dependency facts.
func classifyExpressionMember(keyPart *spec.IndexKeyPart, member indexMemberRow) {
	switch {
	case strings.EqualFold(member.collation.String, "D"):
		keyPart.Kind = spec.IndexKeyPartExpression
		keyPart.Descending = true
	case member.collation.Valid && strings.EqualFold(member.collation.String, "A"):
		keyPart.Kind = spec.IndexKeyPartExpression
	default:
		// A NULL or unrecognized COLLATION withholds only the direction
		// answer — the member is still provably an expression part.
		keyPart.Kind = spec.IndexKeyPartExpression
		keyPart.DirectionUnknown = true
	}
	if member.expression.Valid && strings.TrimSpace(member.expression.String) != "" {
		keyPart.Expression = ptrExpressionFacts(tidbparser.ParseCatalogExpressionFacts(member.expression.String))
		keyPart.MultiValued = tidbparser.CatalogExpressionIsMultiValued(member.expression.String)
		if keyPart.MultiValued {
			keyPart.MultiValuedCastType, keyPart.MultiValuedCastLengthSet = tidbparser.CatalogMultiValuedCastFacts(member.expression.String)
		}
	}
}

// classifyColumnMember marks the member a plain column key part, carrying
// its authored prefix length and catalog sort direction. A named member
// whose collation the row cannot prove stays unmodeled.
func classifyColumnMember(keyPart *spec.IndexKeyPart, index *spec.Index, member indexMemberRow) {
	keyPart.Kind = spec.IndexKeyPartColumn
	keyPart.Column = member.columnName.String
	index.Columns = append(index.Columns, member.columnName.String)
	if member.subPart.Valid {
		length := int(member.subPart.Int64)
		keyPart.PrefixLength = &length
	}
	if !member.collation.Valid {
		// A named member with no collation (e.g. spatial rows) cannot
		// be classified as a plain ascending part.
		keyPart.Kind = spec.IndexKeyPartUnmodeled
	} else if strings.EqualFold(member.collation.String, "D") {
		keyPart.Descending = true
	} else if !strings.EqualFold(member.collation.String, "A") {
		keyPart.Kind = spec.IndexKeyPartUnmodeled
	}
}

// catalogColumnExists reports whether the loaded column set vouches for a
// member's column name — the fact a literal-'NULL' member row needs to
// stay a plain column instead of being read as an expression marker.
func catalogColumnExists(columns []spec.Column, name string) bool {
	for i := range columns {
		if strings.EqualFold(columns[i].Name, name) {
			return true
		}
	}
	return false
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

// faithfulCatalogTypes are the T06-A17 target bases whose COLUMN_TYPE
// parameter text — DECIMAL/FLOAT/DOUBLE second parameters and ENUM/SET member
// literals — is preserved verbatim. Every other type keeps the established
// base-keyword contract.
var faithfulCatalogTypes = map[string]bool{
	"decimal": true, "float": true, "double": true, "enum": true, "set": true,
}

// typeParameterSpan locates the parenthesized parameter list of a catalog
// COLUMN_TYPE, honoring single-quoted members with ” escapes: a comma or
// right paren inside a member never ends the span. Returns (-1, -1) without a
// parameter list; the end index is exclusive so columnType[start:end] is the
// whole "(...)" segment.
func typeParameterSpan(columnType string) (start, end int) {
	start = strings.IndexByte(columnType, '(')
	if start < 0 {
		return -1, -1
	}
	inString := false
	for i := start + 1; i < len(columnType); i++ {
		switch {
		case inString:
			if columnType[i] == '\'' {
				if i+1 < len(columnType) && columnType[i+1] == '\'' {
					i++
					continue
				}
				inString = false
			}
		case columnType[i] == '\'':
			inString = true
		case columnType[i] == ')':
			return start, i + 1
		}
	}
	return start, len(columnType)
}

// stripQuotedLiterals removes '...' member literals (including ” escapes) so
// attribute detection never reads inside member text: 'x unsigned' inside an
// ENUM member is a value, never the UNSIGNED attribute.
func stripQuotedLiterals(s string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				inString = false
			}
			continue
		}
		if c == '\'' {
			inString = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// parseColumnType normalizes a catalog COLUMN_TYPE. Length keeps its
// established first-integer meaning; unsigned is read only outside quoted
// member literals; typeText preserves the verbatim parameter span for the
// five faithfulCatalogTypes bases and stays the bare base keyword otherwise.
func parseColumnType(columnType string) (typeText string, length int, unsigned bool) {
	trimmed := strings.TrimSpace(columnType)
	lower := strings.ToLower(trimmed)
	unsigned = strings.Contains(strings.ToLower(stripQuotedLiterals(trimmed)), " unsigned")
	start, end := typeParameterSpan(trimmed)
	var baseType string
	if start >= 0 {
		baseType = strings.TrimSpace(lower[:start])
		part := trimmed[start+1 : end-1]
		if comma := strings.Index(part, ","); comma >= 0 {
			part = part[:comma]
		}
		if parsed, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			length = parsed
		}
	} else if fields := strings.Fields(lower); len(fields) > 0 {
		baseType = fields[0]
	}
	if !faithfulCatalogTypes[baseType] {
		return baseType, length, unsigned
	}
	var text strings.Builder
	text.WriteString(baseType)
	if start >= 0 {
		text.WriteString(trimmed[start:end])             // verbatim parameters, member case intact
		text.WriteString(strings.ToLower(trimmed[end:])) // attribute keywords after the span
	} else if rest := trimmed[len(baseType):]; rest != "" {
		// No parameter span (e.g. bare "float unsigned"): keep the declared
		// attribute text, keyword-normalized.
		text.WriteString(strings.ToLower(rest))
	}
	return text.String(), length, unsigned
}

func classifyIndex(dialect spec.Dialect, name string, nonUnique int, indexType string) spec.IndexKind {
	// MySQL reserves `primary` for the PK index object outright — a member
	// can never own the name (1280), so the catalog name promotes safely.
	// TiDB lets a secondary literally own `primary` (RENAME … TO `primary`
	// is legal on PK-less and PKIsHandle tables), so the name alone cannot
	// promote — resolveTiDBPrimaryName binds the PK identity under the
	// table's proven pk-presence fact instead.
	if dialect == spec.DialectMySQL && strings.EqualFold(name, "primary") {
		return spec.IndexKindPrimary
	}
	if strings.EqualFold(indexType, "fulltext") {
		return spec.IndexKindFulltext
	}
	if strings.EqualFold(indexType, "spatial") {
		return spec.IndexKindSpatial
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
