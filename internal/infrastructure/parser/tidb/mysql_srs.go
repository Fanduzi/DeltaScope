package tidbparser

// mysql_srs.go recognizes the MySQL spatial reference system family —
// CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM [IF NOT EXISTS] srid
// [srs_attribute ...] and DROP SPATIAL REFERENCE SYSTEM [IF EXISTS]
// srid — which the locked TiDB grammar cannot produce (issue #105 T26).
// The recognizer claims only the pure documented shape: the whole
// input must parse; anything ambiguous falls through to the base
// parser so failures keep their original diagnostics, never a
// silently claimed half-parse.
//
// SRID validation: the identifier is an unsigned 32-bit integer.
// Negative and overflowing spellings are never claimed — the base
// grammar owns the syntax diagnostic; a uint conversion must never
// silently wrap invalid input into a recognized statement.
//
// Attribute payload privacy: DEFINITION and DESCRIPTION are free-form
// string literals — presence and byte length are recorded, never the
// text. NAME and ORGANIZATION identities are kept verbatim because the
// promised semantic checks consume them; IDENTIFIED BY keeps the
// numeric id.

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysqlSRSStatement is the adapter-owned carrier for one MySQL spatial
// reference system statement. The embedded node is a fabricated
// ast.StmtNode — a real ast type with only shared members set — so
// classify and the coverage-boundary paths keep their existing shape.
// The typed declaration rides beside it; extraction binds it.
type mysqlSRSStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.SpatialReferenceDeclaration
}

// parseMySQLSpatialReference recognizes a whole-input MySQL SRS
// statement and fabricates its carrier node. A false result means the
// input is not a pure MySQL-form statement and must take the ordinary
// grammar path — the recognizer never emits partial claims.
func parseMySQLSpatialReference(sqlText string) (ast.StmtNode, bool) {
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
	if len(toks) < 5 {
		return nil, false
	}
	// CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM ...
	if toks[0].wordIs(sqlText, "create") {
		i := 1
		orReplace := false
		if toks[i].wordIs(sqlText, "or") {
			if i+1 >= len(toks) || !toks[i+1].wordIs(sqlText, "replace") {
				return nil, false
			}
			orReplace = true
			i += 2
		}
		if i+2 >= len(toks) || !toks[i].wordIs(sqlText, "spatial") ||
			!toks[i+1].wordIs(sqlText, "reference") || !toks[i+2].wordIs(sqlText, "system") {
			return nil, false
		}
		i += 3
		ine := false
		if i < len(toks) && toks[i].wordIs(sqlText, "if") {
			// OR REPLACE and IF NOT EXISTS are contradictory forms —
			// upstream refuses the combination, so it stays unclaimed.
			if orReplace {
				return nil, false
			}
			if i+2 >= len(toks) || !toks[i+1].wordIs(sqlText, "not") || !toks[i+2].wordIs(sqlText, "exists") {
				return nil, false
			}
			ine = true
			i += 3
		}
		if i >= len(toks) {
			return nil, false
		}
		return mysqlSRSCreate(sqlText, orReplace, ine, toks[i], toks[i+1:])
	}
	// DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] srid
	if toks[0].wordIs(sqlText, "drop") && len(toks) >= 5 {
		if !toks[1].wordIs(sqlText, "spatial") || !toks[2].wordIs(sqlText, "reference") ||
			!toks[3].wordIs(sqlText, "system") {
			return nil, false
		}
		i := 4
		ifExists := false
		if toks[i].wordIs(sqlText, "if") {
			if i+1 >= len(toks) || !toks[i+1].wordIs(sqlText, "exists") {
				return nil, false
			}
			ifExists = true
			i += 2
		}
		if i >= len(toks) {
			return nil, false
		}
		return mysqlSRSDrop(sqlText, ifExists, toks[i], toks[i+1:])
	}
	return nil, false
}

// mysqlSRID reads the authored SRID as an unsigned 32-bit integer —
// digits only, range-checked by ParseUint. Anything else (signed forms,
// non-numeric text, overflow) fails the whole claim so the base grammar
// keeps its own diagnostic.
func mysqlSRID(sqlText string, tok sqlTok) (uint32, bool) {
	if tok.kind != tokWord || !isUnsignedDigits(tok.text(sqlText)) {
		return 0, false
	}
	v, err := strconv.ParseUint(tok.text(sqlText), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// mysqlSRSAttrs consumes the free-order srs_attribute members —
// NAME 'x', DEFINITION 'x', ORGANIZATION 'x' IDENTIFIED BY n, and
// DESCRIPTION 'x'. Payloads for DEFINITION/DESCRIPTION stay opaque
// (presence + byte length); NAME/ORGANIZATION identities are verbatim
// for the promised conflict checks. Repeated members are recorded, not
// rejected — the grammar admits the list and upstream adjudicates it.
func mysqlSRSAttrs(sqlText string, rest []sqlTok, decl *spec.SpatialReferenceDeclaration) bool {
	seen := map[string]bool{}
	dup := func(name string) {
		if seen[name] {
			decl.DuplicateAttrs = append(decl.DuplicateAttrs, name)
		}
		seen[name] = true
	}
	for i := 0; i < len(rest); {
		switch {
		case rest[i].wordIs(sqlText, "name"):
			if i+1 >= len(rest) || rest[i+1].kind != tokStr {
				return false
			}
			value, ok := mysqlSRSStringValue(sqlText, rest[i+1])
			if !ok {
				return false
			}
			dup("name")
			decl.Attrs.HasName = true
			decl.Attrs.Name = value
			decl.Attrs.NameDecodeComplete = mysqlSRSDecodeComplete(sqlText, rest[i+1], value)
			i += 2
		case rest[i].wordIs(sqlText, "definition"):
			if i+1 >= len(rest) || rest[i+1].kind != tokStr {
				return false
			}
			raw, ok := mysqlSRSStringValue(sqlText, rest[i+1])
			if !ok {
				return false
			}
			dup("definition")
			decl.Attrs.HasDefinition = true
			decl.Attrs.DefinitionByteLen = len(raw)
			decl.Attrs.DefHasNewline = strings.ContainsAny(raw, "\r\n")
			i += 2
		case rest[i].wordIs(sqlText, "organization"):
			// ORGANIZATION owns a complete pair: the org name plus
			// mandatory `IDENTIFIED BY <u32>`. A missing or malformed
			// pair leaves the whole statement unclaimed — the grammar
			// has no bare-ORGANIZATION arm.
			if i+1 >= len(rest) || rest[i+1].kind != tokStr {
				return false
			}
			orgName, ok := mysqlSRSStringValue(sqlText, rest[i+1])
			if !ok {
				return false
			}
			dup("organization")
			decl.Attrs.HasOrganization = true
			decl.Attrs.OrgName = orgName
			decl.Attrs.OrgDecodeComplete = mysqlSRSDecodeComplete(sqlText, rest[i+1], orgName)
			i += 2
			if i+2 >= len(rest) || !rest[i].wordIs(sqlText, "identified") ||
				!rest[i+1].wordIs(sqlText, "by") ||
				rest[i+2].kind != tokWord || !isUnsignedDigits(rest[i+2].text(sqlText)) {
				return false
			}
			v, err := strconv.ParseUint(rest[i+2].text(sqlText), 10, 32)
			if err != nil {
				return false
			}
			decl.Attrs.HasIdentifiedBy = true
			decl.Attrs.OrgID = uint32(v)
			i += 3
		case rest[i].wordIs(sqlText, "description"):
			if i+1 >= len(rest) || rest[i+1].kind != tokStr {
				return false
			}
			raw, ok := mysqlSRSStringValue(sqlText, rest[i+1])
			if !ok {
				return false
			}
			dup("description")
			decl.Attrs.HasDescription = true
			decl.Attrs.DescriptionByteLen = len(raw)
			i += 2
		default:
			return false
		}
	}
	return true
}

// mysqlSRSStringValue reads a string literal's inner text with the
// doubled-quote unescape applied — an unclosed token fails rather than
// silently truncating. Identity attributes keep the result;
// DEFINITION/DESCRIPTION consume only derived facts (byte length,
// newline presence) and the payload itself is immediately discarded.
// The result is the unquoted spelling only — backslash escapes and
// charset introducers stay unresolved, so it is not a complete MySQL
// string decode.
func mysqlSRSStringValue(sqlText string, tok sqlTok) (string, bool) {
	raw := tok.text(sqlText)
	if !quotedTokenClosed(raw) {
		return "", false
	}
	inner := raw[1 : len(raw)-1]
	if raw[0] == '\'' {
		return strings.ReplaceAll(inner, "''", "'"), true
	}
	return strings.ReplaceAll(inner, `""`, `"`), true
}

// mysqlSRSDecodeComplete reports whether a decoded identity spelling
// provably equals the bytes MySQL stores — the frozen
// T26-SRS-CONFLICT-IDENTITY-01 query-eligibility rule (I1). It is true
// only while every condition holds on the ORIGINAL token: the literal
// was single-quoted (a double-quoted spelling is sql_mode-dependent —
// ANSI_QUOTES reads it as an identifier, which is unobservable), the
// decoded value carries no backslash byte (a backslash decode is
// sql_mode-dependent — NO_BACKSLASH_ESCAPES changes the stored bytes),
// and the decoded value is ASCII-only (the conservative product
// eligibility boundary — not a claim that MySQL accepts ASCII only).
// The token is the evidence: this never re-derives the quote kind from
// the decoded string.
func mysqlSRSDecodeComplete(sqlText string, tok sqlTok, value string) bool {
	raw := tok.text(sqlText)
	if len(raw) < 1 || raw[0] != '\'' {
		return false
	}
	if strings.ContainsRune(value, '\\') {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
}

// mysqlSRSCreate consumes the CREATE tail — SRID plus the attribute
// list. The whole tail must parse or nothing is claimed.
func mysqlSRSCreate(sqlText string, orReplace, ine bool, sridTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	srid, ok := mysqlSRID(sqlText, sridTok)
	if !ok {
		return nil, false
	}
	kind := spec.SRSKindCreate
	if orReplace {
		kind = spec.SRSKindCreateOrReplace
	}
	decl := &spec.SpatialReferenceDeclaration{Kind: kind, SRID: srid, IfNotExists: ine}
	if !mysqlSRSAttrs(sqlText, rest, decl) {
		return nil, false
	}
	node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(strconv.FormatUint(uint64(srid), 10))}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlSRSStatement{StmtNode: node, operation: spec.DDLOperationCreateSpatialReferenceSystem, declaration: decl}, true
}

// mysqlSRSDrop consumes `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] srid`
// — nothing may follow the SRID.
func mysqlSRSDrop(sqlText string, ifExists bool, sridTok sqlTok, rest []sqlTok) (ast.StmtNode, bool) {
	srid, ok := mysqlSRID(sqlText, sridTok)
	if !ok || len(rest) != 0 {
		return nil, false
	}
	decl := &spec.SpatialReferenceDeclaration{Kind: spec.SRSKindDrop, SRID: srid, IfExists: ifExists}
	node := &ast.DropResourceGroupStmt{ResourceGroupName: ast.NewCIStr(strconv.FormatUint(uint64(srid), 10))}
	node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	return &mysqlSRSStatement{StmtNode: node, operation: spec.DDLOperationDropSpatialReferenceSystem, declaration: decl}, true
}
