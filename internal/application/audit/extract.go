// Package audit orchestrates audit use cases at the application layer.
// input: application-owned parsed SQL statements and hidden TiDB AST nodes
// output: first-pass StatementSpec values plus honest statement-local alter change facts for later rule evaluation
// pos: application extraction step between parsing and rule execution
// note: if this file changes, update this header and module README.md.
package audit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/opcode"
	tidbtypes "github.com/pingcap/tidb/pkg/parser/types"
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
	}
	if parsed.Kind == spec.KindUnknown {
		statement.Warnings = append([]string(nil), warnings...)
	}

	switch node := parsed.node.(type) {
	case *ast.CreateTableStmt:
		statement.DDL = extractCreateTable(node)
	case *ast.CreateViewStmt:
		statement.DDL = extractCreateView(node)
	case *ast.AlterTableStmt:
		statement.DDL = extractAlterTable(node)
	case *ast.DropTableStmt:
		statement.DDL = extractDropTable(node)
	case *ast.TruncateTableStmt:
		statement.DDL = extractTruncateTable(node)
	case *ast.InsertStmt:
		statement.DML = extractInsert(node)
	case *ast.UpdateStmt:
		statement.DML = extractUpdate(node)
	case *ast.DeleteStmt:
		statement.DML = extractDelete(node)
	default:
		statement.Warnings = append(statement.Warnings, fmt.Sprintf("unsupported parsed statement kind %q", parsed.Kind))
	}
	if statement.DML != nil {
		statement.DML.Impact = estimateStatementImpact(statement)
	}

	return statement, nil
}

func normalizeSQL(sql string) string {
	return strings.TrimSuffix(strings.TrimSpace(sql), ";")
}

func extractCreateTable(stmt *ast.CreateTableStmt) *spec.DDL {
	ddl := &spec.DDL{
		Operation: spec.DDLOperationCreateTable,
		Table: &spec.Table{
			Schema: stmt.Table.Schema.L,
			Name:   stmt.Table.Name.L,
		},
		Columns:       make([]spec.Column, 0, len(stmt.Cols)),
		Indexes:       make([]spec.Index, 0, len(stmt.Constraints)),
		Constraints:   make([]spec.Constraint, 0),
		Options:       make(map[string]string),
		HasReferTable: stmt.ReferTable != nil,
		HasSelect:     stmt.Select != nil,
		HasPartition:  stmt.Partition != nil,
	}

	for _, col := range stmt.Cols {
		ddl.Columns = append(ddl.Columns, extractColumn(col))
	}

	for _, c := range stmt.Constraints {
		switch c.Tp {
		case ast.ConstraintPrimaryKey:
			ddl.PrimaryKey = &spec.Index{
				Name:    normalizeConstraintName(c),
				Kind:    spec.IndexKindPrimary,
				Columns: extractIndexColumns(c.Keys),
			}
		case ast.ConstraintKey, ast.ConstraintIndex, ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex, ast.ConstraintFulltext:
			ddl.Indexes = append(ddl.Indexes, spec.Index{
				Name:    normalizeConstraintName(c),
				Kind:    indexKindForConstraint(c.Tp),
				Columns: extractIndexColumns(c.Keys),
			})
		default:
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{
				Type:    constraintTypeName(c.Tp),
				Name:    normalizeConstraintName(c),
				Columns: extractIndexColumns(c.Keys),
			})
		}
	}

	for key, value := range extractTableOptions(stmt.Options) {
		ddl.Options[key] = value
		if key == "comment" {
			ddl.Table.Comment = value
		}
	}

	return ddl
}

func extractCreateView(stmt *ast.CreateViewStmt) *spec.DDL {
	return &spec.DDL{
		Operation: spec.DDLOperationCreateView,
		Table: &spec.Table{
			Schema: stmt.ViewName.Schema.L,
			Name:   stmt.ViewName.Name.L,
		},
		HasSelect: stmt.Select != nil,
	}
}

func extractAlterTable(stmt *ast.AlterTableStmt) *spec.DDL {
	ddl := &spec.DDL{
		Operation: spec.DDLOperationAlterTable,
		Table: &spec.Table{
			Schema: stmt.Table.Schema.L,
			Name:   stmt.Table.Name.L,
		},
		Alter: make([]spec.Alter, 0, len(stmt.Specs)),
	}

	for _, s := range stmt.Specs {
		ddl.Alter = append(ddl.Alter, extractAlterSpecs(s)...)
	}

	return ddl
}

func extractDropTable(stmt *ast.DropTableStmt) *spec.DDL {
	operation := spec.DDLOperationDropTable
	if stmt.IsView {
		operation = spec.DDLOperationDropView
	}
	ddl := &spec.DDL{
		Operation: operation,
		Options:   map[string]string{},
	}
	if stmt.IfExists {
		ddl.Options["if_exists"] = "true"
	}
	if len(stmt.Tables) > 0 && stmt.Tables[0] != nil {
		ddl.Table = &spec.Table{Schema: stmt.Tables[0].Schema.L, Name: stmt.Tables[0].Name.L}
	}
	if len(stmt.Tables) > 1 {
		ddl.Options["multiple_targets"] = strconv.Itoa(len(stmt.Tables))
	}
	return ddl
}

func extractTruncateTable(stmt *ast.TruncateTableStmt) *spec.DDL {
	ddl := &spec.DDL{
		Operation: spec.DDLOperationTruncateTable,
	}
	if stmt.Table != nil {
		ddl.Table = &spec.Table{Schema: stmt.Table.Schema.L, Name: stmt.Table.Name.L}
	}
	return ddl
}

func extractAlterSpecs(specification *ast.AlterTableSpec) []spec.Alter {
	if specification.Tp == ast.AlterTableAddColumns && len(specification.NewColumns) > 0 {
		alters := make([]spec.Alter, 0, len(specification.NewColumns))
		for _, column := range specification.NewColumns {
			if column == nil {
				continue
			}
			alters = append(alters, spec.Alter{
				Action: alterActionName(specification.Tp),
				Name:   column.Name.Name.L,
				Column: alterColumnFromColumnDef(column),
			})
		}
		return alters
	}
	return []spec.Alter{extractAlterSpec(specification)}
}

func extractAlterSpec(specification *ast.AlterTableSpec) spec.Alter {
	alter := spec.Alter{
		Action: alterActionName(specification.Tp),
		Name:   extractAlterName(specification),
	}

	if column := extractAlterColumn(specification); column != nil {
		alter.Column = column
	}
	if index := extractAlterIndex(specification); index != nil {
		alter.Index = index
	}
	if options := extractTableOptions(specification.Options); len(options) > 0 {
		alter.Options = options
	}

	return alter
}

func extractInsert(stmt *ast.InsertStmt) *spec.DML {
	join := tableRefsJoin(stmt.Table)
	tables := extractMutationTables(join)
	if len(tables) == 0 && stmt.Table != nil && stmt.Table.TableRefs != nil {
		tables = extractMutationTables(stmt.Table.TableRefs)
	}
	return &spec.DML{
		Operation:      spec.DMLOperationInsert,
		Tables:         tables,
		InsertRows:     len(stmt.Lists),
		IsReplace:      stmt.IsReplace,
		IsInsertSelect: stmt.Select != nil,
		HasOnDuplicate: len(stmt.OnDuplicate) > 0,
		HasSubquery:    nodeHasSubquery(stmt),
		HasJoin:        joinExists(join),
		HasJoinOn:      joinHasOn(join),
	}
}

func extractUpdate(stmt *ast.UpdateStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractMutationTables(join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{
		Operation:      spec.DMLOperationUpdate,
		Tables:         tables,
		HasWhere:       stmt.Where != nil,
		HasLimit:       stmt.Limit != nil,
		HasOrderBy:     stmt.Order != nil,
		HasSubquery:    hasSubquery,
		HasJoin:        joinExists(join),
		HasJoinOn:      joinHasOn(join),
		PredicateShape: shape,
		LookupColumns:  lookupColumns,
		MatchedKeyName: matchedKeyName,
		MatchedKeyKind: matchedKeyKind,
		IsSingleTable:  isSingleTable,
	}
}

func extractDelete(stmt *ast.DeleteStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractMutationTables(join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{
		Operation:      spec.DMLOperationDelete,
		Tables:         tables,
		HasWhere:       stmt.Where != nil,
		HasLimit:       stmt.Limit != nil,
		HasOrderBy:     stmt.Order != nil,
		HasSubquery:    hasSubquery,
		HasJoin:        joinExists(join),
		HasJoinOn:      joinHasOn(join),
		PredicateShape: shape,
		LookupColumns:  lookupColumns,
		MatchedKeyName: matchedKeyName,
		MatchedKeyKind: matchedKeyKind,
		IsSingleTable:  isSingleTable,
	}
}

func extractColumn(col *ast.ColumnDef) spec.Column {
	column := spec.Column{
		Name:     col.Name.Name.L,
		Type:     strings.ToLower(col.Tp.String()),
		Length:   col.Tp.GetFlen(),
		Unsigned: mysql.HasUnsignedFlag(col.Tp.GetFlag()),
	}
	if tidbtypes.HasCharset(col.Tp) {
		column.Charset = strings.ToLower(col.Tp.GetCharset())
		column.Collation = strings.ToLower(col.Tp.GetCollate())
	}

	for _, option := range col.Options {
		if option == nil {
			continue
		}

		switch option.Tp {
		case ast.ColumnOptionCollate:
			column.Collation = strings.ToLower(option.StrValue)
		case ast.ColumnOptionComment:
			if option.Expr != nil {
				column.Comment = normalizedExprText(option.Expr)
			}
		case ast.ColumnOptionNotNull:
			column.NotNull = true
		case ast.ColumnOptionAutoIncrement:
			column.AutoIncrement = true
		case ast.ColumnOptionDefaultValue:
			column.HasDefault = true
			column.DefaultValue = normalizedExprText(option.Expr)
			column.DefaultIsNull = strings.EqualFold(column.DefaultValue, "null")
			column.DefaultIsCurrentTimestamp = exprIsCurrentTimestamp(option.Expr)
		case ast.ColumnOptionOnUpdate:
			column.OnUpdateCurrentTimestamp = exprIsCurrentTimestamp(option.Expr)
		}
	}

	return column
}

func extractAlterColumn(specification *ast.AlterTableSpec) *spec.AlterColumn {
	switch specification.Tp {
	case ast.AlterTableAddColumns, ast.AlterTableModifyColumn, ast.AlterTableChangeColumn:
		if len(specification.NewColumns) == 0 || specification.NewColumns[0] == nil {
			return nil
		}
		column := alterColumnFromColumnDef(specification.NewColumns[0])
		if specification.Tp == ast.AlterTableModifyColumn || specification.Tp == ast.AlterTableChangeColumn {
			column.Change = alterColumnChangeFacts(specification.NewColumns[0])
		}
		if specification.Tp == ast.AlterTableChangeColumn && specification.OldColumnName != nil {
			column.OldName = specification.OldColumnName.Name.L
		}
		return column
	case ast.AlterTableDropColumn:
		if specification.OldColumnName == nil {
			return nil
		}
		return &spec.AlterColumn{
			OldName: specification.OldColumnName.Name.L,
		}
	case ast.AlterTableRenameColumn:
		if specification.OldColumnName == nil || specification.NewColumnName == nil {
			return nil
		}
		return &spec.AlterColumn{
			OldName: specification.OldColumnName.Name.L,
			Definition: &spec.Column{
				Name: specification.NewColumnName.Name.L,
			},
		}
	default:
		return nil
	}
}

func alterColumnFromColumnDef(col *ast.ColumnDef) *spec.AlterColumn {
	extracted := extractColumn(col)
	return &spec.AlterColumn{
		Definition: &extracted,
	}
}

func alterColumnChangeFacts(col *ast.ColumnDef) *spec.AlterColumnChange {
	if col == nil {
		return nil
	}

	change := &spec.AlterColumnChange{}

	for _, option := range col.Options {
		if option == nil {
			continue
		}

		switch option.Tp {
		case ast.ColumnOptionNull, ast.ColumnOptionNotNull:
			change.TouchesNullability = true
		case ast.ColumnOptionDefaultValue:
			change.TouchesDefault = true
		case ast.ColumnOptionAutoIncrement:
			change.TouchesAutoIncrement = true
		}
	}

	if !change.TouchesNullability &&
		!change.TouchesDefault &&
		!change.TouchesAutoIncrement {
		return nil
	}

	return change
}

func extractAlterIndex(specification *ast.AlterTableSpec) *spec.AlterIndex {
	switch specification.Tp {
	case ast.AlterTableAddConstraint:
		if specification.Constraint == nil || !constraintProducesIndex(specification.Constraint.Tp) {
			return nil
		}
		return &spec.AlterIndex{
			Definition: &spec.Index{
				Kind:    indexKindForConstraint(specification.Constraint.Tp),
				Name:    normalizeConstraintName(specification.Constraint),
				Columns: extractIndexColumns(specification.Constraint.Keys),
			},
		}
	case ast.AlterTableDropIndex:
		name := extractAlterName(specification)
		if name == "" {
			return nil
		}
		return &spec.AlterIndex{OldName: name}
	case ast.AlterTableRenameIndex:
		if specification.FromKey.L == "" && specification.ToKey.L == "" {
			return nil
		}
		return &spec.AlterIndex{
			OldName: specification.FromKey.L,
			Definition: &spec.Index{
				Name: specification.ToKey.L,
			},
		}
	case ast.AlterTableDropPrimaryKey:
		return &spec.AlterIndex{
			OldName: "primary",
		}
	default:
		return nil
	}
}

func constraintProducesIndex(tp ast.ConstraintType) bool {
	switch tp {
	case ast.ConstraintPrimaryKey, ast.ConstraintKey, ast.ConstraintIndex, ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex, ast.ConstraintFulltext:
		return true
	default:
		return false
	}
}

func extractTableOptions(options []*ast.TableOption) map[string]string {
	if len(options) == 0 {
		return nil
	}

	extracted := make(map[string]string)
	for _, option := range options {
		if option == nil {
			continue
		}
		switch option.Tp {
		case ast.TableOptionComment:
			extracted["comment"] = option.StrValue
		case ast.TableOptionEngine:
			extracted["engine"] = option.StrValue
		case ast.TableOptionCharset:
			extracted["charset"] = option.StrValue
		case ast.TableOptionRowFormat:
			if value := rowFormatName(option.UintValue); value != "" {
				extracted["row_format"] = value
			}
		case ast.TableOptionAutoIncrement:
			extracted["auto_increment"] = strconv.FormatUint(option.UintValue, 10)
		}
	}
	if len(extracted) == 0 {
		return nil
	}
	return extracted
}

func rowFormatName(value uint64) string {
	switch value {
	case ast.RowFormatDefault:
		return "DEFAULT"
	case ast.RowFormatDynamic:
		return "DYNAMIC"
	case ast.RowFormatFixed:
		return "FIXED"
	case ast.RowFormatCompressed:
		return "COMPRESSED"
	case ast.RowFormatRedundant:
		return "REDUNDANT"
	case ast.RowFormatCompact:
		return "COMPACT"
	case ast.TokuDBRowFormatDefault:
		return "TOKUDB_DEFAULT"
	case ast.TokuDBRowFormatFast:
		return "TOKUDB_FAST"
	case ast.TokuDBRowFormatSmall:
		return "TOKUDB_SMALL"
	case ast.TokuDBRowFormatZlib:
		return "TOKUDB_ZLIB"
	case ast.TokuDBRowFormatQuickLZ:
		return "TOKUDB_QUICKLZ"
	case ast.TokuDBRowFormatLzma:
		return "TOKUDB_LZMA"
	case ast.TokuDBRowFormatSnappy:
		return "TOKUDB_SNAPPY"
	case ast.TokuDBRowFormatUncompressed:
		return "TOKUDB_UNCOMPRESSED"
	case ast.TokuDBRowFormatZstd:
		return "TOKUDB_ZSTD"
	default:
		return ""
	}
}

func normalizedExprText(expr ast.ExprNode) string {
	if expr == nil {
		return ""
	}
	if valueExpr, ok := expr.(ast.ValueExpr); ok {
		switch value := valueExpr.GetValue().(type) {
		case string:
			return fmt.Sprintf("'%s'", value)
		default:
			return fmt.Sprint(value)
		}
	}
	return strings.TrimSpace(expr.Text())
}

func exprIsCurrentTimestamp(expr ast.ExprNode) bool {
	if expr == nil {
		return false
	}
	if funcCall, ok := expr.(*ast.FuncCallExpr); ok {
		return funcCall.FnName.L == ast.CurrentTimestamp
	}
	return strings.EqualFold(normalizedExprText(expr), "current_timestamp")
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

func indexKindForConstraint(tp ast.ConstraintType) spec.IndexKind {
	switch tp {
	case ast.ConstraintPrimaryKey:
		return spec.IndexKindPrimary
	case ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex:
		return spec.IndexKindUnique
	case ast.ConstraintFulltext:
		return spec.IndexKindFulltext
	case ast.ConstraintKey, ast.ConstraintIndex:
		return spec.IndexKindSecondary
	default:
		return spec.IndexKindUnknown
	}
}

func normalizeConstraintName(c *ast.Constraint) string {
	if c.Name != "" {
		return strings.ToLower(c.Name)
	}
	if c.Tp == ast.ConstraintPrimaryKey {
		return "primary"
	}
	return ""
}

func extractAlterName(specification *ast.AlterTableSpec) string {
	switch {
	case specification.OldColumnName != nil:
		return specification.OldColumnName.Name.L
	case specification.NewColumnName != nil:
		return specification.NewColumnName.Name.L
	case len(specification.NewColumns) > 0 && specification.NewColumns[0] != nil:
		return specification.NewColumns[0].Name.Name.L
	case specification.Constraint != nil:
		return normalizeConstraintName(specification.Constraint)
	case specification.Tp == ast.AlterTableDropPrimaryKey:
		return "primary"
	case specification.FromKey.L != "":
		return specification.FromKey.L
	case specification.ToKey.L != "":
		return specification.ToKey.L
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

func extractMutationTables(join *ast.Join) []spec.Table {
	if join == nil {
		return nil
	}
	tables := make([]spec.Table, 0, 2)
	collectMutationTables(join, &tables)
	return tables
}

func collectMutationTables(node ast.ResultSetNode, tables *[]spec.Table) {
	switch typed := node.(type) {
	case nil:
		return
	case *ast.Join:
		collectMutationTables(typed.Left, tables)
		collectMutationTables(typed.Right, tables)
	case *ast.TableSource:
		collectMutationTables(typed.Source, tables)
	case *ast.TableName:
		if typed.Name.L == "" || containsTable(*tables, typed.Schema.L, typed.Name.L) {
			return
		}
		*tables = append(*tables, spec.Table{Schema: typed.Schema.L, Name: typed.Name.L})
	}
}

func containsTable(items []spec.Table, schema string, name string) bool {
	for _, item := range items {
		if strings.EqualFold(item.Schema, schema) && strings.EqualFold(item.Name, name) {
			return true
		}
	}
	return false
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

func joinExists(join *ast.Join) bool {
	if join == nil {
		return false
	}
	return join.Right != nil
}

func extractMutationPredicateShape(where ast.ExprNode, join *ast.Join, isSingleTable bool) (spec.PredicateShape, []string, string, spec.IndexKind) {
	switch {
	case joinExists(join):
		return spec.PredicateShapeJoin, nil, "", spec.IndexKindUnknown
	case where == nil:
		return spec.PredicateShapeMissingWhere, nil, "", spec.IndexKindUnknown
	case nodeHasSubquery(where):
		return spec.PredicateShapeSubquery, nil, "", spec.IndexKindUnknown
	case predicateIsIDLiteralEquality(where, isSingleTable):
		return spec.PredicateShapeUniqueEquality, []string{"id"}, "PRIMARY", spec.IndexKindPrimary
	default:
		return spec.PredicateShapeUnknown, nil, "", spec.IndexKindUnknown
	}
}

func predicateIsIDLiteralEquality(where ast.ExprNode, isSingleTable bool) bool {
	if !isSingleTable {
		return false
	}

	predicate, ok := unwrapParenthesesExpr(where).(*ast.BinaryOperationExpr)
	if !ok || predicate.Op != opcode.EQ {
		return false
	}

	left := unwrapParenthesesExpr(predicate.L)
	right := unwrapParenthesesExpr(predicate.R)
	return (exprIsIDColumnRef(left) && exprIsLiteralValue(right)) ||
		(exprIsIDColumnRef(right) && exprIsLiteralValue(left))
}

func unwrapParenthesesExpr(expr ast.ExprNode) ast.ExprNode {
	current := expr
	for {
		grouped, ok := current.(*ast.ParenthesesExpr)
		if !ok || grouped == nil {
			return current
		}
		current = grouped.Expr
	}
}

func exprIsIDColumnRef(expr ast.ExprNode) bool {
	column, ok := unwrapParenthesesExpr(expr).(*ast.ColumnNameExpr)
	return ok && column != nil && column.Name != nil && strings.EqualFold(column.Name.Name.L, "id")
}

func exprIsLiteralValue(expr ast.ExprNode) bool {
	switch typed := unwrapParenthesesExpr(expr).(type) {
	case nil:
		return false
	case ast.ValueExpr:
		return true
	case ast.ParamMarkerExpr:
		return true
	case *ast.UnaryOperationExpr:
		return exprIsLiteralValue(typed.V)
	default:
		return false
	}
}

func nodeHasSubquery(node ast.Node) bool {
	found := false
	if node == nil {
		return false
	}
	node.Accept(subqueryVisitor{found: &found})
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
	case ast.AlterTableRenameIndex:
		return "rename_index"
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
	case ast.ConstraintForeignKey:
		return "foreign_key"
	case ast.ConstraintCheck:
		return "check"
	default:
		return fmt.Sprintf("constraint_%d", tp)
	}
}
