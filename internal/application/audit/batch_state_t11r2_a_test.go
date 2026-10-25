// Package audit pins the T11-R2 same-statement column-declaration facts for
// the single-column primary-key semantic rules: on MySQL the member's type
// facts come from the statement's effective declared shape — an ADD COLUMN
// supplies a member that never existed in the pre-state, a MODIFY supplies
// the member's new definition, and either order lands natively (8.0.46
// probe: ADD COLUMN + ADD PRIMARY KEY accepted in both orders, MODIFY +
// ADD PRIMARY KEY reads the new definition). TiDB validates every action
// against the pre-state instead — a same-statement declaration touching
// the member makes the statement refuse natively (1072/8200 probed), so
// the member fact is unprovable and the rule owes the bounded columns gap
// rather than a finding on a refused shape.
package audit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r2PKPolicy enables exactly one single-column primary-key semantic rule.
func t11r2PKPolicy(t *testing.T, ruleID string) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{ruleID: "      required: true\n"})
}

// A member that only the same statement's ADD COLUMN declares is still a
// proven fact on MySQL — the declared INT violates bigint.require exactly
// like a pre-state INT would.
func TestT11R2MySQLAddColumnSuppliesPKMember(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("k", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN id INT, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 {
		t.Fatalf("the in-statement declared INT member must emit the bigint blocker, got findings=%#v gaps=%#v",
			stmt.Findings, stmt.EvidenceGaps)
	}
	finding := stmt.Findings[0]
	if finding.RuleID != "ddl.table.primary_key.bigint.require" || finding.Level != rule.LevelBlocker {
		t.Fatalf("finding = %#v, want bigint blocker", finding)
	}
	if finding.Metadata["column"] != "id" || !strings.HasPrefix(fmt.Sprint(finding.Metadata["type"]), "int") {
		t.Fatalf("finding metadata must carry the declared member facts, got %#v", finding.Metadata)
	}
	if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("a statement-declared member fact owes no gap, got gaps=%#v coverage=%s", stmt.EvidenceGaps, stmt.Coverage)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("blocker must reject, got %s", result.Verdict)
	}
}

// The declared member can satisfy the rule too — a BIGINT ADD COLUMN keeps
// the statement clean end to end.
func TestT11R2MySQLAddColumnCompliantPKMember(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("k", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN id BIGINT NOT NULL, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 ||
		stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("a declared BIGINT member must stay clean: findings=%#v gaps=%#v coverage=%s verdict=%s",
			stmt.Findings, stmt.EvidenceGaps, stmt.Coverage, result.Verdict)
	}
}

// MODIFY rewrites the member's type inside the same statement: the rule
// must read the new BIGINT definition, not the stale pre-state INT.
func TestT11R2MySQLModifyColumnSuppliesPKMember(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("the member's new declared BIGINT satisfies the rule — the stale INT must not fire, got findings=%#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("both facts proven — no gap owed, got gaps=%#v coverage=%s", stmt.EvidenceGaps, stmt.Coverage)
	}
	if result.Verdict != report.VerdictPass {
		t.Fatalf("compliant declared member must pass, got %s", result.Verdict)
	}
}

// The converse: MODIFY narrows a compliant column into a violation, and
// the rule must read the new INT rather than the stale BIGINT.
func TestT11R2MySQLModifyColumnNarrowsPKMember(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "bigint", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t MODIFY COLUMN id INT NOT NULL, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
		t.Fatalf("the narrowed INT member must emit the blocker, got findings=%#v gaps=%#v", stmt.Findings, stmt.EvidenceGaps)
	}
}

// The sibling predicates consume the same resolved declaration: unsigned
// and auto_increment read the member's declared flags, not the pre-state.
func TestT11R2MySQLDeclaredMemberFeedsSiblingRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		ruleID   string
		declared string
		violates bool
	}{
		{"unsigned on declared int", "ddl.table.primary_key.unsigned.require", "id INT", true},
		{"unsigned on declared int unsigned", "ddl.table.primary_key.unsigned.require", "id INT UNSIGNED", false},
		{"auto_increment on declared int", "ddl.table.primary_key.auto_increment.require", "id INT", true},
		{"auto_increment on declared auto_increment", "ddl.table.primary_key.auto_increment.require", "id INT AUTO_INCREMENT", false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := t11r2PKPolicy(t, tc.ruleID)
			provider := &t10Provider{
				version: "8.0.46",
				tables: map[string]*spec.TableSnapshot{
					"t": t11r1PKShape(t10r2Col("k", "int", true, 0)),
				},
			}
			result := t10Audit(t, spec.DialectMySQL,
				"ALTER TABLE t ADD COLUMN "+tc.declared+", ADD PRIMARY KEY(id);", provider, policy)
			stmt := result.Statements[0]
			if tc.violates {
				if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != tc.ruleID {
					t.Fatalf("declared member must emit %s, got %#v", tc.ruleID, stmt.Findings)
				}
			} else if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
				t.Fatalf("compliant declared member must stay clean: findings=%#v gaps=%#v", stmt.Findings, stmt.EvidenceGaps)
			}
		})
	}
}

// An unrelated column action leaves the member's pre-state fact in charge —
// the proven INT member still violates.
func TestT11R2MySQLUnrelatedColumnActionKeepsPreStateFact(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN x BIGINT, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
		t.Fatalf("the untouched pre-state INT member must still emit the blocker, got %#v", stmt.Findings)
	}
}

// RENAME preserves the member's proven attributes under the new name —
// the renamed INT still violates.
func TestT11R2MySQLRenameCarriesPreStateFact(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("a", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t RENAME COLUMN a TO id, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
		t.Fatalf("the renamed member carries the proven INT facts — blocker owed, got findings=%#v gaps=%#v",
			stmt.Findings, stmt.EvidenceGaps)
	}
	if finding := stmt.Findings[0]; finding.Metadata["column"] != "id" ||
		!strings.HasPrefix(fmt.Sprint(finding.Metadata["type"]), "int") {
		t.Fatalf("the finding binds the NEW member name while the type fact comes from the renamed column — column=%v type=%v",
			finding.Metadata["column"], finding.Metadata["type"])
	}
}

// A member the statement drops before the key lands cannot resolve a
// usable type fact — the rule owes the bounded columns gap, never a
// verdict on a stale or fabricated value.
func TestT11R2MySQLDroppedMemberIsUndecidable(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("id", "int", true, 0), t10r2Col("k", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t DROP COLUMN id, ADD PRIMARY KEY(id);", provider, policy)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("a dropped member fabricates no violation, got %#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 {
		t.Fatalf("an unprovable member owes the bounded gap, got none")
	}
}

// TiDB refuses every same-statement declaration that touches the member
// (1072/8200 probed): the member fact is unprovable — no finding, bounded
// gap. An untouched member still resolves through the pre-state set.
func TestT11R2TiDBSameStatementDeclarationUnprovable(t *testing.T) {
	t.Parallel()
	policy := t11r2PKPolicy(t, "ddl.table.primary_key.bigint.require")
	t.Run("member declared in statement", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables: map[string]*spec.TableSnapshot{
				"t": t11r1PKShape(t10r2Col("k", "int", true, 0)),
			},
		}
		result := t10Audit(t, spec.DialectTiDB,
			"ALTER TABLE t ADD COLUMN id INT, ADD PRIMARY KEY(id);", provider, policy)
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("TiDB refuses the add natively — no member-fact finding, got %#v", stmt.Findings)
		}
		if len(stmt.EvidenceGaps) == 0 {
			t.Fatalf("the statement-declared member is unprovable on TiDB — gap owed")
		}
	})
	t.Run("member modified in statement", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables: map[string]*spec.TableSnapshot{
				"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
			},
		}
		result := t10Audit(t, spec.DialectTiDB,
			"ALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL, ADD PRIMARY KEY(id);", provider, policy)
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("TiDB refuses to operate the same column — no finding on the refused shape, got %#v", stmt.Findings)
		}
		if len(stmt.EvidenceGaps) == 0 {
			t.Fatalf("the touched member is unprovable on TiDB — gap owed")
		}
	})
	t.Run("untouched member keeps pre-state fact", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables: map[string]*spec.TableSnapshot{
				"t": t11r1PKShape(t10r2Col("id", "int", true, 0)),
			},
		}
		result := t10Audit(t, spec.DialectTiDB,
			"ALTER TABLE t ADD COLUMN x INT, ADD PRIMARY KEY(id);", provider, policy)
		stmt := result.Statements[0]
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.table.primary_key.bigint.require" {
			t.Fatalf("an untouched member resolves through the pre-state — blocker owed, got %#v", stmt.Findings)
		}
	})
}

// A disabled rule stays silent even with a statement-declared member.
func TestT11R2DeclaredMemberRuleOff(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{"ddl.table.primary_key.bigint.require": "      required: false\n"})
	provider := &t10Provider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"t": t11r1PKShape(t10r2Col("k", "int", true, 0)),
		},
	}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN id INT, ADD PRIMARY KEY(id);", provider, policy)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("disabled rule must stay silent: %#v", result.Statements[0].Findings)
	}
}
