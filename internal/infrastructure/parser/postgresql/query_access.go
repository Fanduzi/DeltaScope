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
	ReadClassification string
	Relations          []RelationFacts
	ColumnReferences   []ColumnRefFacts
	Outputs            []OutputFacts
	Unresolved         []UnresolvedFacts
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

	classifications := make([]string, 0, len(stmts))
	for _, rawStmt := range stmts {
		if rawStmt == nil || rawStmt.GetStmt() == nil {
			classifications = append(classifications, "indeterminate")
			continue
		}
		node := rawStmt.GetStmt()
		classifications = append(classifications, classifyStatement(node))

		if sel := node.GetSelectStmt(); sel != nil {
			cteLineage := buildCTELineage(sel, defaultSchema)
			relations = append(relations, collectRelations(sel, defaultSchema)...)
			columnRefs = append(columnRefs, collectColumnReferences(sel, defaultSchema, cteLineage)...)
			outputs = append(outputs, collectOutputs(sel, defaultSchema, cteLineage)...)
		}
	}

	facts.ReadClassification = foldClassifications(classifications)
	facts.Relations = deduplicateRelations(relations)
	facts.ColumnReferences = columnRefs
	facts.Outputs = outputs

	if hasUnresolvedWildcard(columnRefs) && defaultSchema == "" {
		facts.Unresolved = append(facts.Unresolved, UnresolvedFacts{
			Reference: "*",
			Reason:    "schema_unavailable",
		})
	}

	return facts, nil
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
		if pgSelectHasOperatorExpr(sel) {
			return "indeterminate"
		}
		if pgSelectHasFunctionCall(sel) {
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
			if pgSelectHasOperatorExpr(s.SelectStmt) {
				return "indeterminate"
			}
			if pgSelectHasFunctionCall(s.SelectStmt) {
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

func pgSelectHasOperatorExpr(sel *pg_query.SelectStmt) bool {
	if sel == nil {
		return false
	}
	for _, target := range sel.GetTargetList() {
		if pgContainsOperatorExpr(target) {
			return true
		}
	}
	if sel.GetWhereClause() != nil && pgContainsOperatorExpr(sel.GetWhereClause()) {
		return true
	}
	for _, from := range sel.GetFromClause() {
		if pgContainsOperatorExpr(from) {
			return true
		}
	}
	return false
}

func pgSelectHasFunctionCall(sel *pg_query.SelectStmt) bool {
	if sel == nil {
		return false
	}
	for _, target := range sel.GetTargetList() {
		if pgContainsFunctionCallInExpr(target) {
			return true
		}
	}
	if sel.GetWhereClause() != nil && pgContainsFunctionCallInExpr(sel.GetWhereClause()) {
		return true
	}
	for _, from := range sel.GetFromClause() {
		if pgContainsFunctionCallInExpr(from) {
			return true
		}
	}
	return false
}

func pgContainsOperatorExpr(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_AExpr:
		return true
	case *pg_query.Node_FuncCall:
		return false
	case *pg_query.Node_TypeCast:
		return n.TypeCast.GetArg() != nil && pgContainsOperatorExpr(n.TypeCast.GetArg())
	case *pg_query.Node_SubLink:
		return false
	case *pg_query.Node_ColumnRef:
		return false
	case *pg_query.Node_AConst:
		return false
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			if pgContainsOperatorExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_NullTest:
		return n.NullTest.GetArg() != nil && pgContainsOperatorExpr(n.NullTest.GetArg())
	case *pg_query.Node_CoalesceExpr:
		for _, arg := range n.CoalesceExpr.GetArgs() {
			if pgContainsOperatorExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_CaseExpr:
		caseExpr := n.CaseExpr
		if caseExpr.GetDefresult() != nil && pgContainsOperatorExpr(caseExpr.GetDefresult()) {
			return true
		}
		for _, arg := range caseExpr.GetArgs() {
			if caseWhen := arg.GetCaseWhen(); caseWhen != nil {
				if pgContainsOperatorExpr(caseWhen.GetExpr()) || pgContainsOperatorExpr(caseWhen.GetResult()) {
					return true
				}
			}
		}
	case *pg_query.Node_ArrayExpr:
		for _, elem := range n.ArrayExpr.GetElements() {
			if pgContainsOperatorExpr(elem) {
				return true
			}
		}
	case *pg_query.Node_RowExpr:
		for _, arg := range n.RowExpr.GetArgs() {
			if pgContainsOperatorExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_MinMaxExpr:
		for _, arg := range n.MinMaxExpr.GetArgs() {
			if pgContainsOperatorExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_SqlvalueFunction:
		return false
	case *pg_query.Node_AIndirection:
		return n.AIndirection.GetArg() != nil && pgContainsOperatorExpr(n.AIndirection.GetArg())
	case *pg_query.Node_ResTarget:
		return n.ResTarget.GetVal() != nil && pgContainsOperatorExpr(n.ResTarget.GetVal())
	case *pg_query.Node_SelectStmt:
		return pgSelectHasOperatorExpr(n.SelectStmt)
	case *pg_query.Node_RangeVar:
		return false
	case *pg_query.Node_RangeFunction:
		return false
	case *pg_query.Node_RangeSubselect:
		sub := n.RangeSubselect.GetSubquery()
		if sub != nil {
			return pgContainsOperatorExpr(sub)
		}
		return false
	case *pg_query.Node_JoinExpr:
		join := n.JoinExpr
		if pgContainsOperatorExpr(join.GetLarg()) {
			return true
		}
		if pgContainsOperatorExpr(join.GetRarg()) {
			return true
		}
		if join.GetQuals() != nil && pgContainsOperatorExpr(join.GetQuals()) {
			return true
		}
		for _, u := range join.GetUsingClause() {
			if pgContainsOperatorExpr(u) {
				return true
			}
		}
	}
	return false
}

func pgContainsFunctionCallInExpr(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_FuncCall:
		return true
	case *pg_query.Node_SelectStmt:
		return pgSelectHasFunctionCall(n.SelectStmt)
	case *pg_query.Node_ResTarget:
		return n.ResTarget.GetVal() != nil && pgContainsFunctionCallInExpr(n.ResTarget.GetVal())
	case *pg_query.Node_AExpr:
		if n.AExpr.GetLexpr() != nil && pgContainsFunctionCallInExpr(n.AExpr.GetLexpr()) {
			return true
		}
		return n.AExpr.GetRexpr() != nil && pgContainsFunctionCallInExpr(n.AExpr.GetRexpr())
	case *pg_query.Node_RangeFunction:
		return true
	case *pg_query.Node_SubLink:
		return n.SubLink.GetSubselect() != nil && pgContainsFunctionCallInExpr(n.SubLink.GetSubselect())
	case *pg_query.Node_ColumnRef:
		return false
	case *pg_query.Node_AConst:
		return false
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			if pgContainsFunctionCallInExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_NullTest:
		return n.NullTest.GetArg() != nil && pgContainsFunctionCallInExpr(n.NullTest.GetArg())
	case *pg_query.Node_TypeCast:
		return n.TypeCast.GetArg() != nil && pgContainsFunctionCallInExpr(n.TypeCast.GetArg())
	case *pg_query.Node_CoalesceExpr:
		for _, arg := range n.CoalesceExpr.GetArgs() {
			if pgContainsFunctionCallInExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_CaseExpr:
		caseExpr := n.CaseExpr
		if caseExpr.GetDefresult() != nil && pgContainsFunctionCallInExpr(caseExpr.GetDefresult()) {
			return true
		}
		for _, arg := range caseExpr.GetArgs() {
			if caseWhen := arg.GetCaseWhen(); caseWhen != nil {
				if pgContainsFunctionCallInExpr(caseWhen.GetExpr()) || pgContainsFunctionCallInExpr(caseWhen.GetResult()) {
					return true
				}
			}
		}
	case *pg_query.Node_ArrayExpr:
		for _, elem := range n.ArrayExpr.GetElements() {
			if pgContainsFunctionCallInExpr(elem) {
				return true
			}
		}
	case *pg_query.Node_RowExpr:
		for _, arg := range n.RowExpr.GetArgs() {
			if pgContainsFunctionCallInExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_MinMaxExpr:
		for _, arg := range n.MinMaxExpr.GetArgs() {
			if pgContainsFunctionCallInExpr(arg) {
				return true
			}
		}
	case *pg_query.Node_SqlvalueFunction:
		return false
	case *pg_query.Node_AIndirection:
		return n.AIndirection.GetArg() != nil && pgContainsFunctionCallInExpr(n.AIndirection.GetArg())
	case *pg_query.Node_JoinExpr:
		join := n.JoinExpr
		if pgContainsFunctionCallInExpr(join.GetLarg()) {
			return true
		}
		if pgContainsFunctionCallInExpr(join.GetRarg()) {
			return true
		}
		if join.GetQuals() != nil && pgContainsFunctionCallInExpr(join.GetQuals()) {
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
func relationFromRangeVar(r *pg_query.RangeVar, defaultSchema string) RelationFacts {
	schema := strings.ToLower(r.GetSchemaname())
	if schema == "" {
		schema = defaultSchema
	}
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
	if len(fields) >= 2 {
		qualifier := stringNodeValue(fields[0])
		resolved := qualifier
		if aliasTable, ok := scope.aliasToTable[qualifier]; ok {
			resolved = aliasTable
		}
		return &ColumnRefResult{Table: resolved, Column: colName, QualRef: qualifier}
	}
	if len(scope.tables) == 1 {
		return &ColumnRefResult{Table: scope.tables[0], Column: colName, QualRef: ""}
	}
	return &ColumnRefResult{Table: "", Column: colName, QualRef: ""}
}

// ColumnRefResult holds the resolved components of a column reference.
type ColumnRefResult struct {
	Table      string
	Column     string
	IsWildcard bool
	QualRef    string
}

// selectScope tracks the lexical scope for resolving column references.
type selectScope struct {
	aliasToTable map[string]string
	tables       []string
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
					relations = append(relations, collectRelations(subSel, defaultSchema)...)
				}
			}
			relations = append(relations, RelationFacts{
				Name: cte.GetCtename(),
				Kind: "cte",
			})
		}
	}
	walkFromClause(sel.GetFromClause(), defaultSchema, &relations)
	return relations
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
	}
	return nil
}

func collectColumnReferences(sel *pg_query.SelectStmt, defaultSchema string, cteLineage cteLineageMap) []ColumnRefFacts {
	scope := buildSelectScope(sel)
	refs := make([]ColumnRefFacts, 0)
	seen := make(map[string]bool)

	for _, target := range sel.GetTargetList() {
		collectRefsFromNode(target, scope, defaultSchema, "projection", &refs, seen, cteLineage)
	}
	if sel.GetWhereClause() != nil {
		collectRefsFromNode(sel.GetWhereClause(), scope, defaultSchema, "filter", &refs, seen, cteLineage)
	}
	for _, from := range sel.GetFromClause() {
		if join := from.GetJoinExpr(); join != nil {
			collectJoinColumnRefs(join, scope, defaultSchema, &refs, seen, cteLineage)
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

	return refs
}

func buildSelectScope(sel *pg_query.SelectStmt) *selectScope {
	scope := &selectScope{
		aliasToTable: make(map[string]string),
	}
	for _, from := range sel.GetFromClause() {
		if rv := from.GetRangeVar(); rv != nil {
			scope.tables = append(scope.tables, rv.GetRelname())
			alias := rv.GetAlias().GetAliasname()
			if alias != "" {
				scope.aliasToTable[alias] = rv.GetRelname()
			}
		}
	}
	return scope
}

func buildCTELineage(sel *pg_query.SelectStmt, defaultSchema string) cteLineageMap {
	lineage := make(cteLineageMap)
	with := sel.GetWithClause()
	if with == nil {
		return lineage
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
		subSel := cteBody.GetSelectStmt()
		if subSel == nil {
			continue
		}
		subLineage := buildCTELineage(subSel, defaultSchema)
		for k, v := range subLineage {
			lineage[k] = v
		}
		cteOutputs := collectOutputs(subSel, defaultSchema, subLineage)
		cteEntry := make(map[string][]string)
		for _, out := range cteOutputs {
			if out.Name != "" && len(out.Sources) > 0 {
				cteEntry[strings.ToLower(out.Name)] = out.Sources
			}
		}
		lineage[strings.ToLower(cte.GetCtename())] = cteEntry
	}
	return lineage
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
	case *pg_query.Node_SubLink:
		if sub := n.SubLink.GetSubselect(); sub != nil {
			if subSel := sub.GetSelectStmt(); subSel != nil {
				subScope := buildSelectScope(subSel)
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
		subScope := buildSelectScope(n.SelectStmt)
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
			subScope := buildSelectScope(subSel)
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

func collectJoinColumnRefs(join *pg_query.JoinExpr, scope *selectScope, defaultSchema string, refs *[]ColumnRefFacts, seen map[string]bool, cteLineage cteLineageMap) {
	if join.GetQuals() != nil {
		collectRefsFromNode(join.GetQuals(), scope, defaultSchema, "join", refs, seen, cteLineage)
	}
	for _, u := range join.GetUsingClause() {
		collectRefsFromNode(u, scope, defaultSchema, "join", refs, seen, cteLineage)
	}
}

func collectOutputs(sel *pg_query.SelectStmt, _ string, cteLineage cteLineageMap) []OutputFacts {
	outputs := make([]OutputFacts, 0)
	scope := buildSelectScope(sel)
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
				table := ""
				if len(fields) >= 2 {
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
					source = table + ".*"
				}
				outputs = append(outputs, OutputFacts{Name: name, Sources: []string{source}})
				continue
			}
			colName := stringNodeValue(last)
			if colName == "" {
				continue
			}
			table := ""
			if len(fields) >= 2 {
				table = stringNodeValue(fields[0])
			}
			if table == "" && len(scope.tables) == 1 {
				table = scope.tables[0]
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
				source = table + "." + colName
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
