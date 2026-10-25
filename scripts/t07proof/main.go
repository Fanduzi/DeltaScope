// Package main collects bounded T07 source/catalog/provider fact evidence.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
	gomysql "github.com/go-sql-driver/mysql"
)

const fixtureTable = "golden.t07_column_facts"
const countSQL = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='t07_column_facts'"
const dropSQL = "DROP TABLE golden.t07_column_facts"
const catalogSQL = "SELECT column_name,ordinal_position,data_type,column_type,is_nullable,column_default,extra,generation_expression,HEX(column_default) AS default_hex,HEX(generation_expression) AS generation_hex,character_set_name,collation_name FROM information_schema.columns WHERE table_schema='golden' AND table_name='t07_column_facts' ORDER BY ordinal_position"
const sessionSQL = "SELECT VERSION() AS version,@@sql_mode AS sql_mode,@@character_set_client AS character_set_client,@@character_set_connection AS character_set_connection,@@collation_connection AS collation_connection"
const uuidSQL = "ALTER TABLE golden.t07_column_facts ALTER COLUMN uuid_c SET DEFAULT (UUID())"

var fixedVersions = map[string]string{"mysql57": "5.7.44", "mysql80": "8.0.46", "mysql84": "8.4.10", "tidb85": "8.5.0"}
var fixedPorts = map[string]string{"mysql57": "23357", "mysql80": "23380", "mysql84": "23384", "tidb85": "24000"}

type expectedColumn struct {
	name        string
	storage     string
	genRefs     []string
	defaultExpr bool
	defaultRefs []string
	invisible   bool
}

type columnFact struct {
	Name               string                 `json:"name"`
	Type               string                 `json:"type"`
	HasDefault         bool                   `json:"has_default"`
	DefaultKind        string                 `json:"default_kind"`
	DefaultIsNull      bool                   `json:"default_is_null"`
	DefaultValueEmpty  bool                   `json:"default_value_empty"`
	DefaultQuotedEmpty bool                   `json:"default_quoted_empty"`
	Generated          bool                   `json:"generated"`
	Storage            string                 `json:"storage"`
	GenerationKnown    bool                   `json:"generation_known"`
	GenerationRefs     []spec.ColumnReference `json:"generation_references"`
	DefaultExpression  bool                   `json:"default_expression"`
	DefaultKnown       bool                   `json:"default_known"`
	DefaultRefs        []spec.ColumnReference `json:"default_references"`
	Visibility         string                 `json:"visibility"`
	VisibilityExplicit bool                   `json:"visibility_explicit"`
}

func fixedFixture(anchor string) (string, []expectedColumn) {
	columns := "id BIGINT PRIMARY KEY,a INT,stored_c INT AS(a+1) STORED,virtual_c INT AS(a+1) VIRTUAL,literal_c VARCHAR(8) AS('a') VIRTUAL,nullable_c INT DEFAULT NULL,ordinary_c VARCHAR(8) DEFAULT '',uuid_c VARCHAR(36)"
	want := []expectedColumn{{name: "id"}, {name: "a"}, {name: "stored_c", storage: "stored", genRefs: []string{"a"}}, {name: "virtual_c", storage: "virtual", genRefs: []string{"a"}}, {name: "literal_c", storage: "virtual"}, {name: "nullable_c"}, {name: "ordinary_c"}, {name: "uuid_c"}}
	if anchor != "mysql57" {
		want[7].defaultExpr = true
	}
	if anchor == "mysql80" || anchor == "mysql84" {
		columns += ",expr_c INT DEFAULT(a+1),json_c JSON DEFAULT(JSON_OBJECT('k',1)),hidden_c INT INVISIBLE,hidden_gen INT AS(a+1) STORED INVISIBLE"
		want = append(want, expectedColumn{name: "expr_c", defaultExpr: true, defaultRefs: []string{"a"}}, expectedColumn{name: "json_c", defaultExpr: true}, expectedColumn{name: "hidden_c", invisible: true}, expectedColumn{name: "hidden_gen", storage: "stored", genRefs: []string{"a"}, invisible: true})
	} else if anchor == "tidb85" {
		columns += ",json_c JSON DEFAULT(JSON_OBJECT('k',1))"
		want = append(want, expectedColumn{name: "json_c", defaultExpr: true})
	}
	return "CREATE TABLE " + fixtureTable + " (" + columns + ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin", want
}

func project(columns []spec.Column) []columnFact {
	out := make([]columnFact, 0, len(columns))
	for _, column := range columns {
		fact := columnFact{Name: column.Name, Type: column.Type, HasDefault: column.HasDefault, DefaultKind: column.DefaultKind, DefaultIsNull: column.DefaultIsNull, DefaultValueEmpty: column.DefaultValue == "", DefaultQuotedEmpty: column.DefaultValue == "''", Visibility: column.Visibility, VisibilityExplicit: column.VisibilityExplicit}
		if column.Generated != nil {
			fact.Generated = true
			fact.Storage = column.Generated.Storage
			fact.GenerationKnown = column.Generated.Expression.Known
			fact.GenerationRefs = append([]spec.ColumnReference{}, column.Generated.Expression.References...)
		}
		if column.DefaultExpression != nil {
			fact.DefaultExpression = true
			fact.DefaultKnown = column.DefaultExpression.Known
			fact.DefaultRefs = append([]spec.ColumnReference{}, column.DefaultExpression.References...)
		}
		out = append(out, fact)
	}
	return out
}

func localRefs(actual []spec.ColumnReference, names []string) bool {
	if len(actual) != len(names) {
		return false
	}
	for i, name := range names {
		if actual[i].Schema != "" || actual[i].Table != "" || actual[i].Name != name {
			return false
		}
	}
	return true
}

func typeForms(name string) []string {
	switch name {
	case "id":
		return []string{"bigint", "bigint(20)"}
	case "literal_c", "ordinary_c":
		return []string{"varchar(8)"}
	case "uuid_c":
		return []string{"varchar(36)"}
	case "json_c":
		return []string{"json"}
	default:
		return []string{"int", "int(11)"}
	}
}

func checkFacts(actual []columnFact, want []expectedColumn, catalog bool) error {
	if len(actual) != len(want) {
		return fmt.Errorf("complete column count: got %d want %d", len(actual), len(want))
	}
	seen := make(map[string]bool, len(actual))
	for i, expected := range want {
		fact := actual[i]
		if fact.Name != expected.name || fact.Name == "" || seen[fact.Name] {
			return fmt.Errorf("complete column identity/order at %d", i)
		}
		seen[fact.Name] = true
		typeOK := false
		for _, form := range typeForms(expected.name) {
			typeOK = typeOK || fact.Type == form
		}
		if !typeOK {
			return fmt.Errorf("column type fidelity for %s", expected.name)
		}
		if fact.Generated != (expected.storage != "") || fact.Storage != expected.storage || fact.Generated && (!fact.GenerationKnown || !localRefs(fact.GenerationRefs, expected.genRefs)) {
			return fmt.Errorf("generated facts for %s", expected.name)
		}
		if fact.DefaultExpression != expected.defaultExpr || fact.DefaultExpression && (!fact.HasDefault || !fact.DefaultKnown || !localRefs(fact.DefaultRefs, expected.defaultRefs)) {
			return fmt.Errorf("default expression facts for %s", expected.name)
		}
		visibility := "visible"
		if expected.invisible {
			visibility = "invisible"
		}
		if fact.Visibility != visibility || catalog && fact.VisibilityExplicit {
			return fmt.Errorf("effective visibility/provenance for %s", expected.name)
		}
	}
	if !actual[6].HasDefault || catalog && !actual[6].DefaultValueEmpty || !catalog && !actual[6].DefaultQuotedEmpty {
		return errors.New("empty-string literal must retain source-aware HasDefault/value identity")
	}
	return nil
}

func writeJSON(dir, name string, value any) (err error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create evidence %s: %w", name, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	data = append(data, '\n')
	written, err := file.Write(data)
	if err == nil && written != len(data) {
		err = errors.New("short evidence write")
	}
	return err
}

func queryRows(ctx context.Context, db *sql.DB, query string) (out []map[string]any, err error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		values := make([]sql.RawBytes, len(names))
		args := make([]any, len(names))
		for i := range values {
			args[i] = &values[i]
		}
		if err := rows.Scan(args...); err != nil {
			return out, err
		}
		record := make(map[string]any, len(names))
		for i, name := range names {
			if values[i] == nil {
				record[name] = nil
			} else {
				record[name] = string(values[i])
			}
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func value(record map[string]any, name string) any {
	for key, v := range record {
		if strings.EqualFold(key, name) {
			return v
		}
	}
	return nil
}

func checkCatalog(records []map[string]any, want []expectedColumn) error {
	if len(records) != len(want) {
		return errors.New("incomplete catalog column set")
	}
	for i, expected := range want {
		record := records[i]
		ordinal, err := strconv.Atoi(fmt.Sprint(value(record, "ordinal_position")))
		if err != nil || ordinal != i+1 || value(record, "column_name") != expected.name {
			return fmt.Errorf("catalog identity/order at %d", i)
		}
		family := strings.Split(typeForms(expected.name)[0], "(")[0]
		if value(record, "data_type") != family {
			return fmt.Errorf("catalog type family for %s", expected.name)
		}
		extra := strings.ToLower(fmt.Sprint(value(record, "extra")))
		attributes := []string{}
		if expected.storage != "" {
			attributes = append(attributes, expected.storage+" generated")
			if text, ok := value(record, "generation_expression").(string); !ok || text == "" {
				return fmt.Errorf("missing catalog generation expression for %s", expected.name)
			}
		}
		if expected.defaultExpr {
			attributes = append(attributes, "default_generated")
			if text, ok := value(record, "column_default").(string); !ok || text == "" {
				return fmt.Errorf("missing catalog default expression for %s", expected.name)
			}
		}
		if expected.invisible {
			attributes = append(attributes, "invisible")
		}
		if extra != strings.Join(attributes, " ") {
			return fmt.Errorf("catalog attribute identity for %s", expected.name)
		}
	}
	return nil
}

func tableCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, countSQL).Scan(&count)
	return count, err
}

func projection(ctx context.Context, db *sql.DB, query string) (names []string, err error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	names, err = rows.Columns()
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, errors.New("LIMIT 0 returned a data row")
	}
	return names, rows.Err()
}

func collect(ctx context.Context, db *sql.DB, dir, anchor string, ledger *queryLedger) (err error) {
	var pre, post *int
	owned := false
	defer func() {
		primary := err
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var cleanupErr error
		if owned {
			_, cleanupErr = db.ExecContext(cleanupCtx, dropSQL)
		}
		count, countErr := tableCount(cleanupCtx, db)
		if countErr == nil {
			post = &count
			if owned && count != 0 {
				countErr = fmt.Errorf("owned cleanup final COUNT=%d", count)
			}
		}
		cleanupErr = errors.Join(cleanupErr, countErr)
		err = errors.Join(err, cleanupErr)
		lifecycle := map[string]any{"created_by_probe": owned, "pre_count": pre, "post_count": post, "primary_error": errorText(primary), "cleanup_error": errorText(cleanupErr), "complete": err == nil}
		err = errors.Join(err, writeJSON(dir, "lifecycle.json", lifecycle), writeJSON(dir, "query-ledger.json", ledger.snapshot()))
	}()
	count, err := tableCount(ctx, db)
	if err != nil {
		return fmt.Errorf("prior COUNT query/scan: %w", err)
	}
	pre = &count
	if count != 0 {
		return fmt.Errorf("pre-existing fixed fixture count=%d; refusing create or DROP", count)
	}
	createSQL, want := fixedFixture(anchor)
	dialect := spec.DialectMySQL
	if anchor == "tidb85" {
		dialect = spec.DialectTiDB
	}
	parsed, err := appaudit.Parse(ctx, createSQL, dialect)
	if err != nil {
		return fmt.Errorf("real declaration parse: %w", err)
	}
	statements, err := appaudit.Extract(ctx, parsed)
	if err != nil || len(statements) != 1 || statements[0].DDL == nil {
		return errors.Join(err, errors.New("expected one real declaration"))
	}
	declaredWant := append([]expectedColumn{}, want...)
	declaredWant[7].defaultExpr = false
	declaration := project(statements[0].DDL.Columns)
	if err := writeJSON(dir, "declaration-facts.json", declaration); err != nil {
		return err
	}
	if err := checkFacts(declaration, declaredWant, false); err != nil {
		return fmt.Errorf("declaration: %w", err)
	}
	if err := writeJSON(dir, "fixed-sql.json", map[string]any{"create": createSQL, "alter_uuid": anchor != "mysql57", "uuid_sql": uuidSQL}); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("fixed CREATE: %w", err)
	}
	owned = true
	createdCount, err := tableCount(ctx, db)
	if err != nil || createdCount != 1 {
		return errors.Join(err, errors.New("intermediate CREATE COUNT must be one"))
	}
	initialCatalog, err := queryRows(ctx, db, catalogSQL)
	if err != nil {
		return fmt.Errorf("intermediate CREATE catalog: %w", err)
	}
	if err := errors.Join(writeJSON(dir, "catalog-after-create.json", initialCatalog), checkCatalog(initialCatalog, declaredWant)); err != nil {
		return err
	}
	if anchor != "mysql57" {
		if _, err := db.ExecContext(ctx, uuidSQL); err != nil {
			return fmt.Errorf("fixed UUID setter: %w", err)
		}
		alteredCount, err := tableCount(ctx, db)
		if err != nil || alteredCount != 1 {
			return errors.Join(err, errors.New("intermediate UUID setter COUNT must be one"))
		}
	}
	catalog, err := queryRows(ctx, db, catalogSQL)
	if err != nil {
		return fmt.Errorf("catalog query/scan/iteration: %w", err)
	}
	if err := writeJSON(dir, "catalog-raw.json", catalog); err != nil {
		return err
	}
	if err := checkCatalog(catalog, want); err != nil {
		return err
	}
	ledger.add("provider", "LoadTableSnapshot golden.t07_column_facts")
	snapshot, err := mysqlmeta.NewProvider(db).LoadTableSnapshot(ctx, dialect, "golden", "t07_column_facts")
	if err != nil {
		return fmt.Errorf("real provider: %w", err)
	}
	if !snapshot.Exists {
		return errors.New("real provider withheld existing fixture")
	}
	facts := project(snapshot.Columns)
	if err := errors.Join(writeJSON(dir, "provider-snapshot.json", snapshot), writeJSON(dir, "provider-facts.json", facts)); err != nil {
		return err
	}
	if err := checkFacts(facts, want, true); err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	all, err := projection(ctx, db, "SELECT * FROM "+fixtureTable+" LIMIT 0")
	if err != nil {
		return err
	}
	wantVisible := make([]string, 0, len(want))
	for _, column := range want {
		if !column.invisible {
			wantVisible = append(wantVisible, column.name)
		}
	}
	if strings.Join(all, "\x00") != strings.Join(wantVisible, "\x00") {
		return errors.New("complete LIMIT-0 visible field set differs")
	}
	visible := map[string]any{"select_star": all}
	if anchor == "mysql80" || anchor == "mysql84" {
		explicit, err := projection(ctx, db, "SELECT hidden_c,hidden_gen FROM "+fixtureTable+" LIMIT 0")
		if err != nil {
			return err
		}
		if strings.Join(explicit, "\x00") != "hidden_c\x00hidden_gen" {
			return errors.New("explicit invisible-column identity differs")
		}
		visible["explicit_hidden"] = explicit
	}
	return errors.Join(writeJSON(dir, "projection-fields.json", visible), writeJSON(dir, "summary.json", map[string]any{"anchor": anchor, "columns": len(want), "catalog_rows": len(catalog), "provider_columns": len(facts), "provider_loads": 1, "known_fact_checks": true, "note": "supplemental fixed source/provider facts, not Golden business cases"}))
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func run() (err error) {
	if len(os.Args) != 3 || fixedVersions[os.Args[2]] == "" {
		return errors.New("usage: t07-proof <new-output-root> <mysql57|mysql80|mysql84|tidb85>")
	}
	anchor := os.Args[2]
	if os.Getenv("T07_USER") == "" {
		return errors.New("T07_USER must be provided by environment")
	}
	if err := os.MkdirAll(os.Args[1], 0755); err != nil {
		return err
	}
	dir := filepath.Join(os.Args[1], anchor)
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	cfg := gomysql.NewConfig()
	cfg.Net, cfg.Addr = "tcp", "127.0.0.1:"+fixedPorts[anchor]
	cfg.User, cfg.Passwd = os.Getenv("T07_USER"), os.Getenv("T07_PASS")
	cfg.Timeout = 20 * time.Second
	connector, err := gomysql.NewConnector(cfg)
	if err != nil {
		return err
	}
	ledger := &queryLedger{}
	db := sql.OpenDB(&ledgerConnector{inner: connector, ledger: ledger})
	db.SetMaxOpenConns(1)
	defer func() { err = errors.Join(err, db.Close()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	session, err := queryRows(ctx, db, sessionSQL)
	if err != nil || len(session) != 1 {
		return errors.Join(err, errors.New("session query requires one complete row"))
	}
	banner, ok := value(session[0], "version").(string)
	if !ok {
		return errors.New("session version is absent")
	}
	observed := spec.ParseObservedVersion(banner)
	product := spec.VersionProductMySQL
	if anchor == "tidb85" {
		product = spec.VersionProductTiDB
	}
	if observed == nil || observed.Product != product || observed.Version != fixedVersions[anchor] {
		return errors.New("server identity does not match fixed anchor")
	}
	if err := writeJSON(dir, "session.json", session); err != nil {
		return err
	}
	if err := collect(ctx, db, dir, anchor, ledger); err != nil {
		return err
	}
	fmt.Printf("collected fixed T07 source/provider facts: %s\n", anchor)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
