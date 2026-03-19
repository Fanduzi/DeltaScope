// Package audit orchestrates audit use cases at the application layer.
// input: application-owned parsed SQL statements and hidden TiDB AST nodes
// output: first-pass StatementSpec values for later rule evaluation
// pos: application extraction step between parsing and rule execution
// note: if this file changes, update this header and module README.md.
package audit

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// Extract converts parsed statements into first-pass domain StatementSpec values.
func Extract(parsed ParsedSQL) ([]spec.Statement, error) {
	statements := make([]spec.Statement, 0, len(parsed.Statements))
	for _, stmt := range parsed.Statements {
		extracted, err := extractStatement(parsed.Dialect, parsed.Warnings, stmt)
		if err != nil {
			return nil, err
		}
		statements = append(statements, extracted)
	}
	return statements, nil
}

func extractStatement(dialect spec.Dialect, warnings []string, parsed ParsedStatement) (spec.Statement, error) {
	statement := spec.Statement{
		Kind:          parsed.Kind,
		Dialect:       dialect,
		RawSQL:        parsed.RawSQL,
		NormalizedSQL: normalizeSQL(parsed.RawSQL),
		Warnings:      append([]string(nil), warnings...),
	}

	switch node := parsed.node.(type) {
	case *ast.CreateTableStmt:
		statement.DDL = extractCreateTable(node)
	case *ast.AlterTableStmt:
		statement.DDL = extractAlterTable(node)
	case *ast.InsertStmt:
		statement.DML = extractInsert(node)
	case *ast.UpdateStmt:
		statement.DML = extractUpdate(node)
	case *ast.DeleteStmt:
		statement.DML = extractDelete(node)
	default:
		return spec.Statement{}, fmt.Errorf("unsupported parsed statement kind %q", parsed.Kind)
	}

	return statement, nil
}

func normalizeSQL(sql string) string {
	return strings.TrimSuffix(strings.TrimSpace(sql), ";")
}

func extractCreateTable(stmt *ast.CreateTableStmt) *spec.DDL {
	ddl := &spec.DDL{
		Table: &spec.Table{
			Name: stmt.Table.Name.L,
		},
		Columns: make([]spec.Column, 0, len(stmt.Cols)),
		Indexes: make([]spec.Index, 0, len(stmt.Constraints)),
		Options: make(map[string]string),
	}

	for _, col := range stmt.Cols {
		ddl.Columns = append(ddl.Columns, spec.Column{
			Name:    col.Name.Name.L,
			Type:    col.Tp.String(),
			Comment: extractColumnComment(col.Options),
		})
	}

	for _, c := range stmt.Constraints {
		ddl.Indexes = append(ddl.Indexes, spec.Index{
			Name:    normalizeConstraintName(c),
			Columns: extractIndexColumns(c.Keys),
		})
	}

	for _, option := range stmt.Options {
		switch option.Tp {
		case ast.TableOptionComment:
			ddl.Table.Comment = option.StrValue
			ddl.Options["comment"] = option.StrValue
		case ast.TableOptionEngine:
			ddl.Options["engine"] = option.StrValue
		case ast.TableOptionCharset:
			ddl.Options["charset"] = option.StrValue
		}
	}

	return ddl
}

func extractAlterTable(stmt *ast.AlterTableStmt) *spec.DDL {
	ddl := &spec.DDL{
		Table: &spec.Table{
			Name: stmt.Table.Name.L,
		},
		Alter: make([]spec.Alter, 0, len(stmt.Specs)),
	}

	for _, s := range stmt.Specs {
		ddl.Alter = append(ddl.Alter, spec.Alter{
			Action: alterActionName(s.Tp),
			Name:   extractAlterName(s),
		})
	}

	return ddl
}

func extractInsert(stmt *ast.InsertStmt) *spec.DML {
	return &spec.DML{
		InsertRows:   len(stmt.Lists),
		IsReplace:    stmt.IsReplace,
		IsSelectInto: stmt.Select != nil,
		HasSubquery:  stmt.Select != nil || exprHasSubquery(stmt.WhereExpr()),
		HasJoinOn:    joinHasOn(tableRefsJoin(stmt.Table)),
	}
}

func extractUpdate(stmt *ast.UpdateStmt) *spec.DML {
	return &spec.DML{
		HasWhere:    stmt.Where != nil,
		HasLimit:    stmt.Limit != nil,
		HasOrderBy:  stmt.Order != nil,
		HasSubquery: exprHasSubquery(stmt.Where),
		HasJoinOn:   joinHasOn(tableRefsJoin(stmt.TableRefs)),
	}
}

func extractDelete(stmt *ast.DeleteStmt) *spec.DML {
	return &spec.DML{
		HasWhere:    stmt.Where != nil,
		HasLimit:    stmt.Limit != nil,
		HasOrderBy:  stmt.Order != nil,
		HasSubquery: exprHasSubquery(stmt.Where),
		HasJoinOn:   joinHasOn(tableRefsJoin(stmt.TableRefs)),
	}
}

func extractColumnComment(options []*ast.ColumnOption) string {
	for _, option := range options {
		if option.Tp == ast.ColumnOptionComment && option.Expr != nil {
			return option.Expr.Text()
		}
	}
	return ""
}

func extractIndexColumns(parts []*ast.IndexPartSpecification) []string {
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Column == nil {
			continue
		}
		columns = append(columns, part.Column.Name.L)
	}
	return columns
}

func normalizeConstraintName(c *ast.Constraint) string {
	if c.Name != "" {
		return strings.ToLower(c.Name)
	}
	return constraintTypeName(c.Tp)
}

func extractAlterName(specification *ast.AlterTableSpec) string {
	switch {
	case len(specification.NewColumns) > 0 && specification.NewColumns[0] != nil:
		return specification.NewColumns[0].Name.Name.L
	case specification.OldColumnName != nil:
		return specification.OldColumnName.Name.L
	case specification.NewColumnName != nil:
		return specification.NewColumnName.Name.L
	case specification.Constraint != nil:
		return normalizeConstraintName(specification.Constraint)
	case specification.IndexName.L != "":
		return specification.IndexName.L
	case specification.Name != "":
		return strings.ToLower(specification.Name)
	default:
		return ""
	}
}

func tableRefsJoin(tableRefs *ast.TableRefsClause) *ast.Join {
	if tableRefs == nil {
		return nil
	}
	return tableRefs.TableRefs
}

func joinHasOn(join *ast.Join) bool {
	if join == nil {
		return false
	}
	if join.On != nil {
		return true
	}
	if left, ok := join.Left.(*ast.Join); ok && joinHasOn(left) {
		return true
	}
	if right, ok := join.Right.(*ast.Join); ok && joinHasOn(right) {
		return true
	}
	return false
}

func exprHasSubquery(expr ast.ExprNode) bool {
	found := false
	if expr == nil {
		return false
	}
	expr.Accept(subqueryVisitor{found: &found})
	return found
}

type subqueryVisitor struct {
	found *bool
}

func (v subqueryVisitor) Enter(in ast.Node) (ast.Node, bool) {
	if _, ok := in.(*ast.SubqueryExpr); ok {
		*v.found = true
		return in, true
	}
	return in, false
}

func (v subqueryVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func alterActionName(tp ast.AlterTableType) string {
	switch tp {
	case ast.AlterTableAddColumns:
		return "add_columns"
	case ast.AlterTableDropColumn:
		return "drop_column"
	case ast.AlterTableModifyColumn:
		return "modify_column"
	case ast.AlterTableChangeColumn:
		return "change_column"
	case ast.AlterTableRenameColumn:
		return "rename_column"
	case ast.AlterTableRenameTable:
		return "rename_table"
	case ast.AlterTableDropPrimaryKey:
		return "drop_primary_key"
	case ast.AlterTableDropIndex:
		return "drop_index"
	case ast.AlterTableAddConstraint:
		return "add_constraint"
	case ast.AlterTableOption:
		return "table_option"
	default:
		return fmt.Sprintf("alter_%d", tp)
	}
}

func constraintTypeName(tp ast.ConstraintType) string {
	switch tp {
	case ast.ConstraintPrimaryKey:
		return "primary"
	case ast.ConstraintKey:
		return "key"
	case ast.ConstraintIndex:
		return "index"
	case ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex:
		return "unique"
	default:
		return fmt.Sprintf("constraint_%d", tp)
	}
}
