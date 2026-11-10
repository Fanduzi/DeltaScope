package tidbparser

// mysql_alter_instance.go recognizes the MySQL-only ALTER INSTANCE
// subaction the locked TiDB grammar cannot produce —
// ALTER INSTANCE ROTATE INNODB MASTER KEY. ROTATE is T26/#105
// inventory scope, not a deferred extension: the DDL-ALTER-INSTANCE-EXT
// tracking label holds only the five other subactions that remain
// unclaimed. The recognizer claims only the whole-input documented
// shape; anything else falls through to the base parser so RELOAD TLS
// variants keep their native AST path and unknown subactions keep the
// generic unhandled boundary.

import (
	"strings"

	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysqlAlterInstanceStatement is the adapter-owned carrier for one
// MySQL-only ALTER INSTANCE subaction. The embedded node is a
// fabricated *ast.AlterInstanceStmt — a real ast type with only shared
// members set — so classify and the coverage-boundary paths keep their
// existing shape. feature names the precise subaction.
type mysqlAlterInstanceStatement struct {
	ast.StmtNode
	feature string
	// mysqlOnly marks a subaction MySQL's grammar owns — under TiDB the
	// statement is a vendor boundary rather than a named unaudited one.
	mysqlOnly bool
}

// parseMySQLAlterInstance recognizes the whole-input
// `ALTER INSTANCE ROTATE INNODB MASTER KEY` form — the only MySQL-only
// subaction in the assigned tracking scope. A false result means the
// input takes the ordinary grammar path.
func parseMySQLAlterInstance(sqlText string) (ast.StmtNode, bool) {
	toks := tokenizeSQL(sqlText)
	if len(toks) == 0 {
		return nil, false
	}
	if last := toks[len(toks)-1]; last.kind == tokPunct && last.text(sqlText) == ";" {
		toks = toks[:len(toks)-1]
	}
	if len(toks) != 6 {
		return nil, false
	}
	if !toks[0].wordIs(sqlText, "alter") || !toks[1].wordIs(sqlText, "instance") ||
		!toks[2].wordIs(sqlText, "rotate") || !toks[3].wordIs(sqlText, "innodb") ||
		!toks[4].wordIs(sqlText, "master") || !toks[5].wordIs(sqlText, "key") {
		return nil, false
	}
	node := &ast.AlterInstanceStmt{}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlAlterInstanceStatement{
		StmtNode:  node,
		feature:   "alter_instance.rotate_innodb_master_key",
		mysqlOnly: true,
	}, true
}
