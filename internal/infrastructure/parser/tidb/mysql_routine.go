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
// splits the body into member segments at top-level semicolons only,
// then classifies each segment recursively (T28): compound constructs
// (IF/CASE/LOOP/WHILE/REPEAT/nested BEGIN), declarations, SET/PREPARE
// bindings, and flow verbs become structured members; anything else
// stays a bounded unmodeled member rather than guessed SQL.

type routineStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.RoutineDeclaration
	inner       []routineInnerMember
}

// routineInnerMember is one body member: a parsed SQL statement, a
// structured member (compound/declare/set/prepare/flow) parsed at
// token level, or an unmodeled segment kept with its reason (T28).
type routineInnerMember struct {
	node     ast.StmtNode
	raw      string
	offset   int // byte offset of the member's first byte inside the whole input
	err      string
	flow     string
	set      *spec.RoutineSet
	prepare  *spec.RoutinePrepare
	declare  *routineDeclareMember
	compound *routineCompoundMember
}

// routineDeclareMember carries a parsed DECLARE member; handler and
// cursor forms keep their executable member for normal extraction.
type routineDeclareMember struct {
	kind    string // variable|condition|handler|cursor
	names   []string
	varType string
	member  *routineInnerMember
}

// routineCompoundMember carries a parsed control-flow member. Branches
// holds IF/CASE arm member lists; Body holds the single list of
// WHILE/LOOP/REPEAT/nested BEGIN constructs.
type routineCompoundMember struct {
	kind     string
	label    string
	hasElse  bool
	branches [][]routineInnerMember
	body     []routineInnerMember
}

var routineCompoundOpeners = map[string]bool{
	"begin": true, "if": true, "case": true, "loop": true, "while": true, "repeat": true,
}

// routineBodyMaxDepth bounds compound nesting inside one routine body;
// routineBodyMaxMembers bounds total extracted members. Overflow is a
// bounded incomplete member, never a pass or a crash (T28).
const (
	routineBodyMaxDepth   = 8
	routineBodyMaxMembers = 512
)

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
	// Parameter list: balanced parens. A non-empty list must parse into
	// [direction] name type members — anything else is not a shape this
	// recognizer can claim (T28).
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
		open := i
		depth := 0
		for i < len(toks) {
			if toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
				depth++
			} else if toks[i].kind == tokPunct && toks[i].text(sqlText) == ")" {
				depth--
				if depth == 0 {
					break
				}
			}
			i++
		}
		if i >= len(toks) || depth != 0 {
			return nil, nil, false
		}
		params, ok := parseRoutineParams(sqlText, toks[open+1:i])
		if !ok {
			return nil, nil, false
		}
		i++
		decl.Params = params
		decl.ParamsPresent = len(params) > 0
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
		endTok := routineScanEnd(sqlText, toks, beginTok+1)
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

// convertRoutineMembers turns classified inner members into spec body
// statements — SQL members extract through the shared statement
// machinery, structured members keep their typed payloads, unmodeled
// members keep their boundary reason (T28).
func convertRoutineMembers(dialect spec.Dialect, rawSQL string, members []routineInnerMember) []spec.RoutineBodyStatement {
	out := make([]spec.RoutineBodyStatement, 0, len(members))
	for _, member := range members {
		bs := spec.RoutineBodyStatement{Offset: member.offset}
		line, col := routineOffsetPosition(rawSQL, member.offset)
		switch {
		case member.compound != nil:
			bs.Statement = spec.Statement{Kind: spec.KindUnknown, RawSQL: member.raw}
			bs.Compound = &spec.RoutineCompound{Kind: member.compound.kind, Label: member.compound.label, HasElse: member.compound.hasElse}
			for _, br := range member.compound.branches {
				bs.Compound.Branches = append(bs.Compound.Branches, &spec.RoutineBody{
					Statements: convertRoutineMembers(dialect, rawSQL, br),
				})
			}
			if member.compound.body != nil {
				bs.Compound.Body = &spec.RoutineBody{
					Statements: convertRoutineMembers(dialect, rawSQL, member.compound.body),
				}
			}
		case member.declare != nil:
			bs.Statement = spec.Statement{Kind: spec.KindUnknown, RawSQL: member.raw}
			bs.Declare = &spec.RoutineDeclare{
				Kind: member.declare.kind, Names: member.declare.names, VarType: member.declare.varType,
			}
			if member.declare.member != nil {
				conv := convertRoutineMembers(dialect, rawSQL, []routineInnerMember{*member.declare.member})
				if len(conv) == 1 {
					bs.Declare.Member = &conv[0]
				}
			}
		case member.set != nil:
			bs.Statement = spec.Statement{Kind: spec.KindUnknown, RawSQL: member.raw}
			bs.Set = member.set
		case member.prepare != nil:
			bs.Statement = spec.Statement{Kind: spec.KindUnknown, RawSQL: member.raw}
			bs.Prepare = member.prepare
		case member.flow != "":
			bs.Statement = spec.Statement{Kind: spec.KindUnknown, RawSQL: member.raw}
			bs.Flow = member.flow
		case member.node != nil:
			inner, ierr := tidbExtractor{kind: classify(member.node), node: member.node}.Extract(dialect, member.raw)
			if ierr != nil {
				bs.Statement = spec.Statement{
					Kind:   classify(member.node),
					RawSQL: member.raw,
					Unsupported: &spec.UnsupportedDetail{
						Feature: "routine.body.extract",
						Reason:  spec.UnsupportedUnauditedReason,
					},
				}
			} else {
				bs.Statement = inner
				bs.RawSQL = member.raw
			}
		default:
			bs.Statement = spec.Statement{
				Kind:   spec.KindUnknown,
				RawSQL: member.raw,
				Unsupported: &spec.UnsupportedDetail{
					Feature: member.err,
					Reason:  spec.UnsupportedUnauditedReason,
				},
			}
		}
		bs.Line = line
		bs.Column = col
		out = append(out, bs)
	}
	return out
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
// member segments on top-level semicolons, then classifies each member:
// structured members (control flow, declarations, SET, PREPARE, flow
// verbs) parse at token level, ordinary SQL parses through the real
// grammar, and anything else stays a bounded unmodeled member (T28).
func splitRoutineBodyMembers(sqlText string, toks []sqlTok) []routineInnerMember {
	count := 0
	return classifyRoutineSegments(sqlText, splitMemberSegments(sqlText, toks), 0, &count)
}

// splitMemberSegments cuts a token span at top-level semicolons —
// nested compound constructs suppress the cut. Member-initial words
// only (never mid-member IF/CASE/WHILE-shaped words) open constructs.
func splitMemberSegments(sqlText string, toks []sqlTok) [][]sqlTok {
	var segs [][]sqlTok
	scan := &routineMemberScanner{memberStart: true}
	segStart := -1
	flush := func(endTok int) {
		if segStart >= 0 && segStart < endTok {
			segs = append(segs, toks[segStart:endTok])
		}
		segStart = -1
	}
	for j := 0; j < len(toks); j++ {
		t := toks[j]
		if t.kind == tokPunct && t.text(sqlText) == ";" {
			if scan.depth == 0 {
				flush(j)
			}
			scan.feedToken(sqlText, t)
			continue
		}
		if segStart < 0 {
			segStart = j
		}
		scan.feedToken(sqlText, t)
	}
	flush(len(toks))
	return segs
}

// classifyRoutineSegments classifies every member segment in order.
func classifyRoutineSegments(sqlText string, segs [][]sqlTok, depth int, count *int) []routineInnerMember {
	members := make([]routineInnerMember, 0, len(segs))
	for _, seg := range segs {
		members = append(members, classifyRoutineMember(sqlText, seg, depth, count))
	}
	return members
}

// classifyRoutineMember dispatches one member segment. Depth counts
// enclosing compound constructs (outer body members sit at 0); members
// beyond routineBodyMaxDepth or the total-member bound become explicit
// incomplete members rather than unbounded analysis (T28).
func classifyRoutineMember(sqlText string, seg []sqlTok, depth int, count *int) routineInnerMember {
	raw := sqlText[seg[0].start:seg[len(seg)-1].end]
	offset := seg[0].start
	(*count)++
	if *count > routineBodyMaxMembers {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.size"}
	}
	if depth > routineBodyMaxDepth {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.depth"}
	}
	kwIdx := 0
	label := ""
	if len(seg) >= 3 && seg[0].kind == tokWord && seg[1].kind == tokPunct && seg[1].text(sqlText) == ":" {
		label = strings.ToLower(seg[0].text(sqlText))
		kwIdx = 2
	}
	if kwIdx >= len(seg) || seg[kwIdx].kind != tokWord {
		return classifyRoutineSQLMember(sqlText, seg, offset)
	}
	first := strings.ToLower(seg[kwIdx].text(sqlText))
	// Labels belong to block constructs only; a label before anything
	// else is malformed member text — bounded, never claimed.
	if label != "" && first != "begin" && first != "loop" && first != "while" && first != "repeat" {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.parse"}
	}
	switch first {
	case "if":
		return parseRoutineIfMember(sqlText, seg, kwIdx, label, offset, depth, count)
	case "case":
		return parseRoutineCaseMember(sqlText, seg, kwIdx, offset, depth, count)
	case "while", "loop", "repeat", "begin":
		return parseRoutineBlockMember(sqlText, seg, kwIdx, label, first, offset, depth, count)
	case "declare":
		return parseRoutineDeclareMember(sqlText, seg, kwIdx, offset, depth, count)
	case "set":
		return parseRoutineSetMember(sqlText, seg, kwIdx, offset)
	case "prepare":
		return parseRoutinePrepareMember(sqlText, seg, kwIdx, offset)
	case "return":
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.return"}
	case "call":
		// CALL reaches outside the routine — an external routine
		// dependency stays a bounded incomplete member (T28).
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.call"}
	case "leave", "iterate", "signal", "resignal", "open", "fetch", "close", "get":
		return routineInnerMember{raw: raw, offset: offset, flow: first}
	default:
		return classifyRoutineSQLMember(sqlText, seg, offset)
	}
}

// classifyRoutineSQLMember parses an ordinary body statement through
// the shared grammar — failures stay bounded members with the reason.
func classifyRoutineSQLMember(sqlText string, seg []sqlTok, offset int) routineInnerMember {
	raw := sqlText[seg[0].start:seg[len(seg)-1].end]
	stmts, _, err := parser.New().Parse(raw, "", "")
	switch {
	case err != nil:
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.parse"}
	case len(stmts) != 1:
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.unexpected_members"}
	default:
		return routineInnerMember{node: stmts[0], raw: raw, offset: offset}
	}
}

// routineMemberScanner tracks compound nesting while scanning a token
// span so construct keywords can be located at the construct's own
// level — nested constructs suppress them. A compound keyword counts
// only in member-initial position (segment start or after a label
// prefix): mid-member words such as IF() function calls or DROP TABLE
// IF EXISTS are never constructs. A non-initial "case" instead opens a
// case-expression pairing the next "end" — that is how a mid-member
// END stays inside its own member (T28).
type routineMemberScanner struct {
	depth         int
	exprCase      int
	endSuppressed bool
	memberStart   bool
	lastInitial   bool
}

// feedToken advances the scanner over one token and reports true on the
// construct's own END. ";" re-arms member-initial tracking and resets
// case-expressions — neither spans a member boundary; ":" after a
// member-initial identifier marks a label so the construct keyword
// that follows it still counts.
func (s *routineMemberScanner) feedToken(sqlText string, t sqlTok) bool {
	initial := s.memberStart
	s.memberStart = false
	switch {
	case t.kind == tokPunct && t.text(sqlText) == ";":
		s.memberStart, s.lastInitial, s.endSuppressed, s.exprCase = true, false, false, 0
		return false
	case t.kind == tokPunct && t.text(sqlText) == ":" && s.lastInitial:
		s.memberStart, s.lastInitial = true, false
		return false
	}
	s.lastInitial = false
	if t.kind == tokWord {
		s.lastInitial = initial
		return s.feed(strings.ToLower(t.text(sqlText)), initial)
	}
	s.lastInitial = isIdentToken(t) && initial
	return false
}

// feed handles one word. memberStart marks member-initial position —
// only there may a compound keyword open a construct. It reports true
// on the construct's own END — never a case-expression's END. Branch
// and loop connectives (then/else/do) re-arm member-initial tracking:
// the word that follows them starts a member. begin/loop/repeat open
// the member list directly, so the word after them is member-initial
// too — if/case/while instead lead a condition expression.
func (s *routineMemberScanner) feed(w string, memberStart bool) bool {
	switch {
	case w == "end" && s.exprCase > 0:
		s.exprCase--
	case w == "end":
		if s.depth == 0 {
			return true
		}
		s.depth--
		s.endSuppressed = true
	case s.endSuppressed:
		s.endSuppressed = false // END's optional closing keyword
	case memberStart && routineCompoundOpeners[w]:
		s.depth++
		if w == "begin" || w == "loop" || w == "repeat" {
			s.memberStart = true
		}
	case w == "case" && !memberStart:
		s.exprCase++
	case (w == "then" || w == "else" || w == "do") && s.exprCase == 0:
		s.memberStart = true
	}
	return false
}

// routineScanStartInitial reports whether the token at `from` sits in
// member-initial position: at span start, after a member boundary (;),
// or after a block opener / branch connective — begin|repeat|then|
// else|do. Anything else (if/case/while/until/elseif/when, operands)
// opens an expression, not a member.
func routineScanStartInitial(sqlText string, seg []sqlTok, from int) bool {
	if from <= 0 {
		return true
	}
	prev := seg[from-1]
	if prev.kind == tokPunct {
		return prev.text(sqlText) == ";"
	}
	if prev.kind != tokWord {
		return false
	}
	switch strings.ToLower(prev.text(sqlText)) {
	case "begin", "loop", "repeat", "then", "else", "do":
		return true
	}
	return false
}

// routineScanEnd returns the index of the construct's own END keyword —
// the first END seen at the member's local depth zero.
func routineScanEnd(sqlText string, seg []sqlTok, from int) int {
	scan := &routineMemberScanner{memberStart: routineScanStartInitial(sqlText, seg, from)}
	for j := from; j < len(seg); j++ {
		if scan.feedToken(sqlText, seg[j]) {
			return j
		}
	}
	return -1
}

// routineScanKeyword returns the first index whose word matches one of
// the wanted keywords at local depth zero — connectives inside nested
// constructs and case-expressions do not count.
func routineScanKeyword(sqlText string, seg []sqlTok, from int, wants ...string) int {
	scan := &routineMemberScanner{memberStart: routineScanStartInitial(sqlText, seg, from)}
	for j := from; j < len(seg); j++ {
		if seg[j].kind == tokWord && scan.depth == 0 && scan.exprCase == 0 {
			w := strings.ToLower(seg[j].text(sqlText))
			for _, want := range wants {
				if w == want {
					return j
				}
			}
		}
		scan.feedToken(sqlText, seg[j])
	}
	return -1
}

// routineSegText is the verbatim text of a token segment.
func routineSegText(sqlText string, seg []sqlTok) string {
	return sqlText[seg[0].start:seg[len(seg)-1].end]
}

// routineMembersIn converts a token span into classified members — the
// shared recursion used by every compound arm and nested block.
func routineMembersIn(sqlText string, toks []sqlTok, depth int, count *int) []routineInnerMember {
	return classifyRoutineSegments(sqlText, splitMemberSegments(sqlText, toks), depth, count)
}

// parseRoutineIfMember parses IF cond THEN members [ELSEIF … THEN …]*
// [ELSE …] END IF into a branch-bearing compound member.
func parseRoutineIfMember(sqlText string, seg []sqlTok, kwIdx int, label string, offset, depth int, count *int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	bad := func() routineInnerMember {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.compound_member"}
	}
	if label != "" {
		return bad() // MySQL does not label IF
	}
	thenIdx := routineScanKeyword(sqlText, seg, kwIdx+1, "then")
	if thenIdx < 0 {
		return bad()
	}
	var branches [][]routineInnerMember
	hasElse := false
	cur := thenIdx + 1
	endIdx := -1
	for endIdx < 0 {
		split := routineScanKeyword(sqlText, seg, cur, "elseif", "else", "end")
		if split < 0 {
			return bad()
		}
		branches = append(branches, routineMembersIn(sqlText, seg[cur:split], depth+1, count))
		switch w := strings.ToLower(seg[split].text(sqlText)); w {
		case "end":
			endIdx = split
		case "else":
			endJ := routineScanEnd(sqlText, seg, split+1)
			if endJ < 0 {
				return bad()
			}
			branches = append(branches, routineMembersIn(sqlText, seg[split+1:endJ], depth+1, count))
			endIdx = endJ
			hasElse = true
		case "elseif":
			nthen := routineScanKeyword(sqlText, seg, split+1, "then")
			if nthen < 0 {
				return bad()
			}
			cur = nthen + 1
		}
	}
	if endIdx+2 != len(seg) || !seg[endIdx+1].wordIs(sqlText, "if") {
		return bad()
	}
	return routineInnerMember{raw: raw, offset: offset, compound: &routineCompoundMember{kind: "if", hasElse: hasElse, branches: branches}}
}

// parseRoutineCaseMember parses CASE [operand] WHEN … THEN … [ELSE …]
// END CASE into a branch-bearing compound member.
func parseRoutineCaseMember(sqlText string, seg []sqlTok, kwIdx int, offset, depth int, count *int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	bad := func() routineInnerMember {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.compound_member"}
	}
	var branches [][]routineInnerMember
	hasElse := false
	cur := kwIdx + 1
	endIdx := -1
	for endIdx < 0 {
		kwIdx := routineScanKeyword(sqlText, seg, cur, "when", "else", "end")
		if kwIdx < 0 {
			return bad()
		}
		switch w := strings.ToLower(seg[kwIdx].text(sqlText)); w {
		case "when":
			thenIdx := routineScanKeyword(sqlText, seg, kwIdx+1, "then")
			if thenIdx < 0 {
				return bad()
			}
			cur = thenIdx + 1
		case "else":
			endJ := routineScanEnd(sqlText, seg, kwIdx+1)
			if endJ < 0 {
				return bad()
			}
			branches = append(branches, routineMembersIn(sqlText, seg[kwIdx+1:endJ], depth+1, count))
			endIdx = endJ
			hasElse = true
		case "end":
			endIdx = kwIdx
		}
		if endIdx < 0 && strings.ToLower(seg[kwIdx].text(sqlText)) == "when" {
			// members of this WHEN arm run to the next WHEN/ELSE/END
			armEnd := routineScanKeyword(sqlText, seg, cur, "when", "else", "end")
			if armEnd < 0 {
				return bad()
			}
			branches = append(branches, routineMembersIn(sqlText, seg[cur:armEnd], depth+1, count))
			cur = armEnd
		}
	}
	if endIdx+2 != len(seg) || !seg[endIdx+1].wordIs(sqlText, "case") || len(branches) == 0 {
		return bad()
	}
	return routineInnerMember{raw: raw, offset: offset, compound: &routineCompoundMember{kind: "case", hasElse: hasElse, branches: branches}}
}

// parseRoutineBlockMember parses WHILE/LOOP/REPEAT/nested BEGIN members
// — constructs whose members form one body rather than branches.
func parseRoutineBlockMember(sqlText string, seg []sqlTok, kwIdx int, label, verb string, offset, depth int, count *int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	bad := func() routineInnerMember {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.compound_member"}
	}
	var bodyToks []sqlTok
	trailer := 0 // index just past the construct's own END
	switch verb {
	case "begin":
		endIdx := routineScanEnd(sqlText, seg, kwIdx+1)
		if endIdx < 0 {
			return bad()
		}
		bodyToks = seg[kwIdx+1 : endIdx]
		trailer = endIdx + 1
	case "while":
		doIdx := routineScanKeyword(sqlText, seg, kwIdx+1, "do")
		if doIdx < 0 {
			return bad()
		}
		endIdx := routineScanEnd(sqlText, seg, doIdx+1)
		if endIdx < 0 || endIdx+1 >= len(seg) || !seg[endIdx+1].wordIs(sqlText, "while") {
			return bad()
		}
		bodyToks = seg[doIdx+1 : endIdx]
		trailer = endIdx + 2
	case "loop":
		endIdx := routineScanEnd(sqlText, seg, kwIdx+1)
		if endIdx < 0 || endIdx+1 >= len(seg) || !seg[endIdx+1].wordIs(sqlText, "loop") {
			return bad()
		}
		bodyToks = seg[kwIdx+1 : endIdx]
		trailer = endIdx + 2
	case "repeat":
		untilIdx := routineScanKeyword(sqlText, seg, kwIdx+1, "until")
		if untilIdx < 0 {
			return bad()
		}
		endIdx := routineScanEnd(sqlText, seg, untilIdx+1)
		if endIdx < 0 || endIdx+1 >= len(seg) || !seg[endIdx+1].wordIs(sqlText, "repeat") {
			return bad()
		}
		bodyToks = seg[kwIdx+1 : untilIdx]
		trailer = endIdx + 2
	}
	// Trailing members: an optional block label, then the member ends.
	rest := seg[trailer:]
	if len(rest) > 1 || (len(rest) == 1 && !isIdentToken(rest[0])) {
		return bad()
	}
	return routineInnerMember{raw: raw, offset: offset, compound: &routineCompoundMember{
		kind:  verb,
		label: label,
		body:  routineMembersIn(sqlText, bodyToks, depth+1, count),
	}}
}

// parseRoutineDeclareMember parses DECLARE members: variable,
// condition, handler, and cursor declarations. Handler bodies and
// cursor SELECTs stay executable members — extracted through the same
// classifier so they audit like any other member (T28).
func parseRoutineDeclareMember(sqlText string, seg []sqlTok, kwIdx int, offset, depth int, count *int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	bad := func() routineInnerMember {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.declaration"}
	}
	i := kwIdx + 1
	if i >= len(seg) || !isIdentToken(seg[i]) {
		return bad()
	}
	names := []string{seg[i].identText(sqlText)}
	i++
	for i+1 < len(seg) && seg[i].kind == tokPunct && seg[i].text(sqlText) == "," && isIdentToken(seg[i+1]) {
		names = append(names, seg[i+1].identText(sqlText))
		i += 2
	}
	if i >= len(seg) {
		return bad()
	}
	if len(names) == 1 {
		switch strings.ToLower(seg[i].text(sqlText)) {
		case "condition":
			i++
			if i < len(seg) && seg[i].wordIs(sqlText, "for") && i+1 < len(seg) {
				return routineInnerMember{raw: raw, offset: offset, declare: &routineDeclareMember{kind: "condition", names: names}}
			}
			return bad()
		case "cursor":
			i++
			if i < len(seg) && seg[i].wordIs(sqlText, "for") && i+1 < len(seg) {
				member := classifyRoutineMember(sqlText, seg[i+1:], depth+1, count)
				return routineInnerMember{raw: raw, offset: offset, declare: &routineDeclareMember{kind: "cursor", names: names, member: &member}}
			}
			return bad()
		case "handler":
			i++
			if i >= len(seg) || !seg[i].wordIs(sqlText, "for") {
				return bad()
			}
			i++
			// Handler condition values: sqlstate [value] '…' | not found |
			// sqlwarning | sqlexception | errno | condition name — a bare
			// non-comma word after a complete value opens the statement.
			for i < len(seg) {
				w := strings.ToLower(seg[i].text(sqlText))
				switch {
				case w == "sqlstate":
					i++
					if i < len(seg) && seg[i].wordIs(sqlText, "value") {
						i++
					}
					if i < len(seg) && seg[i].kind == tokStr {
						i++
					}
				case w == "not" && i+1 < len(seg) && seg[i+1].wordIs(sqlText, "found"):
					i += 2
				case w == "sqlwarning" || w == "sqlexception" || seg[i].kind == tokStr:
					i++
				default:
					i++
				}
				if i < len(seg) && seg[i].kind == tokPunct && seg[i].text(sqlText) == "," {
					i++
					continue
				}
				goto handlerTail
			}
		handlerTail:
			if i >= len(seg) {
				return bad()
			}
			member := classifyRoutineMember(sqlText, seg[i:], depth+1, count)
			return routineInnerMember{raw: raw, offset: offset, declare: &routineDeclareMember{kind: "handler", names: names, member: &member}}
		}
	}
	// Variable declaration: names then the type text up to DEFAULT.
	typeEnd := len(seg)
	for j := i; j < len(seg); j++ {
		if seg[j].wordIs(sqlText, "default") {
			typeEnd = j
			break
		}
	}
	if i >= typeEnd {
		return bad()
	}
	varType := strings.ToLower(sqlText[seg[i].start:seg[typeEnd-1].end])
	return routineInnerMember{raw: raw, offset: offset, declare: &routineDeclareMember{kind: "variable", names: names, varType: varType}}
}

// parseRoutineSetMember parses SET members into variable bindings —
// only @user and local-variable targets are recorded; system variables
// and non-variable shapes stay unrecorded dynamic facts (T28).
func parseRoutineSetMember(sqlText string, seg []sqlTok, kwIdx int, offset int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	set := &spec.RoutineSet{}
	// Split the assignment list on top-level commas.
	pdepth, start := 0, kwIdx+1
	flush := func(end int) {
		part := seg[start:end]
		if len(part) == 0 {
			return
		}
		eq, lhsEnd := -1, -1
		for j := 0; j < len(part); j++ {
			if part[j].kind == tokPunct && part[j].text(sqlText) == "=" {
				eq, lhsEnd = j, j
				break
			}
			if part[j].kind == tokPunct && part[j].text(sqlText) == ":" && j+1 < len(part) && part[j+1].kind == tokPunct && part[j+1].text(sqlText) == "=" {
				eq, lhsEnd = j+1, j
				break
			}
		}
		if eq < 1 {
			set.Unmodeled = true
			return
		}
		lhs, rhs := part[:lhsEnd], part[eq+1:]
		target := ""
		switch {
		case len(lhs) == 2 && lhs[0].kind == tokPunct && lhs[0].text(sqlText) == "@" && isIdentToken(lhs[1]):
			target = "@" + lhs[1].identText(sqlText)
		case len(lhs) == 1 && isIdentToken(lhs[0]):
			target = lhs[0].identText(sqlText)
		}
		if target == "" {
			// @@system vars and unusual targets are not bindings — the
			// unmodeled assignment stays visible as a bounded member.
			set.Unmodeled = true
			return
		}
		a := spec.RoutineAssignment{Target: target}
		if lit, ok := routineLiteralText(sqlText, rhs); ok {
			a.Literal = lit
			a.HasLiteral = true
		}
		set.Assignments = append(set.Assignments, a)
	}
	for j := kwIdx + 1; j < len(seg); j++ {
		if seg[j].kind == tokPunct {
			switch seg[j].text(sqlText) {
			case "(":
				pdepth++
			case ")":
				pdepth--
			case ",":
				if pdepth == 0 {
					flush(j)
					start = j + 1
				}
			}
		}
	}
	flush(len(seg))
	if len(set.Assignments) == 0 && !set.Unmodeled {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.set"}
	}
	if len(set.Assignments) == 0 {
		// A SET with no variable assignments at all (SET GLOBAL/NAMES/
		// @@system) is not a binding member — audit it as the ordinary
		// statement it is (T28).
		return classifyRoutineSQLMember(sqlText, seg, offset)
	}
	return routineInnerMember{raw: raw, offset: offset, set: set}
}

// parseRoutinePrepareMember parses PREPARE name FROM source. The source
// keeps its authored shape — literal, variable reference, or other
// expression — and the source token offset so resolved-literal audits
// point at authored SQL (T28).
func parseRoutinePrepareMember(sqlText string, seg []sqlTok, kwIdx int, offset int) routineInnerMember {
	raw := routineSegText(sqlText, seg)
	i := kwIdx + 1
	if i >= len(seg) || !isIdentToken(seg[i]) {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.prepare"}
	}
	name := seg[i].identText(sqlText)
	i++
	if i >= len(seg) || !seg[i].wordIs(sqlText, "from") || i+1 >= len(seg) {
		return routineInnerMember{raw: raw, offset: offset, err: "routine.body.prepare"}
	}
	src := seg[i+1:]
	prep := &spec.RoutinePrepare{Name: name, SourceOffset: src[0].start}
	switch {
	case len(src) == 2 && src[0].kind == tokPunct && src[0].text(sqlText) == "@" && isIdentToken(src[1]):
		prep.Source = spec.RoutinePrepareSource{Kind: "var", Var: "@" + src[1].identText(sqlText)}
	case len(src) == 1 && isIdentToken(src[0]):
		prep.Source = spec.RoutinePrepareSource{Kind: "var", Var: src[0].identText(sqlText)}
	default:
		if lit, ok := routineLiteralText(sqlText, src); ok {
			prep.Source = spec.RoutinePrepareSource{Kind: "literal", Literal: lit}
		} else {
			prep.Source = spec.RoutinePrepareSource{Kind: "expr"}
		}
	}
	return routineInnerMember{raw: raw, offset: offset, prepare: prep}
}

// routineLiteralText resolves a token run of adjacent string literals
// (MySQL concatenates them) into one unquoted text — anything else is
// not a statically known string.
func routineLiteralText(sqlText string, toks []sqlTok) (string, bool) {
	if len(toks) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, t := range toks {
		if t.kind != tokStr {
			return "", false
		}
		s, ok := routineUnquoteString(t.text(sqlText))
		if !ok {
			return "", false
		}
		b.WriteString(s)
	}
	return b.String(), true
}

// routineUnquoteString decodes one 'single' or "double" quoted literal:
// doubled quotes collapse and backslash escapes decode, matching the
// MySQL literal value a PREPARE source would hold.
func routineUnquoteString(raw string) (string, bool) {
	if len(raw) < 2 || (raw[0] != '\'' && raw[0] != '"') || raw[len(raw)-1] != raw[0] {
		return "", false
	}
	q := raw[0]
	inner := raw[1 : len(raw)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == q {
			if i+1 < len(inner) && inner[i+1] == q {
				b.WriteByte(q)
				i++
				continue
			}
			return "", false
		}
		if c == '\\' && i+1 < len(inner) {
			i++
			switch inner[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '0':
				b.WriteByte(0)
			case 'b':
				b.WriteByte('\b')
			case 'Z':
				b.WriteByte(0x1a)
			default:
				b.WriteByte(inner[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), true
}

// parseRoutineParams parses a parameter-list token span into modeled
// parameters — [IN|OUT|INOUT] name type for procedures, name type for
// functions. An empty span is a valid empty list; anything unparseable
// means the shape is not claimable.
func parseRoutineParams(sqlText string, toks []sqlTok) ([]spec.RoutineParam, bool) {
	var out []spec.RoutineParam
	pdepth, start := 0, 0
	ok := true
	flush := func(end int) {
		if !ok {
			return
		}
		part := toks[start:end]
		if len(part) == 0 {
			if start == 0 && end == 0 {
				return // wholly empty list
			}
			ok = false
			return
		}
		j := 0
		p := spec.RoutineParam{}
		if part[j].kind == tokWord {
			switch strings.ToLower(part[j].text(sqlText)) {
			case "in", "out", "inout":
				p.Direction = strings.ToLower(part[j].text(sqlText))
				j++
			}
		}
		if j >= len(part) || !isIdentToken(part[j]) {
			ok = false
			return
		}
		p.Name = part[j].identText(sqlText)
		j++
		if j >= len(part) {
			ok = false // name without a type
			return
		}
		p.VarType = strings.ToLower(sqlText[part[j].start:part[len(part)-1].end])
		out = append(out, p)
	}
	for j := 0; j < len(toks); j++ {
		if toks[j].kind != tokPunct {
			continue
		}
		switch toks[j].text(sqlText) {
		case "(":
			pdepth++
		case ")":
			pdepth--
		case ",":
			if pdepth == 0 {
				flush(j)
				start = j + 1
			}
		}
	}
	flush(len(toks))
	return out, ok
}
