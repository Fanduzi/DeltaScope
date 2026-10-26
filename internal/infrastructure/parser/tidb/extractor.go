// Package tidbparser extracts parser-neutral statements from TiDB AST nodes.
package tidbparser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
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
	// supplements carries adapter-owned typed facts the locked grammar
	// cannot represent (column visibility attributes, the visibility
	// setter). nil for un-supplemented statements.
	supplements *statementSupplements
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
		statement.DDL = extractCreateTable(node, rawSQL)
	case *ast.CreateViewStmt:
		statement.DDL = extractCreateView(node)
	case *ast.AlterTableStmt:
		statement.DDL = extractAlterTable(node, rawSQL)
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
	e.applySupplements(&statement)
	applyCoverageBoundary(dialect, &statement, e.node)
	return statement, nil
}

func WrapStatements(stmts []ast.StmtNode, warnings []string) []ExtractedStatement {
	wrapped := make([]ExtractedStatement, 0, len(stmts))
	for _, stmt := range stmts {
		node := stmt
		var supplements *statementSupplements
		if carried, ok := stmt.(*supplementedStatement); ok {
			// Classification and extraction always see the real underlying
			// node; the typed supplements ride beside it.
			node = carried.StmtNode
			supplements = carried.supp
		}
		kind := classify(node)
		wrapped = append(wrapped, ExtractedStatement{
			Kind:      kind,
			RawSQL:    node.Text(),
			Extractor: tidbExtractor{kind: kind, warnings: warnings, node: node, supplements: supplements},
		})
	}
	return wrapped
}

// applySupplements binds adapter-owned typed facts onto the domain statement:
// declared column visibility attributes and the narrow visibility setter's
// action. Facts were verified against the parsed node at bind time, so each
// named column is guaranteed present.
func (e tidbExtractor) applySupplements(statement *spec.Statement) {
	supp := e.supplements
	if supp == nil || statement.DDL == nil {
		return
	}
	ddl := statement.DDL
	switch ddl.Operation {
	case spec.DDLOperationCreateTable:
		for i := range ddl.Columns {
			if visibility, ok := supp.visibility[ddl.Columns[i].Name]; ok {
				ddl.Columns[i].Visibility = visibility
				ddl.Columns[i].VisibilityExplicit = true
			}
			if supp.defaultParens[ddl.Columns[i].Name] {
				ddl.Columns[i].DefaultParenthesized = true
			}
		}
	case spec.DDLOperationAlterTable:
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if supp.setterVisibility != "" && alter.Column != nil {
				// The setter's underlying node is the real AlterColumn
				// identity; swap the default-placeholder action for the
				// typed visibility fact without inventing option facts.
				alter.Action = "set_column_visibility"
				alter.Column.Change = nil
				alter.Column.Definition.Visibility = supp.setterVisibility
				alter.Column.Definition.VisibilityExplicit = true
				continue
			}
			if alter.Column == nil || alter.Column.Definition == nil {
				continue
			}
			if supp.defaultParens[alter.Column.Definition.Name] {
				alter.Column.Definition.DefaultParenthesized = true
			}
			switch alter.Action {
			case "add_columns", "modify_column", "change_column":
				if visibility, ok := supp.visibility[alter.Column.Definition.Name]; ok {
					alter.Column.Definition.Visibility = visibility
					alter.Column.Definition.VisibilityExplicit = true
				}
			}
		}
	}
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

func extractCreateTable(stmt *ast.CreateTableStmt, rawSQL string) *spec.DDL {
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
	if stmt.IfNotExists {
		ddl.Options["if_not_exists"] = "true"
	}
	switch stmt.TemporaryKeyword {
	case ast.TemporaryLocal:
		ddl.TemporaryScope = spec.TemporaryScopeLocal
	case ast.TemporaryGlobal:
		ddl.TemporaryScope = spec.TemporaryScopeGlobal
		ddl.OnCommitDelete = stmt.OnCommitDelete
	}
	// The AST folds every CHECK declaration's optional [NOT] ENFORCED
	// clause into a bare bool; the element text is the only place the
	// authored clause is still readable. Correlate each comma-split
	// element with the matching Cols/Constraints entry in authored order.
	constraintExplicit, columnExplicit := createCheckExplicitFlags(stmt, rawSQL)

	for _, col := range stmt.Cols {
		column := extractColumn(col)
		ddl.Columns = append(ddl.Columns, column)
		for _, option := range col.Options {
			if option != nil && option.Tp == ast.ColumnOptionPrimaryKey {
				pk := &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{column.Name}}
				if option.PrimaryKeyTp != ast.PrimaryKeyTypeDefault {
					pk.PrimaryKeyType = strings.ToLower(option.PrimaryKeyTp.String())
				}
				pk.SetKeyParts([]spec.IndexKeyPart{{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: column.Name}})
				ddl.PrimaryKey = pk
				break
			}
		}
	}

	for i, c := range stmt.Constraints {
		switch c.Tp {
		case ast.ConstraintPrimaryKey:
			global, hasPredicate, visibility, primaryKeyType, unmodeled := indexOptionFacts(c.Option)
			pk := &spec.Index{Name: normalizeConstraintName(c), Kind: spec.IndexKindPrimary, Columns: extractIndexColumns(c.Keys), Visibility: visibility, VisibilityExplicit: visibility != "", Global: global, HasPredicate: hasPredicate, PrimaryKeyType: primaryKeyType, UnmodeledOptions: unmodeled}
			pk.SetKeyParts(extractIndexKeyParts(c.Keys))
			ddl.PrimaryKey = pk
		case ast.ConstraintKey, ast.ConstraintIndex, ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex, ast.ConstraintFulltext:
			global, hasPredicate, visibility, primaryKeyType, unmodeled := indexOptionFacts(c.Option)
			if primaryKeyType != "" {
				// CLUSTERED|NONCLUSTERED is a primary-key-only option; on any
				// other index kind it stays a bounded unmodeled member.
				unmodeled = append(unmodeled, "primary_key_type")
			}
			index := spec.Index{Name: normalizeConstraintName(c), Kind: indexKindForConstraint(c.Tp), Columns: extractIndexColumns(c.Keys), Visibility: visibility, VisibilityExplicit: visibility != "", Global: global, HasPredicate: hasPredicate, UnmodeledOptions: unmodeled}
			index.SetKeyParts(extractIndexKeyParts(c.Keys))
			ddl.Indexes = append(ddl.Indexes, index)
		default:
			member := extractConstraint(c)
			if c.Tp == ast.ConstraintCheck && constraintExplicit[i] {
				member.EnforcedExplicit = true
			}
			ddl.Constraints = append(ddl.Constraints, member)
		}
	}
	// Column-option CHECK(...) declarations land as table-level CHECK
	// members in the catalog — model them as constraint members with the
	// InColumn binding so ordered state and rules see the same object the
	// database stores.
	for colIdx, col := range stmt.Cols {
		if col == nil || col.Name == nil {
			continue
		}
		for _, option := range col.Options {
			if option == nil || option.Tp != ast.ColumnOptionCheck {
				continue
			}
			check := checkConstraintFromColumnOption(option, col.Name.Name.L)
			check.EnforcedExplicit = columnExplicit[colIdx]
			ddl.Constraints = append(ddl.Constraints, check)
		}
	}

	normalizeCreatePrimaryKeyNullability(ddl, stmt.Cols)

	extracted, unextracted := extractTableOptions(stmt.Options)
	for key, value := range extracted {
		ddl.Options[key] = value
		if key == "comment" {
			ddl.Table.Comment = value
		}
	}
	ddl.UnextractedOptions = append(unextracted, partitionOptionNames(stmt.Partition)...)
	if stmt.OnDuplicate != ast.OnDuplicateKeyHandlingError {
		// CREATE TABLE ... IGNORE|REPLACE SELECT carries duplicate-key
		// handling semantics the model does not project.
		ddl.UnextractedOptions = append(ddl.UnextractedOptions, "on_duplicate")
	}
	if len(stmt.SplitIndex) > 0 {
		// SPLIT PRIMARY KEY BETWEEN ... REGIONS is a separate collection beside
		// the option lists; record bounded presence so the clause cannot pass
		// silently. Region bounds and split semantics stay unmodeled.
		ddl.UnextractedOptions = append(ddl.UnextractedOptions, "split_index")
	}

	return ddl
}

// normalizeCreatePrimaryKeyNullability writes the database-implied NOT NULL
// fact onto primary-key members that bind to a declared column. MySQL and
// TiDB mark primary-key members NOT NULL regardless of the written
// nullability clause, so table-level members get the same fact the inline
// PRIMARY KEY option already records in extractColumn — single or composite,
// with key order independent of column order. An explicit NULL declaration
// keeps the conflict visible: the member stays NotNull=false so the existing
// primary-key-not-null rule can report the declaration conflict instead of
// the model silently pretending a legal table. Members that do not bind to a
// declared column (expression parts, unknown names) are left untouched.
func normalizeCreatePrimaryKeyNullability(ddl *spec.DDL, cols []*ast.ColumnDef) {
	if ddl.PrimaryKey == nil {
		return
	}
	members := make(map[string]struct{}, len(ddl.PrimaryKey.Columns))
	for _, name := range ddl.PrimaryKey.Columns {
		members[name] = struct{}{}
	}
	explicitNull := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		if col == nil || col.Name == nil {
			continue
		}
		for _, option := range col.Options {
			if option != nil && option.Tp == ast.ColumnOptionNull {
				explicitNull[col.Name.Name.L] = struct{}{}
				break
			}
		}
	}
	for i := range ddl.Columns {
		column := &ddl.Columns[i]
		if _, ok := members[column.Name]; !ok {
			continue
		}
		if _, conflict := explicitNull[column.Name]; conflict {
			column.NotNull = false
			continue
		}
		column.NotNull = true
	}
}

func extractCreateView(stmt *ast.CreateViewStmt) *spec.DDL {
	return &spec.DDL{
		Operation:          spec.DDLOperationCreateView,
		Table:              &spec.Table{Schema: stmt.ViewName.Schema.L, Name: stmt.ViewName.Name.L},
		HasSelect:          stmt.Select != nil,
		UnextractedOptions: viewOptionNames(stmt),
	}
}

// viewOptionNames returns bounded names for parsed CREATE VIEW clauses the
// normalized model does not keep. The parser fills MySQL-compatible defaults
// even when clauses are omitted — DEFINER defaults to CURRENT_USER, SQL
// SECURITY to DEFINER, ALGORITHM to UNDEFINED, CHECK OPTION to CASCADED — so
// evidence only records forms that provably deviate from defaults. An
// explicitly written default (e.g. SQL SECURITY DEFINER) is semantically
// identical to the omitted form and stays out of the evidence set.
func viewOptionNames(stmt *ast.CreateViewStmt) []string {
	var names []string
	if stmt.OrReplace {
		names = append(names, "or_replace")
	}
	if len(stmt.Cols) > 0 || len(stmt.SchemaCols) > 0 {
		names = append(names, "view_columns")
	}
	if stmt.Algorithm != ast.AlgorithmUndefined {
		names = append(names, "view_algorithm")
	}
	if stmt.Definer != nil && !stmt.Definer.CurrentUser {
		names = append(names, "definer")
	}
	if stmt.Security != ast.SecurityDefiner {
		names = append(names, "sql_security")
	}
	if stmt.CheckOption == ast.CheckOptionLocal {
		names = append(names, "check_option")
	}
	return names
}

func extractAlterTable(stmt *ast.AlterTableStmt, rawSQL string) *spec.DDL {
	ddl := &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: stmt.Table.Schema.L, Name: stmt.Table.Name.L}, Alter: make([]spec.Alter, 0, len(stmt.Specs))}
	clauses := splitAlterTableClauses(rawSQL)
	for index, s := range stmt.Specs {
		clause := rawSQL
		if len(clauses) == len(stmt.Specs) {
			clause = clauses[index]
		}
		ddl.Alter = append(ddl.Alter, extractAlterSpecs(s, clause)...)
		if s.Tp == ast.AlterTableRenameTable && s.NewTable != nil {
			// The destination keeps its as-written qualifier: an unqualified
			// RENAME TO target resolves to the current schema downstream, not
			// the altered table's schema.
			ddl.Targets = append(ddl.Targets, spec.Table{Schema: s.NewTable.Schema.L, Name: s.NewTable.Name.L})
		}
	}
	if len(ddl.Targets) > 0 {
		ddl.Targets = append([]spec.Table{*ddl.Table}, ddl.Targets...)
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
	switch stmt.TemporaryKeyword {
	case ast.TemporaryLocal:
		ddl.TemporaryScope = spec.TemporaryScopeLocal
	case ast.TemporaryGlobal:
		ddl.TemporaryScope = spec.TemporaryScopeGlobal
	}
	for _, tableName := range stmt.Tables {
		if tableName == nil {
			continue
		}
		ddl.Targets = append(ddl.Targets, spec.Table{Schema: tableName.Schema.L, Name: tableName.Name.L})
	}
	if len(ddl.Targets) > 0 {
		table := ddl.Targets[0]
		ddl.Table = &table
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
	var unextracted []string
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
		default:
			unextracted = append(unextracted, databaseOptionName(opt.Tp))
		}
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationCreateSchema,
		ObjectName:         stmt.Name.L,
		ObjectType:         "database",
		Options:            options,
		UnextractedOptions: unextracted,
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

func extractAlterSpecs(specification *ast.AlterTableSpec, clause string) []spec.Alter {
	if specification.Tp == ast.AlterTableAddColumns {
		alters := make([]spec.Alter, 0, len(specification.NewColumns)+len(specification.NewConstraints))
		// A position clause on a parenthesized multi-column list applies to
		// the whole spec, not to one column — no per-column position can be
		// attributed, so the clause is recorded as an unextracted option
		// instead of duplicating it onto every emitted action.
		specLevelPosition := len(specification.NewColumns) > 1 && hasColumnPositionClause(specification)
		for _, column := range specification.NewColumns {
			if column == nil {
				continue
			}
			alter := spec.Alter{Action: alterActionName(specification.Tp), Name: column.Name.Name.L, Column: alterColumnFromColumnDef(column)}
			for _, option := range column.Options {
				if option == nil || option.Tp != ast.ColumnOptionCheck {
					continue
				}
				check := checkConstraintFromColumnOption(option, column.Name.Name.L)
				check.EnforcedExplicit = clauseDeclaresEnforced(clause)
				alter.Constraints = append(alter.Constraints, check)
			}
			if specLevelPosition {
				if len(alters) == 0 {
					alter.UnextractedOptions = append(alter.UnextractedOptions, "column_position")
				}
			} else {
				alter.HasColumnPosition = hasColumnPositionClause(specification)
				alter.Position = extractAlterPosition(specification)
			}
			markAlterExistenceClauses(specification, &alter)
			alters = append(alters, alter)
		}
		// Constraints inside ADD (..., <constraint>) carry the same payload as
		// standalone ADD <constraint> clauses. The AST does not record a
		// CONSTRAINT keyword inside the list, so index kinds take the bare-form
		// action (add_index) and PK/FK/CHECK take add_constraint.
		for _, constraint := range specification.NewConstraints {
			if constraint == nil {
				continue
			}
			// Pass an empty clause: alterActionNameForSpec derives the action
			// from constraint type only. The parent clause text (e.g. a column
			// comment containing "ADD CONSTRAINT") must not leak into a
			// sibling constraint's action name. The parenthesized element
			// text is not split, so an authored ENFORCED clause anywhere in
			// the shared clause marks the check member — sound at clause
			// scope because the whole statement refuses anyway where the
			// fact decides (MySQL <8.0.16).
			added := extractAlterSpec(&ast.AlterTableSpec{Tp: ast.AlterTableAddConstraint, Constraint: constraint}, "")
			if added.Constraint != nil && added.Constraint.Type == "check" {
				added.Constraint.EnforcedExplicit = clauseDeclaresEnforced(clause)
			}
			alters = append(alters, added)
		}
		if len(alters) > 0 {
			return alters
		}
	}
	return []spec.Alter{extractAlterSpec(specification, clause)}
}

func hasColumnPositionClause(specification *ast.AlterTableSpec) bool {
	return specification.Position != nil && specification.Position.Tp != ast.ColumnPositionNone
}

func extractAlterPosition(specification *ast.AlterTableSpec) *spec.AlterPosition {
	if !hasColumnPositionClause(specification) {
		return nil
	}
	position := &spec.AlterPosition{Kind: "first"}
	if specification.Position.Tp == ast.ColumnPositionAfter {
		position.Kind = "after"
		if specification.Position.RelativeColumn != nil {
			position.After = specification.Position.RelativeColumn.Name.L
		}
	}
	return position
}

func extractAlterSpec(specification *ast.AlterTableSpec, clause string) spec.Alter {
	alter := spec.Alter{Action: alterActionNameForSpec(specification, clause), Name: extractAlterName(specification), HasColumnPosition: hasColumnPositionClause(specification), Position: extractAlterPosition(specification)}
	if column := extractAlterColumn(specification); column != nil {
		alter.Column = column
	}
	if index := extractAlterIndex(specification); index != nil {
		alter.Index = index
	}
	options, unextracted := extractTableOptions(specification.Options)
	if len(options) > 0 {
		alter.Options = options
		if _, ok := options["placement_policy"]; ok && alter.Action == "table_option" {
			alter.Action = "placement_policy"
		}
	}
	if len(unextracted) > 0 {
		alter.UnextractedOptions = unextracted
	}
	if specification.Constraint != nil {
		constraint := extractConstraint(specification.Constraint)
		if constraintProducesIndex(specification.Constraint.Tp) {
			// Index-producing constraints carry their key-part facts on
			// alter.Index.Definition; keeping them here too would emit the
			// same aspect twice under constraint and index feature names.
			constraint.UnmodeledParts = 0
			constraint.UnmodeledReferencedParts = 0
			constraint.UnmodeledReferActions = 0
		}
		alter.Constraint = &constraint
		if constraint.Type == "check" {
			alter.Constraint.EnforcedExplicit = clauseDeclaresEnforced(clause)
		}
	}
	if specification.Tp == ast.AlterTableAlterCheck || specification.Tp == ast.AlterTableDropCheck {
		// The PingCAP AST collapses both authored keywords into one node;
		// the clause text of this spec is the only place the spelling is
		// still readable, and an unreadable clause stays unproven.
		if alter.Options == nil {
			alter.Options = map[string]string{}
		}
		if spelling := checkSpellingForClause(clause); spelling != "" {
			alter.Options["check_spelling"] = spelling
		}
	}
	if specification.Tp == ast.AlterTableAlterCheck && specification.Constraint != nil {
		// ALTER CHECK|CONSTRAINT x [NOT] ENFORCED carries the enforcement
		// direction on the constraint payload — the only place the
		// normalized model can read it, since both directions share the
		// same action name.
		if specification.Constraint.Enforced {
			alter.Options["check_enforced"] = "true"
		} else {
			alter.Options["check_enforced"] = "false"
		}
	}
	for _, column := range specification.NewColumns {
		if column == nil {
			continue
		}
		for _, option := range column.Options {
			if option == nil || option.Tp != ast.ColumnOptionCheck {
				continue
			}
			check := checkConstraintFromColumnOption(option, column.Name.Name.L)
			check.EnforcedExplicit = clauseDeclaresEnforced(clause)
			alter.Constraints = append(alter.Constraints, check)
		}
	}
	markAlterExistenceClauses(specification, &alter)
	return alter
}

// checkEnforcedName normalizes the AST enforcement bool to the shared
// tri-state vocabulary; the grammar cannot distinguish an undeclared
// keyword from ENFORCED (both set true) — a distinction with no native
// difference.
func checkEnforcedName(enforced bool) string {
	if enforced {
		return "enforced"
	}
	return "not_enforced"
}

// checkConstraintFromColumnOption models a column-option CHECK(...) as the
// constraint member the catalog creates: Type=check, InColumn bound to the
// owning column, expression facts and declared enforcement carried; the
// authored CONSTRAINT symbol names it when present. Raw expression text is
// never retained.
func checkConstraintFromColumnOption(option *ast.ColumnOption, columnName string) spec.Constraint {
	check := spec.Constraint{
		Type:     "check",
		Name:     strings.TrimSpace(option.ConstraintName),
		Columns:  []string{columnName},
		InColumn: true,
		Enforced: checkEnforcedName(option.Enforced),
	}
	if option.Expr != nil {
		facts := expressionFacts(option.Expr)
		check.Expression = &facts
		seen := map[string]bool{strings.ToLower(columnName): true}
		for _, ref := range facts.LocalReferences() {
			if seen[strings.ToLower(ref.Name)] {
				continue
			}
			seen[strings.ToLower(ref.Name)] = true
			check.Columns = append(check.Columns, ref.Name)
		}
	}
	return check
}

// markAlterExistenceClauses records parsed IF EXISTS / IF NOT EXISTS clauses
// on an alter spec. They are extracted facts the ordered state pass may
// consume; they are not unmodeled options and never produce aspect gaps.
func markAlterExistenceClauses(specification *ast.AlterTableSpec, alter *spec.Alter) {
	if specification.IfNotExists {
		if alter.Options == nil {
			alter.Options = map[string]string{}
		}
		alter.Options["if_not_exists"] = "true"
	}
	if specification.IfExists {
		if alter.Options == nil {
			alter.Options = map[string]string{}
		}
		alter.Options["if_exists"] = "true"
	}
}

func alterActionNameForSpec(specification *ast.AlterTableSpec, clause string) string {
	if specification.Tp == ast.AlterTableAlterColumn {
		// ALTER [COLUMN] name SET|DROP DEFAULT is the MySQL/TiDB
		// set_column_default/drop_column_default action space — deliberately
		// not the PostgreSQL set_default/drop_default names, whose forbid
		// rules must stay inert here.
		if len(specification.NewColumns) > 0 && specification.NewColumns[0] != nil {
			for _, option := range specification.NewColumns[0].Options {
				if option != nil && option.Expr != nil {
					return "set_column_default"
				}
			}
		}
		return "drop_column_default"
	}
	action := alterActionName(specification.Tp)
	if action != "add_constraint" || specification.Constraint == nil || !constraintIsIndexAddition(specification.Constraint.Tp) {
		return action
	}

	normalizedClause := strings.Join(strings.Fields(strings.ToUpper(clause)), " ")
	if strings.Contains(normalizedClause, "ADD CONSTRAINT") {
		return action
	}
	return "add_index"
}

func constraintIsIndexAddition(tp ast.ConstraintType) bool {
	switch tp {
	case ast.ConstraintKey, ast.ConstraintIndex, ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex, ast.ConstraintFulltext:
		return true
	default:
		return false
	}
}

func splitAlterTableClauses(sql string) []string {
	clauses := make([]string, 0, 1)
	start := 0
	depth := 0
	var quote byte

	for i := 0; i < len(sql); i++ {
		char := sql[i]
		if quote != 0 {
			if char == '\\' && quote != '`' {
				i++
				continue
			}
			if char == quote {
				if quote != '`' && i+1 < len(sql) && sql[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch char {
		case '\'', '"', '`':
			quote = char
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if clause := strings.TrimSpace(sql[start:i]); clause != "" {
					clauses = append(clauses, clause)
				}
				start = i + 1
			}
		}
	}

	if clause := strings.TrimSpace(sql[start:]); clause != "" {
		clauses = append(clauses, clause)
	}
	return clauses
}

func extractInsert(stmt *ast.InsertStmt) *spec.DML {
	join := tableRefsJoin(stmt.Table)
	tables := extractMutationTables(join)
	if len(tables) == 0 && stmt.Table != nil && stmt.Table.TableRefs != nil {
		tables = extractMutationTables(stmt.Table.TableRefs)
	}
	return &spec.DML{Operation: spec.DMLOperationInsert, Tables: tables, MutationTargets: tables, InsertRows: len(stmt.Lists), IsReplace: stmt.IsReplace, IsInsertSelect: stmt.Select != nil, HasOnDuplicate: len(stmt.OnDuplicate) > 0, HasReturning: len(stmt.Returning) > 0, HasSubquery: nodeHasSubquery(stmt), HasJoin: joinExists(join), HasJoinOn: joinHasOn(join)}
}

func extractUpdate(stmt *ast.UpdateStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractUpdateMutationTables(stmt, join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{Operation: spec.DMLOperationUpdate, Tables: tables, MutationTargets: tables, HasWhere: stmt.Where != nil, HasLimit: stmt.Limit != nil, HasOrderBy: stmt.Order != nil, HasSubquery: hasSubquery, HasJoin: joinExists(join), HasJoinOn: joinHasOn(join), HasReturning: len(stmt.Returning) > 0, PredicateShape: shape, LookupColumns: lookupColumns, MatchedKeyName: matchedKeyName, MatchedKeyKind: matchedKeyKind, IsSingleTable: isSingleTable}
}

func extractDelete(stmt *ast.DeleteStmt) *spec.DML {
	join := tableRefsJoin(stmt.TableRefs)
	tables := extractDeleteMutationTables(stmt, join)
	hasSubquery := nodeHasSubquery(stmt)
	isSingleTable := len(tables) == 1 && !joinExists(join)
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(stmt.Where, join, isSingleTable)
	return &spec.DML{Operation: spec.DMLOperationDelete, Tables: tables, MutationTargets: tables, HasWhere: stmt.Where != nil, HasLimit: stmt.Limit != nil, HasOrderBy: stmt.Order != nil, HasSubquery: hasSubquery, HasJoin: joinExists(join), HasJoinOn: joinHasOn(join), HasReturning: len(stmt.Returning) > 0, PredicateShape: shape, LookupColumns: lookupColumns, MatchedKeyName: matchedKeyName, MatchedKeyKind: matchedKeyKind, IsSingleTable: isSingleTable}
}

// declaredTypeRestoreFlags renders the five declared-parameter types with a
// lowercase keyword, single-quoted re-escaped members, and sentinel-faithful
// precision/scale — never the formatted layer's substituted defaults.
const declaredTypeRestoreFlags = format.RestoreStringSingleQuotes |
	format.RestoreStringEscapeBackslash |
	format.RestoreKeyWordLowercase |
	format.RestoreNameBackQuotes

// declaredColumnType renders column types for spec.Column.Type. DECIMAL,
// FLOAT, DOUBLE, ENUM, and SET go through FieldType.Restore: it writes the
// declared (flen[,decimal]) span from the sentinels — preserving DECIMAL(12)
// vs DECIMAL(12,0) vs bare DECIMAL and the explicit (M,0) scale String()
// elides — and writes ENUM/SET members byte-exact so keyword normalization
// can never fold member case or content. Every other type keeps the existing
// lowercase String() text (int(11), bigint(20) unsigned, charset facts).
func declaredColumnType(tp *tidbtypes.FieldType) string {
	switch tp.GetType() {
	case mysql.TypeNewDecimal, mysql.TypeFloat, mysql.TypeDouble,
		mysql.TypeEnum, mysql.TypeSet:
		var text strings.Builder
		if err := tp.Restore(format.NewRestoreCtx(declaredTypeRestoreFlags, &text)); err == nil {
			return text.String()
		}
	}
	return strings.ToLower(tp.String())
}

func extractColumn(col *ast.ColumnDef) spec.Column {
	column := spec.Column{Name: col.Name.Name.L, Type: declaredColumnType(col.Tp), Length: col.Tp.GetFlen(), Unsigned: mysql.HasUnsignedFlag(col.Tp.GetFlag())}
	// A complete column declaration that omits VISIBLE/INVISIBLE is
	// effective-visible; an explicit attribute only arrives through the
	// supplement, which also sets VisibilityExplicit.
	column.Visibility = spec.ColumnVisibilityVisible
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
			column.Comment = decodedStringValue(option.Expr)
		case ast.ColumnOptionNotNull:
			column.NotNull = true
		case ast.ColumnOptionPrimaryKey:
			column.NotNull = true
			if option.StrValue == "Global" {
				column.UnextractedOptions = append(column.UnextractedOptions, "primary_key_global")
			}
			// Inline PRIMARY KEY CLUSTERED|NONCLUSTERED is modeled on the
			// primary-key index's PrimaryKeyType member, not a column gap.
		case ast.ColumnOptionAutoIncrement:
			column.AutoIncrement = true
		case ast.ColumnOptionDefaultValue:
			applyDefaultOptionFacts(&column, option.Expr)
		case ast.ColumnOptionGenerated:
			// The generated attribute is a modeled T07 fact, not an
			// unextracted option. Omitted storage means effective VIRTUAL —
			// the AST never proves the VIRTUAL keyword was written.
			storage := spec.GeneratedStorageVirtual
			if option.Stored {
				storage = spec.GeneratedStorageStored
			}
			column.Generated = &spec.GeneratedColumn{Storage: storage}
			if option.Expr != nil {
				column.Generated.Expression = expressionFacts(option.Expr)
			}
		case ast.ColumnOptionOnUpdate:
			column.OnUpdateCurrentTimestamp = exprIsCurrentTimestamp(option.Expr)
		case ast.ColumnOptionAutoRandom:
			column.AutoRandom = true
		case ast.ColumnOptionUniqKey:
			// The TiDB-only GLOBAL qualifier (StrValue=="Global") changes the
			// index class, so it gets its own bounded option name.
			name := "unique"
			if option.StrValue == "Global" {
				name = "unique_global"
			}
			column.UnextractedOptions = append(column.UnextractedOptions, name)
		case ast.ColumnOptionNull:
			// NULL is the default nullability — a no-op marker, not a gap.
		case ast.ColumnOptionCheck:
			// Modeled as an InColumn CHECK member on the constraint
			// collection — no longer an unextracted option.
		default:
			column.UnextractedOptions = append(column.UnextractedOptions, columnOptionName(option.Tp))
		}
	}
	return column
}

// columnOptionName maps a parsed ColumnOption type to a stable bounded feature
// name used in unextracted column-option evidence; unmapped future types
// synthesize a fail-closed name like tableOptionName.
func columnOptionName(tp ast.ColumnOptionType) string {
	switch tp {
	case ast.ColumnOptionFulltext:
		return "fulltext"
	case ast.ColumnOptionGenerated:
		return "generated"
	case ast.ColumnOptionReference:
		return "reference"
	case ast.ColumnOptionCheck:
		return "check"
	case ast.ColumnOptionColumnFormat:
		return "column_format"
	case ast.ColumnOptionStorage:
		return "storage"
	case ast.ColumnOptionAutoRandom:
		return "auto_random"
	case ast.ColumnOptionSecondaryEngineAttribute:
		return "secondary_engine_attribute"
	default:
		return fmt.Sprintf("column_option_%d", int(tp))
	}
}

// applyDefaultOptionFacts records one default option on the column: the
// literal/NULL/temporal fields keep their existing contract, while a
// non-literal default becomes the structural expression-default fact. The
// authored paren wrapper is preserved separately: TiDB's create-side parens
// grammar accepts only a bare call, and both products require parens for
// non-literal ALTER SET DEFAULT — presence is syntax, not value evidence.
func applyDefaultOptionFacts(column *spec.Column, expr ast.ExprNode) {
	column.HasDefault = true
	if _, ok := expr.(*ast.ParenthesesExpr); ok {
		column.DefaultParenthesized = true
	}
	column.DefaultIsNull = exprIsNullLiteral(expr)
	if column.DefaultIsNull {
		column.DefaultValue = "NULL"
	} else {
		column.DefaultValue = normalizedExprText(expr)
	}
	column.DefaultIsCurrentTimestamp = exprIsCurrentTimestamp(expr)
	column.DefaultLiteral = defaultLiteralFacts(expr)
	if !exprIsLiteralValue(expr) && !column.DefaultIsCurrentTimestamp {
		// A non-literal default is an expression default; the fact is
		// structural (never evaluated) and keeps the literal/default
		// distinction in HasDefault/DefaultValue untouched.
		facts := expressionFacts(expr)
		column.DefaultExpression = &facts
	}
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
	case ast.AlterTableAlterColumn:
		// ALTER [COLUMN] c SET|DROP DEFAULT — the parser produces a name-only
		// ColumnDef (Tp is nil, so the full extractColumn path does not apply)
		// whose single expression option carries the new default. The T07
		// contract shape is OldName + a default-only Definition.
		if len(specification.NewColumns) == 0 || specification.NewColumns[0] == nil {
			return nil
		}
		col := specification.NewColumns[0]
		alter := &spec.AlterColumn{
			OldName:    col.Name.Name.L,
			Change:     &spec.AlterColumnChange{TouchesDefault: true},
			Definition: &spec.Column{Name: col.Name.Name.L},
		}
		for _, option := range col.Options {
			if option != nil && option.Expr != nil {
				applyDefaultOptionFacts(alter.Definition, option.Expr)
			}
		}
		return alter
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
	switch stmt.KeyType {
	case ast.IndexKeyTypeUnique:
		kind = spec.IndexKindUnique
	case ast.IndexKeyTypeFulltext:
		kind = spec.IndexKindFulltext
	case ast.IndexKeyTypeSpatial:
		kind = spec.IndexKindSpatial
	case ast.IndexKeyTypeVector:
		kind = spec.IndexKindVector
	case ast.IndexKeyTypeColumnar:
		kind = spec.IndexKindColumnar
	}
	indexName := stmt.IndexName
	columns := extractIndexColumns(stmt.IndexPartSpecifications)
	indexGlobal, hasPredicate, visibility, primaryKeyType, unmodeled := indexOptionFacts(stmt.IndexOption)
	if primaryKeyType != "" {
		unmodeled = append(unmodeled, "primary_key_type")
	}
	var unextracted []string
	if stmt.LockAlg != nil {
		// CREATE INDEX ... ALGORITHM=/LOCK= is real MySQL syntax the model does
		// not carry; record bounded presence rather than the clause values.
		unextracted = append(unextracted, "lock_algorithm")
	}
	definition := &spec.Index{Name: indexName, Kind: kind, Columns: columns, Visibility: visibility, VisibilityExplicit: visibility != "", Global: indexGlobal, HasPredicate: hasPredicate, UnmodeledOptions: unmodeled}
	definition.SetKeyParts(extractIndexKeyParts(stmt.IndexPartSpecifications))
	alter := spec.Alter{
		Action: "create_index",
		Index:  &spec.AlterIndex{Definition: definition},
	}
	if stmt.IfNotExists {
		// Parsed conditional marker — the audit model records the flag and
		// gates conditional post-state on it regardless of server support.
		alter.Options = map[string]string{"if_not_exists": "true"}
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationCreateIndex,
		Table:              &spec.Table{Name: stmt.Table.Name.L, Schema: stmt.Table.Schema.L},
		UnextractedOptions: unextracted,
		Alter:              []spec.Alter{alter},
	}
}

func extractDropIndex(stmt *ast.DropIndexStmt) *spec.DDL {
	var unextracted []string
	if stmt.LockAlg != nil {
		unextracted = append(unextracted, "lock_algorithm")
	}
	if stmt.IsHypo {
		// DROP HYPO INDEX removes a TiDB hypothetical index — distinct from
		// ordinary DROP INDEX.
		unextracted = append(unextracted, "hypo_index")
	}
	alter := spec.Alter{
		Action: "drop_index",
		Index:  &spec.AlterIndex{OldName: stmt.IndexName},
	}
	if stmt.IfExists {
		alter.Options = map[string]string{"if_exists": "true"}
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationDropIndex,
		Table:              &spec.Table{Name: stmt.Table.Name.L, Schema: stmt.Table.Schema.L},
		UnextractedOptions: unextracted,
		Alter:              []spec.Alter{alter},
	}
}

func extractRenameTable(stmt *ast.RenameTableStmt) *spec.DDL {
	if len(stmt.TableToTables) == 0 {
		return &spec.DDL{Operation: spec.DDLOperationRenameTable}
	}
	first := stmt.TableToTables[0]
	alters := make([]spec.Alter, 0, len(stmt.TableToTables))
	targets := make([]spec.Table, 0, len(stmt.TableToTables)*2)
	for _, tt := range stmt.TableToTables {
		a := spec.Alter{Action: "rename_table", Options: map[string]string{}}
		if tt.OldTable != nil {
			a.Options["old_table"] = tt.OldTable.Name.L
			if tt.OldTable.Schema.L != "" {
				a.Options["old_schema"] = tt.OldTable.Schema.L
			}
			targets = append(targets, spec.Table{Schema: tt.OldTable.Schema.L, Name: tt.OldTable.Name.L})
		}
		if tt.NewTable != nil {
			a.Options["new_table"] = tt.NewTable.Name.L
			if tt.NewTable.Schema.L != "" {
				a.Options["new_schema"] = tt.NewTable.Schema.L
			}
			// The destination keeps its as-written qualifier: an unqualified
			// destination resolves to the current schema downstream, not the
			// source table's schema.
			targets = append(targets, spec.Table{Schema: tt.NewTable.Schema.L, Name: tt.NewTable.Name.L})
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
		Targets:   targets,
		Alter:     alters,
	}
}

func extractAlterDatabase(stmt *ast.AlterDatabaseStmt) *spec.DDL {
	options := map[string]string{}
	var unextracted []string
	for _, opt := range stmt.Options {
		if opt == nil {
			continue
		}
		switch opt.Tp {
		case ast.DatabaseOptionCharset:
			options["charset"] = opt.Value
		case ast.DatabaseOptionCollate:
			options["collate"] = opt.Value
		default:
			unextracted = append(unextracted, databaseOptionName(opt.Tp))
		}
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationAlterSchema,
		ObjectName:         stmt.Name.L,
		ObjectType:         "database",
		Options:            options,
		UnextractedOptions: unextracted,
	}
}

// databaseOptionName maps a parsed DatabaseOption type to a stable bounded
// feature name used in unextracted-option evidence.
func databaseOptionName(tp ast.DatabaseOptionType) string {
	switch tp {
	case ast.DatabaseOptionCharset:
		return "charset"
	case ast.DatabaseOptionCollate:
		return "collate"
	case ast.DatabaseOptionEncryption:
		return "encryption"
	case ast.DatabaseOptionPlacementPolicy:
		return "placement_policy"
	default:
		return "unknown"
	}
}

func extractCreateProcedure(stmt *ast.ProcedureInfo) *spec.DDL {
	name := ""
	if stmt.ProcedureName != nil {
		name = stmt.ProcedureName.Name.L
	}
	ddl := &spec.DDL{
		Operation:  spec.DDLOperationCreateProcedure,
		ObjectName: name,
		ObjectType: "procedure",
	}
	if stmt.ProcedureBody != nil {
		ddl.Options = map[string]string{"has_body": "true"}
	}
	if len(stmt.ProcedureParam) > 0 {
		// Parameter lists are parsed but not modeled; bounded presence keeps
		// the statement incomplete alongside the body marker.
		ddl.UnextractedOptions = append(ddl.UnextractedOptions, "params")
	}
	return ddl
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

// omittedUserTargets counts parsed account targets absent from the normalized
// model: every spec beyond the retained first one, plus a retained first spec
// whose identity is the unresolved CURRENT_USER function rather than a name.
func omittedUserTargets(specs []*ast.UserSpec) int {
	if len(specs) == 0 {
		return 0
	}
	omitted := len(specs) - 1
	if first := specs[0]; first != nil && first.User != nil && first.User.CurrentUser {
		omitted++
	}
	return omitted
}

func extractCreateUser(stmt *ast.CreateUserStmt) *spec.DDL {
	if stmt.IsCreateRole {
		name := ""
		if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
			name = stmt.Specs[0].User.Username
		}
		return &spec.DDL{
			Operation:          spec.DDLOperationCreateRole,
			ObjectName:         name,
			ObjectType:         "role",
			OmittedTargets:     omittedUserTargets(stmt.Specs),
			UnextractedOptions: userOptionNames(stmt.Specs, nil, false, stmt.AuthTokenOrTLSOptions, stmt.ResourceOptions, stmt.PasswordOrLockOptions, stmt.CommentOrAttributeOption, stmt.ResourceGroupNameOption),
		}
	}
	name := ""
	if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
		name = stmt.Specs[0].User.Username
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationCreateUser,
		ObjectName:         name,
		ObjectType:         "user",
		Options:            map[string]string{"has_auth": "true"},
		OmittedTargets:     omittedUserTargets(stmt.Specs),
		UnextractedOptions: userOptionNames(stmt.Specs, nil, false, stmt.AuthTokenOrTLSOptions, stmt.ResourceOptions, stmt.PasswordOrLockOptions, stmt.CommentOrAttributeOption, stmt.ResourceGroupNameOption),
	}
}

func extractAlterUser(stmt *ast.AlterUserStmt) *spec.DDL {
	name := ""
	if len(stmt.Specs) > 0 && stmt.Specs[0] != nil && stmt.Specs[0].User != nil {
		name = stmt.Specs[0].User.Username
	}
	omitted := omittedUserTargets(stmt.Specs)
	if len(stmt.Specs) == 0 && (stmt.CurrentAuth != nil || stmt.CurrentDualPasswordOption != 0) {
		// `ALTER USER USER()` carries its target on CurrentAuth / the
		// current-user dual-password option instead of Specs; that single
		// target is still not modeled.
		omitted = 1
	}
	return &spec.DDL{
		Operation:          spec.DDLOperationAlterUser,
		ObjectName:         name,
		ObjectType:         "user",
		Options:            map[string]string{"has_auth": "true"},
		OmittedTargets:     omitted,
		UnextractedOptions: userOptionNames(stmt.Specs, stmt.CurrentAuth, stmt.CurrentDualPasswordOption != 0, stmt.AuthTokenOrTLSOptions, stmt.ResourceOptions, stmt.PasswordOrLockOptions, stmt.CommentOrAttributeOption, stmt.ResourceGroupNameOption),
	}
}

// userOptionNames maps the parsed-but-unmodeled account clauses a CREATE/ALTER
// USER statement carries to bounded option names: per-spec IDENTIFIED auth
// clauses and dual-password flags, the statement-level current-auth clause, and
// the secondary option lists. No credential values travel downstream — only
// presence names.
func userOptionNames(specs []*ast.UserSpec, currentAuth *ast.AuthOption, currentDualPassword bool, auth []*ast.AuthTokenOrTLSOption, resource []*ast.ResourceOption, locks []*ast.PasswordOrLockOption, comment *ast.CommentOrAttributeOption, resourceGroup *ast.ResourceGroupNameOption) []string {
	names := make([]string, 0, 7)
	identified, dualPassword := currentAuth != nil, currentDualPassword
	for _, s := range specs {
		if s == nil {
			continue
		}
		if s.AuthOpt != nil {
			identified = true
		}
		if s.DualPasswordOption != 0 {
			dualPassword = true
		}
	}
	if identified {
		names = append(names, "identified")
	}
	if dualPassword {
		names = append(names, "dual_password")
	}
	if len(auth) > 0 {
		names = append(names, "auth_token_or_tls")
	}
	if len(resource) > 0 {
		names = append(names, "resource")
	}
	if len(locks) > 0 {
		names = append(names, "password_or_lock")
	}
	if comment != nil {
		names = append(names, "comment_or_attribute")
	}
	if resourceGroup != nil {
		names = append(names, "resource_group_name")
	}
	return names
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
			Operation:      spec.DDLOperationDropRole,
			ObjectName:     objectName,
			ObjectType:     "role",
			OmittedTargets: len(names) - 1,
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
		Operation:      spec.DDLOperationDropUser,
		ObjectName:     objectName,
		ObjectType:     "user",
		OmittedTargets: len(names) - 1,
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
		Operation:          spec.DDLOperationGrant,
		Options:            options,
		OmittedTargets:     len(stmt.Users),
		UnextractedOptions: grantOptionNames(stmt.Privs, stmt.ObjectType, stmt.AuthTokenOrTLSOptions, stmt.WithGrant),
	}
}

// grantOptionNames returns bounded names for parsed GRANT/REVOKE details the
// normalized model drops: column-level privileges, routine objects, TLS
// requirements, and WITH GRANT OPTION. Privilege names and grant levels are
// already projected into ddl.Options.
func grantOptionNames(privs []*ast.PrivElem, objectType ast.ObjectTypeType, tls []*ast.AuthTokenOrTLSOption, withGrant bool) []string {
	var names []string
	for _, p := range privs {
		if p != nil && len(p.Cols) > 0 {
			names = append(names, "column_privileges")
			break
		}
	}
	if objectType == ast.ObjectTypeFunction || objectType == ast.ObjectTypeProcedure {
		// GRANT/REVOKE ON FUNCTION|PROCEDURE binds privileges to routines —
		// TiDB has no routine surface, so coverage classifies this name as a
		// vendor boundary there and unaudited under MySQL.
		names = append(names, "routine_object")
	}
	if len(tls) > 0 {
		names = append(names, "require_tls")
	}
	if withGrant {
		names = append(names, "with_grant")
	}
	return names
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
		Operation:          spec.DDLOperationRevoke,
		Options:            options,
		OmittedTargets:     len(stmt.Users),
		UnextractedOptions: grantOptionNames(stmt.Privs, stmt.ObjectType, nil, false),
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
	ddl := &spec.DDL{
		Operation:  spec.DDLOperationCreatePlacementPolicy,
		ObjectName: stmt.PolicyName.L,
		ObjectType: "placement_policy",
	}
	if stmt.OrReplace {
		// CREATE OR REPLACE PLACEMENT POLICY is parsed but the replacement
		// semantics are unmodeled; record bounded presence.
		ddl.UnextractedOptions = append(ddl.UnextractedOptions, "or_replace")
	}
	if len(stmt.PlacementOptions) > 0 {
		ddl.Options = map[string]string{"has_options": "true"}
	}
	return ddl
}

func extractAlterPlacementPolicy(stmt *ast.AlterPlacementPolicyStmt) *spec.DDL {
	ddl := &spec.DDL{
		Operation:  spec.DDLOperationAlterPlacementPolicy,
		ObjectName: stmt.PolicyName.L,
		ObjectType: "placement_policy",
	}
	if len(stmt.PlacementOptions) > 0 {
		ddl.Options = map[string]string{"has_options": "true"}
	}
	return ddl
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
	ddl := &spec.DDL{
		Operation:  spec.DDLOperationCreateSequence,
		ObjectName: name,
		ObjectType: "sequence",
	}
	if len(stmt.SeqOptions) > 0 {
		ddl.Options = map[string]string{"has_options": "true"}
	}
	for _, opt := range stmt.TblOptions {
		if opt == nil {
			continue
		}
		// CREATE SEQUENCE accepts the shared table-option tail (e.g. COMMENT);
		// every member is an unmodeled aspect on a sequence object.
		ddl.UnextractedOptions = append(ddl.UnextractedOptions, tableOptionName(opt.Tp))
	}
	return ddl
}

func extractAlterSequence(stmt *ast.AlterSequenceStmt) *spec.DDL {
	name := ""
	if stmt.Name != nil {
		name = stmt.Name.Name.L
	}
	ddl := &spec.DDL{
		Operation:  spec.DDLOperationAlterSequence,
		ObjectName: name,
		ObjectType: "sequence",
	}
	if len(stmt.SeqOptions) > 0 {
		ddl.Options = map[string]string{"has_options": "true"}
	}
	return ddl
}

func extractDropSequence(stmt *ast.DropSequenceStmt) *spec.DDL {
	name := ""
	if len(stmt.Sequences) > 0 && stmt.Sequences[0] != nil {
		name = stmt.Sequences[0].Name.L
	}
	return &spec.DDL{
		Operation:      spec.DDLOperationDropSequence,
		ObjectName:     name,
		ObjectType:     "sequence",
		OmittedTargets: len(stmt.Sequences) - 1,
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
		case ast.ColumnOptionPrimaryKey:
			change.DeclaresPrimaryKey = true
		}
	}
	if !change.TouchesNullability && !change.TouchesDefault && !change.TouchesAutoIncrement && !change.DeclaresPrimaryKey {
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
		global, hasPredicate, visibility, primaryKeyType, unmodeled := indexOptionFacts(specification.Constraint.Option)
		definition := &spec.Index{Kind: indexKindForConstraint(specification.Constraint.Tp), Name: normalizeConstraintName(specification.Constraint), Columns: extractIndexColumns(specification.Constraint.Keys), Visibility: visibility, VisibilityExplicit: visibility != "", Global: global, HasPredicate: hasPredicate, UnmodeledOptions: unmodeled}
		definition.SetKeyParts(extractIndexKeyParts(specification.Constraint.Keys))
		if specification.Constraint.Tp == ast.ConstraintPrimaryKey {
			definition.PrimaryKeyType = primaryKeyType
		} else if primaryKeyType != "" {
			definition.UnmodeledOptions = append(definition.UnmodeledOptions, "primary_key_type")
		}
		return &spec.AlterIndex{Definition: definition}
	case ast.AlterTableIndexInvisible:
		// ALTER INDEX name VISIBLE|INVISIBLE — the target rides OldName and
		// the requested visibility becomes a declared member fact.
		name := extractAlterName(specification)
		var target string
		switch specification.Visibility {
		case ast.IndexVisibilityVisible:
			target = spec.ColumnVisibilityVisible
		case ast.IndexVisibilityInvisible:
			target = spec.ColumnVisibilityInvisible
		}
		if name == "" || target == "" {
			return nil
		}
		return &spec.AlterIndex{OldName: name, Definition: &spec.Index{Visibility: target, VisibilityExplicit: true}}
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

// extractTableOptions splits parsed table options into modeled key/value facts
// and bounded names of recognized-but-unmodeled options. Unextracted names let
// the coverage layer mark the statement incomplete instead of silently passing.
func extractTableOptions(options []*ast.TableOption) (map[string]string, []string) {
	if len(options) == 0 {
		return nil, nil
	}
	extracted := make(map[string]string)
	var unextracted []string
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
		case ast.TableOptionCollate:
			extracted["collate"] = option.StrValue
		case ast.TableOptionRowFormat:
			if value := rowFormatName(option.UintValue); value != "" {
				extracted["row_format"] = value
			} else {
				unextracted = append(unextracted, tableOptionName(option.Tp))
			}
		case ast.TableOptionAutoIncrement:
			extracted["auto_increment"] = strconv.FormatUint(option.UintValue, 10)
		case ast.TableOptionPlacementPolicy:
			if option.StrValue != "" {
				extracted["placement_policy"] = option.StrValue
			} else {
				unextracted = append(unextracted, tableOptionName(option.Tp))
			}
		default:
			unextracted = append(unextracted, tableOptionName(option.Tp))
		}
	}
	if len(extracted) == 0 {
		extracted = nil
	}
	return extracted, unextracted
}

// tableOptionName maps a parsed TableOption type to a stable bounded feature
// name used in unextracted-option evidence.
func tableOptionName(tp ast.TableOptionType) string {
	switch tp {
	case ast.TableOptionEngine:
		return "engine"
	case ast.TableOptionCharset:
		return "charset"
	case ast.TableOptionCollate:
		return "collate"
	case ast.TableOptionAutoIdCache:
		return "auto_id_cache"
	case ast.TableOptionAutoIncrement:
		return "auto_increment"
	case ast.TableOptionAutoRandomBase:
		return "auto_random_base"
	case ast.TableOptionComment:
		return "comment"
	case ast.TableOptionAvgRowLength:
		return "avg_row_length"
	case ast.TableOptionCheckSum:
		return "checksum"
	case ast.TableOptionCompression:
		return "compression"
	case ast.TableOptionConnection:
		return "connection"
	case ast.TableOptionPassword:
		return "password"
	case ast.TableOptionKeyBlockSize:
		return "key_block_size"
	case ast.TableOptionMaxRows:
		return "max_rows"
	case ast.TableOptionMinRows:
		return "min_rows"
	case ast.TableOptionDelayKeyWrite:
		return "delay_key_write"
	case ast.TableOptionRowFormat:
		return "row_format"
	case ast.TableOptionStatsPersistent:
		return "stats_persistent"
	case ast.TableOptionStatsAutoRecalc:
		return "stats_auto_recalc"
	case ast.TableOptionShardRowID:
		return "shard_row_id_bits"
	case ast.TableOptionPreSplitRegion:
		return "pre_split_regions"
	case ast.TableOptionPackKeys:
		return "pack_keys"
	case ast.TableOptionTablespace:
		return "tablespace"
	case ast.TableOptionNodegroup:
		return "nodegroup"
	case ast.TableOptionDataDirectory:
		return "data_directory"
	case ast.TableOptionIndexDirectory:
		return "index_directory"
	case ast.TableOptionStorageMedia:
		return "storage_media"
	case ast.TableOptionStatsSamplePages:
		return "stats_sample_pages"
	case ast.TableOptionSecondaryEngine:
		return "secondary_engine"
	case ast.TableOptionSecondaryEngineNull:
		return "secondary_engine_null"
	case ast.TableOptionInsertMethod:
		return "insert_method"
	case ast.TableOptionTableCheckSum:
		return "table_checksum"
	case ast.TableOptionUnion:
		return "union"
	case ast.TableOptionEncryption:
		return "encryption"
	case ast.TableOptionTTL:
		return "ttl"
	case ast.TableOptionTTLEnable:
		return "ttl_enable"
	case ast.TableOptionTTLJobInterval:
		return "ttl_job_interval"
	case ast.TableOptionEngineAttribute:
		return "engine_attribute"
	case ast.TableOptionSecondaryEngineAttribute:
		return "secondary_engine_attribute"
	case ast.TableOptionAutoextendSize:
		return "autoextend_size"
	case ast.TableOptionPageChecksum:
		return "page_checksum"
	case ast.TableOptionPageCompressed:
		return "page_compressed"
	case ast.TableOptionPageCompressionLevel:
		return "page_compression_level"
	case ast.TableOptionTransactional:
		return "transactional"
	case ast.TableOptionIetfQuotes:
		return "ietf_quotes"
	case ast.TableOptionSequence:
		return "sequence"
	case ast.TableOptionAffinity:
		return "affinity"
	case ast.TableOptionPlacementPolicy:
		return "placement_policy"
	case ast.TableOptionStatsBuckets:
		return "stats_buckets"
	case ast.TableOptionStatsTopN:
		return "stats_top_n"
	case ast.TableOptionStatsColsChoice:
		return "stats_cols_choice"
	case ast.TableOptionStatsColList:
		return "stats_col_list"
	case ast.TableOptionStatsSampleRate:
		return "stats_sample_rate"
	default:
		return fmt.Sprintf("table_option_%d", int(tp))
	}
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

// exprIsNullLiteral reports whether expr is the SQL NULL literal, recognized
// from the typed datum, never from rendered text. A param marker embeds a
// ValueExpr whose datum also reports a nil value, so markers are excluded
// first; absent nodes and non-literal expressions (functions, variables,
// parameters, arithmetic) are never NULL literals. The pinned parser driver's
// NULL datum returns nil from GetValue, which is the typed fact this check
// relies on.
func exprIsNullLiteral(expr ast.ExprNode) bool {
	if expr == nil {
		return false
	}
	if _, marker := expr.(ast.ParamMarkerExpr); marker {
		return false
	}
	valueExpr, ok := expr.(ast.ValueExpr)
	if !ok {
		return false
	}
	return valueExpr.GetValue() == nil
}

// decodedStringValue returns the parser-decoded string content of a literal
// expression — no SQL quoting, no re-escaping, no trimming. Non-string or
// non-literal expressions (including nil) yield ""; they never become
// comment content through fmt.Sprint.
func decodedStringValue(expr ast.ExprNode) string {
	if valueExpr, ok := expr.(ast.ValueExpr); ok {
		if value, ok := valueExpr.GetValue().(string); ok {
			return value
		}
	}
	return ""
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

// defaultLiteralFacts decodes the comparable literal identity from the
// typed AST, the same node exprIsLiteralValue classifies: parens unwrap,
// and a unary ± integer keeps its sign as provable digits. String values
// are decoded bytes (never the re-quoted display text), integers decode to
// canonical digits. Every other shape — floats, decimals, bit/hex/byte
// literals, NULLs, and non-literal expressions — returns nil: an unproven
// identity, not a fabricated value or a coercion guess.
func defaultLiteralFacts(expr ast.ExprNode) *spec.LiteralDefaultFacts {
	switch node := unwrapParenthesesExpr(expr).(type) {
	case ast.ValueExpr:
		switch value := node.GetValue().(type) {
		case string:
			return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultString, Value: value}
		case int64:
			return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultNumeric, Value: strconv.FormatInt(value, 10)}
		case uint64:
			return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultNumeric, Value: strconv.FormatUint(value, 10)}
		}
	case *ast.UnaryOperationExpr:
		if inner, ok := unwrapParenthesesExpr(node.V).(ast.ValueExpr); ok {
			switch value := inner.GetValue().(type) {
			case int64:
				if node.Op == opcode.Minus {
					return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultNumeric, Value: strconv.FormatInt(-value, 10)}
				}
				if node.Op == opcode.Plus {
					return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultNumeric, Value: strconv.FormatInt(value, 10)}
				}
			case uint64:
				if node.Op == opcode.Plus {
					return &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultNumeric, Value: strconv.FormatUint(value, 10)}
				}
			}
		}
	}
	return nil
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

// extractIndexKeyParts builds the member-level key-part list: each parsed
// part keeps its ordinal position, its column or expression identity, the
// explicitly written prefix length (including zero), the declared sort
// direction, and the multi-valued CAST(… AS … ARRAY) mark. Expression
// parts carry the shared structured facts so governance can tell
// (a+b) from (a*b) and read column dependencies without re-parsing.
func extractIndexKeyParts(parts []*ast.IndexPartSpecification) []spec.IndexKeyPart {
	keyParts := make([]spec.IndexKeyPart, 0, len(parts))
	for i, part := range parts {
		if part == nil {
			keyParts = append(keyParts, spec.IndexKeyPart{Ordinal: i + 1, Kind: spec.IndexKeyPartUnmodeled})
			continue
		}
		keyPart := spec.IndexKeyPart{Ordinal: i + 1, Descending: part.Desc}
		switch {
		case part.Expr != nil || part.Column == nil:
			keyPart.Kind = spec.IndexKeyPartExpression
			if part.Expr != nil {
				facts := expressionFacts(part.Expr)
				keyPart.Expression = &facts
				if cast, ok := part.Expr.(*ast.FuncCastExpr); ok && cast.Tp != nil && cast.Tp.IsArray() {
					keyPart.MultiValued = true
					keyPart.MultiValuedCastType, keyPart.MultiValuedCastLengthSet = MultiValuedCastFacts(cast)
				}
			}
		default:
			keyPart.Kind = spec.IndexKeyPartColumn
			keyPart.Column = part.Column.Name.L
			// OptFieldLen stores UnspecifiedLength (-1) when no prefix is
			// given; any other value is an explicitly written length,
			// including zero.
			if part.Length != tidbtypes.UnspecifiedLength {
				length := part.Length
				keyPart.PrefixLength = &length
			}
		}
		keyParts = append(keyParts, keyPart)
	}
	return keyParts
}

// countIndexPartKinds counts key-part facts extractIndexColumns cannot keep:
// expression (non-column) parts, column-prefix lengths, descending parts,
// and multi-valued CAST(… AS … ARRAY) members. Multi-valued parts stay
// inside the expression count — the mark exists so coverage and version
// applicability can name the form without inspecting expression contents.
func countIndexPartKinds(parts []*ast.IndexPartSpecification) (expr, prefix, desc, multiValued int) {
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Expr != nil || part.Column == nil {
			expr++
			if cast, ok := part.Expr.(*ast.FuncCastExpr); ok && cast.Tp != nil && cast.Tp.IsArray() {
				multiValued++
			}
		}
		// OptFieldLen stores UnspecifiedLength (-1) when no prefix is given, so
		// any other value on a column part is an explicitly written length —
		// including zero. Expression parts leave Length at zero and are held
		// off by the Column != nil guard.
		if part.Column != nil && part.Length != tidbtypes.UnspecifiedLength {
			prefix++
		}
		if part.Desc {
			desc++
		}
	}
	return expr, prefix, desc, multiValued
}

// partitionOptionNames returns bounded option names found on CREATE TABLE
// partition and sub-partition definitions. Those nested lists are separate
// from the top-level table options the normalized model already tracks, so a
// nested PLACEMENT POLICY or ENGINE binding would otherwise vanish entirely.
// Names are deduplicated: the gap is per option family, not per occurrence.
func partitionOptionNames(partition *ast.PartitionOptions) []string {
	if partition == nil {
		return nil
	}
	seen := make(map[string]bool)
	names := make([]string, 0)
	appendOption := func(option *ast.TableOption) {
		if option == nil {
			return
		}
		name := tableOptionName(option.Tp)
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, definition := range partition.Definitions {
		if definition == nil {
			continue
		}
		for _, option := range definition.Options {
			appendOption(option)
		}
		for _, sub := range definition.Sub {
			if sub == nil {
				continue
			}
			for _, option := range sub.Options {
				appendOption(option)
			}
		}
	}
	if len(partition.UpdateIndexes) > 0 && !seen["partition_update_indexes"] {
		names = append(names, "partition_update_indexes")
	}
	return names
}

// extractConstraint normalizes a non-index table constraint, preserving the
// reference target of foreign keys and counting any parsed key-part or
// reference-option facts the normalized model cannot keep.
func extractConstraint(c *ast.Constraint) spec.Constraint {
	constraint := spec.Constraint{
		Type:           constraintTypeName(c.Tp),
		Name:           normalizeConstraintName(c),
		Columns:        extractIndexColumns(c.Keys),
		UnmodeledParts: countUnmodeledIndexParts(c.Keys),
	}
	if c.Tp == ast.ConstraintCheck {
		// The AST stores the declared enforcement direction on
		// Constraint.Enforced; the grammar defaults it true when the
		// keywords are absent, so the AST cannot distinguish "ENFORCED"
		// from undeclared — both produce the same native state anyway.
		constraint.Enforced = checkEnforcedName(c.Enforced)
		if c.Expr != nil {
			facts := expressionFacts(c.Expr)
			constraint.Expression = &facts
			for _, ref := range facts.LocalReferences() {
				constraint.Columns = append(constraint.Columns, ref.Name)
			}
		}
		constraint.InColumn = c.InColumn
	}
	if c.Refer != nil {
		constraint.ReferencedSchema = c.Refer.Table.Schema.L
		constraint.ReferencedTable = c.Refer.Table.Name.L
		constraint.ReferencedColumns = extractIndexColumns(c.Refer.IndexPartSpecifications)
		constraint.UnmodeledReferencedParts = countUnmodeledIndexParts(c.Refer.IndexPartSpecifications)
		if c.Refer.OnDelete != nil && c.Refer.OnDelete.ReferOpt != ast.ReferOptionNoOption {
			constraint.OnDelete = referOptionName(c.Refer.OnDelete.ReferOpt)
		}
		if c.Refer.OnUpdate != nil && c.Refer.OnUpdate.ReferOpt != ast.ReferOptionNoOption {
			constraint.OnUpdate = referOptionName(c.Refer.OnUpdate.ReferOpt)
		}
		if c.Refer.Match != ast.MatchNone {
			constraint.UnmodeledReferActions++
		}
	}
	return constraint
}

// referOptionName normalizes a parsed referential action to the shared
// lowercase vocabulary; unrecognized options withhold as empty.
func referOptionName(opt ast.ReferOptionType) string {
	switch opt {
	case ast.ReferOptionRestrict:
		return "restrict"
	case ast.ReferOptionCascade:
		return "cascade"
	case ast.ReferOptionSetNull:
		return "set_null"
	case ast.ReferOptionNoAction:
		return "no_action"
	case ast.ReferOptionSetDefault:
		return "set_default"
	default:
		return ""
	}
}

// countUnmodeledIndexParts counts key parts that carry any fact beyond a plain
// column reference. It is used for constraint key lists — foreign-key local and
// referenced parts — where the normalized model keeps only column names.
func countUnmodeledIndexParts(parts []*ast.IndexPartSpecification) int {
	count := 0
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Expr != nil || part.Column == nil || (part.Column != nil && part.Length != tidbtypes.UnspecifiedLength) || part.Desc {
			count++
		}
	}
	return count
}

// indexOptionFacts splits a parsed IndexOption into the members the normalized
// model keeps (GLOBAL, partial-index WHERE predicate, the VISIBLE|INVISIBLE
// attribute) and bounded names for the members it does not. Only names
// travel downstream — option values, comments, and predicate expressions
// stay out of the audit model.
func indexOptionFacts(option *ast.IndexOption) (global, hasPredicate bool, visibility, primaryKeyType string, unmodeled []string) {
	if option == nil {
		return false, false, "", "", nil
	}
	global = option.Global
	hasPredicate = option.Condition != nil
	if option.Comment != "" {
		unmodeled = append(unmodeled, "comment")
	}
	if option.KeyBlockSize != 0 {
		unmodeled = append(unmodeled, "key_block_size")
	}
	if option.Tp != ast.IndexTypeInvalid {
		unmodeled = append(unmodeled, "index_type")
	}
	if option.ParserName.L != "" {
		unmodeled = append(unmodeled, "with_parser")
	}
	switch option.Visibility {
	case ast.IndexVisibilityVisible:
		visibility = spec.ColumnVisibilityVisible
	case ast.IndexVisibilityInvisible:
		visibility = spec.ColumnVisibilityInvisible
	}
	if option.PrimaryKeyTp != ast.PrimaryKeyTypeDefault {
		primaryKeyType = strings.ToLower(option.PrimaryKeyTp.String())
	}
	if option.SplitOpt != nil {
		unmodeled = append(unmodeled, "split_opt")
	}
	if option.SecondaryEngineAttr != "" {
		unmodeled = append(unmodeled, "secondary_engine_attr")
	}
	if option.AddColumnarReplicaOnDemand != 0 {
		unmodeled = append(unmodeled, "columnar_replica")
	}
	return global, hasPredicate, visibility, primaryKeyType, unmodeled
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
	refs := extractMutationTableRefs(join)
	tables := make([]spec.Table, 0, len(refs))
	for _, ref := range refs {
		tables = append(tables, ref.table)
	}
	return tables
}

type mutationTableRef struct {
	table spec.Table
	alias string
}

func extractMutationTableRefs(join *ast.Join) []mutationTableRef {
	if join == nil {
		return nil
	}
	refs := make([]mutationTableRef, 0, 2)
	collectMutationTableRefs(join, &refs)
	return refs
}

func collectMutationTableRefs(node ast.ResultSetNode, refs *[]mutationTableRef) {
	switch typed := node.(type) {
	case nil:
		return
	case *ast.Join:
		collectMutationTableRefs(typed.Left, refs)
		collectMutationTableRefs(typed.Right, refs)
	case *ast.TableSource:
		if table, ok := typed.Source.(*ast.TableName); ok {
			appendMutationTableRef(refs, mutationTableRef{
				table: spec.Table{Schema: table.Schema.L, Name: table.Name.L},
				alias: typed.AsName.L,
			})
			return
		}
		collectMutationTableRefs(typed.Source, refs)
	case *ast.TableName:
		appendMutationTableRef(refs, mutationTableRef{table: spec.Table{Schema: typed.Schema.L, Name: typed.Name.L}})
	}
}

func appendMutationTableRef(refs *[]mutationTableRef, ref mutationTableRef) {
	if ref.table.Name == "" || containsTableRefs(*refs, ref.table.Schema, ref.table.Name) {
		return
	}
	*refs = append(*refs, ref)
}

func containsTableRefs(items []mutationTableRef, schema string, name string) bool {
	for _, item := range items {
		if strings.EqualFold(item.table.Schema, schema) && strings.EqualFold(item.table.Name, name) {
			return true
		}
	}
	return false
}

func extractUpdateMutationTables(stmt *ast.UpdateStmt, join *ast.Join) []spec.Table {
	refs := extractMutationTableRefs(join)
	if !joinExists(join) {
		return mutationTablesFromRefs(refs)
	}

	targetQualifiers := make([]string, 0, len(stmt.List))
	for _, assignment := range stmt.List {
		if assignment == nil || assignment.Column == nil || strings.TrimSpace(assignment.Column.Table.L) == "" {
			return nil
		}
		qualifier := assignment.Column.Table.L
		if assignment.Column.Schema.L != "" {
			qualifier = assignment.Column.Schema.L + "." + qualifier
		}
		if !containsStringFold(targetQualifiers, qualifier) {
			targetQualifiers = append(targetQualifiers, qualifier)
		}
	}
	if len(targetQualifiers) == 0 {
		return nil
	}

	tables := make([]spec.Table, 0, len(targetQualifiers))
	for _, ref := range refs {
		for _, qualifier := range targetQualifiers {
			if mutationTableRefMatches(ref, qualifier) {
				tables = append(tables, ref.table)
				break
			}
		}
	}
	if len(tables) != len(targetQualifiers) {
		return nil
	}
	return tables
}

func extractDeleteMutationTables(stmt *ast.DeleteStmt, join *ast.Join) []spec.Table {
	if stmt.Tables == nil || len(stmt.Tables.Tables) == 0 {
		return extractMutationTables(join)
	}
	tables := make([]spec.Table, 0, len(stmt.Tables.Tables))
	for _, table := range stmt.Tables.Tables {
		if table == nil || table.Name.L == "" || containsTable(tables, table.Schema.L, table.Name.L) {
			continue
		}
		tables = append(tables, spec.Table{Schema: table.Schema.L, Name: table.Name.L})
	}
	return tables
}

func mutationTablesFromRefs(refs []mutationTableRef) []spec.Table {
	tables := make([]spec.Table, 0, len(refs))
	for _, ref := range refs {
		tables = append(tables, ref.table)
	}
	return tables
}

func mutationTableRefMatches(ref mutationTableRef, qualifier string) bool {
	if strings.EqualFold(ref.alias, qualifier) || strings.EqualFold(ref.table.Name, qualifier) {
		return true
	}
	return ref.table.Schema != "" && strings.EqualFold(ref.table.Schema+"."+ref.table.Name, qualifier)
}

func containsStringFold(values []string, value string) bool {
	for _, item := range values {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
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
	case ast.AlterTableOption:
		return "table_option"
	case ast.AlterTableAddColumns:
		return "add_columns"
	case ast.AlterTableAddConstraint:
		return "add_constraint"
	case ast.AlterTableDropColumn:
		return "drop_column"
	case ast.AlterTableDropPrimaryKey:
		return "drop_primary_key"
	case ast.AlterTableDropIndex:
		return "drop_index"
	case ast.AlterTableDropForeignKey:
		return "drop_foreign_key"
	case ast.AlterTableModifyColumn:
		return "modify_column"
	case ast.AlterTableChangeColumn:
		return "change_column"
	case ast.AlterTableRenameColumn:
		return "rename_column"
	case ast.AlterTableRenameTable:
		return "rename_table"
	case ast.AlterTableAlterColumn:
		return "alter_column"
	case ast.AlterTableLock:
		return "lock"
	case ast.AlterTableWriteable:
		return "writeable"
	case ast.AlterTableAlgorithm:
		return "algorithm"
	case ast.AlterTableRenameIndex:
		return "rename_index"
	case ast.AlterTableForce:
		return "force"
	case ast.AlterTableAddPartitions:
		return "add_partitions"
	case ast.AlterTablePartitionAttributes:
		return "partition_attributes"
	case ast.AlterTablePartitionOptions:
		return "partition_options"
	case ast.AlterTableCoalescePartitions:
		return "coalesce_partitions"
	case ast.AlterTableDropPartition:
		return "drop_partition"
	case ast.AlterTableTruncatePartition:
		return "truncate_partition"
	case ast.AlterTablePartition:
		return "partition"
	case ast.AlterTableEnableKeys:
		return "enable_keys"
	case ast.AlterTableDisableKeys:
		return "disable_keys"
	case ast.AlterTableRemovePartitioning:
		return "remove_partitioning"
	case ast.AlterTableWithValidation:
		return "with_validation"
	case ast.AlterTableWithoutValidation:
		return "without_validation"
	case ast.AlterTableSecondaryLoad:
		return "secondary_load"
	case ast.AlterTableSecondaryUnload:
		return "secondary_unload"
	case ast.AlterTableRebuildPartition:
		return "rebuild_partition"
	case ast.AlterTableReorganizePartition:
		return "reorganize_partition"
	case ast.AlterTableCheckPartitions:
		return "check_partitions"
	case ast.AlterTableExchangePartition:
		return "exchange_partition"
	case ast.AlterTableOptimizePartition:
		return "optimize_partition"
	case ast.AlterTableRepairPartition:
		return "repair_partition"
	case ast.AlterTableImportPartitionTablespace:
		return "import_partition_tablespace"
	case ast.AlterTableDiscardPartitionTablespace:
		return "discard_partition_tablespace"
	case ast.AlterTableAlterCheck:
		return "alter_check"
	case ast.AlterTableDropCheck:
		return "drop_check"
	case ast.AlterTableImportTablespace:
		return "import_tablespace"
	case ast.AlterTableDiscardTablespace:
		return "discard_tablespace"
	case ast.AlterTableIndexInvisible:
		return "alter_index"
	case ast.AlterTableOrderByColumns:
		return "order_by_columns"
	case ast.AlterTableSetTiFlashReplica:
		return "set_tiflash_replica"
	case ast.AlterTableAddStatistics:
		return "add_statistics"
	case ast.AlterTableDropStatistics:
		return "drop_statistics"
	case ast.AlterTableAttributes:
		return "attributes"
	case ast.AlterTableCache:
		return "cache"
	case ast.AlterTableNoCache:
		return "nocache"
	case ast.AlterTableStatsOptions:
		return "stats_options"
	case ast.AlterTableDropFirstPartition:
		return "drop_first_partition"
	case ast.AlterTableAddLastPartition:
		return "add_last_partition"
	case ast.AlterTableReorganizeLastPartition:
		return "reorganize_last_partition"
	case ast.AlterTableReorganizeFirstPartition:
		return "reorganize_first_partition"
	case ast.AlterTableRemoveTTL:
		return "remove_ttl"
	case ast.AlterTableSplitIndex:
		return "split_index"
	case ast.AlterTableAddMaskingPolicy:
		return "add_masking_policy"
	case ast.AlterTableEnableMaskingPolicy:
		return "enable_masking_policy"
	case ast.AlterTableDisableMaskingPolicy:
		return "disable_masking_policy"
	case ast.AlterTableDropMaskingPolicy:
		return "drop_masking_policy"
	case ast.AlterTableModifyMaskingPolicyExpression:
		return "modify_masking_policy_expression"
	case ast.AlterTableModifyMaskingPolicyRestrictOn:
		return "modify_masking_policy_restrict_on"
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
	case ast.ConstraintVector:
		return "vector"
	case ast.ConstraintColumnar:
		return "columnar"
	default:
		return fmt.Sprintf("constraint_%d", tp)
	}
}

// sqlBareWords returns the unquoted, uncommented word tokens of one SQL
// fragment — the parser adaptation layer's way to recover facts the PingCAP
// AST normalized away (CHECK vs CONSTRAINT spelling, an authored [NOT]
// ENFORCED clause) from the exact clause text the grammar consumed. Quoted
// identifiers and string literals contribute nothing; comments, including
// versioned /*! ... */ blocks, are skipped wholesale (their inner text is
// not trusted as a fact source). Input is guaranteed to have parsed, so
// unterminated constructs simply end the scan.
func sqlBareWords(text string) []string {
	words := make([]string, 0, 8)
	i, n := 0, len(text)
	for i < n {
		c := text[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			quote := c
			i++
			for i < n {
				if text[i] == '\\' && quote != '`' {
					i += 2
					continue
				}
				if text[i] == quote {
					if quote != '`' && i+1 < n && text[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '-' && i+1 < n && text[i+1] == '-':
			for i < n && text[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && text[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && text[i+1] == '*':
			i += 2
			for i+1 < n && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			}
		case isBareWordChar(c):
			start := i
			for i < n && isBareWordChar(text[i]) {
				i++
			}
			words = append(words, strings.ToLower(text[start:i]))
		default:
			i++
		}
	}
	return words
}

func isBareWordChar(c byte) bool {
	return c == '_' || c == '$' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// checkSpellingForClause recovers whether one alter_check|drop_check
// action was authored as CHECK or as the generic CONSTRAINT alias: the
// spelling keyword is the word immediately after the action verb. The
// clause may still carry its ALTER TABLE <name> prefix (or be the whole
// raw statement when splitting could not align), so the answer is the
// first check|constraint word that directly follows an alter|drop verb —
// a table or constraint name spelled "check" elsewhere in the text is
// never preceded by a verb and cannot collide. Anything else answers ""
// — the spelling is then unproven, never guessed.
func checkSpellingForClause(clause string) string {
	words := sqlBareWords(clause)
	for i := 1; i < len(words); i++ {
		if words[i-1] != "alter" && words[i-1] != "drop" {
			continue
		}
		switch words[i] {
		case "check":
			return spec.CheckSpellingCheck
		case "constraint":
			return spec.CheckSpellingConstraint
		}
	}
	return ""
}

// clauseDeclaresEnforced reports whether the clause's bare words carry an
// authored ENFORCED token — the word appears only in the CHECK [NOT]
// ENFORCED clause, so its presence proves the enforcement clause was
// written somewhere inside the fragment. Callers apply it at clause scope:
// a shared multi-member clause marks every declared member, which stays
// sound because an explicit clause refuses the whole statement on the one
// boundary where the fact matters (MySQL <8.0.16).
func clauseDeclaresEnforced(clause string) bool {
	for _, word := range sqlBareWords(clause) {
		if word == "enforced" {
			return true
		}
	}
	return false
}

// createCheckExplicitFlags splits the CREATE TABLE element list and marks
// which Constraints and Cols entries provably carried an authored
// ENFORCED token — the AST records neither the clause's presence nor the
// element boundary, so the comma-split element text (same splitter ALTER
// clauses use) is correlated by position: constraint elements consume the
// Constraints order, everything else consumes the Cols order. An
// unalignable body answers empty maps — the fact stays unproven rather
// than guessed.
func createCheckExplicitFlags(stmt *ast.CreateTableStmt, rawSQL string) (constraintExplicit, columnExplicit map[int]bool) {
	constraintExplicit = map[int]bool{}
	columnExplicit = map[int]bool{}
	body := createTableElementBody(rawSQL)
	if body == "" {
		return constraintExplicit, columnExplicit
	}
	elements := splitAlterTableClauses(body)
	if len(elements) != len(stmt.Cols)+len(stmt.Constraints) {
		return constraintExplicit, columnExplicit
	}
	constraintIdx, columnIdx := 0, 0
	for _, element := range elements {
		words := sqlBareWords(element)
		explicit := false
		for _, word := range words {
			if word == "enforced" {
				explicit = true
				break
			}
		}
		if len(words) > 0 && createElementIsConstraint(words[0]) {
			if explicit {
				constraintExplicit[constraintIdx] = true
			}
			constraintIdx++
			continue
		}
		if explicit {
			columnExplicit[columnIdx] = true
		}
		columnIdx++
	}
	if constraintIdx != len(stmt.Constraints) || columnIdx != len(stmt.Cols) {
		return map[int]bool{}, map[int]bool{}
	}
	return constraintExplicit, columnExplicit
}

// createTableElementBody extracts the parenthesized element list of a
// CREATE TABLE statement — the first top-level (...) after the table
// name. Statements without an element list (LIKE, pure AS SELECT) answer "".
func createTableElementBody(rawSQL string) string {
	var quote byte
	depth := 0
	start := -1
	for i := 0; i < len(rawSQL); i++ {
		c := rawSQL[i]
		if quote != 0 {
			if c == '\\' && quote != '`' {
				i++
				continue
			}
			if c == quote {
				if quote != '`' && i+1 < len(rawSQL) && rawSQL[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				return rawSQL[start:i]
			}
		}
	}
	return ""
}

// createElementIsConstraint classifies a split element's first bare word:
// everything the grammar allows to open a table-level constraint or index
// clause consumes a Constraints slot; identifiers and column types fall
// through to the Cols order.
func createElementIsConstraint(word string) bool {
	switch word {
	case "constraint", "check", "primary", "unique", "key", "index", "foreign", "fulltext", "spatial", "vector":
		return true
	default:
		return false
	}
}
