package deltascope

import (
	"context"
	"reflect"
	"testing"
)

type t18R1TailProvider struct {
	existingView bool
	unknownU     bool
}

func (p t18R1TailProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{Version: "8.0.46"}, nil
}

func (p t18R1TailProvider) LoadTableSnapshot(_ context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	absent := func(name string) *TableSnapshot {
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: name}, Exists: false}
	}
	base := func(name string) *TableSnapshot {
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: name}, Exists: true,
			RelationKind: "table", Columns: []Column{{Name: "id", Type: "int"}}}
	}
	switch table {
	case "t":
		return base("t"), nil
	case "u":
		if p.unknownU {
			return nil, nil
		}
		return base("u"), nil
	case "v":
		if !p.existingView {
			return absent("v"), nil
		}
		return &TableSnapshot{Schema: schema, Table: &Table{Schema: schema, Name: "v"}, Exists: true,
			RelationKind: "view", Columns: []Column{{Name: "id", Type: "int"}},
			View: &ViewSnapshot{
				Algorithm: "merge", Security: "definer", DefinerKind: "named", DefinerPrincipal: "t18_private_principal",
				Definition: &ViewDefinition{SQL: "SELECT id FROM g.u", SourcesKnown: true, ShapeKnown: true,
					Sources:  []Table{{Schema: schema, Name: "u"}},
					Bindings: []ViewColumnBinding{{Column: "id", Output: "id"}}},
			}}, nil
	default:
		return absent(table), nil
	}
}

func TestT18R1TailTemptableSourceRejectsCheckOption(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;\n" +
			"CREATE VIEW w AS SELECT id FROM v WITH CHECK OPTION;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
		MetadataProvider: t18R1TailProvider{},
	})
	if err != nil || result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete ||
		len(result.Statements) != 2 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("temptable create should not emit a check-option finding: %#v", result.Statements[0])
	}
	findings := findingsForRule(result, 1, "ddl.view.check_option.valid")
	if len(findings) != 1 || findings[0].Level != LevelBlocker {
		t.Fatalf("dependent check-option finding=%#v", result.Statements[1].Findings)
	}
}

func TestT18R1TailCheckRuleOffDoesNotPublishRefusedView(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;\n" +
			"CREATE VIEW w AS SELECT id FROM v WITH CHECK OPTION;\n" +
			"CREATE VIEW x AS SELECT id FROM w;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.source.valid"),
		MetadataProvider: t18R1TailProvider{},
	})
	if err != nil || result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete ||
		len(result.Statements) != 3 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	findings := findingsForRule(result, 2, "ddl.view.source.valid")
	if len(findings) != 1 || findings[0].Level != LevelBlocker {
		t.Fatalf("refused w leaked into later source state: %#v", result.Statements[2].Findings)
	}
}

func TestT18R1TailPendingAlterLeavesControlledSourceGap(t *testing.T) {
	for _, sql := range []string{
		"ALTER VIEW v AS SELECT COUNT(*) AS n FROM u;",
		"ALTER ALGORITHM=UNDEFINED VIEW v AS SELECT COUNT(*) AS n FROM u;",
	} {
		result, err := Audit(context.Background(), Request{
			SQL: sql, Dialect: DialectMySQL, Schema: "g",
			ConfigPath:       t18R1Policy(t, "ddl.view.source.valid"),
			MetadataProvider: t18R1TailProvider{existingView: true, unknownU: true},
		})
		if err != nil || len(result.Statements) != 1 || result.Coverage.Status != CoverageUnverified {
			t.Fatalf("sql=%q err=%v result=%#v", sql, err, result)
		}
		statement := result.Statements[0]
		if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 1 ||
			statement.EvidenceGaps[0].RuleID != "ddl.view.source.valid" ||
			!reflect.DeepEqual(statement.EvidenceGaps[0].RequiredFacts, []string{"table.exists"}) {
			t.Fatalf("sql=%q statement=%#v", sql, statement)
		}
	}
}
