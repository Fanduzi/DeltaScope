package audit

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t18R1SchemaFixture() *t18FirstProvider {
	provider := t18FirstFixture()
	provider.tables["src.t"] = &spec.TableSnapshot{Schema: "src",
		Table: &spec.Table{Schema: "src", Name: "t"}, Exists: true, RelationKind: "table",
		Columns: []spec.Column{{Name: "id", Type: "int"}}}
	return provider
}

func TestT18R1CreateViewConsumesDroppedSchema(t *testing.T) {
	provider := t18R1SchemaFixture()
	state, statements := t18FirstEnrich(t, provider,
		"DROP DATABASE IF EXISTS g;\n"+
			"CREATE VIEW g.v AS SELECT id FROM src.t;\n"+
			"CREATE VIEW g.v AS SELECT id FROM src.t;")
	if len(statements) != 3 {
		t.Fatalf("statements=%d", len(statements))
	}
	for i := 1; i <= 2; i++ {
		assessment := spec.AssessViewLifecycle(statements[i])
		if !assessment.TargetInvalid {
			t.Fatalf("schema-absent create was not refused: statement=%d assessment=%#v", i, assessment)
		}
	}
	entry := state.entries[batchTableKey{dialect: spec.DialectMySQL, schema: "g", table: "v"}]
	if entry == nil || entry.state != tableAbsent {
		t.Fatalf("dropped schema fabricated a view target: %#v", entry)
	}
	source := state.entries[batchTableKey{dialect: spec.DialectMySQL, schema: "src", table: "t"}]
	if source == nil || source.state != tablePresent {
		t.Fatalf("unrelated source was damaged: %#v", source)
	}
}

func TestT18R1CreateViewLeavesNeverTouchedSchemaUnknown(t *testing.T) {
	provider := t18R1SchemaFixture()
	provider.tables["g.v"] = &spec.TableSnapshot{Schema: "g", Exists: false}
	_, statements := t18FirstEnrich(t, provider, "CREATE VIEW v AS SELECT id FROM src.t;")
	assessment := spec.AssessViewLifecycle(statements[0])
	if assessment.TargetInvalid {
		t.Fatalf("never-touched schema was treated as absent: %#v", assessment)
	}
}

func TestT18R1CheckOptionStaticPremise(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"computed_only", "CREATE VIEW calc_v AS SELECT id + 1 AS x FROM t WITH CHECK OPTION;"},
		{"outer_join", "CREATE VIEW join_v AS SELECT t.id, u.uid FROM t LEFT JOIN u ON t.id = u.uid WITH LOCAL CHECK OPTION;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18FirstFixture()
			provider.tables["g.u"].Columns = []spec.Column{{Name: "uid", Type: "int"}}
			_, statements := t18FirstEnrich(t, provider, tc.sql)
			assessment := spec.AssessViewLifecycle(statements[0])
			if !assessment.CheckInvalid || len(assessment.CheckRequired) != 0 {
				t.Fatalf("non-updatable CHECK OPTION was not refused: %#v", assessment)
			}
		})
	}
}

func TestT18R1InheritedAlgorithmFallback(t *testing.T) {
	for _, clause := range []string{"", "ALGORITHM=UNDEFINED "} {
		t.Run("alter_"+clause, func(t *testing.T) {
			provider := t18FirstFixture()
			provider.tables["g.mv"] = &spec.TableSnapshot{Schema: "g", Exists: false}
			_, statements := t18FirstEnrich(t, provider,
				"CREATE ALGORITHM=MERGE VIEW mv AS SELECT id FROM t;\n"+
					"ALTER "+clause+"VIEW mv AS SELECT COUNT(*) AS n FROM t;\n"+
					"CREATE VIEW next_v AS SELECT n FROM mv;")
			assessment := spec.AssessViewLifecycle(statements[1])
			if !assessment.AlgorithmWarn {
				t.Fatalf("inherited merge fallback was not reviewed: %#v", assessment)
			}
			view := statements[2].Metadata.View.Sources[0].Snapshot.View
			if view == nil || view.Algorithm != "undefined" || view.Definition == nil ||
				!reflect.DeepEqual(view.Definition.Sources, []spec.Table{{Schema: "g", Name: "t"}}) {
				t.Fatalf("inherited algorithm stayed publishable as merge: %#v", view)
			}
		})
	}
}

func TestT18R1CorrelatedSubqueryNearestScope(t *testing.T) {
	provider := t18FirstFixture()
	provider.tables["g.a"] = &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: "a"},
		Exists: true, RelationKind: "table", Columns: []spec.Column{{Name: "id", Type: "int"}}}
	provider.tables["g.b"] = &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: "b"},
		Exists: true, RelationKind: "table", Columns: []spec.Column{{Name: "id", Type: "int"}}}
	_, statements := t18FirstEnrich(t, provider,
		"CREATE VIEW cv AS SELECT a.id FROM a WHERE EXISTS (\n"+
			"  SELECT 1 FROM b WHERE EXISTS (\n"+
			"    SELECT 1 WHERE id > 0\n"+
			"  )\n"+
			");")
	assessment := spec.AssessViewLifecycle(statements[0])
	if assessment.SourceInvalid || len(assessment.SourceRequired) != 0 {
		t.Fatalf("nearest outer scope was flattened into ambiguity: %#v", assessment)
	}
}

func TestT18R1CorrelatedSubqueryScopeControls(t *testing.T) {
	newProvider := func() *t18FirstProvider {
		provider := t18FirstFixture()
		provider.tables["g.a"] = &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: "a"},
			Exists: true, RelationKind: "table", Columns: []spec.Column{{Name: "id", Type: "int"}}}
		provider.tables["g.b"] = &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: "b"},
			Exists: true, RelationKind: "table", Columns: []spec.Column{{Name: "bid", Type: "int"}}}
		return provider
	}
	t.Run("outer_fallback", func(t *testing.T) {
		_, statements := t18FirstEnrich(t, newProvider(),
			"CREATE VIEW cv AS SELECT a.id FROM a WHERE EXISTS (\n"+
				"  SELECT 1 FROM b WHERE EXISTS (\n"+
				"    SELECT 1 WHERE id > 0\n"+
				"  )\n"+
				");")
		assessment := spec.AssessViewLifecycle(statements[0])
		if assessment.SourceInvalid || len(assessment.SourceRequired) != 0 {
			t.Fatalf("outer fallback was rejected: %#v", assessment)
		}
	})
	t.Run("same_scope_ambiguity", func(t *testing.T) {
		provider := newProvider()
		provider.tables["g.b"].Columns = []spec.Column{{Name: "id", Type: "int"}}
		_, statements := t18FirstEnrich(t, provider,
			"CREATE VIEW cv AS SELECT a.id FROM a WHERE EXISTS (\n"+
				"  SELECT 1 FROM a, b WHERE id > 0\n"+
				");")
		assessment := spec.AssessViewLifecycle(statements[0])
		if !assessment.SourceInvalid {
			t.Fatalf("same-scope ambiguity was accepted: %#v", assessment)
		}
	})
}
