// Package audit verifies the T10-R3 consumer-interface corrections end to
// end: CREATE TABLE member comparisons between two different declarations
// owe an attributable key-parts gap instead of a silent pass; the exists
// rule reports the PRIMARY-object identity gap when the TiDB handle answer
// is unvouched; unnamed index members keep their product-derived stored
// names so successor ALTER/DROP statements resolve them; and provider-loaded
// expression members keep their catalog sort direction through publication.
// input: real MySQL/TiDB CREATE/ALTER batches audited through AuditSQL with
// stub providers carrying vouched member facts and TiDB handle modes
// output: statement-bound findings, evidence gaps, coverage statuses, and
// post-state resolution proving member identity and direction survive
// pos: application-layer regression for issue #89 T10-R3; the gap path
// lives in internal/domain/rule/ddl, name/handle resolution in
// internal/domain/spec, publication in batch_state*.go, and member
// classification in internal/infrastructure/metadata/mysql
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r3Dialects runs the CREATE TABLE comparison matrix on both MySQL-family
// dialects; the member-comparison contract is shared so both must agree.
var t10r3Dialects = []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

// t10r3AssertKeyPartsGap pins the attributable member-comparison gap on one
// statement: exactly one gap under the named rule carrying the index
// key-parts fact, unverified coverage, and a review verdict.
func t10r3AssertKeyPartsGap(t *testing.T, result report.Result, index int, ruleID string) {
	t.Helper()
	statement := result.Statements[index]
	matched := t05GapsByRule(result, index, ruleID)
	if len(matched) != 1 {
		t.Fatalf("stmt%d must report exactly one %s gap, got %+v", index, ruleID, statement.EvidenceGaps)
	}
	gap := matched[0]
	if gap.ReasonCode != "unknown_table_state" ||
		len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "target_table.indexes.key_parts" {
		t.Fatalf("gap must attribute the index key-parts fact, got %+v", gap)
	}
	if statement.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("skipped comparison must lower coverage to unverified, got %s", statement.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("unverifiable comparison must hold a review verdict, got %s", result.Verdict)
	}
}

// --- A: CREATE TABLE member comparisons keep their gap ---------------------

// Pinned counterexample (MySQL 8.0.46, only ddl.index.duplicate.forbid):
// two same-column prefix indexes inside one CREATE TABLE pair under the
// rule's column shape but cannot complete a plain-key comparison — the
// statement must report an attributable gap, not 0/0/complete.
func TestT10R3CreateTablePrefixDuplicateReportsGap(t *testing.T) {
	t.Parallel()
	for _, dialect := range t10r3Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t09Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i1(c(3)), KEY i2(c(3)));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("an uncompletable comparison must not fabricate findings, got %+v", result.Statements[0].Findings)
			}
			t10r3AssertKeyPartsGap(t, result, 0, "ddl.index.duplicate.forbid")
		})
	}
}

// Each lifecycle pairing reports under its own rule — one rule's gap can
// never mask a sibling rule's omission.
func TestT10R3CreateTableGapAttributionMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		sql    string
		ruleID string
	}{
		{
			name:   "left_prefix",
			sql:    "CREATE TABLE t (c VARCHAR(32), b INT, KEY i_narrow(c), KEY i_wide(c(3), b));",
			ruleID: "ddl.index.redundant_left_prefix.forbid",
		},
		{
			name:   "unique_overlap",
			sql:    "CREATE TABLE t (c VARCHAR(32), UNIQUE KEY uq_c(c DESC), KEY s_c(c));",
			ruleID: "ddl.index.redundant_unique_overlap.forbid",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range t10r3Dialects {
				dialect := dialect
				t.Run(string(dialect), func(t *testing.T) {
					t.Parallel()
					result := t09Audit(t, dialect, tc.sql, &t05AbsentProvider{},
						map[string]string{tc.ruleID: ""})
					if len(result.Statements[0].Findings) != 0 {
						t.Fatalf("an uncompletable comparison must not fabricate findings, got %+v", result.Statements[0].Findings)
					}
					t10r3AssertKeyPartsGap(t, result, 0, tc.ruleID)
				})
			}
		})
	}
}

// Controls that must NOT be downgraded: a lone non-plain member, a
// non-pairing pair, and a provable plain pair keep their exact outcomes.
func TestT10R3CreateTableComparisonControls(t *testing.T) {
	t.Parallel()
	allPairRules := map[string]string{
		"ddl.index.duplicate.forbid":                "",
		"ddl.index.redundant_left_prefix.forbid":    "",
		"ddl.index.redundant_unique_overlap.forbid": "",
	}
	for _, dialect := range t10r3Dialects {
		dialect := dialect
		t.Run(string(dialect)+"/single_non_plain", func(t *testing.T) {
			t.Parallel()
			result := t09Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i1(c(3)));",
				&t05AbsentProvider{}, allPairRules)
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("a lone member self-pairs nothing, got findings=%+v gaps=%+v",
					result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
				t.Fatalf("unaffected statement must keep complete coverage and pass, got %s/%s",
					result.Statements[0].Coverage.Status, result.Verdict)
			}
		})
		t.Run(string(dialect)+"/non_pairing_pair", func(t *testing.T) {
			t.Parallel()
			result := t09Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), b INT, KEY i1(c(3)), KEY i2(b(3)));",
				&t05AbsentProvider{}, allPairRules)
			if len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("non-pairing members must not emit a gap, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
				t.Fatalf("unaffected statement must keep complete coverage and pass, got %s/%s",
					result.Statements[0].Coverage.Status, result.Verdict)
			}
		})
		t.Run(string(dialect)+"/plain_duplicate_finding_kept", func(t *testing.T) {
			t.Parallel()
			result := t09Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i1(c), KEY i2(c));",
				&t05AbsentProvider{}, map[string]string{"ddl.index.duplicate.forbid": ""})
			if got := t05FindingsByRule(result, 0, "ddl.index.duplicate.forbid"); len(got) != 1 {
				t.Fatalf("the provable plain pair must keep its finding, got %+v", result.Statements[0].Findings)
			}
			if len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("a completed comparison emits no gap, got %+v", result.Statements[0].EvidenceGaps)
			}
		})
		t.Run(string(dialect)+"/disabled_rules", func(t *testing.T) {
			t.Parallel()
			result := t09Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i1(c(3)), KEY i2(c(3)));",
				&t05AbsentProvider{}, map[string]string{})
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("disabled rules must emit nothing, got findings=%+v gaps=%+v",
					result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
		})
	}
}

// --- B1: exists rule must honor PrimaryKeyHandleUnknown --------------------

// t10r3HandleUnknownTable is the pinned legal snapshot: every member
// collection vouched, PRIMARY(id) present, but the TiDB handle answer
// unprovable.
func t10r3HandleUnknownTable() *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Table:  &spec.Table{Name: "t"},
		Columns: []spec.Column{
			{Name: "id", Type: "int", NotNull: true},
		},
		PrimaryKey:              &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		Indexes:                 []spec.Index{},
		PrimaryKeyHandleUnknown: true,
	}
}

// exists-only: the suppressed absence finding must surface as the
// primary-key identity fact gap — never a silent pass and never a
// fabricated "does not exist" blocker.
func TestT10R3AlterPrimaryHandleUnknownExistsOnlyGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r3HandleUnknownTable()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	if got := t05FindingsByRule(result, 0, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("unvouched handle must not fabricate an absence finding, got %+v", got)
	}
	gaps := t05GapsByRule(result, 0, t10AlterIndexExist)
	if len(gaps) != 1 {
		t.Fatalf("exists rule must report exactly one identity gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.primary_key" {
		t.Fatalf("gap must attribute the primary-key fact, got %+v", gaps[0])
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("unresolved object identity must lower coverage, got %s", result.Statements[0].Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("unverifiable identity must hold a review verdict, got %s", result.Verdict)
	}
}

// compat-only: the same unknown flows through the assessment's own gap — the
// exists rule's omission cannot be masked by a sibling rule's gap.
func TestT10R3AlterPrimaryHandleUnknownCompatOnlyGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r3HandleUnknownTable()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10r2CompatPolicy(t, "visibility"),
	)
	if got := t05FindingsByRule(result, 0, t10IndexCompat); len(got) != 0 {
		t.Fatalf("unvouched handle must not fabricate a refusal finding, got %+v", got)
	}
	gaps := t05GapsByRule(result, 0, t10IndexCompat)
	if len(gaps) != 1 {
		t.Fatalf("compat rule must report exactly one identity gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.primary_key" {
		t.Fatalf("gap must attribute the primary-key fact, got %+v", gaps[0])
	}
}

// Both enabled: each rule's gap is independently attributed.
func TestT10R3AlterPrimaryHandleUnknownBothRulesGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r3HandleUnknownTable()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10Policy(t,
			t10Rule{id: t10AlterIndexExist},
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
		),
	)
	if len(t05GapsByRule(result, 0, t10AlterIndexExist)) != 1 {
		t.Fatalf("exists rule must own its identity gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(t05GapsByRule(result, 0, t10IndexCompat)) != 1 {
		t.Fatalf("compat rule must own its identity gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("unresolved object identity must lower coverage, got %s", result.Statements[0].Coverage.Status)
	}
}

// Rules disabled: nothing to answer — clean statement, no fabricated gap.
func TestT10R3AlterPrimaryHandleUnknownRulesDisabled(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t10r3HandleUnknownTable()},
	}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10Policy(t),
	)
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("disabled rules must emit nothing, got findings=%+v gaps=%+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
}

// --- B2: unnamed index members keep product-derived stored names -----------

// Pinned native behavior (MySQL 8.0.46 invisible_indexes.result L274-285):
// UNIQUE(a) and UNIQUE(b) INVISIBLE store as indexes named a and b — a
// successor ALTER INDEX b VISIBLE must resolve the member, not report a
// fabricated absence.
func TestT10R3UnnamedUniqueSuccessorResolvesAutoName(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE(a), UNIQUE(b) INVISIBLE);\n"+
			"ALTER TABLE t ALTER INDEX b VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 {
		t.Fatalf("auto-named member must resolve — no fabricated absence, got %+v", stmt1.Findings)
	}
	if len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("the stored name is provable — no gap, got %+v", stmt1.EvidenceGaps)
	}
	if stmt1.Coverage.Status != report.CoverageComplete {
		t.Fatalf("resolved member must keep complete coverage, got %s", stmt1.Coverage.Status)
	}
}

// Direct state assertion on the auto-named member: VISIBLE publishes its
// flip, a later DROP removes it, and only then does the existence rule
// report the member absent.
func TestT10R3UnnamedUniquePostStateDrop(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE(a), UNIQUE(b) INVISIBLE);\n"+
			"DROP INDEX b ON t;\n"+
			"ALTER TABLE t ALTER INDEX b VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	if got := t05FindingsByRule(result, 1, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("DROP on the auto-named member must resolve, got %+v", got)
	}
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 1 {
		t.Fatalf("after the drop the member must read as absent, got %+v", result.Statements[2].Findings)
	}
}

// Named control: an authored name never passes through auto-naming.
func TestT10R3NamedUniqueSuccessorUnchanged(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE(a), UNIQUE KEY ux_b(b) INVISIBLE);\n"+
			"ALTER TABLE t ALTER INDEX ux_b VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("authored name must resolve verbatim, got findings=%+v gaps=%+v", stmt1.Findings, stmt1.EvidenceGaps)
	}
}

// An unnamed expression member stores as `functional_index` on MySQL
// 8.0.46 (native-probed, R4-B) — the stored name is now provable, so a
// wrong guess reports a vouched absence instead of a suppressed one.
// The R3 oracle withheld the name because expression members were not yet
// anchored; R4 corrected the record against the native behavior.
func TestT10R3UnnamedExpressionResolvesFunctionalIndex(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT NOT NULL, UNIQUE((a + 1)));\n"+
			"ALTER TABLE t ALTER INDEX functional_index VISIBLE;\n"+
			"ALTER TABLE t ALTER INDEX functional_idx VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("the stored name functional_index must resolve cleanly, got findings=%+v gaps=%+v",
			stmt1.Findings, stmt1.EvidenceGaps)
	}
	stmt2 := result.Statements[2]
	if got := t05FindingsByRule(result, 2, t10AlterIndexExist); len(got) != 1 {
		t.Fatalf("a vouched name space reports the real absence, got %+v", stmt2.Findings)
	}
	if len(stmt2.EvidenceGaps) != 0 {
		t.Fatalf("a proven name space emits no collection gap, got %+v", stmt2.EvidenceGaps)
	}
}

// Name-collision control: an authored name duplicating a product-assigned
// name is a natively rejected CREATE — the derived shape must not publish a
// vouched name space for it.
func TestT10R3AutoNameCollisionKeepsNameUnknown(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT NOT NULL, UNIQUE(a), KEY a(a));\n"+
			"ALTER TABLE t ALTER INDEX a VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	stmt1 := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("unproven name space must not fabricate an absence finding, got %+v", got)
	}
	if len(t05GapsByRule(result, 1, t10AlterIndexExist)) != 1 {
		t.Fatalf("unproven name space must report the collection gap, got %+v", stmt1.EvidenceGaps)
	}
}

// --- B3: TiDB int_only handle defers to the alter-primary-key fact ----------

// int_only + alter-primary-key=true → the single-int PK is a real PRIMARY
// object: ALTER INDEX `PRIMARY` VISIBLE resolves cleanly and INVISIBLE is
// refused as a real-object 3522, not 1176.
func TestT10R3TiDBIntOnlyAlterPrimaryKeyTrueIsRealObject(t *testing.T) {
	t.Parallel()
	alterPK := true
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0", tidbMode: "int_only", tidbAlterPK: &alterPK,
	}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id INT PRIMARY KEY);\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	stmt1 := result.Statements[1]
	if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("real PRIMARY object must resolve VISIBLE cleanly, got findings=%+v gaps=%+v",
			stmt1.Findings, stmt1.EvidenceGaps)
	}
	compat := t05FindingsByRule(result, 2, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != spec.AdvancedIndexReasonPrimaryVisibility {
		t.Fatalf("real PRIMARY object must refuse INVISIBLE as 3522, got %+v", result.Statements[2].Findings)
	}
}

// int_only + alter-primary-key=false → the single-int PK is the row handle:
// no PRIMARY object exists and the successor reports the member absent.
func TestT10R3TiDBIntOnlyAlterPrimaryKeyFalseIsHandle(t *testing.T) {
	t.Parallel()
	alterPK := false
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0", tidbMode: "int_only", tidbAlterPK: &alterPK,
	}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id INT PRIMARY KEY);\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	got := t05FindingsByRule(result, 1, t10AlterIndexExist)
	if len(got) != 1 {
		t.Fatalf("PKIsHandle must report no operable PRIMARY object, got %+v", result.Statements[1].Findings)
	}
}

// int_only + unobserved alter-primary-key → the handle answer stays unknown:
// no fabricated absence and the primary-key identity gap.
func TestT10R3TiDBIntOnlyAlterPrimaryKeyUnknownStaysUnknown(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0", tidbMode: "int_only",
	}
	result := t10Audit(t, spec.DialectTiDB,
		"CREATE TABLE t (id INT PRIMARY KEY);\n"+
			"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	stmt1 := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t10AlterIndexExist); len(got) != 0 {
		t.Fatalf("unproven handle must not fabricate an absence finding, got %+v", got)
	}
	gaps := t05GapsByRule(result, 1, t10AlterIndexExist)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.primary_key" {
		t.Fatalf("unproven handle must report the primary-key identity gap, got %+v", stmt1.EvidenceGaps)
	}
	if stmt1.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("unproven handle must lower coverage, got %s", stmt1.Coverage.Status)
	}
}

// Controls: on / off / explicit CLUSTERED|NONCLUSTERED ignore the
// alter-primary-key fact exactly as the product does.
func TestT10R3TiDBHandleModeControls(t *testing.T) {
	t.Parallel()
	alterPK := true
	for _, tc := range []struct {
		name     string
		mode     string
		pkClause string
		alterPK  *bool
		wantReal bool
	}{
		{name: "on ignores alter_pk", mode: "on", alterPK: &alterPK, wantReal: false},
		{name: "off is real object", mode: "off", wantReal: true},
		{name: "explicit clustered ignores mode", pkClause: " CLUSTERED", wantReal: false},
		{name: "explicit nonclustered is real", mode: "int_only", pkClause: " NONCLUSTERED", wantReal: true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{
				version: "8.0.11-TiDB-v8.5.0", tidbMode: tc.mode, tidbAlterPK: tc.alterPK,
			}
			result := t10Audit(t, spec.DialectTiDB,
				"CREATE TABLE t (id INT PRIMARY KEY"+tc.pkClause+");\n"+
					"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
				provider, t10ExistsPolicy(t),
			)
			got := t05FindingsByRule(result, 1, t10AlterIndexExist)
			if tc.wantReal {
				if len(got) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
					t.Fatalf("real PRIMARY object must resolve VISIBLE cleanly, got findings=%+v gaps=%+v",
						got, result.Statements[1].EvidenceGaps)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("handle PK must report no operable PRIMARY object, got %+v", result.Statements[1].Findings)
			}
		})
	}
}

// --- C: provider expression direction survives publication -----------------

// A provider-loaded expression member with COLLATION='D' keeps Descending
// through ALTER INDEX publication — the successor comparison proves the
// member marks survived the post-state.
func TestT10R3ExpressionDescendingSurvivesPostState(t *testing.T) {
	t.Parallel()
	existing := spec.Index{
		Name: "i_e", Kind: spec.IndexKindSecondary,
		KeyParts: []spec.IndexKeyPart{{
			Ordinal:    1,
			Kind:       spec.IndexKeyPartExpression,
			Descending: true,
			Expression: &spec.ExpressionFacts{Known: true, Fingerprint: "plus(a,1)", References: []spec.ColumnReference{{Name: "a"}}},
		}},
	}
	existing.SetKeyParts(existing.KeyParts)
	table := t10r2Table("t", []spec.Column{
		t10r2Col("a", "int", true, 0),
		t10r2Col("b", "int", true, 0),
	}, existing)
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX i_e VISIBLE;\n"+
			"CREATE INDEX i_dup ON t((a + 1));",
		provider, t05A7Policy(t, map[string]string{"ddl.index.duplicate.forbid": ""}),
	)
	// Both sides carry hidden members — the duplicate comparison cannot
	// complete under plain-key semantics. The gap only fires because the
	// derived member kept its non-plain marks through publication.
	t10r3AssertKeyPartsGap(t, result, 1, "ddl.index.duplicate.forbid")
}

// --- R3 independent-review corrections -------------------------------------

// A PRIMARY target resolves through the primary-key identity facts, never
// the index name space: an unproven name space must not skip the check —
// a vouched-absent PK still reports the member absent.
func TestT10R3AlterPrimaryResolvesDespiteUnknownIndexNames(t *testing.T) {
	t.Parallel()
	table := t10r2Table("t", []spec.Column{t10r2Col("id", "int", true, 0)},
		spec.Index{Name: "", Kind: spec.IndexKindSecondary, Columns: []string{"id"}})
	table.IndexNamesUnknown = true
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	if got := t05FindingsByRule(result, 0, t10AlterIndexExist); len(got) != 1 {
		t.Fatalf("PRIMARY resolves on the primary-key facts — a vouched absence must still report, got %+v",
			result.Statements[0].Findings)
	}
}

// A CREATE INDEX authored member merges once every member name is
// provable: UNIQUE((a + 1)) derives functional_index through
// StoredIndexNames, so the name space is fully vouched and the merge can
// rule out a hidden 1061 (R4: any unproven member name unvouches instead).
func TestT10R3CreateIndexMergesAuthoredMember(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, c INT, UNIQUE((a + 1)));\n"+
			"CREATE INDEX s ON t(c);\n"+
			"CREATE INDEX s2 ON t(c);",
		provider, t05A7Policy(t, map[string]string{"ddl.index.duplicate.forbid": ""}),
	)
	// s2 pairs against the merged s on the plain column shape — if the
	// member were dropped instead of merged, the comparison would pass
	// silently on an incomplete set.
	got := t05FindingsByRule(result, 2, "ddl.index.duplicate.forbid")
	if len(got) != 1 {
		t.Fatalf("the merged authored member must pair with s2, got %+v", result.Statements[2].Findings)
	}
}

// The ALTER-side member add follows the same premise: an authored
// ADD INDEX member merges under a fully proven name space — an unproven
// member name would unvouch the set (hidden 1061) instead of merging.
func TestT10R3AddIndexMergesAuthoredMember(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, c INT, UNIQUE((a + 1)));\n"+
			"ALTER TABLE t ADD INDEX s (c);\n"+
			"CREATE INDEX s2 ON t(c);",
		provider, t05A7Policy(t, map[string]string{"ddl.index.duplicate.forbid": ""}),
	)
	got := t05FindingsByRule(result, 2, "ddl.index.duplicate.forbid")
	if len(got) != 1 {
		t.Fatalf("the merged authored member must pair with s2, got %+v", result.Statements[2].Findings)
	}
}

// TiDB refuses DROP PRIMARY KEY on a handle table natively (8200 "the table
// is using clustered index", observed on v8.5.0): the statement fails and
// the table is provably unchanged — publishing a dropped-PK shape would
// fabricate success.
func TestT10R3TiDBDropPrimaryHandleKeepsTable(t *testing.T) {
	t.Parallel()
	state := newBatchState(spec.DialectTiDB, "golden", &t10Provider{version: "8.0.11-TiDB-v8.5.0"})
	key := state.keyFor("golden", spec.Table{Name: "t"})
	shape := t10r2Table("t", []spec.Column{t10r2Col("id", "int", true, 0)})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	shape.PrimaryKeyIsHandle = true
	state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape}
	parsed, err := parseSQL(context.Background(), "ALTER TABLE t DROP PRIMARY KEY;", spec.DialectTiDB)
	if err != nil || len(parsed.Statements) != 1 {
		t.Fatalf("parse drop primary key: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := state.entries[key]
	if got == nil || got.state != tablePresent || got.shape == nil || !got.shape.HasPrimaryKey() {
		t.Fatalf("a refused drop leaves the PK in place, got %+v", got)
	}
}

// An unproven handle answer cannot prove the drop outcome either way —
// the entry invalidates instead of guessing.
func TestT10R3TiDBDropPrimaryHandleUnknownInvalidates(t *testing.T) {
	t.Parallel()
	state := newBatchState(spec.DialectTiDB, "golden", &t10Provider{version: "8.0.11-TiDB-v8.5.0"})
	key := state.keyFor("golden", spec.Table{Name: "t"})
	shape := t10r2Table("t", []spec.Column{t10r2Col("id", "int", true, 0)})
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	shape.PrimaryKeyHandleUnknown = true
	state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape}
	parsed, err := parseSQL(context.Background(), "ALTER TABLE t DROP PRIMARY KEY;", spec.DialectTiDB)
	if err != nil || len(parsed.Statements) != 1 {
		t.Fatalf("parse drop primary key: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := state.apply(context.Background(), statements[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := state.entries[key]
	if got != nil && got.state == tablePresent {
		t.Fatalf("an unprovable drop outcome must not publish, got %+v", got)
	}
}

// Controls: a vouched real-object PK publishes the drop on both dialects.
func TestT10R3DropPrimaryRealObjectPublishes(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			state := newBatchState(dialect, "golden", &t10Provider{version: "8.0.46"})
			key := state.keyFor("golden", spec.Table{Name: "t"})
			shape := t10r2Table("t", []spec.Column{t10r2Col("id", "int", true, 0)})
			shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
			state.entries[key] = &batchTableEntry{state: tablePresent, shape: shape}
			parsed, err := parseSQL(context.Background(), "ALTER TABLE t DROP PRIMARY KEY;", dialect)
			if err != nil || len(parsed.Statements) != 1 {
				t.Fatalf("parse drop primary key: %v", err)
			}
			statements, err := Extract(context.Background(), parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if err := state.apply(context.Background(), statements[0]); err != nil {
				t.Fatalf("apply: %v", err)
			}
			got := state.entries[key]
			if got == nil || got.state != tablePresent || got.shape == nil || got.shape.HasPrimaryKey() {
				t.Fatalf("a provable drop must clear the PK, got %+v", got)
			}
		})
	}
}
