// Package audit verifies the T05-A3 bounded single-table DROP lifecycle.
// input: ordered MySQL/TiDB statements exercising the frozen drop state
// table, both drop variants, loaded dependent references, cancellation
// timing, and the drop-existence evidence-gap reporter
// output: deterministic drop transitions, provider read ledgers, dependent
// invalidation, recreated-shape isolation, and the unchanged
// policy/coverage contract for out-of-scope forms
// pos: T05-A3 regression matrix for the conditional-structure state machine
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t05RuleDropExistsRequire = "ddl.table.drop.exists.require"
	t05RuleDropForbid        = "ddl.table.drop.forbid"
)

func t05A3FiveRulePolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:       "",
		t05RuleDropExistsRequire:  "",
		t05RuleAlterRequire:       "",
		t05RuleAddColumnForbid:    "",
		t05RuleCreateIndexColumns: "      required: true\n",
	})
}

func t05A3DropRecreateStatements(dropSQL string) []string {
	return []string{
		"CREATE TABLE t (old_c INT PRIMARY KEY)",
		dropSQL,
		"CREATE TABLE t (id INT PRIMARY KEY)",
		"ALTER TABLE t ADD COLUMN c INT",
		"CREATE INDEX idx_c ON t(c)",
	}
}

func enrichA3(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider) []spec.Statement {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, dialect)
	if parseErr != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), dialect,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return enriched
}

func TestBatchStateA3DropRecreate(t *testing.T) {
	t.Parallel()
	variants := []struct {
		name    string
		dropSQL string
	}{
		{name: "plain", dropSQL: "DROP TABLE t"},
		{name: "if_exists", dropSQL: "DROP TABLE IF EXISTS t"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, variant := range variants {
			variant := variant
			t.Run(string(dialect)+"/"+variant.name, func(t *testing.T) {
				t.Parallel()
				provider := &t05AbsentProvider{}
				inputs := t05A3DropRecreateStatements(variant.dropSQL)
				sql := strings.Join(inputs, ";\n") + ";"
				result, err := AuditSQL(context.Background(), Request{
					SQL:              sql,
					Dialect:          dialect,
					Schema:           "golden",
					ConfigPath:       t05A3FiveRulePolicy(t),
					MetadataProvider: provider,
				})
				if err != nil {
					t.Fatalf("audit: %v", err)
				}
				if len(result.Statements) != len(inputs) {
					t.Fatalf("expected %d statements, got %d", len(inputs), len(result.Statements))
				}
				if len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 {
					t.Fatalf("result must carry no unsupported/diagnostics, got %+v / %+v", result.Unsupported, result.Diagnostics)
				}
				for i := range result.Statements {
					statement := result.Statements[i]
					if statement.Index != i || statement.RawSQL != inputs[i]+";" {
						t.Fatalf("statement %d identity = index %d raw %q, want index %d raw %q", i, statement.Index, statement.RawSQL, i, inputs[i]+";")
					}
					if len(statement.Findings) != 0 {
						t.Fatalf("statement %d must have no findings, got %+v", i, statement.Findings)
					}
					if len(statement.EvidenceGaps) != 0 {
						t.Fatalf("statement %d must have no evidence gaps, got %+v", i, statement.EvidenceGaps)
					}
					if statement.Coverage.Status != report.CoverageComplete {
						t.Fatalf("statement %d coverage = %+v, want complete", i, statement.Coverage)
					}
				}
				if result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("aggregate coverage = %+v, want complete", result.Coverage)
				}
				if result.Verdict != report.VerdictPass {
					t.Fatalf("verdict = %s, want pass", result.Verdict)
				}
				if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
					t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
				}
				enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
				for i, statement := range enriched {
					if statement.Line != i+1 || statement.Column != 1 || statement.RawSQL != inputs[i]+";" {
						t.Fatalf("enriched %d source identity = line %d column %d raw %q, want line %d column 1 raw %q", i, statement.Line, statement.Column, statement.RawSQL, i+1, inputs[i]+";")
					}
				}
				dropPre := enriched[1].Metadata.TargetTable
				if dropPre == nil || !dropPre.Exists || dropPre.FindColumn("old_c") == nil || dropPre.PrimaryKey == nil {
					t.Fatalf("index1 drop pre-state must carry the derived old shape, got %+v", dropPre)
				}
				recreatePre := enriched[2].Metadata.TargetTable
				if recreatePre == nil || recreatePre.Exists {
					t.Fatalf("index2 recreate pre-state must be known-absent, got %+v", recreatePre)
				}
			})
		}
	}
}

func TestBatchStateA3DropPreStateMatrix(t *testing.T) {
	t.Parallel()
	states := []struct {
		name     string
		snapshot *spec.TableSnapshot
		plain    string
		ifExists string
	}{
		{name: "present_complete", snapshot: presentTable("golden", "t", "id"), plain: "absent", ifExists: "absent"},
		{name: "present_partial", snapshot: t05A3PartialTable("golden", "t"), plain: "absent", ifExists: "absent"},
		{name: "absent", snapshot: absentTable("golden", "t"), plain: "unknown", ifExists: "absent"},
		{name: "unknown", snapshot: nil, plain: "unknown", ifExists: "unknown"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, variant := range []struct{ name, dropSQL string }{
			{name: "plain", dropSQL: "DROP TABLE t"},
			{name: "if_exists", dropSQL: "DROP TABLE IF EXISTS t"},
		} {
			variant := variant
			for _, tc := range states {
				tc := tc
				want := tc.plain
				if variant.name == "if_exists" {
					want = tc.ifExists
				}
				t.Run(string(dialect)+"/"+variant.name+"/"+tc.name, func(t *testing.T) {
					t.Parallel()
					provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
					if tc.snapshot != nil {
						provider.snapshots["golden.t"] = tc.snapshot
					}
					enriched := enrichA3(t, variant.dropSQL+"; CREATE INDEX ix ON t(id);", dialect, provider)
					got := "unknown"
					if projection := t05A2R1Target(enriched[1]); projection != nil {
						got = "present"
						if !projection.Exists {
							got = "absent"
						}
					}
					if got != want {
						t.Fatalf("index1 pre-state = %s (%+v), want %s", got, t05A2R1Target(enriched[1]), want)
					}
					if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
						t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
					}
				})
			}
		}
	}
}

func TestBatchStateA3TombstoneThenIfExistsDrop(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}}
	// BIGINT stays inside the ordinary integer MODIFY template, so it no longer
	// tombstones a known column. A cross-family change still does.
	result := t05A3Audit(t, "ALTER TABLE t MODIFY COLUMN id VARCHAR(20); DROP TABLE IF EXISTS t; CREATE INDEX ix ON t(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
	t05A3DropGap(t, result, 1)
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 2, t05RuleCreateIndexColumns), []string{"target_table.columns", "target_table.existence"})
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
		t.Fatalf("tombstoned target must never be re-read, ledger = %#v", provider.calls)
	}
}

func TestBatchStateA3AuditSurfacePollution(t *testing.T) {
	t.Parallel()
	t.Run("parse failure prefix", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "SELECT * FROM WHERE;\nDROP TABLE t;\nCREATE TABLE t (id INT PRIMARY KEY);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A3FiveRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, errParserUnsupported) {
			t.Fatalf("expected errParserUnsupported, got %v", err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("valid siblings must be retained, got %+v", result.Statements)
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Audited || result.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("parser failure must carry one parser diagnostic, got %+v", result.Diagnostics)
		}
		t05A3DropGap(t, result, 0)
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleCreateForbid), []string{"target_table.existence"})
		for i := range result.Statements {
			if len(result.Statements[i].Findings) != 0 {
				t.Fatalf("statement %d on contaminated state must not produce findings, got %+v", i, result.Statements[i].Findings)
			}
		}
		if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("parse failure floor = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
		}
		if len(provider.calls) != 0 {
			t.Fatalf("contaminated batch must not read the provider, got %#v", provider.calls)
		}
	})
	t.Run("execute prefix", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "EXECUTE stmt1;\nDROP TABLE t;\nCREATE TABLE t (id INT PRIMARY KEY);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A3FiveRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
		}
		if len(result.Unsupported) != 1 {
			t.Fatalf("EXECUTE must produce exactly one unsupported entry, got %+v", result.Unsupported)
		}
		if len(result.Statements) != 3 {
			t.Fatalf("the EXECUTE marker and valid siblings must be retained, got %+v", result.Statements)
		}
		if result.Statements[0].Coverage.Status != report.CoverageIncomplete || len(result.Statements[0].EvidenceGaps) != 0 || len(result.Statements[0].Findings) != 0 {
			t.Fatalf("EXECUTE stays a retained unsupported statement, got %+v", result.Statements[0])
		}
		t05A3DropGap(t, result, 1)
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 2, t05RuleCreateForbid), []string{"target_table.existence"})
		if len(provider.calls) != 0 {
			t.Fatalf("contaminated batch must not read the provider, got %#v", provider.calls)
		}
	})
}

func t05A3AbsentProbe(t *testing.T, result report.Result, index int) {
	t.Helper()
	findings := t05FindingsByRule(result, index, t05RuleCreateIndexColumns)
	if len(findings) != 1 || findings[0].Metadata["reason"] != "table_not_found" {
		t.Fatalf("statement %d must emit the settled table_not_found blocker, got %+v", index, result.Statements[index].Findings)
	}
	if len(result.Statements[index].EvidenceGaps) != 0 {
		t.Fatalf("statement %d on a known-absent table must not emit gaps, got %+v", index, result.Statements[index].EvidenceGaps)
	}
	if result.Statements[index].Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement %d coverage = %+v, want complete", index, result.Statements[index].Coverage)
	}
}

func t05A3DropGap(t *testing.T, result report.Result, index int) {
	t.Helper()
	if len(result.Statements[index].Findings) != 0 {
		t.Fatalf("statement %d on unknown state must not produce findings, got %+v", index, result.Statements[index].Findings)
	}
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, index, t05RuleDropExistsRequire), []string{"target_table.existence"})
	if result.Statements[index].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("statement %d coverage = %+v, want unverified", index, result.Statements[index].Coverage)
	}
}

func t05A3DropBlocker(t *testing.T, result report.Result, index int) {
	t.Helper()
	findings := t05FindingsByRule(result, index, t05RuleDropExistsRequire)
	if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
		t.Fatalf("statement %d must emit exactly one drop-existence blocker, got %+v", index, result.Statements[index].Findings)
	}
	if len(result.Statements[index].EvidenceGaps) != 0 {
		t.Fatalf("statement %d on a known-absent drop must not emit gaps, got %+v", index, result.Statements[index].EvidenceGaps)
	}
	if result.Statements[index].Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement %d coverage = %+v, want complete", index, result.Statements[index].Coverage)
	}
}

func t05A3Audit(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, config string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       config,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

func t05A3PartialTable(schema, table string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:             true,
		Schema:             schema,
		Table:              &spec.Table{Schema: schema, Name: table},
		PrimaryKeyUnknown:  true,
		IndexesUnknown:     true,
		ConstraintsUnknown: true,
	}
}

func TestBatchStateA3DropStateTable(t *testing.T) {
	t.Parallel()
	t.Run("present_complete", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}}
		result := t05A3Audit(t, "DROP TABLE t; CREATE INDEX ix ON t(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 || result.Statements[0].Coverage.Status != report.CoverageComplete {
			t.Fatalf("present drop must be clean and complete, got findings=%+v gaps=%+v coverage=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps, result.Statements[0].Coverage)
		}
		t05A3AbsentProbe(t, result, 1)
		if result.Verdict != report.VerdictReject {
			t.Fatalf("verdict = %s, want reject", result.Verdict)
		}
		if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
			t.Fatalf("provider read ledger = %#v, want [golden.t]", provider.calls)
		}
	})
	t.Run("present_partial", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": t05A3PartialTable("golden", "t")}}
		result := t05A3Audit(t, "DROP TABLE t; CREATE INDEX ix ON t(id);", spec.DialectTiDB, provider, t05A3FiveRulePolicy(t))
		t05A3AbsentProbe(t, result, 1)
	})
	t.Run("absent_plain_then_recreate", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result := t05A3Audit(t, "DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3DropBlocker(t, result, 0)
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleCreateForbid), []string{"target_table.existence"})
		if result.Statements[1].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("recreate after absent plain drop coverage = %+v, want unverified", result.Statements[1].Coverage)
		}
	})
	t.Run("absent_if_exists_then_recreate", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result := t05A3Audit(t, "DROP TABLE IF EXISTS t; CREATE TABLE t (id INT PRIMARY KEY);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3DropBlocker(t, result, 0)
		if len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 || result.Statements[1].Coverage.Status != report.CoverageComplete {
			t.Fatalf("recreate after absent if-exists drop must be clean, got findings=%+v gaps=%+v coverage=%+v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps, result.Statements[1].Coverage)
		}
	})
	for _, variant := range []struct{ name, dropSQL string }{
		{name: "plain", dropSQL: "DROP TABLE t;"},
		{name: "if_exists", dropSQL: "DROP TABLE IF EXISTS t;"},
	} {
		variant := variant
		t.Run("unknown_"+variant.name, func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
			result := t05A3Audit(t, variant.dropSQL, spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
			t05A3DropGap(t, result, 0)
			if result.Verdict != report.VerdictReview {
				t.Fatalf("verdict = %s, want review", result.Verdict)
			}
		})
	}
	t.Run("repeated_drop_third_blocker", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result := t05A3Audit(t, "CREATE TABLE t (old_c INT PRIMARY KEY); DROP TABLE t; DROP TABLE t;", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		if len(result.Statements[0].EvidenceGaps) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
			t.Fatalf("first create and drop must stay gap-free, got gaps=%+v / %+v", result.Statements[0].EvidenceGaps, result.Statements[1].EvidenceGaps)
		}
		t05A3DropBlocker(t, result, 2)
	})
	t.Run("gap_disabled_stays_complete", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
		result := t05A3Audit(t, "DROP TABLE t;", spec.DialectMySQL, provider, t05FourRulePolicy(t))
		if len(result.Statements[0].EvidenceGaps) != 0 || result.Statements[0].Coverage.Status != report.CoverageComplete {
			t.Fatalf("disabled drop rule must not emit the gap, got gaps=%+v coverage=%+v", result.Statements[0].EvidenceGaps, result.Statements[0].Coverage)
		}
	})
	t.Run("unknown_then_recreate", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
		result := t05A3Audit(t, "DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3DropGap(t, result, 0)
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleCreateForbid), []string{"target_table.existence"})
	})
}

func TestBatchStateA3OldColumnIndexNegative(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result := t05A3Audit(t, "CREATE TABLE t (old_c INT PRIMARY KEY); DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t(c); CREATE INDEX ix_old ON t(old_c);", dialect, provider, t05A3FiveRulePolicy(t))
			findings := t05FindingsByRule(result, 5, t05RuleCreateIndexColumns)
			if len(findings) != 1 {
				t.Fatalf("statement 5 must emit exactly one missing-column blocker, got %+v", result.Statements[5].Findings)
			}
			metadata := findings[0].Metadata
			if metadata["column"] != "old_c" || metadata["index"] != "ix_old" || metadata["schema"] != "golden" || metadata["table"] != "t" || metadata["exists"] != false {
				t.Fatalf("finding metadata identity = %#v, want column=old_c index=ix_old schema=golden table=t exists=false", metadata)
			}
			if len(result.Statements[5].EvidenceGaps) != 0 {
				t.Fatalf("statement 5 must not emit gaps, got %+v", result.Statements[5].EvidenceGaps)
			}
			if result.Statements[5].Coverage.Status != report.CoverageComplete {
				t.Fatalf("statement 5 coverage = %+v, want complete", result.Statements[5].Coverage)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
		})
	}
}

func t05A3Int64(v int64) *int64 { return &v }

func t05A3OldTable() *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Schema: "golden",
		Table:  &spec.Table{Schema: "golden", Name: "t"},
		Columns: []spec.Column{
			{Name: "old_c", Type: "int"},
			{Name: "keep_c", Type: "varchar(10)"},
		},
		PrimaryKey: &spec.Index{Name: "PRIMARY", Columns: []string{"old_c"}, Cardinality: t05A3Int64(42)},
		Indexes: []spec.Index{
			{Name: "ix_old", Columns: []string{"old_c"}, Cardinality: t05A3Int64(7)},
		},
		Constraints: []spec.Constraint{{
			Type:              "foreign_key",
			Name:              "fk_old",
			Columns:           []string{"old_c"},
			ReferencedTable:   "p",
			ReferencedColumns: []string{"id"},
		}},
		Options: map[string]string{"table_rows": "99", "auto_increment": "55", "comment": "old"},
	}
}

func TestBatchStateA3OldShapeNotInherited(t *testing.T) {
	t.Parallel()
	old := t05A3OldTable()
	oldFrozen := cloneTableSnapshot(old)
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": old}}
	enriched := enrichA3(t, "DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY) COMMENT='fresh'; ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t(c); CREATE INDEX ix2 ON t(id);", spec.DialectMySQL, provider)

	dropPre := enriched[0].Metadata.TargetTable
	if dropPre == nil || !reflect.DeepEqual(dropPre, oldFrozen) {
		t.Fatalf("drop pre-state must retain the old provider shape verbatim, got %+v", dropPre)
	}
	recreatePre := enriched[1].Metadata.TargetTable
	if recreatePre == nil || recreatePre.Exists {
		t.Fatalf("recreate pre-state must be known-absent, got %+v", recreatePre)
	}
	addPre := enriched[2].Metadata.TargetTable
	if addPre == nil || !addPre.Exists {
		t.Fatalf("add pre-state must be the new derived shape, got %+v", addPre)
	}
	if len(addPre.Columns) != 1 || addPre.Columns[0].Name != "id" {
		t.Fatalf("new shape must carry only the declared id column, got %+v", addPre.Columns)
	}
	if addPre.PrimaryKey == nil || addPre.PrimaryKey.Name != "primary" || !reflect.DeepEqual(addPre.PrimaryKey.Columns, []string{"id"}) || addPre.PrimaryKey.Cardinality != nil {
		t.Fatalf("new primary key must be the declared PRIMARY(id) with no old cardinality, got %+v", addPre.PrimaryKey)
	}
	if len(addPre.Options) != 1 || addPre.Options["comment"] != "fresh" {
		t.Fatalf("new shape must carry only the fresh COMMENT option, got %+v", addPre.Options)
	}
	if len(addPre.Indexes) != 0 || len(addPre.Constraints) != 0 {
		t.Fatalf("new shape must not inherit old indexes/constraints, got indexes=%+v constraints=%+v", addPre.Indexes, addPre.Constraints)
	}
	indexPre := enriched[3].Metadata.TargetTable
	if indexPre == nil || len(indexPre.Columns) != 2 || indexPre.FindColumn("c") == nil || len(indexPre.Indexes) != 0 {
		t.Fatalf("index pre-state must be the new shape with c but no indexes yet, got %+v", indexPre)
	}
	postIndexPre := enriched[4].Metadata.TargetTable
	if postIndexPre == nil || len(postIndexPre.Indexes) != 1 || postIndexPre.Indexes[0].Name != "idx_c" || postIndexPre.Indexes[0].Cardinality != nil {
		t.Fatalf("post-index projection must carry only idx_c with nil cardinality, got %+v", postIndexPre)
	}
	if _, ok := postIndexPre.Options["table_rows"]; ok {
		t.Fatalf("post-index projection must not carry old table_rows, got %+v", postIndexPre.Options)
	}
	if _, ok := postIndexPre.Options["auto_increment"]; ok {
		t.Fatalf("post-index projection must not carry old auto_increment, got %+v", postIndexPre.Options)
	}
	postIndexPre.Options["comment"] = "mutated"
	postIndexPre.PrimaryKey.Columns[0] = "mutated"
	postIndexPre.Columns[0].Name = "mutated"
	if enriched[1].Metadata.TargetTable.Exists || dropPre.FindColumn("mutated") != nil || old.FindColumn("mutated") != nil {
		t.Fatalf("mutating a later projection must not touch earlier projections or the provider object")
	}
	if addPre.Columns[0].Name != "id" || addPre.PrimaryKey.Columns[0] != "id" || addPre.Options["comment"] != "fresh" {
		t.Fatalf("mutating a later projection must not touch the earlier new projection, got %+v", addPre)
	}
	if old.Options["table_rows"] != "99" || old.Options["auto_increment"] != "55" || old.PrimaryKey.Columns[0] != "old_c" {
		t.Fatalf("mutating a projection must not touch the provider object, got %+v", old)
	}
	if !reflect.DeepEqual(old, oldFrozen) {
		t.Fatalf("provider object must stay unchanged, got %+v", old)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
		t.Fatalf("provider read ledger = %#v, want [golden.t]", provider.calls)
	}
}

func t05A3NamedDependent(owner, name, refSchema, refTable string) *spec.TableSnapshot {
	child := presentTable(owner, name, "id")
	child.Constraints = []spec.Constraint{{
		Type:              "foreign_key",
		Name:              "fk_" + name + "_ref",
		Columns:           []string{"id"},
		ReferencedSchema:  refSchema,
		ReferencedTable:   refTable,
		ReferencedColumns: []string{"id"},
	}}
	return child
}

func t05A3DependentProviders(t *testing.T, owner, refSchema, refTable string, target *spec.TableSnapshot) *t05PresentProvider {
	t.Helper()
	child := t05A2R1DependentSnapshot(owner, refSchema, refTable)
	return &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		owner + ".child": child,
		"other.u":        presentTable("other", "u", "id"),
		"golden.t":       target,
	}}
}

func t05A3ChildAlterGap(t *testing.T, result report.Result, index int) {
	t.Helper()
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, index, t05RuleAlterRequire), []string{"target_table.existence"})
	t05A2R1AssertUnknownGap(t, t05GapsByRule(result, index, t05RuleAddColumnForbid), []string{"target_table.columns", "target_table.existence"})
}

func TestBatchStateA3LoadedDependentInvalidates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		owner     string
		refSchema string
		wantChild string
	}{
		{name: "explicit target schema", owner: "aux", refSchema: "golden", wantChild: "unknown"},
		{name: "unqualified owner schema match", owner: "golden", refSchema: "", wantChild: "unknown"},
		{name: "unqualified owner schema miss", owner: "aux", refSchema: "", wantChild: "known"},
		{name: "other schema same name", owner: "aux", refSchema: "other", wantChild: "known"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := t05A3DependentProviders(t, tc.owner, tc.refSchema, "t", presentTable("golden", "t", "id"))
			childBefore := cloneTableSnapshot(provider.snapshots[tc.owner+".child"])
			sql := "CREATE INDEX ix_child ON " + tc.owner + ".child(id); DROP TABLE t; ALTER TABLE " + tc.owner + ".child ADD COLUMN c INT; CREATE INDEX ix_u ON other.u(id);"
			result := t05A3Audit(t, sql, spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("first child projection must stay clean, got findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
			if tc.wantChild == "unknown" {
				t05A3ChildAlterGap(t, result, 2)
			} else if len(result.Statements[2].Findings) != 0 || len(result.Statements[2].EvidenceGaps) != 0 || result.Statements[2].Coverage.Status != report.CoverageComplete {
				t.Fatalf("unaffected child must stay known, got findings=%+v gaps=%+v coverage=%+v", result.Statements[2].Findings, result.Statements[2].EvidenceGaps, result.Statements[2].Coverage)
			}
			if len(result.Statements[3].Findings) != 0 || len(result.Statements[3].EvidenceGaps) != 0 {
				t.Fatalf("unrelated other.u must stay known, got findings=%+v gaps=%+v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
			}
			if !reflect.DeepEqual(provider.snapshots[tc.owner+".child"], childBefore) {
				t.Fatalf("provider child snapshot must stay verbatim, got %+v", provider.snapshots[tc.owner+".child"])
			}
			wantCalls := []string{tc.owner + ".child", "golden.t", "other.u"}
			sort.Strings(provider.calls)
			sort.Strings(wantCalls)
			if !reflect.DeepEqual(provider.calls, wantCalls) {
				t.Fatalf("provider read ledger = %#v, want %#v", provider.calls, wantCalls)
			}
		})
	}
	t.Run("conservative branch still tombstones", func(t *testing.T) {
		t.Parallel()
		provider := t05A3DependentProviders(t, "aux", "golden", "t", absentTable("golden", "t"))
		result := t05A3Audit(t, "CREATE INDEX ix_child ON aux.child(id); DROP TABLE t; ALTER TABLE aux.child ADD COLUMN c INT;", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3DropBlocker(t, result, 1)
		t05A3ChildAlterGap(t, result, 2)
	})
	t.Run("recreate does not restore", func(t *testing.T) {
		t.Parallel()
		provider := t05A3DependentProviders(t, "aux", "golden", "t", presentTable("golden", "t", "id"))
		result := t05A3Audit(t, "CREATE INDEX ix_child ON aux.child(id); DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE aux.child ADD COLUMN c INT;", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		if len(result.Statements[2].EvidenceGaps) != 0 {
			t.Fatalf("recreate must be clean, got gaps=%+v", result.Statements[2].EvidenceGaps)
		}
		t05A3ChildAlterGap(t, result, 3)
	})
	t.Run("multi target union", func(t *testing.T) {
		t.Parallel()
		child1 := t05A3NamedDependent("aux", "child1", "golden", "t")
		child1Frozen := cloneTableSnapshot(child1)
		child2 := t05A3NamedDependent("aux", "child2", "other", "u")
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"aux.child1": child1,
			"aux.child2": child2,
			"other.v":    presentTable("other", "v", "id"),
			"golden.t":   presentTable("golden", "t", "id"),
			"other.u":    presentTable("other", "u", "id"),
		}}
		sql := "CREATE INDEX ix_c1 ON aux.child1(id); CREATE INDEX ix_c2 ON aux.child2(id); DROP TABLE t, other.u; ALTER TABLE aux.child1 ADD COLUMN c INT; ALTER TABLE aux.child2 ADD COLUMN c INT; CREATE INDEX ix_v ON other.v(id);"
		result := t05A3Audit(t, sql, spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		for _, index := range []int{3, 4} {
			if len(result.Statements[index].Findings) != 0 || len(result.Statements[index].EvidenceGaps) != 2 {
				t.Fatalf("statement %d must carry exactly two gaps and zero findings, got findings=%+v gaps=%+v", index, result.Statements[index].Findings, result.Statements[index].EvidenceGaps)
			}
			t05A2R1AssertUnknownGap(t, t05GapsByRule(result, index, t05RuleAlterRequire), []string{"target_table.existence"})
			t05A2R1AssertUnknownGap(t, t05GapsByRule(result, index, t05RuleAddColumnForbid), []string{"target_table.columns", "target_table.existence"})
		}
		if len(result.Statements[5].Findings) != 0 || len(result.Statements[5].EvidenceGaps) != 0 || result.Statements[5].Coverage.Status != report.CoverageComplete {
			t.Fatalf("unrelated other.v must stay known, got findings=%+v gaps=%+v coverage=%+v", result.Statements[5].Findings, result.Statements[5].EvidenceGaps, result.Statements[5].Coverage)
		}
		enriched := enrichA3(t, sql, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"aux.child1": t05A3NamedDependent("aux", "child1", "golden", "t"),
			"aux.child2": t05A3NamedDependent("aux", "child2", "other", "u"),
			"other.v":    presentTable("other", "v", "id"),
			"golden.t":   presentTable("golden", "t", "id"),
			"other.u":    presentTable("other", "u", "id"),
		}})
		if projection := t05A2R1Target(enriched[0]); projection == nil || !reflect.DeepEqual(projection.Constraints, child1Frozen.Constraints) {
			t.Fatalf("child1 pre-projection constraints must retain the verbatim reference, got %+v", projection)
		}
		if !reflect.DeepEqual(child1, child1Frozen) {
			t.Fatalf("provider child1 snapshot must stay verbatim, got %+v", child1)
		}
		wantCalls := []string{"aux.child1", "aux.child2", "golden.t", "other.v"}
		gotCalls := append([]string(nil), provider.calls...)
		sort.Strings(gotCalls)
		sort.Strings(wantCalls)
		if !reflect.DeepEqual(gotCalls, wantCalls) {
			t.Fatalf("provider read ledger = %#v, want %#v", gotCalls, wantCalls)
		}
	})
	t.Run("target referencing target", func(t *testing.T) {
		t.Parallel()
		provider := t05A3DependentProviders(t, "aux", "golden", "t", presentTable("golden", "t", "id"))
		result := t05A3Audit(t, "CREATE INDEX ix_child ON aux.child(id); DROP TABLE t, aux.child; ALTER TABLE aux.child ADD COLUMN c INT;", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3ChildAlterGap(t, result, 2)
	})
}

func TestBatchStateA3NonPreciseForms(t *testing.T) {
	t.Parallel()
	t.Run("multi target invalidates every key", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": presentTable("golden", "t", "id"),
			"golden.u": presentTable("golden", "u", "id"),
		}}
		result := t05A3Audit(t, "DROP TABLE t, u; CREATE INDEX ix_t ON t(id); CREATE INDEX ix_u ON u(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleCreateIndexColumns), []string{"target_table.columns", "target_table.existence"})
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 2, t05RuleCreateIndexColumns), []string{"target_table.columns", "target_table.existence"})
		if len(provider.calls) > 2 {
			t.Fatalf("multi-target drop must not reread, got %#v", provider.calls)
		}
	})
	t.Run("duplicate raw target is not precise", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}}
		result := t05A3Audit(t, "DROP TABLE t, t; CREATE INDEX ix ON t(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 1, t05RuleCreateIndexColumns), []string{"target_table.columns", "target_table.existence"})
	})
	t.Run("temporary drop invalidates without absence", func(t *testing.T) {
		t.Parallel()
		state := newBatchState(spec.DialectMySQL, "golden", &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}})
		key := state.keyFor("golden", spec.Table{Name: "t"})
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load target: %v", err)
		}
		statements := t05A2R1Extract(t, "DROP TEMPORARY TABLE t;")
		if statements[0].DDL == nil || statements[0].DDL.Operation != spec.DDLOperationDropTable || statements[0].DDL.TemporaryScope == "" {
			t.Fatalf("extracted temporary drop identity = %+v", statements[0].DDL)
		}
		if err := state.apply(context.Background(), statements[0]); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if state.contaminated {
			t.Fatal("temporary drop must invalidate its bound target, not contaminate")
		}
		if state.entries[key].state != tableUnknown {
			t.Fatalf("temporary drop must not publish a known-absent entry, got %+v", state.entries[key])
		}
	})
	t.Run("execute contamination suppresses derivation", func(t *testing.T) {
		t.Parallel()
		state := newBatchState(spec.DialectMySQL, "golden", &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}})
		key := state.keyFor("golden", spec.Table{Name: "t"})
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load target: %v", err)
		}
		statements := t05A2R1Extract(t, "EXECUTE stmt1; DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY);")
		for i, statement := range statements {
			if err := state.apply(context.Background(), statement); err != nil {
				t.Fatalf("apply %d: %v", i, err)
			}
		}
		if !state.contaminated {
			t.Fatal("EXECUTE must contaminate the batch")
		}
		if state.entries[key].state != tablePresent {
			t.Fatalf("contamination must freeze the loaded present entry, got %+v", state.entries[key])
		}
	})
	t.Run("schema invalidation survives", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")}}
		result := t05A3Audit(t, "DROP DATABASE golden; DROP TABLE t;", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3DropGap(t, result, 1)
		if len(provider.calls) != 0 {
			t.Fatalf("invalidated schema must not reread, got %#v", provider.calls)
		}
	})
	t.Run("unbound drop contaminates", func(t *testing.T) {
		t.Parallel()
		state := newBatchState(spec.DialectMySQL, "golden", &t05AbsentProvider{})
		entry := &batchTableEntry{state: tablePresent, shape: presentTable("golden", "child", "id"), displaySchema: "golden", displayTable: "child"}
		entry.shape.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t"}}
		state.entries[state.keyFor("golden", spec.Table{Name: "child"})] = entry
		statement := spec.Statement{Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationDropTable}}
		if err := state.apply(context.Background(), statement); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !state.contaminated {
			t.Fatal("unbound drop must contaminate the batch")
		}
		if state.entries[state.keyFor("golden", spec.Table{Name: "child"})].state != tablePresent {
			t.Fatal("contamination must not invalidate loaded dependents")
		}
	})
}

func TestBatchStateA3DispatchGuards(t *testing.T) {
	t.Parallel()
	newLoadedState := func() (*batchState, batchTableKey, batchTableKey) {
		state := newBatchState(spec.DialectMySQL, "golden", &t05AbsentProvider{})
		targetKey := state.keyFor("golden", spec.Table{Name: "t"})
		childKey := state.keyFor("golden", spec.Table{Schema: "aux", Name: "child"})
		state.entries[targetKey] = &batchTableEntry{state: tablePresent, shape: presentTable("golden", "t", "id"), displaySchema: "golden", displayTable: "t"}
		child := &batchTableEntry{state: tablePresent, shape: presentTable("aux", "child", "id"), displaySchema: "aux", displayTable: "child"}
		child.shape.Constraints = []spec.Constraint{{Type: "foreign_key", Name: "fk", Columns: []string{"id"}, ReferencedSchema: "golden", ReferencedTable: "t"}}
		state.entries[childKey] = child
		return state, targetKey, childKey
	}
	t.Run("unsupported marker stays conservative", func(t *testing.T) {
		t.Parallel()
		state, targetKey, childKey := newLoadedState()
		marker := &spec.UnsupportedDetail{Feature: "drop_table.temporary", Reason: spec.UnsupportedUnauditedReason}
		statement := spec.Statement{
			Kind:        spec.KindDDL,
			Unsupported: marker,
			DDL:         &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &spec.Table{Name: "t"}, Targets: []spec.Table{{Name: "t"}}},
		}
		if err := state.apply(context.Background(), statement); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if state.entries[targetKey].state != tableUnknown || state.entries[childKey].state != tableUnknown {
			t.Fatalf("marked drop must invalidate target and dependent, got %+v / %+v", state.entries[targetKey], state.entries[childKey])
		}
		if statement.Unsupported == nil || statement.Unsupported.Feature != "drop_table.temporary" {
			t.Fatalf("the unsupported marker must stay attached, got %+v", statement.Unsupported)
		}
	})
	t.Run("mismatched primary and targets stays conservative", func(t *testing.T) {
		t.Parallel()
		state, targetKey, childKey := newLoadedState()
		statement := spec.Statement{
			Kind: spec.KindDDL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationDropTable,
				Table:     &spec.Table{Name: "t"},
				Targets:   []spec.Table{{Name: "u"}},
			},
		}
		if err := state.apply(context.Background(), statement); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if state.entries[targetKey].state != tableUnknown || state.entries[childKey].state != tableUnknown {
			t.Fatalf("mismatched drop must invalidate bound identities, got %+v / %+v", state.entries[targetKey], state.entries[childKey])
		}
	})
	t.Run("omitted targets stays conservative", func(t *testing.T) {
		t.Parallel()
		state, targetKey, childKey := newLoadedState()
		statement := spec.Statement{
			Kind: spec.KindDDL,
			DDL: &spec.DDL{
				Operation:      spec.DDLOperationDropTable,
				Table:          &spec.Table{Name: "t"},
				Targets:        []spec.Table{{Name: "t"}},
				OmittedTargets: 1,
			},
		}
		if err := state.apply(context.Background(), statement); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if state.entries[targetKey].state != tableUnknown || state.entries[childKey].state != tableUnknown {
			t.Fatalf("omitted-target drop must invalidate bound identities, got %+v / %+v", state.entries[targetKey], state.entries[childKey])
		}
	})
}

func TestBatchStateA3TruncateUnchanged(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}}
			result := t05A3Audit(t, "TRUNCATE TABLE t;", dialect, provider, t05PolicyPath(t, map[string]string{"ddl.table.truncate.exists.require": ""}))
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("unknown TRUNCATE must stay silent under this slice, got findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
				t.Fatalf("unknown TRUNCATE must stay complete/pass, got coverage=%s verdict=%s", result.Statements[0].Coverage.Status, result.Verdict)
			}
		})
	}
}

func TestBatchStateA3IdentitySeparation(t *testing.T) {
	t.Parallel()
	t.Run("qualified schema separation", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"a.t": presentTable("a", "t", "id"),
			"b.t": presentTable("b", "t", "id"),
		}}
		result := t05A3Audit(t, "DROP TABLE a.t; CREATE INDEX ix_a ON a.t(id); CREATE INDEX ix_b ON b.t(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3AbsentProbe(t, result, 1)
		if len(result.Statements[2].Findings) != 0 || len(result.Statements[2].EvidenceGaps) != 0 {
			t.Fatalf("b.t must stay known, got findings=%+v gaps=%+v", result.Statements[2].Findings, result.Statements[2].EvidenceGaps)
		}
	})
	t.Run("dotted identifier stays whole", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.a.b": presentTable("golden", "a.b", "id")}}
		result := t05A3Audit(t, "DROP TABLE `a.b`; CREATE INDEX ix ON `a.b`(id);", spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		t05A3AbsentProbe(t, result, 1)
		if len(provider.calls) != 1 || provider.calls[0] != "golden.a.b" {
			t.Fatalf("provider read ledger = %#v, want [golden.a.b]", provider.calls)
		}
	})
}

func TestBatchStateA3CancellationAndProviderError(t *testing.T) {
	t.Parallel()
	for _, dropSQL := range []string{"DROP TABLE t;", "DROP TABLE IF EXISTS t;"} {
		dropSQL := dropSQL
		t.Run("pre canceled apply publishes nothing "+dropSQL, func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t":  presentTable("golden", "t", "id"),
				"aux.child": t05A2R1DependentSnapshot("aux", "golden", "t"),
			}}
			state := newBatchState(spec.DialectMySQL, "golden", provider)
			ctx := context.Background()
			if _, err := state.preState(ctx, "golden", spec.Table{Schema: "aux", Name: "child"}); err != nil {
				t.Fatalf("load child: %v", err)
			}
			if _, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil {
				t.Fatalf("load target: %v", err)
			}
			childKey := state.keyFor("golden", spec.Table{Schema: "aux", Name: "child"})
			targetKey := state.keyFor("golden", spec.Table{Name: "t"})
			childBefore := cloneTableSnapshot(state.entries[childKey].shape)
			targetBefore := *state.entries[targetKey]
			canceledCtx, cancel := context.WithCancel(ctx)
			cancel()
			defer cancel()
			statements := t05A2R1Extract(t, dropSQL)
			err := state.apply(canceledCtx, statements[0])
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("apply must surface context.Canceled, got %v", err)
			}
			if *state.entries[targetKey] != targetBefore {
				t.Fatalf("target entry must stay unchanged, got %+v", state.entries[targetKey])
			}
			if !reflect.DeepEqual(state.entries[childKey].shape, childBefore) {
				t.Fatal("dependent entry must stay unchanged")
			}
			if len(provider.calls) != 2 {
				t.Fatalf("provider ledger must stay unchanged, got %#v", provider.calls)
			}
		})
	}
	t.Run("mid read cancellation aborts publication", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		provider := &t05A2R1CallbackProvider{
			inner: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t":  presentTable("golden", "t", "id"),
				"aux.child": t05A2R1DependentSnapshot("aux", "golden", "t"),
			}},
			onLoad: func(schema, table string) {
				if schema == "golden" && table == "t" {
					cancel()
				}
			},
		}
		_, err := t05A2R1Enrich(t, ctx, "CREATE INDEX ix_child ON aux.child(id); DROP TABLE t; ALTER TABLE aux.child ADD COLUMN c INT;", provider)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("enrich must surface context.Canceled, got %v", err)
		}
	})
	t.Run("mid read cancellation keeps cached state", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		provider := &t05A2R1CallbackProvider{
			inner: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t":  presentTable("golden", "t", "id"),
				"aux.child": t05A2R1DependentSnapshot("aux", "golden", "t"),
			}},
			onLoad: func(schema, table string) {
				if schema == "golden" && table == "t" {
					cancel()
				}
			},
		}
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		if _, err := state.preState(ctx, "golden", spec.Table{Schema: "aux", Name: "child"}); err != nil {
			t.Fatalf("load child: %v", err)
		}
		if _, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load target: %v", err)
		}
		childKey := state.keyFor("golden", spec.Table{Schema: "aux", Name: "child"})
		targetKey := state.keyFor("golden", spec.Table{Name: "t"})
		childBefore := cloneTableSnapshot(state.entries[childKey].shape)
		targetBefore := *state.entries[targetKey]
		statements := t05A2R1Extract(t, "DROP TABLE t;")
		err := state.apply(ctx, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply must surface context.Canceled, got %v", err)
		}
		if *state.entries[targetKey] != targetBefore || state.entries[targetKey].state != tablePresent {
			t.Fatalf("target entry must stay present, got %+v", state.entries[targetKey])
		}
		if state.entries[childKey].state != tablePresent || !reflect.DeepEqual(state.entries[childKey].shape, childBefore) {
			t.Fatalf("dependent must stay present with its original shape, got %+v", state.entries[childKey])
		}
		if len(provider.inner.calls) != 2 {
			t.Fatalf("provider ledger must stay unchanged, got %#v", provider.inner.calls)
		}
	})
	t.Run("provider error keeps identity over cancellation", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sentinel drop read failure")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		provider := &t05A2R1CallbackProvider{
			inner: &t05PresentProvider{
				snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")},
				errOn:     map[string]error{"golden.t": wantErr},
			},
			onLoad: func(schema, table string) {
				if schema == "golden" && table == "t" {
					cancel()
				}
			},
		}
		_, err := t05A2R1Enrich(t, ctx, "DROP TABLE t;", provider)
		if !errors.Is(err, wantErr) {
			t.Fatalf("provider error identity must survive, got %v", err)
		}
		if errors.Is(err, context.Canceled) {
			t.Fatal("provider error must not collapse into cancellation")
		}
	})
	t.Run("provider error propagates from pre state", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sentinel drop read failure")
		provider := &t05PresentProvider{
			snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "id")},
			errOn:     map[string]error{"golden.t": wantErr},
		}
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "DROP TABLE t;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05A3FiveRulePolicy(t),
			MetadataProvider: provider,
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("audit must propagate the provider error, got %v", err)
		}
	})
}

func TestBatchStateA3DropForbidOrthogonal(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t05RuleCreateForbid:       {level: "blocker"},
		t05RuleDropExistsRequire:  {level: "blocker"},
		t05RuleDropForbid:         {level: "blocker", params: "      forbid: true\n"},
		t05RuleAlterRequire:       {level: "blocker"},
		t05RuleAddColumnForbid:    {level: "blocker"},
		t05RuleCreateIndexColumns: {level: "blocker", params: "      required: true\n"},
	})
	provider := &t05AbsentProvider{}
	result := t05A3Audit(t, "CREATE TABLE t (old_c INT PRIMARY KEY); DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY);", spec.DialectMySQL, provider, policy)
	findings := t05FindingsByRule(result, 1, t05RuleDropForbid)
	if len(findings) != 1 {
		t.Fatalf("drop forbid must fire exactly once, got %+v", result.Statements[1].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("drop forbid statement must not emit gaps, got %+v", result.Statements[1].EvidenceGaps)
	}
	if len(result.Statements[2].Findings) != 0 || len(result.Statements[2].EvidenceGaps) != 0 || result.Statements[2].Coverage.Status != report.CoverageComplete {
		t.Fatalf("recreate after forbid-blocked drop must stay clean, got findings=%+v gaps=%+v coverage=%+v", result.Statements[2].Findings, result.Statements[2].EvidenceGaps, result.Statements[2].Coverage)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict = %s, want reject", result.Verdict)
	}
}

func TestBatchStateA3IndependentAuditCalls(t *testing.T) {
	t.Parallel()
	sql := strings.Join(t05A3DropRecreateStatements("DROP TABLE t"), "; ") + ";"
	for i := 0; i < 2; i++ {
		provider := &t05AbsentProvider{}
		result := t05A3Audit(t, sql, spec.DialectMySQL, provider, t05A3FiveRulePolicy(t))
		if result.Verdict != report.VerdictPass {
			t.Fatalf("call %d verdict = %s, want pass", i, result.Verdict)
		}
		if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
			t.Fatalf("call %d provider ledger = %#v, want [golden.t]", i, provider.calls)
		}
	}
}
