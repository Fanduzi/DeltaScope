package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t23alter_test.go pins the T23-R3 ALTER-side contract: the
// shared checkAutoRandom verdict and the layout-option dispatch publish
// refused-preserved, undecidable-poisoned, and applied-projected
// post-states exactly once across rule and state.

// A proven ALTER refusal keeps the exact pre-state — the created shape
// survives with every member and option intact.
func TestT23AlterAutoRandomRefusalPreservesPreState(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	for _, tc := range []struct{ name, sql string }{
		{"remove_auto_random",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL;"},
		{"decrease_shard_bits",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM(5,64));" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM(3);"},
		{"change_range_bits",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM(5,64));" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM(5,32);"},
		{"add_column_auto_random",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);" +
				"ALTER TABLE t ADD COLUMN c BIGINT AUTO_RANDOM;"},
		{"type_change_on_auto_random",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);" +
				"ALTER TABLE t MODIFY COLUMN id INT AUTO_RANDOM;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t23State(t, tc.sql, &t23Provider{facts: facts})
			entry := t23Entry(t, state)
			if entry == nil || entry.state != tablePresent || entry.shape == nil {
				t.Fatalf("%s: pre-state must be preserved, got %+v", tc.name, entry)
			}
			if len(entry.shape.Columns) != 1 || !entry.shape.Columns[0].AutoRandom {
				t.Fatalf("%s: refused alter must keep the auto_random column, got %+v", tc.name, entry.shape.Columns)
			}
		})
	}
}

// An undecidable ALTER — static pass that still owes the runtime
// allocator overflow check — poisons the target rather than publishing
// a guessed successor shape.
func TestT23AlterAutoRandomUndecidablePoisons(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	for _, tc := range []struct{ name, sql string }{
		{"redeclare_same_bits",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM;"},
		{"increase_bits",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM(5,64));" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM(7,64);"},
		{"auto_increment_to_auto_random",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_INCREMENT);" +
				"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t23State(t, tc.sql, &t23Provider{facts: facts})
			entry := t23Entry(t, state)
			if entry == nil || entry.state == tablePresent {
				t.Fatalf("%s: runtime-premise alter must not publish, got %+v", tc.name, entry)
			}
		})
	}
}

// A MODIFY that never touches AUTO_RANDOM publishes the ordinary
// successor — the gate is invisible to unrelated columns.
func TestT23AlterUntouchedPublishesNormally(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM, note VARCHAR(20) NOT NULL);"+
			"ALTER TABLE t MODIFY COLUMN note VARCHAR(40) NOT NULL;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("unrelated modify must publish, got %+v", entry)
	}
	found := false
	for _, column := range entry.shape.Columns {
		if column.Name == "note" && column.Type == "varchar(40)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("modified column must project, got %+v", entry.shape.Columns)
	}
}

// The four layout options: proven refusals preserve, applied options
// project the catalog value, unproven premises unvouch only the option.
func TestT23AlterTableOptionMatrix(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	for _, tc := range []struct {
		name, sql, key, want string
		wantPresent          bool
	}{
		{"auto_id_cache_applied",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) AUTO_ID_CACHE=5;" +
				"ALTER TABLE t AUTO_ID_CACHE=2;", "auto_id_cache", "2", true},
		{"auto_id_cache_to_1_refused",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) AUTO_ID_CACHE=5;" +
				"ALTER TABLE t AUTO_ID_CACHE=1;", "auto_id_cache", "5", true},
		{"auto_id_cache_from_1_refused",
			"CREATE TABLE t (id BIGINT PRIMARY KEY) AUTO_ID_CACHE=1;" +
				"ALTER TABLE t AUTO_ID_CACHE=9;", "auto_id_cache", "1", true},
		{"pre_split_refused",
			"CREATE TABLE t (id BIGINT PRIMARY KEY);" +
				"ALTER TABLE t PRE_SPLIT_REGIONS=2;", "", "", true},
		{"shard_noop_keeps",
			"CREATE TABLE t (id BIGINT PRIMARY KEY NONCLUSTERED) SHARD_ROW_ID_BITS=4;" +
				"ALTER TABLE t SHARD_ROW_ID_BITS=4;", "shard_row_id_bits", "4", true},
		{"auto_random_base_not_applicable",
			"CREATE TABLE t (id BIGINT PRIMARY KEY);" +
				"ALTER TABLE t AUTO_RANDOM_BASE=100;", "", "", true},
		{"auto_random_base_force_lands",
			"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);" +
				"ALTER TABLE t FORCE AUTO_RANDOM_BASE=100;", "auto_random_base", "100", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t23State(t, tc.sql, &t23Provider{facts: facts})
			entry := t23Entry(t, state)
			if (entry != nil && entry.state == tablePresent) != tc.wantPresent {
				t.Fatalf("%s: presence = %+v, want present=%v", tc.name, entry, tc.wantPresent)
			}
			if tc.key != "" {
				if entry.shape.Options[tc.key] != tc.want {
					t.Fatalf("%s: %s = %q, want %q (options %+v)", tc.name, tc.key,
						entry.shape.Options[tc.key], tc.want, entry.shape.Options)
				}
			}
		})
	}
}

// Non-FORCE rebase stores max(declared, nextGlobalID): the statement
// applies but the catalog value is unvouched — the option key drops.
func TestT23AlterAutoRandomBaseNonForceUnvouched(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM) AUTO_RANDOM_BASE=50;"+
			"ALTER TABLE t AUTO_RANDOM_BASE=100;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("applicable rebase must keep the table, got %+v", entry)
	}
	if _, ok := entry.shape.Options["auto_random_base"]; ok {
		t.Fatalf("non-force rebase must unvouch the catalog base, got %+v", entry.shape.Options)
	}
}

// A real shard change owes the runtime row scan — unproven, so the
// option value is unvouched but every other fact survives.
func TestT23AlterShardRuntimePremiseUnvouches(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "off"}
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY) SHARD_ROW_ID_BITS=4;"+
			"ALTER TABLE t SHARD_ROW_ID_BITS=8;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("table must stay present, got %+v", entry)
	}
	if _, ok := entry.shape.Options["shard_row_id_bits"]; ok {
		t.Fatalf("runtime-premise shard change must unvouch the value, got %+v", entry.shape.Options)
	}
	if len(entry.shape.Columns) != 1 {
		t.Fatalf("unrelated facts must survive, got %+v", entry.shape.Columns)
	}
}

// Disabling the new rules silences findings only — the shared verdict
// lives in the state publisher, so an undecidable ALTER still poisons
// the table and a proven refusal still preserves the pre-state.
func TestT23AlterRuleDisableDoesNotChangeState(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	// Enable only the successor-read rule — both new verdict rules are
	// disabled, so the assertions isolate the state publisher.
	policy := t05PolicyPath(t, map[string]string{
		"ddl.alter.modify_column.exists.require": "",
	})
	// Undecidable redeclare poisons: the successor modify must still owe
	// unknown_table_state even with the finding rule off.
	result := t05A4Audit(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);\n"+
			"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM;\n"+
			"ALTER TABLE t MODIFY COLUMN id BIGINT AUTO_RANDOM(7);",
		spec.DialectTiDB, &t23Provider{facts: facts}, policy)
	for i := 1; i <= 2; i++ {
		for _, finding := range result.Statements[i].Findings {
			if finding.RuleID == "ddl.tidb.alter.auto_random.valid" {
				t.Fatalf("disabled rule still spoke on statement %d: %+v", i, finding)
			}
		}
	}
	poisoned := false
	for _, gap := range result.Statements[2].EvidenceGaps {
		if gap.ReasonCode == "unknown_table_state" {
			poisoned = true
		}
	}
	if !poisoned {
		t.Fatalf("disabled rule must not publish the undecidable successor: %+v", result.Statements[2])
	}
	// Proven refusal preserves: PRE_SPLIT_REGIONS stays refused without a
	// finding, and the successor modify reads the intact pre-state.
	result = t05A4Audit(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY, note VARCHAR(10) NOT NULL);\n"+
			"ALTER TABLE t PRE_SPLIT_REGIONS=2;\n"+
			"ALTER TABLE t MODIFY COLUMN note VARCHAR(20) NOT NULL;",
		spec.DialectTiDB, &t23Provider{facts: facts}, policy)
	for _, finding := range result.Statements[1].Findings {
		if finding.RuleID == "ddl.tidb.alter.table_options.valid" {
			t.Fatalf("disabled rule still spoke: %+v", finding)
		}
	}
	if len(result.Statements[2].EvidenceGaps) != 0 {
		t.Fatalf("proven refusal must preserve the pre-state for the successor: %+v", result.Statements[2].EvidenceGaps)
	}
}

// Non-AUTO_RANDOM rebase target — refused, exact pre-state kept.
func TestT23AlterAutoRandomBaseOnNonAutoRandomTable(t *testing.T) {
	t.Parallel()
	facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY, note INT) AUTO_ID_CACHE=3;"+
			"ALTER TABLE t AUTO_RANDOM_BASE=100;",
		&t23Provider{facts: facts})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("refused rebase keeps the table, got %+v", entry)
	}
	if got := entry.shape.Options["auto_id_cache"]; got != "3" {
		t.Fatalf("pre-state option must survive, got %q", got)
	}
}
