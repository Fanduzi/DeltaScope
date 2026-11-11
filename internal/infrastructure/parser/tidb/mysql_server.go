package tidbparser

// mysql_server.go recognizes the MySQL SERVER family —
// CREATE SERVER ... FOREIGN DATA WRAPPER ... OPTIONS (...),
// ALTER SERVER ... OPTIONS (...), and DROP SERVER [IF EXISTS] —
// which the locked TiDB grammar cannot produce (issue #105 T26).
// The recognizer claims only the pure documented shape: the whole
// input must parse; anything ambiguous falls through to the base
// parser so failures keep their original diagnostics, never a
// silently claimed half-parse.
//
// Connection-option privacy: OPTIONS carries credentials and
// connection coordinates. The recognizer records option keys, value
// kinds, and counts only — literal values are consumed as opaque
// tokens and never stored, echoed, or interpreted.

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysqlServerStatement is the adapter-owned carrier for one MySQL
// SERVER-family statement. The embedded node is a fabricated
// ast.StmtNode — a real ast type with only shared members set — so
// classify and the coverage-boundary paths keep their existing shape.
// The typed declaration rides beside it; extraction binds it.
type mysqlServerStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.ServerDeclaration
}

// serverOptionValueKind records the documented literal kind each
// OPTIONS key accepts — char for a string literal, numeric for an
// unsigned literal (PORT is the only numeric member).
var serverOptionValueKind = map[string]string{
	"host":     "char",
	"database": "char",
	"user":     "char",
	"password": "char",
	"socket":   "char",
	"owner":    "char",
	"port":     "numeric",
}

// The documented 64-character server-name bound (mysql.servers
// .server_name is CHAR(64)) is a decoded-length check — outside the
// recognizer's scope. The declaration records only the unquoted
// spelling's rune count (spec.ServerDeclaration.NameSpellingRuneCount);
// comparing decoded length against the bound stays a future semantic
// step that first owes the full string decode.

// parseMySQLServer recognizes a whole-input MySQL SERVER-family
// statement and fabricates its carrier node. A false result means the
// input is not a pure MySQL-form statement and must take the ordinary
// grammar path — the recognizer never emits partial claims.
func parseMySQLServer(sqlText string) (ast.StmtNode, bool) {
	toks, unclosed := tokenizeSQLState(sqlText)
	if unclosed {
		return nil, false
	}
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
	case toks[0].wordIs(sqlText, "create") && toks[1].wordIs(sqlText, "server"):
		return mysqlServerCreate(sqlText, toks[2], toks[3:])
	case toks[0].wordIs(sqlText, "alter") && toks[1].wordIs(sqlText, "server"):
		return mysqlServerAlter(sqlText, toks[2], toks[3:])
	case toks[0].wordIs(sqlText, "drop") && toks[1].wordIs(sqlText, "server"):
		if toks[2].wordIs(sqlText, "if") {
			if len(toks) >= 5 && toks[3].wordIs(sqlText, "exists") {
				return mysqlServerDrop(sqlText, toks[4], toks[5:], true)
			}
			return nil, false
		}
		return mysqlServerDrop(sqlText, toks[2], toks[3:], false)
	}
	return nil, false
}

// mysqlServerName reads the authored server identity — an identifier or
// a string literal (MySQL documents both spellings). Case is preserved:
// server names are object identities stored verbatim. An unquoted word
// must survive upstream's ident rule — reserved keywords and pure-digit
// tokens stay unclaimed. Quoted tokens must be closed: an unterminated
// literal is never silently truncated into a name. String/quoted-identity
// values come back unquoted with doubled-quote escapes resolved — that
// is not a complete MySQL string decode (backslash escapes and charset
// introducers stay unresolved, so the fact is the unquoted spelling).
//
// The second result marks whether the name is a complete decoded
// catalog identity: identifier token forms (word, backquoted) decode
// fully — doubled-backtick unescape is the entire identifier decode —
// while a string-literal spelling stays unprovable under SQL-mode and
// charset-introducer variance (T26/#105 provider round).
func mysqlServerName(sqlText string, tok sqlTok) (name string, ident bool, ok bool) {
	switch tok.kind {
	case tokWord:
		name = tok.text(sqlText)
		if name == "" {
			return "", false, false
		}
		if _, reserved := mysqlResourceGroupReservedWords[strings.ToLower(name)]; reserved {
			return "", false, false
		}
		if isUnsignedDigits(name) {
			return "", false, false
		}
		return name, true, true
	case tokQIdent:
		raw := tok.text(sqlText)
		if !quotedTokenClosed(raw) {
			return "", false, false
		}
		name = strings.ReplaceAll(raw[1:len(raw)-1], "``", "`")
		return name, true, name != ""
	case tokStr:
		raw := tok.text(sqlText)
		if !quotedTokenClosed(raw) {
			return "", false, false
		}
		quote := raw[0]
		inner := raw[1 : len(raw)-1]
		if quote == '\'' {
			inner = strings.ReplaceAll(inner, "''", "'")
		} else {
			inner = strings.ReplaceAll(inner, `""`, `"`)
		}
		return inner, false, inner != ""
	}
	return "", false, false
}

// mysqlServerCreate consumes
// `CREATE SERVER name FOREIGN DATA WRAPPER wrapper OPTIONS (members)`.
// The name may be an identifier or a string literal; the wrapper is an
// identifier. The whole member list must parse or nothing is claimed.
func mysqlServerCreate(sqlText string, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, nameIdent, ok := mysqlServerName(sqlText, nameTok)
	if !ok || len(rest) < 4 {
		return nil, false
	}
	if !rest[0].wordIs(sqlText, "foreign") || !rest[1].wordIs(sqlText, "data") ||
		!rest[2].wordIs(sqlText, "wrapper") {
		return nil, false
	}
	wrapper, wrapperIdent, ok := mysqlServerName(sqlText, rest[3])
	if !ok {
		return nil, false
	}
	decl := &spec.ServerDeclaration{
		Kind:                  spec.ServerKindCreate,
		Name:                  name,
		NameIdent:             nameIdent,
		Wrapper:               wrapper,
		WrapperIdent:          wrapperIdent,
		NameSpellingRuneCount: utf8.RuneCountInString(name),
	}
	if !mysqlServerOptions(sqlText, rest[4:], decl) {
		return nil, false
	}
	node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlServerStatement{StmtNode: node, operation: spec.DDLOperationCreateServer, declaration: decl}, true
}

// mysqlServerAlter consumes `ALTER SERVER name OPTIONS (members)`.
func mysqlServerAlter(sqlText string, nameTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	name, nameIdent, ok := mysqlServerName(sqlText, nameTok)
	if !ok {
		return nil, false
	}
	decl := &spec.ServerDeclaration{
		Kind:                  spec.ServerKindAlter,
		Name:                  name,
		NameIdent:             nameIdent,
		NameSpellingRuneCount: utf8.RuneCountInString(name),
	}
	if !mysqlServerOptions(sqlText, rest, decl) {
		return nil, false
	}
	node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlServerStatement{StmtNode: node, operation: spec.DDLOperationAlterServer, declaration: decl}, true
}

// mysqlServerDrop consumes `DROP SERVER [IF EXISTS] name` — the whole
// input after the name must be empty or the claim fails.
func mysqlServerDrop(sqlText string, nameTok sqlTok, rest []sqlTok, ifExists bool) (ast.StmtNode, bool) {
	name, nameIdent, ok := mysqlServerName(sqlText, nameTok)
	if !ok || len(rest) != 0 {
		return nil, false
	}
	decl := &spec.ServerDeclaration{
		Kind:                  spec.ServerKindDrop,
		Name:                  name,
		NameIdent:             nameIdent,
		IfExists:              ifExists,
		NameSpellingRuneCount: utf8.RuneCountInString(name),
	}
	node := &ast.DropResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlServerStatement{StmtNode: node, operation: spec.DDLOperationDropServer, declaration: decl}, true
}

// mysqlServerOptions consumes `OPTIONS (member [, member] ...)` where
// each member is `KEY literal` from the documented seven-key vocabulary.
// The grammar requires at least one member — an empty list, a trailing
// or doubled comma, or a missing value leaves the whole statement
// unclaimed so the base parser owns the diagnostic. Values are consumed
// as opaque tokens — only key and value kind are recorded. Duplicate
// keys are recorded, not rejected: the grammar admits the list and
// upstream adjudicates the repetition.
func mysqlServerOptions(sqlText string, rest []sqlTok, decl *spec.ServerDeclaration) bool {
	if len(rest) < 2 || !rest[0].wordIs(sqlText, "options") {
		return false
	}
	if rest[1].kind != tokPunct || rest[1].text(sqlText) != "(" {
		return false
	}
	if rest[len(rest)-1].kind != tokPunct || rest[len(rest)-1].text(sqlText) != ")" {
		return false
	}
	inner := rest[2 : len(rest)-1]
	if len(inner) == 0 {
		return false
	}
	seen := map[string]bool{}
	for i := 0; i < len(inner); {
		tok := inner[i]
		if tok.kind != tokWord {
			return false
		}
		key := strings.ToLower(tok.text(sqlText))
		wantKind, known := serverOptionValueKind[key]
		if !known {
			return false
		}
		if seen[key] {
			decl.DuplicateKeys = append(decl.DuplicateKeys, key)
		}
		seen[key] = true
		i++
		if i >= len(inner) {
			return false
		}
		value := inner[i]
		option := spec.ServerOption{Key: key, ValueKind: wantKind}
		switch wantKind {
		case "char":
			if value.kind != tokStr || !quotedTokenClosed(value.text(sqlText)) {
				return false
			}
		case "numeric":
			if value.kind != tokWord || !isUnsignedDigits(value.text(sqlText)) {
				return false
			}
			number, err := strconv.ParseUint(value.text(sqlText), 10, 64)
			if err != nil {
				// An overflowing digit run stays a parser rejection —
				// the recognizer never widens its claim to produce a
				// RangeState.unproven member (T26-SERVER-CREATE-01).
				return false
			}
			option.RangeState = spec.ServerOptionRangeInRange
			if number > 9999 {
				// Parsed but outside the documented 0–9999 bound —
				// audited, and the OPTIONS premise stays owed.
				option.RangeState = spec.ServerOptionRangeOutOfRange
			}
		}
		decl.Options = append(decl.Options, option)
		i++
		if i < len(inner) {
			if inner[i].kind != tokPunct || inner[i].text(sqlText) != "," {
				return false
			}
			i++
			if i >= len(inner) {
				return false
			}
		}
	}
	return true
}
