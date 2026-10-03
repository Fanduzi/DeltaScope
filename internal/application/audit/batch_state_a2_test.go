// Package audit verifies the T05-A2 bounded single-pair RENAME identity
// migration inside the request-local ordered state.
// input: ordered MySQL/TiDB statements exercising the frozen endpoint
// existence table, both equivalent rename syntaxes, loaded dependent
// references, plain primary-key constraint payloads, and cancellation timing
// output: deterministic state transitions, provider read ledgers, dependent
// invalidation boundaries, cancellation error identity, and the unchanged
// policy/coverage contract for in-scope and out-of-scope forms
// pos: T05-A2/A2-R1 regression matrix for the conditional-structure state machine
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

const t05RuleRenameTableForbid = "ddl.alter.rename_table.forbid"

// t05PresentProvider answers recorded snapshots by (schema, table) key and
// absent for keys registered with a nil marker via t05AbsentKey sentinel —
// a missing map entry means "provider silent" (unknown), like the shared
// dmlTableMetadataProvider semantics.
type t05PresentProvider struct {
	snapshots map[string]*spec.TableSnapshot
	calls     []string
	errOn     map[string]error
}

func (p *t05PresentProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t05PresentProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	key := schema + "." + table
	p.calls = append(p.calls, key)
	if p.errOn != nil {
		if err, hit := p.errOn[key]; hit {
			return nil, err
		}
	}
	return p.snapshots[key], nil
}

func presentTable(schema, table string, columns ...string) *spec.TableSnapshot {
	snapshot := &spec.TableSnapshot{
		Exists:      true,
		Schema:      schema,
		Table:       &spec.Table{Schema: schema, Name: table},
		Columns:     make([]spec.Column, 0, len(columns)),
		Indexes:     []spec.Index{},
		Constraints: []spec.Constraint{},
	}
	for _, name := range columns {
		snapshot.Columns = append(snapshot.Columns, spec.Column{Name: name, Type: "int"})
	}
	return snapshot
}

func absentTable(schema, table string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:      false,
		Schema:      schema,
		Table:       &spec.Table{Schema: schema, Name: table},
		Columns:     []spec.Column{},
		Indexes:     []spec.Index{},
		Constraints: []spec.Constraint{},
	}
}

// A2 statement probe: the last probe statement decodes the post-state of the
// probed name — present means clean derivation, absent means a settled
// table_not_found blocker, unknown means the unknown_table_state gap.
func t05ProbeCreateIndex(t *testing.T, result report.Result, index int, want string) {
	t.Helper()
	statement := result.Statements[index]
	switch want {
	case "present":
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d must be clean on a present table, got findings=%+v gaps=%+v", index, statement.Findings, statement.EvidenceGaps)
		}
	case "absent":
		findings := t05FindingsByRule(result, index, t05RuleCreateIndexColumns)
		if len(findings) != 1 {
			t.Fatalf("statement %d must emit exactly one table_not_found blocker, got %+v", index, statement.Findings)
		}
		if findings[0].Metadata["reason"] != "table_not_found" {
			t.Fatalf("statement %d finding reason = %+v, want table_not_found", index, findings[0].Metadata)
		}
		if len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d on an absent table must not emit gaps, got %+v", index, statement.EvidenceGaps)
		}
	case "unknown":
		if len(statement.Findings) != 0 {
			t.Fatalf("statement %d on unknown state must not produce findings, got %+v", index, statement.Findings)
		}
		gaps := t05GapsByRule(result, index, t05RuleCreateIndexColumns)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("statement %d must report one unknown_table_state gap, got %+v", index, statement.EvidenceGaps)
		}
	default:
		t.Fatalf("unknown probe expectation %q", want)
	}
}

// A: the frozen first path — a single-pair RENAME moves the derived shape to
// the destination identity and frees the source name. Both syntaxes and the
// schema controls share one assertion core.
func TestBatchStateA2RenameMigratesIdentity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		renameSQL string
		followSQL string
		wantCalls []string
	}{
		{name: "standalone qualified", renameSQL: "RENAME TABLE src.t TO dst.t2",
			followSQL: "dst.t2", wantCalls: []string{"src.t", "dst.t2"}},
		{name: "alter rename to", renameSQL: "ALTER TABLE src.t RENAME TO dst.t2",
			followSQL: "dst.t2", wantCalls: []string{"src.t", "dst.t2"}},
		{name: "same schema", renameSQL: "RENAME TABLE src.t TO src.t2",
			followSQL: "src.t2", wantCalls: []string{"src.t", "src.t2"}},
		{name: "same name cross schema", renameSQL: "RENAME TABLE src.t TO dst.t",
			followSQL: "dst.t", wantCalls: []string{"src.t", "dst.t"}},
		{name: "unqualified destination uses request schema", renameSQL: "RENAME TABLE src.t TO t2",
			followSQL: "t2", wantCalls: []string{"src.t", "golden.t2"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			sql := "CREATE TABLE src.t (id INT PRIMARY KEY); " + tc.renameSQL + ";" +
				" ALTER TABLE " + tc.followSQL + " ADD COLUMN c INT; CREATE INDEX idx_c ON " + tc.followSQL + "(c);"
			result, err := AuditSQL(context.Background(), Request{
				SQL:              sql,
				Dialect:          spec.DialectMySQL,
				Schema:           "golden",
				ConfigPath:       t05FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 4 {
				t.Fatalf("expected 4 statements, got %d", len(result.Statements))
			}
			for i := range result.Statements {
				if len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d must stay clean, got %+v", i, result.Statements[i].Findings)
				}
				if len(result.Statements[i].EvidenceGaps) != 0 {
					t.Fatalf("statement %d must not report gaps after migration, got %+v", i, result.Statements[i].EvidenceGaps)
				}
				if result.Statements[i].Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %+v, want complete", i, result.Statements[i].Coverage)
				}
			}
			if result.Verdict != report.VerdictPass {
				t.Fatalf("verdict must be pass after migration, got %s", result.Verdict)
			}
			if strings.Join(provider.calls, ",") != strings.Join(tc.wantCalls, ",") {
				t.Fatalf("provider read ledger = %#v, want %#v", provider.calls, tc.wantCalls)
			}
		})
	}
}

// enrichA2 runs the shared parse→extract→enrich seam directly so tests can
// observe per-statement pre-state snapshots the public result does not carry.
func enrichA2(t *testing.T, sql string, provider MetadataProvider) []spec.Statement {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectMySQL)
	if parseErr != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return enriched
}

// A2: the rename statement's own pre-state still shows the source shape —
// migration must never mutate the earlier projection.
func TestBatchStateA2RenameKeepsSourcePreState(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	enriched := enrichA2(t,
		"CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;", provider)
	snapshot := enriched[1].Metadata.TargetTable
	if snapshot == nil || !snapshot.Exists || snapshot.Table == nil || snapshot.Table.Name != "t" || snapshot.Table.Schema != "src" {
		t.Fatalf("rename pre-state must be the derived src.t shape, got %+v", snapshot)
	}
	if snapshot.FindColumn("id") == nil || snapshot.PrimaryKey == nil {
		t.Fatalf("rename pre-state must carry the derived id+PK shape, got %+v", snapshot)
	}
}

// B: after a migrated rename the freed source name is a settled absent fact —
// touching it produces a deterministic table_not_found blocker, not a gap.
func TestBatchStateA2OldNameStaysAbsent(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;" +
			" ALTER TABLE dst.t2 ADD COLUMN c INT; CREATE INDEX idx_c ON dst.t2(c);" +
			" CREATE INDEX ix_old ON src.t(id);",
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i := 0; i < 4; i++ {
		if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d must stay clean, got findings=%+v gaps=%+v", i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
		}
	}
	last := result.Statements[4]
	findings := t05FindingsByRule(result, 4, t05RuleCreateIndexColumns)
	if len(findings) != 1 {
		t.Fatalf("old-name index must emit exactly one blocker, got %+v", last.Findings)
	}
	meta := findings[0].Metadata
	if meta["reason"] != "table_not_found" || meta["schema"] != "src" || meta["table"] != "t" || meta["index"] != "ix_old" {
		t.Fatalf("blocker metadata = %+v, want schema=src table=t index=ix_old reason=table_not_found", meta)
	}
	if len(last.EvidenceGaps) != 0 {
		t.Fatalf("absent old name must not emit gaps, got %+v", last.EvidenceGaps)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("blocker must reject, got %s", result.Verdict)
	}
	if strings.Join(provider.calls, ",") != "src.t,dst.t2" {
		t.Fatalf("the freed name must never be re-read, got %#v", provider.calls)
	}
}

// C: outside the single success cell both ends tombstone — a known-absent
// source or a known-present destination invalidates the pair, and the ALTER
// form keeps its own source-existence finding on the rename statement itself.
func TestBatchStateA2RenamePremiseCellsInvalidate(t *testing.T) {
	t.Parallel()

	t.Run("source absent invalidates both ends", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_s ON src.t(id); CREATE INDEX ix_d ON dst.t2(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		t05ProbeCreateIndex(t, result, 1, "unknown")
		t05ProbeCreateIndex(t, result, 2, "unknown")
	})

	t.Run("destination present invalidates both ends", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"dst.t2": presentTable("dst", "t2", "id"),
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_d ON dst.t2(id); CREATE INDEX ix_s ON src.t(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		t05ProbeCreateIndex(t, result, 2, "unknown")
		t05ProbeCreateIndex(t, result, 3, "unknown")
	})

	t.Run("alter rename keeps source existence finding", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE src.t RENAME TO dst.t2; CREATE INDEX ix_d ON dst.t2(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		findings := t05FindingsByRule(result, 0, t05RuleAlterRequire)
		if len(findings) != 1 {
			t.Fatalf("alter rename on an absent source keeps the existing existence finding, got %+v", result.Statements[0].Findings)
		}
		t05ProbeCreateIndex(t, result, 1, "unknown")
	})
}

// D: the frozen existence grid — every non-success cell tombstones both ends;
// nothing fabricates new absent/present facts out of unknown inputs.
func TestBatchStateA2ExistenceGrid(t *testing.T) {
	t.Parallel()

	present := presentTable("src", "t", "id")
	incomplete := &spec.TableSnapshot{
		Exists:             true,
		Schema:             "src",
		Table:              &spec.Table{Schema: "src", Name: "t"},
		PrimaryKeyUnknown:  true,
		IndexesUnknown:     true,
		ConstraintsUnknown: true,
	}

	cases := []struct {
		name     string
		src      *spec.TableSnapshot
		dst      *spec.TableSnapshot
		srcProbe string
		dstProbe string
	}{
		{name: "present+absent migrates", src: present, dst: absentTable("dst", "t2"), srcProbe: "absent", dstProbe: "present"},
		{name: "present+present", src: present, dst: presentTable("dst", "t2", "id"), srcProbe: "unknown", dstProbe: "unknown"},
		{name: "present+unknown", src: present, dst: nil, srcProbe: "unknown", dstProbe: "unknown"},
		{name: "absent+absent", src: absentTable("src", "t"), dst: absentTable("dst", "t2"), srcProbe: "unknown", dstProbe: "unknown"},
		{name: "absent+present", src: absentTable("src", "t"), dst: presentTable("dst", "t2", "id"), srcProbe: "unknown", dstProbe: "unknown"},
		{name: "absent+unknown", src: absentTable("src", "t"), dst: nil, srcProbe: "unknown", dstProbe: "unknown"},
		{name: "unknown+absent cannot derive source absent", src: nil, dst: absentTable("dst", "t2"), srcProbe: "unknown", dstProbe: "unknown"},
		{name: "unknown+present", src: nil, dst: presentTable("dst", "t2", "id"), srcProbe: "unknown", dstProbe: "unknown"},
		{name: "unknown+unknown", src: nil, dst: nil, srcProbe: "unknown", dstProbe: "unknown"},
		{name: "present-incomplete+absent migrates incompletely", src: incomplete, dst: absentTable("dst", "t2"), srcProbe: "absent", dstProbe: "present-incomplete"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snapshots := map[string]*spec.TableSnapshot{}
			if tc.src != nil {
				snapshots["src.t"] = tc.src
			}
			if tc.dst != nil {
				snapshots["dst.t2"] = tc.dst
			}
			provider := &t05PresentProvider{snapshots: snapshots}
			result, err := AuditSQL(context.Background(), Request{
				SQL: "RENAME TABLE src.t TO dst.t2;" +
					" CREATE INDEX ix_s ON src.t(id); CREATE INDEX ix_d ON dst.t2(id);",
				Dialect:          spec.DialectMySQL,
				Schema:           "golden",
				ConfigPath:       t05FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			t05ProbeCreateIndex(t, result, 1, tc.srcProbe)
			if tc.dstProbe == "present-incomplete" {
				// Existence migrated but member collections stay unknown —
				// the column member check still reports its bounded gap.
				gaps := t05GapsByRule(result, 2, t05RuleCreateIndexColumns)
				if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" ||
					len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.columns" {
					t.Fatalf("present-incomplete destination must keep the column gap, got %+v", result.Statements[2].EvidenceGaps)
				}
				if len(result.Statements[2].Findings) != 0 {
					t.Fatalf("present-incomplete destination must not fabricate findings, got %+v", result.Statements[2].Findings)
				}
			} else {
				t05ProbeCreateIndex(t, result, 2, tc.dstProbe)
			}
		})
	}
}

// E: contamination boundaries and out-of-scope forms still invalidate every
// endpoint without re-reading the provider; unrelated tables keep working.
func TestBatchStateA2ContaminationAndForms(t *testing.T) {
	t.Parallel()

	t.Run("execute contamination survives rename", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); EXECUTE stmt;" +
				" RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_d ON dst.t2(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err == nil {
			t.Fatalf("expected unsupported error for EXECUTE, got result=%#v", result)
		}
		if len(result.Statements) < 4 {
			t.Fatalf("expected partial statements, got %#v", result.Statements)
		}
		t05ProbeCreateIndex(t, result, 3, "unknown")
		if strings.Join(provider.calls, ",") != "src.t" {
			t.Fatalf("contamination must not let rename re-read the provider, got %#v", provider.calls)
		}
	})

	t.Run("multi-pair rename invalidates every endpoint", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE a.t (id INT PRIMARY KEY); CREATE TABLE c.t (id INT PRIMARY KEY);" +
				" RENAME TABLE a.t TO b.t2, c.t TO d.t4;" +
				" CREATE INDEX i1 ON a.t(id); CREATE INDEX i2 ON b.t2(id);" +
				" CREATE INDEX i3 ON c.t(id); CREATE INDEX i4 ON d.t4(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		for i := 3; i <= 6; i++ {
			t05ProbeCreateIndex(t, result, i, "unknown")
		}
	})

	t.Run("mixed alter invalidates both ends", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); ALTER TABLE src.t RENAME TO dst.t2, ADD COLUMN z INT;" +
				" CREATE INDEX ix_s ON src.t(id); CREATE INDEX ix_d ON dst.t2(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		t05ProbeCreateIndex(t, result, 2, "unknown")
		t05ProbeCreateIndex(t, result, 3, "unknown")
	})

	t.Run("unrelated table survives rename", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE app.u (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;" +
				" ALTER TABLE app.u ADD COLUMN c INT; CREATE INDEX iu ON app.u(c);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		for i := 0; i <= 3; i++ {
			if i == 1 {
				continue // the rename itself is unconsumed by the four rules
			}
			if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
				t.Fatalf("unrelated path statement %d must stay clean, got findings=%+v gaps=%+v", i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
			}
		}
	})
}

// F: a policy blocker on the rename action is orthogonal to state migration —
// the structural transition still runs and later checks stay deterministic.
// ddl.alter.rename_table.forbid only applies to the ALTER form (standalone
// RENAME TABLE is governed by ddl.rename_table.notice), so the probe uses
// ALTER TABLE ... RENAME TO.
func TestBatchStateA2PolicyBlockerKeepsMigration(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	policy := t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:       "",
		t05RuleAlterRequire:       "",
		t05RuleAddColumnForbid:    "",
		t05RuleCreateIndexColumns: "      required: true\n",
		t05RuleRenameTableForbid:  "",
	})
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); ALTER TABLE src.t RENAME TO dst.t2;" +
			" ALTER TABLE dst.t2 ADD COLUMN c INT; CREATE INDEX idx_c ON dst.t2(c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	forbidFindings := t05FindingsByRule(result, 1, t05RuleRenameTableForbid)
	if len(forbidFindings) == 0 {
		t.Fatalf("the rename policy blocker must still fire, got %+v", result.Statements[1].Findings)
	}
	for i := 2; i <= 3; i++ {
		if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d must stay deterministic under the policy blocker, got findings=%+v gaps=%+v", i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
		}
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("policy blocker must reject, got %s", result.Verdict)
	}
}

// G: read discipline — destinations resolve at most once, an already-invalidated
// destination is never re-asked, chains migrate deterministically, and provider
// errors abort without publishing a half-written post-state.
func TestBatchStateA2ReadLedger(t *testing.T) {
	t.Parallel()

	t.Run("destination first-touch resolves once", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		_, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;" +
				" RENAME TABLE dst.t2 TO dst.t3; ALTER TABLE dst.t3 ADD COLUMN c INT;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		sort.Strings(provider.calls)
		if strings.Join(provider.calls, ",") != "dst.t2,dst.t3,src.t" {
			t.Fatalf("each rename endpoint must resolve at most once, got %#v", provider.calls)
		}
	})

	t.Run("invalidated destination is never re-read", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); CREATE TABLE dst.t2 (id INT PRIMARY KEY);" +
				" ALTER TABLE dst.t2 ADD COLUMN a INT, ADD COLUMN b INT;" +
				" RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_d ON dst.t2(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		// dst.t2 was read once at its own CREATE, then invalidated in-batch;
		// the rename must reuse the cached tombstone — never a second read —
		// and invalidate both ends.
		if strings.Join(provider.calls, ",") != "src.t,dst.t2" {
			t.Fatalf("provider must never re-read an invalidated destination, got %#v", provider.calls)
		}
		t05ProbeCreateIndex(t, result, 4, "unknown")
	})

	t.Run("destination provider error aborts without half post-state", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("destination read failed")
		provider := &t05PresentProvider{
			snapshots: map[string]*spec.TableSnapshot{"src.t": absentTable("src", "t")},
			errOn:     map[string]error{"dst.t2": wantErr},
		}
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err == nil || !strings.Contains(err.Error(), "destination read failed") {
			t.Fatalf("destination read error must propagate, got %v", err)
		}
	})

	t.Run("second audit does not inherit derived state", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		request := Request{
			SQL:              "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		}
		for run := 0; run < 2; run++ {
			if _, err := AuditSQL(context.Background(), request); err != nil {
				t.Fatalf("audit %d: %v", run, err)
			}
		}
		sort.Strings(provider.calls)
		if strings.Join(provider.calls, ",") != "dst.t2,dst.t2,src.t,src.t" {
			t.Fatalf("each audit must read its own request-local facts, got %#v", provider.calls)
		}
	})
}

// H: member transport boundary — known non-PK constraints conservatively
// invalidate the pair (MySQL may rebind generated constraint names), unknown
// collections keep their flags, and a migrated PK stays provable.
func TestBatchStateA2MemberTransportBoundary(t *testing.T) {
	t.Parallel()

	t.Run("known constraint shape invalidates both ends", func(t *testing.T) {
		t.Parallel()
		src := presentTable("src", "t", "id")
		src.Constraints = []spec.Constraint{{Type: "check", Name: "c_positive"}}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"src.t":  src,
			"dst.t2": absentTable("dst", "t2"),
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_d ON dst.t2(id); CREATE INDEX ix_s ON src.t(id);",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		t05ProbeCreateIndex(t, result, 1, "unknown")
		t05ProbeCreateIndex(t, result, 2, "unknown")
	})

	t.Run("same effective key invalidates instead of self-migration", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO src.t; ALTER TABLE src.t ADD COLUMN c INT;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t05FourRulePolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		gaps := t05GapsByRule(result, 2, t05RuleAlterRequire)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("same-key rename must conservatively invalidate, got %+v", result.Statements[2].EvidenceGaps)
		}
	})

	t.Run("migrated primary key stays provable", func(t *testing.T) {
		t.Parallel()
		provider := &t05AbsentProvider{}
		enriched := enrichA2(t,
			"CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;"+
				" ALTER TABLE dst.t2 DROP PRIMARY KEY;", provider)
		snapshot := enriched[2].Metadata.TargetTable
		if snapshot == nil || snapshot.PrimaryKey == nil {
			t.Fatalf("the migrated destination must still carry the primary key, got %+v", snapshot)
		}
	})

	t.Run("renamed shape keeps statistics values", func(t *testing.T) {
		t.Parallel()
		rows := int64(42)
		src := presentTable("src", "t", "id")
		src.Options = map[string]string{"table_rows": "42"}
		src.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &rows}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"src.t":  src,
			"dst.t2": absentTable("dst", "t2"),
		}}
		enriched := enrichA2(t,
			"RENAME TABLE src.t TO dst.t2; ALTER TABLE dst.t2 ADD COLUMN c INT;", provider)
		snapshot := enriched[1].Metadata.TargetTable
		if snapshot == nil || snapshot.Options["table_rows"] != "42" || snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Cardinality == nil || *snapshot.PrimaryKey.Cardinality != 42 {
			t.Fatalf("migration must preserve observed statistics, got %+v", snapshot)
		}
		// The provider's own object must not be mutated by the migration.
		if src.Table == nil || src.Table.Name != "t" || src.Table.Schema != "src" || src.Schema != "src" {
			t.Fatalf("provider snapshot identity must stay on the source name, got %+v", src.Table)
		}
	})
}

var t05A2R1RenameForms = []struct {
	label string
	sql   string
}{
	{label: "rename table", sql: "RENAME TABLE src.t TO dst.t2"},
	{label: "alter rename to", sql: "ALTER TABLE src.t RENAME TO dst.t2"},
}

func t05A2R1DependentSnapshot(owner, refSchema, refTable string) *spec.TableSnapshot {
	child := presentTable(owner, "child", "id")
	child.Constraints = []spec.Constraint{{
		Type:              "foreign_key",
		Name:              "fk_child_ref",
		Columns:           []string{"id"},
		ReferencedSchema:  refSchema,
		ReferencedTable:   refTable,
		ReferencedColumns: []string{"id"},
	}}
	return child
}

func t05A2R1Target(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.TargetTable
}

func t05A2R1AssertUnknownGap(t *testing.T, gaps []rule.EvidenceGap, wantFacts []string) {
	t.Helper()
	if len(gaps) != 1 {
		t.Fatalf("expected exactly one evidence gap, got %+v", gaps)
	}
	if gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("gap reason = %q, want unknown_table_state", gaps[0].ReasonCode)
	}
	if !reflect.DeepEqual(gaps[0].RequiredFacts, wantFacts) {
		t.Fatalf("gap required_facts = %#v, want %#v", gaps[0].RequiredFacts, wantFacts)
	}
}

func t05A2R1Extract(t *testing.T, sql string) []spec.Statement {
	t.Helper()
	parsed, err := parseSQL(context.Background(), sql, spec.DialectMySQL)
	if err != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse: err=%v statements=%d", err, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return statements
}

func t05A2R1Enrich(t *testing.T, ctx context.Context, sql string, provider MetadataProvider) ([]spec.Statement, error) {
	t.Helper()
	parsed, err := parseSQL(ctx, sql, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return enrichStatementsWithMetadata(ctx, spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
}

type t05A2R1CallbackProvider struct {
	inner  *t05PresentProvider
	onLoad func(schema, table string)
}

func (p *t05A2R1CallbackProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}

func (p *t05A2R1CallbackProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	if p.onLoad != nil {
		p.onLoad(schema, table)
	}
	return p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
}

func t05A2R1PrepareRename(t *testing.T, provider MetadataProvider, statements []spec.Statement, cacheDestination bool) *batchState {
	t.Helper()
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	ctx := context.Background()
	load := statements[0]
	if load.DDL == nil || load.DDL.Table == nil {
		t.Fatalf("setup statement must carry a table target, got %+v", load.DDL)
	}
	if _, err := state.preState(ctx, "golden", *load.DDL.Table); err != nil {
		t.Fatalf("load dependent: %v", err)
	}
	if err := state.apply(ctx, load); err != nil {
		t.Fatalf("apply dependent probe: %v", err)
	}
	if _, err := state.preState(ctx, "golden", spec.Table{Schema: "src", Name: "t"}); err != nil {
		t.Fatalf("load source: %v", err)
	}
	if cacheDestination {
		if _, err := state.preState(ctx, "golden", spec.Table{Schema: "dst", Name: "t2"}); err != nil {
			t.Fatalf("load destination: %v", err)
		}
	}
	return state
}

func TestBatchStateA2R1LoadedDependentInvalidates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		owner      string
		refSchema  string
		refTable   string
		check      bool
		dstPresent bool
		wantChild  string
	}{
		{name: "explicit src reference under aux owner", owner: "aux", refSchema: "src", refTable: "t", wantChild: "unknown"},
		{name: "unqualified reference under src owner", owner: "src", refSchema: "", refTable: "t", wantChild: "unknown"},
		{name: "explicit src reference with check source", owner: "aux", refSchema: "src", refTable: "t", check: true, wantChild: "unknown"},
		{name: "unqualified reference with check source", owner: "src", refSchema: "", refTable: "t", check: true, wantChild: "unknown"},
		{name: "unqualified reference under aux owner stays local", owner: "aux", refSchema: "", refTable: "t", wantChild: "known"},
		{name: "explicit other-schema same-name reference stays known", owner: "aux", refSchema: "other", refTable: "t", wantChild: "known"},
		{name: "destination reference invalidates on non-move cell", owner: "aux", refSchema: "dst", refTable: "t2", dstPresent: true, wantChild: "unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, form := range t05A2R1RenameForms {
				form := form
				t.Run(form.label, func(t *testing.T) {
					childKey := tc.owner + ".child"
					child := t05A2R1DependentSnapshot(tc.owner, tc.refSchema, tc.refTable)
					src := presentTable("src", "t", "id")
					src.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
					if tc.check {
						src.Constraints = []spec.Constraint{{Type: "check", Name: "c_positive"}}
					}
					dst := absentTable("dst", "t2")
					if tc.dstPresent {
						dst = presentTable("dst", "t2", "id")
					}
					unrelated := presentTable("other", "u", "id")
					unrelated.Columns[0].NotNull = true
					provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
						childKey:  child,
						"other.u": unrelated,
						"src.t":   src,
						"dst.t2":  dst,
					}}
					wantChildSnapshot := cloneTableSnapshot(child)
					wantSrcSnapshot := cloneTableSnapshot(src)

					enriched := enrichA2(t,
						"CREATE INDEX ix_child ON "+tc.owner+".child(id); "+
							"CREATE INDEX ix_u ON other.u(id); "+form.sql+"; "+
							"ALTER TABLE "+tc.owner+".child ADD COLUMN c INT; "+
							"CREATE INDEX ix_after_u ON other.u(id);", provider)
					if len(enriched) != 5 {
						t.Fatalf("expected 5 enriched statements, got %d", len(enriched))
					}

					before := t05A2R1Target(enriched[0])
					if before == nil || !before.Exists || len(before.Constraints) != 1 ||
						before.Constraints[0].Type != "foreign_key" ||
						before.Constraints[0].ReferencedSchema != tc.refSchema ||
						before.Constraints[0].ReferencedTable != tc.refTable {
						t.Fatalf("child pre-rename projection must keep the original reference, got %+v", before)
					}

					after := t05A2R1Target(enriched[3])
					switch tc.wantChild {
					case "unknown":
						if after != nil {
							t.Fatalf("dependent child must be unknown after the rename touched %s.%s, got %+v", tc.refSchema, tc.refTable, after)
						}
					case "known":
						if after == nil || !after.Exists || after.FindColumn("id") == nil ||
							len(after.Constraints) != 1 ||
							after.Constraints[0].ReferencedSchema != tc.refSchema ||
							after.Constraints[0].ReferencedTable != tc.refTable {
							t.Fatalf("non-dependent child must keep its loaded facts, got %+v", after)
						}
					default:
						t.Fatalf("unknown expectation %q", tc.wantChild)
					}

					u := t05A2R1Target(enriched[4])
					if u == nil || !u.Exists || u.FindColumn("id") == nil || !u.FindColumn("id").NotNull ||
						!u.HasIndex("ix_u") || len(u.Constraints) != 0 {
						t.Fatalf("unrelated other.u must stay known with its original flags, got %+v", u)
					}

					if !reflect.DeepEqual(provider.snapshots[childKey], wantChildSnapshot) {
						t.Fatalf("provider child snapshot must stay immutable, got %+v", provider.snapshots[childKey])
					}
					if !reflect.DeepEqual(provider.snapshots["src.t"], wantSrcSnapshot) {
						t.Fatalf("provider source snapshot must stay immutable, got %+v", provider.snapshots["src.t"])
					}

					wantCalls := []string{childKey, "other.u", "src.t", "dst.t2"}
					if strings.Join(provider.calls, ",") != strings.Join(wantCalls, ",") {
						t.Fatalf("provider ledger = %#v, want %#v — no rescan, no unloaded-table discovery", provider.calls, wantCalls)
					}
				})
			}
		})
	}
}

func TestBatchStateA2R1LoadedDependentKeepsPublicGapContract(t *testing.T) {
	t.Parallel()
	for _, form := range t05A2R1RenameForms {
		form := form
		t.Run(form.label, func(t *testing.T) {
			src := presentTable("src", "t", "id")
			src.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"aux.child": t05A2R1DependentSnapshot("aux", "src", "t"),
				"other.u":   presentTable("other", "u", "id"),
				"src.t":     src,
				"dst.t2":    absentTable("dst", "t2"),
			}}
			result, err := AuditSQL(context.Background(), Request{
				SQL: "CREATE INDEX ix_child ON aux.child(id); CREATE INDEX ix_u ON other.u(id); " + form.sql + ";" +
					" ALTER TABLE aux.child ADD COLUMN c INT; CREATE INDEX ix_after_u ON other.u(id);",
				Dialect:          spec.DialectMySQL,
				Schema:           "golden",
				ConfigPath:       t05FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 5 {
				t.Fatalf("expected 5 statements, got %d", len(result.Statements))
			}
			for _, i := range []int{0, 1, 2, 4} {
				statement := result.Statements[i]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d must stay clean, got findings=%+v gaps=%+v", i, statement.Findings, statement.EvidenceGaps)
				}
				if statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %s, want complete", i, statement.Coverage.Status)
				}
			}
			affected := result.Statements[3]
			if len(affected.Findings) != 0 {
				t.Fatalf("dependent ADD must not fabricate findings, got %+v", affected.Findings)
			}
			if len(affected.EvidenceGaps) != 2 {
				t.Fatalf("dependent ADD must report exactly the two frozen existence gaps, got %+v", affected.EvidenceGaps)
			}
			t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 3, t05RuleAlterRequire), []string{"target_table.existence"})
			t05A2R1AssertUnknownGap(t, t05GapsByRule(result, 3, t05RuleAddColumnForbid), []string{"target_table.columns", "target_table.existence"})
			if affected.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("dependent ADD coverage = %s, want unverified", affected.Coverage.Status)
			}
			if result.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("result coverage = %s, want unverified", result.Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("verdict = %s, want review", result.Verdict)
			}
			wantCalls := []string{"aux.child", "other.u", "src.t", "dst.t2"}
			if strings.Join(provider.calls, ",") != strings.Join(wantCalls, ",") {
				t.Fatalf("provider ledger = %#v, want %#v", provider.calls, wantCalls)
			}
		})
	}
}

func TestBatchStateA2R1PrimaryKeyPayloadMigrates(t *testing.T) {
	t.Parallel()

	profiles := []struct {
		name        string
		constraints []spec.Constraint
	}{
		{name: "primary key field only"},
		{name: "primary key constraint payload", constraints: []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"id"}}}},
	}
	for _, profile := range profiles {
		profile := profile
		t.Run(profile.name, func(t *testing.T) {
			t.Parallel()
			for _, form := range t05A2R1RenameForms {
				form := form
				t.Run(form.label, func(t *testing.T) {
					src := presentTable("src", "t", "id")
					src.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
					if profile.constraints != nil {
						src.Constraints = profile.constraints
					}
					provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
						"src.t":  src,
						"dst.t2": absentTable("dst", "t2"),
					}}
					wantSrc := cloneTableSnapshot(src)
					sql := form.sql + "; CREATE INDEX ix ON dst.t2(id);"

					enriched := enrichA2(t, sql, provider)
					sourceView := t05A2R1Target(enriched[0])
					if sourceView == nil || !sourceView.Exists || sourceView.Schema != "src" ||
						sourceView.Table == nil || sourceView.Table.Name != "t" {
						t.Fatalf("rename pre-state must keep the source identity, got %+v", sourceView)
					}
					destination := t05A2R1Target(enriched[1])
					if destination == nil || !destination.Exists || destination.Schema != "dst" ||
						destination.Table == nil || destination.Table.Name != "t2" {
						t.Fatalf("destination must carry the migrated shape under its own identity, got %+v", destination)
					}
					if destination.FindColumn("id") == nil || destination.PrimaryKey == nil ||
						destination.PrimaryKey.Name != "PRIMARY" ||
						len(destination.PrimaryKey.Columns) != 1 || destination.PrimaryKey.Columns[0] != "id" {
						t.Fatalf("destination must keep column id and the primary key, got %+v", destination)
					}
					if !reflect.DeepEqual(destination.Constraints, src.Constraints) {
						t.Fatalf("destination constraints = %+v, want verbatim copy of %+v", destination.Constraints, src.Constraints)
					}
					if len(destination.Constraints) > 0 {
						destination.Constraints[0].Columns[0] = "mutated"
						destination.PrimaryKey.Columns[0] = "mutated"
						if src.Constraints[0].Columns[0] != "id" || src.PrimaryKey.Columns[0] != "id" ||
							sourceView.Constraints[0].Columns[0] != "id" || sourceView.PrimaryKey.Columns[0] != "id" ||
							sourceView.Constraints[0].Name != "PRIMARY" || sourceView.PrimaryKey.Name != "PRIMARY" {
							t.Fatalf("mutating the destination projection must not reach the provider object or earlier projections")
						}
					}
					if !reflect.DeepEqual(provider.snapshots["src.t"], wantSrc) {
						t.Fatalf("provider source snapshot must stay immutable, got %+v", provider.snapshots["src.t"])
					}

					result, err := AuditSQL(context.Background(), Request{
						SQL:        sql,
						Dialect:    spec.DialectMySQL,
						Schema:     "golden",
						ConfigPath: t05FourRulePolicy(t),
						MetadataProvider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
							"src.t":  cloneTableSnapshot(src),
							"dst.t2": absentTable("dst", "t2"),
						}},
					})
					if err != nil {
						t.Fatalf("audit: %v", err)
					}
					if len(result.Statements) != 2 {
						t.Fatalf("expected 2 statements, got %d", len(result.Statements))
					}
					for i := range result.Statements {
						if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
							t.Fatalf("statement %d must stay clean after PK migration, findings=%+v gaps=%+v",
								i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
						}
						if result.Statements[i].Coverage.Status != report.CoverageComplete {
							t.Fatalf("statement %d coverage = %s, want complete", i, result.Statements[i].Coverage.Status)
						}
					}
					if result.Verdict != report.VerdictPass {
						t.Fatalf("verdict = %s, want pass", result.Verdict)
					}
				})
			}
		})
	}

	t.Run("partial knowledge keeps unknown flags and statistics", func(t *testing.T) {
		t.Parallel()
		for _, form := range t05A2R1RenameForms {
			form := form
			t.Run(form.label, func(t *testing.T) {
				rows := int64(7)
				src := &spec.TableSnapshot{
					Exists:         true,
					Schema:         "src",
					Table:          &spec.Table{Schema: "src", Name: "t"},
					Columns:        nil,
					PrimaryKey:     &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &rows},
					Indexes:        nil,
					IndexesUnknown: true,
					Constraints:    []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"id"}}},
					Options:        map[string]string{"table_rows": "42"},
				}
				provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
					"src.t":  src,
					"dst.t2": absentTable("dst", "t2"),
				}}
				enriched := enrichA2(t, form.sql+"; CREATE INDEX ix ON dst.t2(id);", provider)
				destination := t05A2R1Target(enriched[1])
				if destination == nil || !destination.Exists || destination.Schema != "dst" ||
					destination.Table == nil || destination.Table.Name != "t2" {
					t.Fatalf("partial-knowledge source must still migrate, got %+v", destination)
				}
				if destination.Columns != nil || !destination.IndexesUnknown ||
					destination.ConstraintsUnknown || destination.PrimaryKeyUnknown {
					t.Fatalf("destination must preserve the exact member-knowledge flags, got %+v", destination)
				}
				if destination.PrimaryKey == nil || destination.PrimaryKey.Cardinality == nil || *destination.PrimaryKey.Cardinality != 7 {
					t.Fatalf("destination must keep the primary key with statistics, got %+v", destination.PrimaryKey)
				}
				if destination.Options["table_rows"] != "42" {
					t.Fatalf("destination must keep statistics options, got %+v", destination.Options)
				}
				if len(destination.Constraints) != 1 || destination.Constraints[0].Type != "primary_key" ||
					destination.Constraints[0].Name != "PRIMARY" ||
					len(destination.Constraints[0].Columns) != 1 || destination.Constraints[0].Columns[0] != "id" {
					t.Fatalf("primary-key constraint must migrate verbatim — no synthetic name rewrite, got %+v", destination.Constraints)
				}
			})
		}
	})

	t.Run("primary key payload keeps unknown member flags", func(t *testing.T) {
		t.Parallel()
		for _, form := range t05A2R1RenameForms {
			form := form
			t.Run(form.label, func(t *testing.T) {
				src := presentTable("src", "t", "id")
				src.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
				src.Constraints = []spec.Constraint{{Type: "primary_key", Name: "PRIMARY", Columns: []string{"id"}}}
				src.PrimaryKeyUnknown = true
				src.ConstraintsUnknown = true
				provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
					"src.t":  src,
					"dst.t2": absentTable("dst", "t2"),
				}}
				enriched := enrichA2(t, form.sql+"; CREATE INDEX ix ON dst.t2(id);", provider)
				destination := t05A2R1Target(enriched[1])
				if destination == nil || !destination.Exists {
					t.Fatalf("primary-key payload source must migrate, got %+v", destination)
				}
				if !destination.PrimaryKeyUnknown || !destination.ConstraintsUnknown {
					t.Fatalf("supplied members must not upgrade the collections to known, got PrimaryKeyUnknown=%v ConstraintsUnknown=%v",
						destination.PrimaryKeyUnknown, destination.ConstraintsUnknown)
				}
				if destination.PrimaryKey == nil || len(destination.Constraints) != 1 ||
					destination.Constraints[0].Type != "primary_key" {
					t.Fatalf("supplied primary-key members must migrate verbatim, got %+v", destination)
				}
			})
		}
	})

	t.Run("foreign key on the source keeps both ends unknown", func(t *testing.T) {
		t.Parallel()
		src := presentTable("src", "t", "id")
		src.Constraints = []spec.Constraint{{
			Type:              "foreign_key",
			Name:              "fk_self",
			Columns:           []string{"id"},
			ReferencedSchema:  "ref",
			ReferencedTable:   "r",
			ReferencedColumns: []string{"id"},
		}}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"src.t":  src,
			"dst.t2": absentTable("dst", "t2"),
		}}
		enriched := enrichA2(t,
			"RENAME TABLE src.t TO dst.t2; CREATE INDEX ix_d ON dst.t2(id); CREATE INDEX ix_s ON src.t(id);", provider)
		if t05A2R1Target(enriched[1]) != nil {
			t.Fatalf("destination must stay unknown when the source carries a rebindable constraint, got %+v", t05A2R1Target(enriched[1]))
		}
		if t05A2R1Target(enriched[2]) != nil {
			t.Fatalf("source must stay unknown when it carries a rebindable constraint, got %+v", t05A2R1Target(enriched[2]))
		}
	})
}

func TestBatchStateA2R1RenameObservesCancellation(t *testing.T) {
	t.Parallel()

	childKey := batchTableKey{dialect: spec.DialectMySQL, schema: "aux", table: "child"}
	srcKey := batchTableKey{dialect: spec.DialectMySQL, schema: "src", table: "t"}
	dstKey := batchTableKey{dialect: spec.DialectMySQL, schema: "dst", table: "t2"}

	for _, form := range t05A2R1RenameForms {
		form := form
		t.Run("pre-cancelled context publishes nothing/"+form.label, func(t *testing.T) {
			statements := t05A2R1Extract(t, "CREATE INDEX ix_child ON aux.child(id); "+form.sql+";")
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"aux.child": t05A2R1DependentSnapshot("aux", "src", "t"),
				"src.t":     presentTable("src", "t", "id"),
				"dst.t2":    absentTable("dst", "t2"),
			}}
			state := t05A2R1PrepareRename(t, provider, statements, true)
			childBefore := cloneTableSnapshot(state.entries[childKey].shape)
			srcBefore := cloneTableSnapshot(state.entries[srcKey].shape)
			callsBefore := append([]string(nil), provider.calls...)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancel()
			err := state.apply(ctx, statements[1])
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("rename on a cancelled context must surface context.Canceled, got %v", err)
			}
			if got := state.entries[srcKey]; got == nil || got.state != tablePresent ||
				!reflect.DeepEqual(got.shape, srcBefore) {
				t.Fatalf("cancelled rename must not publish source absence, got %+v", got)
			}
			if got := state.entries[dstKey]; got == nil || got.state != tableAbsent {
				t.Fatalf("cancelled rename must not publish destination presence, got %+v", got)
			}
			if got := state.entries[childKey]; got == nil || got.state != tablePresent || !reflect.DeepEqual(got.shape, childBefore) {
				t.Fatalf("cancelled rename must not invalidate the dependent child, got %+v", got)
			}
			if strings.Join(provider.calls, ",") != strings.Join(callsBefore, ",") {
				t.Fatalf("cancelled rename must not issue provider reads, got %#v want %#v", provider.calls, callsBefore)
			}
		})

		t.Run("cancel inside destination read publishes nothing/"+form.label, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			inner := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"aux.child": t05A2R1DependentSnapshot("aux", "src", "t"),
				"src.t":     presentTable("src", "t", "id"),
				"dst.t2":    absentTable("dst", "t2"),
			}}
			provider := &t05A2R1CallbackProvider{inner: inner, onLoad: func(schema, table string) {
				if schema == "dst" && table == "t2" {
					cancel()
				}
			}}
			statements := t05A2R1Extract(t, "CREATE INDEX ix_child ON aux.child(id); "+form.sql+";")
			state := t05A2R1PrepareRename(t, provider, statements, false)
			childBefore := cloneTableSnapshot(state.entries[childKey].shape)
			srcBefore := cloneTableSnapshot(state.entries[srcKey].shape)

			err := state.apply(ctx, statements[1])
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("a mid-read cancellation must surface context.Canceled, got %v", err)
			}
			if got := state.entries[srcKey]; got == nil || got.state != tablePresent ||
				!reflect.DeepEqual(got.shape, srcBefore) {
				t.Fatalf("cancelled rename must not publish source absence, got %+v", got)
			}
			if got := state.entries[dstKey]; got != nil && got.state == tablePresent {
				t.Fatalf("cancelled rename must not publish destination presence, got %+v", got)
			}
			if got := state.entries[childKey]; got == nil || got.state != tablePresent || !reflect.DeepEqual(got.shape, childBefore) {
				t.Fatalf("cancelled rename must not invalidate the dependent child, got %+v", got)
			}
			if wantCalls := "aux.child,src.t,dst.t2"; strings.Join(inner.calls, ",") != wantCalls {
				t.Fatalf("provider ledger = %#v, want %q", inner.calls, wantCalls)
			}
		})

		t.Run("provider error keeps its identity over cancellation/"+form.label, func(t *testing.T) {
			wantErr := errors.New("sentinel destination read failure")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			inner := &t05PresentProvider{
				snapshots: map[string]*spec.TableSnapshot{
					"aux.child": t05A2R1DependentSnapshot("aux", "src", "t"),
					"src.t":     presentTable("src", "t", "id"),
					"dst.t2":    absentTable("dst", "t2"),
				},
				errOn: map[string]error{"dst.t2": wantErr},
			}
			provider := &t05A2R1CallbackProvider{inner: inner, onLoad: func(schema, table string) {
				if schema == "dst" && table == "t2" {
					cancel()
				}
			}}
			statements := t05A2R1Extract(t, "CREATE INDEX ix_child ON aux.child(id); "+form.sql+";")
			state := t05A2R1PrepareRename(t, provider, statements, false)
			childBefore := cloneTableSnapshot(state.entries[childKey].shape)
			srcBefore := cloneTableSnapshot(state.entries[srcKey].shape)

			err := state.apply(ctx, statements[1])
			if !errors.Is(err, wantErr) {
				t.Fatalf("provider error identity must survive the cancelled context, got %v", err)
			}
			if errors.Is(err, context.Canceled) {
				t.Fatalf("provider error must not be replaced by the cancellation, got %v", err)
			}
			if got := state.entries[srcKey]; got == nil || got.state != tablePresent ||
				!reflect.DeepEqual(got.shape, srcBefore) {
				t.Fatalf("failed rename must not publish source absence, got %+v", got)
			}
			if got := state.entries[childKey]; got == nil || got.state != tablePresent || !reflect.DeepEqual(got.shape, childBefore) {
				t.Fatalf("failed rename must not invalidate the dependent child, got %+v", got)
			}
		})

		t.Run("enrichment propagates mid-read cancellation/"+form.label, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			inner := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"src.t":  presentTable("src", "t", "id"),
				"dst.t2": absentTable("dst", "t2"),
			}}
			provider := &t05A2R1CallbackProvider{inner: inner, onLoad: func(schema, table string) {
				if schema == "dst" && table == "t2" {
					cancel()
				}
			}}
			_, err := t05A2R1Enrich(t, ctx, form.sql+";", provider)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("enrichment must propagate the observed cancellation, got %v", err)
			}
		})
	}
}
