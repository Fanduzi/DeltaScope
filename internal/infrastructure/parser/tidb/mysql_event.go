package tidbparser

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysql_event.go — whole-input recognizer for the MySQL event family
// (issue #109 T30): CREATE [DEFINER=...] EVENT [IF NOT EXISTS] name
// ON SCHEDULE <AT|EVERY ...> [ON COMPLETION [NOT] PRESERVE]
// [ENABLE|DISABLE|DISABLE ON SLAVE] [COMMENT 'x'] DO body;
// ALTER [DEFINER=...] EVENT name [ON SCHEDULE ...] [ON COMPLETION ...]
// [RENAME TO name] [ENABLE|DISABLE|DISABLE ON SLAVE] [COMMENT 'x']
// [DO body]; and DROP EVENT [IF EXISTS] name.
//
// Schedule, completion and enabled-state clauses are modeled as authored
// facts — clock values are captured verbatim and never evaluated. The DO
// body reuses the routine member machinery; event definitions stay inert
// (creating or enabling an event never executes its body at audit time).

type eventStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.EventDeclaration
	inner       []routineInnerMember
}

// parseMySQLEvent claims whole-input CREATE|ALTER|DROP EVENT forms.
// Anything ambiguous falls through to the real grammar; a claimed
// statement never produces a partial node.
func parseMySQLEvent(sqlText string) (ast.StmtNode, []error, bool) {
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
		return parseMySQLCreateEvent(sqlText, toks)
	case toks[0].wordIs(sqlText, "alter"):
		return parseMySQLAlterEvent(sqlText, toks)
	case toks[0].wordIs(sqlText, "drop"):
		return parseMySQLDropEvent(sqlText, toks)
	}
	return nil, nil, false
}

// eventClauseBoundary reports whether toks[i] opens a clause boundary
// keyword that terminates a schedule sub-expression: ON COMPLETION,
// RENAME TO, ENABLE, DISABLE, COMMENT, DO, STARTS, ENDS, or the next
// schedule keyword inside ALTER's clause loop.
func eventClauseBoundary(sqlText string, toks []sqlTok, i int, stops ...string) bool {
	if i >= len(toks) || toks[i].kind != tokWord {
		return false
	}
	for _, s := range stops {
		if toks[i].wordIs(sqlText, s) {
			return true
		}
	}
	return false
}

// eventExprSlice captures the verbatim authored text of a schedule
// sub-expression starting at toks[i] up to the next clause boundary.
func eventExprSlice(sqlText string, toks []sqlTok, i int, stops ...string) (string, int) {
	j := i
	for j < len(toks) && !eventClauseBoundary(sqlText, toks, j, stops...) {
		j++
	}
	if j == i {
		return "", i
	}
	return strings.TrimSpace(sqlText[toks[i].start:toks[j-1].end]), j
}

var eventExprStops = []string{"on", "rename", "enable", "disable", "comment", "do", "starts", "ends"}

// parseEventSchedule consumes `AT <expr>` or `EVERY <expr> [STARTS
// <expr>] [ENDS <expr>]` starting at toks[i] and returns the schedule
// plus the index just past it. Returns nil when no schedule opens.
func parseEventSchedule(sqlText string, toks []sqlTok, i int) (*spec.EventSchedule, int) {
	if i >= len(toks) {
		return nil, i
	}
	s := &spec.EventSchedule{}
	switch {
	case toks[i].wordIs(sqlText, "at"):
		s.Kind = spec.EventScheduleAt
		text, j := eventExprSlice(sqlText, toks, i+1, eventExprStops...)
		if text == "" {
			return nil, i
		}
		s.At = text
		return s, j
	case toks[i].wordIs(sqlText, "every"):
		s.Kind = spec.EventScheduleEvery
		text, j := eventExprSlice(sqlText, toks, i+1, eventExprStops...)
		if text == "" {
			return nil, i
		}
		s.Every = text
		i = j
	default:
		return nil, i
	}
	// Optional STARTS/ENDS sub-expressions.
	for i < len(toks) {
		switch {
		case toks[i].wordIs(sqlText, "starts"):
			text, j := eventExprSlice(sqlText, toks, i+1, "ends", "on", "rename", "enable", "disable", "comment", "do")
			if text == "" {
				return nil, i
			}
			s.Starts = text
			i = j
		case toks[i].wordIs(sqlText, "ends"):
			text, j := eventExprSlice(sqlText, toks, i+1, "on", "rename", "enable", "disable", "comment", "do")
			if text == "" {
				return nil, i
			}
			s.Ends = text
			i = j
		default:
			return s, i
		}
	}
	return s, i
}

// Clause ranks mirror the authored MySQL grammar order. The claim is
// strict — out-of-order or duplicated clauses decline to the base
// parser rather than fabricating a declaration (T30).
const (
	evPhaseSchedule = iota
	evPhaseCompletion
	evPhaseRename
	evPhaseStatus
	evPhaseComment
)

// parseEventClauses consumes the shared CREATE/ALTER event clause loop
// starting at toks[i], enforcing grammar order and single authorship.
// CREATE stops at DO (body required) and never allows RENAME; ALTER
// may also stop at DO (optional body) or run out of tokens. Returns
// the index at DO (or len(toks) when no DO exists), or -1 when an
// out-of-order, duplicated, or unknown token appears — the caller then
// declines the claim.
func parseEventClauses(sqlText string, toks []sqlTok, i int, decl *spec.EventDeclaration, allowRename bool, phase int) int {
	for i < len(toks) {
		switch {
		case toks[i].wordIs(sqlText, "on") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "schedule"):
			if phase >= evPhaseSchedule {
				return -1
			}
			phase = evPhaseSchedule
			sched, j := parseEventSchedule(sqlText, toks, i+2)
			if sched == nil {
				return -1
			}
			decl.Schedule = sched
			i = j
		case toks[i].wordIs(sqlText, "on") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "completion"):
			if phase >= evPhaseCompletion {
				return -1
			}
			phase = evPhaseCompletion
			i += 2
			preserve := true
			if i < len(toks) && toks[i].wordIs(sqlText, "not") {
				preserve = false
				i++
			}
			if i >= len(toks) || !toks[i].wordIs(sqlText, "preserve") {
				return -1
			}
			decl.OnCompletion = &preserve
			i++
		case toks[i].wordIs(sqlText, "rename") && i+1 < len(toks) && toks[i+1].wordIs(sqlText, "to"):
			if !allowRename || phase >= evPhaseRename {
				return -1
			}
			phase = evPhaseRename
			schema, name, next, ok := triggerQualifiedName(sqlText, toks, i+2)
			if !ok {
				return -1
			}
			decl.RenameTo = &spec.Table{Schema: schema, Name: name}
			i = next
		case toks[i].wordIs(sqlText, "enable"):
			if phase >= evPhaseStatus {
				return -1
			}
			phase = evPhaseStatus
			decl.Status, decl.StatusSet = spec.EventStatusEnabled, true
			i++
		case toks[i].wordIs(sqlText, "disable"):
			if phase >= evPhaseStatus {
				return -1
			}
			phase = evPhaseStatus
			decl.Status, decl.StatusSet = spec.EventStatusDisabled, true
			i++
			if i+1 < len(toks) && toks[i].wordIs(sqlText, "on") && toks[i+1].wordIs(sqlText, "slave") {
				decl.Status = spec.EventStatusDisableOnSlave
				i += 2
			}
		case toks[i].wordIs(sqlText, "comment"):
			if phase >= evPhaseComment {
				return -1
			}
			phase = evPhaseComment
			i++
			if i >= len(toks) || toks[i].kind != tokStr {
				return -1
			}
			decl.CommentDeclared = true
			i++
		case toks[i].wordIs(sqlText, "do"):
			return i
		default:
			return -1
		}
	}
	return i
}

// parseEventBody consumes `DO <single stmt|BEGIN…END>` at toks[i] and
// builds the routine body; returns the body, inner members, and whether
// the tokens end exactly there.
func parseEventBody(sqlText string, toks []sqlTok, i int) (*spec.RoutineBody, []routineInnerMember, bool) {
	if i >= len(toks) || !toks[i].wordIs(sqlText, "do") {
		return nil, nil, false
	}
	i++
	if i >= len(toks) {
		return nil, nil, false
	}
	var body *spec.RoutineBody
	var inner []routineInnerMember
	if toks[i].wordIs(sqlText, "begin") {
		endTok := routineScanEnd(sqlText, toks, i+1)
		if endTok < 0 {
			return nil, nil, false // unbalanced body — never claim
		}
		body = &spec.RoutineBody{Compound: true, Offset: toks[i].start}
		inner = splitRoutineBodyMembers(sqlText, toks[i+1:endTok])
		if endTok+1 != len(toks) {
			return nil, nil, false
		}
	} else {
		body = &spec.RoutineBody{Compound: false, Offset: toks[i].start}
		inner = splitRoutineBodyMembers(sqlText, toks[i:])
	}
	if len(inner) == 0 {
		return nil, nil, false
	}
	body.Statements = make([]spec.RoutineBodyStatement, 0, len(inner))
	return body, inner, true
}

// parseEventDefiner consumes an optional `DEFINER = ...` clause before
// the EVENT keyword at toks[i]; the clause shape is recorded, never the
// account payload. Returns the index at EVENT or -1.
func parseEventDefiner(sqlText string, toks []sqlTok, i int, decl *spec.EventDeclaration) int {
	if i < len(toks) && toks[i].wordIs(sqlText, "definer") {
		i++
		if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
			i++
		}
		start := i
		for i < len(toks) && !toks[i].wordIs(sqlText, "event") {
			i++
		}
		if start >= i {
			return -1 // DEFINER without a payload is malformed
		}
		val := strings.TrimSpace(sqlText[toks[start].start:toks[i-1].end])
		if strings.EqualFold(val, "current_user") || strings.HasPrefix(strings.ToLower(val), "current_user(") {
			decl.Definer = "current_user"
		} else {
			decl.Definer = "named"
		}
	}
	if i >= len(toks) || !toks[i].wordIs(sqlText, "event") {
		return -1
	}
	return i + 1
}

func parseMySQLCreateEvent(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	decl := &spec.EventDeclaration{Kind: spec.EventKindCreate}
	i := parseEventDefiner(sqlText, toks, 1, decl)
	if i < 0 {
		return nil, nil, false
	}
	if i+2 < len(toks) && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "not") && toks[i+2].wordIs(sqlText, "exists") {
		decl.IfNotExists = true
		i += 3
	}
	schema, name, next, ok := triggerQualifiedName(sqlText, toks, i)
	if !ok {
		return nil, nil, false
	}
	decl.Name.Schema, decl.Name.Name = schema, name
	i = next
	// ON SCHEDULE is required for CREATE EVENT.
	if i+1 >= len(toks) || !toks[i].wordIs(sqlText, "on") || !toks[i+1].wordIs(sqlText, "schedule") {
		return nil, nil, false
	}
	sched, j := parseEventSchedule(sqlText, toks, i+2)
	if sched == nil {
		return nil, nil, false
	}
	decl.Schedule = sched
	i = j
	i = parseEventClauses(sqlText, toks, i, decl, false, evPhaseSchedule)
	if i < 0 || i >= len(toks) || !toks[i].wordIs(sqlText, "do") {
		return nil, nil, false
	}
	body, inner, ok := parseEventBody(sqlText, toks, i)
	if !ok {
		return nil, nil, false
	}
	decl.Body = body
	return finishEventStatement(sqlText, decl, inner, spec.DDLOperationCreateEvent)
}

func parseMySQLAlterEvent(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	decl := &spec.EventDeclaration{Kind: spec.EventKindAlter}
	i := parseEventDefiner(sqlText, toks, 1, decl)
	if i < 0 {
		return nil, nil, false
	}
	schema, name, next, ok := triggerQualifiedName(sqlText, toks, i)
	if !ok {
		return nil, nil, false
	}
	decl.Name.Schema, decl.Name.Name = schema, name
	i = next
	i = parseEventClauses(sqlText, toks, i, decl, true, -1)
	if i < 0 {
		return nil, nil, false
	}
	if i < len(toks) && toks[i].wordIs(sqlText, "do") {
		body, inner, ok := parseEventBody(sqlText, toks, i)
		if !ok {
			return nil, nil, false
		}
		decl.Body = body
		return finishEventStatement(sqlText, decl, inner, spec.DDLOperationAlterEvent)
	}
	if i != len(toks) {
		return nil, nil, false
	}
	if decl.Schedule == nil && decl.OnCompletion == nil && !decl.StatusSet && decl.RenameTo == nil && !decl.CommentDeclared {
		return nil, nil, false // bare `ALTER EVENT name` claims nothing
	}
	return finishEventStatement(sqlText, decl, nil, spec.DDLOperationAlterEvent)
}

func parseMySQLDropEvent(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	decl := &spec.EventDeclaration{Kind: spec.EventKindDrop}
	i := 1
	if i >= len(toks) || !toks[i].wordIs(sqlText, "event") {
		return nil, nil, false
	}
	i++
	if i+1 < len(toks) && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "exists") {
		decl.IfExists = true
		i += 2
	}
	schema, name, next, ok := triggerQualifiedName(sqlText, toks, i)
	if !ok {
		return nil, nil, false
	}
	decl.Name.Schema, decl.Name.Name = schema, name
	i = next
	if i != len(toks) {
		return nil, nil, false
	}
	return finishEventStatement(sqlText, decl, nil, spec.DDLOperationDropEvent)
}

func finishEventStatement(sqlText string, decl *spec.EventDeclaration, inner []routineInnerMember, op spec.DDLOperation) (ast.StmtNode, []error, bool) {
	node := &eventStatement{operation: op, declaration: decl, inner: inner}
	carrier := &ast.ProcedureInfo{}
	carrier.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	node.StmtNode = carrier
	return node, nil, true
}

// retagEventMemberFeatures rewrites routine.body.* member features into
// the event namespace — the member machinery is shared but the evidence
// names the owning object family (T30).
func retagEventMemberFeatures(body *spec.RoutineBody) {
	eachTriggerBodyMember(body, func(bs *spec.RoutineBodyStatement) {
		if u := bs.Unsupported; u != nil {
			u.Feature = strings.Replace(u.Feature, "routine.body.", "event.body.", 1)
		}
	})
}
