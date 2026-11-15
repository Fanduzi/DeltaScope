package tidbparser

// mysql_tablespace.go recognizes the MySQL tablespace family —
// CREATE/ALTER/DROP TABLESPACE, CREATE UNDO TABLESPACE, and the NDB-only
// CREATE/ALTER/DROP LOGFILE GROUP forms — which the locked TiDB grammar
// cannot produce (issue #104 T25). The recognizer claims only the pure
// MySQL shape: every member must come from the documented vocabulary and
// the whole input must parse; anything ambiguous falls through to the
// base parser so failures keep their original diagnostics, never a
// silently claimed half-parse.
//
// File payload privacy: ADD/DROP DATAFILE and UNDOFILE paths are opaque
// typed input. The recognizer counts the clauses and never stores,
// echoes, or interprets the string contents — the audit must not read,
// stat, or leak SQL-supplied paths.

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysqlTablespaceStatement is the adapter-owned carrier for one MySQL
// tablespace-family statement. The embedded node is a fabricated
// ast.StmtNode — a real ast type with only shared members set — so
// classify and the coverage-boundary paths keep their existing shape.
// The typed declaration rides beside it; extraction binds it.
type mysqlTablespaceStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.TablespaceDeclaration
}

// tablespaceScalarOptions is the scalar `KEY [=] value` vocabulary the
// MySQL tablespace forms accept. Values are either one word token or one
// string literal; both stay opaque declarations.
var tablespaceScalarOptions = map[string]bool{
	"file_block_size":  true,
	"autoextend_size":  true,
	"extent_size":      true,
	"initial_size":     true,
	"max_size":         true,
	"nodegroup":        true,
	"undo_buffer_size": true,
	"redo_buffer_size": true,
	"comment":          true,
	"engine_attribute": true,
}

// parseMySQLTablespace recognizes a whole-input MySQL tablespace-family
// statement and fabricates its carrier node. A false result means the
// input is not a pure MySQL-form statement and must take the ordinary
// grammar path — the recognizer never emits partial claims.
func parseMySQLTablespace(sqlText string) (ast.StmtNode, bool) {
	toks := tokenizeSQL(sqlText)
	if len(toks) == 0 {
		return nil, false
	}
	if last := toks[len(toks)-1]; last.kind == tokPunct && last.text(sqlText) == ";" {
		toks = toks[:len(toks)-1]
	}
	if len(toks) < 3 {
		return nil, false
	}
	switch {
	case toks[0].wordIs(sqlText, "create") && toks[1].wordIs(sqlText, "undo") &&
		len(toks) >= 5 && toks[2].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceCreate(sqlText, spec.TablespaceKindUndo, toks[3], toks[4:])
	case toks[0].wordIs(sqlText, "create") && toks[1].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceCreate(sqlText, spec.TablespaceKindGeneral, toks[2], toks[3:])
	case toks[0].wordIs(sqlText, "create") && toks[1].wordIs(sqlText, "logfile") &&
		len(toks) >= 4 && toks[2].wordIs(sqlText, "group"):
		return mysqlTablespaceCreate(sqlText, spec.TablespaceKindLogfileGroup, toks[3], toks[4:])
	case toks[0].wordIs(sqlText, "alter") && toks[1].wordIs(sqlText, "undo") &&
		len(toks) >= 4 && toks[2].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceAlterUndo(sqlText, toks[3], toks[4:])
	case toks[0].wordIs(sqlText, "alter") && toks[1].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceAlter(sqlText, spec.TablespaceKindGeneral, toks[2], toks[3:])
	case toks[0].wordIs(sqlText, "alter") && toks[1].wordIs(sqlText, "logfile") &&
		len(toks) >= 4 && toks[2].wordIs(sqlText, "group"):
		return mysqlTablespaceAlter(sqlText, spec.TablespaceKindLogfileGroup, toks[3], toks[4:])
	case toks[0].wordIs(sqlText, "drop") && toks[1].wordIs(sqlText, "undo") &&
		len(toks) >= 4 && toks[2].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceDrop(sqlText, spec.TablespaceKindUndo, spec.DDLOperationDropTablespace, toks[3], toks[4:])
	case toks[0].wordIs(sqlText, "drop") && toks[1].wordIs(sqlText, "tablespace"):
		return mysqlTablespaceDrop(sqlText, spec.TablespaceKindGeneral, spec.DDLOperationDropTablespace, toks[2], toks[3:])
	case toks[0].wordIs(sqlText, "drop") && toks[1].wordIs(sqlText, "logfile") &&
		len(toks) >= 4 && toks[2].wordIs(sqlText, "group"):
		return mysqlTablespaceDrop(sqlText, spec.TablespaceKindLogfileGroup, spec.DDLOperationDropLogfileGroup, toks[3], toks[4:])
	}
	return nil, false
}

// mysqlTablespaceName keeps the authored case: tablespace names are
// case-sensitive identifiers upstream, so folding to lowercase here would
// bind TS1 and ts1 into one identity — the same bug the shared identText
// helper commits by design for ordinary identifiers.
func mysqlTablespaceName(sqlText string, tok sqlTok) (string, bool) {
	if tok.kind != tokWord && tok.kind != tokQIdent {
		return "", false
	}
	name := tok.text(sqlText)
	if tok.kind == tokQIdent {
		name = strings.ReplaceAll(name[1:len(name)-1], "``", "`")
	}
	return name, name != ""
}

// mysqlTablespaceCreate consumes `CREATE <kind> <name> <members>`: file
// clauses, scalar options, ENCRYPTION, and ENGINE. The undo form requires
// its ADD DATAFILE member; general and logfile forms carry their own
// documented members.
func mysqlTablespaceCreate(sqlText, kind string, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, ok := mysqlTablespaceName(sqlText, nameTok)
	if !ok {
		return nil, false
	}
	decl := &spec.TablespaceDeclaration{Kind: kind, Name: name, Options: map[string]string{}}
	i := 0
	for i < len(rest) {
		switch {
		case rest[i].wordIs(sqlText, "add"):
			// ADD DATAFILE 'f' | ADD UNDOFILE 'f' — the path stays opaque.
			// The grammar carries at most one file clause, and each kind
			// takes its documented keyword: LOGFILE GROUP takes UNDOFILE,
			// general and undo take DATAFILE.
			if decl.FileCount > 0 {
				return nil, false
			}
			fileKeyword := "datafile"
			if kind == spec.TablespaceKindLogfileGroup {
				fileKeyword = "undofile"
			}
			if i+2 >= len(rest) || rest[i+2].kind != tokStr ||
				!rest[i+1].wordIs(sqlText, fileKeyword) {
				return nil, false
			}
			decl.FileCount++
			i += 3
		case rest[i].wordIs(sqlText, "encryption"):
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok || (value != "y" && value != "n") {
				return nil, false
			}
			on := value == "y"
			decl.Encryption = &on
			i = next
		case rest[i].wordIs(sqlText, "engine"):
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok {
				return nil, false
			}
			decl.Engine = value
			i = next
		case rest[i].wordIs(sqlText, "wait"):
			decl.Options["wait"] = ""
			i++
		default:
			key := strings.ToLower(rest[i].text(sqlText))
			if rest[i].kind != tokWord || !tablespaceScalarOptions[key] {
				return nil, false
			}
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok {
				return nil, false
			}
			decl.Options[key] = value
			i = next
		}
	}
	if decl.FileCount == 0 && kind != spec.TablespaceKindGeneral {
		// The undo and logfile-group forms require their file clause at
		// the grammar level — ADD DATAFILE / ADD UNDOFILE. A memberless
		// general CREATE TABLESPACE is grammar-valid since 8.0 (NDB
		// accepts it outright; InnoDB refuses it semantically and 5.7's
		// grammar never had it), so only those forms fall through to
		// the base grammar's own diagnostic rather than being claimed.
		return nil, false
	}
	node := &ast.CreateResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	operation := spec.DDLOperationCreateTablespace
	if kind == spec.TablespaceKindLogfileGroup {
		operation = spec.DDLOperationCreateLogfileGroup
	}
	return &mysqlTablespaceStatement{StmtNode: node, operation: operation, declaration: decl}, true
}

// mysqlTablespaceAlter consumes `ALTER <kind> <name> <member>` — one
// mutating member followed by optional ENGINE, matching the MySQL
// contract where ALTER carries a single action.
func mysqlTablespaceAlter(sqlText, kind string, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, ok := mysqlTablespaceName(sqlText, nameTok)
	if !ok || len(rest) == 0 {
		return nil, false
	}
	decl := &spec.TablespaceDeclaration{Kind: kind, Name: name, Options: map[string]string{}}
	i := 0
	switch {
	case rest[i].wordIs(sqlText, "add") || rest[i].wordIs(sqlText, "drop"):
		member := strings.ToLower(rest[i].text(sqlText))
		// Each kind takes its documented file keyword: LOGFILE GROUP
		// carries UNDOFILE members, general tablespaces carry DATAFILE.
		fileKeyword := "datafile"
		if kind == spec.TablespaceKindLogfileGroup {
			fileKeyword = "undofile"
		}
		if i+2 >= len(rest) || rest[i+2].kind != tokStr ||
			!rest[i+1].wordIs(sqlText, fileKeyword) {
			return nil, false
		}
		file := "datafile"
		if fileKeyword == "undofile" {
			file = "undo_file"
		}
		decl.AlterAction = member + "_" + file
		decl.FileCount = 1
		i += 3
	case rest[i].wordIs(sqlText, "rename"):
		if i+2 >= len(rest) || !rest[i+1].wordIs(sqlText, "to") {
			return nil, false
		}
		newName, ok := mysqlTablespaceName(sqlText, rest[i+2])
		if !ok {
			return nil, false
		}
		decl.AlterAction = "rename"
		decl.NewName = newName
		i += 3
	case rest[i].wordIs(sqlText, "encryption"):
		value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
		if !ok || (value != "y" && value != "n") {
			return nil, false
		}
		on := value == "y"
		decl.AlterAction = "encryption"
		decl.Encryption = &on
		i = next
	default:
		// The scalar members may also lead the member list — the
		// trailing-member loop below consumes them from position 0.
		// ENGINE-led ALTER is the documented `ALTER TABLESPACE name
		// ENGINE [=] engine` form whose only member is the engine
		// clause (T57 parser-upgrade candidate).
		if !rest[i].wordIs(sqlText, "autoextend_size") &&
			!rest[i].wordIs(sqlText, "engine_attribute") &&
			!rest[i].wordIs(sqlText, "engine") {
			return nil, false
		}
		if rest[i].wordIs(sqlText, "engine") {
			decl.AlterAction = "engine"
		}
	}
	// Optional trailing members: ENGINE and scalar options.
	for i < len(rest) {
		switch {
		case rest[i].wordIs(sqlText, "engine"):
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok {
				return nil, false
			}
			decl.Engine = value
			i = next
		case rest[i].wordIs(sqlText, "wait"):
			decl.Options["wait"] = ""
			i++
		case rest[i].wordIs(sqlText, "autoextend_size") || rest[i].wordIs(sqlText, "initial_size") ||
			rest[i].wordIs(sqlText, "engine_attribute"):
			key := strings.ToLower(rest[i].text(sqlText))
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok {
				return nil, false
			}
			decl.Options[key] = value
			i = next
		default:
			return nil, false
		}
	}
	node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	operation := spec.DDLOperationAlterTablespace
	if kind == spec.TablespaceKindLogfileGroup {
		operation = spec.DDLOperationAlterLogfileGroup
	}
	return &mysqlTablespaceStatement{StmtNode: node, operation: operation, declaration: decl}, true
}

// mysqlTablespaceAlterUndo consumes the whole `ALTER UNDO TABLESPACE
// name SET {ACTIVE|INACTIVE} [ENGINE [=] engine]` form — the only member
// the grammar carries, plus the documented optional ENGINE clause. The
// statement is recognized so it surfaces as a bounded unaudited
// feature rather than a parser diagnostic (deferred scope, T25-R1).
func mysqlTablespaceAlterUndo(sqlText string, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, ok := mysqlTablespaceName(sqlText, nameTok)
	if !ok || len(rest) < 2 {
		return nil, false
	}
	if !rest[0].wordIs(sqlText, "set") ||
		(!rest[1].wordIs(sqlText, "active") && !rest[1].wordIs(sqlText, "inactive")) {
		return nil, false
	}
	decl := &spec.TablespaceDeclaration{
		Kind:        spec.TablespaceKindUndo,
		Name:        name,
		AlterAction: "set_state",
		Options:     map[string]string{"set_state": strings.ToLower(rest[1].text(sqlText))},
	}
	if len(rest) > 2 {
		// The only legal trailing member is the ENGINE [=] engine_name
		// clause — anything else is real trailing garbage and stays
		// unclaimed so the base parser keeps its own diagnostic.
		if !rest[2].wordIs(sqlText, "engine") {
			return nil, false
		}
		value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, 3)
		if !ok || next != len(rest) {
			return nil, false
		}
		decl.Engine = value
	}
	node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlTablespaceStatement{StmtNode: node, operation: spec.DDLOperationAlterTablespace, declaration: decl}, true
}

// mysqlTablespaceDrop consumes `DROP <kind> <name> [ENGINE [=] e]`.
func mysqlTablespaceDrop(sqlText, kind string, operation spec.DDLOperation, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, ok := mysqlTablespaceName(sqlText, nameTok)
	if !ok {
		return nil, false
	}
	decl := &spec.TablespaceDeclaration{Kind: kind, Name: name, Options: map[string]string{}}
	i := 0
	for i < len(rest) {
		if rest[i].wordIs(sqlText, "engine") {
			value, next, ok := mysqlTablespaceOptionValue(sqlText, rest, i+1)
			if !ok {
				return nil, false
			}
			decl.Engine = value
			i = next
			continue
		}
		return nil, false
	}
	node := &ast.DropResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlTablespaceStatement{StmtNode: node, operation: operation, declaration: decl}, true
}

// mysqlTablespaceOptionValue consumes the optional `=` plus one value
// token — a word or a string literal — returning the lowercased scalar.
// The value is a declaration, never interpreted beyond its literal form.
func mysqlTablespaceOptionValue(sqlText string, toks []sqlTok, i int) (string, int, bool) {
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
		i++
	}
	if i >= len(toks) {
		return "", i, false
	}
	tok := toks[i]
	switch tok.kind {
	case tokWord:
		return strings.ToLower(tok.text(sqlText)), i + 1, true
	case tokStr:
		raw := tok.text(sqlText)
		if len(raw) >= 2 {
			return strings.ToLower(raw[1 : len(raw)-1]), i + 1, true
		}
	}
	return "", i, false
}
