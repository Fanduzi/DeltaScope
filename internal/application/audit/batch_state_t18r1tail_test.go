package audit

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t18R1TailBool(v bool) *bool { return &v }

func t18R1TailAbsent(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: name}, Exists: false}
}

func t18R1TailSourceView(name string, updatable *bool) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Schema: "g", Table: &spec.Table{Schema: "g", Name: name}, Exists: true,
		RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
		View: &spec.ViewSnapshot{
			Security:  "definer",
			Updatable: updatable,
			Definition: &spec.ViewDefinition{SQL: "SELECT id FROM g.u", SourcesKnown: true, ShapeKnown: true,
				Sources:  []spec.Table{{Schema: "g", Name: "u"}},
				Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}}},
		},
	}
}

func TestT18R1TailTemptableViewRejectsDependentCheckOption(t *testing.T) {
	provider := t18FirstFixture()
	provider.tables["g.v"] = t18R1TailAbsent("v")
	provider.tables["g.w"] = t18R1TailAbsent("w")
	state, statements := t18FirstEnrich(t, provider,
		"CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;\n"+
			"CREATE VIEW w AS SELECT id FROM v WITH CHECK OPTION;")
	if len(statements) != 2 {
		t.Fatalf("statements=%d", len(statements))
	}
	first := spec.AssessViewLifecycle(statements[0])
	if first.TargetInvalid || first.SourceInvalid || first.CheckInvalid || first.AlgorithmFinal != "temptable" {
		t.Fatalf("temptable create was not accepted: %#v", first)
	}
	source := statements[1].Metadata.View.Sources[0].Snapshot
	if source == nil || !source.Exists || source.RelationKind != "view" || source.View == nil ||
		source.View.Algorithm != "temptable" || source.View.Updatable == nil || *source.View.Updatable {
		t.Fatalf("temptable view was not published as proven non-updatable: %#v", source)
	}
	second := spec.AssessViewLifecycle(statements[1])
	if !second.CheckInvalid {
		t.Fatalf("check option on a temptable source was not refused: %#v", second)
	}
	entry := state.entries[state.keyFor("g", spec.Table{Name: "w"})]
	if entry != nil && entry.state == tablePresent {
		t.Fatalf("refused dependent view was published: %#v", entry.shape)
	}
}

func TestT18R1TailProviderSourceNonUpdatableRejectsCheck(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*spec.ViewSnapshot)
	}{
		{name: "temptable", setup: func(view *spec.ViewSnapshot) { view.Algorithm = "temptable" }},
		{name: "static", setup: func(view *spec.ViewSnapshot) { view.Definition.NonUpdatable = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18FirstFixture()
			provider.tables["g.v"] = t18R1TailAbsent("v")
			source := t18R1TailSourceView("a", nil)
			tc.setup(source.View)
			provider.tables["g.a"] = source
			state, statements := t18FirstEnrich(t, provider,
				"CREATE VIEW v AS SELECT id FROM a WITH CHECK OPTION;")
			if len(statements) != 1 {
				t.Fatalf("statements=%d", len(statements))
			}
			assessment := spec.AssessViewLifecycle(statements[0])
			if !assessment.CheckInvalid {
				t.Fatalf("check option on a proven non-updatable source was not refused: %#v", assessment)
			}
			entry := state.entries[state.keyFor("g", spec.Table{Name: "v"})]
			if entry != nil && entry.state == tablePresent {
				t.Fatalf("refused dependent view was published: %#v", entry.shape)
			}
		})
	}
}

func TestT18R1TailViewUpdatableProofControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sql   string
		want  *bool
		setup func(*t18FirstProvider)
	}{
		{
			name: "direct_create", sql: "CREATE VIEW v AS SELECT id FROM t;", want: t18R1TailBool(true),
			setup: func(provider *t18FirstProvider) { provider.tables["g.v"] = t18R1TailAbsent("v") },
		},
		{
			name: "explicit_temptable", sql: "CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;", want: t18R1TailBool(false),
			setup: func(provider *t18FirstProvider) { provider.tables["g.v"] = t18R1TailAbsent("v") },
		},
		{
			name: "inherited_temptable", sql: "ALTER VIEW v AS SELECT id FROM t;", want: t18R1TailBool(false),
			setup: func(provider *t18FirstProvider) { provider.tables["g.v"].View.Algorithm = "temptable" },
		},
		{
			name: "inherited_unknown", sql: "ALTER VIEW v AS SELECT id FROM t;", want: nil,
			setup: func(provider *t18FirstProvider) { provider.tables["g.v"].View.Algorithm = "" },
		},
		{
			name: "static_false_beats_unknown_algorithm", sql: "ALTER VIEW v AS SELECT COUNT(*) AS n FROM t;", want: t18R1TailBool(false),
			setup: func(provider *t18FirstProvider) { provider.tables["g.v"].View.Algorithm = "" },
		},
		{
			name: "source_static_false_without_updatable_fact", sql: "CREATE VIEW v AS SELECT id FROM a;", want: t18R1TailBool(false),
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.v"] = t18R1TailAbsent("v")
				source := t18R1TailSourceView("a", nil)
				source.View.Definition.NonUpdatable = true
				provider.tables["g.a"] = source
			},
		},
		{
			name: "source_unknown", sql: "CREATE VIEW v AS SELECT id FROM a;", want: nil,
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.v"] = t18R1TailAbsent("v")
				provider.tables["g.a"] = t18R1TailSourceView("a", nil)
			},
		},
		{
			name: "source_false", sql: "CREATE VIEW v AS SELECT id FROM a;", want: t18R1TailBool(false),
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.v"] = t18R1TailAbsent("v")
				provider.tables["g.a"] = t18R1TailSourceView("a", t18R1TailBool(false))
			},
		},
		{
			name: "source_false_with_unrelated_unknown_stays_unknown", sql: "CREATE VIEW v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id;", want: nil,
			setup: func(provider *t18FirstProvider) {
				provider.tables["g.v"] = t18R1TailAbsent("v")
				provider.tables["g.a"] = t18R1TailSourceView("a", t18R1TailBool(false))
				provider.tables["g.b"] = t18R1TailSourceView("b", nil)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18FirstFixture()
			tc.setup(provider)
			state, statements := t18FirstEnrich(t, provider, tc.sql)
			if len(statements) != 1 {
				t.Fatalf("statements=%d", len(statements))
			}
			entry := state.entries[state.keyFor("g", spec.Table{Name: "v"})]
			if entry == nil || entry.state != tablePresent || entry.shape == nil || entry.shape.View == nil {
				t.Fatalf("view was not published: %#v", entry)
			}
			got := entry.shape.View.Updatable
			if (got == nil) != (tc.want == nil) || got != nil && tc.want != nil && *got != *tc.want {
				t.Fatalf("updatable=%v want=%v view=%#v", got, tc.want, entry.shape.View)
			}
		})
	}
}

func TestT18R1TailUnresolvedAlterMergesAlgorithmBranches(t *testing.T) {
	for _, tc := range []struct {
		name, sql, source, priorAlgorithm, wantAlgorithm string
		pending                                          bool
	}{
		{name: "omitted_algorithm", sql: "ALTER VIEW v AS SELECT COUNT(*) AS n FROM u;", source: "unknown", priorAlgorithm: "merge", wantAlgorithm: "", pending: true},
		{name: "explicit_undefined", sql: "ALTER ALGORITHM=UNDEFINED VIEW v AS SELECT COUNT(*) AS n FROM u;", source: "unknown", priorAlgorithm: "merge", wantAlgorithm: "", pending: true},
		{name: "same_success_and_failure_value", sql: "ALTER VIEW v AS SELECT id FROM u;", source: "unknown", priorAlgorithm: "merge", wantAlgorithm: "merge", pending: true},
		{name: "prior_unknown", sql: "ALTER VIEW v AS SELECT COUNT(*) AS n FROM u;", source: "unknown", priorAlgorithm: "", wantAlgorithm: "", pending: true},
		{name: "proven_refusal_keeps_prior", sql: "ALTER VIEW v AS SELECT COUNT(*) AS n FROM u;", source: "missing", priorAlgorithm: "merge", wantAlgorithm: "merge"},
		{name: "proven_success_publishes_final", sql: "ALTER VIEW v AS SELECT COUNT(*) AS n FROM u;", source: "present", priorAlgorithm: "merge", wantAlgorithm: "undefined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18FirstFixture()
			prior := provider.tables["g.v"]
			prior.View.Algorithm = tc.priorAlgorithm
			switch tc.source {
			case "unknown":
				provider.tables["g.u"] = nil
			case "missing":
				provider.tables["g.u"] = &spec.TableSnapshot{Schema: "g", Exists: false}
			}
			before := cloneTableSnapshot(prior)
			_, statements := t18FirstEnrich(t, provider, tc.sql+"\nCREATE VIEW next_v AS SELECT id FROM v;")
			if len(statements) != 2 {
				t.Fatalf("statements=%d", len(statements))
			}
			assessment := spec.AssessViewLifecycle(statements[0])
			projected := statements[1].Metadata.View.Sources[0].Snapshot
			if projected == nil || !projected.Exists || projected.RelationKind != "view" || projected.View == nil {
				t.Fatalf("missing projected view: %#v", projected)
			}
			if projected.View.Algorithm != tc.wantAlgorithm {
				t.Fatalf("algorithm=%q want=%q assessment=%#v", projected.View.Algorithm, tc.wantAlgorithm, assessment)
			}
			if tc.pending {
				if projected.View.Definition != nil || projected.View.Updatable != nil || projected.Columns != nil ||
					projected.View.Security != "definer" || projected.View.DefinerPrincipal != "private_test_principal" ||
					!containsT18Fact(assessment.SourceRequired, "table.exists") {
					t.Fatalf("pending alter did not withhold only unresolved facts: %#v assessment=%#v", projected, assessment)
				}
			} else if tc.source == "missing" {
				if !assessment.SourceInvalid || projected.View.Definition == nil {
					t.Fatalf("proven refusal did not retain the prior view: %#v", projected)
				}
			} else if projected.View.Definition == nil {
				t.Fatalf("proven success did not publish a definition: %#v", projected)
			}
			if !reflect.DeepEqual(prior, before) {
				t.Fatalf("provider snapshot mutated: before=%#v after=%#v", before, prior)
			}
		})
	}
}
