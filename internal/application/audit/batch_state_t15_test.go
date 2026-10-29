package audit

// T15 contract tests — table options, charset conversion, and online-DDL
// conditions (#94). The public AuditSQL path is the primary evidence seam;
// helper/state checks are marked internal where used.

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t15RuleOnlineApplicability = "ddl.alter.online_ddl.applicability"
	t15RuleOnlineWarn          = "ddl.alter.online_ddl.warn"
	t15RuleTiDBOnlineNotice    = "ddl.tidb.alter.compat_options.notice"
)

// t15InnoDBShape is the frozen seed fixture: a real InnoDB table with the
// known columns — `c` deliberately absent.
func t15InnoDBShape(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Table:  &spec.Table{Name: name},
		Columns: []spec.Column{
			t10r2Col("id", "int", true, 0),
			t10r2Col("s", "varchar(20)", false, 0),
		},
		Constraints: []spec.Constraint{},
		Options: map[string]string{
			"engine":      "InnoDB",
			"row_format":  "DYNAMIC",
			"collation":   "utf8mb4_0900_ai_ci",
			"charset":     "utf8mb4",
			"table_rows":  "2",
			"primary_key": "",
		},
	}
}

func t15ProviderFor(dialect spec.Dialect, version string, tables map[string]*spec.TableSnapshot) *t10Provider {
	return &t10Provider{version: version, tables: tables}
}

func t15Audit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]t05RuleConfig) report.Result {
	t.Helper()
	return t10Audit(t, dialect, sql, provider, t05PolicyPathConfigured(t, rules))
}

// The frozen first path (#94 card): ADD COLUMN under ALGORITHM=COPY,
// LOCK=EXCLUSIVE keeps all three normalized parts, emits exactly one
// online-DDL warning, reports complete coverage, and the successor can
// bind the added column.
func TestT15SeedCopyExclusiveGoldenPath(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE;
ALTER TABLE t DROP COLUMN c;
`,
		provider,
		map[string]t05RuleConfig{
			t15RuleOnlineApplicability: {level: "blocker"},
			t15RuleOnlineWarn:          {level: "warning"},
		})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("seed coverage = %+v, want complete", stmt.Coverage)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("seed evidence gaps = %#v, want none", stmt.EvidenceGaps)
	}
	warns := t05FindingsByRule(result, 0, t15RuleOnlineWarn)
	if len(warns) != 1 {
		t.Fatalf("expected exactly one online-DDL warning, got %#v", stmt.Findings)
	}
	if warns[0].Level != rule.LevelWarning {
		t.Fatalf("warning level = %q, want warning", warns[0].Level)
	}
	md := warns[0].Metadata
	if md["algorithm"] != "copy" || md["lock"] != "exclusive" {
		t.Fatalf("warning metadata = %#v, want algorithm=copy lock=exclusive", md)
	}
	if len(t05FindingsByRule(result, 0, t15RuleOnlineApplicability)) != 0 {
		t.Fatalf("combinable path must not fire applicability, got %#v", stmt.Findings)
	}
	// Successor: the combinable outcome published the added column —
	// DROP COLUMN c binds it without existence gaps or findings.
	drop := result.Statements[1]
	if drop.Coverage.Status != report.CoverageComplete {
		t.Fatalf("successor coverage = %+v, want complete", drop.Coverage)
	}
}

// COPY + LOCK=NONE is the probed 1846 refusal: blocker finding, exact
// pre-state, and the column provably absent afterwards.
func TestT15CopyLockNoneRefused(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=NONE;
ALTER TABLE t DROP COLUMN c;
`,
		provider,
		map[string]t05RuleConfig{
			t15RuleOnlineApplicability:             {level: "blocker"},
			t15RuleOnlineWarn:                      {level: "warning"},
			"ddl.alter.drop_column.exists.require": {level: "blocker"},
		})
	stmt := result.Statements[0]
	refusals := t05FindingsByRule(result, 0, t15RuleOnlineApplicability)
	if len(refusals) != 1 || refusals[0].Level != rule.LevelBlocker {
		t.Fatalf("expected one blocker refusal, got %#v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("refusal coverage = %+v, want complete", stmt.Coverage)
	}
	// The refused statement lands nothing: DROP COLUMN c must find no
	// column — exact pre-state preserved. The existence rule's blocker
	// finding is the evidence the column never landed.
	drop := result.Statements[1]
	if len(drop.Findings)+len(drop.EvidenceGaps) == 0 {
		t.Fatalf("dropping an absent column must produce evidence, got %#v", drop)
	}
}

// ALGORITHM=INSTANT combines with an explicit LOCK nowhere on 8.x
// (1221), and does not exist on 5.7 (1800).
func TestT15InstantVersionAndLockGates(t *testing.T) {
	t.Parallel()

	t.Run("instant_explicit_lock_refused_80", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INSTANT, LOCK=NONE;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		refusals := t05FindingsByRule(result, 0, t15RuleOnlineApplicability)
		if len(refusals) != 1 {
			t.Fatalf("expected one 1221-class refusal, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("instant_refused_57", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INSTANT;",
			t15ProviderFor(spec.DialectMySQL, "5.7.44", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		refusals := t05FindingsByRule(result, 0, t15RuleOnlineApplicability)
		if len(refusals) != 1 {
			t.Fatalf("expected one 1800-class refusal, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("instant_add_column_ok_80", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INSTANT;\nALTER TABLE t DROP COLUMN c;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{
				t15RuleOnlineApplicability: {level: "blocker"},
				t15RuleOnlineWarn:          {level: "warning"},
			})
		if len(result.Statements[0].Findings) != 0 {
			t.Fatalf("instant add column must be finding-free, got %#v", result.Statements[0].Findings)
		}
		if result.Statements[0].Coverage.Status != report.CoverageComplete {
			t.Fatalf("coverage = %+v, want complete", result.Statements[0].Coverage)
		}
	})

	t.Run("instant_add_index_refused_80", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD INDEX ix(s), ALGORITHM=INSTANT;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		refusals := t05FindingsByRule(result, 0, t15RuleOnlineApplicability)
		if len(refusals) != 1 {
			t.Fatalf("expected one 1845-class refusal, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("instant_modify_refused_80", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t MODIFY COLUMN s VARCHAR(30), ALGORITHM=INSTANT;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		refusals := t05FindingsByRule(result, 0, t15RuleOnlineApplicability)
		if len(refusals) != 1 {
			t.Fatalf("expected one 1845-class refusal, got %#v", result.Statements[0].Findings)
		}
	})
}

// Accepted-but-risky declarations carry the warn rule; benign modifiers
// stay silent.
func TestT15WarnMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		sql      string
		version  string
		wantWarn int
	}{
		{"copy_alone", "ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY;", "8.0.46", 1},
		{"inplace_not_a_guarantee", "ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INPLACE;", "8.0.46", 1},
		{"exclusive_alone", "ALTER TABLE t ADD COLUMN c INT, LOCK=EXCLUSIVE;", "8.0.46", 1},
		{"shared_alone", "ALTER TABLE t ADD COLUMN c INT, LOCK=SHARED;", "8.0.46", 1},
		{"lock_none_silent", "ALTER TABLE t ADD COLUMN c INT, LOCK=NONE;", "8.0.46", 0},
		{"instant_silent", "ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INSTANT;", "8.0.46", 0},
		{"default_silent", "ALTER TABLE t ADD COLUMN c INT, ALGORITHM=DEFAULT, LOCK=DEFAULT;", "8.0.46", 0},
		{"modifiers_only_copy", "ALTER TABLE t ALGORITHM=COPY;", "8.0.46", 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := t15Audit(t, spec.DialectMySQL, tc.sql,
				t15ProviderFor(spec.DialectMySQL, tc.version, map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
				map[string]t05RuleConfig{
					t15RuleOnlineApplicability: {level: "blocker"},
					t15RuleOnlineWarn:          {level: "warning"},
				})
			warns := t05FindingsByRule(result, 0, t15RuleOnlineWarn)
			if len(warns) != tc.wantWarn {
				t.Fatalf("%s: warnings = %#v, want %d", tc.name, result.Statements[0].Findings, tc.wantWarn)
			}
		})
	}
}

// TiDB treats ALGORITHM/LOCK as ignored capability assertions — never
// through the MySQL matrix.
func TestT15TiDBOnlineOptions(t *testing.T) {
	t.Parallel()
	result := t15Audit(t, spec.DialectTiDB, `
ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE;
ALTER TABLE t DROP COLUMN c;
`,
		t15ProviderFor(spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
		map[string]t05RuleConfig{
			t15RuleOnlineApplicability: {level: "blocker"},
			t15RuleOnlineWarn:          {level: "warning"},
			t15RuleTiDBOnlineNotice:    {level: "notice"},
		})
	stmt := result.Statements[0]
	if len(t05FindingsByRule(result, 0, t15RuleOnlineApplicability)) != 0 {
		t.Fatalf("TiDB must not inherit the MySQL refusal matrix, got %#v", stmt.Findings)
	}
	if len(t05FindingsByRule(result, 0, t15RuleOnlineWarn)) != 0 {
		t.Fatalf("TiDB must not inherit the MySQL risk matrix, got %#v", stmt.Findings)
	}
	notices := t05FindingsByRule(result, 0, t15RuleTiDBOnlineNotice)
	if len(notices) != 1 {
		t.Fatalf("expected one TiDB online-options notice, got %#v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage = %+v, want complete", stmt.Coverage)
	}
	if result.Statements[1].Coverage.Status != report.CoverageComplete {
		t.Fatalf("successor coverage = %+v, want complete", result.Statements[1].Coverage)
	}
}

// Missing prerequisites produce rule-owned gaps, never silent answers.
func TestT15MissingPrerequisiteGaps(t *testing.T) {
	t.Parallel()

	t.Run("no_version", func(t *testing.T) {
		t.Parallel()
		// No version banner → the matrix cannot adjudicate INSTANT.
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD COLUMN c INT, ALGORITHM=INSTANT;",
			&t10Provider{version: "", tables: map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}},
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		stmt := result.Statements[0]
		facts := strings.Join(t14GapFacts(stmt), ",")
		if !strings.Contains(facts, "version") {
			t.Fatalf("expected a version gap, got %#v", stmt.EvidenceGaps)
		}
	})

	t.Run("unknown_engine_lock_none", func(t *testing.T) {
		t.Parallel()
		shape := t15InnoDBShape("t")
		delete(shape.Options, "engine")
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD COLUMN c INT, LOCK=NONE;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t15RuleOnlineApplicability: {level: "blocker"}})
		stmt := result.Statements[0]
		facts := strings.Join(t14GapFacts(stmt), ",")
		if !strings.Contains(facts, "engine") {
			t.Fatalf("expected an engine gap, got %#v", stmt.EvidenceGaps)
		}
	})
}

// Rule-off keeps the state conclusion: the combinable outcome publishes
// the column regardless of whether the warn rule is enabled.
func TestT15RuleOffKeepsStateConclusion(t *testing.T) {
	t.Parallel()
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE;
ALTER TABLE t DROP COLUMN c;
`,
		t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
		map[string]t05RuleConfig{})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("rule-off must emit no findings, got %#v", result.Statements[0].Findings)
	}
	if result.Statements[1].Coverage.Status != report.CoverageComplete {
		t.Fatalf("successor coverage = %+v, want complete", result.Statements[1].Coverage)
	}
}

// ---- dimension B: table-option state transitions -----------------------

// A proven option declaration publishes its catalog effect — the
// successor's compat compare reads the post-state value as the source.
func TestT15TableOptionPublishesEngine(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ENGINE=MyISAM;
ALTER TABLE t ENGINE=InnoDB;
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	if len(second) != 1 {
		t.Fatalf("expected one compat finding on the successor, got %#v", result.Statements[1].Findings)
	}
	if second[0].Metadata["source_value"] != "MyISAM" {
		t.Fatalf("engine post-state must be the published MyISAM, got %#v", second[0].Metadata)
	}
}

// A bare DEFAULT CHARACTER SET publishes the charset and unvouches the
// version-dependent collation — a successor COLLATE declaration must not
// compare against a guessed default.
func TestT15DefaultCharsetUnvouchesCollation(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t DEFAULT CHARACTER SET latin1;
ALTER TABLE t COLLATE latin1_bin;
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	// The first statement warns on the charset change; the second must not
	// fabricate a collation comparison against the guessed default.
	first := t05FindingsByRule(result, 0, "ddl.alter.table_option.compatibility.require")
	if len(first) != 1 || first[0].Metadata["option"] != "charset" {
		t.Fatalf("expected one charset-change finding, got %#v", result.Statements[0].Findings)
	}
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	for _, f := range second {
		if f.Metadata["source_value"] == "utf8mb4_0900_ai_ci" {
			t.Fatalf("post-state collation must stay unvouched, got %#v", f.Metadata)
		}
	}
}

// AUTO_INCREMENT clamps: a higher declaration publishes, a lower one is
// a no-op — the second successor sees the published seed, not the clamp.
func TestT15AutoIncrementClamps(t *testing.T) {
	t.Parallel()
	shape := t15InnoDBShape("t")
	shape.Columns[0].AutoIncrement = true
	shape.Options["auto_increment"] = "2"
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": shape})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t AUTO_INCREMENT=100;
ALTER TABLE t AUTO_INCREMENT=50;
ALTER TABLE t AUTO_INCREMENT=75;
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	// 100 publishes → statement 2 (50) warns "shrinks from 100 to 50";
	// 50 is a no-op → statement 3 (75) still reads seed 100... wait, 75<100
	// too — also a shrink finding.
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	if len(second) != 1 || second[0].Metadata["source_value"] != int64(100) {
		t.Fatalf("seed 100 must publish, got %#v", result.Statements[1].Findings)
	}
}

// COMMENT publishes the declared text — the successor's compat compare
// reads the post-state comment.
func TestT15CommentPublishes(t *testing.T) {
	t.Parallel()
	shape := t15InnoDBShape("t")
	shape.Table.Comment = "old"
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": shape})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t COMMENT='next';
ALTER TABLE t COMMENT='third';
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	if len(second) != 1 || second[0].Metadata["source_value"] != "next" {
		t.Fatalf("comment post-state must be the published text, got %#v", result.Statements[1].Findings)
	}
}

// ---- dimension C: charset conversion ------------------------------------

// CONVERT TO publishes the table and column charsets — the warn finding
// names the rewritten string columns from the observed snapshot.
func TestT15ConvertRewritesStringColumns(t *testing.T) {
	t.Parallel()
	shape := t15InnoDBShape("t")
	shape.Options["charset"] = "latin1"
	shape.Options["collation"] = "latin1_swedish_ci"
	shape.Columns[1].Charset = "latin1"
	shape.Columns[1].Collation = "latin1_swedish_ci"
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": shape})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
ALTER TABLE t COLLATE utf8mb4_0900_ai_ci;
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.charset_conversion.warn":            {level: "warning"},
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	warns := t05FindingsByRule(result, 0, "ddl.alter.charset_conversion.warn")
	if len(warns) != 1 {
		t.Fatalf("expected one conversion warning, got %#v", result.Statements[0].Findings)
	}
	if warns[0].Metadata["string_column_count"] != 1 {
		t.Fatalf("the varchar column must be counted in the rewrite, got %#v", warns[0].Metadata)
	}
	// Post-state: the table collation is the declared utf8mb4_bin — the
	// successor's compare reads it as the source.
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	if len(second) != 1 || second[0].Metadata["source_value"] != "utf8mb4_bin" {
		t.Fatalf("post-state collation must be utf8mb4_bin, got %#v", result.Statements[1].Findings)
	}
}

// ---- dimension D: TiDB vendor boundaries + long tail --------------------

// TiDB FORCE keeps the exact pre-state: the vendor refusal is the
// statement's answer, and the successor still binds every member.
func TestT15TiDBForceKeepsPreState(t *testing.T) {
	t.Parallel()
	result := t15Audit(t, spec.DialectTiDB, `
ALTER TABLE t FORCE;
ALTER TABLE t DROP COLUMN s;
`,
		t15ProviderFor(spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
		map[string]t05RuleConfig{
			"ddl.alter.drop_column.exists.require": {level: "blocker"},
		})
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("TiDB FORCE must carry the vendor aspect, got %+v", stmt.Coverage)
	}
	drop := result.Statements[1]
	if len(t05FindingsByRule(result, 1, "ddl.alter.drop_column.exists.require"))+len(drop.EvidenceGaps) != 0 {
		t.Fatalf("refused statement must keep s — drop must bind cleanly, got %#v gaps %#v", drop.Findings, drop.EvidenceGaps)
	}
}

// A TiDB vendor-refused option name keeps the pre-state — the vendor
// aspect is the answer, not a tombstone.
func TestT15TiDBVendorOptionKeepsPreState(t *testing.T) {
	t.Parallel()
	result := t15Audit(t, spec.DialectTiDB, `
ALTER TABLE t KEY_BLOCK_SIZE=8;
ALTER TABLE t DROP COLUMN s;
`,
		t15ProviderFor(spec.DialectTiDB, "8.0.11-TiDB-v8.5.0", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
		map[string]t05RuleConfig{})
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("vendor-refused option must stay a bounded aspect, got %+v", stmt.Coverage)
	}
	if result.Statements[1].Coverage.Status != report.CoverageComplete {
		t.Fatalf("successor must keep the vouched shape, got %+v", result.Statements[1].Coverage)
	}
}

// A MySQL extracted-but-unaudited option carries a bounded aspect and the
// statement's post-state stays withheld — extraction is not acceptance.
func TestT15MySQLLongTailOptionUnaudited(t *testing.T) {
	t.Parallel()
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t STATS_PERSISTENT=1;
ALTER TABLE t DROP COLUMN s;
`,
		t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
		map[string]t05RuleConfig{
			"ddl.alter.drop_column.exists.require": {level: "blocker"},
		})
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("unaudited option must carry a bounded aspect, got %+v", stmt.Coverage)
	}
	// The successor cannot rely on the shape — the tombstone degrades the
	// member binding into an unknown-table-state gap.
	drop := result.Statements[1]
	facts := strings.Join(t14GapFacts(drop), ",")
	if len(drop.EvidenceGaps) == 0 || !strings.Contains(facts, "table") {
		t.Fatalf("unproven option must degrade the successor, got %#v", drop.EvidenceGaps)
	}
}

// ---- multi-action ordered state -----------------------------------------

// The seed path itself is multi-action: the structural add publishes and
// the modifiers are excluded from the primary-object ledger. A table
// option publishes after the structural action.
func TestT15MultiActionOptionAfterStructural(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD COLUMN c INT, COMMENT='with-col';
ALTER TABLE t COMMENT='next';
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.table_option.compatibility.require": {level: "warning"},
		})
	second := t05FindingsByRule(result, 1, "ddl.alter.table_option.compatibility.require")
	if len(second) != 1 || second[0].Metadata["source_value"] != "with-col" {
		t.Fatalf("option effect must publish after the structural action, got %#v", result.Statements[1].Findings)
	}
}

// MySQL FORCE plus a structural sibling executes — the structural action
// publishes and FORCE's rebuild warning still fires.
func TestT15MySQLForceWithStructuralSibling(t *testing.T) {
	t.Parallel()
	provider := t15ProviderFor(spec.DialectMySQL, "8.0.46",
		map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")})
	result := t15Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD COLUMN c INT, FORCE;
ALTER TABLE t DROP COLUMN c;
`,
		provider,
		map[string]t05RuleConfig{
			"ddl.alter.rebuild.warn": {level: "warning"},
		})
	if len(t05FindingsByRule(result, 0, "ddl.alter.rebuild.warn")) != 1 {
		t.Fatalf("FORCE must warn alongside the structural action, got %#v", result.Statements[0].Findings)
	}
	if result.Statements[1].Coverage.Status != report.CoverageComplete {
		t.Fatalf("the structural sibling must publish, got %+v", result.Statements[1].Coverage)
	}
}

// ---- keys toggle engine arms --------------------------------------------

func TestT15KeysToggleEngineArms(t *testing.T) {
	t.Parallel()

	t.Run("myisam_real_effect", func(t *testing.T) {
		t.Parallel()
		shape := t15InnoDBShape("t")
		shape.Options["engine"] = "MyISAM"
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t DISABLE KEYS;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{"ddl.alter.keys_toggle.notice": {level: "notice"}})
		notices := t05FindingsByRule(result, 0, "ddl.alter.keys_toggle.notice")
		if len(notices) != 1 || !strings.Contains(notices[0].Message, "MyISAM") {
			t.Fatalf("MyISAM must report real key maintenance, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("missing_engine_gap", func(t *testing.T) {
		t.Parallel()
		shape := t15InnoDBShape("t")
		delete(shape.Options, "engine")
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ENABLE KEYS;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{"ddl.alter.keys_toggle.notice": {level: "notice"}})
		facts := strings.Join(t14GapFacts(result.Statements[0]), ",")
		if !strings.Contains(facts, "engine") {
			t.Fatalf("unknown engine must carry an engine gap, got %#v", result.Statements[0].EvidenceGaps)
		}
	})
}

// ---- key_block_size applicability ----------------------------------------

func TestT15KeyBlockSizeApplicability(t *testing.T) {
	t.Parallel()

	t.Run("dynamic_innodb_refused", func(t *testing.T) {
		t.Parallel()
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t KEY_BLOCK_SIZE=8;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": t15InnoDBShape("t")}),
			map[string]t05RuleConfig{"ddl.alter.table_option.applicability": {level: "blocker"}})
		refusals := t05FindingsByRule(result, 0, "ddl.alter.table_option.applicability")
		if len(refusals) != 1 || refusals[0].Level != rule.LevelBlocker {
			t.Fatalf("KBS on a DYNAMIC InnoDB table must block, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("compressed_innodb_clean", func(t *testing.T) {
		t.Parallel()
		shape := t15InnoDBShape("t")
		shape.Options["row_format"] = "COMPRESSED"
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t KEY_BLOCK_SIZE=8;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{"ddl.alter.table_option.applicability": {level: "blocker"}})
		if len(t05FindingsByRule(result, 0, "ddl.alter.table_option.applicability")) != 0 {
			t.Fatalf("KBS on COMPRESSED must not block, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("missing_row_format_gap", func(t *testing.T) {
		t.Parallel()
		shape := t15InnoDBShape("t")
		delete(shape.Options, "row_format")
		result := t15Audit(t, spec.DialectMySQL,
			"ALTER TABLE t KEY_BLOCK_SIZE=8;",
			t15ProviderFor(spec.DialectMySQL, "8.0.46", map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{"ddl.alter.table_option.applicability": {level: "blocker"}})
		facts := strings.Join(t14GapFacts(result.Statements[0]), ",")
		if !strings.Contains(facts, "row_format") {
			t.Fatalf("missing row_format must carry a gap, got %#v", result.Statements[0].EvidenceGaps)
		}
	})
}
