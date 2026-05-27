// Package tidbparser extracts parser-neutral statements from TiDB AST nodes.
// input: TiDB parser statement nodes and parser-neutral dialect metadata
// output: extractor-backed parsed statements for the application layer
// pos: infrastructure extraction adapter between TiDB AST and domain spec
// note: if this file changes, update this header and module README.md.
package tidbparser

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

type StatementNode = ast.StmtNode

type (
	CreateTableStatement   = *ast.CreateTableStmt
	CreateViewStatement    = *ast.CreateViewStmt
	CreateIndexStatement   = *ast.CreateIndexStmt
	AlterTableStatement    = *ast.AlterTableStmt
	DropTableStatement     = *ast.DropTableStmt
	DropIndexStatement     = *ast.DropIndexStmt
	RenameTableStatement   = *ast.RenameTableStmt
	TruncateTableStatement = *ast.TruncateTableStmt
	InsertStatement        = *ast.InsertStmt
	UpdateStatement        = *ast.UpdateStmt
	DeleteStatement        = *ast.DeleteStmt

	AlterDatabaseStatement         = *ast.AlterDatabaseStmt
	ProcedureInfoStatement         = *ast.ProcedureInfo
	DropProcedureStatement         = *ast.DropProcedureStmt
	CreateUserStatement            = *ast.CreateUserStmt
	AlterUserStatement             = *ast.AlterUserStmt
	DropUserStatement              = *ast.DropUserStmt
	GrantStatement                 = *ast.GrantStmt
	RevokeStatement                = *ast.RevokeStmt
	DropResourceGroupStatement     = *ast.DropResourceGroupStmt
	CreatePlacementPolicyStatement = *ast.CreatePlacementPolicyStmt
	AlterPlacementPolicyStatement  = *ast.AlterPlacementPolicyStmt
	DropPlacementPolicyStatement   = *ast.DropPlacementPolicyStmt
	CreateSequenceStatement        = *ast.CreateSequenceStmt
	AlterSequenceStatement         = *ast.AlterSequenceStmt
	DropSequenceStatement          = *ast.DropSequenceStmt
)

// ExtractedStatement is the adapter-owned parser result used by the application layer.
type ExtractedStatement struct {
	Kind      spec.Kind
	RawSQL    string
	Extractor spec.StatementExtractor
}

type tidbExtractor struct {
	kind     spec.Kind
	warnings []string
	node     ast.StmtNode
}

func (e tidbExtractor) Extract(dialect spec.Dialect, rawSQL string) (spec.Statement, error) {
	statement := spec.Statement{
		Kind:          e.kind,
		Dialect:       dialect,
		RawSQL:        rawSQL,
		NormalizedSQL: normalizeSQL(rawSQL),
	}
	if e.kind == spec.KindUnknown {
		statement.Warnings = append([]string(nil), e.warnings...)
	}

	switch node := e.node.(type) {
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
	case *ast.CreateDatabaseStmt:
		statement.DDL = extractCreateDatabase(node)
	case *ast.DropDatabaseStmt:
		statement.DDL = extractDropDatabase(node)
	case *ast.InsertStmt:
		statement.DML = extractInsert(node)
	case *ast.UpdateStmt:
		statement.DML = extractUpdate(node)
	case *ast.DeleteStmt:
		statement.DML = extractDelete(node)
	case *ast.CreateIndexStmt:
		statement.DDL = extractCreateIndex(node)
	case *ast.DropIndexStmt:
		statement.DDL = extractDropIndex(node)
	case *ast.RenameTableStmt:
		statement.DDL = extractRenameTable(node)
	case *ast.AlterDatabaseStmt:
		statement.DDL = extractAlterDatabase(node)
	case *ast.ProcedureInfo:
		statement.DDL = extractCreateProcedure(node)
	case *ast.DropProcedureStmt:
		statement.DDL = extractDropProcedure(node)
	case *ast.CreateUserStmt:
		statement.DDL = extractCreateUser(node)
	case *ast.AlterUserStmt:
		statement.DDL = extractAlterUser(node)
	case *ast.DropUserStmt:
		statement.DDL = extractDropUser(node)
	case *ast.GrantStmt:
		statement.DDL = extractGrant(node)
	case *ast.RevokeStmt:
		statement.DDL = extractRevoke(node)
	case *ast.DropResourceGroupStmt:
		statement.DDL = extractDropResourceGroup(node)
	case *ast.CreatePlacementPolicyStmt:
		statement.DDL = extractCreatePlacementPolicy(node)
	case *ast.AlterPlacementPolicyStmt:
		statement.DDL = extractAlterPlacementPolicy(node)
	case *ast.DropPlacementPolicyStmt:
		statement.DDL = extractDropPlacementPolicy(node)
	case *ast.CreateSequenceStmt:
		statement.DDL = extractCreateSequence(node)
	case *ast.AlterSequenceStmt:
		statement.DDL = extractAlterSequence(node)
	case *ast.DropSequenceStmt:
		statement.DDL = extractDropSequence(node)
	default:
		statement.Warnings = append(statement.Warnings, fmt.Sprintf("unsupported parsed statement kind %q", e.kind))
	}
	return statement, nil
}

func WrapStatements(stmts []ast.StmtNode, warnings []string) []ExtractedStatement {
	wrapped := make([]ExtractedStatement, 0, len(stmts))
	for _, stmt := range stmts {
		kind := classify(stmt)
		wrapped = append(wrapped, ExtractedStatement{
			Kind:      kind,
			RawSQL:    stmt.Text(),
			Extractor: tidbExtractor{kind: kind, warnings: warnings, node: stmt},
		})
	}
	return wrapped
}

func classify(stmt ast.StmtNode) spec.Kind {
	switch stmt.(type) {
	case *ast.CreateTableStmt,
		*ast.CreateViewStmt,
		*ast.CreateIndexStmt,
		*ast.AlterTableStmt,
		*ast.DropTableStmt,
		*ast.DropIndexStmt,
		*ast.RenameTableStmt,
		*ast.TruncateTableStmt,
		*ast.CreateDatabaseStmt,
		*ast.DropDatabaseStmt,
		*ast.AlterDatabaseStmt,
		*ast.ProcedureInfo,
		*ast.DropProcedureStmt,
		*ast.CreateUserStmt,
		*ast.AlterUserStmt,
		*ast.DropUserStmt,
		*ast.GrantStmt,
		*ast.RevokeStmt,
		*ast.DropResourceGroupStmt,
		*ast.CreatePlacementPolicyStmt,
		*ast.AlterPlacementPolicyStmt,
		*ast.DropPlacementPolicyStmt,
		*ast.CreateSequenceStmt,
		*ast.AlterSequenceStmt,
		*ast.DropSequenceStmt:
		return spec.KindDDL
	case *ast.InsertStmt,
		*ast.UpdateStmt,
		*ast.DeleteStmt:
		return spec.KindDML
	default:
		return spec.KindUnknown
	}
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
			ddl.PrimaryKey = &spec.Index{Name: normalizeConstraintName(c), Kind: spec.IndexKindPrimary, Columns: extractIndexColumns(c.Keys)}
		case ast.ConstraintKey, ast.ConstraintIndex, ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex, ast.ConstraintFulltext:
			ddl.Indexes = append(ddl.Indexes, spec.Index{Name: normalizeConstraintName(c), Kind: indexKindForConstraint(c.Tp), Columns: extractIndexColumns(c.Keys)})
		default:
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{Type: constraintTypeName(c.Tp), Name: normalizeConstraintName(c), Columns: extractIndexColumns(c.Keys)})
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
	return &spec.DDL{Operation: spec.DDLOperationCreateView, Table: &spec.Table{Schema: stmt.ViewName.Schema.L, Name: stmt.ViewName.Name.L}, HasSelect: stmt.Select != nil}
}

func extractAlterTable(stmt *ast.AlterTableStmt) *spec.DDL {
	ddl := &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: stmt.Table.Schema.L, Name: stmt.Table.Name.L}, Alter: make([]spec.Alter, 0, len(stmt.Specs))}
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
	ddl := &spec.DDL{Operation: operation, Options: map[string]string{}}
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
	ddl := &spec.DDL{Operation: spec.DDLOperationTruncateTable}
	if stmt.Table != nil {
		ddl.Table = &spec.Table{Schema: stmt.Table.Schema.L, Name: stmt.Table.Name.L}
	}
	return ddl
}

func extractCreateDatabase(stmt *ast.CreateDatabaseStmt) *spec.DDL {
	options := map[string]string{}
	if stmt.IfNotExists {
		options["if_not_exists"] = "true"
	}
	for _, opt := range stmt.Options {
		if opt == nil {
			continue
		}
		switch opt.Tp {
		case ast.DatabaseOptionCharset:
			options["charset"] = opt.Value
		case ast.DatabaseOptionCollate:
			options["collate"] = opt.Value
		}
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationCreateSchema,
		ObjectName: stmt.Name.L,
		ObjectType: "database",
		Options:    options,
	}
}

func extractDropDatabase(stmt *ast.DropDatabaseStmt) *spec.DDL {
	options := map[string]string{}
	if stmt.IfExists {
		options["if_exists"] = "true"
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationDropSchema,
		ObjectName: stmt.Name.L,
		ObjectType: "database",
		Options:    options,
	}
}

func extractAlterSpecs(specification *ast.AlterTableSpec) []spec.Alter {
	if specification.Tp == ast.AlterTableAddColumns && len(specification.NewColumns) > 0 {
		alters := make([]spec.Alter, 0, len(specification.NewColumns))
		for _, column := range specification.NewColumns {
			if column == nil {
				continue
			}
			alters = append(alters, spec.Alter{Action: alterActionName(specification.Tp), Name: column.Name.Name.L, Column: alterColumnFromColumnDef(column)})
		}
		return alters
	}
	return []spec.Alter{extractAlterSpec(specification)}
}

func extractAlterSpec(specification *ast.AlterTableSpec) spec.Alter {
	alter := spec.Alter{Action: alterActionName(specification.Tp), Name: extractAlterName(specification)}
	if column := extractAlterColumn(specification); column != nil {
		alter.Column = column
	}
	if index := extractAlterIndex(specification); index != nil {
		alter.Index = index
	}
	if options := extractTableOptions(specification.Options); len(options) > 0 {
		alter.Options = options
		if _, ok := options["placement_policy"]; ok && alter.Action == "table_option" {
			alter.Action = "placement_policy"
		}
	}
	return alter
}

func extractInsert(stmt *ast.InsertStmt) *spec.DML {
	join := tableRefsJoin(stmt.Table)
	tables := extractMutationTables(join)
	if len(tables) == 0 && stmt.Table != nil && stmt.Table.TableRefs != nil {
		tables = extractMutationTables(stmt.Table.TableRefs)
	}
	return &spec.DML{Operation: spec.DMLOperationInsert, Tables: tables, InsertRows: len(stmt.Lists), IsReplace: stmt.IsReplace, IsInsertSelect: stmt.Select != nil, HasOnDuplicate: len(stmt.OnDuplicate) > 0, HasSubquery: nodeHasSubquery(stmt), HasJoin: joinExists(join), HasJoinOn: joinHasOn(join)}
}

func extractUpdate(stmt *ast.UpdateStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractMutationTables(join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{Operation: spec.DMLOperationUpdate, Tables: tables, HasWhere: stmt.Where != nil, HasLimit: stmt.Limit != nil, HasOrderBy: stmt.Order != nil, HasSubquery: hasSubquery, HasJoin: joinExists(join), HasJoinOn: joinHasOn(join), PredicateShape: shape, LookupColumns: lookupColumns, MatchedKeyName: matchedKeyName, MatchedKeyKind: matchedKeyKind, IsSingleTable: isSingleTable}
}

func extractDelete(stmt *ast.DeleteStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractMutationTables(join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{Operation: spec.DMLOperationDelete, Tables: tables, HasWhere: stmt.Where != nil, HasLimit: stmt.Limit != nil, HasOrderBy: stmt.Order != nil, HasSubquery: hasSubquery, HasJoin: joinExists(join), HasJoinOn: joinHasOn(join), PredicateShape: shape, LookupColumns: lookupColumns, MatchedKeyName: matchedKeyName, MatchedKeyKind: matchedKeyKind, IsSingleTable: isSingleTable}
}

func extractColumn(col *ast.ColumnDef) spec.Column {
	column := spec.Column{Name: col.Name.Name.L, Type: strings.ToLower(col.Tp.String()), Length: col.Tp.GetFlen(), Unsigned: mysql.HasUnsignedFlag(col.Tp.GetFlag())}
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
		return &spec.AlterColumn{OldName: specification.OldColumnName.Name.L}
	case ast.AlterTableRenameColumn:
		if specification.OldColumnName == nil || specification.NewColumnName == nil {
			return nil
		}
		return &spec.AlterColumn{OldName: specification.OldColumnName.Name.L, Definition: &spec.Column{Name: specification.NewColumnName.Name.L}}
	default:
		return nil
	}
}

func extractCreateIndex(stmt *ast.CreateIndexStmt) *spec.DDL {
	kind := spec.IndexKindSecondary
	if stmt.KeyType == ast.IndexKeyTypeUnique {
		kind = spec.IndexKindUnique
	}
	if stmt.KeyType == ast.IndexKeyTypeFulltext {
		kind = spec.IndexKindFulltext
	}
	indexName := stmt.IndexName
	columns := extractIndexColumns(stmt.IndexPartSpecifications)
	return &spec.DDL{
		Operation: spec.DDLOperationCreateIndex,
		Table:     &spec.Table{Name: stmt.Table.Name.L, Schema: stmt.Table.Schema.L},
		Alter: []spec.Alter{{
			Action: "create_index",
			Index:  &spec.AlterIndex{Definition: &spec.Index{Name: indexName, Kind: kind, Columns: columns}},
		}},
	}
}

func extractDropIndex(stmt *ast.DropIndexStmt) *spec.DDL {
	return &spec.DDL{
		Operation: spec.DDLOperationDropIndex,
		Table:     &spec.Table{Name: stmt.Table.Name.L, Schema: stmt.Table.Schema.L},
		Alter: []spec.Alter{{
			Action: "drop_index",
			Index:  &spec.AlterIndex{OldName: stmt.IndexName},
		}},
	}
}

func extractRenameTable(stmt *ast.RenameTableStmt) *spec.DDL {
	if len(stmt.TableToTables) == 0 {
		return &spec.DDL{Operation: spec.DDLOperationRenameTable}
	}
	first := stmt.TableToTables[0]
	alters := make([]spec.Alter, 0, len(stmt.TableToTables))
	for _, tt := range stmt.TableToTables {
		a := spec.Alter{Action: "rename_table", Options: map[string]string{}}
		if tt.OldTable != nil {
			a.Options["old_table"] = tt.OldTable.Name.L
			if tt.OldTable.Schema.L != "" {
				a.Options["old_schema"] = tt.OldTable.Schema.L
			}
		}
		if tt.NewTable != nil {
			a.Options["new_table"] = tt.NewTable.Name.L
			if tt.NewTable.Schema.L != "" {
				a.Options["new_schema"] = tt.NewTable.Schema.L
			}
		}
		alters = append(alters, a)
	}
	table := &spec.Table{}
	if first.OldTable != nil {
		table = &spec.Table{Name: first.OldTable.Name.L, Schema: first.OldTable.Schema.L}
	}
	return &spec.DDL{
		Operation: spec.DDLOperationRenameTable,
		Table:     table,
		Alter:     alters,
	}
}

func extractAlterDatabase(stmt *ast.AlterDatabaseStmt) *spec.DDL {
	options := map[string]string{}
	for _, opt := range stmt.Options {
		if opt == nil {
			continue
		}
		switch opt.Tp {
		case ast.DatabaseOptionCharset:
			options["charset"] = opt.Value
		case ast.DatabaseOptionCollate:
			options["collate"] = opt.Value
		}
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationAlterSchema,
		ObjectName: stmt.Name.L,
		ObjectType: "database",
		Options:    options,
	}
}

func extractCreateProcedure(stmt *ast.ProcedureInfo) *spec.DDL {
	name := ""
	if stmt.ProcedureName != nil {
		name = stmt.ProcedureName.Name.L
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationCreateProcedure,
		ObjectName: name,
		ObjectType: "procedure",
		Options:    map[string]string{"has_body": "true"},
	}
}

func extractDropProcedure(stmt *ast.DropProcedureStmt) *spec.DDL {
	name := ""
	if stmt.ProcedureName != nil {
		name = stmt.ProcedureName.Name.L
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationDropProcedure,
		ObjectName: name,
		ObjectType: "procedure",
	}
}

func extractCreateUser(stmt *ast.CreateUserStmt) *spec.DDL {
	if stmt.IsCreateRole {
		name := ""
		if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
			name = stmt.Specs[0].User.Username
		}
		return &spec.DDL{
			Operation:  spec.DDLOperationCreateRole,
			ObjectName: name,
			ObjectType: "role",
		}
	}
	name := ""
	if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
		name = stmt.Specs[0].User.Username
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationCreateUser,
		ObjectName: name,
		ObjectType: "user",
		Options:    map[string]string{"has_auth": "true"},
	}
}

func extractAlterUser(stmt *ast.AlterUserStmt) *spec.DDL {
	name := ""
	if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
		name = stmt.Specs[0].User.Username
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationAlterUser,
		ObjectName: name,
		ObjectType: "user",
		Options:    map[string]string{"has_auth": "true"},
	}
}

func extractDropUser(stmt *ast.DropUserStmt) *spec.DDL {
	if stmt.IsDropRole {
		names := make([]string, 0, len(stmt.UserList))
		for _, u := range stmt.UserList {
			if u != nil {
				names = append(names, u.Username)
			}
		}
		objectName := ""
		if len(names) > 0 {
			objectName = names[0]
		}
		return &spec.DDL{
			Operation:  spec.DDLOperationDropRole,
			ObjectName: objectName,
			ObjectType: "role",
		}
	}
	names := make([]string, 0, len(stmt.UserList))
	for _, u := range stmt.UserList {
		if u != nil {
			names = append(names, u.Username)
		}
	}
	objectName := ""
	if len(names) > 0 {
		objectName = names[0]
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationDropUser,
		ObjectName: objectName,
		ObjectType: "user",
	}
}

func extractGrant(stmt *ast.GrantStmt) *spec.DDL {
	options := map[string]string{}
	if len(stmt.Privs) > 0 {
		privNames := make([]string, 0, len(stmt.Privs))
		for _, p := range stmt.Privs {
			if p != nil {
				privNames = append(privNames, strings.ToLower(p.Priv.String()))
			}
		}
		if len(privNames) > 0 {
			options["privilege"] = strings.Join(privNames, ",")
		}
	}
	if stmt.Level != nil {
		switch {
		case stmt.Level.DBName != "":
			options["object_type"] = "database"
		case stmt.Level.TableName != "":
			options["object_type"] = "table"
		default:
			options["object_type"] = "global"
		}
	}
	return &spec.DDL{
		Operation: spec.DDLOperationGrant,
		Options:   options,
	}
}

func extractRevoke(stmt *ast.RevokeStmt) *spec.DDL {
	options := map[string]string{}
	if len(stmt.Privs) > 0 {
		privNames := make([]string, 0, len(stmt.Privs))
		for _, p := range stmt.Privs {
			if p != nil {
				privNames = append(privNames, strings.ToLower(p.Priv.String()))
			}
		}
		if len(privNames) > 0 {
			options["privilege"] = strings.Join(privNames, ",")
		}
	}
	if stmt.Level != nil {
		switch {
		case stmt.Level.DBName != "":
			options["object_type"] = "database"
		case stmt.Level.TableName != "":
			options["object_type"] = "table"
		default:
			options["object_type"] = "global"
		}
	}
	return &spec.DDL{
		Operation: spec.DDLOperationRevoke,
		Options:   options,
	}
}

func extractDropResourceGroup(stmt *ast.DropResourceGroupStmt) *spec.DDL {
	return &spec.DDL{
		Operation:  spec.DDLOperationDropResourceGroup,
		ObjectName: stmt.ResourceGroupName.L,
		ObjectType: "resource_group",
	}
}

func extractCreatePlacementPolicy(stmt *ast.CreatePlacementPolicyStmt) *spec.DDL {
	return &spec.DDL{
		Operation:  spec.DDLOperationCreatePlacementPolicy,
		ObjectName: stmt.PolicyName.L,
		ObjectType: "placement_policy",
	}
}

func extractAlterPlacementPolicy(stmt *ast.AlterPlacementPolicyStmt) *spec.DDL {
	return &spec.DDL{
		Operation:  spec.DDLOperationAlterPlacementPolicy,
		ObjectName: stmt.PolicyName.L,
		ObjectType: "placement_policy",
	}
}

func extractDropPlacementPolicy(stmt *ast.DropPlacementPolicyStmt) *spec.DDL {
	return &spec.DDL{
		Operation:  spec.DDLOperationDropPlacementPolicy,
		ObjectName: stmt.PolicyName.L,
		ObjectType: "placement_policy",
	}
}

func extractCreateSequence(stmt *ast.CreateSequenceStmt) *spec.DDL {
	name := ""
	if stmt.Name != nil {
		name = stmt.Name.Name.L
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationCreateSequence,
		ObjectName: name,
		ObjectType: "sequence",
		Options:    map[string]string{"has_options": "true"},
	}
}

func extractAlterSequence(stmt *ast.AlterSequenceStmt) *spec.DDL {
	name := ""
	if stmt.Name != nil {
		name = stmt.Name.Name.L
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationAlterSequence,
		ObjectName: name,
		ObjectType: "sequence",
		Options:    map[string]string{"has_options": "true"},
	}
}

func extractDropSequence(stmt *ast.DropSequenceStmt) *spec.DDL {
	name := ""
	if len(stmt.Sequences) > 0 && stmt.Sequences[0] != nil {
		name = stmt.Sequences[0].Name.L
	}
	return &spec.DDL{
		Operation:  spec.DDLOperationDropSequence,
		ObjectName: name,
		ObjectType: "sequence",
	}
}

func alterColumnFromColumnDef(col *ast.ColumnDef) *spec.AlterColumn {
	extracted := extractColumn(col)
	return &spec.AlterColumn{Definition: &extracted}
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
	if !change.TouchesNullability && !change.TouchesDefault && !change.TouchesAutoIncrement {
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
		return &spec.AlterIndex{Definition: &spec.Index{Kind: indexKindForConstraint(specification.Constraint.Tp), Name: normalizeConstraintName(specification.Constraint), Columns: extractIndexColumns(specification.Constraint.Keys)}}
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
		return &spec.AlterIndex{OldName: specification.FromKey.L, Definition: &spec.Index{Name: specification.ToKey.L}}
	case ast.AlterTableDropPrimaryKey:
		return &spec.AlterIndex{OldName: "primary"}
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
		case ast.TableOptionPlacementPolicy:
			if option.StrValue != "" {
				extracted["placement_policy"] = option.StrValue
			}
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
	return (exprIsIDColumnRef(left) && exprIsLiteralValue(right)) || (exprIsIDColumnRef(right) && exprIsLiteralValue(left))
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
	case ast.AlterTableDropForeignKey:
		return "drop_foreign_key"
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
