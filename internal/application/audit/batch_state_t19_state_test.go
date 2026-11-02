package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t19SuccessorSnapshot(t *testing.T, provider *t10Provider, sql string) *spec.TableSnapshot {
	t.Helper()
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}, provider)
	if len(statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(statements))
	}
	metadata := statements[1].Metadata
	if metadata == nil || len(metadata.TargetTables) != 1 {
		t.Fatalf("second target bindings = %#v", metadata)
	}
	return metadata.TargetTables[0].Snapshot
}

func TestT19RemoveTTLPublishesNoTTLOnPartialShape(t *testing.T) {
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables: map[string]*spec.TableSnapshot{
			"t": {
				Exists:              true,
				Table:               &spec.Table{Name: "t"},
				Columns:             nil,
				PrimaryKeyUnknown:   true,
				IndexesUnknown:      true,
				ConstraintsUnknown:  true,
				ReferencedByUnknown: true,
			},
		},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t REMOVE TTL; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("REMOVE TTL successor = %#v, want present partial projection", snapshot)
	}
	if snapshot.Columns != nil || !snapshot.PrimaryKeyUnknown || !snapshot.IndexesUnknown || !snapshot.ConstraintsUnknown {
		t.Fatalf("REMOVE TTL must not fabricate member collections: %#v", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("REMOVE TTL must publish proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
}

func TestT19TTLDefinitionUnprovenWhenColumnTailUnknown(t *testing.T) {
	table := t19TTLShape("t")
	table.ColumnsTailUnknown = true
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = hidden_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("unknown column tail must leave TTL unobserved, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
}

func TestT19TTLDefinitionRefusedWhenInboundFKReferenced(t *testing.T) {
	table := t19TTLShape("t")
	table.ReferencedBy = []spec.Table{{Schema: "golden", Name: "child"}}
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("inbound FK refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
	if snapshot.FindColumn("created_at") == nil {
		t.Fatalf("inbound FK refusal must preserve the original columns: %#v", snapshot.Columns)
	}
}

func TestT19TTLDefinitionRefusedOnGlobalTemporaryTable(t *testing.T) {
	table := t19TTLShape("t")
	table.GlobalTemporary = true
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("global-temporary refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
	if !snapshot.GlobalTemporary {
		t.Fatal("global-temporary fact must survive the refused TTL transition")
	}
}

func TestT19TTLDefinitionRefusedOnCommonHandleFloatPrimaryKey(t *testing.T) {
	table := t19TTLShape("t")
	table.Columns = append(table.Columns, t10r2Col("amount", "float", false, 0))
	table.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"amount"}}
	table.PrimaryKeyClustered = true
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("common-handle FLOAT primary key refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
	if snapshot.PrimaryKey == nil || !snapshot.PrimaryKeyClustered {
		t.Fatalf("common-handle fact must survive the refused TTL transition: %#v", snapshot.PrimaryKey)
	}
}

func TestT19TTLDefinitionRefusedOnInvalidInterval(t *testing.T) {
	table := t19TTLShape("t")
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL 'abc' DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("invalid interval refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
}

func TestT19TTLDefinitionUnprovenOnUnsupportedIntervalExpression(t *testing.T) {
	table := t19TTLShape("t")
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL NULL DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("unsupported interval expression must leave TTL unobserved, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
}

func TestT19TTLDefinitionRefusedOnBadTimeColumn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		alter  string
		mutate func(*spec.TableSnapshot)
	}{
		{name: "missing", alter: "ALTER TABLE t TTL = missing_at + INTERVAL 1 DAY"},
		{name: "non temporal", alter: "ALTER TABLE t TTL = label + INTERVAL 1 DAY", mutate: func(table *spec.TableSnapshot) {
			table.Columns = append(table.Columns, t10r2Col("label", "varchar(64)", false, 0))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := t19TTLShape("t")
			if tc.mutate != nil {
				tc.mutate(table)
			}
			provider := &t10Provider{
				version: "8.0.11-TiDB-v8.5.0",
				tables:  map[string]*spec.TableSnapshot{"t": table},
			}
			snapshot := t19SuccessorSnapshot(t, provider, tc.alter+"; ALTER TABLE t ADD COLUMN id INT;")
			if snapshot == nil || !snapshot.Exists {
				t.Fatalf("TTL successor = %#v, want present table", snapshot)
			}
			if !snapshot.TTLKnown || snapshot.TTL != nil {
				t.Fatalf("bad time column refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
			}
		})
	}
}

func TestT19TTLDefinitionRefusedOnCommonHandleDoublePrimaryKey(t *testing.T) {
	table := t19TTLShape("t")
	table.Columns = append(table.Columns, t10r2Col("amount", "double", false, 0))
	table.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"amount"}}
	table.PrimaryKeyClustered = true
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t TTL = created_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("TTL successor = %#v, want present table", snapshot)
	}
	if !snapshot.TTLKnown || snapshot.TTL != nil {
		t.Fatalf("common-handle DOUBLE primary key refusal must preserve proven no TTL, got TTLKnown=%v TTL=%#v", snapshot.TTLKnown, snapshot.TTL)
	}
}

func TestT19TTLDefinitionUnprovenWhenPrerequisitesUnvouched(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*spec.TableSnapshot)
	}{
		{name: "inbound edge set", mutate: func(table *spec.TableSnapshot) { table.ReferencedByUnknown = true }},
		{name: "clustered answer", mutate: func(table *spec.TableSnapshot) { table.PrimaryKeyClusteredUnknown = true }},
		{name: "handle answer", mutate: func(table *spec.TableSnapshot) { table.PrimaryKeyHandleUnknown = true }},
		{name: "common-handle primary key", mutate: func(table *spec.TableSnapshot) {
			table.PrimaryKeyClustered = true
			table.PrimaryKeyUnknown = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := t19TTLShape("t")
			tc.mutate(table)
			provider := &t10Provider{
				version: "8.0.11-TiDB-v8.5.0",
				tables:  map[string]*spec.TableSnapshot{"t": table},
			}
			snapshot := t19SuccessorSnapshot(t, provider,
				"ALTER TABLE t TTL = created_at + INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
			if snapshot == nil || !snapshot.Exists {
				t.Fatalf("TTL successor = %#v, want present table", snapshot)
			}
			if snapshot.TTLKnown || snapshot.TTL != nil {
				t.Fatalf("%s must leave TTL unobserved, got TTLKnown=%v TTL=%#v", tc.name, snapshot.TTLKnown, snapshot.TTL)
			}
		})
	}
}

func TestT19CreateTTLRespectsCheckTTLInfoValid(t *testing.T) {
	for _, tc := range []struct {
		name       string
		create     string
		wantAbsent bool
		wantTTL    bool
	}{
		{
			name:    "plain create publishes enabled ttl",
			create:  "CREATE TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY",
			wantTTL: true,
		},
		{
			name:       "local temporary refuses",
			create:     "CREATE TEMPORARY TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY",
			wantAbsent: true,
		},
		{
			name:       "global temporary refuses",
			create:     "CREATE GLOBAL TEMPORARY TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY ON COMMIT DELETE ROWS",
			wantAbsent: true,
		},
		{
			name:       "non temporal column refuses",
			create:     "CREATE TABLE ttl_ok (label VARCHAR(32)) TTL=label+INTERVAL 1 DAY",
			wantAbsent: true,
		},
		{
			name:       "invalid interval refuses",
			create:     "CREATE TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 'abc' DAY",
			wantAbsent: true,
		},
		{
			name:       "clustered float primary key refuses",
			create:     "CREATE TABLE ttl_ok (amount FLOAT, created_at DATETIME, PRIMARY KEY(amount) CLUSTERED) TTL=created_at+INTERVAL 1 DAY",
			wantAbsent: true,
		},
		{
			name:       "unprovable clustered mode stays unvouched",
			create:     "CREATE TABLE ttl_ok (amount FLOAT, created_at DATETIME, PRIMARY KEY(amount)) TTL=created_at+INTERVAL 1 DAY",
			wantAbsent: false,
			wantTTL:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
			snapshot := t19SuccessorSnapshot(t, provider, tc.create+"; ALTER TABLE ttl_ok ADD COLUMN id INT;")
			if tc.wantAbsent {
				if snapshot == nil || snapshot.Exists {
					t.Fatalf("refused CREATE successor = %#v, want provably absent table", snapshot)
				}
				return
			}
			if !tc.wantTTL {
				if snapshot != nil {
					t.Fatalf("unproven CREATE successor = %#v, want unvouched binding", snapshot)
				}
				return
			}
			if snapshot == nil || !snapshot.Exists || !snapshot.TTLKnown || snapshot.TTL == nil || snapshot.TTL.Enabled == nil || !*snapshot.TTL.Enabled {
				t.Fatalf("CREATE TTL successor = %#v, want proven enabled TTL", snapshot)
			}
		})
	}
}

// TiDB checkTableForeignKey refuses any edge whose parent carries TTL —
// on CREATE declarations and ALTER adds alike. A parent whose TTL fact the
// catalog never observed leaves the edge unproven rather than landing it.
func TestT19ForeignKeyIntoTTLParent(t *testing.T) {
	enabled := true
	ttlParent := func() *spec.TableSnapshot {
		parent := t19TTLShape("parent")
		parent.Columns = append(parent.Columns, t10r2Col("id", "int", true, 0))
		parent.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}
		parent.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "1", IntervalUnit: "day", Enabled: &enabled}
		return parent
	}
	t.Run("create referencing ttl parent refuses", func(t *testing.T) {
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"parent": ttlParent()}}
		snapshot := t19SuccessorSnapshot(t, provider,
			"CREATE TABLE child (c INT, FOREIGN KEY (c) REFERENCES parent(id)); ALTER TABLE child ADD COLUMN id INT;")
		if snapshot != nil && snapshot.Exists {
			t.Fatalf("FK into a TTL parent must refuse the create, got %#v", snapshot)
		}
	})
	t.Run("alter add referencing ttl parent refuses", func(t *testing.T) {
		child := t19TTLShape("child")
		child.Columns = append(child.Columns, t10r2Col("c", "int", false, 0))
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"parent": ttlParent(), "child": child}}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id); ALTER TABLE child ADD COLUMN id INT;")
		if snapshot == nil || !snapshot.Exists {
			t.Fatalf("refused add keeps the child present, got %#v", snapshot)
		}
		if len(snapshot.Constraints) != 0 {
			t.Fatalf("TTL parent refusal publishes no member, got %#v", snapshot.Constraints)
		}
	})
	t.Run("unvouched parent ttl leaves add unproven", func(t *testing.T) {
		parent := ttlParent()
		parent.TTL = nil
		parent.TTLKnown = false
		child := t19TTLShape("child")
		child.Columns = append(child.Columns, t10r2Col("c", "int", false, 0))
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"parent": parent, "child": child}}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id); ALTER TABLE child ADD COLUMN id INT;")
		if snapshot != nil && len(snapshot.Constraints) != 0 {
			t.Fatalf("unvouched parent TTL publishes no member, got %#v", snapshot.Constraints)
		}
	})
	t.Run("self referencing create ttl refuses", func(t *testing.T) {
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
		snapshot := t19SuccessorSnapshot(t, provider,
			"CREATE TABLE t (id INT PRIMARY KEY, parent_id INT, created_at DATETIME, FOREIGN KEY (parent_id) REFERENCES t(id)) TTL=created_at+INTERVAL 1 DAY; ALTER TABLE t ADD COLUMN id INT;")
		if snapshot != nil && snapshot.Exists {
			t.Fatalf("self-referencing CREATE TTL must refuse, got %#v", snapshot)
		}
	})
}

// LIKE + TTL is unparseable in TiDB's grammar (options never follow LIKE),
// and CTAS sits outside the supported statement surface — both boundaries
// keep the destination untouched rather than fabricating a TTL-bearing
// shape. The shared create gate still carries LIKE/CTAS branches so the
// adjudication is already correct if either form ever becomes reachable.
func TestT19CreateLikeAndSelectTTL(t *testing.T) {
	t.Run("like ttl is a parse boundary", func(t *testing.T) {
		parsed, err := Parse(context.Background(),
			"CREATE TABLE dst LIKE src TTL=created_at+INTERVAL 1 DAY", spec.DialectTiDB)
		if err == nil && len(parsed.failures) == 0 {
			t.Fatalf("LIKE + TTL parsed cleanly — the grammar boundary moved; revisit the create gate")
		}
	})
	t.Run("ctas ttl stays an unsupported boundary", func(t *testing.T) {
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
		snapshot := t19SuccessorSnapshot(t, provider,
			"CREATE TABLE dst (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY AS SELECT 1; ALTER TABLE dst ADD COLUMN id INT;")
		if snapshot != nil && snapshot.Exists {
			t.Fatalf("unsupported CTAS TTL must not publish a destination, got %#v", snapshot)
		}
	})
}

func TestT19SuccessorSnapshotDoesNotAliasTTLEnabled(t *testing.T) {
	t.Parallel()
	enabled := false
	table := t19TTLShape("t")
	table.TTL = &spec.TTLConfig{TimeColumn: "created_at", Enabled: &enabled}
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": table},
	}
	snapshot := t19SuccessorSnapshot(t, provider,
		"ALTER TABLE t ADD COLUMN id INT; ALTER TABLE t ADD COLUMN other INT;")
	if snapshot == nil || snapshot.TTL == nil || snapshot.TTL.Enabled == nil {
		t.Fatalf("successor TTL = %#v, want enabled member", snapshot)
	}
	if snapshot.TTL.Enabled == table.TTL.Enabled {
		t.Fatalf("successor aliases provider Enabled pointer: %p", snapshot.TTL.Enabled)
	}
	*snapshot.TTL.Enabled = true
	if *table.TTL.Enabled {
		t.Fatal("successor mutation leaked into provider snapshot")
	}
}

func TestT19ColumnIdentityMaintainsTTLBinding(t *testing.T) {
	t.Parallel()
	t.Run("rename rewrites bound time column", func(t *testing.T) {
		t.Parallel()
		table := t19TTLShape("t")
		table.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "30", IntervalUnit: "day"}
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables:  map[string]*spec.TableSnapshot{"t": table},
		}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE t RENAME COLUMN created_at TO expires_at; ALTER TABLE t ADD COLUMN id INT;")
		if snapshot == nil || snapshot.TTL == nil || snapshot.TTL.TimeColumn != "expires_at" {
			t.Fatalf("rename successor TTL = %#v, want bound expires_at", snapshot)
		}
	})
	t.Run("change keeps temporal binding", func(t *testing.T) {
		t.Parallel()
		table := t19TTLShape("t")
		table.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "30", IntervalUnit: "day"}
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables:  map[string]*spec.TableSnapshot{"t": table},
		}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE t CHANGE COLUMN created_at expires_at TIMESTAMP; ALTER TABLE t ADD COLUMN id INT;")
		if snapshot == nil || snapshot.TTL == nil || snapshot.TTL.TimeColumn != "expires_at" {
			t.Fatalf("change successor TTL = %#v, want bound expires_at", snapshot)
		}
	})
	t.Run("modify non temporal binding withholds", func(t *testing.T) {
		t.Parallel()
		table := t19TTLShape("t")
		table.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "30", IntervalUnit: "day"}
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables:  map[string]*spec.TableSnapshot{"t": table},
		}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE t MODIFY COLUMN created_at VARCHAR(32); ALTER TABLE t ADD COLUMN id INT;")
		if snapshot != nil {
			t.Fatalf("non-temporal modify must withhold successor, got %#v", snapshot)
		}
	})
	t.Run("drop bound time column keeps refusal pre-state", func(t *testing.T) {
		t.Parallel()
		table := t19TTLShape("t")
		table.Columns = append(table.Columns, t10r2Col("other", "int", false, 0))
		table.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "30", IntervalUnit: "day"}
		provider := &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables:  map[string]*spec.TableSnapshot{"t": table},
		}
		snapshot := t19SuccessorSnapshot(t, provider,
			"ALTER TABLE t DROP COLUMN created_at; ALTER TABLE t ADD COLUMN id INT;")
		if snapshot == nil || snapshot.TTL == nil || snapshot.TTL.TimeColumn != "created_at" || snapshot.FindColumn("created_at") == nil {
			t.Fatalf("TTL-bound drop must keep exact pre-state, got %#v", snapshot)
		}
	})
}
