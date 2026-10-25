// Package audit pins the T11-R1 primary-key structural parity: the
// single-column primary-key semantic rules (bigint/unsigned/auto_increment)
// consume the ALTER-declared member through the statement's ordered
// pre-state column facts exactly as they consume CREATE-declared columns —
// a proven violating member emits the same blocker, a proven compliant
// member stays clean, and missing column facts report the bounded gap
// instead of fabricating a violation on a zero-value spec.Column.
// input: seeded provider snapshots plus public AuditSQL ALTER/CREATE pairs
// output: statement findings and evidence gaps asserting CREATE/ALTER parity
// pos: T11-R1 PK structural parity coverage under public audit
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r1PKShape returns the proven pre-state the ALTER add-primary-key
// structural rules read through the public enrichment path: an existing
// table with the given column facts, no primary key, and a proven-empty
// index/constraint space.
func t11r1PKShape(columns ...spec.Column) *spec.TableSnapshot {
	return t10r2Table("t", columns)
}

// t11r1PKAudit audits one statement through the public AuditSQL path with
// the provider answering the target table's pre-state.
func t11r1PKAudit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, policy string) report.Result {
	t.Helper()
	return t10Audit(t, dialect, sql, provider, policy)
}

// A proven INT member on an ALTER add-primary-key must answer the same
// bigint-require blocker the CREATE-declared column does — the statement's
// ordered pre-state column facts feed the predicate, not a name-only stub.
func TestT11R1AlterAddPrimaryKeyBigintParity(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: true\n"})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{
				version: t10r4Version(dialect).Version,
				tables: map[string]*spec.TableSnapshot{
					"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
				},
			}
			alterResult := t11r1PKAudit(t, dialect,
				"ALTER TABLE t ADD PRIMARY KEY(id);", provider, policy)
			if len(alterResult.Statements) != 1 {
				t.Fatalf("expected one statement, got %d", len(alterResult.Statements))
			}
			stmt := alterResult.Statements[0]
			if len(stmt.Findings) != 1 {
				t.Fatalf("ALTER add pk on INT must emit one bigint blocker, got findings=%#v gaps=%#v", stmt.Findings, stmt.EvidenceGaps)
			}
			finding := stmt.Findings[0]
			if finding.RuleID != "ddl.table.primary_key.bigint.require" || finding.Level != rule.LevelBlocker {
				t.Fatalf("finding = %#v, want %s blocker", finding, "ddl.table.primary_key.bigint.require")
			}
			if finding.Metadata["column"] != "id" || finding.Metadata["type"] != "int" {
				t.Fatalf("finding metadata = %#v, want the proven ordered column facts", finding.Metadata)
			}
			if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
				t.Fatalf("proven column facts owe no gap: gaps=%#v coverage=%s", stmt.EvidenceGaps, stmt.Coverage.Status)
			}
			if alterResult.Verdict != report.VerdictReject {
				t.Fatalf("blocker finding must reject, got %s", alterResult.Verdict)
			}
		})
	}

	// The CREATE path emits the identical finding — the parity contract the
	// ALTER path must reproduce.
	createResult := t10Audit(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT NOT NULL, PRIMARY KEY(id));", &t10Provider{version: "8.0.46"}, policy)
	if len(createResult.Statements) != 1 || len(createResult.Statements[0].Findings) != 1 ||
		createResult.Statements[0].Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
		t.Fatalf("CREATE-declared PK on INT must emit the same blocker: %#v", createResult.Statements)
	}
}

// A proven BIGINT member on ALTER satisfies the rule — known compliant,
// no finding, no gap, complete.
func TestT11R1AlterAddPrimaryKeyBigintCompliant(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: true\n"})
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "bigint", true, 0)),
		},
	}
	result := t11r1PKAudit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 ||
		stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("compliant BIGINT member must stay clean: findings=%#v gaps=%#v coverage=%s verdict=%s",
			stmt.Findings, stmt.EvidenceGaps, stmt.Coverage.Status, result.Verdict)
	}
}

// The sibling structural rules consume the same resolved member facts:
// unsigned and auto_increment predicates fire on the proven INT member.
func TestT11R1AlterAddPrimaryKeySiblingRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ruleID   string
		column   spec.Column
		violates bool
	}{
		{"ddl.table.primary_key.unsigned.require", t10r2Col("id", "int", true, 0), true},
		{"ddl.table.primary_key.unsigned.require", spec.Column{Name: "id", Type: "int", NotNull: true, Unsigned: true}, false},
		{"ddl.table.primary_key.auto_increment.require", t10r2Col("id", "int", true, 0), true},
		{"ddl.table.primary_key.auto_increment.require", spec.Column{Name: "id", Type: "int", NotNull: true, AutoIncrement: true}, false},
	} {
		tc := tc
		t.Run(tc.ruleID+"/violates="+map[bool]string{true: "yes", false: "no"}[tc.violates], func(t *testing.T) {
			t.Parallel()
			policy := t05PolicyPath(t, map[string]string{tc.ruleID: "      required: true\n"})
			provider := &t10Provider{
				version: "8.0.46",
				tables: map[string]*spec.TableSnapshot{
					"t": t11r1PKShape(tc.column),
				},
			}
			result := t11r1PKAudit(t, spec.DialectMySQL,
				"ALTER TABLE t ADD PRIMARY KEY(id);", provider, policy)
			stmt := result.Statements[0]
			if tc.violates {
				if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != tc.ruleID ||
					stmt.Findings[0].Level != rule.LevelBlocker {
					t.Fatalf("proven violating member must emit the blocker: %#v", stmt.Findings)
				}
			} else if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
				t.Fatalf("proven compliant member must stay clean: findings=%#v gaps=%#v", stmt.Findings, stmt.EvidenceGaps)
			}
		})
	}
}

// A withheld column set cannot prove the member's type facts — the rule
// owes the bounded columns gap rather than a fabricated violation on the
// name-only stub.
func TestT11R1AlterAddPrimaryKeyWithheldColumnsReportsGap(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: true\n"})
	shape := t11r1PKShape()
	shape.Columns = nil
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": shape},
	}
	result := t11r1PKAudit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("withheld columns must not fabricate a violation: %#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 {
		t.Fatal("withheld columns owe the bounded evidence gap")
	}
	found := false
	for _, gap := range stmt.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == "target_table.columns" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("gap must name target_table.columns: %#v", stmt.EvidenceGaps)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("bounded gap must hold review, got %s", result.Verdict)
	}
}

// The structured add-constraint primary-key form resolves identically —
// the member declaration reaches the same ordered facts.
func TestT11R1AlterAddConstraintPrimaryKeyParity(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: true\n"})
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
		},
	}
	result := t11r1PKAudit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD CONSTRAINT pk_t PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
		t.Fatalf("add_constraint PK form must emit the blocker: %#v", stmt.Findings)
	}
}

// A disabled rule stays silent even with proven violating facts.
func TestT11R1AlterAddPrimaryKeyRuleOff(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: false\n"})
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
		},
	}
	result := t11r1PKAudit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD PRIMARY KEY(id);", provider, policy)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("disabled rule must stay silent: %#v", result.Statements[0].Findings)
	}
}
