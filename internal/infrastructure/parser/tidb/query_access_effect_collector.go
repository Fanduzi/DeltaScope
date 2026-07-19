package tidbparser

import (
	"strings"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

func (c *effectCandidateCollector) collectStatement(stmt ast.StmtNode) {
	switch node := stmt.(type) {
	case *ast.SelectStmt:
		c.collectQuery(node, newScopeStack(c.defaultSchema))
	case *ast.SetOprStmt:
		c.collectQuery(node, newScopeStack(c.defaultSchema))
	case *ast.ExplainStmt:
		if node.Stmt != nil {
			c.collectStatement(node.Stmt)
		}
	}
}

func (c *effectCandidateCollector) collectQuery(node ast.Node, scope *scopeStack) {
	switch query := node.(type) {
	case *ast.SelectStmt:
		c.collectSelect(query, scope)
	case *ast.SetOprStmt:
		c.collectSetOperation(query, scope)
	case *ast.SubqueryExpr:
		if query.Query != nil {
			c.collectQuery(query.Query, cloneScope(scope))
		}
	default:
		c.appendUnsupported()
	}
}

func (c *effectCandidateCollector) collectSelect(sel *ast.SelectStmt, scope *scopeStack) {
	if sel == nil {
		return
	}
	c.collectWith(sel.With, scope)
	if sel.From != nil {
		c.collectJoin(sel.From.TableRefs, scope)
	}
	if sel.Fields != nil {
		for _, field := range sel.Fields.Fields {
			if field != nil && field.Expr != nil {
				c.collectExpression(field.Expr, scope)
			}
		}
	}
	c.collectExpression(sel.Where, scope)
	if sel.GroupBy != nil {
		for _, item := range sel.GroupBy.Items {
			if item != nil {
				c.collectExpression(item.Expr, scope)
			}
		}
	}
	if sel.Having != nil {
		c.collectExpression(sel.Having.Expr, scope)
	}
	for i := range sel.WindowSpecs {
		c.collectNode(&sel.WindowSpecs[i], scope)
	}
	if sel.OrderBy != nil {
		c.collectNode(sel.OrderBy, scope)
	}
	if sel.Limit != nil {
		c.collectNode(sel.Limit, scope)
	}
	for _, row := range sel.Lists {
		c.collectNode(row, scope)
	}
}

func (c *effectCandidateCollector) collectSetOperation(set *ast.SetOprStmt, scope *scopeStack) {
	if set == nil {
		return
	}
	c.collectWith(set.With, scope)
	if set.SelectList != nil {
		c.collectSetOperationList(set.SelectList, scope)
	}
	if set.OrderBy != nil {
		c.collectNode(set.OrderBy, scope)
	}
	if set.Limit != nil {
		c.collectNode(set.Limit, scope)
	}
}

func (c *effectCandidateCollector) collectSetOperationList(list *ast.SetOprSelectList, scope *scopeStack) {
	if list == nil {
		return
	}
	c.collectWith(list.With, scope)
	for _, child := range list.Selects {
		c.collectQuery(child, cloneScope(scope))
	}
	if list.OrderBy != nil {
		c.collectNode(list.OrderBy, scope)
	}
	if list.Limit != nil {
		c.collectNode(list.Limit, scope)
	}
}

func (c *effectCandidateCollector) collectWith(with *ast.WithClause, scope *scopeStack) {
	if with == nil {
		return
	}
	for _, cte := range with.CTEs {
		if cte != nil {
			scope.addCTE(cte.Name.L)
		}
	}
	for _, cte := range with.CTEs {
		if cte == nil || cte.Query == nil {
			continue
		}
		cteScope := newScopeStack(scope.defaultSchema)
		c.collectQuery(cte.Query, cteScope)
		cteFacts := analyzeQueryNode(cte.Query, cteScope)
		lineage := make(lineageEntry)
		for _, output := range cteFacts.outputs {
			if output.Name != "" && len(output.Sources) > 0 {
				lineage[strings.ToLower(output.Name)] = output.Sources
			}
		}
		scope.addLineage(cte.Name.L, lineage)
	}
}

func (c *effectCandidateCollector) collectJoin(join *ast.Join, scope *scopeStack) {
	if join == nil {
		return
	}
	c.collectResultSet(join.Left, scope)
	c.collectResultSet(join.Right, scope)
	if join.On != nil {
		c.collectExpression(join.On.Expr, scope)
	}
}

func (c *effectCandidateCollector) collectResultSet(node ast.ResultSetNode, scope *scopeStack) {
	switch source := node.(type) {
	case *ast.Join:
		c.collectJoin(source, scope)
	case *ast.TableName:
		if source.Schema.L == "" {
			c.hasUnqualifiedRelation = true
		}
		schema := source.Schema.L
		if schema == "" {
			schema = scope.defaultSchema
		}
		scope.addRelation(schema, source.Name.L)
	case *ast.TableSource:
		alias := source.AsName.L
		switch inner := source.Source.(type) {
		case *ast.TableName:
			c.collectResultSet(inner, scope)
			if alias != "" {
				scope.addAlias(alias, inner.Schema.L, inner.Name.L)
			}
		case *ast.SubqueryExpr:
			c.addDerivedSource(inner, alias, scope)
		case *ast.SelectStmt:
			c.addDerivedSource(inner, alias, scope)
		case *ast.SetOprStmt:
			c.addDerivedSource(inner, alias, scope)
		default:
			c.appendUnsupported()
		}
	case *ast.SubqueryExpr:
		c.collectQuery(source, cloneScope(scope))
	case *ast.SelectStmt:
		c.collectQuery(source, cloneScope(scope))
	case *ast.SetOprStmt:
		c.collectQuery(source, cloneScope(scope))
	case nil:
	default:
		c.appendUnsupported()
	}
}

func (c *effectCandidateCollector) addDerivedSource(query ast.Node, alias string, scope *scopeStack) {
	if alias == "" {
		alias = "derived"
	}
	scope.addAlias(alias, "", alias)
	subScope := cloneScope(scope)
	c.collectQuery(query, subScope)
	subFacts := analyzeQueryNode(query, subScope)
	lineage := make(lineageEntry)
	for _, output := range subFacts.outputs {
		if output.Name != "" && len(output.Sources) > 0 {
			lineage[strings.ToLower(output.Name)] = output.Sources
		}
	}
	scope.addLineage(alias, lineage)
}

func (c *effectCandidateCollector) collectExpression(expr ast.ExprNode, scope *scopeStack) {
	if expr != nil {
		c.collectNode(expr, scope)
	}
}

func (c *effectCandidateCollector) collectNode(node ast.Node, scope *scopeStack) {
	if node == nil {
		return
	}
	_, ok := node.Accept(&effectExpressionVisitor{collector: c, scope: scope})
	if !ok {
		c.appendUnsupported()
	}
}

type effectExpressionVisitor struct {
	collector *effectCandidateCollector
	scope     *scopeStack
}

func (v *effectExpressionVisitor) Enter(node ast.Node) (ast.Node, bool) {
	switch current := node.(type) {
	case *ast.SubqueryExpr:
		v.collector.collectQuery(current, cloneScope(v.scope))
		return node, true
	case *ast.FuncCallExpr:
		v.collector.appendFunctionCall(current, v.scope)
	case *ast.AggregateFuncExpr:
		v.collector.appendAggregate(current, v.scope)
	case *ast.WindowFuncExpr:
		v.collector.appendWindow(current, v.scope)
	case *ast.FuncCastExpr:
		v.collector.appendCast(current, v.scope)
	case *ast.JSONSumCrc32Expr:
		v.collector.appendSpecialFunction("json_sum_crc32", current.Expr, v.scope, current.OriginalText())
	case *ast.MatchAgainst:
		v.collector.appendUnsupported()
	}
	return node, false
}

func (*effectExpressionVisitor) Leave(node ast.Node) (ast.Node, bool) {
	return node, true
}
