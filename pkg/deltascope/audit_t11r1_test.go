// Package deltascope pins the T11-R1 constraint-fact contract through the
// public SDK boundary: a provider-proven empty constraint set must survive
// the publicMetadataProvider clone as a proven empty set — never collapse
// into the withheld form — so a name-keyed foreign-key drop answers the
// native missing-name refusal through both public and internal entries,
// while a genuinely unvouched name space still reports its bounded gap.
// input: public Audit requests plus the internal AuditSQL entry over an
// external MetadataProvider fixture
// output: identical findings/verdicts at both entries, proven-empty
// collection fidelity, and unvouched-space evidence-gap controls
// pos: T11-R1 SDK clone/contract coverage under the public API
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t11r1DropFKExistsRule = "ddl.alter.drop_foreign_key.exists.require"

func writeT11R1IsolatedPolicy(t *testing.T, ruleID string) string {
	t.Helper()
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ruleIDs {
		if id == ruleID {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(ruleID))
	path := filepath.Join(t.TempDir(), "t11r1-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

// t11r1FKSnapshot is the provider fixture for the drop-foreign-key family:
// an existing table whose column and constraint collections are proven.
func t11r1FKSnapshot(constraints []spec.Constraint, unknown bool) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Schema:             "golden",
		Exists:             true,
		Table:              &spec.Table{Schema: "golden", Name: "t"},
		Columns:            []spec.Column{{Name: "c", Type: "int", NotNull: true}},
		Indexes:            []spec.Index{},
		Constraints:        constraints,
		ConstraintsUnknown: unknown,
	}
}

// B1: a provider-proven empty constraint set must stay a proven empty set
// through the public SDK clone — the dropped name then answers the native
// missing refusal, not an evidence gap. The internal application entry
// must answer identically on the same snapshot.
func TestT11R1B1PublicSDKProvenEmptyConstraintsAnswersMissing(t *testing.T) {
	t.Parallel()
	snapshot := t11r1FKSnapshot([]spec.Constraint{}, false)
	provider := &fakeMetadataProvider{snapshot: snapshot}
	policyPath := writeT11R1IsolatedPolicy(t, t11r1DropFKExistsRule)

	publicResult, err := Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY missing;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("public audit: %v", err)
	}
	if len(publicResult.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %#v", publicResult.Statements)
	}
	stmt := publicResult.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t11r1DropFKExistsRule ||
		stmt.Findings[0].Level != LevelBlocker {
		t.Fatalf("proven-empty constraint set must answer the missing blocker: findings=%#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("a vouched empty name space owes no gap: %#v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != CoverageComplete || publicResult.Verdict != VerdictReject {
		t.Fatalf("coverage=%s verdict=%s, want complete/reject", stmt.Coverage.Status, publicResult.Verdict)
	}

	internalResult, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY missing;",
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       policyPath,
		MetadataProvider: &t11r1InternalProvider{snapshot: snapshot},
	})
	if err != nil {
		t.Fatalf("internal audit: %v", err)
	}
	if len(internalResult.Statements) != 1 || len(internalResult.Statements[0].Findings) != 1 ||
		internalResult.Statements[0].Findings[0].RuleID != t11r1DropFKExistsRule {
		t.Fatalf("internal entry must answer identically: %#v", internalResult.Statements)
	}
	if len(internalResult.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("internal entry must owe no gap either: %#v", internalResult.Statements[0].EvidenceGaps)
	}
}

// t11r1InternalProvider adapts the same external snapshot to the internal
// provider interface so both entries read identical facts.
type t11r1InternalProvider struct {
	snapshot *spec.TableSnapshot
}

func (p *t11r1InternalProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.46"}, nil
}

func (p *t11r1InternalProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return p.snapshot, nil
}

// B1 control: a genuinely unvouched name space keeps its bounded gap —
// the clone fix must not flatten unknown into proven-empty.
func TestT11R1B1PublicSDKUnvouchedConstraintsStaysGap(t *testing.T) {
	t.Parallel()
	provider := &fakeMetadataProvider{snapshot: t11r1FKSnapshot(nil, true)}
	result, err := Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY missing;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT11R1IsolatedPolicy(t, t11r1DropFKExistsRule),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("public audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("unvouched name space must not fabricate a miss: %#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 || result.Verdict != VerdictReview {
		t.Fatalf("unvouched name space owes the bounded gap: gaps=%#v verdict=%s", stmt.EvidenceGaps, result.Verdict)
	}
}

// B1 clone fidelity: the public provider boundary must preserve the
// non-nil empty slice (proven empty) distinct from nil (withheld), and
// ReferencedColumns must deep-copy so provider-owned slices never alias
// into request-owned snapshots.
func TestT11R1B1PublicProviderClonePreservesEmptyFidelity(t *testing.T) {
	t.Parallel()
	referenced := []string{"id"}
	source := t11r1FKSnapshot([]spec.Constraint{{
		Type:              "foreign_key",
		Name:              "fk_c",
		Columns:           []string{"c"},
		ReferencedTable:   "parent",
		ReferencedColumns: referenced,
	}}, false)
	sourceEmpty := t11r1FKSnapshot([]spec.Constraint{}, false)

	provider := publicMetadataProvider{provider: &fakeMetadataProvider{snapshot: source}}
	mapped, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	mapped.Constraints[0].ReferencedColumns[0] = "mutated"
	if source.Constraints[0].ReferencedColumns[0] != "id" {
		t.Fatal("cloned constraint aliased provider-owned ReferencedColumns")
	}

	provider = publicMetadataProvider{provider: &fakeMetadataProvider{snapshot: sourceEmpty}}
	mappedEmpty, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("clone empty: %v", err)
	}
	if mappedEmpty.Constraints == nil {
		t.Fatal("proven-empty Constraints must survive the clone as non-nil empty, not collapse to nil")
	}
	if len(mappedEmpty.Constraints) != 0 {
		t.Fatalf("proven-empty Constraints cloned with members: %#v", mappedEmpty.Constraints)
	}
}

// B2: a proven foreign-key member answers under an otherwise unvouched
// constraint space — an unmodeled CHECK sibling marks the collection
// unknown, but the bound target owes no gap. A truly unmatched target
// under that same space still owes the collection fact.
func TestT11R1B2ProvenForeignKeyUnderUnvouchedSpaceAnswers(t *testing.T) {
	t.Parallel()
	provider := &fakeMetadataProvider{snapshot: t11r1FKSnapshot([]spec.Constraint{{
		Type:            "foreign_key",
		Name:            "fk_t",
		Columns:         []string{"c"},
		ReferencedTable: "parent",
	}}, true)}
	policyPath := writeT11R1IsolatedPolicy(t, t11r1DropFKExistsRule)

	result, err := Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY fk_t;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 ||
		stmt.Coverage.Status != CoverageComplete || result.Verdict != VerdictPass {
		t.Fatalf("proven member under unvouched space must stay clean: findings=%#v gaps=%#v coverage=%s verdict=%s",
			stmt.Findings, stmt.EvidenceGaps, stmt.Coverage.Status, result.Verdict)
	}

	// An unmatched name under the same unvouched space still owes the
	// collection fact — the answer cannot be confirmed absence.
	result, err = Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY fk_x;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit unmatched: %v", err)
	}
	stmt = result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) == 0 {
		t.Fatalf("unmatched target under unvouched space owes the gap: findings=%#v gaps=%#v", stmt.Findings, stmt.EvidenceGaps)
	}
}

// B2 multi-target: one statement dropping a proven member plus an
// unmatched name — the proven member answers while the unmatched one owes
// the gap, never a single blanket answer over both.
func TestT11R1B2MultiTargetForeignKeyGapParity(t *testing.T) {
	t.Parallel()
	provider := &fakeMetadataProvider{snapshot: t11r1FKSnapshot([]spec.Constraint{{
		Type:            "foreign_key",
		Name:            "fk_t",
		Columns:         []string{"c"},
		ReferencedTable: "parent",
	}}, true)}
	result, err := Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP FOREIGN KEY fk_t, DROP FOREIGN KEY fk_x;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT11R1IsolatedPolicy(t, t11r1DropFKExistsRule),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("proven fk_t must not fire and unvouched fk_x must not fabricate: %#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) == 0 {
		t.Fatal("the unmatched target owes the constraints gap")
	}
}

// B3: a CHECK-bearing MySQL table marks ConstraintsUnknown through the
// unmodeled-type census, but the primary key's absence is independently
// proven — DROP PRIMARY KEY must answer the native 1091-class missing
// blocker, not degrade to ambiguous/gap.
func TestT11R1B3ProvenAbsentPrimaryKeyAnswersUnderCheckCensus(t *testing.T) {
	t.Parallel()
	const dropPKRule = "ddl.alter.drop_primary_key.exists.require"
	provider := &fakeMetadataProvider{snapshot: t11r1FKSnapshot([]spec.Constraint{{
		Type:            "foreign_key",
		Name:            "fk_t",
		Columns:         []string{"c"},
		ReferencedTable: "parent",
	}}, true)}
	result, err := Audit(context.Background(), Request{
		SQL:              "ALTER TABLE t DROP PRIMARY KEY;",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT11R1IsolatedPolicy(t, dropPKRule),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != dropPKRule ||
		stmt.Findings[0].Level != LevelBlocker {
		t.Fatalf("proven PK absence must fire the missing blocker under ConstraintsUnknown: findings=%#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("independently proven PK facts owe no constraint gap: %#v", stmt.EvidenceGaps)
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("verdict = %s, want reject", result.Verdict)
	}
}
