package audit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t18FirstProvider struct {
	tables map[string]*spec.TableSnapshot
}

func (p *t18FirstProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.46"}, nil
}

func (p *t18FirstProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return p.tables[schema+"."+table], nil
}

func t18FirstFixture() *t18FirstProvider {
	table := func(name string) *spec.TableSnapshot {
		return &spec.TableSnapshot{Schema: "g", Table: &spec.Table{Schema: "g", Name: name}, Exists: true,
			RelationKind: "table", Columns: []spec.Column{{Name: "id", Type: "int"}}}
	}
	return &t18FirstProvider{tables: map[string]*spec.TableSnapshot{
		"g.t": table("t"),
		"g.u": table("u"),
		"g.v": {Schema: "g", Table: &spec.Table{Schema: "g", Name: "v"}, Exists: true,
			RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
			View: &spec.ViewSnapshot{Security: "definer", DefinerPrincipal: "private_test_principal",
				Definition: &spec.ViewDefinition{SQL: "SELECT id FROM g.u", SourcesKnown: true,
					Sources: []spec.Table{{Schema: "g", Name: "u"}}, ShapeKnown: true,
					Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}}}}},
		"g.next_v": {Schema: "g", Table: &spec.Table{Schema: "g", Name: "next_v"}, Exists: false},
	}}
}

func t18FirstEnrich(t *testing.T, provider *t18FirstProvider, sql string) (*batchState, []spec.Statement) {
	t.Helper()
	ctx := context.Background()
	parsed, err := Parse(ctx, sql, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	extracted, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	state := newBatchState(spec.DialectMySQL, "g", provider)
	state.resolvedVersion = spec.ParseObservedVersion("8.0.46")
	facts := &spec.InstanceFacts{Version: "8.0.46"}
	state.instanceFacts = facts
	enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "g", Provider: provider},
		extracted, nil, facts, auditLimits{orderedStatements: len(extracted)})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return state, enriched
}

func TestT18FirstOrderedProjection(t *testing.T) {
	provider := t18FirstFixture()
	original := cloneTableSnapshot(provider.tables["g.v"])
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;")
	if len(statements) != 2 || statements[0].DDL.Operation != spec.DDLOperationAlterView ||
		statements[1].DDL.Operation != spec.DDLOperationCreateView {
		t.Fatalf("statements=%#v", statements)
	}
	first := statements[0].Metadata.View
	second := statements[1].Metadata.View
	if first == nil || second == nil || first.Target == nil || first.Target.View == nil ||
		len(first.Sources) != 1 || first.Sources[0].Table != (spec.Table{Schema: "g", Name: "t"}) ||
		len(second.Sources) != 1 || second.Sources[0].Table != (spec.Table{Schema: "g", Name: "v"}) ||
		second.Sources[0].Snapshot == nil || second.Sources[0].Snapshot.View == nil {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	wantOld := []spec.Table{{Schema: "g", Name: "u"}}
	wantNew := []spec.Table{{Schema: "g", Name: "t"}}
	if !reflect.DeepEqual(first.Target.View.Definition.Sources, wantOld) ||
		!reflect.DeepEqual(second.Sources[0].Snapshot.View.Definition.Sources, wantNew) ||
		!reflect.DeepEqual(provider.tables["g.v"], original) {
		t.Fatalf("first=%#v second=%#v provider=%#v", first.Target.View, second.Sources[0].Snapshot.View, provider.tables["g.v"])
	}
	second.Sources[0].Snapshot.View.Definition.Sources[0].Name = "mutated"
	if !reflect.DeepEqual(first.Target.View.Definition.Sources, wantOld) || !reflect.DeepEqual(provider.tables["g.v"], original) {
		t.Fatal("later projection aliased earlier or provider state")
	}
}

func TestT18FirstRefusalPreservesPriorView(t *testing.T) {
	provider := t18FirstFixture()
	provider.tables["g.t"] = &spec.TableSnapshot{Schema: "g", Exists: false}
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;")
	if !spec.AssessViewLifecycle(statements[0]).SourceInvalid {
		t.Fatal("missing source was not a proven refusal")
	}
	got := statements[1].Metadata.View.Sources[0].Snapshot.View.Definition.Sources
	if !reflect.DeepEqual(got, []spec.Table{{Schema: "g", Name: "u"}}) {
		t.Fatalf("refused ALTER changed the prior view: %#v", got)
	}
}

func TestT18ReviewInvalidBindingPreservesPriorView(t *testing.T) {
	for _, sql := range []string{
		"ALTER VIEW v AS SELECT other.t.id FROM g.t;\nCREATE VIEW next_v AS SELECT id FROM v;",
		"ALTER VIEW v AS SELECT t.id FROM g.t AS a;\nCREATE VIEW next_v AS SELECT id FROM v;",
	} {
		provider := t18FirstFixture()
		_, statements := t18FirstEnrich(t, provider, sql)
		if !spec.AssessViewLifecycle(statements[0]).SourceInvalid {
			t.Fatalf("invalid binding not refused: %#v", statements[0])
		}
		view := statements[1].Metadata.View.Sources[0].Snapshot.View
		if view == nil || view.Definition == nil || !reflect.DeepEqual(view.Definition.Sources, []spec.Table{{Schema: "g", Name: "u"}}) {
			t.Fatalf("refusal changed prior dependency: %#v", view)
		}
	}
}

func TestT18ReviewAliasProjectionCloned(t *testing.T) {
	provider := t18FirstFixture()
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW v AS SELECT a.id FROM g.t AS a;\nCREATE VIEW next_v AS SELECT id FROM v;")
	assessment := spec.AssessViewLifecycle(statements[0])
	if assessment.SourceInvalid || len(assessment.SourceRequired) != 0 {
		t.Fatalf("valid alias not accepted: %#v", assessment)
	}
	declared := statements[0].DDL.View.Definition
	projected := statements[1].Metadata.View.Sources[0].Snapshot.View.Definition
	if !reflect.DeepEqual(declared.SourceAliases, []string{"a"}) || !reflect.DeepEqual(projected.SourceAliases, []string{"a"}) {
		t.Fatalf("alias projection: declared=%#v projected=%#v", declared, projected)
	}
	projected.SourceAliases[0] = "mutated"
	if declared.SourceAliases[0] != "a" || provider.tables["g.v"].View.Definition.Sources[0].Name != "u" {
		t.Fatal("alias projection mutated an earlier declaration or provider snapshot")
	}
}

func TestT18ReviewIndependentSourceFacts(t *testing.T) {
	statement := spec.Statement{
		DDL: &spec.DDL{Operation: spec.DDLOperationAlterView, View: &spec.ViewDeclaration{
			Definition: &spec.ViewDefinition{SourcesKnown: true, Sources: []spec.Table{{Name: "missing"}, {Name: "unknown"}}},
		}},
		Metadata: &spec.Metadata{View: &spec.ViewMetadata{
			TargetIdentity: spec.Table{Schema: "g", Name: "v"},
			Target:         &spec.TableSnapshot{Exists: true, RelationKind: "view"},
			Sources: []spec.TargetBinding{
				{Table: spec.Table{Schema: "g", Name: "missing"}, Snapshot: &spec.TableSnapshot{Exists: false}},
				{Table: spec.Table{Schema: "g", Name: "unknown"}},
			},
		}},
	}
	assessment := spec.AssessViewLifecycle(statement)
	if !assessment.SourceInvalid || !containsT18Fact(assessment.SourceRequired, "table.exists") {
		t.Fatalf("independent missing source lost: %#v", assessment)
	}
	statement.DDL.View.Definition.Sources = statement.DDL.View.Definition.Sources[:1]
	statement.Metadata.View.Sources = statement.Metadata.View.Sources[:1]
	assessment = spec.AssessViewLifecycle(statement)
	if !assessment.SourceInvalid || containsT18Fact(assessment.SourceRequired, "view.columns") {
		t.Fatalf("absent source requested unnecessary columns: %#v", assessment)
	}
}

func TestT18FullDerivedColumnsDoNotCloneBaseAttributes(t *testing.T) {
	provider := t18FirstFixture()
	provider.tables["g.t"].Columns[0] = spec.Column{
		Name: "id", Type: "int", NotNull: true, AutoIncrement: true, HasDefault: true, DefaultValue: "42",
	}
	original := cloneTableSnapshot(provider.tables["g.t"])
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;")
	projected := statements[1].Metadata.View.Sources[0].Snapshot
	if projected == nil || !reflect.DeepEqual(projected.Columns, []spec.Column{{Name: "id"}}) {
		t.Fatalf("view inherited base column attributes: %#v", projected)
	}
	if !reflect.DeepEqual(provider.tables["g.t"], original) {
		t.Fatal("provider snapshot mutated")
	}
}

func TestT18FullStarProjectionFrozen(t *testing.T) {
	provider := t18FirstFixture()
	provider.tables["g.t"].Columns = []spec.Column{
		{Name: "id", Type: "int", AutoIncrement: true},
		{Name: "payload", Type: "int"},
		{Name: "hidden", Type: "int", Visibility: "invisible"},
	}
	_, statements := t18FirstEnrich(t, provider,
		"ALTER VIEW v AS SELECT * FROM t;\nALTER TABLE t ADD COLUMN extra INT;\nCREATE VIEW next_v AS SELECT id FROM v;")
	view := statements[2].Metadata.View.Sources[0].Snapshot
	if view == nil || !reflect.DeepEqual(view.Columns, []spec.Column{{Name: "id"}, {Name: "payload"}}) ||
		view.View == nil || view.View.Definition == nil || len(view.View.Definition.Bindings) != 2 {
		t.Fatalf("star not frozen at creation: %#v", view)
	}
}

func TestT18FullOutputProjectionAndRefusal(t *testing.T) {
	provider := t18FirstFixture()
	_, statements := t18FirstEnrich(t, provider,
		"ALTER VIEW v (renamed) AS SELECT id + 1 FROM t;\nCREATE VIEW next_v AS SELECT renamed FROM v;")
	view := statements[1].Metadata.View.Sources[0].Snapshot
	if view == nil || !reflect.DeepEqual(view.Columns, []spec.Column{{Name: "renamed"}}) ||
		view.View == nil || view.View.Definition == nil {
		t.Fatalf("explicit output was not projected: %#v", view)
	}
	provider = t18FirstFixture()
	_, statements = t18FirstEnrich(t, provider,
		"ALTER VIEW v (a, b) AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;")
	view = statements[1].Metadata.View.Sources[0].Snapshot
	if view == nil || view.View == nil || view.View.Definition == nil ||
		!reflect.DeepEqual(view.View.Definition.Sources, []spec.Table{{Schema: "g", Name: "u"}}) {
		t.Fatalf("invalid output list mutated the prior view: %#v", view)
	}
}

func TestT18FullOptionTransitions(t *testing.T) {
	t.Run("replace_resets_defaults", func(t *testing.T) {
		provider := t18FirstFixture()
		provider.tables["g.v"].View.Algorithm = "merge"
		provider.tables["g.v"].View.Security = "invoker"
		provider.tables["g.v"].View.DefinerKind = "named"
		provider.tables["g.v"].View.DefinerPrincipal = "private_old"
		provider.tables["g.v"].View.CheckOption = "local"
		_, statements := t18FirstEnrich(t, provider,
			"CREATE OR REPLACE VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;")
		view := statements[1].Metadata.View.Sources[0].Snapshot.View
		if view == nil || view.Security != "definer" || view.Algorithm != "undefined" ||
			view.DefinerKind != "current_user" || view.DefinerPrincipal != "" || view.CheckOption != "none" {
			t.Fatalf("replacement did not reset defaults: %#v", view)
		}
	})
	t.Run("merge_fallback", func(t *testing.T) {
		provider := t18FirstFixture()
		_, statements := t18FirstEnrich(t, provider,
			"ALTER ALGORITHM=MERGE VIEW v AS SELECT COUNT(*) AS n FROM t;\nCREATE VIEW next_v AS SELECT n FROM v;")
		view := statements[1].Metadata.View.Sources[0].Snapshot
		if view == nil || view.View == nil || view.View.Algorithm != "undefined" ||
			!reflect.DeepEqual(view.Columns, []spec.Column{{Name: "n"}}) {
			t.Fatalf("merge fallback not projected: %#v", view)
		}
	})
	t.Run("check_refusal_preserves_prior", func(t *testing.T) {
		provider := t18FirstFixture()
		_, statements := t18FirstEnrich(t, provider,
			"ALTER VIEW v AS SELECT COUNT(*) AS n FROM t WITH CASCADED CHECK OPTION;\nCREATE VIEW next_v AS SELECT id FROM v;")
		view := statements[1].Metadata.View.Sources[0].Snapshot.View
		if view == nil || view.Definition == nil ||
			!reflect.DeepEqual(view.Definition.Sources, []spec.Table{{Schema: "g", Name: "u"}}) {
			t.Fatalf("check refusal changed prior definition: %#v", view)
		}
	})
}

func TestT18FullDropViewLeavesTemporaryShadow(t *testing.T) {
	provider := t18FirstFixture()
	_, statements := t18FirstEnrich(t, provider,
		"CREATE TEMPORARY TABLE v(id INT);\nDROP VIEW v;\nCREATE VIEW next_v AS SELECT id FROM v;")
	source := statements[2].Metadata.View.Sources[0].Snapshot
	if source == nil || !source.Exists || !source.Temporary || !spec.AssessViewLifecycle(statements[2]).SourceInvalid {
		t.Fatalf("temporary source was not retained: %#v", source)
	}
}

func TestT18FullAlterTableCannotMutateView(t *testing.T) {
	provider := t18FirstFixture()
	original := cloneTableSnapshot(provider.tables["g.v"])
	_, statements := t18FirstEnrich(t, provider,
		"ALTER TABLE v ADD COLUMN extra INT;\nCREATE VIEW next_v AS SELECT id FROM v;")
	view := statements[1].Metadata.View.Sources[0].Snapshot
	if view == nil || !reflect.DeepEqual(view.Columns, original.Columns) ||
		view.View == nil || !reflect.DeepEqual(view.View, original.View) {
		t.Fatalf("table DDL mutated view: %#v", view)
	}
}

func containsT18Fact(facts []string, want string) bool {
	for _, fact := range facts {
		if fact == want {
			return true
		}
	}
	return false
}

func TestT18ReviewUncertainAlterAttributes(t *testing.T) {
	for _, tc := range []struct {
		name, sql, algorithm, security, principal string
	}{
		{"inherited", "ALTER ALGORITHM=UNDEFINED VIEW v AS SELECT id FROM t;", "merge", "definer", "prior"},
		{"changed", "ALTER ALGORITHM=MERGE DEFINER='new' SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;", "merge", "", ""},
		{"agreeing_security", "ALTER SQL SECURITY DEFINER VIEW v AS SELECT id FROM t;", "merge", "definer", "prior"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := t18FirstFixture()
			provider.tables["g.t"] = nil
			prior := provider.tables["g.v"]
			updatable := true
			prior.View.Updatable = &updatable
			prior.View.Algorithm = "merge"
			prior.View.DefinerKind = "named"
			prior.View.DefinerPrincipal = "prior"
			before := cloneTableSnapshot(prior)
			_, statements := t18FirstEnrich(t, provider, tc.sql+"\nCREATE VIEW next_v AS SELECT id FROM v;")
			projected := statements[1].Metadata.View.Sources[0].Snapshot
			first := statements[0].Metadata.View.Target
			if projected == nil || first == nil {
				t.Fatalf("missing projection: projected=%#v first=%#v", projected, first)
			}
			if !projected.Exists || projected.RelationKind != "view" || projected.View == nil ||
				projected.View.Definition != nil || projected.Columns != nil || projected.View.Updatable != nil ||
				projected.View.Algorithm != tc.algorithm || projected.View.Security != tc.security ||
				projected.View.DefinerPrincipal != tc.principal || (tc.principal == "" && projected.View.DefinerKind != "") ||
				!reflect.DeepEqual(prior, before) || first.Exists != before.Exists ||
				first.RelationKind != before.RelationKind || !reflect.DeepEqual(first.Table, before.Table) ||
				!reflect.DeepEqual(first.Columns, before.Columns) || !reflect.DeepEqual(first.View, before.View) {
				t.Fatalf("projected=%#v prior=%#v first=%#v", projected.View, before.View, first.View)
			}
		})
	}
}

func TestT18FullUnknownInheritedAlgorithmWithholdsAlter(t *testing.T) {
	provider := t18FirstFixture()
	prior := cloneTableSnapshot(provider.tables["g.v"])
	_, statements := t18FirstEnrich(t, provider,
		"ALTER VIEW v AS SELECT id FROM t WITH LOCAL CHECK OPTION;\nCREATE VIEW next_v AS SELECT id FROM v;")
	assessment := spec.AssessViewLifecycle(statements[0])
	projected := statements[1].Metadata.View.Sources[0].Snapshot
	if !reflect.DeepEqual(assessment.CheckRequired, []string{"view.algorithm"}) ||
		projected == nil || !projected.Exists || projected.RelationKind != "view" ||
		projected.Columns != nil || projected.View == nil || projected.View.Definition != nil ||
		projected.View.Security != "definer" || !reflect.DeepEqual(provider.tables["g.v"], prior) {
		t.Fatalf("assessment=%#v projected=%#v provider=%#v", assessment, projected, provider.tables["g.v"])
	}
}

func TestT18FullReviewQueryOrderRefusalPreservesPrior(t *testing.T) {
	provider := t18FirstFixture()
	prior := cloneTableSnapshot(provider.tables["g.v"])
	_, statements := t18FirstEnrich(t, provider,
		"ALTER VIEW v AS SELECT id FROM t ORDER BY missing_col;\nCREATE VIEW next_v AS SELECT id FROM v;")
	assessment := spec.AssessViewLifecycle(statements[0])
	projected := statements[1].Metadata.View.Sources[0].Snapshot
	if !assessment.SourceInvalid || projected == nil || projected.View == nil ||
		projected.View.Definition == nil ||
		!reflect.DeepEqual(projected.View.Definition.Sources, []spec.Table{{Schema: "g", Name: "u"}}) ||
		!reflect.DeepEqual(provider.tables["g.v"], prior) {
		t.Fatalf("assessment=%#v projected=%#v prior=%#v", assessment, projected, prior)
	}
}

func TestT18FullReviewQueryUnmodeledAlterWithholdsChangedFacts(t *testing.T) {
	for _, query := range []string{
		"SELECT ROW_NUMBER() OVER(ORDER BY id) AS rn FROM t",
		"SELECT id FROM t LIMIT 1",
	} {
		t.Run(query, func(t *testing.T) {
			provider := t18FirstFixture()
			prior := cloneTableSnapshot(provider.tables["g.v"])
			_, statements := t18FirstEnrich(t, provider,
				"ALTER VIEW v AS "+query+";\nCREATE VIEW next_v AS SELECT id FROM v;")
			projected := statements[1].Metadata.View.Sources[0].Snapshot
			assessment := spec.AssessViewLifecycle(statements[1])
			owesDefinition := false
			for _, fact := range assessment.SourceRequired {
				owesDefinition = owesDefinition || fact == "view.definition"
			}
			if projected == nil || !projected.Exists || projected.RelationKind != "view" ||
				projected.Columns != nil || projected.View == nil || projected.View.Definition != nil ||
				projected.View.Security != "definer" || !owesDefinition ||
				!reflect.DeepEqual(provider.tables["g.v"], prior) {
				t.Fatalf("projected=%#v assessment=%#v prior=%#v", projected, assessment, prior)
			}
		})
	}
}

func TestT18FullReviewTiDBUnknownDuplicateNotView(t *testing.T) {
	for _, ifExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("if_exists=%t", ifExists), func(t *testing.T) {
			provider := t18FirstFixture()
			provider.tables["g.v"] = nil
			sqlText := "DROP VIEW "
			if ifExists {
				sqlText += "IF EXISTS "
			}
			sqlText += "v, v;\nCREATE OR REPLACE VIEW v AS SELECT id FROM t;"
			ctx := context.Background()
			parsed, err := Parse(ctx, sqlText, spec.DialectTiDB)
			if err != nil {
				t.Fatal(err)
			}
			extracted, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			facts := &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}
			state := newBatchState(spec.DialectTiDB, "g", provider)
			state.resolvedVersion = spec.ParseObservedVersion(facts.Version)
			state.instanceFacts = facts
			statements, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "g", Provider: provider},
				extracted, nil, facts, auditLimits{orderedStatements: len(extracted)})
			if err != nil || len(statements) != 2 {
				t.Fatalf("err=%v statements=%#v", err, statements)
			}
			target := statements[1].Metadata.View.Targets[0]
			if !target.NotView || target.Snapshot != nil || provider.tables["g.v"] != nil {
				t.Fatalf("target=%#v provider=%#v", target, provider.tables["g.v"])
			}
		})
	}
}

func TestT18FirstPolicyIndependent(t *testing.T) {
	sql := "ALTER VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;"
	enabled := t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		"ddl.view.target.valid":          {level: "blocker"},
		"ddl.view.source.valid":          {level: "blocker"},
		"ddl.view.security.definer.warn": {level: "warning"},
	})
	disabled := t05PolicyPathConfigured(t, map[string]t05RuleConfig{})
	for _, tc := range []struct {
		path   string
		loaded int
	}{{enabled, 3}, {disabled, 0}} {
		provider := t18FirstFixture()
		_, statements := t18FirstEnrich(t, provider, sql)
		prior := cloneTableSnapshot(statements[1].Metadata.View.Sources[0].Snapshot)
		cfg, err := apppolicy.Load(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		registry, err := buildRegistry(cfg)
		if err != nil {
			t.Fatal(err)
		}
		result, err := EvaluateStatements(context.Background(), registry, statements)
		if err != nil || registry.LoadedStatementRuleCount() != tc.loaded ||
			!reflect.DeepEqual(statements[1].Metadata.View.Sources[0].Snapshot, prior) {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		public, err := AuditSQL(context.Background(), Request{
			SQL: sql, Dialect: spec.DialectMySQL, Schema: "g", ConfigPath: tc.path, MetadataProvider: t18FirstFixture(),
		})
		if err != nil || len(public.Statements) != 2 {
			t.Fatalf("audit=%#v err=%v", public, err)
		}
		if tc.loaded == 3 && public.Summary.Warnings != 2 || tc.loaded == 0 && public.Summary.Warnings != 0 {
			t.Fatalf("loaded=%d summary=%#v", tc.loaded, public.Summary)
		}
	}
}

type t18FirstErrorProvider struct {
	*t18FirstProvider
	cause error
	calls int
}

func (p *t18FirstErrorProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	if table == "t" {
		p.calls++
		return nil, p.cause
	}
	return p.t18FirstProvider.LoadTableSnapshot(ctx, dialect, schema, table)
}

func TestT18FirstReadFailureNotCached(t *testing.T) {
	cause := errors.New("t18_private_error")
	provider := &t18FirstErrorProvider{t18FirstProvider: t18FirstFixture(), cause: cause}
	parsed, err := Parse(context.Background(), "ALTER VIEW v AS SELECT id FROM t;", spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "g", provider)
	for i := 0; i < 2; i++ {
		_, err := state.enrichStatements(context.Background(), &MetadataRequest{Schema: "g", Provider: provider},
			statements, nil, &spec.InstanceFacts{Version: "8.0.46"}, auditLimits{orderedStatements: 1})
		if !errors.Is(err, cause) || strings.Contains(err.Error(), "t18_private_error") {
			t.Fatalf("err=%v", err)
		}
	}
	if provider.calls != 2 {
		t.Fatalf("failed reads were cached: calls=%d", provider.calls)
	}
}

func TestT18FirstCrossSchemaSourcesBindRequestSchema(t *testing.T) {
	provider := t18FirstFixture()
	target := cloneTableSnapshot(provider.tables["g.v"])
	target.Schema, target.Table.Schema = "other", "other"
	provider.tables["other.v"] = target
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW other.v AS SELECT id FROM t;")
	source := statements[0].Metadata.View.Sources[0]
	if source.Table != (spec.Table{Schema: "g", Name: "t"}) || source.Snapshot == nil || !source.Snapshot.Exists {
		t.Fatalf("source=%#v", source)
	}
}

func TestT18FirstDependencyLimitIsIntrinsic(t *testing.T) {
	provider := t18FirstFixture()
	for i := 1; i <= 33; i++ {
		name := fmt.Sprintf("v%d", i)
		source := "t"
		if i < 33 {
			source = fmt.Sprintf("v%d", i+1)
		}
		provider.tables["g."+name] = &spec.TableSnapshot{
			Schema: "g", Table: &spec.Table{Schema: "g", Name: name}, Exists: true,
			RelationKind: "view", Columns: []spec.Column{{Name: "id", Type: "int"}},
			View: &spec.ViewSnapshot{Security: "definer", Definition: &spec.ViewDefinition{
				SourcesKnown: true, Sources: []spec.Table{{Schema: "g", Name: source}},
			}},
		}
	}
	_, statements := t18FirstEnrich(t, provider, "ALTER VIEW v AS SELECT id FROM v1;")
	if !statements[0].Metadata.View.DependencyLimit {
		t.Fatal("depth limit was not recorded")
	}
	assessment := spec.AssessViewLifecycle(statements[0])
	if !assessment.DependencyLimit || len(assessment.SourceRequired) != 0 {
		t.Fatalf("limit confused with missing metadata: %#v", assessment)
	}
	cfg, err := apppolicy.Load(t05PolicyPathConfigured(t, map[string]t05RuleConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := buildRegistry(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateStatements(context.Background(), registry, statements)
	if err != nil || string(result.Coverage.Status) != "incomplete" || len(result.Unsupported) != 1 ||
		result.Unsupported[0].Feature != "view.dependencies.limit" || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
