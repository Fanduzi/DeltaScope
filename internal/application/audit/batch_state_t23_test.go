package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t23_test.go pins the T23/#102 ordered-state contract: a
// statically-refused AUTO_RANDOM CREATE never publishes a shape, an
// undecidable clustered premise invalidates rather than guesses, and a
// fully-premised valid CREATE publishes the shape its successors read.

// t23Provider keeps the three seams independent: configurable instance
// facts (clustered-mode premise), an unvouched table snapshot, and a
// proven-absent object resolver for schema/lookup reads.
type t23Provider struct {
	facts    *spec.InstanceFacts
	snapshot *spec.TableSnapshot
}

func (p *t23Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	if p.facts != nil {
		facts := *p.facts
		return &facts, nil
	}
	return &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}, nil
}

func (p *t23Provider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	if p.snapshot == nil {
		return nil, nil
	}
	shape := *p.snapshot
	return &shape, nil
}

func (p *t23Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusNotFound}, nil
}

func t23State(t *testing.T, sql string, provider *t23Provider) (*batchState, []spec.Statement) {
	t.Helper()
	return t17Enrich(t, spec.DialectTiDB, sql, "req", t17v8046, provider)
}

func t23Entry(t *testing.T, state *batchState) *batchTableEntry {
	t.Helper()
	return state.entries[state.keyFor("req", spec.Table{Schema: "req", Name: "t"})]
}

// A statically-refused CREATE shares the statement's atomic fate: the
// bound slot must never carry a present entry or a fabricated member set.
func TestT23RefusedAutoRandomCreatePublishesNoShape(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE TABLE t (id INT PRIMARY KEY AUTO_RANDOM);",
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM AUTO_INCREMENT);",
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM(16,64));",
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM(15,41));",
		"CREATE TABLE t (id BIGINT AUTO_RANDOM, c INT, PRIMARY KEY (c, id));",
		"CREATE TABLE t (id BIGINT AUTO_RANDOM, c INT, PRIMARY KEY (id) NONCLUSTERED);",
		"CREATE TABLE t (id BIGINT AUTO_RANDOM);",
	} {
		state, _ := t23State(t, sql, &t23Provider{facts: &spec.InstanceFacts{
			Version:                  "8.0.11-TiDB-v8.5.0",
			TiDBEnableClusteredIndex: "on",
			TiDBAlterPrimaryKey:      true,
			TiDBAlterPrimaryKeyKnown: true,
		}})
		entry := t23Entry(t, state)
		if entry != nil && entry.state == tablePresent {
			t.Fatalf("refused create %q published t: %+v", sql, entry)
		}
	}
}

// An unresolvable clustered premise is an honest gap — the slot
// invalidates to unknown instead of guessing either outcome.
func TestT23UndecidableAutoRandomCreateInvalidates(t *testing.T) {
	t.Parallel()
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);",
		&t23Provider{})
	if entry := t23Entry(t, state); entry == nil || entry.state != tableUnknown {
		t.Fatalf("undecidable create must invalidate the bound fact, got %+v", entry)
	}
}

// mode=int_only with the alter-primary-key fact unobserved narrows the
// undecidable premise to alter_primary_key — still no publish.
func TestT23IntOnlyWithoutAlterPKFactInvalidates(t *testing.T) {
	t.Parallel()
	state, _ := t23State(t,
		"CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);",
		&t23Provider{facts: &spec.InstanceFacts{
			Version:                  "8.0.11-TiDB-v8.5.0",
			TiDBEnableClusteredIndex: "int_only",
		}})
	if entry := t23Entry(t, state); entry == nil || entry.state != tableUnknown {
		t.Fatalf("int_only without alter-pk fact must invalidate, got %+v", entry)
	}
}

// Sufficient premises publish the derived shape: the column lands and
// the clustered primary key is the row handle.
func TestT23ApplicableAutoRandomCreatePublishesShape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		facts *spec.InstanceFacts
	}{
		{"mode_on", &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "on"}},
		{"int_only_no_alter_pk", &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0", TiDBEnableClusteredIndex: "int_only", TiDBAlterPrimaryKey: false, TiDBAlterPrimaryKeyKnown: true}},
		{"explicit_clustered_no_facts", &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql := "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);"
			if tc.name == "explicit_clustered_no_facts" {
				sql = "CREATE TABLE t (id BIGINT PRIMARY KEY CLUSTERED AUTO_RANDOM);"
			}
			state, statements := t23State(t, sql, &t23Provider{facts: tc.facts})
			entry := t23Entry(t, state)
			if entry == nil || entry.state != tablePresent || entry.shape == nil {
				t.Fatalf("applicable create must publish t, got %+v", entry)
			}
			if entry.shape.FindColumn("id") == nil || !entry.shape.PrimaryKeyIsHandle {
				t.Fatalf("shape must carry id and the handle primary key, got %+v", entry.shape)
			}
			if len(statements) != 1 {
				t.Fatalf("expected one statement, got %d", len(statements))
			}
		})
	}
}

// A proven refusal preserves an existing table's exact pre-state: the
// published pre-existing shape survives the rejected redefinition.
func TestT23RefusedCreateKeepsPreexistingShape(t *testing.T) {
	t.Parallel()
	existing := &spec.TableSnapshot{
		Exists:  true,
		Schema:  "req",
		Columns: []spec.Column{{Name: "id", Type: "bigint(20)", NotNull: true, Unsigned: true}},
	}
	state, _ := t23State(t,
		"CREATE TABLE t (id INT PRIMARY KEY AUTO_RANDOM);",
		&t23Provider{snapshot: existing})
	entry := t23Entry(t, state)
	if entry == nil || entry.state != tablePresent || entry.shape == nil ||
		entry.shape.FindColumn("id") == nil || !entry.shape.FindColumn("id").Unsigned {
		t.Fatalf("pre-existing shape must survive the refused create, got %+v", entry)
	}
}
