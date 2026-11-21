//go:build postgresql

package postgresql

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgCreateExpressionRefusal(items []*pg_query.Node) string {
	generated := make(map[string]bool)
	for _, item := range items {
		column := item.GetColumnDef()
		if hasColumnConstraint(column, pg_query.ConstrType_CONSTR_GENERATED) {
			generated[column.GetColname()] = true
		}
	}
	for _, item := range items {
		if column := item.GetColumnDef(); column != nil {
			for _, node := range column.GetConstraints() {
				if reason := pgCreateConstraintExpressionRefusal(node.GetConstraint(), generated); reason != "" {
					return reason
				}
			}
		} else if reason := pgCreateConstraintExpressionRefusal(item.GetConstraint(), generated); reason != "" {
			return reason
		}
	}
	return ""
}

func pgCreateConstraintExpressionRefusal(c *pg_query.Constraint, generated map[string]bool) string {
	expr := c.GetRawExpr()
	switch c.GetContype() {
	case pg_query.ConstrType_CONSTR_DEFAULT:
		if pgExpressionHasReference(expr, true) {
			return "create_default_disallowed_reference"
		}
	case pg_query.ConstrType_CONSTR_CHECK:
		if pgExpressionHasReference(expr, false) {
			return "create_check_subquery"
		}
	case pg_query.ConstrType_CONSTR_GENERATED:
		if pgExpressionHasReference(expr, false) {
			return "create_generated_subquery"
		}
		fields := expr.GetColumnRef().GetFields()
		if len(fields) == 1 && generated[stringNodeValue(fields[0])] {
			return "create_generated_references_generated"
		}
	}
	return ""
}
