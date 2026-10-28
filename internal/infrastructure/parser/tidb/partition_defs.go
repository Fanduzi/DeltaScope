// Package tidbparser: partition member-definition and scheme extraction.
//
// The member name list (extractor.go partitionMemberNames) keeps the
// coarse directory; this file keeps the fidelity layer underneath it —
// declared member boundary clauses, COLUMNS member names, the partitioning
// expression identity, and authored counts — so topology actions can bind
// declared targets to catalog bounds without re-parsing authored text.
//
// Comparison discipline: a bound literal is comparable only inside its own
// value domain (numeric digits, decoded string bytes, NULL). Non-literal
// bound items record their structured expression identity and are never
// equated by it — the model answers "unproven", not "equal".
package tidbparser

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/opcode"
)

// partitionMethodKind normalizes one method node to the bounded type
// vocabulary (the partitionMethodName contract lifted to the
// PartitionMethod level so subpartition methods share it).
func partitionMethodKind(m *ast.PartitionMethod) string {
	base := strings.ToLower(m.Tp.String())
	switch m.Tp {
	case ast.PartitionTypeRange, ast.PartitionTypeList:
		if len(m.ColumnNames) > 0 {
			return base + " columns"
		}
	case ast.PartitionTypeHash, ast.PartitionTypeKey:
		if m.Linear {
			return "linear " + base
		}
	}
	return base
}

// partitionDefsFromAST projects the declared member definitions into the
// normalized list — names in authored order, each member's boundary clause
// and subpartition mark preserved. A nil member entry produces a
// nameless def (the parser may keep the slot), never a skipped position:
// positions are the catalog ordinal contract.
func partitionDefsFromAST(defs []*ast.PartitionDefinition) []spec.PartitionDef {
	if len(defs) == 0 {
		return nil
	}
	out := make([]spec.PartitionDef, 0, len(defs))
	for _, def := range defs {
		if def == nil {
			out = append(out, spec.PartitionDef{})
			continue
		}
		member := spec.PartitionDef{
			Name:           def.Name.L,
			Bound:          partitionBoundFromClause(def.Clause),
			Subpartitioned: len(def.Sub) > 0,
		}
		out = append(out, member)
	}
	return out
}

// partitionBoundFromClause projects one member's VALUES clause. ClauseNone
// (HASH/KEY members) yields nil — the member declared no bound at all,
// which is not an unproven bound. Unmodeled clause shapes (system-time
// HISTORY/CURRENT) yield a bare Kind-less bound with no rows: the clause
// existed, its items stay unproven.
func partitionBoundFromClause(clause ast.PartitionDefinitionClause) *spec.PartitionBound {
	switch c := clause.(type) {
	case *ast.PartitionDefinitionClauseLessThan:
		bound := &spec.PartitionBound{Kind: "less_than"}
		row := make([]spec.BoundLiteral, 0, len(c.Exprs))
		allMax := len(c.Exprs) > 0
		for _, expr := range c.Exprs {
			if _, isMax := expr.(*ast.MaxValueExpr); isMax {
				// Keep the item in position — a COLUMNS tuple like
				// (10,MAXVALUE) is not the scalar +∞ tail; only a row
				// composed entirely of MAXVALUE items is.
				row = append(row, spec.BoundLiteral{Kind: spec.PartitionBoundMaxValue})
				continue
			}
			allMax = false
			row = append(row, boundLiteralFromExpr(expr))
		}
		if len(row) > 0 {
			bound.Rows = [][]spec.BoundLiteral{row}
			bound.MaxValue = allMax
		}
		return bound
	case *ast.PartitionDefinitionClauseIn:
		bound := &spec.PartitionBound{Kind: "in"}
		if len(c.Values) == 0 {
			bound.Default = true
			return bound
		}
		if len(c.Values) == 1 && len(c.Values[0]) == 1 {
			if _, isDefault := c.Values[0][0].(*ast.DefaultExpr); isDefault {
				bound.Default = true
				return bound
			}
		}
		bound.Rows = make([][]spec.BoundLiteral, 0, len(c.Values))
		for _, tuple := range c.Values {
			row := make([]spec.BoundLiteral, 0, len(tuple))
			for _, expr := range tuple {
				row = append(row, boundLiteralFromExpr(expr))
			}
			bound.Rows = append(bound.Rows, row)
		}
		return bound
	case *ast.PartitionDefinitionClauseNone, nil:
		return nil
	default:
		return &spec.PartitionBound{}
	}
}

// boundLiteralFromExpr projects one bound item into a comparable literal
// or a structured-expression record:
//   - NULL → PartitionBoundNull
//   - string literal → PartitionBoundString with decoded bytes
//   - integer literal (including the signed-unary form) → PartitionBoundNumeric
//     with canonical digits
//   - every other literal (decimal, float, hex, time) and every
//     non-literal expression → PartitionBoundExpression carrying the
//     structured facts the extractor already computes; equality across
//     such items is never claimed.
func boundLiteralFromExpr(expr ast.ExprNode) spec.BoundLiteral {
	switch node := unwrapParenthesesExpr(expr).(type) {
	case ast.ValueExpr:
		switch value := node.GetValue().(type) {
		case nil:
			return spec.BoundLiteral{Kind: spec.PartitionBoundNull}
		case string:
			return spec.BoundLiteral{Kind: spec.PartitionBoundString, Value: value}
		case int64:
			return spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: strconv.FormatInt(value, 10)}
		case uint64:
			return spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: strconv.FormatUint(value, 10)}
		}
	case *ast.UnaryOperationExpr:
		if inner, ok := unwrapParenthesesExpr(node.V).(ast.ValueExpr); ok {
			switch value := inner.GetValue().(type) {
			case int64:
				if node.Op == opcode.Minus {
					return spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: strconv.FormatInt(-value, 10)}
				}
				if node.Op == opcode.Plus {
					return spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: strconv.FormatInt(value, 10)}
				}
			case uint64:
				if node.Op == opcode.Plus {
					return spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: strconv.FormatUint(value, 10)}
				}
			}
		}
	}
	facts := expressionFacts(expr)
	return spec.BoundLiteral{Kind: spec.PartitionBoundExpression, Expression: &facts}
}

// partitionSchemeFromAST projects a PARTITION BY declaration (the `partition`
// action's payload) into the normalized scheme: method, COLUMNS member
// names, expression identity, authored count, member definitions, and the
// subpartition/interval marks that keep the member space unproven.
func partitionSchemeFromAST(opt *ast.PartitionOptions) *spec.PartitionScheme {
	if opt == nil {
		return nil
	}
	scheme := &spec.PartitionScheme{
		Type:    partitionMethodName(opt),
		Columns: partitionColumnNames(opt),
		Defs:    partitionDefsFromAST(opt.Definitions),
	}
	if opt.Expr != nil {
		facts := expressionFacts(opt.Expr)
		scheme.Expr = &facts
	}
	if opt.Num > 0 {
		count := opt.Num
		scheme.Count = &count
	}
	if opt.Sub != nil {
		scheme.Subpartitioned = true
		scheme.SubType = partitionMethodKind(opt.Sub)
	}
	if opt.Interval != nil {
		scheme.Interval = true
	}
	for _, def := range opt.Definitions {
		if def != nil && len(def.Sub) > 0 {
			scheme.Subpartitioned = true
		}
	}
	return scheme
}

// partitionColumnNames projects the declared COLUMNS member list — the
// KEY / RANGE COLUMNS / LIST COLUMNS column arguments in authored order.
func partitionColumnNames(opt *ast.PartitionOptions) []string {
	if len(opt.ColumnNames) == 0 {
		return nil
	}
	columns := make([]string, 0, len(opt.ColumnNames))
	for _, column := range opt.ColumnNames {
		if column == nil {
			continue
		}
		columns = append(columns, column.Name.L)
	}
	return columns
}
