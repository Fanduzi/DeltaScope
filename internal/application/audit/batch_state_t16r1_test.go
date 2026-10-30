// Package audit verifies the T16-R1 reviewer-remediation cells: each
// case pins the probe-proven native semantics the independent review
// found divergent — F1, the MySQL 8.x multi-target DROP temporary arm
// conflating "the engine can veto" with "the veto fired"; F2, the rename
// working set losing last-write ordering (produced-then-vacated and
// produced-then-consumed); F3, the ALTER self-rename no-op firing a
// false destination blocker; F4, provider inbound-FK completeness being
// claimed where only outbound edges were loaded.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// F1-A: `DROP tp16, p16_a` with every name resolvable drops BOTH objects on
// MySQL 8.0/8.4 (probe p16_lifecycle_matrix drop-multi-temp-and-persistent:
// tp16 gone → 1146 on the follow-up SELECT). The veto fires only on a
// missing or unvouched sibling; a fully resolvable list never vetoes.
// A successor plain DROP of the same name is the state oracle: a dropped
// temp means the read falls through to the proven-absent persistent name
// (a settled drop.exists blocker), never a ghost temp hit.
func TestBatchStateT16R1DropMultiTempResolvableDropsOn80(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16_a": presentTable("golden", "p16_a", "id"),
			"golden.tp16":  absentTable("golden", "tp16"),
		},
	}
	enriched, err := t05A2R1Enrich(t, context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE tp16, p16_a; DROP TABLE tp16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	// The successor read must prove the temp object is gone: the binding
	// lands on the persistent name, which the provider vouches absent.
	binding := enriched[2].Metadata.TargetTables[0]
	if binding.Snapshot == nil || binding.Snapshot.Exists || binding.Snapshot.Temporary {
		t.Fatalf("resolvable multi-DROP must drop the temp object on 8.x — successor must bind proven-absent persistent, got %+v", binding)
	}
}

// F1-B: `DROP tp16, p16_u` on MySQL 8.x where the sibling is unvouched —
// the veto may or may not have fired (the unknown name may exist), so the
// temp object's fate is undecidable: the slot tombstones rather than
// claiming a refusal it cannot prove. A successor read binds ambiguous.
func TestBatchStateT16R1DropMultiTempUnvouchedUnprovenOn80(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.tp16": absentTable("golden", "tp16"),
			// golden.p16_u intentionally absent from the provider map — the
			// snapshot resolves nil → unvouched.
		},
	}
	enriched, err := t05A2R1Enrich(t, context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE tp16, p16_u; DROP TABLE tp16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	binding := enriched[2].Metadata.TargetTables[0]
	if !binding.Ambiguous {
		t.Fatalf("unvouched-sibling multi-DROP on 8.x leaves the temp fate undecidable — successor must bind ambiguous, got %+v", binding)
	}
}

// F1 regression lock — the arm the review confirmed correct: a proven-missing
// sibling makes the 8.x veto fire, so the temp object survives.
func TestBatchStateT16R1DropMultiTempMissingSiblingKeptOn80(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16_missing": absentTable("golden", "p16_missing"),
			"golden.tp16":        absentTable("golden", "tp16"),
		},
	}
	enriched, err := t05A2R1Enrich(t, context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE tp16, p16_missing; DROP TABLE tp16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	binding := enriched[2].Metadata.TargetTables[0]
	if binding.Snapshot == nil || !binding.Snapshot.Exists || !binding.Snapshot.Temporary {
		t.Fatalf("proven-missing sibling fires the 8.x veto — temp object must survive bound as Temporary, got %+v", binding)
	}
}

// F1 regression lock — MySQL 5.7 sequential: a resolvable list drops the
// temp object regardless of the veto (no veto arm on 5.x).
func TestBatchStateT16R1DropMultiTempResolvableDropsOn57(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "5.7.44",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16_a": presentTable("golden", "p16_a", "id"),
			"golden.tp16":  absentTable("golden", "tp16"),
		},
	}
	enriched, err := t05A2R1Enrich(t, context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE tp16, p16_a; DROP TABLE tp16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	binding := enriched[2].Metadata.TargetTables[0]
	if binding.Snapshot == nil || binding.Snapshot.Exists || binding.Snapshot.Temporary {
		t.Fatalf("5.7 sequential multi-DROP drops the temp object — successor must bind proven-absent persistent, got %+v", binding)
	}
}

// F1 regression lock — TiDB's veto-shaped temp slot: a missing sibling
// keeps the temp object, a resolvable list drops it (probed).
func TestBatchStateT16R1DropMultiTempTiDBVetoShape(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.11-TiDB-v8.5.0",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16_a":       presentTable("golden", "p16_a", "id"),
			"golden.p16_missing": absentTable("golden", "p16_missing"),
			"golden.tp16":        absentTable("golden", "tp16"),
		},
	}
	parsed, err := parseSQL(context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE tp16, p16_a; DROP TABLE tp16;", spec.DialectTiDB)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectTiDB,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	binding := enriched[2].Metadata.TargetTables[0]
	if binding.Snapshot == nil || binding.Snapshot.Exists || binding.Snapshot.Temporary {
		t.Fatalf("resolvable TiDB multi-DROP drops the temp object — successor must bind proven-absent persistent, got %+v", binding)
	}
}

// t16R1Policy enables exactly the listed rules at blocker level through the
// same policy-file path production audits take.
func t16R1Policy(t *testing.T, ruleIDs ...string) string {
	t.Helper()
	enabled := make(map[string]string, len(ruleIDs))
	for _, id := range ruleIDs {
		enabled[id] = ""
	}
	return t05PolicyPath(t, enabled)
}

// F2-A: `RENAME TABLE a TO b, b TO c, d TO b` migrates natively — pair two
// vacates b, so pair three's destination is free at its turn. The reviewer
// counterexample: a produced-then-consumed identity must adjudicate by its
// LAST write, not by a boolean produced set. The successor reads are the
// state oracle: b must land d's shape (column dk), c must land a's shape
// (column id), and a must be gone.
func TestBatchStateT16R1RenameProducedThenVacatedMigrates(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.a": presentTable("golden", "a", "id"),
			"golden.b": absentTable("golden", "b"),
			"golden.c": absentTable("golden", "c"),
			"golden.d": presentTable("golden", "d", "dk"),
		},
	}
	ctx := context.Background()
	result, err := AuditSQL(ctx, Request{
		SQL:     "RENAME TABLE a TO b, b TO c, d TO b; DROP TABLE b; DROP TABLE c; DROP TABLE a;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.rename.source.exists.require",
			"ddl.table.rename.dest.exists.forbid",
			"ddl.table.drop.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("statement %d must not gap on resolved endpoints, got %+v", i, stmt.EvidenceGaps)
		}
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("the full pair sequence migrates — rename must stay clean, got %+v", result.Statements[0].Findings)
	}
	// b lands d's shape and c lands a's shape — proven through the successor
	// DROP bindings, not only through the absent/present verdicts.
	enriched, err := t05A2R1Enrich(t, ctx,
		"RENAME TABLE a TO b, b TO c, d TO b; DROP TABLE b; DROP TABLE c; DROP TABLE a;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	bBind := enriched[1].Metadata.TargetTables[0]
	if bBind.Snapshot == nil || !bBind.Snapshot.Exists || bBind.Snapshot.FindColumn("dk") == nil {
		t.Fatalf("successor b must bind d's migrated shape (column dk), got %+v", bBind.Snapshot)
	}
	cBind := enriched[2].Metadata.TargetTables[0]
	if cBind.Snapshot == nil || !cBind.Snapshot.Exists || cBind.Snapshot.FindColumn("id") == nil {
		t.Fatalf("successor c must bind a's migrated shape (column id), got %+v", cBind.Snapshot)
	}
	// a is consumed and never reoccupied: its successor DROP is the proven
	// 1146 — the only finding the whole batch may report.
	if len(result.Statements[3].Findings) != 1 {
		t.Fatalf("DROP on the consumed a must fire exactly once, got %+v", result.Statements[3].Findings)
	}
	if len(result.Statements[1].Findings) != 0 || len(result.Statements[2].Findings) != 0 {
		t.Fatalf("DROP on migrated b/c must stay clean, got b=%+v c=%+v",
			result.Statements[1].Findings, result.Statements[2].Findings)
	}
}

// F2-B: `RENAME TABLE a TO b, b TO c, b TO a` fails natively at pair three —
// b was already consumed, so the source is missing (1146) and the whole
// statement rolls back: no phantom c may be published. Every endpoint must
// keep its pre-state.
func TestBatchStateT16R1RenameProducedThenConsumedRollsBack(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.a": presentTable("golden", "a", "id"),
			"golden.b": absentTable("golden", "b"),
			"golden.c": absentTable("golden", "c"),
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "RENAME TABLE a TO b, b TO c, b TO a; DROP TABLE c; DROP TABLE a;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.rename.source.exists.require",
			"ddl.table.rename.dest.exists.forbid",
			"ddl.table.drop.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	rename := result.Statements[0]
	if len(rename.Findings) != 1 || rename.Findings[0].RuleID != "ddl.table.rename.source.exists.require" {
		t.Fatalf("pair-three missing source must fire exactly the 1146 blocker, got %+v", rename.Findings)
	}
	// Rollback oracle: c was never published, a was never moved — the
	// successor DROPs see the pre-state.
	if got := result.Statements[1].Findings; len(got) != 1 || got[0].RuleID != "ddl.table.drop.exists.require" {
		t.Fatalf("rollback must not publish phantom c — its DROP is the proven 1146, got %+v", got)
	}
	if got := result.Statements[2].Findings; len(got) != 0 {
		t.Fatalf("rollback must keep a present — its DROP stays clean, got %+v", got)
	}
}

// F3: `ALTER TABLE a RENAME TO a` is a probe-proven no-op — the rule layer
// must not report a destination-occupied blocker, and the state must keep
// the object. The standalone `RENAME TABLE a TO a` stays a 1050 refusal:
// the exemption is bound to the ALTER form, never a global self-pair skip.
func TestBatchStateT16R1AlterRenameSelfPairNoOp(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.a": presentTable("golden", "a", "id"),
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE a RENAME TO a; DROP TABLE a;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.rename.source.exists.require",
			"ddl.table.rename.dest.exists.forbid",
			"ddl.table.drop.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("ALTER self-rename is a proven no-op — no finding, no gap, got findings=%+v gaps=%+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("the no-op keeps a present — successor DROP stays clean, got %+v", result.Statements[1].Findings)
	}
}

// F3 lock: standalone RENAME TABLE self-pair keeps the native 1050 refusal —
// same identity, different syntax form, different verdict.
func TestBatchStateT16R1StandaloneRenameSelfPairRefuses(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.a": presentTable("golden", "a", "id"),
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "RENAME TABLE a TO a;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.rename.dest.exists.forbid"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if got := result.Statements[0].Findings; len(got) != 1 || got[0].RuleID != "ddl.table.rename.dest.exists.forbid" {
		t.Fatalf("standalone self-rename keeps the 1050 blocker, got %+v", got)
	}
}

// F4-A: a provider-loaded present table cannot vouch its inbound-FK set —
// the provider's catalog read covers outbound keys only, so TRUNCATE on the
// bound shape emits the bounded target_table.inbound_fk gap instead of
// fabricating a proven-empty set. The post-state keeps existence and
// members (TRUNCATE never mutates them) while the volatile counter claims
// are withheld: under the undecidable 1701-vs-success outcome neither
// "stats cleared" nor "stats kept" may be claimed.
func TestBatchStateT16R1ProviderTruncateGapsInboundFK(t *testing.T) {
	t.Parallel()
	parent := presentTable("golden", "p16", "id")
	parent.Options = map[string]string{"table_rows": "42"}
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16":  parent,
			"golden.succ": presentTable("golden", "succ", "id"),
		},
	}
	ctx := context.Background()
	result, err := AuditSQL(ctx, Request{
		SQL:     "TRUNCATE TABLE p16; DROP TABLE succ;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.truncate.inbound_fk",
			"ddl.table.truncate.exists.require",
			"ddl.table.drop.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	trunc := result.Statements[0]
	if len(trunc.Findings) != 0 {
		t.Fatalf("an unvouched inbound set must not fabricate a blocker, got %+v", trunc.Findings)
	}
	if len(trunc.EvidenceGaps) != 1 {
		t.Fatalf("provider-path TRUNCATE owes one inbound_fk gap, got %+v", trunc.EvidenceGaps)
	}
	if got := trunc.EvidenceGaps[0].RequiredFacts; len(got) != 1 || got[0] != "target_table.inbound_fk" {
		t.Fatalf("gap must name target_table.inbound_fk, got %+v", got)
	}
	// The bound pre-state carries the unvouched marker while the provider's
	// own object stays untouched.
	if parent.ReferencedByUnknown {
		t.Fatal("the flag must live on the bound clone, never the provider snapshot")
	}
	// Successor state oracle: the table stays present with structure intact,
	// but the volatile counter claim is withheld — the undecidable outcome
	// may be a refusal (rows intact) or a success (counters reset).
	enriched, err := t05A2R1Enrich(t, ctx, "TRUNCATE TABLE p16; DROP TABLE p16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	succBind := enriched[1].Metadata.TargetTables[0]
	if succBind.Snapshot == nil || !succBind.Snapshot.Exists {
		t.Fatalf("undecidable TRUNCATE must keep the table present, got %+v", succBind.Snapshot)
	}
	if _, claimed := succBind.Snapshot.Options["table_rows"]; claimed {
		t.Fatalf("the volatile counter claim must be withheld under the unproven outcome, got %+v", succBind.Snapshot.Options)
	}
	if succBind.Snapshot.FindColumn("id") == nil {
		t.Fatalf("structure members are untouched either way — column must survive, got %+v", succBind.Snapshot)
	}
	if !succBind.Snapshot.ReferencedByUnknown {
		t.Fatal("the unvouched-inbound marker must persist through the transition")
	}
}

// F4-B: a provider parent whose inbound edge is proven by an in-batch child
// fires the 1701 blocker — the proven edge already answers the premise, so
// the unvouched catalog tail adds no redundant gap.
func TestBatchStateT16R1ProviderParentBatchEdgeBlocksNoGap(t *testing.T) {
	t.Parallel()
	// The parent must carry a usable referenced key — the native premise
	// refuses the child's FK otherwise, and no edge would ever land.
	parent := presentTable("golden", "p16", "id")
	parent.PrimaryKey = &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16": parent,
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE ch16 (id INT, p INT, FOREIGN KEY (p) REFERENCES p16 (id)); " +
			"TRUNCATE TABLE p16; DROP TABLE ch16;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.truncate.inbound_fk",
			"ddl.table.drop.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	trunc := result.Statements[1]
	if len(trunc.Findings) != 1 || trunc.Findings[0].RuleID != "ddl.table.truncate.inbound_fk" {
		t.Fatalf("the proven batch edge must fire the 1701 blocker, got %+v", trunc.Findings)
	}
	if len(trunc.EvidenceGaps) != 0 {
		t.Fatalf("the proven edge already answers the premise — no redundant gap, got %+v", trunc.EvidenceGaps)
	}
	// The refused TRUNCATE keeps the parent's exact pre-state: the child
	// survives too, so its successor DROP stays clean.
	if got := result.Statements[2].Findings; len(got) != 0 {
		t.Fatalf("refused TRUNCATE keeps parent and child — successor DROP stays clean, got %+v", got)
	}
}

// F4-C: a batch-created parent owns its complete inbound set — with no
// children, the set is proven empty and TRUNCATE stays clean, no gap.
func TestBatchStateT16R1BatchParentProvenEmptyClean(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16": absentTable("golden", "p16"),
		},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE p16 (id INT PRIMARY KEY); TRUNCATE TABLE p16;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		ConfigPath: t16R1Policy(t,
			"ddl.table.truncate.inbound_fk",
			"ddl.table.truncate.exists.require"),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	trunc := result.Statements[1]
	if len(trunc.Findings) != 0 || len(trunc.EvidenceGaps) != 0 {
		t.Fatalf("a proven-empty inbound set is clean — no finding, no gap, got findings=%+v gaps=%+v",
			trunc.Findings, trunc.EvidenceGaps)
	}
}

// F1 regression lock — IF EXISTS arms the conditional: a missing sibling
// becomes a proven no-op note, so no veto can fire on 8.x and every
// resolvable member drops.
func TestBatchStateT16R1DropIfExistsMultiTempDropsOn80(t *testing.T) {
	t.Parallel()
	provider := &t05PresentProvider{
		t07InstanceVersion: "8.0.46",
		snapshots: map[string]*spec.TableSnapshot{
			"golden.p16_missing": absentTable("golden", "p16_missing"),
			"golden.tp16":        absentTable("golden", "tp16"),
		},
	}
	enriched, err := t05A2R1Enrich(t, context.Background(),
		"CREATE TEMPORARY TABLE tp16 (id INT); DROP TABLE IF EXISTS tp16, p16_missing; DROP TABLE tp16;", provider)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	binding := enriched[2].Metadata.TargetTables[0]
	if binding.Snapshot == nil || binding.Snapshot.Exists || binding.Snapshot.Temporary {
		t.Fatalf("IF EXISTS arms the missing sibling — no veto, temp object must drop, got %+v", binding)
	}
}
