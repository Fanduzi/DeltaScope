// Probe for T06-A15-BASELINE: controlled AuditSQL requests against the real
// parser/extractor/registry/evaluation chain, with a constructed
// MetadataProvider (explicitly synthetic — never a real database), plus raw
// parse/extract field observation for the five fixed SQLs.
// Read-only; built outside the repo; touches no database.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// ledgerProvider is a constructed MetadataProvider: returns one of three fixed
// fact shapes and records every call. Its data is synthetic input, never a
// real server observation.
type ledgerProvider struct {
	facts     *spec.InstanceFacts
	factsNil  bool // return (nil, nil) from LoadInstanceFacts
	snapshots map[string]*spec.TableSnapshot
	calls     []string
}

func (p *ledgerProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	p.calls = append(p.calls, fmt.Sprintf("LoadInstanceFacts(dialect=%s schema=%q)", dialect, schema))
	if p.factsNil {
		return nil, nil
	}
	return p.facts, nil
}

func (p *ledgerProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, fmt.Sprintf("LoadTableSnapshot(dialect=%s schema=%q table=%q)", dialect, schema, table))
	snap, ok := p.snapshots[schema+"."+table]
	if !ok {
		return nil, nil
	}
	return snap, nil
}

var sqls = map[string]string{
	"L_MINUS":         "CREATE TABLE t (c VARCHAR(65532)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
	"L_EQUAL":         "CREATE TABLE t (c VARCHAR(65533)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
	"L_PLUS":          "CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
	"OTHER_ENGINE":    "CREATE TABLE t (c VARCHAR(65534)) ENGINE=MyISAM DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
	"COLUMN_OVERRIDE": "CREATE TABLE t (c VARCHAR(16384) CHARACTER SET utf8mb4) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;",
}

func factsFull(dialect spec.Dialect) *spec.InstanceFacts {
	version := "8.4.10"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	return &spec.InstanceFacts{
		Version:                version,
		DefaultCharset:         "latin1",
		InnoDBDefaultRowFormat: "DYNAMIC",
	}
}

func probeFields(dialect spec.Dialect, label, sql string) {
	parser := tidbparser.New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		fmt.Printf("%s/%s parse error: %v\n", label, dialect, err)
		return
	}
	create, ok := result.Statements[0].(*ast.CreateTableStmt)
	if !ok {
		fmt.Printf("%s/%s statement type %T\n", label, dialect, result.Statements[0])
		return
	}
	fmt.Printf("=== %s dialect=%s statements=%d ast-options=%d\n", label, dialect, len(result.Statements), len(create.Options))
	for i, o := range create.Options {
		if o == nil {
			continue
		}
		fmt.Printf("  ast[%d] tp=%d str=%q uint=%d\n", i, o.Tp, o.StrValue, o.UintValue)
	}
	wrapped := tidbparser.WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		fmt.Printf("  extract error: %v\n", err)
		return
	}
	keys := make([]string, 0, len(statement.DDL.Options))
	for k := range statement.DDL.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  options=%v unextracted=%v\n", keys, statement.DDL.UnextractedOptions)
	for _, k := range keys {
		fmt.Printf("    Options[%q]=%q\n", k, statement.DDL.Options[k])
	}
	for i, c := range statement.DDL.Columns {
		fmt.Printf("  col[%d] name=%s type=%q length=%d charset=%q collation=%q\n",
			i, c.Name, c.Type, c.Length, c.Charset, c.Collation)
	}
	fmt.Printf("  pk=%v indexes=%d\n", statement.DDL.PrimaryKey, len(statement.DDL.Indexes))
}

func probeAudit(dialect spec.Dialect, label, factsLabel string, provider *ledgerProvider) {
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              sqls[label],
		Dialect:          dialect,
		ConfigPath:       "p-on.yaml",
		Schema:           "golden",
		MetadataProvider: provider,
	})
	fmt.Printf("=== %s dialect=%s facts=%s err=%v\n", label, dialect, factsLabel, err)
	if err == nil {
		raw, _ := json.Marshal(result)
		fmt.Printf("  result=%s\n", raw)
	}
	for i, c := range provider.calls {
		fmt.Printf("  ledger[%d] %s\n", i, c)
	}
}

func main() {
	dialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	order := []string{"L_MINUS", "L_EQUAL", "L_PLUS", "OTHER_ENGINE", "COLUMN_OVERRIDE"}
	fmt.Println("##### FIELD PROBE #####")
	for _, d := range dialects {
		for _, label := range order {
			probeFields(d, label, sqls[label])
		}
	}

	fmt.Println("##### AUDITSQL PROBE #####")
	absent := func(schema string) map[string]*spec.TableSnapshot {
		return map[string]*spec.TableSnapshot{
			schema + ".t": {
				Schema: schema, Table: &spec.Table{Schema: schema, Name: "t"},
				Exists: false, Columns: []spec.Column{}, Indexes: []spec.Index{}, Constraints: []spec.Constraint{},
			},
		}
	}
	for _, d := range dialects {
		for _, step := range []struct{ sql, facts string }{
			{"L_MINUS", "F_FULL"},
			{"L_EQUAL", "F_FULL"},
			{"L_PLUS", "F_FULL"},
			{"L_PLUS", "F_EMPTY"},
			{"L_PLUS", "F_NIL"},
			{"OTHER_ENGINE", "F_FULL"},
			{"COLUMN_OVERRIDE", "F_FULL"},
		} {
			p := &ledgerProvider{snapshots: absent("golden")}
			switch step.facts {
			case "F_FULL":
				p.facts = factsFull(d)
			case "F_EMPTY":
				p.facts = &spec.InstanceFacts{}
			case "F_NIL":
				p.factsNil = true
			}
			probeAudit(d, step.sql, step.facts, p)
		}
	}
}
