// Package deltascope verifies the public library audit API.
// input: inline SQL text, optional YAML policy overrides, and the public request contract
// output: regression coverage for public audit orchestration and stable result mapping
// pos: public API test coverage for pkg/deltascope
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataProvider struct {
	instanceCalls int
	tableCalls    []string
	indexCalls    []string
	indexSchemas  []string
	indexDialects []Dialect
	indexTable    string
	instance      *InstanceFacts
	snapshot      *TableSnapshot
	err           error
}

func (f *fakeMetadataProvider) LoadInstanceFacts(_ context.Context, _ Dialect, _ string) (*InstanceFacts, error) {
	f.instanceCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.instance, nil
}

func (f *fakeMetadataProvider) LoadTableSnapshot(_ context.Context, _ Dialect, _ string, table string) (*TableSnapshot, error) {
	f.tableCalls = append(f.tableCalls, table)
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func (f *fakeMetadataProvider) ResolveTableForIndex(_ context.Context, dialect Dialect, schema string, index string) (string, error) {
	f.indexCalls = append(f.indexCalls, index)
	f.indexDialects = append(f.indexDialects, dialect)
	f.indexSchemas = append(f.indexSchemas, schema)
	if f.err != nil {
		return "", f.err
	}
	return f.indexTable, nil
}

func TestAuditUsesDefaultPolicy(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "update users set name = 'delta';",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 1 {
		t.Fatalf("expected 1 blocker, got %#v", result.Summary)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("expected 1 statement finding, got %d", len(result.Statements[0].Findings))
	}
	if result.Statements[0].Findings[0].RuleID != "dml.where.require" {
		t.Fatalf("expected where rule finding, got %q", result.Statements[0].Findings[0].RuleID)
	}
}

func TestAuditSupportsConfigOverridePath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	config := "rules:\n  dml.where.require:\n    enabled: false\n    level: blocker\n    params:\n      required: true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Audit(context.Background(), Request{
		SQL:        "update users set name = 'delta';",
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictPass {
		t.Fatalf("expected pass verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("expected empty findings after disabling rule, got %#v", result.Summary)
	}
}

func TestAuditReturnsGroupedMultiStatementResults(t *testing.T) {
	sql := "create table users (id bigint); update users set name = 'delta';"

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected first statement kind ddl, got %q", result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != "dml" {
		t.Fatalf("expected second statement kind dml, got %q", result.Statements[1].Kind)
	}
	if len(result.Statements[0].Findings) == 0 {
		t.Fatalf("expected first statement findings to be grouped")
	}
	if len(result.Statements[1].Findings) == 0 {
		t.Fatalf("expected second statement findings to be grouped")
	}
	if result.Statements[0].Findings[0].StatementIndex != 0 {
		t.Fatalf("expected first statement finding index 0, got %d", result.Statements[0].Findings[0].StatementIndex)
	}
	if result.Statements[1].Findings[0].StatementIndex != 1 {
		t.Fatalf("expected second statement finding index 1, got %d", result.Statements[1].Findings[0].StatementIndex)
	}
}

func TestAuditSupportsPostgreSQLDialectMapping(t *testing.T) {
	_, err := Audit(context.Background(), Request{
		SQL:     "select 1;",
		Dialect: DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected postgresql parse stub error")
	}
	if err == appaudit.ErrUnknownDialect {
		t.Fatal("expected postgresql to map through public API")
	}
}

func TestAuditReturnsCapabilityBoundaryErrorForExplicitPostgreSQLOnUnsupportedBuild(t *testing.T) {
	if _, err := appaudit.Parse(context.Background(), "SELECT 1", spec.DialectPostgreSQL); err == nil {
		t.Skip("skipping: real PG parser available, capability boundary test requires stub build")
	}
	_, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id) values (1) returning id;",
		Dialect: DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected postgresql capability-boundary error")
	}
	message := err.Error()
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if !errors.As(err, &capabilityErr) {
		t.Fatalf("expected typed capability-boundary error, got %T", err)
	}
	if message != capabilityErr.Error() {
		t.Fatalf("expected stable capability-boundary message, got %q want %q", message, capabilityErr.Error())
	}
	if strings.Contains(strings.ToLower(message), "possible dialect mismatch") {
		t.Fatalf("did not expect mismatch wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "if you are auditing postgresql") {
		t.Fatalf("did not expect heuristic suggestion wording, got %q", message)
	}
}

func TestAuditMySQLParseablePGSyntaxReturnsInputErrorWithoutPartialResult(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id) values (1) returning id;",
		Dialect: DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "possible dialect mismatch") {
		t.Fatalf("did not expect adapter-only mismatch wording in public api error, got %q", err.Error())
	}
	if result.Verdict != "" || len(result.Statements) != 0 || len(result.GlobalFindings) != 0 || len(result.Unsupported) != 0 || result.Explanation != nil {
		t.Fatalf("expected zero public result on input error, got %#v", result)
	}
}

func TestAuditMySQLSerialDoesNotAddGlobalNotice(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create table users (id serial primary key);",
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("did not expect global advisory finding, got %#v", result.GlobalFindings)
	}
}

func TestAuditRejectsUnsupportedDialect(t *testing.T) {
	_, err := Audit(context.Background(), Request{
		SQL:     "select 1;",
		Dialect: Dialect("postgres"),
	})
	if err == nil {
		t.Fatal("expected unsupported dialect error")
	}
}

func TestAuditSupportsTopLevelMetadataRequestFields(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance: &InstanceFacts{
			Version:                "8.0.36",
			DefaultCharset:         "utf8mb4",
			InnoDBDefaultRowFormat: "dynamic",
		},
		snapshot: &TableSnapshot{
			Exists: true,
			Table:  &Table{Name: "users"},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users add column email varchar(255);",
		Dialect:          DialectMySQL,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit with metadata fields: %v", err)
	}

	if provider.instanceCalls != 1 {
		t.Fatalf("expected one instance-facts call, got %d", provider.instanceCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 || result.Statements[0].Findings == nil {
		t.Fatalf("expected statement result, got %#v", result.Statements)
	}
}

func TestFromDomainResultMapsTopLevelExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Verdict: report.VerdictPass,
		Explanation: &report.Explanation{
			Summary: "result explanation",
			Reasons: []string{"shared context"},
		},
	})

	if public.Explanation == nil {
		t.Fatal("expected top-level explanation to be mapped")
	}
	if public.Explanation.Summary != "result explanation" {
		t.Fatalf("expected top-level explanation summary to be mapped, got %#v", public.Explanation)
	}
	if len(public.Explanation.Reasons) != 1 || public.Explanation.Reasons[0] != "shared context" {
		t.Fatalf("expected top-level explanation reasons to be mapped, got %#v", public.Explanation)
	}
}

func TestFromDomainResultMapsStatementExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Explanation: &report.Explanation{
				Summary: "statement explanation",
				Reasons: []string{"missing predicate"},
			},
		}},
	})

	if len(public.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(public.Statements))
	}
	if public.Statements[0].Explanation == nil {
		t.Fatal("expected statement explanation to be mapped")
	}
	if public.Statements[0].Explanation.Summary != "statement explanation" {
		t.Fatalf("expected statement explanation summary to be mapped, got %#v", public.Statements[0].Explanation)
	}
	if len(public.Statements[0].Explanation.Reasons) != 1 || public.Statements[0].Explanation.Reasons[0] != "missing predicate" {
		t.Fatalf("expected statement explanation reasons to be mapped, got %#v", public.Statements[0].Explanation)
	}
}

func TestFromDomainResultMapsStatementImpact(t *testing.T) {
	domainRows := ptrInt64(12)
	domainRatio := ptrFloat64(0.25)
	domainReasonCodes := []string{"indexed_range"}
	domainNotes := []string{"metadata refined"}

	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Impact: &report.Impact{
				EstimatedRows:  domainRows,
				EstimatedRatio: domainRatio,
				RiskLevel:      report.ImpactRiskHigh,
				Confidence:     report.ImpactConfidenceMedium,
				Source:         report.ImpactSourceMetadata,
				ReasonCodes:    domainReasonCodes,
				Notes:          domainNotes,
			},
		}},
	})

	if len(public.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(public.Statements))
	}
	if public.Statements[0].Impact == nil {
		t.Fatal("expected statement impact to be mapped")
	}
	if public.Statements[0].Impact.Source != ImpactSourceMetadata {
		t.Fatalf("expected statement impact source to be preserved, got %#v", public.Statements[0].Impact)
	}
	if public.Statements[0].Impact.EstimatedRows == nil || *public.Statements[0].Impact.EstimatedRows != 12 {
		t.Fatalf("expected statement impact estimated rows to be preserved, got %#v", public.Statements[0].Impact)
	}
	if public.Statements[0].Impact.EstimatedRatio == nil || *public.Statements[0].Impact.EstimatedRatio != 0.25 {
		t.Fatalf("expected statement impact estimated ratio to be preserved, got %#v", public.Statements[0].Impact)
	}
	if len(public.Statements[0].Impact.ReasonCodes) != 1 || public.Statements[0].Impact.ReasonCodes[0] != "indexed_range" {
		t.Fatalf("expected statement impact reason codes to be preserved, got %#v", public.Statements[0].Impact)
	}
	if len(public.Statements[0].Impact.Notes) != 1 || public.Statements[0].Impact.Notes[0] != "metadata refined" {
		t.Fatalf("expected statement impact notes to be preserved, got %#v", public.Statements[0].Impact)
	}

	*public.Statements[0].Impact.EstimatedRows = 99
	*public.Statements[0].Impact.EstimatedRatio = 0.75
	public.Statements[0].Impact.ReasonCodes[0] = "mutated_reason"
	public.Statements[0].Impact.Notes[0] = "mutated_note"

	if *domainRows != 12 {
		t.Fatalf("expected estimated rows pointer to be defensive copied, got %d", *domainRows)
	}
	if *domainRatio != 0.25 {
		t.Fatalf("expected estimated ratio pointer to be defensive copied, got %v", *domainRatio)
	}
	if domainReasonCodes[0] != "indexed_range" {
		t.Fatalf("expected reason codes slice to be defensive copied, got %#v", domainReasonCodes)
	}
	if domainNotes[0] != "metadata refined" {
		t.Fatalf("expected notes slice to be defensive copied, got %#v", domainNotes)
	}
}

func TestAuditBuildsAggregateExplanations(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "update users set name = 'delta';",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Explanation == nil {
		t.Fatal("expected result explanation to be populated")
	}
	if result.Explanation.Summary == "" || len(result.Explanation.Reasons) == 0 {
		t.Fatalf("expected result explanation content, got %#v", result.Explanation)
	}
	if len(result.Statements) != 1 || result.Statements[0].Explanation == nil {
		t.Fatalf("expected statement explanation to be populated, got %#v", result.Statements)
	}
	if result.Statements[0].Explanation.Summary == "" || len(result.Statements[0].Explanation.Reasons) == 0 {
		t.Fatalf("expected statement explanation content, got %#v", result.Statements[0].Explanation)
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("expected verdict semantics unchanged, got %q", result.Verdict)
	}
}

func TestFromDomainResultMapsFindingExplanation(t *testing.T) {
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
				Explanation: &rule.FindingExplanation{
					Summary:    "Require DML where",
					Why:        "The statement is missing a clause that the shipped policy requires.",
					Risk:       "Ignoring this rule can allow high-impact data changes.",
					Suggestion: "Add a WHERE clause.",
					Metadata: &rule.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	})

	finding := public.Statements[0].Findings[0]
	if finding.Explanation == nil {
		t.Fatal("expected finding explanation to be mapped")
	}
	if finding.Explanation.Why == "" || finding.Explanation.Risk == "" || finding.Explanation.Suggestion == "" {
		t.Fatalf("expected mapped finding explanation fields, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata == nil || finding.Explanation.Metadata.Status != "limited" {
		t.Fatalf("expected mapped finding metadata note, got %#v", finding.Explanation)
	}
}

func TestFromDomainResultDeepCopiesFindingMetadata(t *testing.T) {
	domainMetadata := map[string]any{
		"operation": "delete",
	}
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause required",
				Metadata: domainMetadata,
			}},
		}},
	})

	public.Statements[0].Findings[0].Metadata["operation"] = "mutated"
	if domainMetadata["operation"] != "delete" {
		t.Fatalf("expected public finding metadata to be defensive copied, got %#v", domainMetadata)
	}
}

func TestFromDomainResultDeepCopiesNestedFindingMetadata(t *testing.T) {
	domainMetadata := map[string]any{
		"labels":        []string{"delete", "high-risk"},
		"details":       map[string]any{"table": "users"},
		"label_groups":  [][]string{{"delete", "high-risk"}},
		"detail_groups": []map[string]any{{"table": "users"}},
	}
	public := fromDomainResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause required",
				Metadata: domainMetadata,
			}},
		}},
	})

	publicLabels := public.Statements[0].Findings[0].Metadata["labels"].([]string)
	publicLabels[0] = "mutated"
	publicDetails := public.Statements[0].Findings[0].Metadata["details"].(map[string]any)
	publicDetails["table"] = "mutated_users"
	publicLabelGroups := public.Statements[0].Findings[0].Metadata["label_groups"].([][]string)
	publicLabelGroups[0][0] = "mutated_group"
	publicDetailGroups := public.Statements[0].Findings[0].Metadata["detail_groups"].([]map[string]any)
	publicDetailGroups[0]["table"] = "mutated_group_users"

	if domainMetadata["labels"].([]string)[0] != "delete" {
		t.Fatalf("expected nested metadata slice to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["details"].(map[string]any)["table"] != "users" {
		t.Fatalf("expected nested metadata map to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["label_groups"].([][]string)[0][0] != "delete" {
		t.Fatalf("expected nested string matrix to be defensive copied, got %#v", domainMetadata)
	}
	if domainMetadata["detail_groups"].([]map[string]any)[0]["table"] != "users" {
		t.Fatalf("expected nested map slice to be defensive copied, got %#v", domainMetadata)
	}
}

func TestPublicMetadataProviderClonesProviderResults(t *testing.T) {
	primaryCardinality := ptrInt64(1000)
	secondaryCardinality := ptrInt64(250)
	instance := &InstanceFacts{
		Version:                "8.0.36",
		DefaultCharset:         "utf8mb4",
		InnoDBDefaultRowFormat: "dynamic",
	}
	snapshot := &TableSnapshot{
		Schema:     "app",
		Exists:     true,
		Table:      &Table{Name: "users"},
		PrimaryKey: &Index{Name: "PRIMARY", Columns: []string{"id"}, Cardinality: primaryCardinality},
		Columns: []Column{
			{Name: "id", Type: "bigint"},
		},
		Indexes:     []Index{{Name: "idx_email", Columns: []string{"email"}, Cardinality: secondaryCardinality}},
		Constraints: []Constraint{{Name: "fk_team", Columns: []string{"team_id"}}},
		Options:     map[string]string{"engine": "InnoDB"},
	}
	provider := publicMetadataProvider{provider: &fakeMetadataProvider{instance: instance, snapshot: snapshot}}

	mappedInstance, err := provider.LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
	if err != nil {
		t.Fatalf("load instance facts: %v", err)
	}
	mappedSnapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "users")
	if err != nil {
		t.Fatalf("load table snapshot: %v", err)
	}

	mappedInstance.DefaultCharset = "latin1"
	mappedSnapshot.Table.Name = "mutated_users"
	mappedSnapshot.Columns[0].Name = "mutated_id"
	mappedSnapshot.PrimaryKey.Columns[0] = "mutated_pk"
	mappedSnapshot.Indexes[0].Columns[0] = "mutated_email"
	mappedSnapshot.Constraints[0].Columns[0] = "mutated_team"
	mappedSnapshot.Options["engine"] = "MyISAM"

	*snapshot.PrimaryKey.Cardinality = 2000
	*snapshot.Indexes[0].Cardinality = 500

	if instance.DefaultCharset != "utf8mb4" {
		t.Fatalf("expected cloned instance facts, got %#v", instance)
	}
	if snapshot.Table == nil || snapshot.Table.Name != "users" {
		t.Fatalf("expected cloned table pointer, got %#v", snapshot.Table)
	}
	if len(snapshot.Columns) != 1 || snapshot.Columns[0].Name != "id" {
		t.Fatalf("expected cloned columns slice, got %#v", snapshot.Columns)
	}
	if snapshot.PrimaryKey == nil || len(snapshot.PrimaryKey.Columns) != 1 || snapshot.PrimaryKey.Columns[0] != "id" {
		t.Fatalf("expected cloned primary key columns, got %#v", snapshot.PrimaryKey)
	}
	if mappedSnapshot.PrimaryKey == nil || mappedSnapshot.PrimaryKey.Cardinality == nil || *mappedSnapshot.PrimaryKey.Cardinality != 1000 {
		t.Fatalf("expected cloned primary key cardinality pointer, got %#v", mappedSnapshot.PrimaryKey)
	}
	if len(snapshot.Indexes) != 1 || len(snapshot.Indexes[0].Columns) != 1 || snapshot.Indexes[0].Columns[0] != "email" {
		t.Fatalf("expected cloned index columns, got %#v", snapshot.Indexes)
	}
	if len(mappedSnapshot.Indexes) != 1 || mappedSnapshot.Indexes[0].Cardinality == nil || *mappedSnapshot.Indexes[0].Cardinality != 250 {
		t.Fatalf("expected cloned secondary index cardinality pointer, got %#v", mappedSnapshot.Indexes)
	}
	if len(snapshot.Constraints) != 1 || len(snapshot.Constraints[0].Columns) != 1 || snapshot.Constraints[0].Columns[0] != "team_id" {
		t.Fatalf("expected cloned constraint columns, got %#v", snapshot.Constraints)
	}
	if snapshot.Options["engine"] != "InnoDB" {
		t.Fatalf("expected cloned options map, got %#v", snapshot.Options)
	}
}

func TestAuditDefaultPolicyDialectHygieneMySQLExcludesPostgreSQLRules(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';",
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	assertNoPGRuleIDs(t, result)
}

func TestAuditDefaultPolicyDialectHygieneTiDBExcludesPostgreSQLRules(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';",
		Dialect: DialectTiDB,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	assertNoPGRuleIDs(t, result)
}

func assertNoPGRuleIDs(t *testing.T, result Result) {
	t.Helper()
	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			if strings.HasPrefix(finding.RuleID, "ddl.pg.") {
				t.Errorf("MySQL/TiDB default audit should not emit PG-only rule %q", finding.RuleID)
			}
		}
	}
	for _, finding := range result.GlobalFindings {
		if strings.HasPrefix(finding.RuleID, "ddl.pg.") {
			t.Errorf("MySQL/TiDB default audit should not emit PG-only rule %q in global findings", finding.RuleID)
		}
	}
}

func TestAuditReturnsSourceLocationsForMultiStatementFileLikeSQL(t *testing.T) {
	sql := `create table ok_users (
  id bigint unsigned not null auto_increment comment 'id',
  name varchar(32) not null default '' comment 'name',
  created_at datetime not null default current_timestamp comment 'created',
  updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated',
  primary key (id)
) comment='ok users';

delete from users;`

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(result.Statements))
	}

	deleteStmt := result.Statements[1]
	var whereFinding *Finding
	for i := range deleteStmt.Findings {
		if deleteStmt.Findings[i].RuleID == "dml.where.require" {
			whereFinding = &deleteStmt.Findings[i]
			break
		}
	}
	if whereFinding == nil {
		t.Fatal("expected dml.where.require finding for 'delete from users'")
	}
	if whereFinding.Location == nil {
		t.Fatal("dml.where.require finding Location is nil, expected {Line:9,Column:1}")
	}
	if whereFinding.Location.Line != 9 {
		t.Errorf("finding Location.Line=%d, want 9", whereFinding.Location.Line)
	}
	if whereFinding.Location.Column != 1 {
		t.Errorf("finding Location.Column=%d, want 1", whereFinding.Location.Column)
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}
