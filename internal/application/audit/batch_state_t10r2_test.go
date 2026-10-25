// Package audit verifies the T10-R2 member-audit chain end to end: the
// four-state primary-role model (explicit primary / implicit holder /
// resolved absence / unresolved), the member-legality routing by index
// kind (FULLTEXT, SPATIAL, prefix, JSON, GLOBAL, multi-valued), the
// coverage graduation of formerly unsupported member forms, and the
// ordered post-state those forms publish.
// input: CREATE/ADD/ALTER index statements audited through AuditSQL with a
// stub provider carrying versioned instance facts, observed
// tidb_enable_clustered_index modes, and vouched column/index/constraint
// sets (the shared fixture now carries a proven-empty constraint
// collection so the T11 dependency gate never misreads it as withheld)
// output: statement-bound findings, evidence gaps, coverage statuses, and
// post-state behavior proving member facts survive publication — and the
// controls proving unknown role/member facts tombstone instead of
// fabricating a success shape
// pos: application-layer ordered-state regression for issue #89 T10-R2;
// the determination lives in internal/domain/spec/advanced_indexes.go and
// primary_key_role.go, publication in batch_state*.go
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r2CompatPolicy isolates the advanced-compat rule so member-legality
// issues cannot hide behind unrelated rule findings.
func t10r2CompatPolicy(t *testing.T, features ...string) string {
	t.Helper()
	params := "      required: true\n"
	if len(features) > 0 {
		params += "      features:\n"
		for _, feature := range features {
			params += "        - " + feature + "\n"
		}
	}
	return t10Policy(t, t10Rule{id: t10IndexCompat, params: params})
}

// t10r2Table builds a vouched table snapshot from explicit column facts and
// indexes — the member-legality matrix needs typed columns t09Table's
// int-only set cannot express.
func t10r2Table(name string, columns []spec.Column, indexes ...spec.Index) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:      true,
		Table:       &spec.Table{Name: name},
		Columns:     columns,
		Indexes:     indexes,
		Constraints: []spec.Constraint{},
	}
}

func t10r2Col(name, typ string, notNull bool, length int) spec.Column {
	return spec.Column{Name: name, Type: typ, NotNull: notNull, Length: length}
}

// --- B: primary-role four-state model ------------------------------------

// B1: with an explicit primary key vouched, an appended UNIQUE INVISIBLE
// can never hold the implicit role — the add succeeds and the member
// publishes. Previously Resolved=true/HasHolder=false collapsed "explicit
// primary present" into "no holder", letting the appended candidate claim
// the role.
func TestT10R2ExplicitPrimaryFreesUniqueInvisibility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		dialect spec.Dialect
		banner  string
		sql     string
	}{
		{"create_index mysql", spec.DialectMySQL, "8.0.46", "CREATE UNIQUE INDEX u_a ON t(a) INVISIBLE;"},
		{"add_index mysql", spec.DialectMySQL, "8.0.46", "ALTER TABLE t ADD UNIQUE KEY u_a (a) INVISIBLE;"},
		{"create_index tidb", spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", "CREATE UNIQUE INDEX u_a ON t(a) INVISIBLE;"},
		{"add_index tidb", spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", "ALTER TABLE t ADD UNIQUE KEY u_a (a) INVISIBLE;"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			table := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
			table.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}}
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": table}}
			result := t10Audit(t, tc.dialect, tc.sql+"\nALTER TABLE t ALTER INDEX u_a VISIBLE;", provider, t10VisibilityPolicy(t))
			stmt0 := result.Statements[0]
			for _, finding := range stmt0.Findings {
				if finding.Metadata["reason_code"] == spec.AdvancedIndexReasonPrimaryVisibility {
					t.Fatalf("explicit primary frees the appended unique — no role refusal, got %+v", finding)
				}
			}
			// The invisible warning fires for the declared intent; the
			// second statement resolves the published member — proof the
			// transition produced state, not a tombstone.
			if len(t05FindingsByRule(result, 0, t10InvisibleForbid)) != 1 {
				t.Fatalf("declared invisibility intent must report, got %+v", stmt0.Findings)
			}
			stmt1 := result.Statements[1]
			if len(stmt1.Findings) != 0 || len(stmt1.EvidenceGaps) != 0 {
				t.Fatalf("published member must resolve cleanly, got findings=%+v gaps=%+v", stmt1.Findings, stmt1.EvidenceGaps)
			}
			if stmt1.Coverage.Status != report.CoverageComplete {
				t.Fatalf("post-state member read must stay complete, got %s", stmt1.Coverage.Status)
			}
		})
	}
}

// B1 negative control: without an explicit primary, the first qualifying
// UNIQUE INVISIBLE holds the implicit role — the refusal stays.
func TestT10R2ImplicitHolderStillRefusesInvisibility(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{
		"t": t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)}),
	}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD UNIQUE KEY u_a (a) INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != spec.AdvancedIndexReasonPrimaryVisibility {
		t.Fatalf("the sole qualifying unique must refuse invisibility, got %+v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("role refusal must reject, got %s", result.Verdict)
	}
}

// B2: two unnamed unique members — only the first can hold the role; the
// second is a different member even though both carry the same generated
// name shape. The named variant and the negative control (first INVISIBLE)
// pin both directions.
func TestT10R2SecondUnnamedUniqueMayHide(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		sql      string
		wantDeny bool
	}{
		{name: "unnamed second invisible", sql: "CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE(a), UNIQUE(b) INVISIBLE);"},
		{name: "named second invisible", sql: "CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE KEY u_a (a), UNIQUE KEY u_b (b) INVISIBLE);"},
		{name: "first unnamed invisible denies", sql: "CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE(a) INVISIBLE, UNIQUE(b));", wantDeny: true},
		{name: "first named invisible denies", sql: "CREATE TABLE t (a INT NOT NULL, b INT NOT NULL, UNIQUE KEY u_a (a) INVISIBLE, UNIQUE KEY u_b (b));", wantDeny: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.46"}
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider, t10VisibilityPolicy(t))
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			roleDenied := false
			for _, finding := range compat {
				if finding.Metadata["reason_code"] == spec.AdvancedIndexReasonPrimaryVisibility {
					roleDenied = true
				}
			}
			if roleDenied != tc.wantDeny {
				t.Fatalf("role denial = %v, want %v; compat findings %+v", roleDenied, tc.wantDeny, compat)
			}
		})
	}
}

// B3: an unvouched primary-key collection leaves the role answer unknown —
// the statement that depends on it surfaces the bounded role gap, never
// fabricates a refusal or a pass, and must not publish the member as if
// the create had landed.
func TestT10R2UnresolvedPrimaryRoleWithholdsMember(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "add_index", sql: "ALTER TABLE t ADD UNIQUE KEY u_a (a) INVISIBLE;\nALTER TABLE t ALTER INDEX u_a VISIBLE;"},
		{name: "create_index", sql: "CREATE UNIQUE INDEX u_a ON t(a) INVISIBLE;\nALTER TABLE t ALTER INDEX u_a VISIBLE;"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snapshot := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
			snapshot.PrimaryKeyUnknown = true
			provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": snapshot}}
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider, t10VisibilityPolicy(t))
			stmt0 := result.Statements[0]
			gaps := t05GapsByRule(result, 0, t10IndexCompat)
			if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != spec.PrimaryRoleGapPrimaryKey {
				t.Fatalf("unresolved role must report the primary_key fact, got %+v", stmt0.EvidenceGaps)
			}
			for _, finding := range t05FindingsByRule(result, 0, t10IndexCompat) {
				if finding.Metadata["reason_code"] == spec.AdvancedIndexReasonPrimaryVisibility {
					t.Fatalf("unresolved role never fabricates the refusal, got %+v", finding)
				}
			}
			// The unresolved transition tombstones the entry: the follow-up
			// statement must not resolve u_a as if the create had landed.
			stmt1 := result.Statements[1]
			exists := t05FindingsByRule(result, 1, t10AlterIndexExist)
			if len(exists) != 0 {
				t.Fatalf("tombstoned state must not fabricate an existence verdict, got %+v", exists)
			}
			gaps1 := t05GapsByRule(result, 1, t10AlterIndexExist)
			if len(gaps1) != 1 || len(gaps1[0].RequiredFacts) != 2 ||
				gaps1[0].RequiredFacts[0] != "target_table.existence" || gaps1[0].RequiredFacts[1] != "target_table.indexes" {
				t.Fatalf("tombstone must surface existence+indexes gaps, got %+v", stmt1.EvidenceGaps)
			}
			if stmt1.Coverage.Status != report.CoverageUnverified {
				t.Fatalf("tombstone must lower coverage to unverified, got %s", stmt1.Coverage.Status)
			}
		})
	}
}

// B4: a TiDB create-derived PK without CLUSTERED|NONCLUSTERED defers to the
// observed tidb_enable_clustered_index — on makes the single-int PK the
// row handle (no PRIMARY object), off makes it a real index, int_only
// defers further to the alter-primary-key config fact (its conditioned
// answers live in the T10-R3 matrix), and an unobserved mode — or an
// int_only mode with the config fact unobserved — leaves the answer
// unknown instead of guessing.
func TestT10R2TiDBHandleFollowsObservedMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		mode           string
		wantExistsDeny bool // 1176-class exists blocker
		wantVisibility bool // 3522-class compat issue
		wantGap        bool // unresolved — bounded gap only
	}{
		{name: "mode on", mode: "on", wantExistsDeny: true},
		{name: "mode int_only + alter_pk unobserved", mode: "int_only", wantGap: true},
		{name: "mode off", mode: "off", wantVisibility: true},
		{name: "mode unobserved", mode: "", wantGap: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: tc.mode}
			result := t10Audit(t, spec.DialectTiDB,
				"CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			stmt1 := result.Statements[1]
			exists := t05FindingsByRule(result, 1, t10AlterIndexExist)
			compat := t05FindingsByRule(result, 1, t10IndexCompat)
			existsDenied := len(exists) == 1 && exists[0].Metadata["exists"] == false
			visibilityDenied := false
			for _, finding := range compat {
				if finding.Metadata["reason_code"] == spec.AdvancedIndexReasonPrimaryVisibility {
					visibilityDenied = true
				}
			}
			if existsDenied != tc.wantExistsDeny || visibilityDenied != tc.wantVisibility {
				t.Fatalf("mode %q: exists_deny=%v visibility_deny=%v, want %v/%v; findings %+v",
					tc.mode, existsDenied, visibilityDenied, tc.wantExistsDeny, tc.wantVisibility, stmt1.Findings)
			}
			if tc.wantGap {
				if len(stmt1.EvidenceGaps) == 0 {
					t.Fatalf("unobserved mode must leave a bounded gap, got none")
				}
			} else if len(stmt1.EvidenceGaps) != 0 {
				t.Fatalf("mode %q resolved — no gaps expected, got %+v", tc.mode, stmt1.EvidenceGaps)
			}
		})
	}
}

// B4 controls: explicit PRIMARY KEY … CLUSTERED|NONCLUSTERED declares the
// storage answer, so the observed session mode must not override it.
func TestT10R2TiDBExplicitClusteredTypeOverridesMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		pkType         string
		wantExistsDeny bool
		wantVisibility bool
	}{
		{name: "explicit nonclustered under on", pkType: " NONCLUSTERED", wantVisibility: true},
		{name: "explicit clustered under off", pkType: " CLUSTERED", wantExistsDeny: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tidbMode: map[string]string{
				" NONCLUSTERED": "on", " CLUSTERED": "off",
			}[tc.pkType]}
			result := t10Audit(t, spec.DialectTiDB,
				"CREATE TABLE t (id INT PRIMARY KEY"+tc.pkType+");\nALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			stmt1 := result.Statements[1]
			exists := t05FindingsByRule(result, 1, t10AlterIndexExist)
			compat := t05FindingsByRule(result, 1, t10IndexCompat)
			existsDenied := len(exists) == 1 && exists[0].Metadata["exists"] == false
			visibilityDenied := false
			for _, finding := range compat {
				if finding.Metadata["reason_code"] == spec.AdvancedIndexReasonPrimaryVisibility {
					visibilityDenied = true
				}
			}
			if existsDenied != tc.wantExistsDeny || visibilityDenied != tc.wantVisibility {
				t.Fatalf("%s: exists_deny=%v visibility_deny=%v; findings %+v",
					tc.name, existsDenied, visibilityDenied, stmt1.Findings)
			}
		})
	}
}

// --- C: member-legality routing by index kind -----------------------------

// C positive: FULLTEXT binds a TEXT member directly — no prefix_required,
// no type refusal, and coverage completes. The ordinary TEXT index keeps
// the 1170 contract in the same run.
func TestT10R2FulltextBindsTextWithoutPrefix(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{t10r2Col("body", "text", true, 0)}
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t10r2Table("t", columns)}}
	policy := t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - prefix_length\n        - fulltext\n"})
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE FULLTEXT INDEX ft_body ON t(body);\nCREATE INDEX idx_body ON t(body);",
		provider, policy,
	)
	stmt0 := result.Statements[0]
	if len(stmt0.Findings) != 0 || len(stmt0.EvidenceGaps) != 0 {
		t.Fatalf("legal FULLTEXT text member must stay clean, got findings=%+v gaps=%+v", stmt0.Findings, stmt0.EvidenceGaps)
	}
	if stmt0.Coverage.Status != report.CoverageComplete {
		t.Fatalf("audited FULLTEXT form must complete, got %s", stmt0.Coverage.Status)
	}
	ordinary := t05FindingsByRule(result, 1, t10IndexCompat)
	if len(ordinary) != 1 || ordinary[0].Metadata["reason_code"] != spec.AdvancedIndexReasonPrefixRequired {
		t.Fatalf("ordinary TEXT index keeps prefix_required, got %+v", result.Statements[1].Findings)
	}
}

// C create-table path: an inline FULLTEXT member binds the declared TEXT
// column the same way.
func TestT10R2FulltextInlineCreateTable(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.46"}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (body TEXT, FULLTEXT KEY ft_body (body));",
		provider, t10r2CompatPolicy(t, "prefix_length", "fulltext"),
	)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("inline FULLTEXT text member must stay clean, got findings=%+v gaps=%+v", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("audited FULLTEXT form must complete, got %s", stmt.Coverage.Status)
	}
}

// C negative: a FULLTEXT member bound outside the text family is the 1283
// determined negative — the transition is provably refused.
func TestT10R2FulltextRejectsNonTextMember(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{t10r2Col("n", "int", true, 0)}
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t10r2Table("t", columns)}}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE FULLTEXT INDEX ft_n ON t(n);\nALTER TABLE t ALTER INDEX ft_n INVISIBLE;",
		provider, t10Policy(t,
			t10Rule{id: t10AlterIndexExist},
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - fulltext\n"},
		),
	)
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != spec.AdvancedIndexReasonFulltextColumnType {
		t.Fatalf("non-text FULLTEXT member must answer fulltext_column_type, got %+v", result.Statements[0].Findings)
	}
	// The provably refused create tombstones the entry — the follow-up
	// alter must not resolve ft_n as if it had landed.
	gaps1 := t05GapsByRule(result, 1, t10AlterIndexExist)
	if len(gaps1) != 1 || len(gaps1[0].RequiredFacts) != 2 ||
		gaps1[0].RequiredFacts[0] != "target_table.existence" {
		t.Fatalf("refused create must tombstone, got %+v", result.Statements[1].EvidenceGaps)
	}
}

// C gap attribution: only the fulltext feature enabled, a FULLTEXT member
// whose column type is unproven reports the fulltext-attributed gap — not
// a filtered-out prefix_length gap that leaves the enabled check silent.
func TestT10R2FulltextUnknownTypeAttributesGap(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "body"}}
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": t10r2Table("t", columns)}}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE FULLTEXT INDEX ft_body ON t(body);",
		provider, t10r2CompatPolicy(t, "fulltext"),
	)
	gaps := t05GapsByRule(result, 0, t10IndexCompat)
	if len(gaps) != 1 || gaps[0].RequiredFacts[0] != spec.PrimaryRoleGapColumnNullables {
		t.Fatalf("unproven member type must attribute the column-set gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("the member gap must lower coverage to unverified, got %s", result.Statements[0].Coverage.Status)
	}
}

// --- A: coverage graduation + post-state fidelity -------------------------

// A: legal member forms that used to be blanket-unsupported now complete —
// and the post-state they publish keeps member identity for the follow-up
// statement.
func TestT10R2GraduatedFormsPublishMembers(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{
		t10r2Col("vc", "varchar", true, 32),
		t10r2Col("a", "int", true, 0),
		t10r2Col("j", "json", true, 0),
	}
	cases := []struct {
		name    string
		dialect spec.Dialect
		banner  string
		addSQL  string
		index   string
	}{
		{name: "prefix", dialect: spec.DialectMySQL, banner: "8.0.46", addSQL: "ALTER TABLE t ADD INDEX i_p (vc(3));", index: "i_p"},
		{name: "descending", dialect: spec.DialectMySQL, banner: "8.0.46", addSQL: "ALTER TABLE t ADD INDEX i_d (a DESC);", index: "i_d"},
		{name: "expression", dialect: spec.DialectMySQL, banner: "8.0.46", addSQL: "ALTER TABLE t ADD INDEX i_e ((a + 1));", index: "i_e"},
		{name: "multi_valued", dialect: spec.DialectMySQL, banner: "8.0.46", addSQL: "ALTER TABLE t ADD INDEX i_mv ((CAST(j AS UNSIGNED ARRAY)));", index: "i_mv"},
		{name: "expression tidb", dialect: spec.DialectTiDB, banner: "8.0.11-TiDB-v8.5.0", addSQL: "ALTER TABLE t ADD INDEX i_e ((LOWER(vc)));", index: "i_e"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": t10r2Table("t", columns)}}
			result := t10Audit(t, tc.dialect,
				tc.addSQL+"\nALTER TABLE t ALTER INDEX "+tc.index+" INVISIBLE;",
				provider, t10VisibilityPolicy(t),
			)
			stmt0 := result.Statements[0]
			if len(result.Unsupported) != 0 {
				t.Fatalf("graduated member form must carry no unsupported evidence, got %+v", result.Unsupported)
			}
			if stmt0.Coverage.Status != report.CoverageComplete {
				t.Fatalf("audited member form must complete, got coverage=%s gaps=%+v", stmt0.Coverage.Status, stmt0.EvidenceGaps)
			}
			stmt1 := result.Statements[1]
			if len(stmt1.EvidenceGaps) != 0 {
				t.Fatalf("published member must resolve — no gaps, got %+v", stmt1.EvidenceGaps)
			}
			if len(t05FindingsByRule(result, 1, t10AlterIndexExist)) != 0 {
				t.Fatalf("published member must exist for the follow-up alter, got %+v", stmt1.Findings)
			}
			if stmt1.Coverage.Status != report.CoverageComplete {
				t.Fatalf("member-resolution read must complete, got %s", stmt1.Coverage.Status)
			}
		})
	}
}

// A TiDB GLOBAL on a vouched partitioned table: the unique-plus-partitioned
// gate passes, the statement completes, and the member publishes.
func TestT10R2TiDBGlobalPartitionedPublishes(t *testing.T) {
	t.Parallel()
	partitioned := true
	table := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
	table.Partitioned = &partitioned
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ADD UNIQUE KEY u_g (a) GLOBAL;\nALTER TABLE t ALTER INDEX u_g INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	if len(result.Unsupported) != 0 {
		t.Fatalf("TiDB GLOBAL on a partitioned table is audited, got %+v", result.Unsupported)
	}
	stmt0 := result.Statements[0]
	if len(stmt0.Findings) != 0 || len(stmt0.EvidenceGaps) != 0 {
		t.Fatalf("legal GLOBAL form must stay clean, got findings=%+v gaps=%+v", stmt0.Findings, stmt0.EvidenceGaps)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 || len(t05FindingsByRule(result, 1, t10AlterIndexExist)) != 0 {
		t.Fatalf("published GLOBAL member must resolve, got findings=%+v gaps=%+v",
			result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
}

// A negatives: GLOBAL on a non-unique kind and on a non-partitioned table
// are the 8200 determined negatives — refused, so nothing publishes.
func TestT10R2TiDBGlobalDeterminedNegatives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		sql    string
		reason string
		table  func() *spec.TableSnapshot
	}{
		{name: "non-unique global", sql: "ALTER TABLE t ADD KEY g (a) GLOBAL;", reason: spec.AdvancedIndexReasonGlobalNotUnique,
			table: func() *spec.TableSnapshot {
				p := true
				tb := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
				tb.Partitioned = &p
				return tb
			}},
		{name: "non-partitioned global", sql: "ALTER TABLE t ADD UNIQUE KEY g (a) GLOBAL;", reason: spec.AdvancedIndexReasonGlobalNotPartitioned,
			table: func() *spec.TableSnapshot {
				p := false
				tb := t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)})
				tb.Partitioned = &p
				return tb
			}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": tc.table()}}
			result := t10Audit(t, spec.DialectTiDB, tc.sql, provider,
				t10r2CompatPolicy(t, "global"))
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != tc.reason {
				t.Fatalf("%s must answer %s, got %+v", tc.name, tc.reason, result.Statements[0].Findings)
			}
		})
	}
}

// A unknown: unvouched partition state on a GLOBAL declaration is a bounded
// gap, never a guessed refusal.
func TestT10R2TiDBGlobalUnvouchedPartitionGap(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{
		"t": t10r2Table("t", []spec.Column{t10r2Col("a", "int", true, 0)}),
	}}
	result := t10Audit(t, spec.DialectTiDB, "ALTER TABLE t ADD UNIQUE KEY g (a) GLOBAL;", provider,
		t10r2CompatPolicy(t, "global"))
	gaps := t05GapsByRule(result, 0, t10IndexCompat)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("unvouched partition state must attribute the partitions gap, got %+v", result.Statements[0].EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
		t.Fatalf("unvouched partition state never fabricates the negative, got %+v", result.Statements[0].Findings)
	}
}

// --- D: provider-classification regression --------------------------------

// D: a real column literally named `NULL` stays a plain member end to end —
// the provider classification fix must not disturb ordinary index
// resolution or primary-role determination on such a table.
func TestT10R2RealNullNamedColumnStaysPlain(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{
		{Name: "NULL", Type: "int", NotNull: true},
		t10r2Col("a", "int", true, 0),
	}
	index := spec.Index{Name: "i_null", Kind: spec.IndexKindSecondary, Columns: []string{"NULL"}}
	index.SetKeyParts([]spec.IndexKeyPart{{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "NULL"}})
	table := t10r2Table("t", columns, index)
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX i_null INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	stmt := result.Statements[0]
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("a real `NULL` column member must resolve plainly, got gaps %+v", stmt.EvidenceGaps)
	}
	if len(t05FindingsByRule(result, 0, t10InvisibleForbid)) != 1 {
		t.Fatalf("declared invisibility intent must report on the member, got %+v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("plain member resolution must complete, got %s", stmt.Coverage.Status)
	}
}
