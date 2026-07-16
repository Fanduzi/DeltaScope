//go:build postgresql

// Package postgresql defines internal effect-candidate facts for Query Access.
// input: raw parse-tree operator / function / cast nodes during effect traversal
// output: ordered EffectCandidate slices (untrusted; never public Result fields)
// pos: internal facts for future catalog identity resolution (T5); not a trust root
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// EffectCandidateKind is a bounded kind for internal effect candidates.
type EffectCandidateKind string

const (
	// EffectCandidateOperator is an A_Expr (comparison/arithmetic/etc.) effect.
	EffectCandidateOperator EffectCandidateKind = "operator"
	// EffectCandidateFunction is a FuncCall / table-function / function-like effect.
	// Aggregates use the same kind with IsAggregate=true.
	EffectCandidateFunction EffectCandidateKind = "function"
	// EffectCandidateCast is a TypeCast effect.
	EffectCandidateCast EffectCandidateKind = "cast"
)

// OperandKindHint is a structural operand/argument clue for later type resolution.
// It never carries literal values, SQL text, or OIDs.
type OperandKindHint string

const (
	OperandKindColumn   OperandKindHint = "column"
	OperandKindConst    OperandKindHint = "const"
	OperandKindParam    OperandKindHint = "param"
	OperandKindStar     OperandKindHint = "star"
	OperandKindExpr     OperandKindHint = "expr"
	OperandKindSubquery OperandKindHint = "subquery"
	OperandKindUnknown  OperandKindHint = "unknown"
)

// OperandColumnRef records the provenance of a resolved column operand.
// Only populated for base_table bindings; CTE/derived columns are omitted (fail-closed).
type OperandColumnRef struct {
	Schema string
	Table  string
	Column string
}

// EffectCandidate is an internal, untrusted effect fact for future catalog
// identity resolution. It is NOT a trust root and must never appear on
// domain.Result, reason codes, or SDK/CLI/HTTP JSON.
//
// NamePath / TargetTypePath are internal spelling facts for the resolver only;
// public outputs must continue to use only bounded unproven_* reason codes.
type EffectCandidate struct {
	Kind              EffectCandidateKind
	Ordinal           int // stable 0-based traversal order
	NamePath          []string
	ExplicitSchema    bool
	Arity             int
	OperandKinds      []OperandKindHint
	OperandColumnRefs []OperandColumnRef
	IsAggregate       bool
	HasWindow         bool
	HasFilter         bool
	HasDistinct       bool `json:"-"`
	HasAggOrder       bool `json:"-"`
	HasWithinGroup    bool `json:"-"`
	HasFrame          bool `json:"-"`
	TargetTypePath    []string
}

// effectCollector is the single-pass accumulator for unproven effect presence
// flags and structured candidates during the complete SelectStmt walk.
type effectCollector struct {
	flags      effectReasonFlags
	candidates []EffectCandidate
	nextOrd    int
}

func (c *effectCollector) reasonCodes() []string {
	if c == nil {
		return nil
	}
	return c.flags.toReasonCodes()
}

func (c *effectCollector) hasUnprovenEffect() bool {
	if c == nil {
		return false
	}
	return c.flags.operator || c.flags.function || c.flags.cast || c.flags.unsupportedTraversal
}

func (c *effectCollector) appendCandidate(cand EffectCandidate) {
	if c == nil {
		return
	}
	cand.Ordinal = c.nextOrd
	c.nextOrd++
	// Defensive copies so later mutation of source slices cannot alias storage.
	if len(cand.NamePath) > 0 {
		cand.NamePath = append([]string(nil), cand.NamePath...)
	}
	if len(cand.OperandKinds) > 0 {
		cand.OperandKinds = append([]OperandKindHint(nil), cand.OperandKinds...)
	}
	if len(cand.OperandColumnRefs) > 0 {
		cand.OperandColumnRefs = append([]OperandColumnRef(nil), cand.OperandColumnRefs...)
	}
	if len(cand.TargetTypePath) > 0 {
		cand.TargetTypePath = append([]string(nil), cand.TargetTypePath...)
	}
	c.candidates = append(c.candidates, cand)
	switch cand.Kind {
	case EffectCandidateOperator:
		c.flags.operator = true
	case EffectCandidateFunction:
		c.flags.function = true
	case EffectCandidateCast:
		c.flags.cast = true
	}
}

func stringPathFromNodes(nodes []*pg_query.Node) []string {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if s := n.GetString_(); s != nil {
			if v := s.GetSval(); v != "" {
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func typeNamePath(tn *pg_query.TypeName) []string {
	if tn == nil {
		return nil
	}
	// Never read TypeOid — OIDs are not allowed in candidate facts at parse time.
	return stringPathFromNodes(tn.GetNames())
}

func namePathExplicitSchema(path []string) bool {
	return len(path) > 1
}

func operandKindHint(node *pg_query.Node) OperandKindHint {
	if node == nil {
		return OperandKindUnknown
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_ColumnRef:
		return OperandKindColumn
	case *pg_query.Node_AConst:
		return OperandKindConst
	case *pg_query.Node_ParamRef:
		return OperandKindParam
	case *pg_query.Node_AStar:
		return OperandKindStar
	case *pg_query.Node_SubLink:
		return OperandKindSubquery
	case *pg_query.Node_AExpr, *pg_query.Node_BoolExpr, *pg_query.Node_FuncCall,
		*pg_query.Node_TypeCast, *pg_query.Node_CaseExpr, *pg_query.Node_CoalesceExpr,
		*pg_query.Node_RowExpr, *pg_query.Node_MinMaxExpr, *pg_query.Node_NullTest,
		*pg_query.Node_BooleanTest, *pg_query.Node_ArrayExpr, *pg_query.Node_AArrayExpr,
		*pg_query.Node_CollateClause, *pg_query.Node_AIndirection, *pg_query.Node_List:
		return OperandKindExpr
	default:
		return OperandKindUnknown
	}
}

func operandColumnRef(node *pg_query.Node, scope *selectScope) (OperandColumnRef, bool) {
	if node == nil || scope == nil {
		return OperandColumnRef{}, false
	}
	colRef := node.GetColumnRef()
	if colRef == nil {
		return OperandColumnRef{}, false
	}
	resolved := resolveColumnRef(colRef, scope)
	if resolved == nil || !resolved.Resolved || resolved.Column == "*" {
		return OperandColumnRef{}, false
	}
	return OperandColumnRef{
		Schema: resolved.Schema,
		Table:  resolved.Table,
		Column: resolved.Column,
	}, true
}

func recordOperatorCandidate(c *effectCollector, expr *pg_query.A_Expr, scope *selectScope) {
	if c == nil || expr == nil {
		return
	}
	namePath := stringPathFromNodes(expr.GetName())
	var kinds []OperandKindHint
	var colRefs []OperandColumnRef
	arity := 0
	if expr.GetLexpr() != nil {
		kinds = append(kinds, operandKindHint(expr.GetLexpr()))
		arity++
		if ref, ok := operandColumnRef(expr.GetLexpr(), scope); ok {
			colRefs = append(colRefs, ref)
		}
	}
	if expr.GetRexpr() != nil {
		kinds = append(kinds, operandKindHint(expr.GetRexpr()))
		arity++
		if ref, ok := operandColumnRef(expr.GetRexpr(), scope); ok {
			colRefs = append(colRefs, ref)
		}
	}
	c.appendCandidate(EffectCandidate{
		Kind:              EffectCandidateOperator,
		NamePath:          namePath,
		ExplicitSchema:    namePathExplicitSchema(namePath),
		Arity:             arity,
		OperandKinds:      kinds,
		OperandColumnRefs: colRefs,
	})
}

func recordFunctionCandidate(c *effectCollector, fc *pg_query.FuncCall, scope *selectScope) {
	if c == nil || fc == nil {
		return
	}
	namePath := stringPathFromNodes(fc.GetFuncname())
	args := fc.GetArgs()
	kinds := make([]OperandKindHint, 0, len(args)+1)
	arity := len(args)
	if fc.GetAggStar() {
		kinds = append(kinds, OperandKindStar)
		if arity == 0 {
			arity = 0 // COUNT(*) has zero args + star flag
		}
	}
	for _, arg := range args {
		kinds = append(kinds, operandKindHint(arg))
	}
	var colRefs []OperandColumnRef
	for _, arg := range args {
		if ref, ok := operandColumnRef(arg, scope); ok {
			colRefs = append(colRefs, ref)
		}
	}
	// pg_query exposes aggregate syntax bits, not catalog prokind; plain SUM/AVG
	// therefore remain non-aggregate until catalog identity resolution can decide.
	isAgg := fc.GetAggStar() || fc.GetAggDistinct() || fc.GetAggWithinGroup() ||
		fc.GetAggFilter() != nil || len(fc.GetAggOrder()) > 0
	window := fc.GetOver()
	c.appendCandidate(EffectCandidate{
		Kind:              EffectCandidateFunction,
		NamePath:          namePath,
		ExplicitSchema:    namePathExplicitSchema(namePath),
		Arity:             arity,
		OperandKinds:      kinds,
		OperandColumnRefs: colRefs,
		IsAggregate:       isAgg,
		HasWindow:         window != nil,
		HasFilter:         fc.GetAggFilter() != nil,
		HasDistinct:       fc.GetAggDistinct(),
		HasAggOrder:       len(fc.GetAggOrder()) > 0,
		HasWithinGroup:    fc.GetAggWithinGroup(),
		HasFrame:          window != nil && window.GetFrameOptions() != 0,
	})
}

func recordCastCandidate(c *effectCollector, tc *pg_query.TypeCast) {
	if c == nil || tc == nil {
		return
	}
	target := typeNamePath(tc.GetTypeName())
	kinds := []OperandKindHint{operandKindHint(tc.GetArg())}
	c.appendCandidate(EffectCandidate{
		Kind:           EffectCandidateCast,
		NamePath:       nil, // casts are identified by target type path + arg structure
		ExplicitSchema: namePathExplicitSchema(target),
		Arity:          1,
		OperandKinds:   kinds,
		TargetTypePath: target,
	})
}

func recordSyntheticFunctionCandidate(c *effectCollector, namePath []string, arity int, kinds []OperandKindHint) {
	if c == nil {
		return
	}
	c.appendCandidate(EffectCandidate{
		Kind:           EffectCandidateFunction,
		NamePath:       namePath,
		ExplicitSchema: namePathExplicitSchema(namePath),
		Arity:          arity,
		OperandKinds:   kinds,
	})
}

func recordSQLValueFunctionCandidate(c *effectCollector, svf *pg_query.SQLValueFunction) {
	if c == nil || svf == nil {
		return
	}
	name := sqlValueFunctionName(svf.GetOp())
	if name == "" {
		return
	}
	c.appendCandidate(EffectCandidate{
		Kind:           EffectCandidateFunction,
		NamePath:       []string{name},
		ExplicitSchema: false,
		Arity:          0,
		OperandKinds:   nil,
	})
}

func sqlValueFunctionName(op pg_query.SQLValueFunctionOp) string {
	switch op {
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_DATE:
		return "current_date"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_TIME:
		return "current_time"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_TIME_N:
		return "current_time"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_TIMESTAMP:
		return "current_timestamp"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_TIMESTAMP_N:
		return "current_timestamp"
	case pg_query.SQLValueFunctionOp_SVFOP_LOCALTIME:
		return "localtime"
	case pg_query.SQLValueFunctionOp_SVFOP_LOCALTIME_N:
		return "localtime"
	case pg_query.SQLValueFunctionOp_SVFOP_LOCALTIMESTAMP:
		return "localtimestamp"
	case pg_query.SQLValueFunctionOp_SVFOP_LOCALTIMESTAMP_N:
		return "localtimestamp"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_ROLE:
		return "current_role"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_USER:
		return "current_user"
	case pg_query.SQLValueFunctionOp_SVFOP_USER:
		return "user"
	case pg_query.SQLValueFunctionOp_SVFOP_SESSION_USER:
		return "session_user"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_CATALOG:
		return "current_catalog"
	case pg_query.SQLValueFunctionOp_SVFOP_CURRENT_SCHEMA:
		return "current_schema"
	default:
		return ""
	}
}
