package tidbparser

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT18FirstStructuredViewGrammar(t *testing.T) {
	sql := "ALTER /* comment VIEW */ ALGORITHM=UNDEFINED DEFINER='private_user'@'host' SQL SECURITY INVOKER VIEW `g`.`v` (`x`) AS SELECT id FROM t WITH CHECK OPTION;"
	parsed, err := New().Parse(context.Background(), sql)
	if err != nil || len(parsed.Statements) != 1 {
		t.Fatalf("parse=%#v err=%v", parsed, err)
	}
	wrapped := WrapStatements(parsed.Statements, parsed.Warnings)
	statement, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
	if err != nil || statement.DDL == nil || statement.DDL.View == nil {
		t.Fatalf("statement=%#v err=%v", statement, err)
	}
	view := statement.DDL.View
	if statement.DDL.Operation != spec.DDLOperationAlterView || *statement.DDL.Table != (spec.Table{Schema: "g", Name: "v"}) ||
		!reflect.DeepEqual(view.Columns, []string{"x"}) || !view.AlgorithmExplicit || view.Algorithm != "undefined" ||
		!view.SecurityExplicit || view.Security != "invoker" || view.DefinerKind != "named" ||
		!view.CheckExplicit || !view.CheckBare || view.CheckOption != "cascaded" ||
		view.Definition == nil || !reflect.DeepEqual(view.Definition.Sources, []spec.Table{{Name: "t"}}) ||
		!strings.Contains(statement.RawSQL, "ALTER /* comment VIEW */") {
		t.Fatalf("statement=%#v view=%#v", statement, view)
	}
	tidb, err := wrapped[0].Extractor.Extract(spec.DialectTiDB, wrapped[0].RawSQL)
	if err != nil || tidb.Unsupported == nil || tidb.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb=%#v err=%v", tidb, err)
	}
}

func TestT18FirstBareCheckAndInvalidHeaders(t *testing.T) {
	for _, sql := range []string{
		"CREATE VIEW v AS SELECT id FROM t WITH CHECK OPTION;",
		"ALTER VIEW v AS SELECT id FROM t WITH CHECK OPTION;",
	} {
		parsed, err := New().Parse(context.Background(), sql)
		if err != nil || len(parsed.Statements) != 1 {
			t.Fatalf("%q parse=%#v err=%v", sql, parsed, err)
		}
		wrapped := WrapStatements(parsed.Statements, parsed.Warnings)
		got, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
		if err != nil || got.DDL == nil || got.DDL.View == nil || !got.DDL.View.CheckBare {
			t.Fatalf("%q got=%#v err=%v", sql, got, err)
		}
	}
	for _, sql := range []string{
		"ALTER ALGORITHM=MERGE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;",
		"ALTER VIEW v AS SELECT FROM t;",
		"ALTER VIEW v AS SELECT id INTO OUTFILE '/tmp/private' FROM t;",
		"ALTER VIEW v AS SELECT id FROM t; DROP TABLE t;",
	} {
		if _, err := New().Parse(context.Background(), sql); err == nil {
			t.Fatalf("expected invalid statement: %q", sql)
		}
	}
}

func TestT18FirstExternalDefinitionSources(t *testing.T) {
	definition, err := ParseViewDefinition(context.Background(), "WITH c AS (SELECT id FROM g.t) SELECT id FROM c")
	if err != nil || definition == nil || !definition.SourcesKnown || !definition.HasCTE || definition.Query == nil ||
		!reflect.DeepEqual(definition.Sources, []spec.Table{{Schema: "g", Name: "t"}}) {
		t.Fatalf("definition=%#v err=%v", definition, err)
	}
	if _, err := ParseViewDefinition(context.Background(), "SELECT FROM t"); err == nil {
		t.Fatal("invalid external definition parsed")
	}
}

func TestT18FullReviewDefinerKeyword(t *testing.T) {
	for _, tc := range []struct{ value, kind string }{
		{"view", "named"}, {"'view'", "named"}, {"'u'@view", "named"},
		{"CURRENT_USER", "current_user"}, {"CURRENT_USER()", "current_user"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			sql := "ALTER DEFINER=" + tc.value + " SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;"
			parsed, err := New().Parse(context.Background(), sql)
			if err != nil || len(parsed.Statements) != 1 {
				t.Fatalf("parse=%#v err=%v", parsed, err)
			}
			wrapped := WrapStatements(parsed.Statements, parsed.Warnings)
			statement, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
			if err != nil || statement.DDL == nil || statement.DDL.View == nil ||
				statement.DDL.Operation != spec.DDLOperationAlterView ||
				statement.DDL.View.DefinerKind != tc.kind || statement.DDL.View.Security != "invoker" ||
				(tc.kind == "named" && statement.DDL.View.DefinerPrincipal != tc.value) ||
				statement.RawSQL != sql {
				t.Fatalf("statement=%#v err=%v", statement, err)
			}
		})
	}
	for _, value := range []string{"SELECT", "123", "'u'@"} {
		if _, err := New().Parse(context.Background(),
			"ALTER DEFINER="+value+" SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;"); err == nil {
			t.Fatalf("invalid definer accepted: %q", value)
		}
	}
}

func TestT18FullExternalDefinitionCTEScope(t *testing.T) {
	definition, err := ParseViewDefinition(context.Background(),
		"WITH unused AS (SELECT id FROM g.hidden), c AS (SELECT id FROM g.c) SELECT id FROM c")
	if err != nil || definition == nil || !definition.SourcesKnown || definition.Query == nil {
		t.Fatalf("definition=%#v err=%v", definition, err)
	}
	mysql := spec.SelectViewDefinition(definition, spec.DialectMySQL)
	tidb := spec.SelectViewDefinition(definition, spec.DialectTiDB)
	if !reflect.DeepEqual(mysql.Sources, []spec.Table{{Schema: "g", Name: "c"}}) ||
		!reflect.DeepEqual(tidb.Sources, []spec.Table{{Schema: "g", Name: "hidden"}, {Schema: "g", Name: "c"}}) {
		t.Fatalf("mysql=%#v tidb=%#v", mysql.Sources, tidb.Sources)
	}
	tidb.Query.With[0].Sources[0].Table.Name = "changed"
	if mysql.Query.With[0].Sources[0].Table.Name != "hidden" ||
		definition.Query.With[0].Sources[0].Table.Name != "hidden" {
		t.Fatal("selected query graphs alias their source definitions")
	}
}

func TestT18ReviewIdentifierAndDefinerProductions(t *testing.T) {
	for _, tc := range []struct {
		sql, schema, name, definer string
		columns                    []string
	}{
		{"ALTER VIEW `g.dot`.`v.dot` (`c.dot`) AS SELECT id FROM t;", "g.dot", "v.dot", "", []string{"c.dot"}},
		{"ALTER VIEW g.action AS SELECT id FROM t;", "g", "action", "", nil},
		{"ALTER VIEW g.123 AS SELECT id FROM t;", "g", "123", "", nil},
		{"ALTER VIEW g.select AS SELECT id FROM t;", "g", "select", "", nil},
		{"ALTER DEFINER=CURRENT_USER() VIEW v AS SELECT id FROM t;", "", "v", "current_user", nil},
		{"ALTER DEFINER='private' VIEW v AS SELECT id FROM t;", "", "v", "named", nil},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			parsed, err := New().Parse(context.Background(), tc.sql)
			if err != nil || len(parsed.Statements) != 1 {
				t.Fatalf("parse=%#v err=%v", parsed, err)
			}
			wrapped := WrapStatements(parsed.Statements, parsed.Warnings)
			statement, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
			if err != nil || statement.DDL == nil || statement.DDL.View == nil ||
				*statement.DDL.Table != (spec.Table{Schema: tc.schema, Name: tc.name}) ||
				statement.DDL.View.DefinerKind != tc.definer || !reflect.DeepEqual(statement.DDL.View.Columns, tc.columns) {
				t.Fatalf("statement=%#v err=%v", statement, err)
			}
		})
	}
}
