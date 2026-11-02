package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t18R1Tail2Fixture() *t18FirstProvider {
	provider := t18FirstFixture()
	provider.tables["g.t_amount"] = &spec.TableSnapshot{Schema: "g",
		Table: &spec.Table{Schema: "g", Name: "t_amount"}, Exists: true,
		RelationKind: "table", Columns: []spec.Column{{Name: "x", Type: "int"}}}
	provider.tables["g.t_base"] = &spec.TableSnapshot{Schema: "g",
		Table: &spec.Table{Schema: "g", Name: "t_base"}, Exists: true,
		RelationKind: "table", Columns: []spec.Column{{Name: "id", Type: "int"}}}
	for _, name := range []string{"agg_v", "base_v", "joined_v", "checked_v", "joined_bad_v"} {
		provider.tables["g."+name] = &spec.TableSnapshot{Schema: "g",
			Table: &spec.Table{Schema: "g", Name: name}, Exists: false}
	}
	return provider
}

func t18R1Tail2ViewSource(name, leaf string, updatable *bool) *spec.TableSnapshot {
	return &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: name},
		Exists: true, RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
		View: &spec.ViewSnapshot{Security: "definer", Updatable: updatable,
			Definition: &spec.ViewDefinition{SQL: "SELECT id FROM g." + leaf, SourcesKnown: true,
				Sources: []spec.Table{{Schema: "g", Name: leaf}}, ShapeKnown: true,
				Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}}}}}
}

func t18R1Tail2Updatable(t *testing.T, state *batchState, name string) *bool {
	t.Helper()
	entry := state.entries[state.keyFor("g", spec.Table{Schema: "g", Name: name})]
	if entry == nil || entry.state != tablePresent || entry.shape == nil || entry.shape.View == nil {
		t.Fatalf("%s was not published as a view: %#v", name, entry)
	}
	return entry.shape.View.Updatable
}

func TestT18R1Tail2MixedJoinKeepsUpdatableComponent(t *testing.T) {
	provider := t18R1Tail2Fixture()
	state, statements := t18FirstEnrich(t, provider,
		"CREATE VIEW agg_v AS SELECT SUM(x) AS total FROM t_amount;\n"+
			"CREATE VIEW base_v AS SELECT id FROM t_base;\n"+
			"CREATE VIEW joined_v AS SELECT agg_v.total, base_v.id FROM agg_v JOIN base_v ON agg_v.total = base_v.id;\n"+
			"CREATE VIEW checked_v AS SELECT id FROM joined_v WITH CHECK OPTION;")
	if len(statements) != 4 {
		t.Fatalf("statements=%d", len(statements))
	}
	if got := t18R1Tail2Updatable(t, state, "agg_v"); got == nil || *got {
		t.Fatalf("agg_v updatable=%v", got)
	}
	if got := t18R1Tail2Updatable(t, state, "base_v"); got == nil || !*got {
		t.Fatalf("base_v updatable=%v", got)
	}
	if got := t18R1Tail2Updatable(t, state, "joined_v"); got == nil || !*got {
		t.Fatalf("joined_v updatable=%v, want true", got)
	}
	check := spec.AssessViewLifecycle(statements[3])
	if check.CheckInvalid {
		t.Fatalf("check option on a view with an updatable component was refused: %#v", check)
	}
	if got := t18R1Tail2Updatable(t, state, "checked_v"); got == nil || !*got {
		t.Fatalf("checked_v updatable=%v, want true", got)
	}
}

func TestT18R1Tail2SourceCombinationControls(t *testing.T) {
	viewSource := func(name string, updatable *bool) *spec.TableSnapshot {
		return &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: name},
			Exists: true, RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
			View: &spec.ViewSnapshot{Security: "definer", Updatable: updatable,
				Definition: &spec.ViewDefinition{SQL: "SELECT id FROM g.t_base", SourcesKnown: true,
					Sources: []spec.Table{{Schema: "g", Name: "t_base"}}, ShapeKnown: true,
					Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}}}}}
	}
	for _, tc := range []struct {
		name string
		sql  string
		want *bool
		a, b *bool
	}{
		{name: "false_and_unknown", sql: "CREATE VIEW v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id;", want: nil,
			a: t18R1TailBool(false), b: nil},
		{name: "false_and_true", sql: "CREATE VIEW v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id;", want: t18R1TailBool(true),
			a: t18R1TailBool(false), b: t18R1TailBool(true)},
		{name: "false_output_with_true_source", sql: "CREATE VIEW v AS SELECT a.id FROM a JOIN b ON a.id = b.id;", want: t18R1TailBool(true),
			a: t18R1TailBool(false), b: t18R1TailBool(true)},
		{name: "only_false_source_output", sql: "CREATE VIEW v AS SELECT a.id FROM a JOIN b ON a.id = b.id;", want: nil,
			a: t18R1TailBool(false), b: nil},
		{name: "expression_output_only", sql: "CREATE VIEW v AS SELECT a.id + b.id AS x FROM a JOIN b ON a.id = b.id;", want: t18R1TailBool(false),
			a: t18R1TailBool(false), b: t18R1TailBool(true)},
		{name: "all_sources_false", sql: "CREATE VIEW v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id;", want: t18R1TailBool(false),
			a: t18R1TailBool(false), b: t18R1TailBool(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18R1Tail2Fixture()
			provider.tables["g.v"] = t18R1TailAbsent("v")
			provider.tables["g.a"] = viewSource("a", tc.a)
			provider.tables["g.b"] = viewSource("b", tc.b)
			state, statements := t18FirstEnrich(t, provider, tc.sql)
			if len(statements) != 1 {
				t.Fatalf("statements=%d", len(statements))
			}
			got := t18R1Tail2Updatable(t, state, "v")
			if (got == nil) != (tc.want == nil) || got != nil && tc.want != nil && *got != *tc.want {
				t.Fatalf("updatable=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestT18R1Tail2CheckSharesUnprovenUpdatable(t *testing.T) {
	provider := t18R1Tail2Fixture()
	provider.tables["g.v"] = t18R1TailAbsent("v")
	source := func(name string, updatable *bool) *spec.TableSnapshot {
		return &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: name},
			Exists: true, RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
			View: &spec.ViewSnapshot{Security: "definer", Updatable: updatable,
				Definition: &spec.ViewDefinition{SQL: "SELECT id FROM g.t_base", SourcesKnown: true,
					Sources: []spec.Table{{Schema: "g", Name: "t_base"}}, ShapeKnown: true,
					Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}}}}}
	}
	provider.tables["g.a"] = source("a", t18R1TailBool(false))
	provider.tables["g.b"] = source("b", nil)
	state, statements := t18FirstEnrich(t, provider,
		"CREATE VIEW v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id WITH CHECK OPTION;")
	if len(statements) != 1 {
		t.Fatalf("statements=%d", len(statements))
	}
	assessment := spec.AssessViewLifecycle(statements[0])
	if assessment.CheckInvalid || len(assessment.CheckRequired) != 1 || assessment.CheckRequired[0] != "view.updatable" {
		t.Fatalf("check assessment=%#v", assessment)
	}
	entry := state.entries[state.keyFor("g", spec.Table{Name: "v"})]
	if entry != nil && entry.state == tablePresent {
		t.Fatalf("unresolved checked view was published: %#v", entry.shape)
	}
}

func TestT18R1Tail2CatalogUpdatableDominatesParsedSourceShape(t *testing.T) {
	provider := t18R1Tail2Fixture()
	provider.tables["g.v"] = t18R1TailAbsent("v")
	source := t18R1TailSourceView("a", t18R1TailBool(true))
	source.View.Definition.NonMergeable = true
	provider.tables["g.a"] = source
	state, statements := t18FirstEnrich(t, provider, "CREATE VIEW v AS SELECT id FROM a;")
	if len(statements) != 1 {
		t.Fatalf("statements=%d", len(statements))
	}
	if got := t18R1Tail2Updatable(t, state, "v"); got == nil || !*got {
		t.Fatalf("updatable=%v, want catalog-proven true", got)
	}
}

func TestT18R1Tail2MissingOutputDoesNotProveUpdatable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sql   string
		setup func(*t18FirstProvider)
	}{
		{name: "single_source_view", sql: "CREATE VIEW v AS SELECT a.missing FROM a;",
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.a"] = t18R1Tail2ViewSource("a", "t_base", t18R1TailBool(true))
			}},
		{name: "joined_source_view", sql: "CREATE VIEW v AS SELECT a.missing FROM a JOIN b ON a.id = b.id;",
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.a"] = t18R1Tail2ViewSource("a", "t_base", t18R1TailBool(true))
				provider.tables["g.b"] = t18R1Tail2ViewSource("b", "t_base", t18R1TailBool(true))
			}},
		{name: "derived_source", sql: "CREATE VIEW v AS SELECT dt.missing FROM (SELECT id FROM t_base) dt;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18R1Tail2Fixture()
			provider.tables["g.v"] = t18R1TailAbsent("v")
			if tc.setup != nil {
				tc.setup(provider)
			}
			state, statements := t18FirstEnrich(t, provider, tc.sql)
			if len(statements) != 1 {
				t.Fatalf("statements=%d", len(statements))
			}
			assessment := spec.AssessViewLifecycle(statements[0])
			if assessment.Updatable == nil || *assessment.Updatable {
				t.Fatalf("missing output proved updatable: %#v", assessment.Updatable)
			}
			if !assessment.SourceInvalid {
				t.Fatalf("missing output was not marked as a source failure: %#v", assessment)
			}
			entry := state.entries[state.keyFor("g", spec.Table{Name: "v"})]
			if entry != nil && entry.state == tablePresent {
				t.Fatalf("invalid view was published: %#v", entry.shape)
			}
		})
	}
}

func TestT18R1Tail2QuerySourcesAndGlobalReuseControls(t *testing.T) {
	viewOn := func(leaf string) func(*t18FirstProvider) {
		return func(provider *t18FirstProvider) {
			provider.tables["g.a"] = t18R1Tail2ViewSource("a", leaf, t18R1TailBool(true))
		}
	}
	nestedViews := func(provider *t18FirstProvider) {
		provider.tables["g.a"] = t18R1Tail2ViewSource("a", "t", t18R1TailBool(true))
		provider.tables["g.a2"] = t18R1Tail2ViewSource("a2", "a", t18R1TailBool(true))
	}
	for _, tc := range []struct {
		name  string
		sql   string
		want  *bool
		setup func(*t18FirstProvider)
	}{
		{name: "derived_expression_source", sql: "CREATE VIEW v AS SELECT dt.x FROM (SELECT id + 1 AS x FROM t_base) dt;", want: t18R1TailBool(true)},
		{name: "derived_inner_reuse", sql: "CREATE VIEW v AS SELECT dt.id FROM (SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM t)) AS dt;", want: t18R1TailBool(true)},
		{name: "derived_outer_inner_same_leaf", sql: "CREATE VIEW v AS SELECT dt.id FROM (SELECT t.id FROM t) AS dt WHERE EXISTS (SELECT 1 FROM t);", want: t18R1TailBool(true)},
		{name: "cte_expression_source", sql: "CREATE VIEW v AS WITH c AS (SELECT id + 1 AS x FROM t_base) SELECT x FROM c;", want: t18R1TailBool(true)},
		{name: "cte_inner_reuse", sql: "CREATE VIEW v AS WITH c AS (SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM t)) SELECT c.id FROM c;", want: t18R1TailBool(true)},
		{name: "cte_outer_inner_same_leaf", sql: "CREATE VIEW v AS WITH c AS (SELECT t.id FROM t) SELECT c.id FROM c WHERE EXISTS (SELECT 1 FROM t);", want: t18R1TailBool(true)},
		{name: "cte_referenced_again", sql: "CREATE VIEW v AS WITH c AS (SELECT t.id FROM t) SELECT c.id FROM c WHERE EXISTS (SELECT 1 FROM c);", want: t18R1TailBool(true)},
		{name: "with_unused_same", sql: "CREATE VIEW v AS WITH c AS (SELECT 1 FROM t) SELECT t.id FROM t;", want: t18R1TailBool(true)},
		{name: "with_unused_diff", sql: "CREATE VIEW v AS WITH c AS (SELECT 1 FROM u) SELECT t.id FROM t;", want: t18R1TailBool(true)},
		{name: "distinct_nested_table", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM u);", want: t18R1TailBool(true)},
		{name: "view_source_reused", sql: "CREATE VIEW v AS SELECT a.id FROM a WHERE EXISTS (SELECT 1 FROM a);", want: t18R1TailBool(true), setup: viewOn("t")},
		{name: "view_main_base_subquery", sql: "CREATE VIEW v AS SELECT a.id FROM a WHERE EXISTS (SELECT 1 FROM t);", want: t18R1TailBool(true), setup: viewOn("t")},
		{name: "reuse_where", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM t);", want: t18R1TailBool(false)},
		{name: "reuse_select_list", sql: "CREATE VIEW v AS SELECT (SELECT 1 FROM t) AS x, t.id FROM t;", want: t18R1TailBool(false)},
		{name: "reuse_derived_subquery", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM (SELECT 1 FROM t) AS dt);", want: t18R1TailBool(false)},
		{name: "same_table_second_alias", sql: "CREATE VIEW v AS SELECT t.id FROM t AS t WHERE EXISTS (SELECT 1 FROM t AS t2);", want: t18R1TailBool(false)},
		{name: "main_via_cte_subquery", sql: "CREATE VIEW v AS WITH c AS (SELECT t.id FROM t) SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM c);", want: t18R1TailBool(false)},
		{name: "main_via_cte_derived", sql: "CREATE VIEW v AS WITH c AS (SELECT t.id FROM t) SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM (SELECT c.id FROM c) AS dt);", want: t18R1TailBool(false)},
		{name: "main_via_view_subquery", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM a);", want: t18R1TailBool(false), setup: viewOn("t")},
		{name: "main_via_view_derived", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM (SELECT a.id FROM a) AS dt);", want: t18R1TailBool(false), setup: viewOn("t")},
		{name: "main_via_nested_view", sql: "CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM a2);", want: t18R1TailBool(false), setup: nestedViews},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18R1Tail2Fixture()
			provider.tables["g.v"] = t18R1TailAbsent("v")
			if tc.setup != nil {
				tc.setup(provider)
			}
			state, statements := t18FirstEnrich(t, provider, tc.sql)
			if len(statements) != 1 {
				t.Fatalf("statements=%d", len(statements))
			}
			got := t18R1Tail2Updatable(t, state, "v")
			if (got == nil) != (tc.want == nil) || got != nil && tc.want != nil && *got != *tc.want {
				t.Fatalf("updatable=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestT18R1Tail2GlobalReuseUnprovenViewStaysUnknown(t *testing.T) {
	provider := t18R1Tail2Fixture()
	provider.tables["g.v"] = t18R1TailAbsent("v")
	provider.tables["g.a"] = &spec.TableSnapshot{Schema: "g",
		Table: &spec.Table{Schema: "g", Name: "a"}, Exists: true,
		RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
		View: &spec.ViewSnapshot{Security: "definer", Updatable: t18R1TailBool(true)}}
	state, statements := t18FirstEnrich(t, provider,
		"CREATE VIEW v AS SELECT t.id FROM t WHERE EXISTS (SELECT 1 FROM a) WITH CHECK OPTION;")
	if len(statements) != 1 {
		t.Fatalf("statements=%d", len(statements))
	}
	assessment := spec.AssessViewLifecycle(statements[0])
	if assessment.Updatable != nil || assessment.CheckInvalid ||
		!containsT18Fact(assessment.CheckRequired, "view.updatable") ||
		!containsT18Fact(assessment.SourceRequired, "view.definition") {
		t.Fatalf("unproven global reuse assessment=%#v", assessment)
	}
	entry := state.entries[state.keyFor("g", spec.Table{Name: "v"})]
	if entry != nil && entry.state == tablePresent {
		t.Fatalf("view with an unexpanded hidden source was published: %#v", entry.shape)
	}
}

func TestT18R1Tail2ViewSourceColumnStaysFieldForUpdatableFlag(t *testing.T) {
	provider := t18R1Tail2Fixture()
	for _, name := range []string{"mixed_v", "bad_check_v", "expr_v", "next_check_v"} {
		provider.tables["g."+name] = t18R1TailAbsent(name)
	}
	state, statements := t18FirstEnrich(t, provider,
		"CREATE VIEW mixed_v AS SELECT id, id + 1 AS x FROM t_base;\n"+
			"CREATE VIEW bad_check_v AS SELECT x FROM mixed_v WITH CHECK OPTION;\n"+
			"CREATE VIEW expr_v AS SELECT x FROM mixed_v;\n"+
			"CREATE VIEW next_check_v AS SELECT x FROM expr_v WITH CHECK OPTION;")
	if len(statements) != 4 {
		t.Fatalf("statements=%d", len(statements))
	}
	for _, name := range []string{"mixed_v", "bad_check_v", "expr_v", "next_check_v"} {
		if got := t18R1Tail2Updatable(t, state, name); got == nil || !*got {
			t.Fatalf("%s updatable=%v, want MySQL 8.0.46 true", name, got)
		}
	}
	for i := 1; i < len(statements); i++ {
		if assessment := spec.AssessViewLifecycle(statements[i]); assessment.CheckInvalid {
			t.Fatalf("statement %d check option was refused contrary to MySQL 8.0.46: %#v", i, assessment)
		}
	}
}
