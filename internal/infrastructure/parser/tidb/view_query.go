package tidbparser

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

type viewQueryDetector struct {
	complex bool
}

func (d *viewQueryDetector) Enter(node ast.Node) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.Join:
		d.complex = d.complex || n.Right != nil
	case *ast.WithClause, *ast.SetOprStmt, *ast.SubqueryExpr, *ast.WindowFuncExpr:
		d.complex = true
	case *ast.TableSource:
		if _, physical := n.Source.(*ast.TableName); !physical {
			d.complex = true
		}
	case *ast.SelectStmt:
		d.complex = d.complex || n.GroupBy != nil || n.Having != nil || n.OrderBy != nil ||
			n.Limit != nil || n.Distinct || len(n.WindowSpecs) > 0
	}
	return node, false
}

func (d *viewQueryDetector) Leave(node ast.Node) (ast.Node, bool) { return node, true }

func needsViewQuery(node ast.Node) bool {
	detector := &viewQueryDetector{}
	node.Accept(detector)
	return detector.complex
}

type viewQueryBuilder struct {
	sql         string
	selectStart int
	offsetBase  int
	locations   bool
	hasCTE      bool
	nodes       int
	usedOffsets map[int]bool
}

func copyViewScope(scope map[string]*spec.ViewQuery) map[string]*spec.ViewQuery {
	copied := make(map[string]*spec.ViewQuery, len(scope))
	for name, query := range scope {
		copied[name] = query
	}
	return copied
}

func (b *viewQueryBuilder) build(node ast.Node, inherited map[string]*spec.ViewQuery, depth int) *spec.ViewQuery {
	query := &spec.ViewQuery{}
	b.nodes++
	if depth > 32 || b.nodes > 1024 {
		query.Limited = true
		return query
	}
	scope := copyViewScope(inherited)
	var with *ast.WithClause
	switch n := node.(type) {
	case *ast.SelectStmt:
		with = n.With
	case *ast.SetOprStmt:
		with = n.With
	case *ast.SetOprSelectList:
		with = n.With
	default:
		query.Unsupported = true
		return query
	}
	if with != nil {
		b.hasCTE = true
		if with.IsRecursive {
			query.Unsupported = true
		}
		for _, cte := range with.CTEs {
			if cte == nil || cte.Query == nil || cte.Query.Query == nil {
				query.Unsupported = true
				continue
			}
			body := b.build(cte.Query.Query, scope, depth+1)
			for _, name := range cte.ColNameList {
				body.ColumnAliases = append(body.ColumnAliases, name.L)
			}
			query.With = append(query.With, body)
			scope[cte.Name.L] = body
		}
	}
	switch n := node.(type) {
	case *ast.SelectStmt:
		b.selectQuery(query, n, scope, depth)
	case *ast.SetOprStmt:
		query.NonMergeable, query.NonUpdatable = true, true
		if n.SelectList == nil {
			query.Unsupported = true
			return query
		}
		if n.Limit != nil || n.OrderBy != nil || n.SelectList.Limit != nil || n.SelectList.OrderBy != nil {
			query.Unsupported = true
		}
		for _, operand := range n.SelectList.Selects {
			query.SetOperands = append(query.SetOperands, b.build(operand, scope, depth+1))
		}
		if n.SelectList.With != nil {
			query.Unsupported = true
		}
	case *ast.SetOprSelectList:
		query.NonMergeable, query.NonUpdatable = true, true
		if n.Limit != nil || n.OrderBy != nil {
			query.Unsupported = true
		}
		for _, operand := range n.Selects {
			query.SetOperands = append(query.SetOperands, b.build(operand, scope, depth+1))
		}
	}
	return query
}

func (b *viewQueryBuilder) selectQuery(query *spec.ViewQuery, node *ast.SelectStmt, scope map[string]*spec.ViewQuery, depth int) {
	if node.Fields == nil || node.LockInfo != nil || node.SelectIntoOpt != nil || len(node.WindowSpecs) != 0 {
		query.Unsupported = true
		return
	}
	if node.Limit != nil {
		query.Unsupported = true
	}
	if node.From != nil {
		b.addSource(query, node.From.TableRefs, scope, depth)
	} else {
		query.NonMergeable, query.NonUpdatable = true, true
	}
	if node.Distinct || node.GroupBy != nil || node.Having != nil {
		query.NonMergeable, query.NonUpdatable = true, true
	}
	for _, field := range node.Fields.Fields {
		offset := -1
		if b.locations && field.Offset >= 0 && b.offsetBase+field.Offset < len(b.sql) {
			offset = b.offsetBase + field.Offset
		}
		if field.WildCard != nil {
			query.Bindings = append(query.Bindings, spec.ViewColumnBinding{
				Schema: field.WildCard.Schema.L, Table: field.WildCard.Table.L, Star: true, Offset: offset,
			})
			continue
		}
		if column, ok := field.Expr.(*ast.ColumnNameExpr); ok && column.Name != nil {
			output := field.AsName.L
			if output == "" {
				output = column.Name.Name.L
			}
			query.Bindings = append(query.Bindings, spec.ViewColumnBinding{
				Schema: column.Name.Schema.L, Table: column.Name.Table.L,
				Column: column.Name.Name.L, Output: output, Offset: offset,
			})
			continue
		}
		if field.Expr == nil {
			query.Unsupported = true
			continue
		}
		b.addExpression(query, field.Expr, scope, depth, offset, false, "")
		if field.AsName.L == "" {
			if _, literal := field.Expr.(ast.ValueExpr); !literal {
				query.Unlabeled = true
			}
		}
		query.Bindings = append(query.Bindings, spec.ViewColumnBinding{Output: field.AsName.L, Expression: true, Offset: offset})
	}
	if len(query.Sources) > 0 && !viewDefinitionHasUpdatableColumn(query.Bindings) {
		query.NonUpdatable = true
	}
	if node.Where != nil {
		b.addExpression(query, node.Where, scope, depth, -1, false, "WHERE")
	}
	if node.GroupBy != nil {
		for _, item := range node.GroupBy.Items {
			if item != nil && item.Expr != nil {
				b.addExpression(query, item.Expr, scope, depth, -1, true, "GROUP")
			}
		}
	}
	if node.Having != nil && node.Having.Expr != nil {
		b.addExpression(query, node.Having.Expr, scope, depth, -1, true, "HAVING")
	}
	if node.OrderBy != nil {
		for _, item := range node.OrderBy.Items {
			if item != nil && item.Expr != nil {
				b.addExpression(query, item.Expr, scope, depth, -1, true, "ORDER")
			}
		}
	}
}

func (b *viewQueryBuilder) addSource(query *spec.ViewQuery, source ast.ResultSetNode, scope map[string]*spec.ViewQuery, depth int) {
	switch n := source.(type) {
	case *ast.Join:
		if n.Right != nil && (n.Tp == ast.LeftJoin || n.Tp == ast.RightJoin) {
			query.NonUpdatable = true
		}
		if n.Left != nil {
			b.addSource(query, n.Left, scope, depth)
		}
		if n.Right != nil {
			b.addSource(query, n.Right, scope, depth)
		}
		if n.On != nil && n.On.Expr != nil {
			b.addExpression(query, n.On.Expr, scope, depth, -1, false, "ON")
		}
		if n.NaturalJoin || len(n.Using) > 0 {
			query.Unsupported = true
		}
	case *ast.TableSource:
		if n.Lateral {
			query.Unsupported = true
			return
		}
		alias := n.AsName.L
		if table, ok := n.Source.(*ast.TableName); ok {
			name := spec.Table{Schema: table.Schema.L, Name: table.Name.L}
			if name.Schema == "" && scope[name.Name] != nil {
				if alias == "" {
					alias = name.Name
				}
				if len(n.ColumnNames) != 0 {
					query.Unsupported = true
				}
				query.Sources = append(query.Sources, spec.ViewQuerySource{Alias: alias, Query: scope[name.Name], Index: -1})
				return
			}
			query.Sources = append(query.Sources, spec.ViewQuerySource{
				Table: &name, Alias: alias, Offset: b.sourceOffset(name), Index: -1,
			})
			return
		}
		if n.Source == nil {
			query.Unsupported = true
			return
		}
		child := b.build(n.Source, scope, depth+1)
		for _, name := range n.ColumnNames {
			child.ColumnAliases = append(child.ColumnAliases, name.L)
		}
		query.Sources = append(query.Sources, spec.ViewQuerySource{Alias: alias, Query: child, Index: -1})
	default:
		query.Unsupported = true
	}
}

type viewQueryExpression struct {
	builder         *viewQueryBuilder
	query           *spec.ViewQuery
	scope           map[string]*spec.ViewQuery
	depth           int
	offset          int
	outputReference bool
	clause          string
}

func (v *viewQueryExpression) Enter(node ast.Node) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.AggregateFuncExpr:
		v.query.NonMergeable, v.query.NonUpdatable = true, true
	case *ast.WindowFuncExpr, *ast.PositionExpr:
		v.query.Unsupported = true
		return node, true
	case *ast.SubqueryExpr:
		if n.Query == nil {
			v.query.Unsupported = true
		} else {
			v.query.Subqueries = append(v.query.Subqueries, v.builder.build(n.Query, v.scope, v.depth+1))
		}
		return node, true
	case *ast.ColumnNameExpr:
		if n.Name != nil {
			offset := v.offset
			if offset < 0 && v.builder.locations {
				offset = viewReferenceOffset(v.builder.sql, v.builder.selectStart, n.Name, v.clause)
			}
			v.query.References = append(v.query.References, spec.ViewColumnBinding{
				Schema: n.Name.Schema.L, Table: n.Name.Table.L, Column: n.Name.Name.L, Offset: offset, OutputReference: v.outputReference,
			})
		}
		return node, true
	}
	return node, false
}

func (v *viewQueryExpression) Leave(node ast.Node) (ast.Node, bool) { return node, true }

func (b *viewQueryBuilder) addExpression(query *spec.ViewQuery, expression ast.ExprNode, scope map[string]*spec.ViewQuery, depth, offset int, outputReference bool, clause string) {
	expression.Accept(&viewQueryExpression{builder: b, query: query, scope: scope, depth: depth, offset: offset, outputReference: outputReference, clause: clause})
}

func (b *viewQueryBuilder) sourceOffset(table spec.Table) int {
	if !b.locations {
		return -1
	}
	tokens := tokenizeSQL(b.sql)
	for i, token := range tokens {
		if token.start < b.selectStart || i == 0 || b.usedOffsets[token.start] ||
			!tokens[i-1].wordIs(b.sql, "FROM") && !tokens[i-1].wordIs(b.sql, "JOIN") && tokens[i-1].text(b.sql) != "," {
			continue
		}
		if token.kind != tokWord && token.kind != tokQIdent {
			continue
		}
		name, schema := token.identText(b.sql), ""
		if i+2 < len(tokens) && tokens[i+1].text(b.sql) == "." {
			schema = name
			name = tokens[i+2].identText(b.sql)
		}
		if strings.EqualFold(name, table.Name) && strings.EqualFold(schema, table.Schema) {
			b.usedOffsets[token.start] = true
			return token.start
		}
	}
	return -1
}
