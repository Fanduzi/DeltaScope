package tidbparser

import (
	"sort"
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
	decl := &spec.TriggerDeclaration{Kind: spec.TriggerKindCreate}
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
	nameSchema, nameIdent, nameNext, nameOK := triggerQualifiedName(sqlText, toks, i)
	if !nameOK {
		return nil, nil, false
	}
	decl.Name.Schema, decl.Name.Name = nameSchema, nameIdent
	i = nameNext
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
	targetSchema, targetName, targetNext, targetOK := triggerQualifiedName(sqlText, toks, i)
	if !targetOK {
		return nil, nil, false
	}
	decl.Target.Schema, decl.Target.Name = targetSchema, targetName
	i = targetNext
	if i+2 >= len(toks) || !toks[i].wordIs(sqlText, "for") || !toks[i+1].wordIs(sqlText, "each") || !toks[i+2].wordIs(sqlText, "row") {
		return nil, nil, false
	}
	i += 3
	// Optional FOLLOWS|PRECEDES <trigger> (MySQL 5.7.2+, WL#3253).
	if i+1 < len(toks) && (toks[i].wordIs(sqlText, "follows") || toks[i].wordIs(sqlText, "precedes")) {
		kind := strings.ToLower(toks[i].text(sqlText))
		if sib := routineNameToken(sqlText, toks[i+1]); sib != "" {
			decl.Order = &spec.TriggerOrder{Kind: spec.TriggerOrderKind(kind), Name: sib}
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
	decl := &spec.TriggerDeclaration{Kind: spec.TriggerKindDrop}
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
	nameSchema, nameIdent, nameNext, nameOK := triggerQualifiedName(sqlText, toks, i)
	if !nameOK {
		return nil, nil, false
	}
	decl.Name.Schema, decl.Name.Name = nameSchema, nameIdent
	i = nameNext
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

// triggerQualifiedName reads one [schema.]name pair starting at toks[i]
// and returns the normalized parts plus the index just past the pair.
func triggerQualifiedName(sqlText string, toks []sqlTok, i int) (schema, name string, next int, ok bool) {
	if i >= len(toks) || toks[i].kind == tokPunct {
		return "", "", i, false
	}
	name = routineNameToken(sqlText, toks[i])
	if name == "" {
		return "", "", i, false
	}
	next = i + 1
	if next+1 < len(toks) && toks[next].kind == tokPunct && toks[next].text(sqlText) == "." {
		if sibling := routineNameToken(sqlText, toks[next+1]); sibling != "" {
			schema, name, next = name, sibling, next+2
		}
	}
	return schema, name, next, true
}

// eachTriggerBodyMember visits every member of a trigger body: flat
// members, compound branches and bodies, and handler/cursor executable
// members — matching the walk the evaluator performs.
func eachTriggerBodyMember(body *spec.RoutineBody, fn func(*spec.RoutineBodyStatement)) {
	for i := range body.Statements {
		bs := &body.Statements[i]
		fn(bs)
		if c := bs.Compound; c != nil {
			for _, br := range c.Branches {
				eachTriggerBodyMember(br, fn)
			}
			if c.Body != nil {
				eachTriggerBodyMember(c.Body, fn)
			}
		}
		if d := bs.Declare; d != nil && d.Member != nil {
			fn(d.Member)
			if c := d.Member.Compound; c != nil {
				for _, br := range c.Branches {
					eachTriggerBodyMember(br, fn)
				}
				if c.Body != nil {
					eachTriggerBodyMember(c.Body, fn)
				}
			}
		}
	}
}

// retagTriggerMemberFeatures rewrites routine.body.* member features
// into the trigger namespace — the member machinery is shared but the
// evidence names the owning object family (T29).
func retagTriggerMemberFeatures(body *spec.RoutineBody) {
	eachTriggerBodyMember(body, func(bs *spec.RoutineBodyStatement) {
		if u := bs.Unsupported; u != nil {
			u.Feature = strings.Replace(u.Feature, "routine.body.", "trigger.body.", 1)
		}
	})
}

// scanTriggerOldNew records OLD.<col>/NEW.<col> pseudo-column usage on
// the declaration: an old|new word immediately followed by a '.' token.
// Bare names are ordinary identifiers — only the qualified shape is a
// pseudo-column. Referenced columns land sorted-unique so the column
// dependencies the body reads stay explicit.
func scanTriggerOldNew(sqlText string, toks []sqlTok, decl *spec.TriggerDeclaration) {
	decl.UsesOld, decl.UsesNew, decl.OldColumns, decl.NewColumns = triggerOldNewUse(sqlText, toks)
}

// triggerOldNewUse collects OLD.<col>/NEW.<col> usage in one token span.
func triggerOldNewUse(sqlText string, toks []sqlTok) (usesOld, usesNew bool, oldCols, newCols []string) {
	for j := 0; j+1 < len(toks); j++ {
		if toks[j+1].kind != tokPunct || toks[j+1].text(sqlText) != "." {
			continue
		}
		if toks[j].kind != tokWord {
			continue
		}
		column := ""
		if j+2 < len(toks) && toks[j+2].kind == tokWord {
			column = toks[j+2].identText(sqlText)
		}
		switch strings.ToLower(toks[j].text(sqlText)) {
		case "old":
			usesOld = true
			if column != "" {
				oldCols = appendUniqueSorted(oldCols, column)
			}
		case "new":
			usesNew = true
			if column != "" {
				newCols = appendUniqueSorted(newCols, column)
			}
		}
	}
	return usesOld, usesNew, oldCols, newCols
}

// applyTriggerOldNewBounds marks every modeled body member whose
// pseudo-column use is impossible under the declared event as bounded
// trigger.body.old_new evidence — the violating member stays explicit
// while sibling members still audit (T29).
func applyTriggerOldNewBounds(decl *spec.TriggerDeclaration) {
	if decl == nil || decl.Body == nil || decl.Kind != spec.TriggerKindCreate {
		return
	}
	eachTriggerBodyMember(decl.Body, func(bs *spec.RoutineBodyStatement) {
		if bs.Compound != nil || bs.Declare != nil || bs.Unsupported != nil {
			return
		}
		memberToks, _ := tokenizeSQLState(bs.RawSQL)
		usesOld, usesNew, oldCols, newCols := triggerOldNewUse(bs.RawSQL, memberToks)
		if (!usesOld || decl.Event.AllowsOld()) && (!usesNew || decl.Event.AllowsNew()) {
			return
		}
		meta := map[string]any{
			"trigger": decl.Name.Name, "event": string(decl.Event),
			"uses_old": usesOld, "uses_new": usesNew,
		}
		if usesOld {
			meta["old_columns"] = oldCols
		}
		if usesNew {
			meta["new_columns"] = newCols
		}
		bs.Unsupported = &spec.UnsupportedDetail{
			Feature:  "trigger.body.old_new",
			Reason:   spec.UnsupportedUnauditedReason,
			Metadata: meta,
		}
	})
}

func appendUniqueSorted(list []string, value string) []string {
	i := sort.SearchStrings(list, value)
	if i < len(list) && list[i] == value {
		return list
	}
	list = append(list, "")
	copy(list[i+1:], list[i:])
	list[i] = value
	return list
}
