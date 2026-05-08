//go:build postgresql

package audit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type censusStatus string

const (
	statusFindingCovered   censusStatus = "finding-covered"
	statusNormalizedSilent censusStatus = "normalized-silent-pass"
	statusUnsupportedExp   censusStatus = "unsupported-explicit"
	statusParserError      censusStatus = "parser-error"
	statusUnclassified     censusStatus = "unclassified"
	statusDefer            censusStatus = "defer"
)

type censusCase struct {
	Name        string
	SQL         string
	DeferReason string
}

type censusResult struct {
	Name            string
	ParseOK         bool
	StmtCount       int
	Kind            string
	Unsupported     bool
	UnsupportedFeat string
	UnsupportedWhy  string
	Normalized      bool
	DDLOperation    string
	AlterActions    []string
	StmtFindings    []string
	GlobalFindings  []string
	Status          censusStatus
	CorpusCovered   bool
}

var pgDDLCensusCases = []censusCase{
	// --- CREATE TABLE ---
	{Name: "CREATE TABLE basic", SQL: "CREATE TABLE users (id bigint NOT NULL, name text)"},
	{Name: "CREATE TABLE with primary key", SQL: "CREATE TABLE users (id bigint PRIMARY KEY, name text)"},
	{Name: "CREATE TABLE with unique constraint", SQL: "CREATE TABLE users (id bigint, name text, CONSTRAINT uniq_name UNIQUE (name))"},
	{Name: "CREATE TABLE with foreign key", SQL: "CREATE TABLE orders (id bigint PRIMARY KEY, user_id bigint REFERENCES users(id))"},
	{Name: "CREATE TABLE with check constraint", SQL: "CREATE TABLE products (id bigint PRIMARY KEY, price integer CONSTRAINT chk_price_positive CHECK (price > 0))"},
	{Name: "CREATE TABLE generated column", SQL: "CREATE TABLE t (id bigint PRIMARY KEY, a integer, b integer GENERATED ALWAYS AS (a + 1) STORED)"},
	{Name: "CREATE TABLE identity column", SQL: "CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name text)"},

	// --- VIEW ---
	{Name: "CREATE VIEW basic", SQL: "CREATE VIEW v_active AS SELECT id FROM users WHERE active = true"},
	{Name: "CREATE OR REPLACE VIEW", SQL: "CREATE OR REPLACE VIEW v_active AS SELECT id, name FROM users WHERE active = true"},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_active"},

	// --- DROP / TRUNCATE ---
	{Name: "DROP TABLE", SQL: "DROP TABLE users"},
	{Name: "DROP INDEX", SQL: "DROP INDEX idx_users_name"},
	{Name: "TRUNCATE TABLE", SQL: "TRUNCATE TABLE users"},

	// --- CREATE INDEX ---
	{Name: "CREATE INDEX", SQL: "CREATE INDEX idx_users_name ON users (name)"},
	{Name: "CREATE UNIQUE INDEX", SQL: "CREATE UNIQUE INDEX uniq_users_email ON users (email)"},
	{Name: "CREATE INDEX CONCURRENTLY", SQL: "CREATE INDEX CONCURRENTLY idx_users_name ON users (name)"},
	{Name: "CREATE INDEX partial", SQL: "CREATE INDEX idx_active ON users (name) WHERE active = true"},
	{Name: "CREATE INDEX expression", SQL: "CREATE INDEX idx_lower ON users (LOWER(name))"},
	{Name: "CREATE INDEX INCLUDE", SQL: "CREATE INDEX idx_cover ON users (name) INCLUDE (email)"},
	{Name: "CREATE INDEX USING gin", SQL: "CREATE INDEX idx_body ON docs USING gin (body)"},

	// --- ALTER TABLE ---
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email text NOT NULL DEFAULT ''"},
	{Name: "ALTER TABLE DROP COLUMN", SQL: "ALTER TABLE users DROP COLUMN email"},
	{Name: "ALTER TABLE RENAME COLUMN", SQL: "ALTER TABLE users RENAME COLUMN email TO mail"},
	{Name: "ALTER TABLE RENAME TO", SQL: "ALTER TABLE users RENAME TO accounts"},
	{Name: "ALTER INDEX RENAME TO", SQL: "ALTER INDEX idx_users_name RENAME TO idx_accounts_name"},
	{Name: "ALTER TABLE ADD CONSTRAINT PRIMARY KEY", SQL: "ALTER TABLE users ADD CONSTRAINT pk_users PRIMARY KEY (id)"},
	{Name: "ALTER TABLE ADD CONSTRAINT UNIQUE", SQL: "ALTER TABLE users ADD CONSTRAINT uniq_email UNIQUE (email)"},
	{Name: "ALTER TABLE ADD CONSTRAINT FOREIGN KEY", SQL: "ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)"},
	{Name: "ALTER TABLE ADD CONSTRAINT CHECK", SQL: "ALTER TABLE products ADD CONSTRAINT chk_price CHECK (price > 0)"},
	{Name: "ALTER TABLE DROP CONSTRAINT", SQL: "ALTER TABLE users DROP CONSTRAINT uniq_email"},
	{Name: "ALTER TABLE VALIDATE CONSTRAINT", SQL: "ALTER TABLE users VALIDATE CONSTRAINT chk_price"},
	{Name: "ALTER TABLE ALTER COLUMN TYPE", SQL: "ALTER TABLE users ALTER COLUMN name TYPE text"},
	{Name: "ALTER TABLE ALTER COLUMN SET DEFAULT", SQL: "ALTER TABLE users ALTER COLUMN name SET DEFAULT 'unknown'"},
	{Name: "ALTER TABLE ALTER COLUMN DROP DEFAULT", SQL: "ALTER TABLE users ALTER COLUMN name DROP DEFAULT"},
	{Name: "ALTER TABLE ALTER COLUMN SET NOT NULL", SQL: "ALTER TABLE users ALTER COLUMN name SET NOT NULL"},
	{Name: "ALTER TABLE ALTER COLUMN DROP NOT NULL", SQL: "ALTER TABLE users ALTER COLUMN name DROP NOT NULL"},
	{Name: "ALTER TABLE ALTER COLUMN DROP EXPRESSION", SQL: "ALTER TABLE t ALTER COLUMN b DROP EXPRESSION"},
	{Name: "ALTER TABLE ALTER COLUMN SET GENERATED", SQL: "ALTER TABLE t ALTER COLUMN id SET GENERATED BY DEFAULT"},
	{Name: "ALTER TABLE ALTER COLUMN DROP IDENTITY", SQL: "ALTER TABLE t ALTER COLUMN id DROP IDENTITY"},

	// --- OBJECT LIFECYCLE ---
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA IF EXISTS staging"},
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA staging"},
	{Name: "CREATE SEQUENCE", SQL: "CREATE SEQUENCE seq_order_id START WITH 1 INCREMENT BY 1"},
	{Name: "ALTER SEQUENCE", SQL: "ALTER SEQUENCE seq_order_id RESTART WITH 100"},
	{Name: "DROP SEQUENCE", SQL: "DROP SEQUENCE seq_order_id"},
	{Name: "CREATE TYPE enum", SQL: "CREATE TYPE color AS ENUM ('red', 'green', 'blue')"},
	{Name: "ALTER TYPE enum ADD VALUE", SQL: "ALTER TYPE color ADD VALUE 'yellow'"},
	{Name: "DROP TYPE", SQL: "DROP TYPE color"},

	// --- COMPOSITE TYPE LIFECYCLE ---
	{Name: "CREATE TYPE composite", SQL: "CREATE TYPE address AS (street text, city text)"},
	{Name: "ALTER TYPE RENAME TO", SQL: "ALTER TYPE address RENAME TO mailing_address"},
	{Name: "ALTER TYPE SET SCHEMA", SQL: "ALTER TYPE address SET SCHEMA archive"},

	// --- DOMAIN LIFECYCLE ---
	{Name: "CREATE DOMAIN basic", SQL: "CREATE DOMAIN email AS text"},
	{Name: "CREATE DOMAIN NOT NULL", SQL: "CREATE DOMAIN email AS text NOT NULL"},
	{Name: "CREATE DOMAIN DEFAULT", SQL: "CREATE DOMAIN email AS text DEFAULT 'unknown@example.com'"},
	{Name: "CREATE DOMAIN CHECK", SQL: "CREATE DOMAIN email AS text CHECK (VALUE <> '')"},
	{Name: "CREATE DOMAIN named CHECK", SQL: "CREATE DOMAIN email AS text CONSTRAINT email_not_empty CHECK (VALUE <> '')"},
	{Name: "DROP DOMAIN", SQL: "DROP DOMAIN email"},
	{Name: "DROP DOMAIN IF EXISTS CASCADE", SQL: "DROP DOMAIN IF EXISTS email CASCADE"},
	{Name: "ALTER DOMAIN SET DEFAULT", SQL: "ALTER DOMAIN email SET DEFAULT 'unknown@example.com'"},
	{Name: "ALTER DOMAIN DROP DEFAULT", SQL: "ALTER DOMAIN email DROP DEFAULT"},
	{Name: "ALTER DOMAIN SET NOT NULL", SQL: "ALTER DOMAIN email SET NOT NULL"},
	{Name: "ALTER DOMAIN DROP NOT NULL", SQL: "ALTER DOMAIN email DROP NOT NULL"},
	{Name: "ALTER DOMAIN ADD CONSTRAINT", SQL: "ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (VALUE <> '')"},
	{Name: "ALTER DOMAIN DROP CONSTRAINT", SQL: "ALTER DOMAIN email DROP CONSTRAINT email_not_empty"},
	{Name: "ALTER DOMAIN VALIDATE CONSTRAINT", SQL: "ALTER DOMAIN email VALIDATE CONSTRAINT email_not_empty"},
	{Name: "ALTER DOMAIN RENAME", SQL: "ALTER DOMAIN email RENAME TO contact_email"},
	{Name: "CREATE MATERIALIZED VIEW", SQL: "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users"},
	{Name: "DROP MATERIALIZED VIEW", SQL: "DROP MATERIALIZED VIEW mv_stats"},
	{Name: "REFRESH MATERIALIZED VIEW basic", SQL: "REFRESH MATERIALIZED VIEW mv_stats"},
	{Name: "REFRESH MATERIALIZED VIEW CONCURRENTLY", SQL: "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats"},
	{Name: "REFRESH MATERIALIZED VIEW WITH DATA", SQL: "REFRESH MATERIALIZED VIEW mv_stats WITH DATA"},
	{Name: "REFRESH MATERIALIZED VIEW WITH NO DATA", SQL: "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA"},

	// --- GOVERNANCE / ANNOTATION ---
	{Name: "COMMENT ON TABLE", SQL: "COMMENT ON TABLE users IS 'user accounts'"},
	{Name: "GRANT SELECT ON TABLE", SQL: "GRANT SELECT ON TABLE users TO analyst"},
	{Name: "GRANT ALL PRIVILEGES ON TABLE", SQL: "GRANT ALL PRIVILEGES ON TABLE users TO analyst"},
	{Name: "REVOKE SELECT ON TABLE", SQL: "REVOKE SELECT ON TABLE users FROM analyst"},
	{Name: "REVOKE ALL PRIVILEGES ON TABLE CASCADE", SQL: "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE"},
	{Name: "CREATE EXTENSION", SQL: "CREATE EXTENSION IF NOT EXISTS pg_trgm"},
	{Name: "DROP EXTENSION", SQL: "DROP EXTENSION pg_trgm"},
	{Name: "ALTER EXTENSION UPDATE", SQL: "ALTER EXTENSION pg_trgm UPDATE"},
	{Name: "ALTER EXTENSION SET SCHEMA", SQL: "ALTER EXTENSION pg_trgm SET SCHEMA extensions"},
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()"},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_audit ON users"},
}

// pgCorpusCovered maps representative form names to whether a corpus fixture
// exists under testdata/sql-corpus/postgresql/.
var pgCorpusCovered = map[string]bool{
	"CREATE TABLE basic":                       true,
	"CREATE TABLE with primary key":            true,
	"CREATE TABLE with unique constraint":      true,
	"CREATE TABLE with foreign key":            true,
	"CREATE TABLE with check constraint":       true,
	"CREATE TABLE generated column":            true,
	"CREATE TABLE identity column":             true,
	"CREATE VIEW basic":                        false,
	"CREATE OR REPLACE VIEW":                   true,
	"DROP VIEW":                                false,
	"DROP TABLE":                               false,
	"DROP INDEX":                               true,
	"TRUNCATE TABLE":                           false,
	"CREATE INDEX":                             false,
	"CREATE UNIQUE INDEX":                      true,
	"CREATE INDEX CONCURRENTLY":                false,
	"CREATE INDEX partial":                     true,
	"CREATE INDEX expression":                  true,
	"CREATE INDEX INCLUDE":                     true,
	"CREATE INDEX USING gin":                   true,
	"ALTER TABLE ADD COLUMN":                   false,
	"ALTER TABLE DROP COLUMN":                  false,
	"ALTER TABLE RENAME COLUMN":                false,
	"ALTER TABLE RENAME TO":                    false,
	"ALTER INDEX RENAME TO":                    false,
	"ALTER TABLE ADD CONSTRAINT PRIMARY KEY":   true,
	"ALTER TABLE ADD CONSTRAINT UNIQUE":        true,
	"ALTER TABLE ADD CONSTRAINT FOREIGN KEY":   true,
	"ALTER TABLE ADD CONSTRAINT CHECK":         true,
	"ALTER TABLE DROP CONSTRAINT":              true,
	"ALTER TABLE VALIDATE CONSTRAINT":          true,
	"ALTER TABLE ALTER COLUMN TYPE":            false,
	"ALTER TABLE ALTER COLUMN SET DEFAULT":     false,
	"ALTER TABLE ALTER COLUMN DROP DEFAULT":    false,
	"ALTER TABLE ALTER COLUMN SET NOT NULL":    false,
	"ALTER TABLE ALTER COLUMN DROP NOT NULL":   false,
	"ALTER TABLE ALTER COLUMN DROP EXPRESSION": true,
	"ALTER TABLE ALTER COLUMN SET GENERATED":   true,
	"ALTER TABLE ALTER COLUMN DROP IDENTITY":   true,
	"DROP SCHEMA":                              true,
	"CREATE SCHEMA":                            false,
	"CREATE SEQUENCE":                          false,
	"ALTER SEQUENCE":                           true,
	"DROP SEQUENCE":                            true,
	"CREATE TYPE enum":                         true,
	"ALTER TYPE enum ADD VALUE":                true,
	"DROP TYPE":                                true,
	"CREATE TYPE composite":                    true,
	"ALTER TYPE RENAME TO":                     true,
	"ALTER TYPE SET SCHEMA":                    true,
	"CREATE DOMAIN basic":                      false,
	"CREATE DOMAIN NOT NULL":                   false,
	"CREATE DOMAIN DEFAULT":                    false,
	"CREATE DOMAIN CHECK":                      false,
	"CREATE DOMAIN named CHECK":                false,
	"DROP DOMAIN":                              false,
	"DROP DOMAIN IF EXISTS CASCADE":            false,
	"ALTER DOMAIN SET DEFAULT":                 false,
	"ALTER DOMAIN DROP DEFAULT":                false,
	"ALTER DOMAIN SET NOT NULL":                false,
	"ALTER DOMAIN DROP NOT NULL":               false,
	"ALTER DOMAIN ADD CONSTRAINT":              false,
	"ALTER DOMAIN DROP CONSTRAINT":             false,
	"ALTER DOMAIN VALIDATE CONSTRAINT":         false,
	"ALTER DOMAIN RENAME":                      false,
	"CREATE MATERIALIZED VIEW":                 false,
	"DROP MATERIALIZED VIEW":                   true,
	"REFRESH MATERIALIZED VIEW basic":          true,
	"REFRESH MATERIALIZED VIEW CONCURRENTLY":   false,
	"REFRESH MATERIALIZED VIEW WITH DATA":      true,
	"REFRESH MATERIALIZED VIEW WITH NO DATA":   true,
	"COMMENT ON TABLE":                         false,
	"GRANT SELECT ON TABLE":                    true,
	"GRANT ALL PRIVILEGES ON TABLE":            true,
	"REVOKE SELECT ON TABLE":                   true,
	"REVOKE ALL PRIVILEGES ON TABLE CASCADE":   true,
	"CREATE EXTENSION":                         false,
	"DROP EXTENSION":                           false,
	"ALTER EXTENSION UPDATE":                   false,
	"ALTER EXTENSION SET SCHEMA":               false,
	"CREATE TRIGGER":                           false,
	"DROP TRIGGER":                             false,
}

// TestPostgreSQLDDLCoverageCensus runs the full representative-form coverage
// census for PostgreSQL DDL. It characterizes current behavior without
// requiring production changes.
func TestPostgreSQLDDLCoverageCensus(t *testing.T) {
	var results []censusResult

	for _, tc := range pgDDLCensusCases {
		if tc.DeferReason != "" {
			results = append(results, censusResult{
				Name:   tc.Name,
				Status: statusDefer,
			})
			continue
		}
		res := runCensusCase(t, tc)
		results = append(results, res)
	}

	logCensusTable(t, results)

	for _, r := range results {
		if r.Status == statusDefer {
			continue
		}
		if r.Status == "" {
			t.Errorf("census case %q: empty status", r.Name)
		}
	}
}

func runCensusCase(t *testing.T, tc censusCase) censusResult {
	t.Helper()

	res := censusResult{Name: tc.Name}

	// Step 1: parse-level check. Determines parseable vs parser-error.
	parsed, parseErr := Parse(context.Background(), tc.SQL, spec.DialectPostgreSQL)
	if parseErr != nil {
		res.ParseOK = false
		res.Status = statusParserError
		res.CorpusCovered = pgCorpusCovered[tc.Name]
		t.Logf("census: %-45s → parser-error: %v", tc.Name, parseErr)
		return res
	}
	res.ParseOK = true

	// Step 2: extract-level check. Determines unsupported vs normalized.
	statements, extractErr := Extract(context.Background(), parsed)
	if extractErr != nil {
		res.Status = statusUnclassified
		res.CorpusCovered = pgCorpusCovered[tc.Name]
		t.Logf("census: %-45s → extract-error: %v", tc.Name, extractErr)
		return res
	}

	var supported *spec.Statement
	for i := range statements {
		if statements[i].Unsupported == nil {
			s := statements[i]
			supported = &s
			break
		}
	}

	if supported == nil {
		res.Unsupported = true
		if len(statements) > 0 && statements[0].Unsupported != nil {
			res.UnsupportedFeat = statements[0].Unsupported.Feature
			res.UnsupportedWhy = statements[0].Unsupported.Reason
		}
		if len(statements) > 0 {
			res.Kind = string(statements[0].Kind)
		}
		res.Status = statusUnsupportedExp
		res.CorpusCovered = pgCorpusCovered[tc.Name]
		t.Logf("census: %-45s → unsupported-explicit(%s): %s", tc.Name, res.UnsupportedFeat, res.UnsupportedWhy)
		return res
	}

	// Step 3: normalized — collect spec facts.
	res.Normalized = true
	res.Kind = string(supported.Kind)
	if supported.DDL != nil {
		res.DDLOperation = string(supported.DDL.Operation)
		for _, a := range supported.DDL.Alter {
			res.AlterActions = append(res.AlterActions, a.Action)
		}
	}

	// Step 4: run full AuditSQL to collect findings.
	result, auditErr := AuditSQL(context.Background(), Request{
		SQL:     tc.SQL,
		Dialect: spec.DialectPostgreSQL,
	})

	if auditErr != nil {
		t.Logf("census: %-45s → audit-returned-error: %v", tc.Name, auditErr)
	}

	if len(result.Statements) > 0 {
		res.StmtCount = len(result.Statements)
		for _, f := range result.Statements[0].Findings {
			res.StmtFindings = append(res.StmtFindings, f.RuleID)
		}
	}
	if result.GlobalFindings != nil {
		for _, f := range result.GlobalFindings {
			res.GlobalFindings = append(res.GlobalFindings, f.RuleID)
		}
	}

	allFindings := append(res.StmtFindings, res.GlobalFindings...)
	if len(allFindings) > 0 {
		res.Status = statusFindingCovered
	} else {
		res.Status = statusNormalizedSilent
	}

	res.CorpusCovered = pgCorpusCovered[tc.Name]
	t.Logf("census: %-45s → %s | kind=%s op=%s findings=%v alter=%v",
		tc.Name, res.Status, res.Kind, res.DDLOperation,
		allFindings, res.AlterActions)
	return res
}

func logCensusTable(t *testing.T, results []censusResult) {
	t.Helper()
	t.Log("")
	t.Log("=== PostgreSQL DDL Coverage Census ===")
	t.Log("")
	t.Logf("%-45s | %-4s | %-22s | %-24s | %-6s | %-5s | %-6s | %s",
		"Form", "OK?", "Status", "DDL Op", "Norm?", "Find?", "Corp?", "Detail")
	t.Log(strings.Repeat("-", 180))

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	for _, r := range results {
		if r.Status == statusDefer {
			t.Logf("%-45s | %-4s | %-22s | %-24s | %-6s | %-5s | %-6s | %s",
				r.Name, "-", "defer", "-", "-", "-", "-", "-")
			continue
		}

		parseOK := "yes"
		if !r.ParseOK {
			parseOK = "no"
		}
		norm := "yes"
		if !r.Normalized {
			norm = "no"
		}
		hasFind := "yes"
		if len(r.StmtFindings)+len(r.GlobalFindings) == 0 {
			hasFind = "no"
		}
		corpus := "yes"
		if !r.CorpusCovered {
			corpus = "no"
		}

		detail := findingSummary(r)
		if r.Unsupported {
			detail = fmt.Sprintf("unsupported: %s (%s)", r.UnsupportedFeat, r.UnsupportedWhy)
		}

		t.Logf("%-45s | %-4s | %-22s | %-24s | %-6s | %-5s | %-6s | %s",
			r.Name, parseOK, r.Status, r.DDLOperation, norm, hasFind, corpus, detail)
	}

	var counts [7]int
	for _, r := range results {
		switch r.Status {
		case statusFindingCovered:
			counts[0]++
		case statusNormalizedSilent:
			counts[1]++
		case statusUnsupportedExp:
			counts[2]++
		case statusParserError:
			counts[3]++
		case statusUnclassified:
			counts[4]++
		case statusDefer:
			counts[5]++
		}
	}
	var corpusCount int
	for _, r := range results {
		if r.CorpusCovered {
			corpusCount++
		}
	}

	t.Log("")
	t.Log("=== Census Summary ===")
	t.Logf("Total representative forms: %d", len(results))
	t.Logf("  finding-covered:          %d", counts[0])
	t.Logf("  normalized-silent-pass:   %d", counts[1])
	t.Logf("  unsupported-explicit:     %d", counts[2])
	t.Logf("  parser-error:             %d", counts[3])
	t.Logf("  unclassified:             %d", counts[4])
	t.Logf("  defer:                    %d", counts[5])
	t.Logf("  corpus-covered:           %d / %d", corpusCount, len(results))

	var parseable, classifiedDDL, normalized int
	for _, r := range results {
		if r.ParseOK {
			parseable++
		}
		if r.Kind == "ddl" {
			classifiedDDL++
		}
		if r.Normalized {
			normalized++
		}
	}
	t.Logf("  parseable (pg_query):     %d", parseable)
	t.Logf("  classified DDL:           %d", classifiedDDL)
	t.Logf("  normalized:               %d", normalized)
}

func findingSummary(r censusResult) string {
	all := append([]string{}, r.StmtFindings...)
	all = append(all, r.GlobalFindings...)
	if len(all) == 0 {
		return "(none)"
	}
	dedup := make(map[string]struct{})
	for _, id := range all {
		dedup[id] = struct{}{}
	}
	unique := make([]string, 0, len(dedup))
	for id := range dedup {
		unique = append(unique, id)
	}
	sort.Strings(unique)
	return strings.Join(unique, ", ")
}
