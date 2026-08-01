//go:build postgresql

// Package postgresql extracts query access facts from PostgreSQL AST.
// input: SQL text, dialect, default schema
// output: QueryAccessFacts with read classification, relations, column references, and output lineage
// pos: infrastructure adapter for PostgreSQL query access extraction
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// QueryAccessFacts is the intermediate result from PostgreSQL query access extraction.
// The application layer converts this to the domain Result.
type QueryAccessFacts struct {
	ReadClassification            string
	Relations                     []RelationFacts
	ColumnReferences              []ColumnRefFacts
	Outputs                       []OutputFacts
	Unresolved                    []UnresolvedFacts
	ExactCountIntegerOneStatement bool
	// ReasonCodes are bounded machine identifiers for unproven effects.
	// Never SQL text, operator/function/cast names, OIDs, or literals.
	ReasonCodes []string
	// EffectCandidates are internal, untrusted effect facts for future catalog
	// identity resolution. They must not be copied into domain.Result or public JSON.
	EffectCandidates []EffectCandidate
}

// RelationFacts describes a relation reference extracted from the AST.
type RelationFacts struct {
	Schema string
	Name   string
	Alias  string
	Kind   string
}

// ColumnRefFacts describes a column reference with usage contexts.
type ColumnRefFacts struct {
	Schema  string
	Table   string
	Column  string
	Usages  []string
	QualRef string
}

// OutputFacts describes an output column with source lineage.
type OutputFacts struct {
	Name    string
	Sources []string
}

// UnresolvedFacts describes a reference that could not be resolved.
type UnresolvedFacts struct {
	Reference string
	Reason    string
}

// QueryAccessExtractor extracts query access facts from PostgreSQL AST.
type QueryAccessExtractor struct{}

// ExtractQueryAccess parses SQL and extracts query access facts.
func (e *QueryAccessExtractor) ExtractQueryAccess(ctx context.Context, sql string, dialect string, defaultSchema string) (*QueryAccessFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("extract cancelled: %w", err)
	}

	result, err := pg_query.Parse(sql)
	if err != nil {
		return &QueryAccessFacts{ReadClassification: "indeterminate"}, nil
	}

	stmts := result.GetStmts()
	if len(stmts) == 0 {
		return &QueryAccessFacts{ReadClassification: "indeterminate"}, nil
	}

	facts := &QueryAccessFacts{}
	relations := make([]RelationFacts, 0)
	columnRefs := make([]ColumnRefFacts, 0)
	outputs := make([]OutputFacts, 0)
	var collector effectCollector
	var extraReasons []string

	classifications := make([]string, 0, len(stmts))
	for _, rawStmt := range stmts {
		if rawStmt == nil || rawStmt.GetStmt() == nil {
			classifications = append(classifications, "indeterminate")
			continue
		}
		node := rawStmt.GetStmt()
		classifications = append(classifications, classifyStatement(node))
		collectEffects(node, &collector, defaultSchema, nil)

		// Extract relations/columns/outputs from SELECT or EXPLAIN inner query.
		// Skip EXPLAIN ANALYZE (classified as not_read_only because it executes).
		sel := node.GetSelectStmt()
		if sel == nil {
			if explain := node.GetExplainStmt(); explain != nil && !explainHasAnalyze(explain) {
				if inner := explain.GetQuery(); inner != nil {
					sel = inner.GetSelectStmt()
				}
			}
		}
		if sel != nil {
			cteLineage, cteBodyRels := buildCTELineage(sel, defaultSchema)
			relations = append(relations, collectRelations(sel, defaultSchema)...)
			relations = append(relations, cteBodyRels...)
			columnRefs = append(columnRefs, collectColumnReferences(sel, defaultSchema, cteLineage, &extraReasons)...)
			outputs = append(outputs, collectOutputs(sel, defaultSchema, cteLineage)...)
		}
	}

	facts.ReadClassification = foldClassifications(classifications)
	facts.Relations = deduplicateRelations(relations)
	facts.ColumnReferences = columnRefs
	facts.Outputs = outputs
	// Public reason codes remain presence-only machine ids (no candidate names).
	reasonCodes := collector.reasonCodes()
	reasonCodes = append(reasonCodes, extraReasons...)
	facts.ReasonCodes = reasonCodes
	facts.EffectCandidates = collector.candidates
	if len(stmts) == 1 {
		facts.ExactCountIntegerOneStatement = exactCountIntegerOneStatement(stmts[0].GetStmt())
	}

	if hasUnresolvedWildcard(columnRefs) && defaultSchema == "" {
		facts.Unresolved = append(facts.Unresolved, UnresolvedFacts{
			Reference: "*",
			Reason:    "schema_unavailable",
		})
	}

	return facts, nil
}

// effectReasonFlags tracks which unproven effect kinds appear in the statement tree.
// Codes are presence-only; they never capture operator/function/cast spellings.
type effectReasonFlags struct {
	operator             bool
	function             bool
	cast                 bool
	unsupportedTraversal bool
}

func (f effectReasonFlags) toReasonCodes() []string {
	var codes []string
	// Deterministic emission order (application will re-sort alphabetically).
	if f.unsupportedTraversal {
		codes = append(codes, "unsupported_traversal")
	}
	if f.cast {
		codes = append(codes, "unproven_cast_effect")
	}
	if f.function {
		codes = append(codes, "unproven_function_effect")
	}
	if f.operator {
		codes = append(codes, "unproven_operator_effect")
	}
	return codes
}

// collectEffects is the single auditable effect traversal for raw parse trees.
// It records:
//   - bounded public reason presence flags (unproven_*), and
//   - internal EffectCandidate facts for future identity resolution.
//
// Prefer extending this walker over adding ad-hoc field checks elsewhere.
// Structural BoolExpr (AND/OR/NOT) is not a catalog candidate; only children are walked.
func collectEffects(node *pg_query.Node, c *effectCollector, defaultSchema string, cteNames map[string]bool) {
	if node == nil || c == nil {
		return
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		collectSelectEffects(n.SelectStmt, c, defaultSchema, cteNames)
	case *pg_query.Node_ExplainStmt:
		if n.ExplainStmt.GetQuery() != nil {
			collectEffects(n.ExplainStmt.GetQuery(), c, defaultSchema, cteNames)
		}
	default:
		collectNodeEffects(node, c, nil, defaultSchema, cteNames)
	}
}

// collectSelectEffects visits every expression-bearing field of a SelectStmt
// (set ops, CTEs, DISTINCT ON, target, FROM, WHERE, GROUP/HAVING, WINDOW,
// VALUES, ORDER BY, LIMIT/OFFSET).
func collectSelectEffects(sel *pg_query.SelectStmt, c *effectCollector, defaultSchema string, parentCTENames map[string]bool) {
	if sel == nil || c == nil {
		return
	}
	if sel.GetLarg() != nil {
		collectSelectEffects(sel.GetLarg(), c, defaultSchema, parentCTENames)
	}
	if sel.GetRarg() != nil {
		collectSelectEffects(sel.GetRarg(), c, defaultSchema, parentCTENames)
	}
	if with := sel.GetWithClause(); with != nil {
		// Accumulate CTE names as we iterate so sibling CTEs see each other.
		// For recursive CTEs, the name must be in scope before the body.
		accumulatedCTENames := make(map[string]bool)
		for name := range parentCTENames {
			accumulatedCTENames[name] = true
		}
		for _, cteNode := range with.GetCtes() {
			if cteNode == nil {
				continue
			}
			cte := cteNode.GetCommonTableExpr()
			if cte == nil || cte.GetCtequery() == nil {
				continue
			}
			cteName := strings.ToLower(cte.GetCtename())
			accumulatedCTENames[cteName] = true
			collectEffects(cte.GetCtequery(), c, defaultSchema, accumulatedCTENames)
		}
	}
	scope := buildSelectScope(sel, defaultSchema, parentCTENames)
	for _, distinct := range sel.GetDistinctClause() {
		collectNodeEffects(distinct, c, scope, defaultSchema, scope.cteNames)
	}
	for _, target := range sel.GetTargetList() {
		collectNodeEffects(target, c, scope, defaultSchema, scope.cteNames)
	}
	for _, from := range sel.GetFromClause() {
		collectNodeEffects(from, c, scope, defaultSchema, scope.cteNames)
	}
	if sel.GetWhereClause() != nil {
		collectNodeEffects(sel.GetWhereClause(), c, scope, defaultSchema, scope.cteNames)
	}
	for _, group := range sel.GetGroupClause() {
		collectNodeEffects(group, c, scope, defaultSchema, scope.cteNames)
	}
	if sel.GetHavingClause() != nil {
		collectNodeEffects(sel.GetHavingClause(), c, scope, defaultSchema, scope.cteNames)
	}
	for _, window := range sel.GetWindowClause() {
		collectNodeEffects(window, c, scope, defaultSchema, scope.cteNames)
	}
	for _, values := range sel.GetValuesLists() {
		collectNodeEffects(values, c, scope, defaultSchema, scope.cteNames)
	}
	for _, sort := range sel.GetSortClause() {
		collectNodeEffects(sort, c, scope, defaultSchema, scope.cteNames)
	}
	if sel.GetLimitOffset() != nil {
		collectNodeEffects(sel.GetLimitOffset(), c, scope, defaultSchema, scope.cteNames)
	}
	if sel.GetLimitCount() != nil {
		collectNodeEffects(sel.GetLimitCount(), c, scope, defaultSchema, scope.cteNames)
	}
}

func selectHasUnprovenEffect(sel *pg_query.SelectStmt) bool {
	var c effectCollector
	collectSelectEffects(sel, &c, "", nil)
	return c.hasUnprovenEffect()
}

func collectWindowRefs(win *pg_query.WindowDef, scope *selectScope, defaultSchema string, refs *[]ColumnRefFacts, seen map[string]bool, cteLineage cteLineageMap) {
	if win == nil {
		return
	}
	for _, part := range win.GetPartitionClause() {
		collectRefsFromNode(part, scope, defaultSchema, "window", refs, seen, cteLineage)
	}
	for _, order := range win.GetOrderClause() {
		if sortBy := order.GetSortBy(); sortBy != nil {
			collectRefsFromNode(sortBy.GetNode(), scope, defaultSchema, "ordering", refs, seen, cteLineage)
		}
	}
	if win.GetStartOffset() != nil {
		collectRefsFromNode(win.GetStartOffset(), scope, defaultSchema, "window", refs, seen, cteLineage)
	}
	if win.GetEndOffset() != nil {
		collectRefsFromNode(win.GetEndOffset(), scope, defaultSchema, "window", refs, seen, cteLineage)
	}
}

func collectWindowDefEffects(win *pg_query.WindowDef, c *effectCollector, scope *selectScope, defaultSchema string, cteNames map[string]bool) {
	if win == nil || c == nil {
		return
	}
	for _, part := range win.GetPartitionClause() {
		collectNodeEffects(part, c, scope, defaultSchema, cteNames)
	}
	for _, order := range win.GetOrderClause() {
		collectNodeEffects(order, c, scope, defaultSchema, cteNames)
	}
	if win.GetStartOffset() != nil {
		collectNodeEffects(win.GetStartOffset(), c, scope, defaultSchema, cteNames)
	}
	if win.GetEndOffset() != nil {
		collectNodeEffects(win.GetEndOffset(), c, scope, defaultSchema, cteNames)
	}
}

func collectNodeEffects(node *pg_query.Node, c *effectCollector, scope *selectScope, defaultSchema string, cteNames map[string]bool) {
	if node == nil || c == nil {
		return
	}
	// No early-exit on flags: candidates must be complete for every effect node.
	switch n := node.GetNode().(type) {
	case *pg_query.Node_AExpr:
		recordOperatorCandidate(c, n.AExpr, scope)
		if n.AExpr.GetLexpr() != nil {
			collectNodeEffects(n.AExpr.GetLexpr(), c, scope, defaultSchema, cteNames)
		}
		if n.AExpr.GetRexpr() != nil {
			collectNodeEffects(n.AExpr.GetRexpr(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_TypeCast:
		recordCastCandidate(c, n.TypeCast)
		if n.TypeCast.GetArg() != nil {
			collectNodeEffects(n.TypeCast.GetArg(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_FuncCall:
		recordFunctionCandidate(c, n.FuncCall, scope)
		for _, arg := range n.FuncCall.GetArgs() {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
		for _, order := range n.FuncCall.GetAggOrder() {
			collectNodeEffects(order, c, scope, defaultSchema, cteNames)
		}
		if n.FuncCall.GetAggFilter() != nil {
			collectNodeEffects(n.FuncCall.GetAggFilter(), c, scope, defaultSchema, cteNames)
		}
		if n.FuncCall.GetOver() != nil {
			collectWindowDefEffects(n.FuncCall.GetOver(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_RangeFunction:
		for _, fn := range n.RangeFunction.GetFunctions() {
			collectNodeEffects(fn, c, scope, defaultSchema, cteNames)
		}
		if !c.flags.function {
			recordSyntheticFunctionCandidate(c, nil, 0, nil)
		}
	case *pg_query.Node_RangeTableFunc:
		recordSyntheticFunctionCandidate(c, nil, 0, nil)
		collectNodeEffects(n.RangeTableFunc.GetDocexpr(), c, scope, defaultSchema, cteNames)
		collectNodeEffects(n.RangeTableFunc.GetRowexpr(), c, scope, defaultSchema, cteNames)
		for _, ns := range n.RangeTableFunc.GetNamespaces() {
			collectNodeEffects(ns, c, scope, defaultSchema, cteNames)
		}
		for _, col := range n.RangeTableFunc.GetColumns() {
			collectNodeEffects(col, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_RangeTableSample:
		c.flags.unsupportedTraversal = true
		return
	case *pg_query.Node_ResTarget:
		if n.ResTarget.GetVal() != nil {
			collectNodeEffects(n.ResTarget.GetVal(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_SelectStmt:
		collectSelectEffects(n.SelectStmt, c, defaultSchema, cteNames)
	case *pg_query.Node_SubLink:
		if n.SubLink.GetSubselect() != nil {
			collectNodeEffects(n.SubLink.GetSubselect(), c, scope, defaultSchema, cteNames)
		}
		if n.SubLink.GetTestexpr() != nil {
			collectNodeEffects(n.SubLink.GetTestexpr(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_NullTest:
		if n.NullTest.GetArg() != nil {
			collectNodeEffects(n.NullTest.GetArg(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_BooleanTest:
		if n.BooleanTest.GetArg() != nil {
			collectNodeEffects(n.BooleanTest.GetArg(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_CoalesceExpr:
		args := n.CoalesceExpr.GetArgs()
		allColumns := len(args) >= 2
		var kinds []OperandKindHint
		var colRefs []OperandColumnRef
		for _, arg := range args {
			kind := operandKindHint(arg)
			kinds = append(kinds, kind)
			if kind != OperandKindColumn {
				allColumns = false
			}
			if ref, ok := operandColumnRef(arg, scope); ok {
				colRefs = append(colRefs, ref)
			}
		}
		if allColumns && len(colRefs) == len(args) {
			recordCoalesceSyntheticCandidate(c, len(args), kinds, colRefs)
		}
		for _, arg := range args {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_CaseExpr:
		caseExpr := n.CaseExpr
		if caseExpr.GetArg() != nil {
			collectNodeEffects(caseExpr.GetArg(), c, scope, defaultSchema, cteNames)
		}
		if caseExpr.GetDefresult() != nil {
			collectNodeEffects(caseExpr.GetDefresult(), c, scope, defaultSchema, cteNames)
		}
		for _, arg := range caseExpr.GetArgs() {
			if caseWhen := arg.GetCaseWhen(); caseWhen != nil {
				collectNodeEffects(caseWhen.GetExpr(), c, scope, defaultSchema, cteNames)
				collectNodeEffects(caseWhen.GetResult(), c, scope, defaultSchema, cteNames)
			}
		}
	case *pg_query.Node_ArrayExpr:
		for _, elem := range n.ArrayExpr.GetElements() {
			collectNodeEffects(elem, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_AArrayExpr:
		for _, elem := range n.AArrayExpr.GetElements() {
			collectNodeEffects(elem, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_RowExpr:
		for _, arg := range n.RowExpr.GetArgs() {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_MinMaxExpr:
		for _, arg := range n.MinMaxExpr.GetArgs() {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_AIndirection:
		if n.AIndirection.GetArg() != nil {
			collectNodeEffects(n.AIndirection.GetArg(), c, scope, defaultSchema, cteNames)
		}
		for _, ind := range n.AIndirection.GetIndirection() {
			collectNodeEffects(ind, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_CollateClause:
		if n.CollateClause.GetArg() != nil {
			collectNodeEffects(n.CollateClause.GetArg(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_GroupingFunc:
		args := n.GroupingFunc.GetArgs()
		kinds := make([]OperandKindHint, 0, len(args))
		for _, arg := range args {
			kinds = append(kinds, operandKindHint(arg))
		}
		recordSyntheticFunctionCandidate(c, []string{"grouping"}, len(args), kinds)
		for _, arg := range args {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_XmlExpr:
		args := n.XmlExpr.GetArgs()
		named := n.XmlExpr.GetNamedArgs()
		kinds := make([]OperandKindHint, 0, len(args)+len(named))
		for _, arg := range args {
			kinds = append(kinds, operandKindHint(arg))
		}
		for _, arg := range named {
			kinds = append(kinds, operandKindHint(arg))
		}
		recordSyntheticFunctionCandidate(c, nil, len(args)+len(named), kinds)
		for _, arg := range args {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
		for _, arg := range named {
			collectNodeEffects(arg, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_JoinExpr:
		join := n.JoinExpr
		collectNodeEffects(join.GetLarg(), c, scope, defaultSchema, cteNames)
		collectNodeEffects(join.GetRarg(), c, scope, defaultSchema, cteNames)
		if join.GetQuals() != nil {
			collectNodeEffects(join.GetQuals(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_RangeSubselect:
		if n.RangeSubselect.GetSubquery() != nil {
			collectNodeEffects(n.RangeSubselect.GetSubquery(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_SortBy:
		if n.SortBy.GetNode() != nil {
			collectNodeEffects(n.SortBy.GetNode(), c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_WindowDef:
		collectWindowDefEffects(n.WindowDef, c, scope, defaultSchema, cteNames)
	case *pg_query.Node_List:
		for _, item := range n.List.GetItems() {
			collectNodeEffects(item, c, scope, defaultSchema, cteNames)
		}
	case *pg_query.Node_SqlvalueFunction:
		// SQLValueFunction (current_user, session_user, current_database, etc.)
		// reads session/context state — not a pure function of AST operands.
		// Must be treated as an unproven effect for Query Access admission.
		recordSQLValueFunctionCandidate(c, n.SqlvalueFunction)
	case *pg_query.Node_ColumnRef, *pg_query.Node_AConst, *pg_query.Node_RangeVar,
		*pg_query.Node_ParamRef, *pg_query.Node_AStar,
		*pg_query.Node_String_, *pg_query.Node_Integer, *pg_query.Node_Float,
		*pg_query.Node_Boolean, *pg_query.Node_BitString:
		return
	case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt,
		*pg_query.Node_CreateStmt, *pg_query.Node_AlterTableStmt, *pg_query.Node_DropStmt,
		*pg_query.Node_IndexStmt, *pg_query.Node_TruncateStmt, *pg_query.Node_ViewStmt:
		// Statement-level nodes that cannot appear in expression contexts.
		// They are classified separately by classifyStatement.
		return
	default:
		// Fail-closed: any node type not explicitly handled above may contain
		// expression subnodes with operators/functions/casts. Emitting
		// unsupported_traversal prevents promotion to admissible. Do NOT
		// recurse — we don't know the node's structure.
		c.flags.unsupportedTraversal = true
	}
}

func foldClassifications(classifications []string) string {
	hasIndeterminate := false
	for _, c := range classifications {
		switch c {
		case "not_read_only":
			return "not_read_only"
		case "indeterminate":
			hasIndeterminate = true
		}
	}
	if hasIndeterminate {
		return "indeterminate"
	}
	return "read_only"
}

func classifyStatement(node *pg_query.Node) string {
	if node == nil {
		return "indeterminate"
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		sel := n.SelectStmt
		if len(sel.GetLockingClause()) > 0 || sel.GetIntoClause() != nil {
			return "not_read_only"
		}
		if selectHasDataModifyingCTE(sel) {
			return "not_read_only"
		}
		if selectHasWildcard(sel) {
			return "indeterminate"
		}
		// Single effect traversal: operator / function / cast anywhere in the SELECT
		// (including LIMIT/OFFSET, VALUES, window, aggregate FILTER, DISTINCT ON).
		if selectHasUnprovenEffect(sel) {
			return "indeterminate"
		}
		return "read_only"
	case *pg_query.Node_ExplainStmt:
		for _, opt := range n.ExplainStmt.GetOptions() {
			if defElem := opt.GetDefElem(); defElem != nil && defElem.GetDefname() == "analyze" {
				return "not_read_only"
			}
		}
		inner := n.ExplainStmt.GetQuery()
		if inner == nil {
			return "indeterminate"
		}
		switch s := inner.GetNode().(type) {
		case *pg_query.Node_SelectStmt:
			if len(s.SelectStmt.GetLockingClause()) > 0 || s.SelectStmt.GetIntoClause() != nil {
				return "not_read_only"
			}
			if selectHasDataModifyingCTE(s.SelectStmt) {
				return "not_read_only"
			}
			if selectHasWildcard(s.SelectStmt) {
				return "indeterminate"
			}
			if selectHasUnprovenEffect(s.SelectStmt) {
				return "indeterminate"
			}
			return "read_only"
		default:
			return "indeterminate"
		}
	case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt,
		*pg_query.Node_CreateStmt, *pg_query.Node_AlterTableStmt, *pg_query.Node_DropStmt,
		*pg_query.Node_IndexStmt, *pg_query.Node_TruncateStmt, *pg_query.Node_ViewStmt:
		return "not_read_only"
	default:
		return "indeterminate"
	}
}

func explainHasAnalyze(explain *pg_query.ExplainStmt) bool {
	if explain == nil {
		return false
	}
	for _, opt := range explain.GetOptions() {
		if defElem := opt.GetDefElem(); defElem != nil && defElem.GetDefname() == "analyze" {
			return true
		}
	}
	return false
}

func selectHasDataModifyingCTE(sel *pg_query.SelectStmt) bool {
	with := sel.GetWithClause()
	if with == nil {
		return false
	}
	for _, cteNode := range with.GetCtes() {
		if cteNode == nil {
			continue
		}
		cte := cteNode.GetCommonTableExpr()
		if cte == nil {
			continue
		}
		cteQuery := cte.GetCtequery()
		if cteQuery == nil {
			continue
		}
		switch cteQuery.GetNode().(type) {
		case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt:
			return true
		}
	}
	return false
}

func selectHasWildcard(sel *pg_query.SelectStmt) bool {
	for _, target := range sel.GetTargetList() {
		if nodeContainsWildcard(target) {
			return true
		}
	}
	for _, from := range sel.GetFromClause() {
		if sub := from.GetRangeSubselect(); sub != nil {
			subQuery := sub.GetSubquery()
			if subQuery != nil {
				if subSel := subQuery.GetSelectStmt(); subSel != nil {
					if selectHasWildcard(subSel) {
						return true
					}
				}
			}
		}
	}
	return false
}

func nodeContainsWildcard(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	if colRef := node.GetColumnRef(); colRef != nil {
		fields := colRef.GetFields()
		if len(fields) > 0 {
			last := fields[len(fields)-1]
			if _, isStar := last.GetNode().(*pg_query.Node_AStar); isStar {
				return true
			}
		}
	}
	if resTarget := node.GetResTarget(); resTarget != nil {
		if val := resTarget.GetVal(); val != nil {
			return nodeContainsWildcard(val)
		}
	}
	return false
}

// relationFromRangeVar extracts relation information from a RangeVar.
// For unqualified relations, Schema is left empty so the resolver can use
// the session's search_path instead of defaultSchema. Qualified relations
// keep their explicit schemaname.
func relationFromRangeVar(r *pg_query.RangeVar, _ string) RelationFacts {
	schema := strings.ToLower(r.GetSchemaname())
	return RelationFacts{
		Schema: schema,
		Name:   r.GetRelname(),
		Alias:  r.GetAlias().GetAliasname(),
		Kind:   "table",
	}
}

// resolveColumnRef resolves a ColumnRef to its table and column components.
func resolveColumnRef(ref *pg_query.ColumnRef, scope *selectScope) *ColumnRefResult {
	fields := ref.GetFields()
	if len(fields) == 0 {
		return nil
	}
	last := fields[len(fields)-1]
	if _, isStar := last.GetNode().(*pg_query.Node_AStar); isStar {
		table := ""
		if len(fields) >= 2 {
			table = stringNodeValue(fields[0])
		}
		return &ColumnRefResult{Table: table, Column: "*", IsWildcard: true}
	}
	colName := stringNodeValue(last)
	if colName == "" {
		return nil
	}

	if len(fields) >= 3 {
		schema := stringNodeValue(fields[0])
		table := stringNodeValue(fields[1])
		binding := lookupBinding(scope, schema, table, "")
		if binding != nil {
			return &ColumnRefResult{
				Table: binding.Table, Column: colName, QualRef: schema + "." + table,
				Schema: binding.Schema, Kind: binding.Kind, Resolved: binding.Kind == "base_table",
			}
		}
		// Unbound three-part reference: fail-closed. Without a lexical binding
		// we cannot prove this is a physical base table column.
		return &ColumnRefResult{
			Table: table, Column: colName, QualRef: schema + "." + table,
			Schema: schema, Kind: "unknown", Resolved: false,
		}
	}

	if len(fields) >= 2 {
		qualifier := stringNodeValue(fields[0])
		resolved := qualifier
		if aliasTable, ok := scope.aliasToTable[qualifier]; ok {
			resolved = aliasTable
		}
		binding := lookupBinding(scope, "", resolved, qualifier)
		result := &ColumnRefResult{Table: resolved, Column: colName, QualRef: qualifier}
		if binding != nil {
			result.Schema = binding.Schema
			result.Kind = binding.Kind
			result.Resolved = binding.Kind == "base_table"
		} else if scope != nil && scope.cteNames[strings.ToLower(resolved)] {
			result.Kind = "cte"
			result.Resolved = false
		} else {
			// Unbound qualified reference: fail-closed.
			result.Kind = "unknown"
			result.Resolved = false
		}
		return result
	}

	if len(scope.tables) == 1 {
		result := &ColumnRefResult{Table: scope.tables[0], Column: colName, QualRef: ""}
		if len(scope.bindings) > 0 {
			b := scope.bindings[0]
			result.Schema = b.Schema
			result.Kind = b.Kind
			result.Resolved = b.Kind == "base_table"
		} else if scope.cteNames[strings.ToLower(scope.tables[0])] {
			result.Kind = "cte"
			result.Resolved = false
		}
		return result
	}
	return &ColumnRefResult{Table: "", Column: colName, QualRef: ""}
}

func lookupBinding(scope *selectScope, schema, table, alias string) *scopeBinding {
	if scope == nil {
		return nil
	}
	for i := range scope.bindings {
		b := &scope.bindings[i]
		if alias != "" {
			if strings.EqualFold(b.Alias, alias) {
				return b
			}
			continue
		}
		if table != "" && strings.EqualFold(b.Table, table) {
			if schema == "" || strings.EqualFold(b.Schema, schema) {
				return b
			}
		}
	}
	return nil
}

// ColumnRefResult holds the resolved components of a column reference.
type ColumnRefResult struct {
	Table      string
	Column     string
	IsWildcard bool
	QualRef    string
	Schema     string
	Kind       string // "base_table", "cte", "derived"
	Resolved   bool
}

// scopeBinding records a single relation binding with full provenance.
type scopeBinding struct {
	Schema string
	Table  string
	Alias  string
	Kind   string // "base_table", "cte", "derived"
}

// selectScope tracks the lexical scope for resolving column references.
type selectScope struct {
	aliasToTable map[string]string
	tables       []string
	bindings     []scopeBinding
	cteNames     map[string]bool
}

type cteLineageMap map[string]map[string][]string

func collectRelations(sel *pg_query.SelectStmt, defaultSchema string) []RelationFacts {
	relations := make([]RelationFacts, 0)
	if sel.GetOp() != pg_query.SetOperation_SETOP_NONE {
		if larg := sel.GetLarg(); larg != nil {
			relations = append(relations, collectRelations(larg, defaultSchema)...)
		}
		if rarg := sel.GetRarg(); rarg != nil {
			relations = append(relations, collectRelations(rarg, defaultSchema)...)
		}
		return relations
	}
	cteNames := make(map[string]bool)
	var cteBodyRelations []RelationFacts
	if with := sel.GetWithClause(); with != nil {
		for _, cteNode := range with.GetCtes() {
			if cteNode == nil {
				continue
			}
			cte := cteNode.GetCommonTableExpr()
			if cte == nil {
				continue
			}
			cteBody := cte.GetCtequery()
			if cteBody != nil {
				if subSel := cteBody.GetSelectStmt(); subSel != nil {
					cteBodyRelations = append(cteBodyRelations, collectRelations(subSel, defaultSchema)...)
				}
			}
			cteNames[strings.ToLower(cte.GetCtename())] = true
			relations = append(relations, RelationFacts{
				Name: cte.GetCtename(),
				Kind: "cte",
			})
		}
	}
	walkFromClause(sel.GetFromClause(), defaultSchema, &relations)
	// Walk expression-bearing clauses for scalar subqueries (SubLink).
	for _, target := range sel.GetTargetList() {
		collectSubLinkRelations(target, defaultSchema, &relations)
	}
	if sel.GetWhereClause() != nil {
		collectSubLinkRelations(sel.GetWhereClause(), defaultSchema, &relations)
	}
	for _, group := range sel.GetGroupClause() {
		collectSubLinkRelations(group, defaultSchema, &relations)
	}
	if sel.GetHavingClause() != nil {
		collectSubLinkRelations(sel.GetHavingClause(), defaultSchema, &relations)
	}
	for _, sort := range sel.GetSortClause() {
		collectSubLinkRelations(sort, defaultSchema, &relations)
	}
	if sel.GetLimitOffset() != nil {
		collectSubLinkRelations(sel.GetLimitOffset(), defaultSchema, &relations)
	}
	if sel.GetLimitCount() != nil {
		collectSubLinkRelations(sel.GetLimitCount(), defaultSchema, &relations)
	}
	// Mark FROM-clause references to CTEs as kind "cte" instead of "table"
	for i := range relations {
		if relations[i].Kind == "table" && cteNames[strings.ToLower(relations[i].Name)] {
			relations[i].Kind = "cte"
		}
	}
	relations = append(relations, cteBodyRelations...)
	return relations
}

// collectSubLinkRelations walks an expression node tree looking for SubLink
// (scalar subquery) nodes and recurses into their subselects to discover
// referenced relations that would otherwise be missed.
func collectSubLinkRelations(node *pg_query.Node, defaultSchema string, relations *[]RelationFacts) {
	if node == nil {
		return
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_SubLink:
		if sub := n.SubLink.GetSubselect(); sub != nil {
			if subSel := sub.GetSelectStmt(); subSel != nil {
				*relations = append(*relations, collectRelations(subSel, defaultSchema)...)
			}
		}
		if n.SubLink.GetTestexpr() != nil {
			collectSubLinkRelations(n.SubLink.GetTestexpr(), defaultSchema, relations)
		}
	case *pg_query.Node_ResTarget:
		if n.ResTarget.GetVal() != nil {
			collectSubLinkRelations(n.ResTarget.GetVal(), defaultSchema, relations)
		}
	case *pg_query.Node_AExpr:
		if n.AExpr.GetLexpr() != nil {
			collectSubLinkRelations(n.AExpr.GetLexpr(), defaultSchema, relations)
		}
		if n.AExpr.GetRexpr() != nil {
			collectSubLinkRelations(n.AExpr.GetRexpr(), defaultSchema, relations)
		}
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			collectSubLinkRelations(arg, defaultSchema, relations)
		}
	case *pg_query.Node_FuncCall:
		for _, arg := range n.FuncCall.GetArgs() {
			collectSubLinkRelations(arg, defaultSchema, relations)
		}
	case *pg_query.Node_TypeCast:
		if n.TypeCast.GetArg() != nil {
			collectSubLinkRelations(n.TypeCast.GetArg(), defaultSchema, relations)
		}
	case *pg_query.Node_CaseExpr:
		caseExpr := n.CaseExpr
		if caseExpr.GetArg() != nil {
			collectSubLinkRelations(caseExpr.GetArg(), defaultSchema, relations)
		}
		if caseExpr.GetDefresult() != nil {
			collectSubLinkRelations(caseExpr.GetDefresult(), defaultSchema, relations)
		}
		for _, arg := range caseExpr.GetArgs() {
			if caseWhen := arg.GetCaseWhen(); caseWhen != nil {
				collectSubLinkRelations(caseWhen.GetExpr(), defaultSchema, relations)
				collectSubLinkRelations(caseWhen.GetResult(), defaultSchema, relations)
			}
		}
	case *pg_query.Node_CoalesceExpr:
		for _, arg := range n.CoalesceExpr.GetArgs() {
			collectSubLinkRelations(arg, defaultSchema, relations)
		}
	case *pg_query.Node_MinMaxExpr:
		for _, arg := range n.MinMaxExpr.GetArgs() {
			collectSubLinkRelations(arg, defaultSchema, relations)
		}
	case *pg_query.Node_NullTest:
		if n.NullTest.GetArg() != nil {
			collectSubLinkRelations(n.NullTest.GetArg(), defaultSchema, relations)
		}
	case *pg_query.Node_SortBy:
		if n.SortBy.GetNode() != nil {
			collectSubLinkRelations(n.SortBy.GetNode(), defaultSchema, relations)
		}
	case *pg_query.Node_List:
		for _, item := range n.List.GetItems() {
			collectSubLinkRelations(item, defaultSchema, relations)
		}
	}
}

func walkFromClause(fromClause []*pg_query.Node, defaultSchema string, relations *[]RelationFacts) {
	for _, from := range fromClause {
		if from == nil {
			continue
		}
		switch n := from.GetNode().(type) {
		case *pg_query.Node_RangeVar:
			*relations = append(*relations, relationFromRangeVar(n.RangeVar, defaultSchema))
		case *pg_query.Node_RangeSubselect:
			sub := n.RangeSubselect.GetSubquery()
			if subSel := sub.GetSelectStmt(); subSel != nil {
				*relations = append(*relations, collectRelations(subSel, defaultSchema)...)
			}
		case *pg_query.Node_RangeFunction:
			// Set-returning functions don't add table relations.
		case *pg_query.Node_JoinExpr:
			*relations = append(*relations, collectJoinRelations(n.JoinExpr, defaultSchema)...)
		case *pg_query.Node_RangeTableSample:
			if rel := n.RangeTableSample.GetRelation(); rel != nil {
				*relations = append(*relations, collectNodeRelations(rel, defaultSchema)...)
			}
		}
	}
}

func collectJoinRelations(join *pg_query.JoinExpr, defaultSchema string) []RelationFacts {
	relations := make([]RelationFacts, 0)
	if join.GetLarg() != nil {
		relations = append(relations, collectNodeRelations(join.GetLarg(), defaultSchema)...)
	}
	if join.GetRarg() != nil {
		relations = append(relations, collectNodeRelations(join.GetRarg(), defaultSchema)...)
	}
	return relations
}

func collectNodeRelations(node *pg_query.Node, defaultSchema string) []RelationFacts {
	if node == nil {
		return nil
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_RangeVar:
		return []RelationFacts{relationFromRangeVar(n.RangeVar, defaultSchema)}
	case *pg_query.Node_RangeSubselect:
		relations := make([]RelationFacts, 0)
		sub := n.RangeSubselect.GetSubquery()
		if subSel := sub.GetSelectStmt(); subSel != nil {
			relations = append(relations, collectRelations(subSel, defaultSchema)...)
		}
		return relations
	case *pg_query.Node_JoinExpr:
		return collectJoinRelations(n.JoinExpr, defaultSchema)
	case *pg_query.Node_RangeTableSample:
		if rel := n.RangeTableSample.GetRelation(); rel != nil {
			return collectNodeRelations(rel, defaultSchema)
		}
	}
	return nil
}

func collectColumnReferences(sel *pg_query.SelectStmt, defaultSchema string, cteLineage cteLineageMap, extraReasons *[]string) []ColumnRefFacts {
	if sel.GetOp() != pg_query.SetOperation_SETOP_NONE {
		refs := make([]ColumnRefFacts, 0)
		if larg := sel.GetLarg(); larg != nil {
			refs = append(refs, collectColumnReferences(larg, defaultSchema, cteLineage, extraReasons)...)
		}
		if rarg := sel.GetRarg(); rarg != nil {
			refs = append(refs, collectColumnReferences(rarg, defaultSchema, cteLineage, extraReasons)...)
		}
		return refs
	}
	scope := buildSelectScope(sel, defaultSchema, nil)
	refs := make([]ColumnRefFacts, 0)
	seen := make(map[string]bool)

	for _, distinct := range sel.GetDistinctClause() {
		collectRefsFromNode(distinct, scope, defaultSchema, "distinct_on", &refs, seen, cteLineage)
	}
	for _, target := range sel.GetTargetList() {
		collectRefsFromNode(target, scope, defaultSchema, "projection", &refs, seen, cteLineage)
	}
	if sel.GetWhereClause() != nil {
		collectRefsFromNode(sel.GetWhereClause(), scope, defaultSchema, "filter", &refs, seen, cteLineage)
	}
	for _, from := range sel.GetFromClause() {
		if join := from.GetJoinExpr(); join != nil {
			collectJoinColumnRefs(join, scope, defaultSchema, &refs, seen, cteLineage, extraReasons)
		}
	}
	for _, group := range sel.GetGroupClause() {
		collectRefsFromNode(group, scope, defaultSchema, "grouping", &refs, seen, cteLineage)
	}
	if sel.GetHavingClause() != nil {
		collectRefsFromNode(sel.GetHavingClause(), scope, defaultSchema, "having", &refs, seen, cteLineage)
	}
	for _, sort := range sel.GetSortClause() {
		if sortBy := sort.GetSortBy(); sortBy != nil {
			collectRefsFromNode(sortBy.GetNode(), scope, defaultSchema, "ordering", &refs, seen, cteLineage)
		}
	}
	if sel.GetLimitOffset() != nil {
		collectRefsFromNode(sel.GetLimitOffset(), scope, defaultSchema, "limit", &refs, seen, cteLineage)
	}
	if sel.GetLimitCount() != nil {
		collectRefsFromNode(sel.GetLimitCount(), scope, defaultSchema, "limit", &refs, seen, cteLineage)
	}

	return refs
}

func buildSelectScope(sel *pg_query.SelectStmt, defaultSchema string, parentCTENames map[string]bool) *selectScope {
	scope := &selectScope{
		aliasToTable: make(map[string]string),
		cteNames:     make(map[string]bool),
	}
	// Initialize with parent CTE names (lexical scope inheritance).
	for name := range parentCTENames {
		scope.cteNames[name] = true
	}
	if with := sel.GetWithClause(); with != nil {
		for _, cteNode := range with.GetCtes() {
			if cteNode == nil {
				continue
			}
			cte := cteNode.GetCommonTableExpr()
			if cte != nil {
				scope.cteNames[strings.ToLower(cte.GetCtename())] = true
			}
		}
	}
	for _, from := range sel.GetFromClause() {
		collectScopeFromNode(from, defaultSchema, scope)
	}
	return scope
}

func collectScopeFromNode(node *pg_query.Node, defaultSchema string, scope *selectScope) {
	if node == nil || scope == nil {
		return
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_RangeVar:
		addRangeVarToScope(n.RangeVar, scope)
	case *pg_query.Node_JoinExpr:
		collectScopeFromJoin(n.JoinExpr, defaultSchema, scope)
	case *pg_query.Node_RangeSubselect:
		alias := n.RangeSubselect.GetAlias().GetAliasname()
		scope.bindings = append(scope.bindings, scopeBinding{
			Alias: alias,
			Kind:  "derived",
		})
		if alias != "" {
			scope.aliasToTable[alias] = alias
			scope.tables = append(scope.tables, alias)
		}
	}
}

func collectScopeFromJoin(join *pg_query.JoinExpr, defaultSchema string, scope *selectScope) {
	if join == nil || scope == nil {
		return
	}
	if join.GetLarg() != nil {
		collectScopeFromNode(join.GetLarg(), defaultSchema, scope)
	}
	if join.GetRarg() != nil {
		collectScopeFromNode(join.GetRarg(), defaultSchema, scope)
	}
}

func addRangeVarToScope(rv *pg_query.RangeVar, scope *selectScope) {
	if rv == nil || scope == nil {
		return
	}
	// For unqualified relations, leave Schema empty so the resolver can
	// use the session's search_path instead of defaultSchema. Qualified
	// relations keep their explicit schemaname.
	schema := strings.ToLower(rv.GetSchemaname())
	table := rv.GetRelname()
	alias := rv.GetAlias().GetAliasname()
	scope.tables = append(scope.tables, table)
	kind := "base_table"
	if scope.cteNames[strings.ToLower(table)] {
		kind = "cte"
	}
	if alias != "" {
		scope.aliasToTable[alias] = table
	}
	scope.bindings = append(scope.bindings, scopeBinding{
		Schema: schema,
		Table:  table,
		Alias:  alias,
		Kind:   kind,
	})
}

func buildCTELineage(sel *pg_query.SelectStmt, defaultSchema string) (cteLineageMap, []RelationFacts) {
	lineage := make(cteLineageMap)
	var bodyRels []RelationFacts
	with := sel.GetWithClause()
	if with == nil {
		return lineage, bodyRels
	}
	for _, cteNode := range with.GetCtes() {
		if cteNode == nil {
			continue
		}
		cte := cteNode.GetCommonTableExpr()
		if cte == nil {
			continue
		}
		cteBody := cte.GetCtequery()
		if cteBody == nil {
			continue
		}
		cteEntry := make(map[string][]string)

		// Try SELECT CTE body first
		if subSel := cteBody.GetSelectStmt(); subSel != nil {
			subLineage, subBodyRels := buildCTELineage(subSel, defaultSchema)
			for k, v := range subLineage {
				lineage[k] = v
			}
			bodyRels = append(bodyRels, subBodyRels...)
			cteOutputs := collectOutputs(subSel, defaultSchema, subLineage)
			for _, out := range cteOutputs {
				if out.Name != "" && len(out.Sources) > 0 {
					cteEntry[strings.ToLower(out.Name)] = out.Sources
				}
			}
		} else {
			// Data-modifying CTE: extract target table and lineage from RETURNING clause
			returning := extractReturningFromNode(cteBody, defaultSchema)
			for name, sources := range returning {
				cteEntry[name] = sources
			}
			if targetTable := extractTargetTableFromNode(cteBody, defaultSchema); targetTable != nil {
				bodyRels = append(bodyRels, *targetTable)
			}
		}

		if len(cteEntry) > 0 {
			lineage[strings.ToLower(cte.GetCtename())] = cteEntry
		}
	}
	return lineage, bodyRels
}

// extractReturningFromNode extracts column lineage from RETURNING clauses of DML statements.
func extractReturningFromNode(node *pg_query.Node, defaultSchema string) map[string][]string {
	result := make(map[string][]string)
	var tableName string
	var returningList []*pg_query.Node

	switch n := node.GetNode().(type) {
	case *pg_query.Node_DeleteStmt:
		stmt := n.DeleteStmt
		if rel := stmt.GetRelation(); rel != nil {
			tableName = rel.GetRelname()
		}
		returningList = stmt.GetReturningList()
	case *pg_query.Node_UpdateStmt:
		stmt := n.UpdateStmt
		if rel := stmt.GetRelation(); rel != nil {
			tableName = rel.GetRelname()
		}
		returningList = stmt.GetReturningList()
	case *pg_query.Node_InsertStmt:
		stmt := n.InsertStmt
		if rel := stmt.GetRelation(); rel != nil {
			tableName = rel.GetRelname()
		}
		returningList = stmt.GetReturningList()
	default:
		return result
	}

	if tableName == "" || len(returningList) == 0 {
		return result
	}

	schema := defaultSchema
	for _, retNode := range returningList {
		resTarget := retNode.GetResTarget()
		if resTarget == nil {
			continue
		}
		val := resTarget.GetVal()
		if val == nil {
			continue
		}
		colRef := val.GetColumnRef()
		if colRef == nil {
			continue
		}
		fields := colRef.GetFields()
		if len(fields) == 0 {
			continue
		}
		colName := stringNodeValue(fields[len(fields)-1])
		if colName == "" {
			continue
		}
		alias := resTarget.GetName()
		name := alias
		if name == "" {
			name = colName
		}
		source := formatSourceKey(schema, tableName, colName)
		result[strings.ToLower(name)] = []string{source}
	}
	return result
}

func formatSourceKey(schema, table, column string) string {
	var b strings.Builder
	if schema != "" {
		b.WriteString(schema)
		b.WriteByte('.')
	}
	b.WriteString(table)
	b.WriteByte('.')
	b.WriteString(column)
	return b.String()
}

func extractTargetTableFromNode(node *pg_query.Node, defaultSchema string) *RelationFacts {
	var relName string
	switch n := node.GetNode().(type) {
	case *pg_query.Node_DeleteStmt:
		if r := n.DeleteStmt.GetRelation(); r != nil {
			relName = r.GetRelname()
		}
	case *pg_query.Node_UpdateStmt:
		if r := n.UpdateStmt.GetRelation(); r != nil {
			relName = r.GetRelname()
		}
	case *pg_query.Node_InsertStmt:
		if r := n.InsertStmt.GetRelation(); r != nil {
			relName = r.GetRelname()
		}
	}
	if relName == "" {
		return nil
	}
	return &RelationFacts{
		Schema: defaultSchema,
		Name:   relName,
		Kind:   "table",
	}
}

func collectRefsFromNode(node *pg_query.Node, scope *selectScope, defaultSchema string, usage string, refs *[]ColumnRefFacts, seen map[string]bool, cteLineage cteLineageMap) { //nolint:unparam // defaultSchema used in recursive calls
	if node == nil {
		return
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_ColumnRef:
		resolved := resolveColumnRef(n.ColumnRef, scope)
		if resolved == nil || resolved.IsWildcard {
			return
		}
		resTable := resolved.Table
		if lineage, ok := cteLineage[strings.ToLower(resTable)]; ok {
			if sources, ok := lineage[strings.ToLower(resolved.Column)]; ok && len(sources) > 0 {
				_, physTable := pgParseSourceTable(sources[0])
				key := formatRefKey(physTable, resolved.Column)
				if seen[key] {
					addUsageToRef(refs, key, usage)
					return
				}
				seen[key] = true
				*refs = append(*refs, ColumnRefFacts{
					Schema:  resolved.Schema,
					Table:   physTable,
					Column:  resolved.Column,
					Usages:  []string{usage},
					QualRef: resolved.QualRef,
				})
				return
			}
		}
		key := formatRefKey(resTable, resolved.Column)
		if seen[key] {
			addUsageToRef(refs, key, usage)
			return
		}
		seen[key] = true
		*refs = append(*refs, ColumnRefFacts{
			Schema:  resolved.Schema,
			Table:   resTable,
			Column:  resolved.Column,
			Usages:  []string{usage},
			QualRef: resolved.QualRef,
		})
	case *pg_query.Node_ResTarget:
		if n.ResTarget.GetVal() != nil {
			collectRefsFromNode(n.ResTarget.GetVal(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_AExpr:
		if n.AExpr.GetLexpr() != nil {
			collectRefsFromNode(n.AExpr.GetLexpr(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
		if n.AExpr.GetRexpr() != nil {
			collectRefsFromNode(n.AExpr.GetRexpr(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			collectRefsFromNode(arg, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_NullTest:
		if n.NullTest.GetArg() != nil {
			collectRefsFromNode(n.NullTest.GetArg(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_TypeCast:
		if n.TypeCast.GetArg() != nil {
			collectRefsFromNode(n.TypeCast.GetArg(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_CoalesceExpr:
		for _, arg := range n.CoalesceExpr.GetArgs() {
			collectRefsFromNode(arg, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_CaseExpr:
		caseExpr := n.CaseExpr
		if caseExpr.GetDefresult() != nil {
			collectRefsFromNode(caseExpr.GetDefresult(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
		for _, arg := range caseExpr.GetArgs() {
			if caseWhen := arg.GetCaseWhen(); caseWhen != nil {
				collectRefsFromNode(caseWhen.GetExpr(), scope, defaultSchema, usage, refs, seen, cteLineage)
				collectRefsFromNode(caseWhen.GetResult(), scope, defaultSchema, usage, refs, seen, cteLineage)
			}
		}
	case *pg_query.Node_ArrayExpr:
		for _, elem := range n.ArrayExpr.GetElements() {
			collectRefsFromNode(elem, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_RowExpr:
		for _, arg := range n.RowExpr.GetArgs() {
			collectRefsFromNode(arg, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_FuncCall:
		for _, arg := range n.FuncCall.GetArgs() {
			collectRefsFromNode(arg, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
		if n.FuncCall.GetAggFilter() != nil {
			collectRefsFromNode(n.FuncCall.GetAggFilter(), scope, defaultSchema, "filter", refs, seen, cteLineage)
		}
		for _, order := range n.FuncCall.GetAggOrder() {
			if sortBy := order.GetSortBy(); sortBy != nil {
				collectRefsFromNode(sortBy.GetNode(), scope, defaultSchema, "ordering", refs, seen, cteLineage)
			}
		}
		if n.FuncCall.GetOver() != nil {
			collectWindowRefs(n.FuncCall.GetOver(), scope, defaultSchema, refs, seen, cteLineage)
		}
	case *pg_query.Node_SubLink:
		if sub := n.SubLink.GetSubselect(); sub != nil {
			if subSel := sub.GetSelectStmt(); subSel != nil {
				subScope := buildSelectScope(subSel, defaultSchema, scope.cteNames)
				for _, target := range subSel.GetTargetList() {
					collectRefsFromNode(target, subScope, defaultSchema, usage, refs, seen, cteLineage)
				}
				if subSel.GetWhereClause() != nil {
					collectRefsFromNode(subSel.GetWhereClause(), subScope, defaultSchema, usage, refs, seen, cteLineage)
				}
			}
		}
	case *pg_query.Node_MinMaxExpr:
		for _, arg := range n.MinMaxExpr.GetArgs() {
			collectRefsFromNode(arg, scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_SqlvalueFunction:
		// No column references.
	case *pg_query.Node_AIndirection:
		if n.AIndirection.GetArg() != nil {
			collectRefsFromNode(n.AIndirection.GetArg(), scope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_SelectStmt:
		subScope := buildSelectScope(n.SelectStmt, defaultSchema, scope.cteNames)
		for _, target := range n.SelectStmt.GetTargetList() {
			collectRefsFromNode(target, subScope, defaultSchema, usage, refs, seen, cteLineage)
		}
		if n.SelectStmt.GetWhereClause() != nil {
			collectRefsFromNode(n.SelectStmt.GetWhereClause(), subScope, defaultSchema, usage, refs, seen, cteLineage)
		}
	case *pg_query.Node_RangeVar:
		// Table reference; no column reference.
	case *pg_query.Node_RangeSubselect:
		sub := n.RangeSubselect.GetSubquery()
		if subSel := sub.GetSelectStmt(); subSel != nil {
			subScope := buildSelectScope(subSel, defaultSchema, scope.cteNames)
			for _, target := range subSel.GetTargetList() {
				collectRefsFromNode(target, subScope, defaultSchema, usage, refs, seen, cteLineage)
			}
			if subSel.GetWhereClause() != nil {
				collectRefsFromNode(subSel.GetWhereClause(), subScope, defaultSchema, usage, refs, seen, cteLineage)
			}
		}
	case *pg_query.Node_JoinExpr:
		join := n.JoinExpr
		if join.GetQuals() != nil {
			collectRefsFromNode(join.GetQuals(), scope, defaultSchema, "join", refs, seen, cteLineage)
		}
	}
}

func collectJoinColumnRefs(join *pg_query.JoinExpr, scope *selectScope, defaultSchema string, refs *[]ColumnRefFacts, seen map[string]bool, cteLineage cteLineageMap, extraReasons *[]string) {
	if join.GetQuals() != nil {
		collectRefsFromNode(join.GetQuals(), scope, defaultSchema, "join", refs, seen, cteLineage)
	}
	if len(join.GetUsingClause()) > 0 && extraReasons != nil {
		*extraReasons = append(*extraReasons, "unsupported_traversal")
	}
}

func collectOutputs(sel *pg_query.SelectStmt, defaultSchema string, cteLineage cteLineageMap) []OutputFacts {
	outputs := make([]OutputFacts, 0)
	scope := buildSelectScope(sel, defaultSchema, nil)
	for _, target := range sel.GetTargetList() {
		resTarget := target.GetResTarget()
		if resTarget == nil {
			continue
		}
		val := resTarget.GetVal()
		if val == nil {
			continue
		}
		if colRef := val.GetColumnRef(); colRef != nil {
			fields := colRef.GetFields()
			if len(fields) == 0 {
				continue
			}
			last := fields[len(fields)-1]
			if _, isStar := last.GetNode().(*pg_query.Node_AStar); isStar {
				schema := ""
				table := ""
				if len(fields) >= 3 {
					schema = stringNodeValue(fields[0])
					table = stringNodeValue(fields[1])
				} else if len(fields) >= 2 {
					table = stringNodeValue(fields[0])
				}
				alias := resTarget.GetName()
				name := alias
				if name == "" {
					if table != "" {
						name = table + ".*"
					} else {
						name = "*"
					}
				}
				source := "*"
				if table != "" {
					source = formatSourceKey(schema, table, "*")
				}
				outputs = append(outputs, OutputFacts{Name: name, Sources: []string{source}})
				continue
			}
			colName := stringNodeValue(last)
			if colName == "" {
				continue
			}
			schema := ""
			table := ""
			if len(fields) >= 3 {
				schema = stringNodeValue(fields[0])
				table = stringNodeValue(fields[1])
			} else if len(fields) >= 2 {
				table = stringNodeValue(fields[0])
			}
			if table == "" && len(scope.tables) == 1 {
				table = scope.tables[0]
				if len(scope.bindings) > 0 {
					schema = scope.bindings[0].Schema
				}
			}
			alias := resTarget.GetName()
			name := alias
			if name == "" {
				name = colName
			}
			if table != "" {
				if lineage, ok := cteLineage[strings.ToLower(table)]; ok {
					if sources, ok := lineage[strings.ToLower(colName)]; ok && len(sources) > 0 {
						outputs = append(outputs, OutputFacts{Name: name, Sources: sources})
						continue
					}
				}
			}
			source := colName
			if table != "" {
				source = formatSourceKey(schema, table, colName)
			}
			outputs = append(outputs, OutputFacts{Name: name, Sources: []string{source}})
		}
	}
	return outputs
}

func deduplicateRelations(relations []RelationFacts) []RelationFacts {
	seen := make(map[string]bool)
	result := make([]RelationFacts, 0, len(relations))
	for _, r := range relations {
		key := formatRelKey(r.Schema, r.Name)
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}
	return result
}

func hasUnresolvedWildcard(refs []ColumnRefFacts) bool {
	for _, ref := range refs {
		if ref.Column == "*" {
			return true
		}
	}
	return false
}

func formatRelKey(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

func pgParseSourceTable(source string) (schema, table string) {
	parts := strings.SplitN(source, ".", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1]
	case 2:
		return "", parts[0]
	default:
		return "", ""
	}
}

func formatRefKey(table, column string) string {
	if table == "" {
		return column
	}
	return table + "." + column
}

func addUsageToRef(refs *[]ColumnRefFacts, key string, usage string) {
	for i := range *refs {
		if formatRefKey((*refs)[i].Table, (*refs)[i].Column) == key {
			for _, u := range (*refs)[i].Usages {
				if u == usage {
					return
				}
			}
			(*refs)[i].Usages = append((*refs)[i].Usages, usage)
			return
		}
	}
}
