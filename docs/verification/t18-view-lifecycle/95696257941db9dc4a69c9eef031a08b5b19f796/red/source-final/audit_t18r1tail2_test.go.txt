package deltascope

import (
	"context"
	"reflect"
	"testing"
)

type t18R1Tail2Provider struct{ views map[string]*bool }

func (t18R1Tail2Provider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{Version: "8.0.46"}, nil
}

func (p t18R1Tail2Provider) LoadTableSnapshot(_ context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	if updatable, ok := p.views[table]; ok {
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: table}, Exists: true,
			RelationKind: "view", Columns: []Column{{Name: "id", Type: "int"}},
			View: &ViewSnapshot{Updatable: updatable, Definition: &ViewDefinition{
				SQL: "SELECT id FROM g.t_base", Sources: []Table{{Schema: schema, Name: "t_base"}},
				SourcesKnown: true, ShapeKnown: true,
				Bindings: []ViewColumnBinding{{Column: "id", Output: "id"}},
			}}}, nil
	}
	switch table {
	case "t_amount":
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: table}, Exists: true,
			RelationKind: "table", Columns: []Column{{Name: "x", Type: "int"}}}, nil
	case "t_base":
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: table}, Exists: true,
			RelationKind: "table", Columns: []Column{{Name: "id", Type: "int"}}}, nil
	default:
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: table}, Exists: false}, nil
	}
}

func TestT18R1Tail2MixedJoinDoesNotPublishFalseCheckBlocker(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE VIEW agg_v AS SELECT SUM(x) AS total FROM t_amount;\n" +
			"CREATE VIEW base_v AS SELECT id FROM t_base;\n" +
			"CREATE VIEW joined_v AS SELECT agg_v.total, base_v.id FROM agg_v JOIN base_v ON agg_v.total = base_v.id;\n" +
			"CREATE VIEW checked_v AS SELECT id FROM joined_v WITH CHECK OPTION;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
		MetadataProvider: t18R1Tail2Provider{},
	})
	if err != nil || len(result.Statements) != 4 || result.Coverage.Status != CoverageComplete ||
		result.Verdict != VerdictPass {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	for i, statement := range result.Statements {
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d unexpected public evidence: %#v", i, statement)
		}
	}
}

func TestT18R1Tail2NestedViewColumnCheckMatchesNative(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE VIEW mixed_v AS SELECT id, id + 1 AS x FROM t_base;\n" +
			"CREATE VIEW bad_check_v AS SELECT x FROM mixed_v WITH CHECK OPTION;\n" +
			"CREATE VIEW expr_v AS SELECT x FROM mixed_v;\n" +
			"CREATE VIEW next_check_v AS SELECT x FROM expr_v WITH CHECK OPTION;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
		MetadataProvider: t18R1Tail2Provider{},
	})
	if err != nil || len(result.Statements) != 4 || result.Coverage.Status != CoverageComplete ||
		result.Verdict != VerdictPass {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	for i, statement := range result.Statements {
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d unexpected public evidence: %#v", i, statement)
		}
	}
}

func TestT18R1Tail2GlobalReuseBoundaryMatchesNative(t *testing.T) {
	yes := true
	for _, tc := range []struct {
		name, sql string
		reject    bool
	}{
		{name: "source_view_same_name", reject: false,
			sql: "CREATE VIEW checked_v AS SELECT a.id FROM a WHERE EXISTS (SELECT 1 FROM a) WITH CHECK OPTION;"},
		{name: "base_table_through_view", reject: true,
			sql: "CREATE VIEW checked_v AS SELECT t.id FROM t_base AS t WHERE EXISTS (SELECT 1 FROM a) WITH CHECK OPTION;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
				MetadataProvider: t18R1Tail2Provider{views: map[string]*bool{"a": &yes}},
			})
			if err != nil || len(result.Statements) != 1 || result.Coverage.Status != CoverageComplete {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			findings := findingsForRule(result, 0, "ddl.view.check_option.valid")
			if tc.reject {
				if result.Verdict != VerdictReject || len(findings) != 1 || findings[0].Level != LevelBlocker {
					t.Fatalf("expected native check-option blocker: %#v", result)
				}
				return
			}
			if result.Verdict != VerdictPass || len(findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("source-view reuse was rejected contrary to MySQL 8.0.46: %#v", result)
			}
		})
	}
}

func TestT18R1Tail2CheckReportsUnprovenUpdatable(t *testing.T) {
	no := false
	provider := t18R1Tail2Provider{views: map[string]*bool{"a": &no, "b": nil}}
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE VIEW checked_v AS SELECT a.id, b.id AS b_id FROM a JOIN b ON a.id = b.id WITH CHECK OPTION;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
		MetadataProvider: provider,
	})
	if err != nil || len(result.Statements) != 1 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	if result.Coverage.Status != CoverageUnverified || result.Verdict != VerdictReview {
		t.Fatalf("coverage=%s verdict=%s result=%#v", result.Coverage.Status, result.Verdict, result)
	}
	statement := result.Statements[0]
	if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 1 {
		t.Fatalf("checked statement evidence=%#v", statement)
	}
	if !reflect.DeepEqual(statement.EvidenceGaps[0].RequiredFacts, []string{"view.updatable"}) {
		t.Fatalf("required facts=%v", statement.EvidenceGaps[0].RequiredFacts)
	}
}
