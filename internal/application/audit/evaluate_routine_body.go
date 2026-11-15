// Package audit orchestrates audit use cases at the application layer.
package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// evaluate_routine_body.go — stored-routine body evaluation (T28/#107).
// Body members fold through the shared statement machinery under a
// conservative variable environment: statically proven literals audit
// like authored members, anything indeterminate stays bounded
// incomplete evidence — never executed, never a fabricated pass.

// routineRootMember marks a walk that supplies each member's own index;
// nested walks pass their enclosing member's index instead.
const routineRootMember = -1

// evaluateRoutineBody audits each statically bounded body member of a
// stored-routine definition through the shared registry. Member findings
// keep their own rule IDs and levels; their locations are translated
// from parent-raw space into file space using the parent's line/column,
// and every finding is tagged with the owning routine and member index
// (T27/#106). Unparseable members surface as unsupported details instead
// of being silently skipped.
//
// T28/#107: control-flow members recurse — inner members audit through
// the same machinery while variable bindings merge conservatively
// across branches and never survive a loop body unchanged-unless-proven.
// PREPARE resolves statically known literal sources (direct or
// SET-bound); everything else stays a bounded incomplete member.
func evaluateRoutineBody(ctx context.Context, registry *rule.Registry, parent spec.Statement, parentIndex int) ([]rule.Finding, []spec.UnsupportedDetail, *rule.StatementEvaluation, error) {
	decl := parent.DDL.Routine
	w := &routineBodyWalker{
		ctx:         ctx,
		registry:    registry,
		parent:      parent,
		parentIndex: parentIndex,
		routine:     decl.Name.Name,
		dialect:     parent.Dialect,
		combined:    &rule.StatementEvaluation{},
		findings:    make([]rule.Finding, 0),
		unsupported: make([]spec.UnsupportedDetail, 0),
	}
	env := map[string]*string{}
	for _, p := range decl.Params {
		env[p.Name] = nil // parameters are caller-supplied — never static
	}
	w.walkBody(decl.Body, env, routineRootMember)
	if w.err != nil {
		return nil, nil, nil, w.err
	}
	return w.findings, w.unsupported, w.combined, nil
}

// routineBodyWalker folds one routine body through the shared registry
// tracking variable bindings so dynamic-SQL sources resolve only to
// statically proven literals. env maps variable name → *literal: nil
// means bound-but-dynamic, absence means never bound (T28/#107).
type routineBodyWalker struct {
	ctx         context.Context
	registry    *rule.Registry
	parent      spec.Statement
	parentIndex int
	routine     string
	dialect     spec.Dialect
	combined    *rule.StatementEvaluation
	findings    []rule.Finding
	unsupported []spec.UnsupportedDetail
	err         error
}

func (w *routineBodyWalker) walkBody(body *spec.RoutineBody, env map[string]*string, topMember int) {
	for i, bs := range body.Statements {
		if w.err != nil {
			return
		}
		if err := w.ctx.Err(); err != nil {
			w.err = err
			return
		}
		memberIdx := topMember
		if memberIdx == routineRootMember {
			memberIdx = i
		}
		w.walkMember(bs, env, memberIdx)
	}
}

func (w *routineBodyWalker) walkMember(bs spec.RoutineBodyStatement, env map[string]*string, memberIdx int) {
	if w.err != nil {
		return
	}
	switch {
	case bs.Compound != nil:
		w.walkCompound(bs.Compound, env, memberIdx)
	case bs.Declare != nil:
		w.walkDeclare(bs.Declare, env, memberIdx)
	case bs.Set != nil:
		for _, a := range bs.Set.Assignments {
			if a.HasLiteral {
				lit := a.Literal
				env[a.Target] = &lit
			} else {
				env[a.Target] = nil
			}
		}
		if bs.Set.Unmodeled {
			w.appendMemberGap(bs.Statement, memberIdx, "routine.body.set", nil)
		}
	case bs.Prepare != nil:
		w.walkPrepare(bs, env, memberIdx)
	case bs.Flow != "":
		// Recognized flow control stays bounded evidence — the verb's
		// effects are never audited as SQL. Verbs that rebind variables
		// or redirect control flow also invalidate every proven binding:
		// after them nothing sequential can be proven (T28).
		w.appendMemberGap(bs.Statement, memberIdx, "routine.body.flow", map[string]any{"flow": bs.Flow})
		switch bs.Flow {
		case "leave", "iterate", "signal", "resignal", "fetch", "get":
			clearRoutineEnv(env)
		}
	default:
		w.evalMemberStatement(bs.Statement, env, memberIdx)
	}
}

// walkCompound folds a control-flow member. Branched constructs evaluate
// every arm and merge bindings by unanimity — a fact only survives when
// every possible path agrees on it. Loop bodies run zero-or-more times,
// so any binding the body could have changed becomes unknown (T28).
func (w *routineBodyWalker) walkCompound(c *spec.RoutineCompound, env map[string]*string, memberIdx int) {
	switch {
	case len(c.Branches) > 0 || c.Kind == "if" || c.Kind == "case":
		outs := make([]map[string]*string, 0, len(c.Branches)+1)
		for _, br := range c.Branches {
			envB := copyRoutineEnv(env)
			w.walkBody(br, envB, memberIdx)
			outs = append(outs, envB)
		}
		// An IF/CASE without ELSE keeps the pre-branch bindings as one
		// implicit path — no arm executes at all.
		if !c.HasElse {
			outs = append(outs, copyRoutineEnv(env))
		}
		mergeRoutineEnvs(env, outs)
	case c.Kind == "begin":
		if c.Body != nil {
			w.walkBody(c.Body, env, memberIdx)
		}
	default: // while|loop|repeat — zero-or-more iterations
		if c.Body == nil {
			return
		}
		envB := copyRoutineEnv(env)
		w.walkBody(c.Body, envB, memberIdx)
		for name, v := range envB {
			if !routineEnvValueEqual(env[name], v) {
				env[name] = nil
			}
		}
	}
}

func (w *routineBodyWalker) walkDeclare(d *spec.RoutineDeclare, env map[string]*string, memberIdx int) {
	switch d.Kind {
	case "variable":
		for _, name := range d.Names {
			env[name] = nil // declared but unassigned — never a literal
		}
	case "handler", "cursor":
		if d.Member != nil {
			// Handler bodies run only when their condition fires —
			// bindings inside never leak into the sequential env.
			envH := copyRoutineEnv(env)
			w.walkMember(*d.Member, envH, memberIdx)
		}
	case "condition":
		// Definitional fact — no executable member.
	}
}

// walkPrepare resolves the statically known part of a PREPARE source.
// A literal source — direct or resolved through SET bindings — audits
// like a member; anything else is a bounded dynamic gap (T28).
func (w *routineBodyWalker) walkPrepare(bs spec.RoutineBodyStatement, env map[string]*string, memberIdx int) {
	p := bs.Prepare
	literal, known := "", false
	switch p.Source.Kind {
	case "literal":
		literal, known = p.Source.Literal, true
	case "var":
		if v, ok := env[p.Source.Var]; ok && v != nil {
			literal, known = *v, true
		}
	}
	if !known {
		w.appendMemberGap(bs.Statement, memberIdx, "routine.body.prepare", map[string]any{"source": p.Source.Kind})
		return
	}
	w.auditPreparedSQL(literal, bs, memberIdx, p)
}

// auditPreparedSQL parses and audits a statically proven prepared text.
// The text is verbatim authored SQL — never executed, never inferred
// from runtime values. Findings point at the source expression's own
// authored position; a text that does not parse stays bounded (T28).
func (w *routineBodyWalker) auditPreparedSQL(sqlText string, bs spec.RoutineBodyStatement, memberIdx int, p *spec.RoutinePrepare) {
	memberStmt := bs.Statement
	name := p.Name
	parsed, err := Parse(w.ctx, sqlText, w.dialect)
	if err != nil {
		w.appendMemberGap(memberStmt, memberIdx, "routine.body.prepare", map[string]any{"source": "literal_parse", "prepare": name})
		return
	}
	stmts, err := Extract(w.ctx, parsed)
	if err != nil {
		w.appendMemberGap(memberStmt, memberIdx, "routine.body.prepare", map[string]any{"source": "literal_parse", "prepare": name})
		return
	}
	for _, inner := range stmts {
		if inner.Unsupported != nil {
			w.appendMemberGap(memberStmt, memberIdx, inner.Unsupported.Feature, map[string]any{"prepare": name})
			continue
		}
		eval, err := w.registry.EvaluateStatementDetailed(w.ctx, inner)
		if err != nil {
			w.err = err
			return
		}
		w.combined.AppliedRuleIDs = append(w.combined.AppliedRuleIDs, eval.AppliedRuleIDs...)
		w.combined.Skipped = append(w.combined.Skipped, eval.Skipped...)
		w.combined.EvidenceGaps = append(w.combined.EvidenceGaps, eval.EvidenceGaps...)
		for i := range eval.Findings {
			f := eval.Findings[i]
			// The prepared text has no authored coordinates of its own —
			// report findings at the source expression's authored position.
			f.Location = w.prepareSourceLocation(bs, p.SourceOffset)
			if f.Metadata == nil {
				f.Metadata = map[string]any{}
			}
			f.Metadata["routine"] = w.routine
			f.Metadata["routine_member"] = memberIdx
			f.Metadata["prepare"] = name
			w.findings = append(w.findings, f)
		}
	}
}

// evalMemberStatement audits one ordinary body member through the shared
// registry, then re-anchors findings from parent-raw space into file
// space with routine membership metadata. A bounded member carries
// unmodeled executable effects — it may rebind variables the tracker
// cannot see (EXECUTE runs prepared text that can itself SET, a
// SELECT…INTO or unparseable member is opaque), so every proven literal
// binding is invalidated rather than possibly audited as stale (T28).
func (w *routineBodyWalker) evalMemberStatement(inner spec.Statement, env map[string]*string, memberIdx int) {
	if inner.Unsupported != nil {
		item := *inner.Unsupported
		item.Index = w.parentIndex
		item.SQL = inner.RawSQL
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata["routine"] = w.routine
		item.Metadata["routine_member"] = memberIdx
		w.unsupported = append(w.unsupported, item)
		clearRoutineEnv(env)
		return
	}
	eval, err := w.registry.EvaluateStatementDetailed(w.ctx, inner)
	if err != nil {
		w.err = err
		return
	}
	w.combined.AppliedRuleIDs = append(w.combined.AppliedRuleIDs, eval.AppliedRuleIDs...)
	w.combined.Skipped = append(w.combined.Skipped, eval.Skipped...)
	w.combined.EvidenceGaps = append(w.combined.EvidenceGaps, eval.EvidenceGaps...)
	for i := range eval.Findings {
		f := eval.Findings[i]
		if f.Location == nil && (inner.Line > 0 || inner.Column > 0) {
			f.Location = &rule.Location{Line: inner.Line, Column: inner.Column}
		}
		if f.Location != nil {
			// Inner positions live in parent-raw space; translate to
			// file space using the parent statement's own origin.
			f.Location = rule.ChildLocation(w.parent, f.Location.Line, f.Location.Column)
		}
		if f.Metadata == nil {
			f.Metadata = map[string]any{}
		}
		f.Metadata["routine"] = w.routine
		f.Metadata["routine_member"] = memberIdx
		w.findings = append(w.findings, f)
	}
}

// prepareSourceLocation translates the PREPARE source expression's
// authored offset (parent-raw space) into file space, falling back to
// the member's own position when the offset is not provable.
func (w *routineBodyWalker) prepareSourceLocation(bs spec.RoutineBodyStatement, sourceOffset int) *rule.Location {
	inner := bs.Statement
	if sourceOffset <= bs.Offset || sourceOffset > bs.Offset+len(inner.RawSQL) {
		return rule.ChildLocation(w.parent, inner.Line, inner.Column)
	}
	rel := sourceOffset - bs.Offset
	line, col := inner.Line, inner.Column
	for _, r := range inner.RawSQL[:rel] {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return rule.ChildLocation(w.parent, line, col)
}

func (w *routineBodyWalker) appendMemberGap(inner spec.Statement, memberIdx int, feature string, extra map[string]any) {
	item := spec.UnsupportedDetail{
		Index:   w.parentIndex,
		Feature: feature,
		Reason:  spec.UnsupportedUnauditedReason,
		SQL:     inner.RawSQL,
		Metadata: map[string]any{
			"routine":        w.routine,
			"routine_member": memberIdx,
		},
	}
	for k, v := range extra {
		item.Metadata[k] = v
	}
	w.unsupported = append(w.unsupported, item)
}

// clearRoutineEnv drops every proven literal — an unmodeled executable
// member may have rebound any variable (T28).
func clearRoutineEnv(env map[string]*string) {
	for name := range env {
		env[name] = nil
	}
}

// copyRoutineEnv copies the binding map; literal pointers are immutable
// values so sharing them is safe.
func copyRoutineEnv(env map[string]*string) map[string]*string {
	out := make(map[string]*string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

func routineEnvValueEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// mergeRoutineEnvs keeps a binding only when every branch outcome
// agrees — disagreement degrades to bound-but-dynamic (T28).
func mergeRoutineEnvs(env map[string]*string, outs []map[string]*string) {
	if len(outs) == 0 {
		return
	}
	keys := map[string]bool{}
	for _, o := range outs {
		for name := range o {
			keys[name] = true
		}
	}
	for name := range keys {
		v := outs[0][name]
		for _, o := range outs[1:] {
			if !routineEnvValueEqual(v, o[name]) {
				v = nil
				break
			}
		}
		env[name] = v
	}
}
