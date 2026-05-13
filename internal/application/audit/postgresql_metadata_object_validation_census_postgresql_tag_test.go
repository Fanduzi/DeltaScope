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

// PostgreSQL Metadata Object Validation Census (v0.90.0 Task 1)
//
// Characterizes how the current metadata enrichment path handles PostgreSQL DDL
// statements that target non-table objects (types, domains, extensions, publications,
// subscriptions, foreign objects, annotation targets, event triggers, rewrite rules).
//
// Each case is classified as one of:
//   - metadataConfirmed:    TargetTable is populated, indicating current metadata-aware
//                           path confirmed live object state (existing table/index path)
//   - metadataUnavailable:  Metadata is attached (schema/instance) but TargetTable is
//                           nil — no object-level validation for this statement/object
//   - offlineOnly:          Statement emits offline findings but metadata enrichment
//                           produces no object validation
//   - parserBlocked:        Parser or extractor blocks; statement cannot reach the
//                           metadata enrichment path at all
//
// This file must not modify production code. It only observes current behavior.

// pgMetaCensusClassification describes the metadata-aware object validation state.
type pgMetaCensusClassification string

const (
	pgMetaConfirmed     pgMetaCensusClassification = "metadata_confirmed"
	pgMetaUnavailable   pgMetaCensusClassification = "metadata_unavailable"
	pgMetaOfflineOnly   pgMetaCensusClassification = "offline_only"
	pgMetaParserBlocked pgMetaCensusClassification = "parser_or_extractor_blocked"
)

type pgMetaCensusCase struct {
	Name     string
	SQL      string
	Expected pgMetaCensusClassification
}

type pgMetaCensusResult struct {
	Name           string
	SQL            string
	Expected       pgMetaCensusClassification
	Actual         pgMetaCensusClassification
	ParseOK        bool
	ExtractOK      bool
	HasMetadata    bool
	HasTargetTable bool
	ProviderCalled bool
	DDLOperation   string
	ObjectType     string
	ObjectName     string
	FindingRuleIDs []string
	Mismatch       bool
}

var pgMetadataObjectValidationCensusCases = []pgMetaCensusCase{
	// ===== Known existing baseline cases =====

	{Name: "ALTER TABLE DROP CONSTRAINT (PK)",
		SQL:      "ALTER TABLE users DROP CONSTRAINT users_pkey",
		Expected: pgMetaConfirmed},

	{Name: "ALTER INDEX RENAME",
		SQL:      "ALTER INDEX idx_users_email RENAME TO idx_users_email_v2",
		Expected: pgMetaConfirmed},

	{Name: "DELETE FROM (DML planner)",
		SQL:      "DELETE FROM users WHERE id = 1",
		Expected: pgMetaConfirmed},

	// ===== Type/domain =====

	{Name: "ALTER TYPE ADD ATTRIBUTE",
		SQL:      "ALTER TYPE address ADD ATTRIBUTE country text",
		Expected: pgMetaUnavailable},

	{Name: "ALTER TYPE DROP ATTRIBUTE",
		SQL:      "ALTER TYPE address DROP ATTRIBUTE city",
		Expected: pgMetaUnavailable},

	{Name: "ALTER DOMAIN SET NOT NULL",
		SQL:      "ALTER DOMAIN email SET NOT NULL",
		Expected: pgMetaUnavailable},

	// ===== Extension =====

	{Name: "DROP EXTENSION",
		SQL:      "DROP EXTENSION pg_trgm",
		Expected: pgMetaUnavailable},

	{Name: "ALTER EXTENSION ADD TABLE",
		SQL:      "ALTER EXTENSION pg_trgm ADD TABLE users",
		Expected: pgMetaUnavailable},

	// ===== Publication/subscription =====

	{Name: "DROP PUBLICATION",
		SQL:      "DROP PUBLICATION pub_all",
		Expected: pgMetaUnavailable},

	{Name: "ALTER SUBSCRIPTION DISABLE",
		SQL:      "ALTER SUBSCRIPTION sub DISABLE",
		Expected: pgMetaUnavailable},

	// ===== Foreign objects =====

	{Name: "DROP FOREIGN TABLE",
		SQL:      "DROP FOREIGN TABLE ft_users",
		Expected: pgMetaUnavailable},

	{Name: "ALTER FOREIGN SERVER OPTIONS",
		SQL:      "ALTER SERVER srv OPTIONS (SET host 'db')",
		Expected: pgMetaUnavailable},

	{Name: "DROP USER MAPPING",
		SQL:      "DROP USER MAPPING FOR app SERVER srv",
		Expected: pgMetaUnavailable},

	{Name: "DROP FOREIGN DATA WRAPPER",
		SQL:      "DROP FOREIGN DATA WRAPPER fdw",
		Expected: pgMetaUnavailable},

	// ===== Annotation target =====

	{Name: "COMMENT ON TABLE",
		SQL:      "COMMENT ON TABLE users IS 'user accounts'",
		Expected: pgMetaUnavailable},

	{Name: "SECURITY LABEL ON TABLE NULL",
		SQL:      "SECURITY LABEL FOR selinux ON TABLE users IS NULL",
		Expected: pgMetaUnavailable},

	// ===== Event trigger / rewrite rule =====

	{Name: "DROP EVENT TRIGGER",
		SQL:      "DROP EVENT TRIGGER trg_ddl",
		Expected: pgMetaUnavailable},

	{Name: "ALTER EVENT TRIGGER DISABLE",
		SQL:      "ALTER EVENT TRIGGER trg_ddl DISABLE",
		Expected: pgMetaUnavailable},

	{Name: "DROP RULE",
		SQL:      "DROP RULE users_insert ON users",
		Expected: pgMetaUnavailable},

	// ===== Optional schema-level objects =====

	{Name: "DROP SCHEMA",
		SQL:      "DROP SCHEMA app",
		Expected: pgMetaUnavailable},

	{Name: "DROP SEQUENCE",
		SQL:      "DROP SEQUENCE seq_order_id",
		Expected: pgMetaUnavailable},

	{Name: "DROP MATERIALIZED VIEW",
		SQL:      "DROP MATERIALIZED VIEW mv_stats",
		Expected: pgMetaUnavailable},
}

func TestPostgreSQLMetadataObjectValidationCensus(t *testing.T) {
	t.Parallel()

	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{Version: "PostgreSQL 16.0"},
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users", Schema: "public"},
			Columns: []spec.Column{
				{Name: "id"},
				{Name: "email"},
			},
			PrimaryKey:  &spec.Index{Name: "users_pkey", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
			Indexes:     []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}}},
		},
		indexTable: "users",
	}

	var results []pgMetaCensusResult
	var mismatchCount int

	for _, tc := range pgMetadataObjectValidationCensusCases {
		res := classifyPGMetaCensusCase(t, tc, provider)
		results = append(results, res)
		if res.Mismatch {
			mismatchCount++
		}
	}

	logPGMetaCensusTable(t, results)
	logPGMetaCensusSummary(t, results, mismatchCount)
}

func classifyPGMetaCensusCase(t *testing.T, tc pgMetaCensusCase, provider *fakeMetadataProvider) pgMetaCensusResult {
	t.Helper()
	res := pgMetaCensusResult{
		Name:     tc.Name,
		SQL:      tc.SQL,
		Expected: tc.Expected,
	}

	// Reset provider call tracking.
	provider.tableCalls = nil
	provider.indexCalls = nil
	provider.indexSchemas = nil
	provider.indexDialects = nil
	provider.instanceCalls = 0
	provider.plannerCalls = 0

	// Parse.
	parsed, parseErr := Parse(context.Background(), tc.SQL, spec.DialectPostgreSQL)
	if parseErr != nil {
		res.Actual = pgMetaParserBlocked
		res.Mismatch = res.Expected != res.Actual
		t.Logf("pg-meta-census: %-45s -> parser_blocked: %v", tc.Name, parseErr)
		return res
	}
	res.ParseOK = true

	// Extract.
	statements, extractErr := Extract(context.Background(), parsed)
	if extractErr != nil {
		res.Actual = pgMetaParserBlocked
		res.Mismatch = res.Expected != res.Actual
		t.Logf("pg-meta-census: %-45s -> extract_blocked: %v", tc.Name, extractErr)
		return res
	}
	res.ExtractOK = true

	// Record DDL fields from extracted statement.
	if len(statements) > 0 {
		s := statements[0]
		if s.DDL != nil {
			res.DDLOperation = string(s.DDL.Operation)
			res.ObjectType = s.DDL.ObjectType
			res.ObjectName = s.DDL.ObjectName
		}
	}

	// Run enrichStatementsWithMetadata directly to observe metadata attachment.
	metaReq := &MetadataRequest{
		Schema:   "public",
		Provider: provider,
	}
	enriched, enrichErr := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, metaReq, statements)
	if enrichErr != nil {
		t.Logf("pg-meta-census: %-45s -> enrich_error: %v", tc.Name, enrichErr)
	}

	// Observe metadata state on enriched statements.
	if len(enriched) > 0 {
		s := enriched[0]
		res.HasMetadata = s.Metadata != nil
		if s.Metadata != nil {
			res.HasTargetTable = s.Metadata.TargetTable != nil
		}
	}
	res.ProviderCalled = len(provider.tableCalls) > 0 || len(provider.indexCalls) > 0

	// Collect offline findings via AuditSQL without provider (avoids rule panics
	// when fake metadata is attached to non-table DDL statements).
	auditResult, auditErr := AuditSQL(context.Background(), Request{
		SQL:     tc.SQL,
		Dialect: spec.DialectPostgreSQL,
	})
	_ = auditErr

	seen := make(map[string]struct{})
	if auditResult.Statements != nil {
		for _, s := range auditResult.Statements {
			for _, f := range s.Findings {
				seen[f.RuleID] = struct{}{}
			}
		}
	}
	for _, f := range auditResult.GlobalFindings {
		seen[f.RuleID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	res.FindingRuleIDs = ids

	// Classify from enrichment observations.
	res.Actual = classifyPGMetaResult(res.HasMetadata, res.HasTargetTable, res.ProviderCalled, len(res.FindingRuleIDs) > 0)
	res.Mismatch = res.Expected != res.Actual

	status := string(res.Actual)
	if res.Mismatch {
		status = fmt.Sprintf("MISMATCH(expected=%s, got=%s)", res.Expected, res.Actual)
	}
	t.Logf("pg-meta-census: %-45s -> %-20s | op=%-25s | type=%-10s | name=%-15s | meta=%v tt=%v prov=%v findings=%d",
		tc.Name, status, res.DDLOperation, res.ObjectType, res.ObjectName,
		res.HasMetadata, res.HasTargetTable, res.ProviderCalled, len(res.FindingRuleIDs))

	return res
}

func classifyPGMetaResult(hasMetadata, hasTargetTable, providerCalled, hasFindings bool) pgMetaCensusClassification {
	if hasMetadata && hasTargetTable {
		return pgMetaConfirmed
	}
	if hasMetadata && !hasTargetTable {
		return pgMetaUnavailable
	}
	if !hasMetadata && hasFindings {
		return pgMetaOfflineOnly
	}
	return pgMetaUnavailable
}

func logPGMetaCensusTable(t *testing.T, results []pgMetaCensusResult) {
	t.Helper()
	t.Log("")
	t.Log("=== PostgreSQL Metadata Object Validation Census ===")
	t.Log("")
	t.Logf("%-45s | %-8s | %-20s | %-25s | %-5s | %-5s | %-5s | %s",
		"Case", "OK?", "Classification", "DDL Op", "Meta?", "TT?", "Prov?", "Rule IDs")
	t.Log(strings.Repeat("-", 180))

	sorted := make([]pgMetaCensusResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, r := range sorted {
		ok := "yes"
		if !r.ParseOK || !r.ExtractOK {
			ok = "no"
		}
		class := string(r.Expected)
		if r.Mismatch {
			class = fmt.Sprintf("%s!=%s", r.Expected, r.Actual)
		}
		findings := strings.Join(r.FindingRuleIDs, ", ")
		if findings == "" {
			findings = "(none)"
		}
		meta := "n"
		if r.HasMetadata {
			meta = "y"
		}
		tt := "n"
		if r.HasTargetTable {
			tt = "y"
		}
		prov := "n"
		if r.ProviderCalled {
			prov = "y"
		}
		t.Logf("%-45s | %-8s | %-20s | %-25s | %-5s | %-5s | %-5s | %s",
			r.Name, ok, class, r.DDLOperation, meta, tt, prov, findings)
	}
}

func logPGMetaCensusSummary(t *testing.T, results []pgMetaCensusResult, mismatchCount int) {
	t.Helper()

	counts := make(map[pgMetaCensusClassification]int)
	for _, r := range results {
		counts[r.Expected]++
	}

	t.Log("")
	t.Log("=== PostgreSQL Metadata Object Validation Census Summary ===")
	t.Logf("Total:                       %d", len(results))
	t.Logf("metadata_confirmed:          %d", counts[pgMetaConfirmed])
	t.Logf("metadata_unavailable:        %d", counts[pgMetaUnavailable])
	t.Logf("offline_only:                %d", counts[pgMetaOfflineOnly])
	t.Logf("parser_or_extractor_blocked: %d", counts[pgMetaParserBlocked])
	t.Logf("Classification mismatches:   %d", mismatchCount)
}
