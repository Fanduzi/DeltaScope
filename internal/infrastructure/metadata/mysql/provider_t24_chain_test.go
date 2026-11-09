package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"slices"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T24-R6 chain acceptance: the occupancy/binding contracts are proven
// through the FULL formal path — the stub driver → the real
// mysqlmeta.Provider (LoadMySQLResourceGroupFacts + ResolveObject) →
// public appaudit.AuditSQL — never by hand-seeding BoundComplete at the
// application layer. Each cell asserts the public result or the
// propagated error identity, plus which queries were never issued.

// t24ChainFacts returns the complete-wire fixture set minus the
// occupancy block, which each cell supplies. The base LoadInstanceFacts
// SHOW VARIABLES read gets the version row the audit's resolved-version
// premise consumes.
func t24ChainFacts() map[string]testQueryResult {
	return map[string]testQueryResult{
		"show variables where": {
			columns: []string{"Variable_name", "Value"},
			rows:    [][]driver.Value{{"version", "8.0.36"}},
		},
		t24FactsVarsFragment: {
			columns: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{
				{"read_only", "OFF"},
				{"super_read_only", "OFF"},
			},
		},
		t24FactsRolesFragment: {columns: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		t24FactsGrantFragment: {
			columns: []string{"Grants for u@%"},
			rows:    [][]driver.Value{{"GRANT RESOURCE_GROUP_ADMIN ON *.* TO 'u'@'%'"}},
		},
		t24FactsUUIDFragment:    {columns: []string{"@@server_uuid"}, rows: [][]driver.Value{{"srv-uuid-1"}}},
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsFeatureFragment: {columns: []string{"resource_group_name"}, rows: [][]driver.Value{{"usr_default"}}},
	}
}

// t24ChainGroup stubs the catalog row so `rg1` resolves proven-present.
func t24ChainGroup(results map[string]testQueryResult) map[string]testQueryResult {
	results[testResourceGroupFragment] = testQueryResult{
		columns: testMySQLResourceGroupColumns,
		rows:    [][]driver.Value{{"rg1", "USER", "1", "0-3", "5"}},
	}
	return results
}

func t24ChainAudit(t *testing.T, results map[string]testQueryResult, sql string) (report.Result, error) {
	t.Helper()
	db := openTestDB(t, results)
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		TargetVersion:    "8.0.36",
		MetadataProvider: NewProvider(db),
	})
	return result, err
}

func t24ChainGapCodes(stmt report.StatementResult) []string {
	codes := make([]string, 0, len(stmt.EvidenceGaps))
	for _, gap := range stmt.EvidenceGaps {
		codes = append(codes, gap.ReasonCode)
	}
	return codes
}

const t24OccupancyGap = "unknown_resource_group_occupancy"

// Cell 1: clean threads read + lost=0 — a group absent from the proven-
// complete bound set is proven unoccupied through the public path; the
// occupancy gap is gone, and a still-unproven sibling premise (host
// VCPUs on a declared range) keeps its own gap — completeness never
// bleeds across premises.
func TestT24ChainOccupancyCompleteProvesUnoccupied(t *testing.T) {
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{
		columns: []string{"resource_group"},
		rows:    [][]driver.Value{{"other_rg"}},
	}
	results[t24FactsLostFragment] = t24FactsLostRow(0)
	result, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;")
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); slices.Contains(gaps, t24OccupancyGap) {
		t.Fatalf("gaps = %v, want no occupancy gap — the complete census proves rg1 unoccupied", gaps)
	}
	// The declared VCPU still owes the host premise — occupancy
	// completeness never fills a different gap.
	result, err = t24ChainAudit(t, results, "ALTER RESOURCE GROUP rg1 VCPU=2-4;")
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); !slices.Contains(gaps, "unknown_host_vcpu_count") {
		t.Fatalf("gaps = %v, want the host gap still owed — completeness never bleeds premises", gaps)
	}
}

// Cell 2: lost>0 — observed names still prove occupancy (rg1 refuses a
// bare DROP as busy through the public path) while absent names stay
// unproven rather than "unoccupied".
func TestT24ChainOccupancyLostRecordsPartial(t *testing.T) {
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{
		columns: []string{"resource_group"},
		rows:    [][]driver.Value{{"rg1"}},
	}
	results[t24FactsLostFragment] = t24FactsLostRow(3)
	result, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;")
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	found := 0
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.mysql.resource_group.valid" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("findings = %d, want the proven-occupied refusal", found)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); slices.Contains(gaps, t24OccupancyGap) {
		t.Fatalf("gaps = %v, want no occupancy gap for the proven-occupied group", gaps)
	}
	// A second group absent from the partial census still owes the
	// occupancy gap — FORCE is not used because a FORCE drop never
	// consults occupancy upstream.
	results[testResourceGroupFragment] = testQueryResult{
		columns: testMySQLResourceGroupColumns,
		rows: [][]driver.Value{
			{"rg1", "USER", "1", "0-3", "5"},
			{"rg2", "USER", "1", "0-3", "5"},
		},
	}
	db2 := openTestDB(t, results)
	result, err = appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "DROP RESOURCE GROUP rg2;",
		Dialect:          spec.DialectMySQL,
		TargetVersion:    "8.0.36",
		MetadataProvider: NewProvider(db2),
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccupancyGap) {
		t.Fatalf("gaps = %v, want %s for rg2 — a partial census never proves unoccupied", gaps, t24OccupancyGap)
	}
}

// Cell 3: the counter answers cleanly but is unobserved (ErrNoRows) —
// completeness stays unproven and the missing key owes the gap.
func TestT24ChainOccupancyCounterUnobserved(t *testing.T) {
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{columns: []string{"resource_group"}}
	result, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;")
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccupancyGap) {
		t.Fatalf("gaps = %v, want %s — an unobserved counter keeps completeness unproven", gaps, t24OccupancyGap)
	}
}

// Cell 4: a mid-iteration threads failure reaches the public audit as an
// error — no complete-empty census is ever published for consumption.
func TestT24ChainOccupancyMidReadFailure(t *testing.T) {
	want := errors.New("threads read aborted mid-iteration")
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{
		columns:  []string{"resource_group"},
		rows:     [][]driver.Value{{"rg1"}},
		rowErrAt: 1,
		rowErr:   want,
	}
	if _, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;"); !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v) through the public audit", err, want)
	}
}

// Cell 5: a counter read error propagates — it is a read failure, not a
// "normal missing value" degrading into a partial census.
func TestT24ChainOccupancyCounterErrorPropagates(t *testing.T) {
	want := errors.New("counter read denied")
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{columns: []string{"resource_group"}}
	results[t24FactsLostFragment] = testQueryResult{queryErr: want}
	if _, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;"); !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v) through the public audit", err, want)
	}
}

// Cell 6: PFS proven OFF — nothing about occupancy is observed, the
// gap stands, and the version/threads/counter queries are never issued
// (sentinel errors prove none ran).
func TestT24ChainOccupancyPFSOffUnobserved(t *testing.T) {
	mustNotQuery := errors.New("occupancy channel query must not be issued")
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsPFSFragment] = testQueryResult{
		columns: []string{"@@global.performance_schema"},
		rows:    [][]driver.Value{{"OFF"}},
	}
	// The version read is also the dialect banner — it may legitimately
	// run elsewhere, so only the occupancy-specific queries carry the
	// sentinel.
	results[t24FactsThreadsFragment] = testQueryResult{queryErr: mustNotQuery}
	results[t24FactsLostFragment] = testQueryResult{queryErr: mustNotQuery}
	result, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;")
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccupancyGap) {
		t.Fatalf("gaps = %v, want %s — PFS OFF never fabricates unoccupied", gaps, t24OccupancyGap)
	}
}

// Cell 7: a proven 8.0.3+ server must have the column — a missing-object
// error propagates through the public audit instead of degrading.
func TestT24ChainOccupancyColumnErrorPropagates(t *testing.T) {
	missingColumn := &gomysql.MySQLError{Number: 1054, Message: "Unknown column 'resource_group' in 'field list'"}
	results := t24ChainGroup(t24ChainFacts())
	results[t24FactsThreadsFragment] = testQueryResult{queryErr: missingColumn}
	if _, err := t24ChainAudit(t, results, "DROP RESOURCE GROUP rg1;"); err == nil {
		t.Fatal("want the missing-column error propagated — the column exists on every supported server")
	}
}

// Cell 8: a proven pre-8.0.3 banner skips the doomed column query
// entirely; a non-MySQL dialect never invokes the MySQL bundle at all.
func TestT24ChainOccupancyInapplicableSkipsQueries(t *testing.T) {
	mustNotQuery := errors.New("occupancy channel query must not be issued")
	results := t24ChainGroup(t24ChainFacts())
	results["show variables where"] = testQueryResult{
		columns: []string{"Variable_name", "Value"},
		rows:    [][]driver.Value{{"version", "5.7.44"}},
	}
	results[t24FactsVersionFragment] = t24FactsVersionRow("5.7.44")
	results[t24FactsThreadsFragment] = testQueryResult{queryErr: mustNotQuery}
	results[t24FactsLostFragment] = testQueryResult{queryErr: mustNotQuery}
	// The 5.7 banner also resolves the version premise: CREATE refuses
	// as version-unsupported with every fact gap silent — and the
	// sentinel proves the occupancy channel never ran.
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "CREATE RESOURCE GROUP rg_new TYPE=USER;",
		Dialect:          spec.DialectMySQL,
		TargetVersion:    "5.7.44",
		MetadataProvider: NewProvider(openTestDB(t, results)),
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	if got := len(result.Statements[0].Findings); got == 0 {
		t.Fatal("want the proven <8.0.3 refusal")
	}
	if gaps := t24ChainGapCodes(result.Statements[0]); len(gaps) != 0 {
		t.Fatalf("gaps = %v, want none — a proven refusal silences every premise gap", gaps)
	}
}

// Binding: the provider-observed @@server_uuid (a real read, "srv-uuid-1"
// above) outranks a mismatched declared identity — a bundle bound to the
// observed uuid fills the unproven host premise through the public path,
// while a bundle bound elsewhere is refused even though the declared
// identity matches it. An explicit capability=false is consumed as
// false: the authored nonzero priority warns rather than gapping.
func TestT24ChainEvidenceObservedIdentityBinds(t *testing.T) {
	vcpus := uint64(8)
	incapable := false
	results := t24ChainFacts()
	results[t24FactsThreadsFragment] = testQueryResult{columns: []string{"resource_group"}}
	results[t24FactsLostFragment] = t24FactsLostRow(0)
	audit := func(evidence *spec.DeploymentEvidence, declared string) report.Result {
		result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
			SQL:                "CREATE RESOURCE GROUP rg_new TYPE=USER VCPU=2-4 THREAD_PRIORITY=5;",
			Dialect:            spec.DialectMySQL,
			TargetVersion:      "8.0.36",
			MetadataProvider:   NewProvider(openTestDB(t, results)),
			DeploymentEvidence: evidence,
			TargetIdentity:     declared,
		})
		if err != nil {
			t.Fatalf("AuditSQL: %v", err)
		}
		return result
	}

	// Observed uuid binds over a mismatched declared identity.
	result := audit(&spec.DeploymentEvidence{
		Source: "cap:1", Product: "mysql", Version: "8.0.36",
		Target: "srv-uuid-1", HostVCPUs: &vcpus, PriorityCapable: &incapable,
	}, "declared-other")
	if gaps := t24ChainGapCodes(result.Statements[0]); slices.Contains(gaps, "unknown_host_vcpu_count") {
		t.Fatalf("gaps = %v, want the host premise filled — the observed uuid binds", gaps)
	}
	priorityWarned := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.mysql.resource_group.priority.warn" {
			priorityWarned = true
		}
	}
	if !priorityWarned {
		t.Fatal("capability=false must be consumed as false — the authored nonzero priority warns, never gaps")
	}

	// A bundle bound away from the observed uuid is refused despite a
	// declared match — observation outranks declaration.
	result = audit(&spec.DeploymentEvidence{
		Source: "cap:1", Product: "mysql", Version: "8.0.36",
		Target: "foreign-host", HostVCPUs: &vcpus,
	}, "foreign-host")
	if gaps := t24ChainGapCodes(result.Statements[0]); !slices.Contains(gaps, "unknown_host_vcpu_count") {
		t.Fatalf("gaps = %v, want the host gap — a declared match never overrides the observed identity", gaps)
	}
}

// Lazy gating: a batch naming no MySQL resource group never pays the
// premise bundle — evidence and target flags alone trigger zero RG
// queries (every channel carries a sentinel error).
func TestT24ChainLazyGateNoRGStatement(t *testing.T) {
	mustNotQuery := errors.New("resource-group premise query must not be issued")
	results := map[string]testQueryResult{
		"show variables where": {
			columns: []string{"Variable_name", "Value"},
			rows:    [][]driver.Value{{"version", "8.0.36"}},
		},
		t24FactsVarsFragment:    {queryErr: mustNotQuery},
		t24FactsRolesFragment:   {queryErr: mustNotQuery},
		t24FactsGrantFragment:   {queryErr: mustNotQuery},
		t24FactsUUIDFragment:    {queryErr: mustNotQuery},
		t24FactsPFSFragment:     {queryErr: mustNotQuery},
		t24FactsThreadsFragment: {queryErr: mustNotQuery},
		t24FactsLostFragment:    {queryErr: mustNotQuery},
		t24FactsFeatureFragment: {queryErr: mustNotQuery},
	}
	vcpus := uint64(8)
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "CREATE TABLE t (id bigint);",
		Dialect:          spec.DialectMySQL,
		TargetVersion:    "8.0.36",
		MetadataProvider: NewProvider(openTestDB(t, results)),
		DeploymentEvidence: &spec.DeploymentEvidence{
			Source: "cap:1", Product: "mysql", Version: "8.0.36",
			Target: "any", HostVCPUs: &vcpus,
		},
		TargetIdentity: "any",
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v — evidence flags must never trigger the RG premise bundle", err)
	}
}
