// Package audit verifies the T08 column-governance ordered-state contract.
// input: real parse+enrich chains over positioned add/modify/change actions,
// bounded multi-action successors on both dialect families, and the frozen
// setter/position existence rules against present, absent, and unknown
// provider states
// output: exact column order in published shapes, dialect-parametrized
// premise validation (MySQL accumulates left-to-right, TiDB validates
// against the pre-state), whole-statement tombstones on unprovable net
// effects with no prefix publication, and existence findings/gaps carrying
// the frozen table/action/name/exists metadata
// pos: T08-A8 state machine + rule regression matrix (issue #87)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// columnNames projects a published column set to its ordered names.
func columnNames(t *testing.T, shape *spec.TableSnapshot) []string {
	t.Helper()
	if shape == nil {
		return nil
	}
	names := make([]string, 0, len(shape.Columns))
	for _, column := range shape.Columns {
		names = append(names, column.Name)
	}
	return names
}

func wantColumnOrder(t *testing.T, shape *spec.TableSnapshot, want ...string) {
	t.Helper()
	got := columnNames(t, shape)
	if len(got) != len(want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("columns = %v, want %v", got, want)
		}
	}
}

// TestAuditSQLT08PositionedAlterActionsComplete replaces the frozen T03
// column_position unsupported entries: positioned add/modify/change are
// audited semantics on both dialect families.
func TestAuditSQLT08PositionedAlterActionsComplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)
	cases := []struct {
		name    string
		sql     string
		dialect spec.Dialect
	}{
		{name: "mysql add first", sql: "ALTER TABLE t ADD COLUMN c INT FIRST;", dialect: spec.DialectMySQL},
		{name: "tidb add after", sql: "ALTER TABLE t ADD COLUMN c INT AFTER id;", dialect: spec.DialectTiDB},
		{name: "mysql modify after", sql: "ALTER TABLE t MODIFY COLUMN c INT AFTER id;", dialect: spec.DialectMySQL},
		{name: "mysql change first", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT FIRST;", dialect: spec.DialectMySQL},
		{name: "tidb modify first", sql: "ALTER TABLE t MODIFY COLUMN c INT FIRST;", dialect: spec.DialectTiDB},
		{name: "tidb change after", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT AFTER id;", dialect: spec.DialectTiDB},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    tc.dialect,
				ConfigPath: configPath,
			})
			if err != nil {
				t.Fatalf("expected clean audit, got %v", err)
			}
			for _, u := range result.Unsupported {
				if strings.Contains(u.Feature, "column_position") {
					t.Fatalf("positioned action reported unaudited: %+v", u)
				}
			}
			if result.Statements[0].Coverage.Status != report.CoverageComplete {
				t.Fatalf("coverage = %#v, want complete", result.Statements[0].Coverage)
			}
		})
	}
}

// TestBatchStateT08AddColumnPosition covers single-action add positioning:
// FIRST lands at index 0, AFTER resolves against the known shape, and an
// unresolvable relative column tombstones instead of guessing.
func TestBatchStateT08AddColumnPosition(t *testing.T) {
	t.Parallel()
	t.Run("add first publishes index zero", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ADD COLUMN c INT FIRST;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "c", "a", "b")
	})
	t.Run("add after resolves relative column", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ADD COLUMN c INT AFTER a;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "a", "c", "b")
	})
	t.Run("add after missing reference invalidates", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ADD COLUMN c INT AFTER missing;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("unresolvable AFTER published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("tidb add after resolves", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ADD COLUMN c INT AFTER a;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB, &t05AbsentProvider{})
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "a", "c", "b")
	})
}

// TestBatchStateT08ModifyChangePosition covers position clauses on the
// full-definition actions: the new definition applies, then the column
// moves.
func TestBatchStateT08ModifyChangePosition(t *testing.T) {
	t.Parallel()
	t.Run("modify first moves column", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT, c INT);\nALTER TABLE t MODIFY COLUMN c BIGINT FIRST;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		shape := enriched[2].Metadata.TargetTable
		wantColumnOrder(t, shape, "c", "a", "b")
		if col := shape.FindColumn("c"); col == nil || !strings.HasPrefix(col.Type, "bigint") {
			t.Fatalf("modified type not applied before move: %+v", shape.FindColumn("c"))
		}
	})
	t.Run("modify after moves column", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT, c INT);\nALTER TABLE t MODIFY COLUMN a INT AFTER b;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "b", "a", "c")
	})
	t.Run("modify after self invalidates", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t MODIFY COLUMN a INT AFTER a;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("self-referencing AFTER published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("modify after missing invalidates", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t MODIFY COLUMN a INT AFTER missing;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("unresolvable MODIFY AFTER published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("change after moves renamed column", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT, c INT);\nALTER TABLE t CHANGE COLUMN a a2 BIGINT AFTER b;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		shape := enriched[2].Metadata.TargetTable
		wantColumnOrder(t, shape, "b", "a2", "c")
		if shape.FindColumn("a") != nil {
			t.Fatal("old identity survived the rename")
		}
	})
	t.Run("change after new-name self reference invalidates", func(t *testing.T) {
		t.Parallel()
		enriched := enrichA3(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t CHANGE COLUMN a a2 INT AFTER a2;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL, &t05AbsentProvider{})
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("new-name self AFTER published %+v", enriched[2].Metadata.TargetTable)
		}
	})
}

// TestBatchStateT08MultiActionSuccessor covers the bounded net-effect
// model: disjoint column actions accumulate under MySQL semantics, TiDB
// validates every premise against the pre-state, and any unprovable step
// tombstones the whole statement without publishing a prefix.
func TestBatchStateT08MultiActionSuccessor(t *testing.T) {
	t.Parallel()
	enrich := func(t *testing.T, sql string, dialect spec.Dialect) []spec.Statement {
		t.Helper()
		return enrichA3(t, sql, dialect, &t05AbsentProvider{})
	}

	t.Run("mysql sibling reference accumulates", func(t *testing.T) {
		t.Parallel()
		// MySQL validates left-to-right: c1 exists by the time c2 AFTER c1
		// resolves.
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "a", "c1", "c2")
	})
	t.Run("tidb sibling reference tombstones", func(t *testing.T) {
		t.Parallel()
		// TiDB validates every premise against the pre-state: c1 is not yet
		// a column when c2 AFTER c1 is checked.
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("TiDB sibling AFTER premise published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("tidb disjoint actions publish", func(t *testing.T) {
		t.Parallel()
		enriched := enrich(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ADD COLUMN c1 INT, MODIFY COLUMN b BIGINT;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB)
		shape := enriched[2].Metadata.TargetTable
		wantColumnOrder(t, shape, "a", "b", "c1")
		if col := shape.FindColumn("b"); col == nil || !strings.HasPrefix(col.Type, "bigint") {
			t.Fatalf("modify action not applied: %+v", shape.FindColumn("b"))
		}
	})
	t.Run("duplicate object touch tombstones", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			enriched := enrich(t, "CREATE TABLE t (a INT, c INT);\nALTER TABLE t MODIFY COLUMN c INT, MODIFY COLUMN c BIGINT;\nALTER TABLE t ADD COLUMN d INT;", dialect)
			if enriched[2].Metadata.TargetTable != nil {
				t.Fatalf("%s duplicate modify published %+v", dialect, enriched[2].Metadata.TargetTable)
			}
		}
	})
	t.Run("add then rename same column tombstones", func(t *testing.T) {
		t.Parallel()
		// c1 is touched by both the add and the rename — the pair is the
		// same-column conflict the servers reject.
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t ADD COLUMN c1 INT, RENAME COLUMN c1 TO c2;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("add+rename same column published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("atomicity no prefix on late failure", func(t *testing.T) {
		t.Parallel()
		// The first action is provable but the second premise fails, so the
		// whole statement tombstones — c1 must not survive as a published
		// prefix.
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t ADD COLUMN c1 INT, MODIFY COLUMN missing BIGINT;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		next := enriched[2].Metadata.TargetTable
		if next != nil && next.FindColumn("c1") != nil {
			t.Fatalf("failed statement published a prefix: %+v", next.Columns)
		}
		if next != nil {
			t.Fatalf("failed statement published %+v", next)
		}
	})
	t.Run("unresolvable position premise tombstones", func(t *testing.T) {
		t.Parallel()
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t MODIFY COLUMN a BIGINT, ADD COLUMN c INT AFTER missing;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("unresolvable multi-action AFTER published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("unsupported member action tombstones", func(t *testing.T) {
		t.Parallel()
		// add_index is outside the provable action set.
		enriched := enrich(t, "CREATE TABLE t (a INT);\nALTER TABLE t ADD COLUMN c1 INT, ADD INDEX ix (a);\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("unsupported member action published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("setter pair on distinct columns publishes", func(t *testing.T) {
		t.Parallel()
		enriched := enrich(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t ALTER COLUMN a SET DEFAULT 1, ALTER COLUMN b DROP DEFAULT;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		shape := enriched[2].Metadata.TargetTable
		wantColumnOrder(t, shape, "a", "b")
		a := shape.FindColumn("a")
		b := shape.FindColumn("b")
		if a == nil || !a.HasDefault || a.DefaultValue != "1" {
			t.Fatalf("set default not applied: %+v", a)
		}
		if b == nil || b.HasDefault {
			t.Fatalf("drop default not applied: %+v", b)
		}
	})
	t.Run("canceled multi-action apply publishes nothing", func(t *testing.T) {
		t.Parallel()
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": presentTable("golden", "t", "a", "b")}}
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatalf("load: %v", err)
		}
		key := state.keyFor("golden", spec.Table{Name: "t"})
		before := cloneTableSnapshot(state.entries[key].shape)
		statements := t05A2R1Extract(t, "ALTER TABLE t ADD COLUMN c1 INT, MODIFY COLUMN b BIGINT;")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		defer cancel()
		err := state.apply(ctx, statements[0])
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("apply = %v, want context.Canceled", err)
		}
		if !reflect.DeepEqual(state.entries[key].shape, before) || state.entries[key].state != tablePresent {
			t.Fatal("canceled multi-action ALTER must leave the loaded shape unchanged")
		}
	})
	t.Run("mysql multi modify after mid-state resolves", func(t *testing.T) {
		t.Parallel()
		// Under MySQL accumulation the second action sees the c2 rename
		// already applied? No — c→c2 renames the object; AFTER c2 resolves
		// against accumulated state under MySQL but fails under TiDB
		// pre-state validation... keep to a single clear cell: AFTER an
		// existing column in a mixed modify+add pair.
		enriched := enrich(t, "CREATE TABLE t (a INT, b INT);\nALTER TABLE t MODIFY COLUMN a BIGINT AFTER b, ADD COLUMN c INT;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		wantColumnOrder(t, enriched[2].Metadata.TargetTable, "b", "a", "c")
	})
	t.Run("tidb auto_increment removal tombstones", func(t *testing.T) {
		t.Parallel()
		// TiDB 8.5 rejects dropping AUTO_INCREMENT (8200); no successor may
		// be published for a statement the server refuses.
		enriched := enrich(t, "CREATE TABLE t (pk INT PRIMARY KEY AUTO_INCREMENT, a INT);\nALTER TABLE t MODIFY COLUMN pk INT NOT NULL;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("TiDB AI-removal published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("tidb change dropping auto_increment tombstones", func(t *testing.T) {
		t.Parallel()
		enriched := enrich(t, "CREATE TABLE t (pk INT PRIMARY KEY AUTO_INCREMENT, a INT);\nALTER TABLE t CHANGE COLUMN pk pk INT NOT NULL;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("TiDB change AI-removal published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("tidb multi-action ai removal tombstones whole statement", func(t *testing.T) {
		t.Parallel()
		// The add is provable but the modify premise fails natively, so
		// nothing publishes.
		enriched := enrich(t, "CREATE TABLE t (pk INT PRIMARY KEY AUTO_INCREMENT, a INT);\nALTER TABLE t MODIFY COLUMN pk INT NOT NULL, ADD COLUMN c INT;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectTiDB)
		if enriched[2].Metadata.TargetTable != nil {
			t.Fatalf("TiDB multi-action AI-removal published %+v", enriched[2].Metadata.TargetTable)
		}
	})
	t.Run("mysql modify dropping auto_increment publishes", func(t *testing.T) {
		t.Parallel()
		enriched := enrich(t, "CREATE TABLE t (pk INT PRIMARY KEY AUTO_INCREMENT, a INT);\nALTER TABLE t MODIFY COLUMN pk INT NOT NULL;\nALTER TABLE t ADD COLUMN d INT;", spec.DialectMySQL)
		shape := enriched[2].Metadata.TargetTable
		wantColumnOrder(t, shape, "pk", "a")
		if pk := shape.FindColumn("pk"); pk == nil || pk.AutoIncrement {
			t.Fatalf("MySQL AI removal not applied: %+v", pk)
		}
	})
}

// TestAuditSQLT08ExistenceRules pins the three frozen rule bindings: the
// setter rules fire on the MySQL-family actions (never renaming them to PG
// spellings) and the position rule checks only AFTER references.
func TestAuditSQLT08ExistenceRules(t *testing.T) {
	t.Parallel()
	snapshots := map[string]*spec.TableSnapshot{
		"golden.t": {
			Exists:      true,
			Schema:      "golden",
			Table:       &spec.Table{Schema: "golden", Name: "t"},
			Columns:     []spec.Column{{Name: "existing", Type: "int"}},
			Indexes:     []spec.Index{},
			Constraints: []spec.Constraint{},
		},
	}
	findingsFor := func(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider) []string {
		t.Helper()
		result, err := AuditSQL(context.Background(), Request{
			SQL:              sql,
			Dialect:          dialect,
			Schema:           "golden",
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		ids := make([]string, 0)
		for _, f := range result.Statements[0].Findings {
			ids = append(ids, f.RuleID)
		}
		return ids
	}
	contains := func(ids []string, want string) bool {
		for _, id := range ids {
			if id == want {
				return true
			}
		}
		return false
	}

	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := func() *t05PresentProvider {
				return &t05PresentProvider{snapshots: snapshots}
			}
			t.Run("set default missing column", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t ALTER COLUMN missing SET DEFAULT 1;", dialect, provider())
				if !contains(ids, "ddl.alter.set_default.exists.require") {
					t.Fatalf("missing set_default.exists.require finding, got %v", ids)
				}
			})
			t.Run("drop default missing column", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t ALTER COLUMN missing DROP DEFAULT;", dialect, provider())
				if !contains(ids, "ddl.alter.drop_default.exists.require") {
					t.Fatalf("missing drop_default.exists.require finding, got %v", ids)
				}
			})
			t.Run("after reference missing column", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t MODIFY COLUMN existing INT AFTER missing;", dialect, provider())
				if !contains(ids, "ddl.alter.column_position.exists.require") {
					t.Fatalf("missing column_position.exists.require finding, got %v", ids)
				}
			})
			t.Run("add after missing column", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t ADD COLUMN newc INT AFTER missing;", dialect, provider())
				if !contains(ids, "ddl.alter.column_position.exists.require") {
					t.Fatalf("missing column_position.exists.require finding, got %v", ids)
				}
			})
			t.Run("first has no position premise", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t MODIFY COLUMN existing INT FIRST;", dialect, provider())
				if contains(ids, "ddl.alter.column_position.exists.require") {
					t.Fatalf("FIRST emitted a position-existence finding, got %v", ids)
				}
			})
			t.Run("existing after reference silent", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t MODIFY COLUMN existing INT AFTER existing;", dialect, provider())
				if contains(ids, "ddl.alter.column_position.exists.require") {
					t.Fatalf("resolvable AFTER emitted a finding, got %v", ids)
				}
			})
			t.Run("set default existing column silent", func(t *testing.T) {
				ids := findingsFor(t, "ALTER TABLE t ALTER COLUMN existing SET DEFAULT 1;", dialect, provider())
				if contains(ids, "ddl.alter.set_default.exists.require") {
					t.Fatalf("existing setter target emitted a finding, got %v", ids)
				}
			})
			t.Run("unknown table reports gap not finding", func(t *testing.T) {
				result, err := AuditSQL(context.Background(), Request{
					SQL:              "ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;",
					Dialect:          dialect,
					Schema:           "golden",
					MetadataProvider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{}},
				})
				if err != nil {
					t.Fatalf("audit: %v", err)
				}
				for _, f := range result.Statements[0].Findings {
					if f.RuleID == "ddl.alter.set_default.exists.require" {
						t.Fatalf("unknown state fabricated exists:false finding: %+v", f)
					}
				}
				found := false
				for _, gap := range result.Statements[0].EvidenceGaps {
					if gap.ReasonCode == "unknown_table_state" {
						found = true
					}
				}
				if !found {
					t.Fatalf("unknown state produced no evidence gap: %+v", result.Statements[0].EvidenceGaps)
				}
			})
		})
	}

	// MySQL validates AFTER references left-to-right within the statement, so
	// a sibling-produced name resolves; TiDB validates against the pre-state
	// and must flag the same reference.
	t.Run("sibling add after", func(t *testing.T) {
		t.Parallel()
		ids := findingsFor(t, "ALTER TABLE t ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1;",
			spec.DialectMySQL, &t05PresentProvider{snapshots: snapshots})
		if contains(ids, "ddl.alter.column_position.exists.require") {
			t.Fatalf("MySQL sibling AFTER emitted a position-existence finding, got %v", ids)
		}
		ids = findingsFor(t, "ALTER TABLE t ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1;",
			spec.DialectTiDB, &t05PresentProvider{snapshots: snapshots})
		if !contains(ids, "ddl.alter.column_position.exists.require") {
			t.Fatalf("TiDB sibling AFTER missed the pre-state existence finding, got %v", ids)
		}
	})

	// The accumulated set tracks every sibling action, not only positioned
	// ones: a rename destination is visible to a later AFTER, while names a
	// sibling drop or rename destroys no longer resolve.
	t.Run("mysql rename destination satisfies sibling after", func(t *testing.T) {
		t.Parallel()
		ids := findingsFor(t, "ALTER TABLE t RENAME COLUMN existing TO renamed, ADD COLUMN c INT AFTER renamed;",
			spec.DialectMySQL, &t05PresentProvider{snapshots: snapshots})
		if contains(ids, "ddl.alter.column_position.exists.require") {
			t.Fatalf("MySQL rename destination emitted a position-existence finding, got %v", ids)
		}
	})
	t.Run("mysql dropped sibling fails after premise", func(t *testing.T) {
		t.Parallel()
		ids := findingsFor(t, "ALTER TABLE t DROP COLUMN existing, ADD COLUMN c INT AFTER existing;",
			spec.DialectMySQL, &t05PresentProvider{snapshots: snapshots})
		if !contains(ids, "ddl.alter.column_position.exists.require") {
			t.Fatalf("MySQL dropped-name AFTER missed the existence finding, got %v", ids)
		}
	})
	t.Run("mysql renamed-away name fails after premise", func(t *testing.T) {
		t.Parallel()
		ids := findingsFor(t, "ALTER TABLE t RENAME COLUMN existing TO renamed, ADD COLUMN c INT AFTER existing;",
			spec.DialectMySQL, &t05PresentProvider{snapshots: snapshots})
		if !contains(ids, "ddl.alter.column_position.exists.require") {
			t.Fatalf("MySQL renamed-away AFTER missed the existence finding, got %v", ids)
		}
	})
}

// TestAuditSQLT08PositionFindingMetadata pins the frozen finding identity:
// the offending name is the AFTER relative column, and metadata carries the
// standard table/action/name/exists tuple.
func TestAuditSQLT08PositionFindingMetadata(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE t MODIFY COLUMN existing INT AFTER missing;",
		Dialect: spec.DialectMySQL,
		Schema:  "golden",
		MetadataProvider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": {
				Exists:      true,
				Schema:      "golden",
				Table:       &spec.Table{Schema: "golden", Name: "t"},
				Columns:     []spec.Column{{Name: "existing", Type: "int"}},
				Indexes:     []spec.Index{},
				Constraints: []spec.Constraint{},
			},
		}},
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	var found *struct {
		meta map[string]any
	}
	for i := range result.Statements[0].Findings {
		f := &result.Statements[0].Findings[i]
		if f.RuleID == "ddl.alter.column_position.exists.require" {
			found = &struct{ meta map[string]any }{meta: f.Metadata}
		}
	}
	if found == nil {
		t.Fatalf("position existence finding absent: %+v", result.Statements[0].Findings)
	}
	if found.meta["name"] != "missing" || found.meta["action"] != "modify_column" ||
		found.meta["table"] != "t" || found.meta["exists"] != false {
		t.Fatalf("finding metadata = %#v", found.meta)
	}
}

// PostgreSQL inertness for the new MySQL-family bindings lives in
// t08_column_governance_postgresql_tag_test.go (postgresql build tag).
