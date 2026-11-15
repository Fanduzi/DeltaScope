package tidbparser

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysql_routine.go — whole-input recognizer for the MySQL stored-routine
// family (issue #106 T27): CREATE PROCEDURE|FUNCTION including compound
// BEGIN…END bodies the upstream grammar cannot represent. The recognizer
// splits the body into member segments at top-level semicolons only —
// members that are themselves compound constructs (IF/CASE/LOOP/WHILE/
// REPEAT/nested BEGIN) or routine-local declarations stay unmodeled
// members rather than being split into false SQL statements.

type routineStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.RoutineDeclaration
	inner       []routineInnerMember
}

// routineInnerMember is one body member: either a parsed SQL statement
// or an unmodeled compound/declaration segment kept with its reason.
type routineInnerMember struct {
	node   ast.StmtNode
	raw    string
	offset int // byte offset of the member's first byte inside the whole input
	err    string
}

var routineCompoundOpeners = map[string]bool{
	"begin": true, "if": true, "case": true, "loop": true, "while": true, "repeat": true,
}

// parseMySQLRoutine claims whole-input CREATE PROCEDURE|FUNCTION forms.
// Anything ambiguous falls through to the real grammar; a claimed
// statement never produces a partial node (T27/#106).
func parseMySQLRoutine(sqlText string) (ast.StmtNode, []error, bool) {
	toks, unclosed := tokenizeSQLState(sqlText)
	if unclosed || len(toks) < 3 {
		return nil, nil, false
	}
	for len(toks) > 0 {
		last := toks[len(toks)-1]
		if last.kind != tokPunct || last.text(sqlText) != ";" {
			break
		}
		toks = toks[:len(toks)-1]
	}
	if len(toks) < 3 {
		return nil, nil, false
	}
	switch {
	case toks[0].wordIs(sqlText, "create"):
		decl := &spec.RoutineDeclaration{}
		return parseMySQLCreateRoutine(sqlText, toks, decl)
	case toks[0].wordIs(sqlText, "alter"):
		return parseMySQLAlterRoutine(sqlText, toks)
	case toks[0].wordIs(sqlText, "drop"):
		return parseMySQLDropRoutine(sqlText, toks)
	}
	return nil, nil, false
}

// parseMySQLCreateRoutine claims CREATE PROCEDURE|FUNCTION forms.
// Anything ambiguous falls through to the real grammar; a claimed
// statement never produces a partial node (T27/#106).
func parseMySQLCreateRoutine(sqlText string, toks []sqlTok, decl *spec.RoutineDeclaration) (ast.StmtNode, []error, bool) {
	// Header: CREATE [DEFINER=...] [AGGREGATE] PROCEDURE|FUNCTION …
	i := 1
	if i < len(toks) && toks[i].wordIs(sqlText, "definer") {
		i++
		if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
			i++
		}
		start := i
		for i < len(toks) && !toks[i].wordIs(sqlText, "procedure") && !toks[i].wordIs(sqlText, "function") && !toks[i].wordIs(sqlText, "aggregate") {
			i++
		}
		if start < i {
			decl.DefinerOffset = toks[start].start
			val := strings.TrimSpace(sqlText[toks[start].start:toks[i-1].end])
			if strings.EqualFold(val, "current_user") || strings.HasPrefix(strings.ToLower(val), "current_user(") {
				decl.Definer = "current_user"
			} else {
				decl.Definer = "named"
			}
		}
	}
	if i < len(toks) && toks[i].wordIs(sqlText, "aggregate") {
		i++ // UDF aggregate marker; carries no modeled payload
	}
	if i >= len(toks) {
		return nil, nil, false
	}
	switch {
	case toks[i].wordIs(sqlText, "procedure"):
		decl.Kind = spec.RoutineKindProcedure
	case toks[i].wordIs(sqlText, "function"):
		decl.Kind = spec.RoutineKindFunction
	default:
		return nil, nil, false
	}
	i++
	// Optional IF NOT EXISTS.
	if i+2 < len(toks) && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "not") && toks[i+2].wordIs(sqlText, "exists") {
		i += 3
	}
	// Name: word|qident|str [, '.', name].
	if i >= len(toks) || toks[i].kind == tokPunct {
		return nil, nil, false
	}
	name := routineNameToken(sqlText, toks[i])
	if name == "" {
		return nil, nil, false
	}
	decl.Name.Name = name
	i++
	if i+1 < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." {
		if toks[i+1].kind != tokPunct {
			if next := routineNameToken(sqlText, toks[i+1]); next != "" {
				decl.Name.Schema = name
				decl.Name.Name = next
				i += 2
			}
		}
	}
	// Parameter list: balanced parens. Only a non-empty member list
	// records the params-present fact — an empty `()` is no declaration.
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
		depth, members := 0, 0
		for i < len(toks) {
			if toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
				depth++
			} else if toks[i].kind == tokPunct && toks[i].text(sqlText) == ")" {
				depth--
				if depth == 0 {
					i++
					break
				}
			} else if depth > 0 {
				members++
			}
			i++
		}
		if depth != 0 {
			return nil, nil, false
		}
		decl.ParamsPresent = members > 0
	}
	// RETURNS type (FUNCTION): normalized base type word.
	if i < len(toks) && toks[i].wordIs(sqlText, "returns") {
		i++
		if i < len(toks) && toks[i].kind == tokWord {
			decl.ReturnsType = strings.ToLower(toks[i].text(sqlText))
			i++
			// optional (len) / unsigned / zerofill / charset tail members —
			// bounded skip until a characteristic keyword, the RETURN
			// body, BEGIN, or SONAME.
			for i < len(toks) && !isRoutineCharacteristicStart(sqlText, toks, i) && !toks[i].wordIs(sqlText, "begin") && !toks[i].wordIs(sqlText, "soname") && !toks[i].wordIs(sqlText, "return") {
				i++
			}
		}
	}
	// Characteristics until the body or SONAME.
	i, udf := scanRoutineCharacteristics(sqlText, toks, i, decl)
	if udf {
		if i != len(toks) {
			return nil, nil, false // trailing members after SONAME
		}
		return finishRoutineStatement(sqlText, decl, nil, nil, spec.DDLOperationCreateFunction)
	}
	if i >= len(toks) {
		return nil, nil, false // no routine_body — let the grammar refuse
	}
	var body *spec.RoutineBody
	var inner []routineInnerMember
	if toks[i].wordIs(sqlText, "begin") {
		// Compound body: match this BEGIN's END at keyword depth.
		beginTok := i
		depth, endTok, endSuppressed := 1, -1, false
		for j := beginTok + 1; j < len(toks); j++ {
			if toks[j].kind != tokWord {
				continue
			}
			w := strings.ToLower(toks[j].text(sqlText))
			switch {
			case w == "end":
				depth--
				endSuppressed = true
				if depth == 0 {
					endTok = j
				}
			case routineCompoundOpeners[w]:
				if !endSuppressed {
					depth++
				}
				endSuppressed = false
			default:
				endSuppressed = false
			}
			if endTok >= 0 {
				break
			}
		}
		if endTok < 0 {
			return nil, nil, false // unbalanced body — never claim
		}
		body = &spec.RoutineBody{Compound: true, Offset: toks[beginTok].start}
		inner = splitRoutineBodyMembers(sqlText, toks[beginTok+1:endTok])
		if endTok+1 != len(toks) {
			// Trailing tokens after the outer END are not part of a pure
			// routine CREATE — let the real grammar own it.
			return nil, nil, false
		}
	} else {
		// Single-statement body: everything from here is one member.
		body = &spec.RoutineBody{Compound: false, Offset: toks[i].start}
		inner = splitRoutineBodyMembers(sqlText, toks[i:])
	}
	if len(inner) == 0 {
		return nil, nil, false
	}
	body.Statements = make([]spec.RoutineBodyStatement, 0, len(inner))
	op := spec.DDLOperationCreateProcedure
	if decl.Kind == spec.RoutineKindFunction {
		op = spec.DDLOperationCreateFunction
	}
	return finishRoutineStatement(sqlText, decl, body, inner, op)
}

// parseMySQLAlterRoutine claims ALTER PROCEDURE|FUNCTION name
// [characteristic …] — the MySQL grammar alters only characteristics,
// never the body (T27/#106).
func parseMySQLAlterRoutine(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	i := 1
	decl := &spec.RoutineDeclaration{}
	switch {
	case toks[i].wordIs(sqlText, "procedure"):
		decl.Kind = spec.RoutineKindProcedure
	case toks[i].wordIs(sqlText, "function"):
		decl.Kind = spec.RoutineKindFunction
	default:
		return nil, nil, false
	}
	i++
	if i >= len(toks) || toks[i].kind == tokPunct {
		return nil, nil, false
	}
	name := routineNameToken(sqlText, toks[i])
	if name == "" {
		return nil, nil, false
	}
	decl.Name.Name = name
	i++
	if i+1 < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." {
		if next := routineNameToken(sqlText, toks[i+1]); next != "" {
			decl.Name.Schema = name
			decl.Name.Name = next
			i += 2
		}
	}
	var udf bool
	i, udf = scanRoutineCharacteristics(sqlText, toks, i, decl)
	if udf || i != len(toks) {
		// Trailing tokens a characteristic scan cannot own are not a
		// pure routine ALTER — let the grammar refuse it.
		return nil, nil, false
	}
	op := spec.DDLOperationAlterProcedure
	if decl.Kind == spec.RoutineKindFunction {
		op = spec.DDLOperationAlterFunction
	}
	return finishRoutineStatement(sqlText, decl, nil, nil, op)
}

// parseMySQLDropRoutine claims DROP FUNCTION [IF EXISTS] name — the
// upstream grammar owns DROP PROCEDURE already, so this recognizer only
// covers the missing function form (T27/#106).
func parseMySQLDropRoutine(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	i := 1
	if !toks[i].wordIs(sqlText, "function") {
		return nil, nil, false
	}
	i++
	if i+1 < len(toks) && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "exists") {
		i += 2
	}
	if i >= len(toks) || toks[i].kind == tokPunct {
		return nil, nil, false
	}
	decl := &spec.RoutineDeclaration{Kind: spec.RoutineKindFunction}
	name := routineNameToken(sqlText, toks[i])
	if name == "" {
		return nil, nil, false
	}
	decl.Name.Name = name
	i++
	if i+1 < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." {
		if next := routineNameToken(sqlText, toks[i+1]); next != "" {
			decl.Name.Schema = name
			decl.Name.Name = next
			i += 2
		}
	}
	if i != len(toks) {
		return nil, nil, false
	}
	return finishRoutineStatement(sqlText, decl, nil, nil, spec.DDLOperationDropFunction)
}

// scanRoutineCharacteristics consumes the characteristic keyword list a
// routine declaration may carry — shared by the CREATE and ALTER forms
// (T27/#106). Returns the next token index and whether a SONAME UDF
// tail was consumed.
func scanRoutineCharacteristics(sqlText string, toks []sqlTok, i int, decl *spec.RoutineDeclaration) (int, bool) {
	udf := false
	for i < len(toks) {
		switch {
		case toks[i].wordIs(sqlText, "comment") && i+1 < len(toks) && toks[i+1].kind == tokStr:
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "comment", Offset: toks[i].start})
			i += 2
		case toks[i].wordIs(sqlText, "language") && i+1 < len(toks) && toks[i+1].kind == tokWord:
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "language", Value: strings.ToLower(toks[i+1].text(sqlText)), Offset: toks[i].start})
			i += 2
		case toks[i].wordIs(sqlText, "deterministic"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "deterministic", Value: "true", Offset: toks[i].start})
			i++
		case toks[i].wordIs(sqlText, "not") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "deterministic"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "deterministic", Value: "false", Offset: toks[i].start})
			i += 2
		case toks[i].wordIs(sqlText, "contains") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "sql"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_data_access", Value: "contains_sql", Offset: toks[i].start})
			i += 2
		case toks[i].wordIs(sqlText, "no") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "sql"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_data_access", Value: "no_sql", Offset: toks[i].start})
			i += 2
		case toks[i].wordIs(sqlText, "reads") && i+2 < len(toks) && toks[i+1].wordIs(sqlText, "sql") && toks[i+2].wordIs(sqlText, "data"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_data_access", Value: "reads_sql_data", Offset: toks[i].start})
			i += 3
		case toks[i].wordIs(sqlText, "modifies") && i+2 < len(toks) && toks[i+1].wordIs(sqlText, "sql") && toks[i+2].wordIs(sqlText, "data"):
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_data_access", Value: "modifies_sql_data", Offset: toks[i].start})
			i += 3
		case toks[i].wordIs(sqlText, "sql") && i+2 < len(toks) && toks[i+1].wordIs(sqlText, "security") && toks[i+2].kind == tokWord:
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_security", Value: strings.ToLower(toks[i+2].text(sqlText)), Offset: toks[i].start})
			i += 3
		case toks[i].wordIs(sqlText, "soname") && i+1 < len(toks) && toks[i+1].kind == tokStr:
			decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "soname", Offset: toks[i].start})
			udf = true
			i += 2
		default:
			return i, udf
		}
	}
	return i, udf
}

func finishRoutineStatement(sqlText string, decl *spec.RoutineDeclaration, body *spec.RoutineBody, inner []routineInnerMember, op spec.DDLOperation) (ast.StmtNode, []error, bool) {
	decl.Body = body
	node := &routineStatement{operation: op, declaration: decl, inner: inner}
	// The carrier is a real ast type with only shared members set — the
	// same fabrication shape the other MySQL-family adapters use.
	carrier := &ast.ProcedureInfo{}
	carrier.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	node.StmtNode = carrier
	return node, nil, true
}

// routineOffsetPosition returns the 1-based line/column of a byte offset
// inside the parent statement's raw text — positions live in parent-raw
// space so the evaluation fold can translate them into file space.
func routineOffsetPosition(raw string, offset int) (int, int) {
	line, col := 1, 1
	for i := 0; i < offset && i < len(raw); i++ {
		if raw[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}

// isRoutineCharacteristicStart reports whether tokens[i] opens a modeled
// or recognizable routine characteristic keyword.
func isRoutineCharacteristicStart(sqlText string, toks []sqlTok, i int) bool {
	for _, w := range []string{"comment", "language", "deterministic", "not", "contains", "no", "reads", "modifies", "sql"} {
		if toks[i].wordIs(sqlText, w) {
			return true
		}
	}
	return false
}

func routineNameToken(sqlText string, tok sqlTok) string {
	switch tok.kind {
	case tokWord:
		return strings.ToLower(tok.text(sqlText))
	case tokQIdent:
		return tok.identText(sqlText)
	}
	return ""
}

// splitRoutineBodyMembers cuts the token span of a routine body into
// member segments on top-level semicolons. Nested compound constructs
// suppress the cut; each member then parses through the real grammar —
// failures stay unmodeled members with their reason, never dropped.
func splitRoutineBodyMembers(sqlText string, toks []sqlTok) []routineInnerMember {
	var members []routineInnerMember
	depth, endSuppressed, segStart := 0, false, -1
	flush := func(endTok int) {
		if segStart < 0 {
			return
		}
		seg := toks[segStart:endTok]
		segStart = -1
		if len(seg) == 0 {
			return
		}
		raw := sqlText[seg[0].start:seg[len(seg)-1].end]
		offset := seg[0].start
		first := strings.ToLower(seg[0].text(sqlText))
		switch {
		case routineCompoundOpeners[first] || first == "declare" || first == "handler" || first == "return":
			reason := "routine.body.compound_member"
			if first == "declare" || first == "handler" {
				reason = "routine.body.declaration"
			} else if first == "return" {
				reason = "routine.body.return"
			}
			members = append(members, routineInnerMember{raw: raw, offset: offset, err: reason})
		default:
			stmts, _, err := parser.New().Parse(raw, "", "")
			switch {
			case err != nil:
				members = append(members, routineInnerMember{raw: raw, offset: offset, err: "routine.body.parse"})
			case len(stmts) != 1:
				members = append(members, routineInnerMember{raw: raw, offset: offset, err: "routine.body.unexpected_members"})
			default:
				members = append(members, routineInnerMember{node: stmts[0], raw: raw, offset: offset})
			}
		}
	}
	for j := 0; j < len(toks); j++ {
		t := toks[j]
		if t.kind == tokPunct && t.text(sqlText) == ";" {
			if depth == 0 {
				flush(j)
			}
			continue
		}
		if t.kind != tokWord {
			if segStart < 0 {
				segStart = j
			}
			continue
		}
		w := strings.ToLower(t.text(sqlText))
		switch {
		case w == "end":
			if depth > 0 {
				depth--
			}
			endSuppressed = true
		case routineCompoundOpeners[w]:
			if !endSuppressed {
				depth++
			}
			endSuppressed = false
		default:
			endSuppressed = false
		}
		if segStart < 0 {
			segStart = j
		}
	}
	flush(len(toks))
	return members
}
