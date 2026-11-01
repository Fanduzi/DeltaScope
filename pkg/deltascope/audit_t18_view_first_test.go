package deltascope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

type t18FirstProvider struct {
	sourceMode string
	targetMode string
	version    string
	extraTable string
	extraView  string
	reads      *[]string
	readErr    error
}

func (p t18FirstProvider) LoadInstanceFacts(_ context.Context, dialect Dialect, _ string) (*InstanceFacts, error) {
	if p.version != "" {
		if p.version == "unknown" {
			return &InstanceFacts{}, nil
		}
		return &InstanceFacts{Version: p.version}, nil
	}
	if dialect == DialectTiDB {
		return &InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}, nil
	}
	return &InstanceFacts{Version: "8.0.46"}, nil
}

func (p t18FirstProvider) LoadTableSnapshot(_ context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	if p.reads != nil {
		*p.reads = append(*p.reads, table)
	}
	if table == "t" {
		switch p.sourceMode {
		case "missing":
			return &TableSnapshot{Schema: schema, Exists: false}, nil
		case "unknown":
			return nil, nil
		case "error":
			return nil, p.readErr
		}
	}
	if table != "v" && table != "w" && table != "t" && table != "u" && table != p.extraTable && table != p.extraView {
		return &TableSnapshot{Schema: schema, Exists: false}, nil
	}
	snapshot := &TableSnapshot{
		Schema:       schema,
		Exists:       true,
		Table:        &Table{Schema: schema, Name: table},
		Columns:      []Column{{Name: "id", Type: "int"}},
		RelationKind: "table",
	}
	if table == "t" && p.sourceMode == "kind" {
		snapshot.RelationKind = ""
	}
	if table == "t" && p.sourceMode == "star" {
		snapshot.Columns = append(snapshot.Columns,
			Column{Name: "payload", Type: "int"},
			Column{Name: "hidden", Type: "int", Visibility: "invisible"})
	}
	if table == "v" || table == "w" || table == p.extraView {
		if p.targetMode == "unknown" {
			return nil, nil
		}
		if p.targetMode == "table" {
			return snapshot, nil
		}
		snapshot.RelationKind = "view"
		snapshot.View = &ViewSnapshot{
			Security: "definer", DefinerKind: "named", DefinerPrincipal: "t18_private_principal",
			Definition: &ViewDefinition{
				SQL:     "SELECT id FROM g.u /* t18_private_external */",
				Sources: []Table{{Schema: schema, Name: "u"}}, SourcesKnown: true, ShapeKnown: true,
				Bindings: []ViewColumnBinding{{Column: "id", Output: "id"}},
			},
		}
		if p.targetMode == "merge" || p.targetMode == "temptable" {
			snapshot.View.Algorithm = p.targetMode
		}
	}
	if table == "t" && p.sourceMode == "viewunknown" {
		snapshot.RelationKind = "view"
		snapshot.View = &ViewSnapshot{}
	}
	if table == "t" && p.sourceMode == "viewbroken" {
		snapshot.RelationKind = "view"
		snapshot.View = &ViewSnapshot{Definition: &ViewDefinition{
			SQL: "SELECT id FROM g.t18_private_hidden", SourcesKnown: true,
			Sources: []Table{{Schema: schema, Name: "t18_private_hidden"}},
		}}
	}
	return snapshot, nil
}

func t18FirstPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t18-first.yaml")
	policy := "rules:\n  ddl.view.create.forbid:\n    enabled: false\n  ddl.view.drop.forbid:\n    enabled: false\n"
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func t18ReviewPolicy(t *testing.T, security bool) string {
	return t18ReviewSelectedPolicy(t, true, security)
}

func t18ReviewSelectedPolicy(t *testing.T, semantic, security bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var config strings.Builder
	config.WriteString("rules:\n")
	for _, id := range ids {
		enabled := semantic && (id == "ddl.view.target.valid" || id == "ddl.view.source.valid") ||
			security && id == "ddl.view.security.definer.warn"
		fmt.Fprintf(&config, "  %s:\n    enabled: %t\n", strconv.Quote(id), enabled)
	}
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT18ReviewIndependentEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, target, source, blocker, gap string
	}{
		{"unknown_target_missing_source", "unknown", "missing", "ddl.view.source.valid", "ddl.view.target.valid"},
		{"wrong_target_unknown_source", "table", "unknown", "ddl.view.target.valid", "ddl.view.source.valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
				ConfigPath:       t18ReviewPolicy(t, false),
				MetadataProvider: t18FirstProvider{sourceMode: tc.source, targetMode: tc.target},
			})
			if err != nil || result.Verdict != VerdictReject || result.Coverage.Status != CoverageUnverified ||
				len(result.Statements) != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			stmt := result.Statements[0]
			if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != tc.blocker || stmt.Findings[0].Level != LevelBlocker ||
				len(stmt.EvidenceGaps) != 1 || stmt.EvidenceGaps[0].RuleID != tc.gap ||
				len(stmt.EvidenceGaps[0].RequiredFacts) != 1 || stmt.EvidenceGaps[0].RequiredFacts[0] != "table.exists" {
				t.Fatalf("statement=%#v", stmt)
			}
		})
	}
}

func TestT18ReviewInvalidViewIdentifiers(t *testing.T) {
	for _, sqlText := range []string{
		"ALTER VIEW 123 AS SELECT id FROM t;", "ALTER VIEW 1e3 AS SELECT id FROM t;",
		"ALTER VIEW SELECT AS SELECT id FROM t;",
		"ALTER VIEW v (123) AS SELECT id FROM t;", "ALTER VIEW v (1e3) AS SELECT id FROM t;",
		"ALTER VIEW v (SELECT) AS SELECT id FROM t;",
	} {
		t.Run(sqlText, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{SQL: sqlText, Dialect: DialectMySQL, ConfigPath: t18ReviewSelectedPolicy(t, false, false)})
			if err == nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "parser_error" {
				t.Fatalf("err=%v result=%#v", err, result)
			}
		})
	}
}

func TestT18ReviewNamedDefinerWithoutHost(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "ALTER DEFINER='private_named' VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{},
	})
	if err != nil || len(result.Statements) != 1 || result.Coverage.Status != CoverageComplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Classification == "parser_error" {
			t.Fatalf("named definer was rejected by the parser: %#v", diagnostic)
		}
	}
}

func TestT18ReviewColumnBindings(t *testing.T) {
	for _, expression := range []string{"other.t.id", "t.id"} {
		sqlText := "ALTER VIEW v AS SELECT " + expression + " FROM g.t"
		if expression == "t.id" {
			sqlText += " AS a"
		}
		sqlText += ";"
		t.Run(expression, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: sqlText, Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete ||
				len(result.Statements) != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			var source, security *Finding
			for i := range result.Statements[0].Findings {
				f := &result.Statements[0].Findings[i]
				switch f.RuleID {
				case "ddl.view.source.valid":
					source = f
				case "ddl.view.security.definer.warn":
					security = f
				}
			}
			if source == nil || source.Level != LevelBlocker || source.Location == nil ||
				source.Location.Line != 1 || source.Location.Column != strings.Index(sqlText, expression)+1 ||
				source.Metadata["role"] != "source" || source.Metadata["view"] != "v" ||
				source.Metadata["view_schema"] != "g" || source.Metadata["table"] != "t" || source.Metadata["schema"] != "g" ||
				security == nil || security.Level != LevelWarning || security.Location == nil || security.Location.Column != 1 ||
				security.Metadata["role"] != "view" || security.Metadata["view"] != "v" {
				t.Fatalf("findings=%#v", result.Statements[0].Findings)
			}
		})
	}
}

func TestT18ReviewTransitiveFindingUsesAuthoredRoot(t *testing.T) {
	sqlText := "ALTER VIEW v AS\n  SELECT id\n  FROM g.t;"
	result, err := Audit(context.Background(), Request{
		SQL: sqlText, Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{sourceMode: "viewbroken"},
	})
	if err != nil || len(result.Statements) != 1 || result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	var source *Finding
	for i := range result.Statements[0].Findings {
		if result.Statements[0].Findings[i].RuleID == "ddl.view.source.valid" {
			source = &result.Statements[0].Findings[i]
		}
	}
	if source == nil || source.Location == nil || source.Location.Line != 3 || source.Location.Column != 8 ||
		source.Metadata["role"] != "source" || source.Metadata["view"] != "v" ||
		source.Metadata["schema"] != "g" || source.Metadata["table"] != "t" {
		t.Fatalf("source finding=%#v", source)
	}
	payload, err := json.Marshal(struct {
		Findings    []Finding
		Gaps        []EvidenceGap
		Diagnostics any
	}{result.Statements[0].Findings, result.Statements[0].EvidenceGaps, result.Diagnostics})
	if err != nil || strings.Contains(string(payload), "t18_private_hidden") ||
		strings.Contains(string(payload), "t18_private_principal") || strings.Contains(string(payload), "SELECT id") {
		t.Fatalf("private dependency leaked: %s err=%v", payload, err)
	}
}

func TestT18ReviewMissingDefinitionAndInterfaceError(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18ReviewPolicy(t, false), MetadataProvider: t18FirstProvider{sourceMode: "viewunknown"},
	})
	if err != nil || result.Coverage.Status != CoverageUnverified || len(result.Statements) != 1 {
		t.Fatalf("missing definition: err=%v result=%#v", err, result)
	}
	found := false
	for _, gap := range result.Statements[0].EvidenceGaps {
		found = found || gap.RuleID == "ddl.view.source.valid" && len(gap.RequiredFacts) == 1 && gap.RequiredFacts[0] == "view.definition"
	}
	if !found {
		t.Fatalf("missing definition gap: %#v", result)
	}
	_, err = Audit(context.Background(), Request{
		SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18ReviewPolicy(t, false), MetadataProvider: t18FirstProvider{sourceMode: "error", readErr: sql.ErrNoRows},
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("provider error lost: %v", err)
	}
}

type t18ReviewConnector struct{ cause error }
type t18ReviewDriver struct{}
type t18ReviewConn struct{ cause error }

func (c t18ReviewConnector) Connect(context.Context) (driver.Conn, error) {
	return t18ReviewConn{cause: c.cause}, nil
}
func (t18ReviewConnector) Driver() driver.Driver          { return t18ReviewDriver{} }
func (t18ReviewDriver) Open(string) (driver.Conn, error)  { return t18ReviewConn{}, nil }
func (t18ReviewConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t18ReviewConn) Close() error                        { return nil }
func (t18ReviewConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c t18ReviewConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "information_schema.tables"):
		return &t06a4SDKRows{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows", "table_type"},
			rows: [][]driver.Value{{nil, nil, "", nil, nil, nil, "VIEW"}},
		}, nil
	case strings.Contains(query, "information_schema.columns"):
		return &t06a4SDKRows{
			cols: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows: [][]driver.Value{{"id", "int", nil, nil, "", nil, "YES", "", ""}},
		}, nil
	case strings.Contains(query, "information_schema.views"):
		if c.cause != nil {
			return nil, c.cause
		}
		return &t06a4SDKRows{cols: []string{"view_definition", "security_type", "check_option", "is_updatable", "definer"}}, nil
	default:
		return nil, sql.ErrNoRows
	}
}

type t18ReviewRealViewProvider struct{ inner *mysqlmeta.Provider }

func (p t18ReviewRealViewProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{Version: "8.0.46"}, nil
}
func (p t18ReviewRealViewProvider) LoadTableSnapshot(ctx context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	if table == "t" {
		return p.inner.LoadTableSnapshot(ctx, spec.DialectMySQL, schema, table)
	}
	return t18FirstProvider{}.LoadTableSnapshot(ctx, DialectMySQL, schema, table)
}

func TestT18ReviewRealProviderMissingAttributesPublic(t *testing.T) {
	cause := errors.New("t18_private_query_failure")
	for _, tc := range []struct {
		name  string
		cause error
	}{{"empty", nil}, {"error", cause}} {
		t.Run(tc.name, func(t *testing.T) {
			db := sql.OpenDB(t18ReviewConnector{cause: tc.cause})
			t.Cleanup(func() { db.Close() })
			result, err := Audit(context.Background(), Request{
				SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18ReviewPolicy(t, false), MetadataProvider: t18ReviewRealViewProvider{inner: mysqlmeta.NewProvider(db)},
			})
			if tc.cause != nil {
				if !errors.Is(err, tc.cause) || strings.Contains(err.Error(), "t18_private_query_failure") {
					t.Fatalf("query error not preserved safely: %v", err)
				}
				return
			}
			if err != nil || result.Coverage.Status != CoverageUnverified || len(result.Statements) != 1 {
				t.Fatalf("empty attributes: err=%v result=%#v", err, result)
			}
			found := false
			for _, gap := range result.Statements[0].EvidenceGaps {
				found = found || gap.RuleID == "ddl.view.source.valid" &&
					len(gap.RequiredFacts) == 1 && gap.RequiredFacts[0] == "view.definition"
			}
			if !found {
				t.Fatalf("missing definition gap: %#v", result)
			}
		})
	}
}

func TestT18FullWhereAndColumnValidation(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		blocker   bool
	}{
		{"where", "ALTER VIEW v AS SELECT id FROM t WHERE id > 0;", false},
		{"missing_where_column", "ALTER VIEW v AS SELECT id FROM t WHERE missing_col = 1;", true},
		{"replace", "CREATE OR REPLACE VIEW v AS SELECT id FROM t WHERE id > 0;", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{},
			})
			if err != nil || len(result.Statements) != 1 || len(result.Diagnostics) != 0 ||
				len(result.Unsupported) != 0 || result.Coverage.Status != CoverageComplete {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			wantVerdict, wantBlockers := VerdictReview, 0
			if tc.blocker {
				wantVerdict, wantBlockers = VerdictReject, 1
			}
			if result.Verdict != wantVerdict || result.Summary.Blockers != wantBlockers || result.Summary.Warnings != 1 {
				t.Fatalf("result=%#v", result)
			}
			if tc.blocker {
				found := false
				for _, finding := range result.Statements[0].Findings {
					found = found || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker &&
						finding.Location != nil && finding.Location.Line == 1 &&
						finding.Location.Column == strings.Index(tc.sql, "missing_col")+1
				}
				if !found {
					t.Fatalf("missing source blocker: %#v", result.Statements[0].Findings)
				}
			}
		})
	}
}

func TestT18FullStarFreezesAndRevalidates(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		index     int
	}{
		{"later_add", "ALTER VIEW v AS SELECT * FROM t;\nALTER TABLE t ADD COLUMN extra INT;\nCREATE VIEW next_v AS SELECT extra FROM v;", 2},
		{"invisible", "ALTER VIEW v AS SELECT * FROM t;\nCREATE VIEW next_v AS SELECT hidden FROM v;", 1},
		{"later_drop", "ALTER VIEW v AS SELECT * FROM t;\nALTER TABLE t DROP COLUMN id;\nCREATE VIEW next_v AS SELECT payload FROM v;", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{sourceMode: "star"},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Diagnostics) != 0 ||
				len(result.Unsupported) != 0 || len(result.Statements) != tc.index+1 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			findings := result.Statements[tc.index].Findings
			source, security := false, false
			for _, finding := range findings {
				source = source || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
				security = security || finding.RuleID == "ddl.view.security.definer.warn" && finding.Level == LevelWarning
			}
			if !source || !security {
				t.Fatalf("expected refusal and review: %#v", findings)
			}
		})
	}
}

type t18FullCountingProvider struct{ calls []string }

func (p *t18FullCountingProvider) LoadInstanceFacts(ctx context.Context, dialect Dialect, schema string) (*InstanceFacts, error) {
	return t18FirstProvider{}.LoadInstanceFacts(ctx, dialect, schema)
}
func (p *t18FullCountingProvider) LoadTableSnapshot(ctx context.Context, dialect Dialect, schema, table string) (*TableSnapshot, error) {
	p.calls = append(p.calls, table)
	return t18FirstProvider{}.LoadTableSnapshot(ctx, dialect, schema, table)
}

func TestT18FullDirectSourceLimitBeforeLookup(t *testing.T) {
	var tables []string
	for i := 0; i < 257; i++ {
		tables = append(tables, fmt.Sprintf("t%d", i))
	}
	provider := &t18FullCountingProvider{}
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE VIEW v AS SELECT t0.id FROM " + strings.Join(tables, ",") + ";",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18ReviewSelectedPolicy(t, false, false), MetadataProvider: provider,
	})
	if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
		len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("limit result: err=%v result=%#v", err, result)
	}
	found := false
	for _, detail := range result.Unsupported {
		found = found || detail.Feature == "view.dependencies.limit"
	}
	if !found || len(provider.calls) != 1 || provider.calls[0] != "v" {
		t.Fatalf("eager source lookup or missing boundary: calls=%#v unsupported=%#v", provider.calls, result.Unsupported)
	}
}

func TestT18FullOutputNamesAndColumnLists(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		blocker   bool
	}{
		{"named_constant", "CREATE VIEW next_v AS SELECT 1 AS answer;", false},
		{"unnamed_literal", "CREATE VIEW next_v AS SELECT 1;", false},
		{"named_expression", "CREATE VIEW next_v AS SELECT id + 1 AS next_id FROM t;", false},
		{"missing_expression_column", "CREATE VIEW next_v AS SELECT missing_col + 1 AS n FROM t;", true},
		{"explicit_list", "CREATE VIEW next_v (renamed) AS SELECT id FROM t;", false},
		{"count_mismatch", "CREATE VIEW next_v (a, b) AS SELECT id FROM t;", true},
		{"duplicate_names", "CREATE VIEW next_v AS SELECT id, id FROM t;", true},
		{"duplicate_explicit_names", "CREATE VIEW next_v (a, a) AS SELECT id, id FROM t;", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18ReviewPolicy(t, true), MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 1 ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 || result.Summary.Warnings != 1 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			wantBlockers, wantVerdict := 0, VerdictReview
			if tc.blocker {
				wantBlockers, wantVerdict = 1, VerdictReject
			}
			if result.Summary.Blockers != wantBlockers || result.Verdict != wantVerdict {
				t.Fatalf("result=%#v", result)
			}
			if tc.blocker {
				found := false
				for _, finding := range result.Statements[0].Findings {
					found = found || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
				}
				if !found {
					t.Fatalf("missing output-list blocker: %#v", result.Statements[0].Findings)
				}
			}
		})
	}
}

func t18FullPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "full.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var config strings.Builder
	config.WriteString("rules:\n")
	for _, id := range ids {
		enabled := id == "ddl.view.target.valid" || id == "ddl.view.source.valid" ||
			id == "ddl.view.security.definer.warn" || id == "ddl.view.algorithm.warn" ||
			id == "ddl.view.check_option.valid" || id == "ddl.view.options.ignored.warn"
		fmt.Fprintf(&config, "  %s:\n    enabled: %t\n", strconv.Quote(id), enabled)
	}
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT18FullOptionsSemanticOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		blockers  int
		warnings  int
	}{
		{"named_invoker", "CREATE DEFINER='private_named' SQL SECURITY INVOKER VIEW next_v AS SELECT id FROM t;", 0, 1},
		{"temptable", "CREATE ALGORITHM=TEMPTABLE VIEW next_v AS SELECT id FROM t;", 0, 2},
		{"merge_aggregate", "CREATE ALGORITHM=MERGE VIEW next_v AS SELECT COUNT(*) AS n FROM t;", 0, 2},
		{"aggregate_check", "CREATE VIEW next_v AS SELECT COUNT(*) AS n FROM t WITH CASCADED CHECK OPTION;", 1, 1},
		{"temptable_check", "CREATE ALGORITHM=TEMPTABLE VIEW next_v AS SELECT id FROM t WITH LOCAL CHECK OPTION;", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 1 ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
				result.Summary.Blockers != tc.blockers || result.Summary.Warnings != tc.warnings {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			want := VerdictReview
			if tc.blockers > 0 {
				want = VerdictReject
			}
			if result.Verdict != want {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}

func TestT18FullCheckRequiresInheritedAlgorithm(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER VIEW v AS SELECT id FROM t WITH LOCAL CHECK OPTION;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
	})
	if err != nil || result.Coverage.Status != CoverageUnverified || len(result.Statements) != 1 ||
		len(result.Statements[0].EvidenceGaps) != 1 ||
		result.Statements[0].EvidenceGaps[0].RuleID != "ddl.view.check_option.valid" ||
		!reflect.DeepEqual(result.Statements[0].EvidenceGaps[0].RequiredFacts, []string{"view.algorithm"}) {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}

func TestT18FullCheckResolvedAlgorithms(t *testing.T) {
	for _, tc := range []struct {
		name, clause, prior string
		blockers            int
	}{
		{"inherited_merge", "", "merge", 0},
		{"inherited_temptable", "", "temptable", 1},
		{"explicit_merge", "ALGORITHM=MERGE ", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     "ALTER " + tc.clause + "VIEW v AS SELECT id FROM t WITH LOCAL CHECK OPTION;",
				Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{targetMode: tc.prior},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 1 ||
				len(result.Statements[0].EvidenceGaps) != 0 || result.Summary.Blockers != tc.blockers {
				t.Fatalf("err=%v result=%#v", err, result)
			}
		})
	}
}

func TestT18FullInvalidDefinerGrammar(t *testing.T) {
	for _, principal := range []string{"SELECT", "123"} {
		sqlText := "ALTER DEFINER=" + principal + " VIEW v AS SELECT id FROM t;"
		result, err := Audit(context.Background(), Request{
			SQL: sqlText, Dialect: DialectMySQL, ConfigPath: t18ReviewSelectedPolicy(t, false, false),
		})
		if err == nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "parser_error" {
			t.Fatalf("principal=%s err=%v result=%#v", principal, err, result)
		}
	}
}

func TestT18FullTiDBIgnoredOptions(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE ALGORITHM=TEMPTABLE VIEW next_v AS SELECT id FROM t WITH LOCAL CHECK OPTION;",
		Dialect: DialectTiDB, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
	})
	if err != nil || result.Coverage.Status != CoverageComplete || result.Verdict != VerdictReview ||
		result.Summary.Warnings != 3 || result.Summary.Blockers != 0 || len(result.Statements) != 1 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	ignored := 0
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.options.ignored.warn" {
			ignored++
			if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column <= 1 {
				t.Fatalf("missing option location: %#v", finding)
			}
		}
	}
	if ignored != 2 {
		t.Fatalf("ignored-option findings: %#v", result.Statements[0].Findings)
	}
}

func TestT18FullDropViewOrderedOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, sql, version string
		dialect            Dialect
		targetBlockers     int
		sourceBlockers     []bool
		versionGap         bool
	}{
		{"mysql80_atomic", "DROP VIEW missing, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, 1, []bool{false}, false},
		{"mysql57_partial", "DROP VIEW missing, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "5.7.44", DialectMySQL, 1, []bool{true}, false},
		{"mysql_unknown", "DROP VIEW missing, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "unknown", DialectMySQL, 1, []bool{false}, true},
		{"mysql_duplicate", "DROP VIEW v, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, 1, []bool{false}, false},
		{"mysql_if_exists", "DROP VIEW IF EXISTS t, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, 0, []bool{true}, false},
		{"tidb_wrong_kind", "DROP VIEW IF EXISTS v, t, w;\nCREATE VIEW next_v AS SELECT id FROM v;\nCREATE VIEW next_w AS SELECT id FROM w;", "8.0.11-TiDB-v8.5.0", DialectTiDB, 1, []bool{true, false}, false},
		{"tidb_duplicate", "DROP VIEW v, v;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.11-TiDB-v8.5.0", DialectTiDB, 1, []bool{true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: tc.dialect, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{version: tc.version},
			})
			if err != nil || len(result.Statements) != len(tc.sourceBlockers)+1 ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			targetBlockers := 0
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "ddl.view.target.valid" && finding.Level == LevelBlocker {
					targetBlockers++
					if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column <= 1 {
						t.Fatalf("target location missing: %#v", finding)
					}
				}
			}
			if targetBlockers != tc.targetBlockers {
				t.Fatalf("target findings=%#v", result.Statements[0].Findings)
			}
			versionGap := false
			for _, gap := range result.Statements[0].EvidenceGaps {
				for _, fact := range gap.RequiredFacts {
					versionGap = versionGap || gap.RuleID == "ddl.view.target.valid" && fact == "database.version"
				}
			}
			if versionGap != tc.versionGap {
				t.Fatalf("target gaps=%#v", result.Statements[0].EvidenceGaps)
			}
			for i, want := range tc.sourceBlockers {
				found := false
				for _, finding := range result.Statements[i+1].Findings {
					found = found || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
				}
				if found != want {
					t.Fatalf("statement %d findings=%#v", i+1, result.Statements[i+1].Findings)
				}
			}
		})
	}
}

func TestT18FullDropIfExistsProvesNotViewNotAbsence(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "DROP VIEW IF EXISTS v;\nALTER VIEW v AS SELECT id FROM t;\nCREATE VIEW v AS SELECT id FROM t;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
		MetadataProvider: t18FirstProvider{targetMode: "unknown"},
	})
	if err != nil || len(result.Statements) != 3 || len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("conditional drop is not complete: %#v", result.Statements[0])
	}
	var targetBlocker, targetGap bool
	for _, finding := range result.Statements[1].Findings {
		targetBlocker = targetBlocker || finding.RuleID == "ddl.view.target.valid" && finding.Level == LevelBlocker
	}
	for _, gap := range result.Statements[2].EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			targetGap = targetGap || gap.RuleID == "ddl.view.target.valid" && fact == "table.exists"
		}
	}
	if !targetBlocker || !targetGap {
		t.Fatalf("not-view fact confused with absence: %#v", result.Statements)
	}
}

func TestT18FullDropTablePreservesViews(t *testing.T) {
	for _, tc := range []struct {
		name, sql, version string
		dialect            Dialect
		baseMissing        bool
	}{
		{"single_view", "DROP TABLE v;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, false},
		{"mysql57_mixed", "DROP TABLE t, v;\nCREATE VIEW next_t AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;", "5.7.44", DialectMySQL, true},
		{"mysql80_mixed", "DROP TABLE t, v;\nCREATE VIEW next_t AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, false},
		{"mysql80_if", "DROP TABLE IF EXISTS t, v;\nCREATE VIEW next_t AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.46", DialectMySQL, true},
		{"tidb_mixed", "DROP TABLE t, v;\nCREATE VIEW next_t AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;", "8.0.11-TiDB-v8.5.0", DialectTiDB, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: tc.dialect, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{version: tc.version},
			})
			if err != nil || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			for i := 1; i < len(result.Statements); i++ {
				found := false
				for _, finding := range result.Statements[i].Findings {
					found = found || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
				}
				want := len(result.Statements) == 3 && i == 1 && tc.baseMissing
				if found != want {
					t.Fatalf("statement %d findings=%#v", i, result.Statements[i].Findings)
				}
			}
		})
	}
}

func TestT18FullStructuredQueries(t *testing.T) {
	for _, tc := range []struct {
		name, sql, extraTable string
		dialect               Dialect
		blockers              int
	}{
		{"join", "SELECT a.id FROM t AS a JOIN u AS b ON a.id=b.id", "", DialectMySQL, 0},
		{"group", "SELECT id FROM t GROUP BY id", "", DialectMySQL, 0},
		{"join_missing_on", "SELECT a.id FROM t AS a JOIN u AS b ON a.absent=b.id", "", DialectMySQL, 1},
		{"join_ambiguous", "SELECT id FROM t AS a JOIN u AS b ON a.id=b.id", "", DialectMySQL, 1},
		{"derived", "SELECT d.id FROM (SELECT id FROM t) AS d", "", DialectMySQL, 0},
		{"union", "SELECT id FROM t UNION ALL SELECT id FROM u", "", DialectMySQL, 0},
		{"mysql_cte", "WITH c AS (SELECT id FROM t) SELECT id FROM c", "", DialectMySQL, 0},
		{"tidb_cte_rename", "WITH c(x) AS (SELECT id FROM t) SELECT x FROM c", "", DialectTiDB, 0},
		{"mysql_cte_self_base", "WITH c AS (SELECT id FROM c) SELECT id FROM c", "c", DialectMySQL, 0},
		{"tidb_cte_self_base", "WITH c AS (SELECT id FROM c) SELECT id FROM c", "c", DialectTiDB, 0},
		{"mysql_cte_forward", "WITH first_c AS (SELECT id FROM later_c), later_c AS (SELECT id FROM t) SELECT id FROM first_c", "", DialectMySQL, 1},
		{"tidb_cte_forward", "WITH first_c AS (SELECT id FROM later_c), later_c AS (SELECT id FROM t) SELECT id FROM first_c", "", DialectTiDB, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: "CREATE VIEW next_v AS " + tc.sql + ";", Dialect: tc.dialect, Schema: "g",
				ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{extraTable: tc.extraTable},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 1 ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
				result.Summary.Blockers != tc.blockers || result.Summary.Warnings != 1 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
		})
	}
}

func TestT18FullQuotedDotIdentities(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE VIEW `view.x` AS SELECT `a.b`.`id` FROM `t.x` AS `a.b`;\n" +
			"DROP TABLE `t.x`;\nCREATE VIEW next_v AS SELECT id FROM `view.x`;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
		MetadataProvider: t18FirstProvider{extraTable: "t.x"},
	})
	if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 3 ||
		len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.Level == LevelBlocker {
			t.Fatalf("valid quoted identities refused: %#v", result.Statements[0])
		}
	}
	for _, finding := range result.Statements[2].Findings {
		if finding.RuleID == "ddl.view.source.valid" && finding.Metadata["table"] == "view.x" {
			return
		}
	}
	t.Fatalf("missing dependency was not attributed to the authored dotted root: %#v", result.Statements[2])
}

func TestT18FullJoinStarFreezesAndRevalidates(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
	}{
		{"added_column", "CREATE VIEW next_v AS SELECT a.*, b.id AS uid FROM t a JOIN u b ON a.id=b.id;\nALTER TABLE t ADD COLUMN later INT;\nCREATE VIEW after_v AS SELECT later FROM next_v;"},
		{"dropped_dependency", "CREATE VIEW next_v AS SELECT a.id FROM t a JOIN u b ON a.id=b.id;\nALTER TABLE t DROP COLUMN id;\nCREATE VIEW after_v AS SELECT id FROM next_v;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{sourceMode: "star"},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Diagnostics) != 0 ||
				len(result.Unsupported) != 0 || len(result.Statements) != 3 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			blocker := false
			for _, finding := range result.Statements[2].Findings {
				blocker = blocker || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
			}
			if !blocker {
				t.Fatalf("stale joined output/dependency: %#v", result.Statements[2])
			}
		})
	}
}

func TestT18FullSourceFindingsPerAuthoredRoot(t *testing.T) {
	for _, tc := range []struct {
		name, projection string
		roots            []string
	}{
		{"same_root", "a.missing_one, a.missing_two", []string{"t"}},
		{"separate_roots", "a.missing_one, b.missing_two", []string{"t", "u"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     "CREATE VIEW next_v AS SELECT " + tc.projection + " FROM t a JOIN u b ON a.id=b.id;",
				Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Statements) != 1 ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			var roots []string
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "ddl.view.source.valid" {
					name, _ := finding.Metadata["table"].(string)
					roots = append(roots, name)
					if finding.Location == nil {
						t.Fatalf("source finding has no authored location: %#v", finding)
					}
				}
			}
			if !reflect.DeepEqual(roots, tc.roots) {
				t.Fatalf("roots=%#v, want %#v", roots, tc.roots)
			}
		})
	}
}

func TestT18FullUnusedCTEVendorScope(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		dialect     Dialect
		blockers    int
		readMissing bool
	}{
		{"mysql_missing", "SELECT id FROM missing_t", DialectMySQL, 0, false},
		{"mysql_missing_column", "SELECT missing_col FROM t", DialectMySQL, 0, false},
		{"tidb_missing", "SELECT id FROM missing_t", DialectTiDB, 1, true},
		{"tidb_missing_column", "SELECT missing_col FROM t", DialectTiDB, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads []string
			result, err := Audit(context.Background(), Request{
				SQL:     "CREATE VIEW next_v AS WITH unused AS (" + tc.body + ") SELECT id FROM t;",
				Dialect: tc.dialect, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{reads: &reads},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || len(result.Diagnostics) != 0 ||
				len(result.Unsupported) != 0 || result.Summary.Blockers != tc.blockers || result.Summary.Warnings != 1 {
				t.Fatalf("err=%v result=%#v reads=%#v", err, result, reads)
			}
			readMissing := false
			for _, table := range reads {
				readMissing = readMissing || table == "missing_t"
			}
			if readMissing != tc.readMissing {
				t.Fatalf("unexpected metadata reads: %#v", reads)
			}
		})
	}
}

func TestT18FullCTEVersionBoundary(t *testing.T) {
	sqlText := "CREATE VIEW g.next_v AS WITH c AS (SELECT id FROM g.t) SELECT id FROM c;"
	result, err := Audit(context.Background(), Request{
		SQL: sqlText, Dialect: DialectMySQL, TargetVersion: "5.7.44",
		ConfigPath: t18ReviewSelectedPolicy(t, false, false),
	})
	if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete || len(result.Unsupported) != 1 ||
		result.Unsupported[0].Feature != "view_query_cte" ||
		result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}

func TestT18FullStructuredQueryLimitsAreIntrinsic(t *testing.T) {
	t.Run("direct_sources_before_reads", func(t *testing.T) {
		var sqlText strings.Builder
		sqlText.WriteString("CREATE VIEW next_v AS SELECT 1 FROM t a0")
		for i := 1; i < 257; i++ {
			fmt.Fprintf(&sqlText, " JOIN t a%d ON 1=1", i)
		}
		sqlText.WriteByte(';')
		var reads []string
		result, err := Audit(context.Background(), Request{
			SQL: sqlText.String(), Dialect: DialectMySQL, Schema: "g",
			ConfigPath:       t18ReviewSelectedPolicy(t, false, false),
			MetadataProvider: t18FirstProvider{reads: &reads},
		})
		if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
			len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "view.dependencies.limit" ||
			len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("err=%v result=%#v reads=%#v", err, result, reads)
		}
		for _, table := range reads {
			if table == "t" {
				t.Fatalf("direct sources were read before the bound: %#v", reads)
			}
		}
	})
	t.Run("repeated_cte_edges", func(t *testing.T) {
		var sqlText strings.Builder
		sqlText.WriteString("CREATE VIEW next_v AS WITH c AS (SELECT id FROM t) SELECT 1 FROM c a0")
		for i := 1; i < 1025; i++ {
			fmt.Fprintf(&sqlText, " JOIN c a%d ON 1=1", i)
		}
		sqlText.WriteByte(';')
		result, err := Audit(context.Background(), Request{
			SQL: sqlText.String(), Dialect: DialectMySQL, Schema: "g",
			ConfigPath: t18ReviewSelectedPolicy(t, false, false), MetadataProvider: t18FirstProvider{},
		})
		if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
			len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "view.dependencies.limit" ||
			len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("err=%v result=%#v", err, result)
		}
	})
}

func TestT18FullReviewZeroSourceCTEDAGBudget(t *testing.T) {
	t18ReviewZeroSourceCTEDAG(t, 30)
}

func TestT18FullReviewZeroSourceCTEDAGDepthControl(t *testing.T) {
	t18ReviewZeroSourceCTEDAG(t, 36)
}

func t18ReviewZeroSourceCTEDAG(t *testing.T, depth int) {
	t.Helper()
	var sqlText strings.Builder
	sqlText.WriteString("CREATE VIEW next_v AS WITH c0 AS (SELECT 1 AS id)")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&sqlText, ", c%d AS (SELECT a.id FROM c%d a JOIN c%d b ON a.id=b.id)", i, i-1, i-1)
	}
	fmt.Fprintf(&sqlText, " SELECT id FROM c%d;", depth)
	result, err := Audit(context.Background(), Request{
		SQL: sqlText.String(), Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18ReviewSelectedPolicy(t, false, false), MetadataProvider: t18FirstProvider{},
	})
	if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
		len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "view.dependencies.limit" ||
		len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 ||
		len(result.Statements[0].Findings) != 0 || result.Statements[0].Index != 0 ||
		result.Statements[0].RawSQL != sqlText.String() || result.Statements[0].Coverage.Status != CoverageIncomplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}

func TestT18FullReviewQueryOrderBinding(t *testing.T) {
	firstSQL := "ALTER VIEW v AS SELECT id FROM t ORDER BY missing_col;"
	sqlText := firstSQL + "\nCREATE VIEW next_v AS SELECT id FROM v;"
	result, err := Audit(context.Background(), Request{
		SQL: sqlText, Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
	})
	if err != nil || result.Coverage.Status != CoverageComplete || result.Verdict != VerdictReject ||
		len(result.Statements) != 2 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
		result.Statements[0].Index != 0 || result.Statements[0].RawSQL != firstSQL ||
		result.Statements[0].Coverage.Status != CoverageComplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	var source, security *Finding
	for i := range result.Statements[0].Findings {
		finding := &result.Statements[0].Findings[i]
		switch finding.RuleID {
		case "ddl.view.source.valid":
			if source != nil {
				t.Fatalf("multiple source findings: %#v", result.Statements[0].Findings)
			}
			source = finding
		case "ddl.view.security.definer.warn":
			security = finding
		}
	}
	if source == nil || source.Level != LevelBlocker || source.Location == nil ||
		source.Location.Line != 1 || source.Location.Column != strings.Index(sqlText, "missing_col")+1 ||
		source.Metadata["table"] != "t" || source.Metadata["schema"] != "g" ||
		source.Metadata["view"] != "v" || source.Metadata["view_schema"] != "g" ||
		security == nil || security.Level != LevelWarning || len(result.Statements[0].Findings) != 2 {
		t.Fatalf("findings=%#v", result.Statements[0].Findings)
	}
}

func TestT18FullReviewQueryOrderAlias(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE VIEW next_v AS SELECT id AS renamed FROM t ORDER BY renamed;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
	})
	if err != nil || result.Coverage.Status != CoverageComplete || result.Summary.Blockers != 0 ||
		result.Summary.Warnings != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}

func TestT18FullReviewQueryClauseLocations(t *testing.T) {
	for _, tc := range []struct{ name, query, table, site string }{
		{"on", "SELECT t.id FROM t JOIN u ON t.id = u.missing_col", "u", "u.missing_col"},
		{"where", "SELECT id FROM t WHERE missing_col > 0", "t", "missing_col"},
		{"group", "SELECT id FROM t GROUP BY missing_col", "t", "missing_col"},
		{"having", "SELECT id FROM t GROUP BY id HAVING missing_col > 0", "t", "missing_col"},
		{"order", "SELECT id FROM t ORDER BY missing_col", "t", "missing_col"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sqlText := "ALTER VIEW v AS " + tc.query + ";"
			result, err := Audit(context.Background(), Request{
				SQL: sqlText, Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t),
				MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || result.Summary.Blockers != 1 ||
				result.Summary.Warnings != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID != "ddl.view.source.valid" {
					continue
				}
				if finding.Location == nil || finding.Location.Line != 1 ||
					finding.Location.Column != strings.Index(sqlText, tc.site)+1 ||
					finding.Metadata["table"] != tc.table || finding.Metadata["schema"] != "g" {
					t.Fatalf("finding=%#v", finding)
				}
				return
			}
			t.Fatalf("missing source finding: %#v", result.Statements[0].Findings)
		})
	}
}

func TestT18FullReviewQuerySetLimitBoundary(t *testing.T) {
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		for _, query := range []string{
			"SELECT id FROM t UNION SELECT id FROM u LIMIT 1",
			"SELECT id FROM t UNION SELECT id FROM u ORDER BY 1",
			"SELECT id FROM t UNION ALL SELECT id FROM u ORDER BY id",
		} {
			t.Run(string(dialect)+"/"+query, func(t *testing.T) {
				sqlText := "CREATE VIEW next_v AS " + query + ";"
				result, err := Audit(context.Background(), Request{
					SQL: sqlText, Dialect: dialect, Schema: "g",
					ConfigPath: t18ReviewSelectedPolicy(t, false, false), MetadataProvider: t18FirstProvider{},
				})
				if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
					result.Verdict != VerdictReview || len(result.Unsupported) != 1 ||
					result.Unsupported[0].Feature != "create_view.option.view_query_shape" ||
					len(result.Statements) != 1 || result.Statements[0].Index != 0 ||
					result.Statements[0].RawSQL != sqlText || result.Statements[0].Coverage.Status != CoverageIncomplete ||
					len(result.Statements[0].EvidenceGaps) != 0 || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("err=%v result=%#v", err, result)
				}
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Classification == "parser_error" {
						t.Fatalf("unexpected parser diagnostic: %#v", diagnostic)
					}
				}
			})
		}
	}
}

func TestT18FullReviewQueryUnsupportedExpressions(t *testing.T) {
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		for _, tc := range []struct{ name, query string }{
			{"window", "SELECT ROW_NUMBER() OVER(ORDER BY id) AS rn FROM t"},
			{"limit", "SELECT id FROM t LIMIT 1"},
			{"position", "SELECT id FROM t ORDER BY 1"},
		} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/enabled=%t", dialect, tc.name, enabled), func(t *testing.T) {
					config := t18ReviewSelectedPolicy(t, false, false)
					if enabled {
						config = t18FullPolicy(t)
					}
					result, err := Audit(context.Background(), Request{
						SQL: "CREATE VIEW next_v AS " + tc.query + ";", Dialect: dialect, Schema: "g",
						ConfigPath: config, MetadataProvider: t18FirstProvider{},
					})
					wantWarnings := 0
					if enabled {
						wantWarnings = 1
					}
					if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
						len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "create_view.option.view_query_shape" ||
						len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 ||
						result.Statements[0].Coverage.Status != CoverageIncomplete ||
						result.Summary.Blockers != 0 || result.Summary.Warnings != wantWarnings || result.Summary.Notices != 0 {
						t.Fatalf("err=%v result=%#v", err, result)
					}
					for _, diagnostic := range result.Diagnostics {
						if diagnostic.Classification == "parser_error" {
							t.Fatalf("unexpected parser diagnostic: %#v", diagnostic)
						}
					}
				})
			}
		}
	}
}

func TestT18FullReviewTiDBUnknownDuplicate(t *testing.T) {
	for _, ifExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("if_exists=%t", ifExists), func(t *testing.T) {
			sqlText := "DROP VIEW "
			if ifExists {
				sqlText += "IF EXISTS "
			}
			sqlText += "v, v;"
			result, err := Audit(context.Background(), Request{
				SQL: sqlText, Dialect: DialectTiDB, Schema: "g",
				ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{targetMode: "unknown"},
			})
			if err != nil || result.Coverage.Status != CoverageUnverified || len(result.Statements) != 1 ||
				result.Statements[0].Coverage.Status != CoverageUnverified ||
				len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
				len(result.Statements[0].EvidenceGaps) != 1 ||
				result.Statements[0].EvidenceGaps[0].RuleID != "ddl.view.target.valid" ||
				!reflect.DeepEqual(result.Statements[0].EvidenceGaps[0].RequiredFacts, []string{"table.exists"}) {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			findings := result.Statements[0].Findings
			if ifExists {
				if len(findings) != 0 || result.Verdict != VerdictReview {
					t.Fatalf("unexpected conditional finding: %#v", result)
				}
				return
			}
			if len(findings) != 1 || findings[0].RuleID != "ddl.view.target.valid" ||
				findings[0].Level != LevelBlocker || result.Verdict != VerdictReject ||
				findings[0].Location == nil || findings[0].Location.Column != strings.LastIndex(sqlText, "v")+1 ||
				findings[0].Metadata["role"] != "target" || findings[0].Metadata["view"] != "v" ||
				findings[0].Metadata["view_schema"] != "g" {
				t.Fatalf("findings=%#v result=%#v", findings, result)
			}
		})
	}
}

func TestT18FullReviewDefinerKeyword(t *testing.T) {
	for _, tc := range []struct {
		name, definer string
		warnings      int
	}{
		{"view", "view", 1}, {"quoted_view", "'view'", 1}, {"view_host", "'u'@view", 1},
		{"current_user", "CURRENT_USER", 0}, {"current_user_call", "CURRENT_USER()", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     "ALTER DEFINER=" + tc.definer + " SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;",
				Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FullPolicy(t), MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Coverage.Status != CoverageComplete || result.Summary.Blockers != 0 ||
				result.Summary.Warnings != tc.warnings || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
		})
	}
	for _, definer := range []string{"SELECT", "123", "'u'@"} {
		result, err := Audit(context.Background(), Request{
			SQL:     "ALTER DEFINER=" + definer + " SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;",
			Dialect: DialectMySQL, ConfigPath: t18ReviewSelectedPolicy(t, false, false),
		})
		if err == nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "parser_error" {
			t.Fatalf("definer=%q err=%v result=%#v", definer, err, result)
		}
	}
}

func TestT18FullDenylistHiddenDependencyEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "denylist.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var config strings.Builder
	config.WriteString("rules:\n")
	for _, id := range ids {
		fmt.Fprintf(&config, "  %s:\n    enabled: %t\n", strconv.Quote(id), id == "ddl.table.denylist.forbid")
		if id == "ddl.table.denylist.forbid" {
			config.WriteString("    params:\n      tables: [t18_private_hidden]\n")
		}
	}
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	sqlText := "CREATE VIEW next_v AS SELECT id FROM t;"
	for _, tc := range []struct {
		mode, coverage string
		blockers       int
		fact           string
	}{
		{"viewunknown", string(CoverageUnverified), 0, "view.definition"},
		{"viewbroken", string(CoverageComplete), 1, ""},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: sqlText, Dialect: DialectMySQL, Schema: "g", ConfigPath: path,
				MetadataProvider: t18FirstProvider{sourceMode: tc.mode},
			})
			if err != nil || string(result.Coverage.Status) != tc.coverage || result.Summary.Blockers != tc.blockers ||
				len(result.Statements) != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			if tc.fact != "" {
				if len(result.Statements[0].EvidenceGaps) != 1 ||
					result.Statements[0].EvidenceGaps[0].RuleID != "ddl.table.denylist.forbid" ||
					!reflect.DeepEqual(result.Statements[0].EvidenceGaps[0].RequiredFacts, []string{tc.fact}) {
					t.Fatalf("gaps=%#v", result.Statements[0].EvidenceGaps)
				}
			} else {
				finding := result.Statements[0].Findings[0]
				if finding.RuleID != "ddl.table.denylist.forbid" || finding.Metadata["table"] != "t" ||
					finding.Location == nil || finding.Location.Column != strings.LastIndex(sqlText, "t;")+1 {
					t.Fatalf("finding=%#v", finding)
				}
			}
			raw, err := json.Marshal(result)
			if err != nil || strings.Contains(string(raw), "t18_private_hidden") {
				t.Fatalf("private dependency leaked: err=%v result=%s", err, raw)
			}
		})
	}
	protectedPath := filepath.Join(t.TempDir(), "protected.yaml")
	if err := os.WriteFile(protectedPath, []byte(strings.ReplaceAll(config.String(), "t18_private_hidden", "protected_v")), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Audit(context.Background(), Request{
		SQL: "DROP VIEW v,protected_v;", Dialect: DialectMySQL, Schema: "g",
		ConfigPath: protectedPath, MetadataProvider: t18FirstProvider{extraView: "protected_v"},
	})
	if err != nil || result.Coverage.Status != CoverageComplete || result.Summary.Blockers != 1 ||
		len(result.Statements) != 1 || len(result.Statements[0].Findings) != 1 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	finding := result.Statements[0].Findings[0]
	if finding.RuleID != "ddl.table.denylist.forbid" || finding.Location == nil ||
		finding.Location.Column != 13 || finding.Metadata["table"] != "protected_v" || finding.Metadata["schema"] != "g" {
		t.Fatalf("nonfirst protected target=%#v", finding)
	}
}

func TestT18ReviewUncertainAlterLeavesSourceGap(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;\nCREATE VIEW next_v AS SELECT id FROM v;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18ReviewPolicy(t, true),
		MetadataProvider: t18FirstProvider{sourceMode: "unknown"},
	})
	if err != nil || len(result.Statements) != 2 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	found := false
	for _, gap := range result.Statements[1].EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			found = found || gap.RuleID == "ddl.view.source.valid" && fact == "view.definition"
		}
	}
	if !found || result.Statements[1].Coverage.Status != CoverageUnverified {
		t.Fatalf("following source was falsely vouched: %#v", result.Statements[1])
	}
}

func TestT18ReviewUncertainSecurityConvergence(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SQL SECURITY DEFINER VIEW v AS SELECT id FROM t;\nALTER VIEW v AS SELECT id FROM u;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18ReviewPolicy(t, true),
		MetadataProvider: t18FirstProvider{sourceMode: "unknown"},
	})
	if err != nil || len(result.Statements) != 2 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	stmt := result.Statements[1]
	if stmt.Coverage.Status != CoverageComplete || len(stmt.EvidenceGaps) != 0 ||
		len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.view.security.definer.warn" ||
		stmt.Findings[0].Level != LevelWarning {
		t.Fatalf("unchanged security was withheld: %#v", stmt)
	}
}

func TestT18PublicAlterViewFirstPath(t *testing.T) {
	const sql = "ALTER VIEW v AS SELECT id FROM t;"
	result, err := Audit(context.Background(), Request{
		SQL: sql, Dialect: DialectMySQL, Schema: "g", TargetVersion: "8.0.46",
		ConfigPath: t18FirstPolicy(t), MetadataProvider: t18FirstProvider{},
	})
	proof, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("T18_PUBLIC_PROOF %s", proof)
	if err != nil || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("audit error=%v diagnostics=%#v unsupported=%#v", err, result.Diagnostics, result.Unsupported)
	}
	if len(result.Statements) != 1 || result.Statements[0].Index != 0 ||
		result.Statements[0].Kind != "ddl" || strings.TrimSuffix(strings.TrimSpace(result.Statements[0].RawSQL), ";") != strings.TrimSuffix(sql, ";") {
		t.Fatalf("statements=%#v", result.Statements)
	}
	if result.Coverage.Status != CoverageComplete || result.Statements[0].Coverage.Status != CoverageComplete {
		t.Fatalf("coverage=%#v statement=%#v", result.Coverage, result.Statements[0].Coverage)
	}
	if result.Verdict != VerdictReview || len(result.Statements[0].Findings) != 1 ||
		result.Statements[0].Findings[0].RuleID != "ddl.view.security.definer.warn" ||
		result.Statements[0].Findings[0].Level != LevelWarning {
		t.Fatalf("verdict=%q findings=%#v", result.Verdict, result.Statements[0].Findings)
	}
}

func TestT18PublicInvalidViewSelect(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "ALTER VIEW v AS SELECT FROM t;", Dialect: DialectMySQL,
		ConfigPath: t18FirstPolicy(t),
	})
	if err == nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "parser_error" {
		t.Fatalf("err=%v diagnostics=%#v", err, result.Diagnostics)
	}
}

func TestT18PublicCreateViewDefaultForbid(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL,
	})
	if err != nil || len(result.Statements) != 1 {
		t.Fatalf("err=%v statements=%#v", err, result.Statements)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.create.forbid" {
			return
		}
	}
	t.Fatalf("missing create-view forbid: %#v", result.Statements[0].Findings)
}

func TestT18PublicAlterViewDefaultForbid(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
		MetadataProvider: t18FirstProvider{},
	})
	if err != nil || len(result.Statements) != 1 || result.Coverage.Status != CoverageComplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	found := map[string]bool{}
	for _, finding := range result.Statements[0].Findings {
		found[finding.RuleID] = true
	}
	if result.Verdict != VerdictReject || !found["ddl.view.create.forbid"] || !found["ddl.view.security.definer.warn"] {
		t.Fatalf("verdict=%s findings=%#v", result.Verdict, result.Statements[0].Findings)
	}
}

func TestT18PublicSourceEvidence(t *testing.T) {
	for _, tc := range []struct {
		mode, wantFact string
		wantInvalid    bool
	}{
		{mode: "missing", wantInvalid: true},
		{mode: "unknown", wantFact: "table.exists"},
		{mode: "kind", wantFact: "relation.kind"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
				ConfigPath: t18FirstPolicy(t), MetadataProvider: t18FirstProvider{sourceMode: tc.mode},
			})
			if err != nil || len(result.Statements) != 1 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			stmt := result.Statements[0]
			if tc.wantInvalid {
				found := false
				for _, finding := range stmt.Findings {
					found = found || finding.RuleID == "ddl.view.source.valid" && finding.Level == LevelBlocker
				}
				if !found || result.Coverage.Status != CoverageComplete {
					t.Fatalf("result=%#v", result)
				}
				return
			}
			found := false
			for _, gap := range stmt.EvidenceGaps {
				if gap.RuleID == "ddl.view.source.valid" && gap.ReasonCode == "unknown_table_state" {
					for _, fact := range gap.RequiredFacts {
						found = found || fact == tc.wantFact
					}
				}
			}
			if !found || result.Coverage.Status != CoverageUnverified {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}

func TestT18PublicViewPrivacyAndError(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER VIEW v AS SELECT id FROM t WHERE id = 't18_private_literal';",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FirstPolicy(t),
		MetadataProvider: t18FirstProvider{},
	})
	if err != nil || len(result.Statements) != 1 || result.Coverage.Status != CoverageComplete {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	payload, err := json.Marshal(struct {
		Findings    []Finding
		Gaps        []EvidenceGap
		Diagnostics any
	}{result.Statements[0].Findings, result.Statements[0].EvidenceGaps, result.Diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"t18_private_literal", "t18_private_principal", "t18_private_external"} {
		if strings.Contains(string(payload), sentinel) {
			t.Fatalf("sensitive value leaked: %s", payload)
		}
	}
	named, namedErr := Audit(context.Background(), Request{
		SQL:     "ALTER DEFINER='t18_private_named'@'t18_private_host' SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FirstPolicy(t),
		MetadataProvider: t18FirstProvider{},
	})
	if namedErr != nil || len(named.Statements) != 1 {
		t.Fatalf("named definer result=%#v err=%v", named, namedErr)
	}
	warned := false
	for _, finding := range named.Statements[0].Findings {
		warned = warned || finding.RuleID == "ddl.view.security.definer.warn"
	}
	if !warned {
		t.Fatalf("named principal did not trigger review: %#v", named)
	}
	namedPayload, err := json.Marshal(struct {
		Findings    []Finding
		Gaps        []EvidenceGap
		Diagnostics any
	}{named.Statements[0].Findings, named.Statements[0].EvidenceGaps, named.Diagnostics})
	if err != nil || strings.Contains(string(namedPayload), "t18_private_named") || strings.Contains(string(namedPayload), "t18_private_host") {
		t.Fatalf("named definer leaked: %s err=%v", namedPayload, err)
	}
	cause := errors.New("t18_private_provider_error")
	_, err = Audit(context.Background(), Request{
		SQL: "ALTER VIEW v AS SELECT id FROM t;", Dialect: DialectMySQL, Schema: "g",
		ConfigPath: t18FirstPolicy(t), MetadataProvider: t18FirstProvider{sourceMode: "error", readErr: cause},
	})
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "t18_private_provider_error") {
		t.Fatalf("unbounded or lost provider error: %v", err)
	}
}

func TestT18PublicUnmodeledViewShapeRemainsIncomplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules-off.yaml")
	config := "rules:\n" +
		"  ddl.view.create.forbid:\n    enabled: false\n" +
		"  ddl.view.target.valid:\n    enabled: false\n" +
		"  ddl.view.source.valid:\n    enabled: false\n" +
		"  ddl.view.security.definer.warn:\n    enabled: false\n"
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{"CREATE VIEW a AS SELECT t.id FROM t NATURAL JOIN u;"} {
		result, err := Audit(context.Background(), Request{
			SQL: sql, Dialect: DialectMySQL, Schema: "g",
			ConfigPath: path, MetadataProvider: t18FirstProvider{},
		})
		if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
			len(result.Statements) != 1 || len(result.Unsupported) == 0 {
			t.Fatalf("unmodeled shape falsely completed: sql=%q err=%v result=%#v", sql, err, result)
		}
	}
}

func TestT18FullUnmodeledShapeIsNotMetadataGap(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE VIEW next_v AS SELECT t.id FROM t NATURAL JOIN u;",
		Dialect: DialectMySQL, Schema: "g", ConfigPath: t18ReviewPolicy(t, false),
		MetadataProvider: t18FirstProvider{},
	})
	if !errors.Is(err, ErrUnsupportedStatement) || result.Coverage.Status != CoverageIncomplete ||
		len(result.Unsupported) != 1 || len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}

func TestT18PublicInvalidSelectRetainsSiblings(t *testing.T) {
	sql := "CREATE VIEW a AS SELECT id FROM t;\nALTER VIEW v AS SELECT FROM t;\nCREATE VIEW b AS SELECT id FROM t;"
	result, err := Audit(context.Background(), Request{SQL: sql, Dialect: DialectMySQL, Schema: "g", ConfigPath: t18FirstPolicy(t), MetadataProvider: t18FirstProvider{}})
	if err == nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "parser_error" ||
		result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Column != 1 || len(result.Statements) != 2 ||
		result.Statements[0].Index != 0 || result.Statements[1].Index != 1 ||
		!strings.Contains(result.Statements[0].RawSQL, "VIEW a") || !strings.Contains(result.Statements[1].RawSQL, "VIEW b") {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	foundLineThree := false
	for _, finding := range result.Statements[1].Findings {
		if finding.Location != nil && finding.Location.Line == 3 && finding.Location.Column == 1 {
			foundLineThree = true
		}
	}
	if !foundLineThree {
		t.Fatalf("trailing sibling lost original source location: %#v", result.Statements[1])
	}
}
