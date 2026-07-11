// Package tidbparser extracts query access facts from TiDB AST nodes.
// input: SQL text, dialect, and default schema through the TiDB parser
// output: parser-neutral QueryAccessFacts for relation, column, output, and read-classification analysis
// pos: infrastructure query access adapter between TiDB AST and application query access contracts
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

type QueryAccessExtractor struct{}

type QueryAccessFacts struct {
	ReadClassification string           `json:"read_classification"`
	Relations          []RelationFact   `json:"relations,omitempty"`
	ColumnReferences   []ColumnFact     `json:"column_references,omitempty"`
	Outputs            []OutputFact     `json:"outputs,omitempty"`
	Unresolved         []UnresolvedFact `json:"unresolved,omitempty"`
	ReasonCodes        []string         `json:"reason_codes,omitempty"`
}

type RelationFact struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
	Alias  string `json:"alias,omitempty"`
	Kind   string `json:"kind"`
}

type ColumnFact struct {
	Schema string   `json:"schema,omitempty"`
	Table  string   `json:"table"`
	Column string   `json:"column"`
	Usages []string `json:"usages"`
}

type OutputFact struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

type UnresolvedFact struct {
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

type statementFacts struct {
	classification string
	relations      []RelationFact
	columns        []ColumnFact
	outputs        []OutputFact
	unresolved     []UnresolvedFact
	reasons        []string
}

func (e *QueryAccessExtractor) ExtractQueryAccess(ctx context.Context, sql string, dialect string, defaultSchema string) (*QueryAccessFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("extract cancelled: %w", err)
	}

	parsed, _, parseErr := parser.New().Parse(sql, "", "")
	if parseErr != nil {
		return &QueryAccessFacts{
			ReadClassification: "indeterminate",
			ReasonCodes:        []string{"parse_failure"},
		}, nil
	}

	if len(parsed) == 0 {
		return &QueryAccessFacts{
			ReadClassification: "indeterminate",
			ReasonCodes:        []string{"zero_statements"},
		}, nil
	}

	classifications := make([]string, 0, len(parsed))
	allRelations := make([]RelationFact, 0)
	allColumns := make([]ColumnFact, 0)
	allOutputs := make([]OutputFact, 0)
	allUnresolved := make([]UnresolvedFact, 0)
	allReasons := make([]string, 0)

	for _, stmt := range parsed {
		scope := newScopeStack(defaultSchema)
		facts := analyzeStatement(stmt, scope)
		classifications = append(classifications, facts.classification)
		allRelations = append(allRelations, facts.relations...)
		allColumns = append(allColumns, facts.columns...)
		allOutputs = append(allOutputs, facts.outputs...)
		allUnresolved = append(allUnresolved, facts.unresolved...)
		allReasons = append(allReasons, facts.reasons...)
	}

	return &QueryAccessFacts{
		ReadClassification: foldClassifications(classifications),
		Relations:          deduplicateRelations(allRelations),
		ColumnReferences:   mergeColumnFacts(allColumns),
		Outputs:            allOutputs,
		Unresolved:         allUnresolved,
		ReasonCodes:        deduplicateStrings(allReasons),
	}, nil
}

func analyzeStatement(stmt ast.StmtNode, scope *scopeStack) statementFacts {
	switch s := stmt.(type) {
	case *ast.SelectStmt:
		return analyzeSelect(s, scope)
	case *ast.SetOprStmt:
		return analyzeSetOperation(s, scope)
	case *ast.ExplainStmt:
		return statementFacts{classification: "read_only"}
	default:
		return statementFacts{classification: "not_read_only", reasons: []string{"write_operation"}}
	}
}

func analyzeSelect(sel *ast.SelectStmt, scope *scopeStack) statementFacts {
	f := statementFacts{
		relations:  make([]RelationFact, 0),
		columns:    make([]ColumnFact, 0),
		outputs:    make([]OutputFact, 0),
		unresolved: make([]UnresolvedFact, 0),
		reasons:    make([]string, 0),
	}

	if sel.LockInfo != nil {
		return statementFacts{classification: "not_read_only", reasons: []string{"write_operation"}}
	}
	if sel.SelectIntoOpt != nil {
		return statementFacts{classification: "not_read_only", reasons: []string{"write_operation"}}
	}

	if sel.With != nil {
		for _, cte := range sel.With.CTEs {
			if cte == nil {
				continue
			}
			cteName := cte.Name.L
			scope.addCTE(cteName)
			f.relations = append(f.relations, RelationFact{Name: cteName, Kind: "cte"})
			if cte.Query != nil {
				cteScope := newScopeStack(scope.defaultSchema)
				for k, v := range scope.lineage {
					cteScope.lineage[k] = v
				}
				cteFacts := analyzeQueryNode(cte.Query, cteScope)
				f.relations = append(f.relations, cteFacts.relations...)
				f.columns = append(f.columns, cteFacts.columns...)
				f.unresolved = append(f.unresolved, cteFacts.unresolved...)
				f.reasons = append(f.reasons, cteFacts.reasons...)

				cteLineage := make(lineageEntry)
				for _, out := range cteFacts.outputs {
					if out.Name != "" && len(out.Sources) > 0 {
						cteLineage[strings.ToLower(out.Name)] = out.Sources
					}
				}
				scope.addLineage(cteName, cteLineage)
			}
		}
	}

	if sel.From != nil && sel.From.TableRefs != nil {
		fromFacts := processTableRefs(sel.From.TableRefs, scope)
		f.relations = append(f.relations, fromFacts.relations...)
		f.columns = append(f.columns, fromFacts.columns...)
		f.unresolved = append(f.unresolved, fromFacts.unresolved...)
	}

	if sel.Fields != nil {
		for _, field := range sel.Fields.Fields {
			if field == nil {
				continue
			}
			outputName := field.AsName.L

			if field.WildCard != nil {
				ref := "*"
				if field.WildCard.Table.L != "" {
					ref = field.WildCard.Table.L + ".*"
				}
				f.unresolved = append(f.unresolved, UnresolvedFact{Reference: ref, Reason: "schema_unavailable"})
				if outputName == "" {
					outputName = ref
				}
				f.outputs = append(f.outputs, OutputFact{Name: outputName})
				continue
			}

			if field.Expr == nil {
				continue
			}

			if cne, ok := field.Expr.(*ast.ColumnNameExpr); ok && cne.Name != nil && cne.Name.Name.L == "*" {
				f.unresolved = append(f.unresolved, UnresolvedFact{Reference: "*", Reason: "schema_unavailable"})
				if outputName == "" {
					outputName = "*"
				}
				f.outputs = append(f.outputs, OutputFact{Name: outputName})
				continue
			}

			if nodeHasFunctionCall(field.Expr) {
				f.reasons = append(f.reasons, "function_call")
			}

			fieldCols, fieldSources := collectColumnRefsFromExpr(field.Expr, "projection", scope, &f.unresolved)
			f.columns = append(f.columns, fieldCols...)
			if outputName == "" {
				outputName = exprDisplayName(field.Expr)
			}
			f.outputs = append(f.outputs, OutputFact{Name: outputName, Sources: fieldSources})
		}
	}

	if sel.Where != nil {
		whereRels := make([]RelationFact, 0)
		whereUnres := make([]UnresolvedFact, 0)
		f.columns = append(f.columns, collectColumnRefsWithSubqueries(sel.Where, "filter", scope, &whereRels, &whereUnres)...)
		f.relations = append(f.relations, whereRels...)
		f.unresolved = append(f.unresolved, whereUnres...)
		if nodeHasFunctionCall(sel.Where) {
			f.reasons = append(f.reasons, "function_call")
		}
	}

	if sel.GroupBy != nil {
		for _, item := range sel.GroupBy.Items {
			if item != nil {
				gbUnres := make([]UnresolvedFact, 0)
				f.columns = append(f.columns, collectColumnRefsWithSubqueries(item.Expr, "grouping", scope, nil, &gbUnres)...)
				f.unresolved = append(f.unresolved, gbUnres...)
			}
		}
	}

	if sel.Having != nil && sel.Having.Expr != nil {
		havingUnres := make([]UnresolvedFact, 0)
		f.columns = append(f.columns, collectColumnRefsWithSubqueries(sel.Having.Expr, "having", scope, nil, &havingUnres)...)
		f.unresolved = append(f.unresolved, havingUnres...)
		if nodeHasFunctionCall(sel.Having.Expr) {
			f.reasons = append(f.reasons, "function_call")
		}
	}

	if sel.OrderBy != nil {
		for _, item := range sel.OrderBy.Items {
			if item != nil {
				obUnres := make([]UnresolvedFact, 0)
				f.columns = append(f.columns, collectColumnRefsWithSubqueries(item.Expr, "ordering", scope, nil, &obUnres)...)
				f.unresolved = append(f.unresolved, obUnres...)
			}
		}
	}

	for i := range sel.WindowSpecs {
		ws := &sel.WindowSpecs[i]
		if ws.PartitionBy != nil {
			for _, item := range ws.PartitionBy.Items {
				if item != nil {
					winUnres := make([]UnresolvedFact, 0)
					f.columns = append(f.columns, collectColumnRefsWithSubqueries(item.Expr, "window", scope, nil, &winUnres)...)
					f.unresolved = append(f.unresolved, winUnres...)
				}
			}
		}
		if ws.OrderBy != nil {
			for _, item := range ws.OrderBy.Items {
				if item != nil {
					winUnres := make([]UnresolvedFact, 0)
					f.columns = append(f.columns, collectColumnRefsWithSubqueries(item.Expr, "window", scope, nil, &winUnres)...)
					f.unresolved = append(f.unresolved, winUnres...)
				}
			}
		}
	}

	if sel.Fields != nil {
		for _, field := range sel.Fields.Fields {
			if field != nil && field.Expr != nil && nodeHasWindowFunc(field.Expr) {
				f.reasons = append(f.reasons, "function_call")
			}
		}
	}

	if len(f.reasons) > 0 || hasWildcardUnresolved(f.unresolved) || hasAmbiguousUnresolved(f.unresolved) {
		f.classification = "indeterminate"
	} else {
		f.classification = "read_only"
	}

	return f
}

func analyzeSetOperation(setOpr *ast.SetOprStmt, scope *scopeStack) statementFacts {
	f := statementFacts{
		relations:  make([]RelationFact, 0),
		columns:    make([]ColumnFact, 0),
		outputs:    make([]OutputFact, 0),
		unresolved: make([]UnresolvedFact, 0),
		reasons:    make([]string, 0),
	}
	classifications := make([]string, 0)

	if setOpr.SelectList != nil {
		for _, node := range setOpr.SelectList.Selects {
			if node == nil {
				continue
			}
			childScope := newScopeStack(scope.defaultSchema)
			childFacts := analyzeQueryNode(node, childScope)
			classifications = append(classifications, childFacts.classification)
			f.relations = append(f.relations, childFacts.relations...)
			f.columns = append(f.columns, childFacts.columns...)
			f.outputs = append(f.outputs, childFacts.outputs...)
			f.unresolved = append(f.unresolved, childFacts.unresolved...)
			f.reasons = append(f.reasons, childFacts.reasons...)
		}
	}

	f.classification = foldClassifications(classifications)
	return f
}

func analyzeQueryNode(node ast.Node, scope *scopeStack) statementFacts {
	switch q := node.(type) {
	case *ast.SelectStmt:
		return analyzeSelect(q, scope)
	case *ast.SetOprStmt:
		return analyzeSetOperation(q, scope)
	case *ast.SubqueryExpr:
		if q.Query != nil {
			return analyzeQueryNode(q.Query, scope)
		}
		return statementFacts{classification: "indeterminate", reasons: []string{"unsupported_node"}}
	default:
		return statementFacts{classification: "indeterminate", reasons: []string{"unsupported_node"}}
	}
}

type joinFacts struct {
	relations  []RelationFact
	columns    []ColumnFact
	unresolved []UnresolvedFact
}

func processTableRefs(join *ast.Join, scope *scopeStack) joinFacts {
	f := joinFacts{
		relations:  make([]RelationFact, 0),
		columns:    make([]ColumnFact, 0),
		unresolved: make([]UnresolvedFact, 0),
	}
	if join == nil {
		return f
	}

	leftFacts := processResultSetNode(join.Left, scope)
	f.relations = append(f.relations, leftFacts.relations...)
	f.columns = append(f.columns, leftFacts.columns...)
	f.unresolved = append(f.unresolved, leftFacts.unresolved...)

	if join.Right != nil {
		rightFacts := processResultSetNode(join.Right, scope)
		f.relations = append(f.relations, rightFacts.relations...)
		f.columns = append(f.columns, rightFacts.columns...)
		f.unresolved = append(f.unresolved, rightFacts.unresolved...)
	}

	if join.On != nil {
		onUnres := make([]UnresolvedFact, 0)
		f.columns = append(f.columns, collectColumnRefsWithSubqueries(join.On.Expr, "join", scope, nil, &onUnres)...)
		f.unresolved = append(f.unresolved, onUnres...)
	}

	if join.Using != nil {
		for _, col := range join.Using {
			if col != nil {
				f.columns = append(f.columns, ColumnFact{Column: col.Name.L, Usages: []string{"join"}})
			}
		}
	}

	return f
}

func processResultSetNode(node ast.ResultSetNode, scope *scopeStack) joinFacts {
	f := joinFacts{
		relations:  make([]RelationFact, 0),
		columns:    make([]ColumnFact, 0),
		unresolved: make([]UnresolvedFact, 0),
	}

	switch n := node.(type) {
	case *ast.Join:
		joinF := processTableRefs(n, scope)
		f.relations = append(f.relations, joinF.relations...)
		f.columns = append(f.columns, joinF.columns...)
		f.unresolved = append(f.unresolved, joinF.unresolved...)
		return f

	case *ast.TableSource:
		alias := n.AsName.L
		switch src := n.Source.(type) {
		case *ast.TableName:
			schema := src.Schema.L
			if schema == "" {
				schema = scope.defaultSchema
			}
			name := src.Name.L
			f.relations = append(f.relations, RelationFact{Schema: schema, Name: name, Alias: alias, Kind: "table"})
			if alias != "" {
				scope.addAlias(alias, schema, name)
			}
			scope.addRelation(schema, name)

		case *ast.SubqueryExpr:
			derivedName := alias
			if derivedName == "" {
				derivedName = fmt.Sprintf("subquery_%d", len(scope.relations))
			}
			f.relations = append(f.relations, RelationFact{Name: derivedName, Kind: "derived"})
			scope.addAlias(derivedName, "", derivedName)
			if src.Query != nil {
				subScope := newScopeStack(scope.defaultSchema)
				subFacts := analyzeQueryNode(src.Query, subScope)
				f.relations = append(f.relations, subFacts.relations...)
				f.columns = append(f.columns, subFacts.columns...)
				f.unresolved = append(f.unresolved, subFacts.unresolved...)

				derivedLineage := make(lineageEntry)
				for _, out := range subFacts.outputs {
					if out.Name != "" && len(out.Sources) > 0 {
						derivedLineage[strings.ToLower(out.Name)] = out.Sources
					}
				}
				scope.addLineage(derivedName, derivedLineage)
			}

		case *ast.SelectStmt:
			derivedName := alias
			if derivedName == "" {
				derivedName = fmt.Sprintf("subquery_%d", len(scope.relations))
			}
			f.relations = append(f.relations, RelationFact{Name: derivedName, Kind: "derived"})
			scope.addAlias(derivedName, "", derivedName)
			subScope := newScopeStack(scope.defaultSchema)
			subFacts := analyzeSelect(src, subScope)
			f.relations = append(f.relations, subFacts.relations...)
			f.columns = append(f.columns, subFacts.columns...)
			f.unresolved = append(f.unresolved, subFacts.unresolved...)

			derivedLineage := make(lineageEntry)
			for _, out := range subFacts.outputs {
				if out.Name != "" && len(out.Sources) > 0 {
					derivedLineage[strings.ToLower(out.Name)] = out.Sources
				}
			}
			scope.addLineage(derivedName, derivedLineage)
		}
	}

	return f
}

func collectColumnRefsWithSubqueries(expr ast.ExprNode, usage string, scope *scopeStack, extraRels *[]RelationFact, extraUnres *[]UnresolvedFact) []ColumnFact {
	if expr == nil {
		return nil
	}
	cols, _ := collectColumnRefsFromExpr(expr, usage, scope, extraUnres)
	if extraRels != nil {
		rels := make([]RelationFact, 0)
		expr.Accept(&subqueryRelationCollector{scope: scope, relations: &rels, unresolved: extraUnres})
		*extraRels = append(*extraRels, rels...)
	}
	return cols
}

func collectColumnRefsFromExpr(expr ast.ExprNode, usage string, scope *scopeStack, unresolved *[]UnresolvedFact) ([]ColumnFact, []string) {
	if expr == nil {
		return nil, nil
	}

	cols := make([]ColumnFact, 0)
	sources := make([]string, 0)
	ambiguous := false

	expr.Accept(&columnCollectVisitor{
		usage:      usage,
		scope:      scope,
		columns:    &cols,
		sources:    &sources,
		unresolved: unresolved,
		ambiguous:  &ambiguous,
	})

	if ambiguous && unresolved != nil {
		*unresolved = append(*unresolved, UnresolvedFact{Reference: "unqualified_column", Reason: "ambiguous_reference"})
	}

	return cols, sources
}

type columnCollectVisitor struct {
	usage      string
	scope      *scopeStack
	columns    *[]ColumnFact
	sources    *[]string
	unresolved *[]UnresolvedFact
	ambiguous  *bool
}

func (v *columnCollectVisitor) Enter(in ast.Node) (ast.Node, bool) {
	if _, ok := in.(*ast.SubqueryExpr); ok {
		return in, true
	}

	if colExpr, ok := in.(*ast.ColumnNameExpr); ok {
		if colExpr.Name == nil {
			return in, false
		}
		colName := colExpr.Name.Name.L
		if colName == "*" {
			return in, false
		}

		tableName := colExpr.Name.Table.L
		schemaName := colExpr.Name.Schema.L

		if tableName != "" {
			if sources, ok := v.scope.lookupLineage(tableName, colName); ok && len(sources) > 0 {
				physSchema, physTable := parseSourceTable(sources[0])
				*v.columns = append(*v.columns, ColumnFact{
					Schema: physSchema,
					Table:  physTable,
					Column: colName,
					Usages: []string{v.usage},
				})
				*v.sources = append(*v.sources, sources...)
				return in, false
			}
		}

		var resolvedSchema, resolvedTable string
		if tableName != "" {
			resolvedSchema, resolvedTable = v.scope.resolveColumn(tableName, schemaName)
		}
		if resolvedTable == "" {
			resolvedSchema, resolvedTable = v.scope.findColumnInRelations()
			if resolvedTable == "" {
				if v.ambiguous != nil && len(v.scope.relations) > 1 && tableName == "" {
					*v.ambiguous = true
				}
				return in, false
			}
			if sources, ok := v.scope.lookupLineage(resolvedTable, colName); ok && len(sources) > 0 {
				physSchema, physTable := parseSourceTable(sources[0])
				*v.columns = append(*v.columns, ColumnFact{
					Schema: physSchema,
					Table:  physTable,
					Column: colName,
					Usages: []string{v.usage},
				})
				*v.sources = append(*v.sources, sources...)
				return in, false
			}
		}

		*v.columns = append(*v.columns, ColumnFact{
			Schema: resolvedSchema,
			Table:  resolvedTable,
			Column: colName,
			Usages: []string{v.usage},
		})

		sourceKey := resolvedTable + "." + colName
		if resolvedSchema != "" {
			sourceKey = resolvedSchema + "." + sourceKey
		}
		*v.sources = append(*v.sources, sourceKey)
	}
	return in, false
}

func (v *columnCollectVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

type subqueryRelationCollector struct {
	scope      *scopeStack
	relations  *[]RelationFact
	unresolved *[]UnresolvedFact
}

func (v *subqueryRelationCollector) Enter(in ast.Node) (ast.Node, bool) {
	if subq, ok := in.(*ast.SubqueryExpr); ok && subq.Query != nil {
		subScope := newScopeStack(v.scope.defaultSchema)
		subFacts := analyzeQueryNode(subq.Query, subScope)
		*v.relations = append(*v.relations, subFacts.relations...)
		if v.unresolved != nil {
			*v.unresolved = append(*v.unresolved, subFacts.unresolved...)
		}
		return in, true
	}
	return in, false
}

func (v *subqueryRelationCollector) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func parseSourceTable(source string) (schema, table string) {
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

func exprDisplayName(expr ast.ExprNode) string {
	if expr == nil {
		return ""
	}
	if colExpr, ok := expr.(*ast.ColumnNameExpr); ok && colExpr.Name != nil {
		return colExpr.Name.Name.L
	}
	return strings.TrimSpace(expr.Text())
}

func nodeHasFunctionCall(node ast.Node) bool {
	found := false
	node.Accept(&funcDetector{found: &found})
	return found
}

type funcDetector struct {
	found *bool
}

func (v *funcDetector) Enter(in ast.Node) (ast.Node, bool) {
	if *v.found {
		return in, true
	}
	switch in.(type) {
	case *ast.FuncCallExpr, *ast.WindowFuncExpr, *ast.AggregateFuncExpr:
		*v.found = true
		return in, true
	}
	return in, false
}

func (v *funcDetector) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func nodeHasWindowFunc(expr ast.ExprNode) bool {
	found := false
	expr.Accept(&windowDetector{found: &found})
	return found
}

type windowDetector struct {
	found *bool
}

func (v *windowDetector) Enter(in ast.Node) (ast.Node, bool) {
	if *v.found {
		return in, true
	}
	if _, ok := in.(*ast.WindowFuncExpr); ok {
		*v.found = true
		return in, true
	}
	return in, false
}

func (v *windowDetector) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func hasWildcardUnresolved(unresolved []UnresolvedFact) bool {
	for _, u := range unresolved {
		if u.Reason == "schema_unavailable" {
			return true
		}
	}
	return false
}

func hasAmbiguousUnresolved(unresolved []UnresolvedFact) bool {
	for _, u := range unresolved {
		if u.Reason == "ambiguous_reference" {
			return true
		}
	}
	return false
}

func foldClassifications(classifications []string) string {
	if len(classifications) == 0 {
		return "indeterminate"
	}
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

func deduplicateRelations(relations []RelationFact) []RelationFact {
	seen := make(map[string]bool)
	result := make([]RelationFact, 0, len(relations))
	for _, r := range relations {
		key := r.Schema + "." + r.Name
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}
	return result
}

func mergeColumnFacts(columns []ColumnFact) []ColumnFact {
	type colKey struct{ schema, table, column string }
	merged := make(map[colKey][]string)
	order := make([]colKey, 0)
	for _, c := range columns {
		k := colKey{c.Schema, c.Table, c.Column}
		if _, exists := merged[k]; !exists {
			order = append(order, k)
		}
		merged[k] = append(merged[k], c.Usages...)
	}
	result := make([]ColumnFact, 0, len(order))
	for _, k := range order {
		result = append(result, ColumnFact{
			Schema: k.schema, Table: k.table, Column: k.column,
			Usages: deduplicateStrings(merged[k]),
		})
	}
	return result
}

func deduplicateStrings(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// lineageEntry maps a virtual column name to its physical source keys.
// e.g. for CTE "x AS (SELECT id FROM users)", lineage["x"]["id"] = ["users.id"]
type lineageEntry map[string][]string

type scopeStack struct {
	defaultSchema string
	aliases       map[string]resolvedTable
	ctes          map[string]bool
	relations     []resolvedTable
	lineage       map[string]lineageEntry // relation name → column → []physical sources
}

type resolvedTable struct {
	schema string
	name   string
}

func newScopeStack(defaultSchema string) *scopeStack {
	return &scopeStack{
		defaultSchema: defaultSchema,
		aliases:       make(map[string]resolvedTable),
		ctes:          make(map[string]bool),
		relations:     make([]resolvedTable, 0),
		lineage:       make(map[string]lineageEntry),
	}
}

func (s *scopeStack) addAlias(alias, schema, name string) {
	s.aliases[strings.ToLower(alias)] = resolvedTable{schema: schema, name: name}
}

func (s *scopeStack) addCTE(name string) {
	if name != "" {
		s.ctes[strings.ToLower(name)] = true
	}
}

func (s *scopeStack) addRelation(schema, name string) {
	s.relations = append(s.relations, resolvedTable{schema: schema, name: name})
}

func (s *scopeStack) addLineage(relationName string, entry lineageEntry) {
	if relationName != "" && entry != nil {
		s.lineage[strings.ToLower(relationName)] = entry
	}
}

func (s *scopeStack) lookupLineage(relationName, columnName string) ([]string, bool) {
	entry, ok := s.lineage[strings.ToLower(relationName)]
	if !ok {
		return nil, false
	}
	sources, ok := entry[strings.ToLower(columnName)]
	return sources, ok
}

func (s *scopeStack) resolveColumn(tableQualifier, schemaQualifier string) (schema, table string) {
	if tableQualifier == "" {
		return "", ""
	}
	lower := strings.ToLower(tableQualifier)
	if resolved, ok := s.aliases[lower]; ok {
		return resolved.schema, resolved.name
	}
	if s.ctes[lower] {
		return "", tableQualifier
	}
	for _, rel := range s.relations {
		if strings.EqualFold(rel.name, tableQualifier) {
			return rel.schema, rel.name
		}
	}
	schema = schemaQualifier
	if schema == "" {
		schema = s.defaultSchema
	}
	return schema, tableQualifier
}

func (s *scopeStack) findColumnInRelations() (schema, table string) {
	if len(s.relations) == 1 {
		return s.relations[0].schema, s.relations[0].name
	}
	return "", ""
}
