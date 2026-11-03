package audit

import (
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var t20v805 = &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}

func t20SuccessorSnapshot(t *testing.T, provider *t10Provider, sql string) *spec.TableSnapshot {
	t.Helper()
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, provider)
	if len(statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(statements))
	}
	metadata := statements[1].Metadata
	if metadata == nil || len(metadata.TargetTables) != 1 {
		t.Fatalf("second target bindings = %#v", metadata)
	}
	return metadata.TargetTables[0].Snapshot
}

func t20PlacementShape(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Table:  &spec.Table{Name: name},
		Columns: []spec.Column{
			t10r2Col("id", "int", false, 0),
		},
		Constraints:          []spec.Constraint{},
		ReferencedBy:         []spec.Table{},
		PlacementPolicyKnown: true,
		Options: map[string]string{
			"engine": "InnoDB",
		},
	}
}

func t20Catalog(policies ...string) map[string]*spec.PlacementPolicyFact {
	catalog := map[string]*spec.PlacementPolicyFact{}
	for _, name := range policies {
		catalog[name] = &spec.PlacementPolicyFact{BoundTargets: []string{}}
	}
	return catalog
}

// The #99 first Golden Path: a proven policy binds a present table and the
// successor publishes the binding plus the inbound reference.
func TestT20AlterTablePlacementGoldenPath(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog("p"),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	snapshot := t20SuccessorSnapshot(t, provider,
		"ALTER TABLE t PLACEMENT POLICY=p; ALTER TABLE t ADD COLUMN c INT;")
	if snapshot == nil || !snapshot.PlacementPolicyKnown || snapshot.PlacementPolicy != "p" {
		t.Fatalf("golden-path successor = %#v, want proven binding to p", snapshot)
	}
}

// Binding a proven-missing policy refuses (native 8239): the successor
// keeps the exact pre-state — proven-unbound stays proven-unbound.
func TestT20BindMissingPolicyRefused(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog(),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	snapshot := t20SuccessorSnapshot(t, provider,
		"ALTER TABLE t PLACEMENT POLICY=missing; ALTER TABLE t ADD COLUMN c INT;")
	if snapshot == nil || !snapshot.PlacementPolicyKnown || snapshot.PlacementPolicy != "" {
		t.Fatalf("missing-policy successor = %#v, want proven pre-state unbound", snapshot)
	}
}

// An unvouched policy catalog withholds the binding answer: the successor
// marks the placement fact unobserved rather than guessing bound/unbound.
func TestT20BindUnvouchedCatalogUnproven(t *testing.T) {
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	snapshot := t20SuccessorSnapshot(t, provider,
		"ALTER TABLE t PLACEMENT POLICY=p; ALTER TABLE t ADD COLUMN c INT;")
	if snapshot == nil || snapshot.PlacementPolicyKnown {
		t.Fatalf("unvouched successor = %#v, want unobserved placement", snapshot)
	}
}

// = DEFAULT is the unconditional unbind: a proven bound table drops the
// binding and the registry forgets the reference.
func TestT20TablePlacementDefaultUnbinds(t *testing.T) {
	bound := t20PlacementShape("t")
	bound.PlacementPolicy = "p"
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog("p"),
		tables:    map[string]*spec.TableSnapshot{"t": bound},
	}
	state, statements := t17Enrich(t, spec.DialectTiDB,
		"ALTER TABLE t PLACEMENT POLICY=DEFAULT; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
	snapshot := statements[1].Metadata.TargetTables[0].Snapshot
	if snapshot == nil || !snapshot.PlacementPolicyKnown || snapshot.PlacementPolicy != "" {
		t.Fatalf("DEFAULT successor = %#v, want proven unbound", snapshot)
	}
	entry := state.placements["p"]
	if entry != nil && len(entry.refs) != 0 {
		t.Fatalf("DEFAULT must dissolve the inbound reference, got %#v", entry.refs)
	}
}

// DROP PLACEMENT POLICY with a proven bound object refuses (8241): the
// policy stays present; an unbound policy drops to proven-absent.
func TestT20DropPolicyDependency(t *testing.T) {
	bound := t20PlacementShape("t")
	bound.PlacementPolicy = "p"
	t.Run("bound policy refuses", func(t *testing.T) {
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: map[string]*spec.PlacementPolicyFact{"p": {BoundTargets: []string{"table:golden.t"}}},
			tables:    map[string]*spec.TableSnapshot{"t": bound},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"DROP PLACEMENT POLICY p; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
		entry := state.placements["p"]
		if entry == nil || entry.state != placementPolicyPresent {
			t.Fatalf("bound-policy drop must keep presence, got %#v", entry)
		}
	})
	t.Run("unbound policy drops", func(t *testing.T) {
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: t20Catalog("p"),
			tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"DROP PLACEMENT POLICY p; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
		entry := state.placements["p"]
		if entry == nil || entry.state != placementPolicyAbsent {
			t.Fatalf("unbound drop = %#v, want proven absent", entry)
		}
	})
	t.Run("drop if exists on absent is a no-op", func(t *testing.T) {
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: t20Catalog(),
			tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"DROP PLACEMENT POLICY IF EXISTS p; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
		entry := state.placements["p"]
		if entry == nil || entry.state != placementPolicyAbsent {
			t.Fatalf("IF EXISTS absent drop = %#v, want proven absent", entry)
		}
	})
}

// CREATE publishes proven presence with the modeled member set; a bare
// re-create on a proven name refuses; IF NOT EXISTS no-ops.
func TestT20CreatePolicyLifecycle(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog(),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	state, _ := t17Enrich(t, spec.DialectTiDB,
		"CREATE PLACEMENT POLICY p PRIMARY_REGION='r1' REGIONS='r1' FOLLOWERS=2; CREATE PLACEMENT POLICY IF NOT EXISTS p FOLLOWERS=5; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
	entry := state.placements["p"]
	if entry == nil || entry.state != placementPolicyPresent {
		t.Fatalf("created policy = %#v, want proven present", entry)
	}
	if len(entry.attrs) != 3 {
		t.Fatalf("created policy options = %#v, want 3 modeled members (IF NOT EXISTS must not overwrite)", entry.attrs)
	}
	t.Run("duplicate bare create refuses", func(t *testing.T) {
		provider2 := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: t20Catalog(),
			tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
		}
		state2, _ := t17Enrich(t, spec.DialectTiDB,
			"CREATE PLACEMENT POLICY p PRIMARY_REGION='r1'; CREATE PLACEMENT POLICY p FOLLOWERS=5; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider2)
		entry2 := state2.placements["p"]
		if entry2 == nil || len(entry2.attrs) != 1 {
			t.Fatalf("refused duplicate must keep first options, got %#v", entry2)
		}
	})
}

// ALTER PLACEMENT POLICY on a proven policy updates the modeled members; on
// a proven-absent name it refuses and keeps the registry absent.
func TestT20AlterPolicyLifecycle(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog(),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	state, _ := t17Enrich(t, spec.DialectTiDB,
		"CREATE PLACEMENT POLICY p PRIMARY_REGION='r1'; ALTER PLACEMENT POLICY p REGIONS='r2'; ALTER PLACEMENT POLICY ghost FOLLOWERS=1; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
	entry := state.placements["p"]
	if entry == nil || entry.state != placementPolicyPresent || len(entry.attrs) != 2 ||
		entry.attrs["primary_region"] != "r1" || entry.attrs["regions"] != "r2" {
		t.Fatalf("altered policy = %#v, want merged members (alter rewrites only named members)", entry)
	}
	if ghost := state.placements["ghost"]; ghost != nil && ghost.state == placementPolicyPresent {
		t.Fatalf("alter on absent name must not fabricate presence, got %#v", ghost)
	}
}

// ALTER TABLE ... PARTITION <name> PLACEMENT POLICY binds one member; the
// table-level fact stays unbound while the member carries the policy.
func TestT20PartitionBind(t *testing.T) {
	shape := t20PlacementShape("t")
	shape.Partitioned = boolPtr(true)
	shape.PartitionNames = []string{"p0", "p1"}
	shape.PartitionPlacements = map[string]string{}
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog("p"),
		tables:    map[string]*spec.TableSnapshot{"t": shape},
	}
	snapshot := t20SuccessorSnapshot(t, provider,
		"ALTER TABLE t PARTITION p0 PLACEMENT POLICY=p; ALTER TABLE t ADD COLUMN c INT;")
	if snapshot == nil || snapshot.PartitionPlacements == nil || snapshot.PartitionPlacements["p0"] != "p" {
		t.Fatalf("partition bind successor = %#v, want p0->p", snapshot)
	}
	if snapshot.PlacementPolicy != "" {
		t.Fatalf("member bind must not publish a table-level binding, got %#v", snapshot.PlacementPolicy)
	}
}

// ALTER RANGE global lands on the probed scope set; a name outside the
// v8.5-probed set (dist/region) refuses (1105) with unchanged state.
func TestT20AlterRange(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog("p"),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	state, _ := t17Enrich(t, spec.DialectTiDB,
		"ALTER RANGE global PLACEMENT POLICY=p; ALTER RANGE dist PLACEMENT POLICY=p; ALTER TABLE t ADD COLUMN c INT;", "golden", t20v805, provider)
	entry := state.placements["p"]
	if entry == nil || !func() bool { _, ok := entry.refs["range:global"]; return ok }() {
		t.Fatalf("global range bind = %#v, want range:global ref", entry)
	}
	if func() bool { _, ok := entry.refs["range:dist"]; return ok }() {
		t.Fatalf("unsupported range must refuse, got %#v", entry.refs)
	}
}

// A placement spec sharing a multi-action ALTER settles atomically: a
// missing policy refuses the whole statement — the sibling column never
// publishes (native mixed ALTER is atomic).
func TestT20MixedAlterAtomicRefusal(t *testing.T) {
	provider := &t10Provider{
		version:   "8.0.11-TiDB-v8.5.0",
		placement: t20Catalog(),
		tables:    map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	snapshot := t20SuccessorSnapshot(t, provider,
		"ALTER TABLE t ADD COLUMN c INT, PLACEMENT POLICY=missing; ALTER TABLE t ADD COLUMN d INT;")
	if snapshot == nil {
		t.Fatal("expected successor snapshot")
	}
	if snapshot.FindColumn("c") != nil {
		t.Fatalf("atomic refusal must not publish the sibling spec: %#v", snapshot.Columns)
	}
}

// A proven table drop dissolves its inbound reference; a proven rename
// rekeys it to the destination identity.
func TestT20DropAndRenameReferences(t *testing.T) {
	t.Run("drop dissolves", func(t *testing.T) {
		bound := t20PlacementShape("t")
		bound.PlacementPolicy = "p"
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: map[string]*spec.PlacementPolicyFact{"p": {BoundTargets: []string{"table:golden.t"}}},
			tables:    map[string]*spec.TableSnapshot{"t": bound},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"DROP TABLE t; CREATE TABLE n (id INT);", "golden", t20v805, provider)
		entry := state.placements["p"]
		if entry != nil && len(entry.refs) != 0 {
			t.Fatalf("proven drop must dissolve the reference, got %#v", entry.refs)
		}
	})
	t.Run("rename rekeys", func(t *testing.T) {
		bound := t20PlacementShape("t")
		bound.PlacementPolicy = "p"
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: map[string]*spec.PlacementPolicyFact{"p": {BoundTargets: []string{"table:golden.t"}}},
			tables:    map[string]*spec.TableSnapshot{"t": bound},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"RENAME TABLE t TO n; DROP PLACEMENT POLICY p;", "golden", t20v805, provider)
		entry := state.placements["p"]
		if entry == nil || entry.state != placementPolicyPresent {
			t.Fatalf("a bound policy must refuse the drop (8241), got %#v", entry)
		}
		if _, ok := entry.refs["table:golden.n"]; !ok {
			t.Fatalf("rename must rekey the reference to table:golden.n, got %#v", entry.refs)
		}
		if _, ok := entry.refs["table:golden.t"]; ok {
			t.Fatalf("rename must dissolve the old reference, got %#v", entry.refs)
		}
	})
}

// A failed catalog read propagates instead of seeding absence: the whole
// placement answer stays unvouched, never a fabricated empty registry.
func TestT20CatalogLoadErrorPropagates(t *testing.T) {
	provider := &t10Provider{
		version:      "8.0.11-TiDB-v8.5.0",
		placementErr: errors.New("catalog unreachable"),
		tables:       map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	_, err := AuditSQL(t.Context(), Request{
		SQL:              "ALTER TABLE t PLACEMENT POLICY=p;",
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		MetadataProvider: provider,
	})
	if err == nil {
		t.Fatal("catalog read failure must propagate, got nil error")
	}
}

// The MySQL vendor boundary holds for bindings: a proven bind under TiDB
// publishes, the identical statement under MySQL refuses — pre-state kept.
func TestT20MySQLVendorBoundary(t *testing.T) {
	bound := t20PlacementShape("t")
	bound.PlacementPolicy = "p"
	provider := &t10Provider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"t": bound},
	}
	state, _ := t17Enrich(t, spec.DialectMySQL,
		"ALTER TABLE t PLACEMENT POLICY=DEFAULT; ALTER TABLE t ADD COLUMN c INT;", "golden", t17v8046, provider)
	entry := state.entries[state.keyFor("golden", spec.Table{Name: "t"})]
	if entry == nil || entry.shape == nil || !entry.shape.PlacementPolicyKnown || entry.shape.PlacementPolicy != "p" {
		t.Fatalf("MySQL vendor refusal must keep the exact pre-state, got %#v", entry)
	}
}
