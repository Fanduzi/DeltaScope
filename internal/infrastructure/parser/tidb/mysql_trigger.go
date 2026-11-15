package tidbparser

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysql_trigger.go — whole-input recognizer for the MySQL trigger
// family (issue #108 T29): CREATE [DEFINER=...] TRIGGER name
// BEFORE|AFTER INSERT|UPDATE|DELETE ON tbl FOR EACH ROW
// [FOLLOWS|PRECEDES name] body, and DROP TRIGGER [IF EXISTS] name.
// The FOR EACH ROW body reuses the routine member machinery — compound
// constructs, declarations, bindings, and flow verbs classify exactly
// as in stored routines. OLD.<col>/NEW.<col> pseudo-column usage is
// recorded lexically so the audit binds it to the trigger target and
// event — never treated as a table reference.

type triggerStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.TriggerDeclaration
	inner       []routineInnerMember
}

// parseMySQLTrigger claims whole-input CREATE|DROP TRIGGER forms.
// Anything ambiguous falls through to the real grammar; a claimed
// statement never produces a partial node.
func parseMySQLTrigger(sqlText string) (ast.StmtNode, []error, bool) {
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
		return parseMySQLCreateTrigger(sqlText, toks)
	case toks[0].wordIs(sqlText, "drop"):
		return parseMySQLDropTrigger(sqlText, toks)
	}
	return nil, nil, false
}

func parseMySQLCreateTrigger(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	decl := &spec.TriggerDeclaration{Kind: "create"}
	i := 1
	// Optional DEFINER clause — shape recorded, payload never stored.
	if i < len(toks) && toks[i].wordIs(sqlText, "definer") {
		i++
		if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
			i++
		}
		start := i
		for i < len(toks) && !toks[i].wordIs(sqlText, "trigger") {
			i++
		}
		if start < i {
			val := strings.TrimSpace(sqlText[toks[start].start:toks[i-1].end])
			if strings.EqualFold(val, "current_user") || strings.HasPrefix(strings.ToLower(val), "current_user(") {
				decl.Definer = "current_user"
			} else {
				decl.Definer = "named"
			}
		}
	}
	if i >= len(toks) || !toks[i].wordIs(sqlText, "trigger") {
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
	// Timing + event.
	if i >= len(toks) {
		return nil, nil, false
	}
	switch {
	case toks[i].wordIs(sqlText, "before"):
		decl.Timing = spec.TriggerTimingBefore
	case toks[i].wordIs(sqlText, "after"):
		decl.Timing = spec.TriggerTimingAfter
	default:
		return nil, nil, false
	}
	i++
	if i >= len(toks) {
		return nil, nil, false
	}
	switch {
	case toks[i].wordIs(sqlText, "insert"):
		decl.Event = spec.TriggerEventInsert
	case toks[i].wordIs(sqlText, "update"):
		decl.Event = spec.TriggerEventUpdate
	case toks[i].wordIs(sqlText, "delete"):
		decl.Event = spec.TriggerEventDelete
	default:
		return nil, nil, false
	}
	i++
	// ON <table> FOR EACH ROW
	if i >= len(toks) || !toks[i].wordIs(sqlText, "on") {
		return nil, nil, false
	}
	i++
	if i >= len(toks) || toks[i].kind == tokPunct {
		return nil, nil, false
	}
	target := routineNameToken(sqlText, toks[i])
	if target == "" {
		return nil, nil, false
	}
	decl.Target.Name = target
	i++
	if i+1 < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." {
		if next := routineNameToken(sqlText, toks[i+1]); next != "" {
			decl.Target.Schema = target
			decl.Target.Name = next
			i += 2
		}
	}
	if i+2 >= len(toks) || !toks[i].wordIs(sqlText, "for") || !toks[i+1].wordIs(sqlText, "each") || !toks[i+2].wordIs(sqlText, "row") {
		return nil, nil, false
	}
	i += 3
	// Optional FOLLOWS|PRECEDES <trigger> (MySQL 8.0.23+).
	if i+1 < len(toks) && (toks[i].wordIs(sqlText, "follows") || toks[i].wordIs(sqlText, "precedes")) {
		kind := strings.ToLower(toks[i].text(sqlText))
		if sib := routineNameToken(sqlText, toks[i+1]); sib != "" {
			decl.Order = &spec.TriggerOrder{Kind: kind, Name: sib}
			i += 2
		}
	}
	if i >= len(toks) {
		return nil, nil, false
	}
	// Body: BEGIN…END compound or a single statement — the routine
	// member machinery classifies either.
	var body *spec.RoutineBody
	var inner []routineInnerMember
	if toks[i].wordIs(sqlText, "begin") {
		beginTok := i
		endTok := routineScanEnd(sqlText, toks, beginTok+1)
		if endTok < 0 {
			return nil, nil, false // unbalanced body — never claim
		}
		body = &spec.RoutineBody{Compound: true, Offset: toks[beginTok].start}
		inner = splitRoutineBodyMembers(sqlText, toks[beginTok+1:endTok])
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
	decl.Body = body
	scanTriggerOldNew(sqlText, toks[i:], decl)
	return finishTriggerStatement(sqlText, decl, inner, spec.DDLOperationCreateTrigger)
}

func parseMySQLDropTrigger(sqlText string, toks []sqlTok) (ast.StmtNode, []error, bool) {
	decl := &spec.TriggerDeclaration{Kind: "drop"}
	i := 1
	if i >= len(toks) || !toks[i].wordIs(sqlText, "trigger") {
		return nil, nil, false
	}
	i++
	if i+2 < len(toks) && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "exists") {
		decl.IfExists = true
		i += 2
	}
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
	if i != len(toks) {
		return nil, nil, false
	}
	return finishTriggerStatement(sqlText, decl, nil, spec.DDLOperationDropTrigger)
}

func finishTriggerStatement(sqlText string, decl *spec.TriggerDeclaration, inner []routineInnerMember, op spec.DDLOperation) (ast.StmtNode, []error, bool) {
	node := &triggerStatement{operation: op, declaration: decl, inner: inner}
	carrier := &ast.ProcedureInfo{}
	carrier.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
	node.StmtNode = carrier
	return node, nil, true
}

// retagTriggerMemberFeatures rewrites routine.body.* member features
// into the trigger namespace — the member machinery is shared but the
// evidence names the owning object family (T29).
func retagTriggerMemberFeatures(body *spec.RoutineBody) {
	for i := range body.Statements {
		bs := &body.Statements[i]
		if u := bs.Statement.Unsupported; u != nil {
			u.Feature = strings.Replace(u.Feature, "routine.body.", "trigger.body.", 1)
		}
		if c := bs.Compound; c != nil {
			for _, br := range c.Branches {
				retagTriggerMemberFeatures(br)
			}
			if c.Body != nil {
				retagTriggerMemberFeatures(c.Body)
			}
		}
		if d := bs.Declare; d != nil && d.Member != nil {
			if u := d.Member.Statement.Unsupported; u != nil {
				u.Feature = strings.Replace(u.Feature, "routine.body.", "trigger.body.", 1)
			}
		}
	}
}

// scanTriggerOldNew records OLD.<col>/NEW.<col> pseudo-column usage:
// an old|new word immediately followed by a '.' token. Bare names are
// ordinary identifiers — only the qualified shape is a pseudo-column.
func scanTriggerOldNew(sqlText string, toks []sqlTok, decl *spec.TriggerDeclaration) {
	for j := 0; j+1 < len(toks); j++ {
		if toks[j+1].kind != tokPunct || toks[j+1].text(sqlText) != "." {
			continue
		}
		if toks[j].kind != tokWord {
			continue
		}
		switch strings.ToLower(toks[j].text(sqlText)) {
		case "old":
			decl.UsesOld = true
		case "new":
			decl.UsesNew = true
		}
	}
}
