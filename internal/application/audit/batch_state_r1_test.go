// Package audit verifies the T05-A1-R1 rework contract for the ordered
// schema-state path.
// input: multi-statement MySQL/TiDB batches exercising cross-schema identity,
// unbound unsupported effects, multi-action alters, partial provider
// snapshots, and the frozen evidence-gap vocabulary
// output: failing-then-fixed assertions pinning effective-identity
// resolution, scope-bounded invalidation, the three frozen success
// templates, per-collection completeness, and frozen gap reason codes
// pos: application-layer rework regressions for issue #84/T05-A1-R1
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// C1: explicit schema qualifiers name the real identity — `a.t` and `b.t`
// must never share state, and the provider must be consulted under each
// target's own schema, not the request schema.
func TestBatchStateR1QualifiedNamesUseTheirOwnSchema(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE a.t (id INT PRIMARY KEY); ALTER TABLE a.t ADD COLUMN c INT; CREATE INDEX ix ON a.t (c);" +
			" CREATE TABLE b.t (id INT PRIMARY KEY); CREATE INDEX iy ON b.t (missing_c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i := 0; i < 3; i++ {
		if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("a.t statement %d must analyze on derived state, findings=%+v gaps=%+v", i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
		}
	}
	if len(result.Statements[3].Findings) != 0 || len(result.Statements[3].EvidenceGaps) != 0 {
		t.Fatalf("b.t create must derive its own absent pre-state, got findings=%+v gaps=%+v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
	}
	// b.t only has id — the index references missing_c under schema b.
	findings := t05FindingsByRule(result, 4, t05RuleCreateIndexColumns)
	if len(findings) != 1 || findings[0].Metadata["column"] != "missing_c" {
		t.Fatalf("expected one missing-column blocker for b.t, got %+v", result.Statements[4].Findings)
	}
	sort.Strings(provider.calls)
	if strings.Join(provider.calls, ",") != "a.t,b.t" {
		t.Fatalf("provider must be consulted once per qualified identity, got %#v", provider.calls)
	}
}

// C1 counterexample: a qualified create must not leak its shape into a
// different schema's same-named table.
func TestBatchStateR1CrossSchemaNoStateLeak(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE a.t (id INT PRIMARY KEY); CREATE INDEX ix ON b.t (id);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 1, t05RuleCreateIndexColumns)
	if len(findings) != 1 || findings[0].Metadata["reason"] != "table_not_found" {
		t.Fatalf("b.t is confirmed absent — expected one table_not_found blocker, got %+v", result.Statements[1].Findings)
	}
	if findings[0].Metadata["schema"] != "b" || findings[0].Metadata["table"] != "t" {
		t.Fatalf("finding must carry schema=b table=t, got %#v", findings[0].Metadata)
	}
	sort.Strings(provider.calls)
	if strings.Join(provider.calls, ",") != "a.t,b.t" {
		t.Fatalf("provider must see a.t and b.t, got %#v", provider.calls)
	}
}

// C1: mixed qualified/unqualified targets resolve independently.
func TestBatchStateR1MixedQualifiedAndUnqualified(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE app.t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX ix ON t (c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	// app.t and (request-schema) app.t are the same identity — one provider
	// read, fully derived downstream.
	if len(provider.calls) != 1 || provider.calls[0] != "app.t" {
		t.Fatalf("expected exactly one provider read for app.t, got %#v", provider.calls)
	}
	for i := range result.Statements {
		if len(result.Statements[i].Findings) != 0 || len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d must be clean, findings=%+v gaps=%+v", i, result.Statements[i].Findings, result.Statements[i].EvidenceGaps)
		}
	}
}

// C1+A2: a cross-schema RENAME migrates under each end's own schema — the
// source becomes a settled absent fact and the destination carries the
// derived shape. Each endpoint is read exactly once and a stale read under
// either name must never resurrect post-rename facts.
func TestBatchStateR1CrossSchemaRenameMigrates(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE src.t (id INT PRIMARY KEY); RENAME TABLE src.t TO dst.t2;" +
			" ALTER TABLE src.t ADD COLUMN c INT; ALTER TABLE dst.t2 ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	// The freed source name is confirmed absent — a deterministic finding,
	// not an unknown-state gap.
	findings := t05FindingsByRule(result, 2, t05RuleAlterRequire)
	if len(findings) != 1 || findings[0].Metadata["exists"] != false {
		t.Fatalf("statement 2 must report a confirmed-absent finding, got %+v", result.Statements[2].Findings)
	}
	// The destination runs on the migrated shape — clean and complete.
	if len(result.Statements[3].Findings) != 0 || len(result.Statements[3].EvidenceGaps) != 0 {
		t.Fatalf("statement 3 on the migrated destination must be clean, findings=%+v gaps=%+v",
			result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
	}
	sort.Strings(provider.calls)
	if strings.Join(provider.calls, ",") != "dst.t2,src.t" {
		t.Fatalf("each rename endpoint must be read once and never re-asked, got %#v", provider.calls)
	}
}

// C2: an execution-capable unsupported statement with no table scope
// contaminates later knowledge — EXECUTE can run anything, so the derived
// t(id) shape must not survive it.
func TestBatchStateR1ExecuteContaminatesLaterState(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE t (id INT PRIMARY KEY); EXECUTE stmt; CREATE INDEX ix ON t (id);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("expected unsupported error for EXECUTE, got result=%#v", result)
	}
	if len(result.Statements) < 3 {
		t.Fatalf("expected partial statements, got %#v", result.Statements)
	}
	gaps := t05GapsByRule(result, 2, t05RuleCreateIndexColumns)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("post-EXECUTE index check must see unknown state, got %+v", result.Statements[2].EvidenceGaps)
	}
	wantFacts := []string{"target_table.columns", "target_table.existence"}
	if len(gaps[0].RequiredFacts) != 2 || gaps[0].RequiredFacts[0] != wantFacts[0] || gaps[0].RequiredFacts[1] != wantFacts[1] {
		t.Fatalf("required_facts must be [target_table.columns target_table.existence], got %+v", gaps[0].RequiredFacts)
	}
}

// C2: DROP DATABASE has no table targets but a bound schema scope — every
// cached entry under it is invalidated, and a stale provider read must not
// resurrect it. The reviewer's original sequence derives app.t first, so the
// invalidation must tombstone that entry too.
func TestBatchStateR1DropDatabaseInvalidatesSchemaScope(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.t": {
			Exists:  false,
			Table:   &spec.Table{Schema: "app", Name: "t"},
			Columns: []spec.Column{},
		},
		"other.u": {
			Exists:  true,
			Table:   &spec.Table{Schema: "other", Name: "u"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE app.t (id INT PRIMARY KEY); DROP DATABASE app;" +
			" CREATE INDEX ix ON app.t (id); ALTER TABLE other.u ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("create must derive cleanly, findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	// app.t was dropped with its schema: the index check must see unknown
	// state (gap), and the provider must not be re-asked to resurrect it.
	gaps := t05GapsByRule(result, 2, t05RuleCreateIndexColumns)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("index on schema-dropped table must report unknown_table_state, got %+v", result.Statements[2].EvidenceGaps)
	}
	if len(result.Statements[2].Findings) != 0 {
		t.Fatalf("schema-dropped table must not fabricate findings, got %+v", result.Statements[2].Findings)
	}
	// The unrelated table in another schema stays clean.
	if len(result.Statements[3].Findings) != 0 || len(result.Statements[3].EvidenceGaps) != 0 {
		t.Fatalf("other.u must stay clean, findings=%+v gaps=%+v", result.Statements[3].Findings, result.Statements[3].EvidenceGaps)
	}
	// app.t was read once for the create's pre-state; after the schema drop
	// the provider is never re-asked — only other.u resolves afterwards.
	sort.Strings(provider.snapshotCalls)
	if strings.Join(provider.snapshotCalls, ",") != "app.t,other.u" {
		t.Fatalf("provider must answer app.t once and other.u once, got %#v", provider.snapshotCalls)
	}
}

// C3: the frozen success template is the single plain ADD COLUMN — a
// multi-action ALTER (even all-adds) must invalidate rather than fabricate.
func TestBatchStateR1MultiActionAlterInvalidates(t *testing.T) {
	t.Parallel()
	provider := &t05AbsentProvider{}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT, ADD COLUMN c INT; CREATE INDEX ix ON t (c);",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[2].Findings) != 0 {
		t.Fatalf("statement 2 must not read a fabricated column, got %+v", result.Statements[2].Findings)
	}
	gaps := t05GapsByRule(result, 2, t05RuleCreateIndexColumns)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("statement 2 must report unknown_table_state after the multi-action ALTER, got %+v", result.Statements[2].EvidenceGaps)
	}
}

// C5: a provider snapshot that supplies the primary key but withholds the
// column collection must not produce a missing-PK blocker — the known fact
// survives while the unknown columns stay a gap.
func TestBatchStateR1PartialProviderShapeKeepsKnownPrimaryKey(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.t": {
			Exists:     true,
			Table:      &spec.Table{Schema: "app", Name: "t"},
			PrimaryKey: &spec.Index{Name: "PRIMARY", Kind: "primary", Columns: []string{"id"}},
			Columns:    nil,
		},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE t DROP PRIMARY KEY;",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		ConfigPath: t05PolicyPath(t, map[string]string{
			"ddl.alter.drop_primary_key.exists.require": "",
		}),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("known primary key must not yield a missing-PK blocker, got %+v", result.Statements[0].Findings)
	}
}

// C5: a derived-incomplete shape (conditional CREATE on unknown existence)
// leaves the primary key unknown — DROP PRIMARY KEY reports a bounded gap,
// never a fabricated missing-PK blocker.
func TestBatchStateR1ConditionalCreateLeavesPrimaryKeyUnknown(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE IF NOT EXISTS t (id INT PRIMARY KEY); ALTER TABLE t DROP PRIMARY KEY;",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		ConfigPath: t05PolicyPath(t, map[string]string{
			"ddl.table.exists.create.forbid":            "",
			"ddl.alter.drop_primary_key.exists.require": "",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("unknown primary key must not fabricate a blocker, got %+v", result.Statements[1].Findings)
	}
	gaps := t05GapsByRule(result, 1, "ddl.alter.drop_primary_key.exists.require")
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("statement 1 must report unknown_table_state, got %+v", result.Statements[1].EvidenceGaps)
	}
	if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.primary_key" {
		t.Fatalf("required_facts must be [target_table.primary_key], got %+v", gaps[0].RequiredFacts)
	}
}

// C5 regression control: a provider that fully loads the shape and reports
// no primary key still yields the real missing-PK blocker.
func TestBatchStateR1KnownAbsentPrimaryKeyKeepsBlocker(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.t": {
			Exists:  true,
			Table:   &spec.Table{Schema: "app", Name: "t"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE t DROP PRIMARY KEY;",
		Dialect: spec.DialectMySQL,
		Schema:  "app",
		ConfigPath: t05PolicyPath(t, map[string]string{
			"ddl.alter.drop_primary_key.exists.require": "",
		}),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 0, "ddl.alter.drop_primary_key.exists.require")
	if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
		t.Fatalf("known absent primary key must keep its blocker, got %+v", result.Statements[0].Findings)
	}
}

// C6: the frozen gap vocabulary is unknown_table_state — a confirmed-present
// table whose column set was never provided reports it with only the column
// fact, and no slice-local reason code may survive.
func TestBatchStateR1IncompleteColumnsUseFrozenReason(t *testing.T) {
	t.Parallel()
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"app.t": {Exists: true, Table: &spec.Table{Schema: "app", Name: "t"}},
	}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE t ADD COLUMN c INT;",
		Dialect:          spec.DialectMySQL,
		Schema:           "app",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	gaps := t05GapsByRule(result, 0, t05RuleAddColumnForbid)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("present-with-unknown-columns must report unknown_table_state, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.columns" {
		t.Fatalf("required_facts must be [target_table.columns], got %+v", gaps[0].RequiredFacts)
	}
}
