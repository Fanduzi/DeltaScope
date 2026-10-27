// Package audit verifies the T13-R3 ordered-state remediation on the
// public Audit seam: a DROP PARTITION directory publication consumes the
// action's own applicability premise — proven-inapplicable types keep the
// exact pre-state, an unproven method unvouches the directory instead of
// publishing either a removal or a retention it cannot vouch — and a PK
// fact the index-drop path unvouched (residual pointer + Unknown flag)
// never answers the exchange comparison as proven presence.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t13r3RuleTruncateTarget = "ddl.alter.truncate_partition.target"
	t13r3RuleExchangeCtr    = "ddl.alter.exchange_partition.counterpart"
)

// A: a DROP PARTITION on a HASH table is a proven native refusal (1512 —
// probed p6) — the directory and the exact pre-state survive, so a
// successor naming the "dropped" member must not see a fabricated
// partition_absent. The batch-declared shape carries the proven type.
func TestT13R3HashDropKeepsProvenDirectory(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		decl string
	}{
		{"hash", "PARTITION BY HASH(id) PARTITIONS 2"},
		{"key", "PARTITION BY KEY(id) PARTITIONS 2"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := t13r2Audit(t,
				"CREATE TABLE t(id INT) "+tc.decl+";\n"+
					"ALTER TABLE t DROP PARTITION p0;\n"+
					"ALTER TABLE t TRUNCATE PARTITION p0;",
				nil, t13r3RuleTruncateTarget)
			successor := result.Statements[2]
			if got := t05FindingsByRule(result, 2, t13r3RuleTruncateTarget); len(got) != 0 {
				t.Fatalf("%s drop is a proven refusal — p0 must stay proven, got %#v",
					tc.name, successor.Findings)
			}
			if len(successor.EvidenceGaps) != 0 {
				t.Fatalf("%s drop keeps the exact pre-state — successor owes nothing, got %+v",
					tc.name, successor.EvidenceGaps)
			}
		})
	}
}

// A: the proven refusal binds the combinable modifier routes too —
// ALGORITHM=DEFAULT+DROP on HASH refuses (the modifier never rescues the
// action's own applicability), and a version-permitted ALGORITHM=INPLACE
// keeps the same boundary.
func TestT13R3HashDropModifierRoutesKeepDirectory(t *testing.T) {
	t.Parallel()
	t.Run("algorithm_default", func(t *testing.T) {
		t.Parallel()
		result := t13r2Audit(t,
			"CREATE TABLE t(id INT) PARTITION BY HASH(id) PARTITIONS 2;\n"+
				"ALTER TABLE t ALGORITHM=DEFAULT, DROP PARTITION p0;\n"+
				"ALTER TABLE t TRUNCATE PARTITION p0;",
			nil, t13r3RuleTruncateTarget)
		if got := t05FindingsByRule(result, 2, t13r3RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("ALGORITHM=DEFAULT+DROP on hash refuses — p0 stays proven, got %#v",
				result.Statements[2].Findings)
		}
	})
	t.Run("algorithm_inplace_versioned", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{}}
		result := t13r2Audit(t,
			"CREATE TABLE t(id INT) PARTITION BY HASH(id) PARTITIONS 2;\n"+
				"ALTER TABLE t ALGORITHM=INPLACE, DROP PARTITION p0;\n"+
				"ALTER TABLE t TRUNCATE PARTITION p0;",
			provider, t13r3RuleTruncateTarget)
		if got := t05FindingsByRule(result, 2, t13r3RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("INPLACE+DROP on hash refuses even with a permitting version — p0 stays proven, got %#v",
				result.Statements[2].Findings)
		}
	})
}

// A: a known-partitioned table whose method was never observed makes the
// DROP outcome undecidable — the post-state may neither publish a removal
// it cannot prove nor retain a directory it cannot vouch. The directory
// unvouches; the successor owes the member-set fact, never a fabricated
// partition_absent.
func TestT13R3UnknownMethodDropUnvouchesDirectory(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": {Exists: true, Table: &spec.Table{Name: "t"},
			Partitioned: t13r1BoolP(true), PartitionType: nil,
			PartitionNames: []string{"p0", "p1"}},
	})
	result := t13r2Audit(t,
		"ALTER TABLE t DROP PARTITION p0;\nALTER TABLE t TRUNCATE PARTITION p0;",
		provider, t13r3RuleTruncateTarget)
	successor := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t13r3RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("undecidable drop must not fabricate member absence, got %#v", successor.Findings)
	}
	gaps := t05GapsByRule(result, 1, t13r3RuleTruncateTarget)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("unvouched directory owes target_table.partitions, got %+v", successor.EvidenceGaps)
	}
}

// A control: a name the vouched directory never carried is a proven
// refusal under every method state — even while the method is unproven,
// the exact pre-state (vouched members included) stands.
func TestT13R3UnknownMethodDropAbsentNameKeepsDirectory(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": {Exists: true, Table: &spec.Table{Name: "t"},
			Partitioned: t13r1BoolP(true), PartitionType: nil,
			PartitionNames: []string{"p0", "p1"}},
	})
	result := t13r2Audit(t,
		"ALTER TABLE t DROP PARTITION p9;\nALTER TABLE t TRUNCATE PARTITION p0;",
		provider, t13r3RuleTruncateTarget)
	if got := t05FindingsByRule(result, 1, t13r3RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("absent-name drop is a proven refusal — p0 stays proven, got %#v",
			result.Statements[1].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("proven refusal keeps the exact pre-state — successor owes nothing, got %+v",
			result.Statements[1].EvidenceGaps)
	}
}

// C1 public chain: a TiDB withhold-shaped PK placeholder (presence proven,
// object unattributed) gets unvouched by a `DROP INDEX maybe_pk` whose
// ambiguous binding could have dropped the PK object — the residual
// pointer + PrimaryKeyUnknown flag is a legal ordered-state shape. The
// following EXCHANGE must read the flag, not the pointer: PK presence is
// unproven and owes its own fact, never a fabricated definition mismatch.
func TestT13R3ResidualPKPointerStaysUnproven(t *testing.T) {
	t.Parallel()
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables: map[string]*spec.TableSnapshot{
			"t": {Exists: true, Table: &spec.Table{Name: "t"},
				Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
				PrimaryKey:  &spec.Index{Kind: spec.IndexKindPrimary, NameUnknown: true},
				Indexes:     []spec.Index{},
				Partitioned: t13r1BoolP(true), PartitionType: t13r1StrP("range"),
				PartitionNames: []string{"p0", "p1"}},
			"t2": {Exists: true, Table: &spec.Table{Name: "t2"},
				Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
				Partitioned: t13r1BoolP(false),
				Constraints: []spec.Constraint{}},
		},
	}
	enabled := map[string]string{t13r3RuleExchangeCtr: ""}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP INDEX maybe_pk ON t;\n" +
			"ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2;",
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		ConfigPath:       t05PolicyPath(t, enabled),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	exchange := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t13r3RuleExchangeCtr); len(got) != 0 {
		t.Fatalf("unvouched PK presence must not fabricate 1736, got %#v", exchange.Findings)
	}
	found := false
	for _, gap := range t05GapsByRule(result, 1, t13r3RuleExchangeCtr) {
		for _, fact := range gap.RequiredFacts {
			found = found || fact == "target_table.primary_key"
		}
	}
	if !found {
		t.Fatalf("unvouched PK presence owes target_table.primary_key, got %+v", exchange.EvidenceGaps)
	}
}

// B public seam: DECIMAL(M) and DECIMAL(M,0) are the same effective type —
// the declared-shape target against a provider catalog answer must not
// fabricate counterpart_definition_mismatch.
func TestT13R3ExchangeDecimalEffectiveTypePublic(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t2": {Exists: true, Table: &spec.Table{Name: "t2"},
			Columns: []spec.Column{
				{Name: "id", Type: "int", Length: 11},
				{Name: "v", Type: "decimal(12,0)", Length: 12},
			},
			Partitioned: t13r1BoolP(false),
			Constraints: []spec.Constraint{},
			Options:     map[string]string{"engine": "InnoDB"}},
	})
	result := t13r2Audit(t,
		"CREATE TABLE t(id INT, v DECIMAL(12)) ENGINE=InnoDB "+
			"PARTITION BY RANGE(id)(PARTITION p0 VALUES LESS THAN(10), PARTITION p1 VALUES LESS THAN(20));\n"+
			"ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2;",
		provider, t13r3RuleExchangeCtr)
	exchange := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t13r3RuleExchangeCtr); len(got) != 0 {
		t.Fatalf("decimal(12) and decimal(12,0) are the same effective type — no 1736, got %#v",
			exchange.Findings)
	}
}
