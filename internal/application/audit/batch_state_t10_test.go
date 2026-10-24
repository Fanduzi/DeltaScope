// Package audit verifies the T10 advanced-index and visibility chain end
// to end: ALTER INDEX VISIBLE|INVISIBLE is an audited action whose target
// resolves against the ordered index set, declared visibility attributes
// carry modeled facts, the version/applicability rule answers for the
// advanced index forms, and the published post-state preserves member
// visibility so later statements read the derived catalog.
// input: ALTER INDEX statements plus declared visibility/prefix/expression/
// descending/multi-valued index forms audited through AuditSQL with a stub
// provider carrying versioned instance facts and vouched index sets
// output: statement-bound findings and evidence gaps proving visibility
// transitions, existence, product/version applicability, and post-state
// fidelity — and the controls proving unvouched or inapplicable cases never
// fabricate a finding
// pos: application-layer ordered-state regression for issue #89 T10; the
// transitions under test live in batch_state_index.go and the rules in
// internal/domain/rule/ddl
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10Rule configures one catalog rule: the ID plus the level and raw params
// block the policy file writes.
type t10Rule struct {
	id     string
	level  string
	params string
}

func t10Policy(t *testing.T, rules ...t10Rule) string {
	t.Helper()
	enabled := map[string]t10Rule{}
	for _, r := range rules {
		if _, ok := catalog.Lookup(r.id); !ok {
			t.Fatalf("missing catalog rule %s", r.id)
		}
		enabled[r.id] = r
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		r, on := enabled[entry.RuleID]
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, on)
		if on {
			level := r.level
			if level == "" {
				level = "blocker"
			}
			fmt.Fprintf(&text, "    level: %s\n", level)
			if r.params != "" {
				text.WriteString("    params:\n" + r.params)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// t10Provider serves fixed per-table snapshots plus an optional observed
// version banner so version-gated features run against a known identity.
// tidbMode carries the session-observed tidb_enable_clustered_index value
// TiDB handle determination defers to when CLUSTERED|NONCLUSTERED is
// unwritten.
type t10Provider struct {
	version  string
	tidbMode string
	tables   map[string]*spec.TableSnapshot
	calls    []string
}

func (p *t10Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version, TiDBEnableClusteredIndex: p.tidbMode}, nil
}

func (p *t10Provider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	if snap, ok := p.tables[strings.ToLower(table)]; ok {
		return snap, nil
	}
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t10Audit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit: %v", err)
	}
	return result
}

const (
	t10InvisibleForbid = "ddl.index.invisible.forbid"
	t10AlterIndexExist = "ddl.alter.alter_index.exists.require"
	t10IndexCompat     = "ddl.index.advanced.compatibility.require"
	t10TableExist      = "ddl.table.exists.alter.require"
)

// t10VisibilityPolicy is the contract's three-rule cell.
func t10VisibilityPolicy(t *testing.T) string {
	return t10Policy(t,
		t10Rule{id: t10InvisibleForbid, level: "warning"},
		t10Rule{id: t10AlterIndexExist},
		t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n        - expression_keys\n        - descending_keys\n        - multi_valued\n"},
	)
}

// t10ExistsPolicy isolates the existence rule so tombstone evidence cannot
// be masked by sibling findings.
func t10ExistsPolicy(t *testing.T) string {
	return t10Policy(t, t10Rule{id: t10AlterIndexExist})
}

// The contract first path: a real index the catalog proves visible flips to
// invisible — one warning bound to the target, no gaps, complete coverage.
func TestT10AlterIndexInvisibleFirstPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, Visibility: spec.ColumnVisibilityVisible}),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			findings := t05FindingsByRule(result, 0, t10InvisibleForbid)
			if len(findings) != 1 {
				t.Fatalf("exactly one invisibility finding expected, got %+v", result.Statements[0].Findings)
			}
			f := findings[0]
			if f.Level != rule.LevelWarning || f.Metadata["index"] != "idx_c" ||
				f.Metadata["table"] != "t" || f.Metadata["visibility"] != spec.ColumnVisibilityInvisible {
				t.Fatalf("finding must bind idx_c/t/invisible at warning level, got %+v", f)
			}
			if len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("vouched index set and known version must leave no gaps, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageComplete {
				t.Fatalf("alter_index is now an audited action, got coverage %s", result.Statements[0].Coverage.Status)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("no unsupported evidence expected, got %+v", result.Unsupported)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("warning-only outcome must hold review, got %s", result.Verdict)
			}
		})
	}
}

// The reverse direction stays silent: VISIBLE on a present index reports no
// finding and a clean pass.
func TestT10AlterIndexVisibleDirectionSilent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "idx_i", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, Visibility: spec.ColumnVisibilityInvisible}),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX idx_i VISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("VISIBLE on a present index must stay silent, got findings=%+v gaps=%+v",
					result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
			if result.Verdict != report.VerdictPass {
				t.Fatalf("clean VISIBLE must pass, got %s", result.Verdict)
			}
		})
	}
}

// A missing index is a determined existence failure — the blocker pairs
// with the declared invisibility finding; no fabricated post-state.
func TestT10AlterIndexMissingIndex(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"b"}}),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX idx_nope INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			exists := t05FindingsByRule(result, 0, t10AlterIndexExist)
			if len(exists) != 1 || exists[0].Metadata["exists"] != false {
				t.Fatalf("missing index must emit the exists blocker, got %+v", result.Statements[0].Findings)
			}
			if len(t05FindingsByRule(result, 0, t10InvisibleForbid)) != 1 {
				t.Fatalf("declared invisibility intent still reports, got %+v", result.Statements[0].Findings)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("blocker finding must reject, got %s", result.Verdict)
			}
		})
	}
}

// An unvouched index set answers with the member-collection gap — never a
// fabricated existence finding.
func TestT10AlterIndexUnknownSet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			snapshot := t09Table("t")
			snapshot.IndexesUnknown = true
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": snapshot}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10AlterIndexExist)) != 0 {
				t.Fatalf("unknown index set must not fabricate an existence finding, got %+v", result.Statements[0].Findings)
			}
			gaps := t05GapsByRule(result, 0, t10AlterIndexExist)
			if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.indexes" {
				t.Fatalf("unvouched set must report the indexes fact, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageUnverified {
				t.Fatalf("the gap must lower coverage to unverified, got %s", result.Statements[0].Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("gap + warning holds review, got %s", result.Verdict)
			}
		})
	}
}

// The visibility transition publishes the member's new effective state:
// the second statement still resolves the index, stays clean, and keeps
// complete coverage — a tombstoned transition would surface as gaps.
func TestT10AlterIndexBatchChain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dialect spec.Dialect
		banner  string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
	} {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, Visibility: spec.ColumnVisibilityVisible}),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX idx_c INVISIBLE;\nALTER TABLE t ALTER INDEX idx_c VISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10InvisibleForbid)) != 1 {
				t.Fatalf("stmt0 must report the invisibility intent, got %+v", result.Statements[0].Findings)
			}
			stmt1 := result.Statements[1]
			if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
				t.Fatalf("stmt1 must read the derived member and stay clean, got findings=%+v gaps=%+v",
					stmt1.Findings, stmt1.EvidenceGaps)
			}
			if stmt1.Coverage.Status != report.CoverageComplete {
				t.Fatalf("stmt1 coverage must stay complete, got %s", stmt1.Coverage.Status)
			}
		})
	}
}

// A multi-action ALTER carrying alter_index outside the bounded successor
// template tombstones the entry instead of guessing a shape: the next
// statement must read existence+indexes gaps, never a fabricated member.
func TestT10AlterIndexMultiActionTombstones(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t09Table("t", spec.Index{Name: "i", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, Visibility: spec.ColumnVisibilityVisible}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN x INT, ALTER INDEX i INVISIBLE;\nALTER TABLE t ALTER INDEX i INVISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("stmt0 resolves against the vouched pre-state and stays clean, got findings=%+v gaps=%+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 {
		t.Fatalf("tombstoned state must not fabricate an existence finding, got %+v", stmt1.Findings)
	}
	gaps := t05GapsByRule(result, 1, t10AlterIndexExist)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 2 ||
		gaps[0].RequiredFacts[0] != "target_table.existence" || gaps[0].RequiredFacts[1] != "target_table.indexes" {
		t.Fatalf("tombstone must surface the existence+indexes gap pair, got %+v", stmt1.EvidenceGaps)
	}
	if stmt1.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("the gap must lower coverage to unverified, got %s", stmt1.Coverage.Status)
	}
}

// Provider-observed invisibility is a fact, not a declared intent: loading
// an invisible member does not itself convict a VISIBLE statement.
func TestT10ProviderInvisibleMemberNotIntent(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t09Table("t", spec.Index{Name: "idx_i", Kind: spec.IndexKindSecondary, Columns: []string{"b"}, Visibility: spec.ColumnVisibilityInvisible}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX idx_i VISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("provider state is not a declaration — VISIBLE must stay clean, got %+v", result.Statements[0].Findings)
	}
}

// The published post-state carries the new visibility: a member the batch
// turned invisible still resolves for a later VISIBLE statement.
func TestT10AlterIndexPostStateMemberVisible(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t09Table("t", spec.Index{Name: "idx_i", Kind: spec.IndexKindSecondary, Columns: []string{"b"}}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX idx_i INVISIBLE;\nALTER TABLE t ALTER INDEX idx_i INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	stmt1 := result.Statements[1]
	// A broken transition (tombstone or lost member) would surface as the
	// indexes gap or a spurious exists finding here.
	if len(stmt1.EvidenceGaps) != 0 || len(t05FindingsByRule(result, 1, t10AlterIndexExist)) != 0 {
		t.Fatalf("derived member must keep resolving, got findings=%+v gaps=%+v", stmt1.Findings, stmt1.EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 1, t10InvisibleForbid)) != 1 {
		t.Fatalf("declared invisibility intent still reports on stmt1, got %+v", stmt1.Findings)
	}
}

// Every declared path carrying an INVISIBLE attribute convicts the same
// rule — create_table member, add_index action, standalone CREATE INDEX.
func TestT10DeclaredInvisiblePaths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "create_table key", sql: "CREATE TABLE t (a INT, KEY i (a) INVISIBLE)"},
		{name: "alter add_index", sql: "ALTER TABLE t ADD INDEX i (a) INVISIBLE"},
		{name: "standalone create_index", sql: "CREATE INDEX i ON t(a) INVISIBLE"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t"),
			}}
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider, t10VisibilityPolicy(t))
			findings := t05FindingsByRule(result, 0, t10InvisibleForbid)
			if len(findings) != 1 || findings[0].Metadata["index"] != "i" {
				t.Fatalf("declared INVISIBLE must convict on %s, got %+v", tc.name, result.Statements[0].Findings)
			}
		})
	}
}

// A declared PRIMARY KEY invisibility is rejected by the product — the
// compatibility rule owns that determined negative on top of the
// invisibility intent.
func TestT10DeclaredPrimaryKeyInvisible(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, PRIMARY KEY (a) INVISIBLE)",
		provider, t10VisibilityPolicy(t),
	)
	if len(t05FindingsByRule(result, 0, t10InvisibleForbid)) != 1 {
		t.Fatalf("invisibility intent must report, got %+v", result.Statements[0].Findings)
	}
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
		t.Fatalf("primary-key invisibility is a determined negative, got %+v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("blocker finding must reject, got %s", result.Verdict)
	}
}

// MySQL ALTER INDEX `PRIMARY` resolves the vouched PK — the product
// rejection (3522) is the compatibility issue, not a missing index.
func TestT10AlterIndexPrimaryMySQL(t *testing.T) {
	t.Parallel()
	table := t09Table("t", spec.Index{Name: "idx_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}})
	table.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}}
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	if len(t05FindingsByRule(result, 0, t10AlterIndexExist)) != 0 {
		t.Fatalf("vouched primary key resolves — no exists finding, got %+v", result.Statements[0].Findings)
	}
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
		t.Fatalf("ALTER INDEX PRIMARY is a 3522-class determined negative, got %+v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("blocker finding must reject, got %s", result.Verdict)
	}
}

// TiDB ALTER INDEX `PRIMARY` splits on the real index-object question: a
// single-int clustered primary key is stored as PKIsHandle and names no
// index (1176 → exists blocker), while a non-handle primary key is an
// operable index object — INVISIBLE is refused by compatibility (3522),
// not a missing-index error.
func TestT10AlterIndexPrimaryTiDB(t *testing.T) {
	t.Parallel()
	handle := t09Table("t", spec.Index{Name: "idx_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}})
	handle.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}}
	handle.PrimaryKeyIsHandle = true
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": handle}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	exists := t05FindingsByRule(result, 0, t10AlterIndexExist)
	if len(exists) != 1 || exists[0].Metadata["exists"] != false {
		t.Fatalf("PKIsHandle names no index object — exists blocker expected, got %+v", result.Statements[0].Findings)
	}
	if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
		t.Fatalf("the TiDB negative belongs to existence, not compatibility, got %+v", result.Statements[0].Findings)
	}
}

// Version applicability: the observed 5.7 anchor makes the same first path
// a determined version negative — never a silent pass.
func TestT10AlterIndexVersionGate57(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{
		"t": t09Table("t", spec.Index{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"b"}}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != spec.AdvancedReasonVersionUnsupported ||
		compat[0].Metadata["minimum_version"] != spec.AdvancedIndexMinimumVisibilityMySQL {
		t.Fatalf("5.7 must answer version_unsupported ≥%s, got %+v", spec.AdvancedIndexMinimumVisibilityMySQL, result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("version blocker must reject, got %s", result.Verdict)
	}
}

// A missing version fact is a bounded gap, not silence and not a finding.
func TestT10AlterIndexVersionMissing(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "", tables: map[string]*spec.TableSnapshot{
		"t": t09Table("t", spec.Index{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"b"}}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX idx_c INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	gaps := t05GapsByRule(result, 0, t10IndexCompat)
	if len(gaps) != 1 || gaps[0].ReasonCode != spec.AdvancedReasonMissingTargetVersion ||
		len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != spec.AdvancedFactTargetVersion {
		t.Fatalf("missing version must report the bounded fact gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
		t.Fatalf("missing version never fabricates a finding, got %+v", result.Statements[0].Findings)
	}
}

// The advanced-form version gates fire per form at their own minimums.
func TestT10CompatFormGatesMySQL57(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		sql    string
		reason string
		// feature → expected minimum_version for each version-gated issue the
		// form produces; a multi-valued member is also an expression member,
		// so it carries two determined issues.
		minimums map[string]string
	}{
		{name: "declared invisible attr", sql: "CREATE INDEX i ON t(a) INVISIBLE", reason: spec.AdvancedReasonVersionUnsupported, minimums: map[string]string{"visibility": "8.0.0"}},
		{name: "expression key part", sql: "CREATE INDEX i ON t((a+1))", reason: spec.AdvancedReasonVersionUnsupported, minimums: map[string]string{"expression_keys": "8.0.13"}},
		{name: "multi-valued member", sql: "CREATE INDEX i ON t((CAST(a AS UNSIGNED ARRAY)))", reason: spec.AdvancedReasonVersionUnsupported, minimums: map[string]string{"expression_keys": "8.0.13", "multi_valued": "8.0.17"}},
		{name: "descending member", sql: "CREATE INDEX i ON t(a DESC)", reason: "descending_ignored", minimums: map[string]string{"descending_keys": ""}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider,
				t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n"}))
			findings := t05FindingsByRule(result, 0, t10IndexCompat)
			if len(findings) != len(tc.minimums) {
				t.Fatalf("expected %d %s finding(s), got %+v", len(tc.minimums), tc.reason, result.Statements[0].Findings)
			}
			for _, finding := range findings {
				feature, _ := finding.Metadata["feature"].(string)
				minimum, ok := tc.minimums[feature]
				if !ok || finding.Metadata["reason_code"] != tc.reason {
					t.Fatalf("unexpected finding %s/%s, got %+v", feature, finding.Metadata["reason_code"], result.Statements[0].Findings)
				}
				if finding.Metadata["minimum_version"] != minimum {
					t.Fatalf("feature %s minimum = %v, want %q", feature, finding.Metadata["minimum_version"], minimum)
				}
			}
		})
	}
}

// The same forms on a supported version stay silent — gating is by version,
// never by form presence.
func TestT10CompatFormGatesMySQL80Silent(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, sql string }{
		{"invisible attr", "CREATE INDEX i ON t(a) INVISIBLE"},
		{"expression part", "CREATE INDEX i ON t((a+1))"},
		{"multi-valued", "CREATE INDEX i ON t((CAST(a AS UNSIGNED ARRAY)))"},
		{"descending", "CREATE INDEX i ON t(a DESC)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider,
				t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n"}))
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("8.0 supports every advanced index form, got %+v", result.Statements[0].Findings)
			}
		})
	}
}

// TiDB silently drops DESC at the catalog layer — a determined product
// negative independent of version.
func TestT10CompatDescTiDB(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
	result := t10Audit(t, spec.DialectTiDB, "CREATE INDEX i ON t(a DESC)", provider,
		t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n"}))
	findings := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "descending_ignored" {
		t.Fatalf("TiDB must answer descending_ignored, got %+v", result.Statements[0].Findings)
	}
}

// Feature params isolate the rule: with only visibility enabled an
// expression index carries no compat finding even on 5.7.
func TestT10CompatFeatureIsolation(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
	result := t10Audit(t, spec.DialectMySQL, "CREATE INDEX i ON t((a+1))", provider,
		t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"}))
	if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
		t.Fatalf("disabled feature must not fire, got %+v", result.Statements[0].Findings)
	}
}

// With every rule off the action is still audited — complete coverage and a
// pass verdict prove the promotion is not rule-dependent.
func TestT10AlterIndexRulesOffAudited(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t10Audit(t, dialect, "ALTER TABLE t ALTER INDEX idx_c INVISIBLE;", provider, t10Policy(t))
			stmt := result.Statements[0]
			if len(result.Unsupported) != 0 || stmt.Coverage.Status != report.CoverageComplete {
				t.Fatalf("alter_index must be audited rule-free, got unsupported=%+v coverage=%s",
					result.Unsupported, stmt.Coverage.Status)
			}
			if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 || result.Verdict != report.VerdictPass {
				t.Fatalf("rules-off must stay silent, got findings=%+v gaps=%+v verdict=%s",
					stmt.Findings, stmt.EvidenceGaps, result.Verdict)
			}
		})
	}
}
