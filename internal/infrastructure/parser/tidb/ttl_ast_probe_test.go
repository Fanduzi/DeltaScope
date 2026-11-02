package tidbparser

import (
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
)

// T19: the extractor depends on PingCAP's normalized TTL option shape. Pin the
// AST facts it consumes — a dedicated remove action, bool/string members, and
// restored interval expression text including charset introducers.
func TestT19TTLASTShape(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE TABLE t (created_at DATETIME) TTL=created_at+INTERVAL 30 DAY",
		"CREATE TABLE t (created_at DATETIME) /*T![ttl] TTL=created_at+INTERVAL 30 DAY */",
	} {
		node, err := parser.New().ParseOneStmt(sql, "", "")
		if err != nil {
			t.Fatalf("parse %q: %v", sql, err)
		}
		create, ok := node.(*ast.CreateTableStmt)
		if !ok || len(create.Options) != 1 || create.Options[0].Tp != ast.TableOptionTTL {
			t.Fatalf("%q produced %#v, want one TTL table option", sql, node)
		}
		option := create.Options[0]
		if option.ColumnName == nil || option.ColumnName.Name.L != "created_at" || option.Value == nil || option.TimeUnitValue == nil || !strings.EqualFold(option.TimeUnitValue.Unit.String(), "DAY") {
			t.Fatalf("TTL option shape = %#v", option)
		}
	}
}

func TestT19TTLAlterASTShape(t *testing.T) {
	t.Parallel()
	node, err := parser.New().ParseOneStmt(
		"ALTER TABLE t TTL=created_at+INTERVAL '1:3' DAY_SECOND, TTL_ENABLE='ON', TTL_JOB_INTERVAL='5h'", "", "")
	if err != nil {
		t.Fatalf("parse alter ttl: %v", err)
	}
	alter, ok := node.(*ast.AlterTableStmt)
	if !ok || len(alter.Specs) != 3 {
		t.Fatalf("alter specs = %#v, want three option specs", node)
	}
	definition := alter.Specs[0].Options[0]
	if definition.Tp != ast.TableOptionTTL || definition.Value == nil {
		t.Fatalf("TTL definition option = %#v", definition)
	}
	var restored strings.Builder
	if err := definition.Value.Restore(format.NewRestoreCtx(format.RestoreStringSingleQuotes|format.RestoreNameBackQuotes, &restored)); err != nil {
		t.Fatalf("restore interval expr: %v", err)
	}
	if restored.String() != "_utf8mb4'1:3'" {
		t.Fatalf("restored interval expr = %q, want charset-introduced quoted literal", restored.String())
	}
	enable := alter.Specs[1].Options[0]
	if enable.Tp != ast.TableOptionTTLEnable || !enable.BoolValue {
		t.Fatalf("TTL_ENABLE option = %#v", enable)
	}
	job := alter.Specs[2].Options[0]
	if job.Tp != ast.TableOptionTTLJobInterval || job.StrValue != "5h" {
		t.Fatalf("TTL_JOB_INTERVAL option = %#v", job)
	}
}

func TestT19TTLRemoveASTShape(t *testing.T) {
	t.Parallel()
	node, err := parser.New().ParseOneStmt("ALTER TABLE t REMOVE TTL", "", "")
	if err != nil {
		t.Fatalf("parse remove ttl: %v", err)
	}
	alter, ok := node.(*ast.AlterTableStmt)
	if !ok || len(alter.Specs) != 1 || alter.Specs[0].Tp != ast.AlterTableRemoveTTL || len(alter.Specs[0].Options) != 0 {
		t.Fatalf("REMOVE TTL shape = %#v, want dedicated option-free spec", node)
	}
}

func TestT19TTLQuotedWholeClauseStaysParserError(t *testing.T) {
	t.Parallel()
	if _, err := parser.New().ParseOneStmt("ALTER TABLE t TTL = 'created_at + INTERVAL 30 DAY'", "", ""); err == nil {
		t.Fatal("quoted whole TTL clause parsed; want parser error")
	}
}
