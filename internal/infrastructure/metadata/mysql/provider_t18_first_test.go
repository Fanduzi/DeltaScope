package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t18ViewRows(view testQueryResult) map[string]testQueryResult {
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows", "table_type"},
			rows:    [][]driver.Value{{nil, nil, "VIEW", nil, nil, nil, "VIEW"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    [][]driver.Value{{"id", "int", nil, nil, "", nil, "YES", "", ""}},
		},
		"from information_schema.views": view,
	}
}

func TestT18FirstViewCatalogFacts(t *testing.T) {
	rows := testQueryResult{
		columns: []string{"view_definition", "security_type", "check_option", "is_updatable", "definer"},
		rows:    [][]driver.Value{{"SELECT `g`.`u`.`id` AS `id` FROM `g`.`u`", "DEFINER", "NONE", "YES", "t18_private_principal@host"}},
	}
	snapshot, err := NewProvider(openTestDB(t, t18ViewRows(rows))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if err != nil || snapshot == nil || !snapshot.Exists || snapshot.RelationKind != "view" || snapshot.View == nil ||
		snapshot.View.Security != "definer" || snapshot.View.CheckOption != "none" ||
		snapshot.View.Updatable == nil || !*snapshot.View.Updatable || snapshot.View.Definition == nil ||
		!snapshot.View.Definition.SourcesKnown || len(snapshot.View.Definition.Sources) != 1 ||
		snapshot.View.Definition.Sources[0] != (spec.Table{Schema: "g", Name: "u"}) {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(encoded), "t18_private_principal") || strings.Contains(string(encoded), "SELECT") {
		t.Fatalf("snapshot JSON=%s err=%v", encoded, err)
	}
}

func TestT18FirstViewCatalogNullAndError(t *testing.T) {
	row := testQueryResult{
		columns: []string{"view_definition", "security_type", "check_option", "is_updatable", "definer"},
		rows:    [][]driver.Value{{nil, "DEFINER", "NONE", nil, nil}},
	}
	snapshot, err := NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if err != nil || snapshot == nil || !snapshot.Exists || snapshot.View == nil || snapshot.View.Definition != nil {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
	row.rows = nil
	snapshot, err = NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if err != nil || snapshot == nil || !snapshot.Exists || snapshot.RelationKind != "view" ||
		snapshot.View == nil || snapshot.View.Definition != nil {
		t.Fatalf("missing attributes must remain unknown: snapshot=%#v err=%v", snapshot, err)
	}
	cause := errors.New("t18_private_driver_error")
	row = testQueryResult{queryErr: cause}
	_, err = NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "t18_private_driver_error") {
		t.Fatalf("unbounded or lost error: %v", err)
	}
	row = testQueryResult{queryErr: sql.ErrNoRows}
	_, err = NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("driver error was treated as an empty result: %v", err)
	}
	row = testQueryResult{
		columns:  []string{"view_definition", "security_type", "check_option", "is_updatable", "definer"},
		rows:     [][]driver.Value{{"SELECT id FROM t", "DEFINER", "NONE", "YES", "private"}},
		rowErrAt: 1, rowErr: cause,
	}
	_, err = NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "t18_private_driver_error") {
		t.Fatalf("unbounded or lost iteration error: %v", err)
	}
	row = testQueryResult{queryErr: context.DeadlineExceeded}
	_, err = NewProvider(openTestDB(t, t18ViewRows(row))).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "g", "v")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("provider timeout was lost: %v", err)
	}
}
