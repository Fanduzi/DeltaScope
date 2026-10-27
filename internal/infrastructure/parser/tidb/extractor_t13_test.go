// Package tidb verifies the T13 destructive/data-moving partition alter
// extraction facts: named partition targets, the TRUNCATE ALL form, and the
// EXCHANGE counterpart table + validation clause.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// DROP PARTITION carries every named member in clause order, with the first
// name bound on alter.Name for name-keyed consumers.
func TestExtractorT13DropPartitionTargets(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			statement := extractOne(t, dialect, "ALTER TABLE t DROP PARTITION p0, P1")
			if len(statement.DDL.Alter) != 1 {
				t.Fatalf("expected one alter action, got %#v", statement.DDL.Alter)
			}
			alter := statement.DDL.Alter[0]
			if alter.Action != "drop_partition" {
				t.Fatalf("action = %q, want drop_partition", alter.Action)
			}
			if alter.Name != "p0" {
				t.Fatalf("alter.Name = %q, want p0", alter.Name)
			}
			if len(alter.Partitions) != 2 || alter.Partitions[0] != "p0" || alter.Partitions[1] != "p1" {
				t.Fatalf("Partitions = %#v, want [p0 p1] lower-cased in order", alter.Partitions)
			}
			if alter.AllPartitions {
				t.Fatalf("named DROP PARTITION must not mark AllPartitions")
			}
			// DROP PARTITION binds no second object identity.
			if len(statement.DDL.TableTargets()) != 1 {
				t.Fatalf("targets = %#v, want [t] only", statement.DDL.TableTargets())
			}
		})
	}
}

// TRUNCATE PARTITION accepts a name list or the ALL form; the ALL form is a
// distinct modeled fact, never a fabricated empty name.
func TestExtractorT13TruncatePartitionForms(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			named := extractOne(t, dialect, "ALTER TABLE t TRUNCATE PARTITION p0, p1")
			if len(named.DDL.Alter) != 1 || named.DDL.Alter[0].Action != "truncate_partition" {
				t.Fatalf("named truncate action = %#v", named.DDL.Alter)
			}
			if named.DDL.Alter[0].AllPartitions || len(named.DDL.Alter[0].Partitions) != 2 {
				t.Fatalf("named truncate = %#v, want 2 names without ALL", named.DDL.Alter[0])
			}
			all := extractOne(t, dialect, "ALTER TABLE t TRUNCATE PARTITION ALL")
			if len(all.DDL.Alter) != 1 || all.DDL.Alter[0].Action != "truncate_partition" {
				t.Fatalf("ALL truncate action = %#v", all.DDL.Alter)
			}
			if !all.DDL.Alter[0].AllPartitions || len(all.DDL.Alter[0].Partitions) != 0 {
				t.Fatalf("ALL truncate = %#v, want AllPartitions with empty name list", all.DDL.Alter[0])
			}
		})
	}
}

// EXCHANGE PARTITION keeps the partition name, the exchanged table identity
// as-written (schema qualifier preserved), and the effective validation
// clause — with the exchanged table also entering TableTargets so the
// protected-object path sees both sides.
func TestExtractorT13ExchangePartitionSides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		sql          string
		wantSchema   string
		wantTable    string
		wantValidate string
	}{
		{name: "absent clause", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2", wantTable: "t2", wantValidate: "with"},
		{name: "with validation", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2 WITH VALIDATION", wantTable: "t2", wantValidate: "with"},
		{name: "without validation", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2 WITHOUT VALIDATION", wantTable: "t2", wantValidate: "without"},
		{name: "qualified counterpart", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE s2.t2", wantSchema: "s2", wantTable: "t2", wantValidate: "with"},
	} {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				statement := extractOne(t, dialect, tc.sql)
				if len(statement.DDL.Alter) != 1 {
					t.Fatalf("expected one alter action, got %#v", statement.DDL.Alter)
				}
				alter := statement.DDL.Alter[0]
				if alter.Action != "exchange_partition" {
					t.Fatalf("action = %q, want exchange_partition", alter.Action)
				}
				if len(alter.Partitions) != 1 || alter.Partitions[0] != "p0" {
					t.Fatalf("Partitions = %#v, want [p0]", alter.Partitions)
				}
				if alter.ExchangeTable == nil || alter.ExchangeTable.Name != tc.wantTable {
					t.Fatalf("ExchangeTable = %#v, want name %q", alter.ExchangeTable, tc.wantTable)
				}
				if alter.ExchangeTable.Schema != tc.wantSchema {
					t.Fatalf("ExchangeTable.Schema = %q, want %q (as-written)", alter.ExchangeTable.Schema, tc.wantSchema)
				}
				if alter.ExchangeValidation != tc.wantValidate {
					t.Fatalf("ExchangeValidation = %q, want %q", alter.ExchangeValidation, tc.wantValidate)
				}
				targets := statement.DDL.TableTargets()
				if len(targets) != 2 || targets[0].Name != "t" || targets[1].Name != tc.wantTable {
					t.Fatalf("targets = %#v, want [t %s] — the exchange counterpart is a named object identity", targets, tc.wantTable)
				}
				if targets[1].Schema != tc.wantSchema {
					t.Fatalf("counterpart target schema = %q, want %q", targets[1].Schema, tc.wantSchema)
				}
			})
		}
	}
}

// A partition action inside a multi-spec ALTER keeps its payload: the
// refusal model consumes the complete statement shape.
func TestExtractorT13MixedSpecPartitionPayload(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectMySQL, "ALTER TABLE t ADD COLUMN x INT, DROP PARTITION p0")
	if len(statement.DDL.Alter) != 2 {
		t.Fatalf("expected two alter actions, got %#v", statement.DDL.Alter)
	}
	if statement.DDL.Alter[1].Action != "drop_partition" || statement.DDL.Alter[1].Name != "p0" {
		t.Fatalf("partition payload lost in multi-spec alter: %#v", statement.DDL.Alter[1])
	}
}
