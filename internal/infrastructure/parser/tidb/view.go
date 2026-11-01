package tidbparser

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
	"github.com/pingcap/tidb/pkg/parser/format"
)

type viewStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	target      spec.Table
	declaration *spec.ViewDeclaration
}

type viewHeader struct {
	operation   spec.DDLOperation
	target      spec.Table
	declaration *spec.ViewDeclaration
	selectStart int
	selectEnd   int
}

func unmodeledViewOptions(view *spec.ViewDeclaration) []string {
	var options []string
	if view.Definition == nil || !view.Definition.ShapeKnown || !view.Definition.SourcesKnown {
		options = append(options, "view_query_shape")
	} else if len(view.Columns) == 0 {
		unnamed := 0
		for _, binding := range view.Definition.Bindings {
			if !binding.Star && binding.Output == "" {
				unnamed++
			}
		}
		if view.Definition.UnlabeledComplex || unnamed > 0 && len(view.Definition.Bindings) > 1 {
			options = append(options, "view_output_label")
		}
	}
	return options
}

func parseViewIdentifier(text string, components int) (string, string, bool) {
	nodes, _, err := parser.New().Parse("SELECT "+text, "", "")
	if err != nil || len(nodes) != 1 {
		return "", "", false
	}
	query, ok := nodes[0].(*ast.SelectStmt)
	if !ok || query.Fields == nil || len(query.Fields.Fields) != 1 || query.From != nil {
		return "", "", false
	}
	field := query.Fields.Fields[0]
	column, ok := field.Expr.(*ast.ColumnNameExpr)
	if !ok || column.Name == nil || field.AsName.L != "" || column.Name.Schema.L != "" || column.Name.Name.L == "" {
		return "", "", false
	}
	if components == 1 && column.Name.Table.L == "" {
		return "", column.Name.Name.L, true
	}
	if components == 2 && column.Name.Table.L != "" {
		return column.Name.Table.L, column.Name.Name.L, true
	}
	return "", "", false
}

func parseViewDefiner(text string) bool {
	tokens := tokenizeSQL(text)
	if len(tokens) == 0 || tokens[len(tokens)-1].text(text) == "@" {
		return false
	}
	nodes, _, err := parser.New().Parse("SHOW GRANTS FOR "+text, "", "")
	if err != nil || len(nodes) != 1 {
		return false
	}
	show, ok := nodes[0].(*ast.ShowStmt)
	return ok && show.Tp == ast.ShowGrants && show.User != nil && len(show.Roles) == 0 &&
		show.Where == nil && show.Pattern == nil && show.Limit == nil
}

func viewDropTargetOffsets(sql string) []int {
	tokens := tokenizeSQL(sql)
	i := 0
	for i < len(tokens) && !tokens[i].wordIs(sql, "VIEW") {
		i++
	}
	i++
	if i < len(tokens) && tokens[i].wordIs(sql, "IF") {
		i += 2
	}
	var offsets []int
	for i < len(tokens) && tokens[i].text(sql) != ";" {
		offsets = append(offsets, tokens[i].start)
		for i < len(tokens) && tokens[i].text(sql) != "," && tokens[i].text(sql) != ";" {
			i++
		}
		if i < len(tokens) && tokens[i].text(sql) == "," {
			i++
		}
	}
	return offsets
}

func parseViewHeader(sql string) (*viewHeader, bool) {
	tokens := tokenizeSQL(sql)
	if len(tokens) < 5 {
		return nil, false
	}
	i := 0
	take := func(word string) bool {
		if i < len(tokens) && tokens[i].wordIs(sql, word) {
			i++
			return true
		}
		return false
	}
	punct := func(value string) bool {
		if i < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text(sql) == value {
			i++
			return true
		}
		return false
	}
	identifierToken := func() (sqlTok, bool) {
		if i >= len(tokens) || (tokens[i].kind != tokWord && tokens[i].kind != tokQIdent) {
			return sqlTok{}, false
		}
		token := tokens[i]
		raw := token.text(sql)
		if token.kind == tokQIdent && (len(raw) < 2 || raw[len(raw)-1] != '`') {
			return sqlTok{}, false
		}
		i++
		return token, true
	}
	h := &viewHeader{declaration: &spec.ViewDeclaration{}}
	switch {
	case take("ALTER"):
		h.operation = spec.DDLOperationAlterView
	case take("CREATE"):
		h.operation = spec.DDLOperationCreateView
		if take("OR") {
			if !take("REPLACE") {
				return nil, false
			}
			h.declaration.OrReplace = true
		}
	default:
		return nil, false
	}
	if i < len(tokens) && tokens[i].wordIs(sql, "ALGORITHM") {
		h.declaration.AlgorithmOffset = tokens[i].start
	}
	if take("ALGORITHM") {
		if !punct("=") {
			return nil, false
		}
		switch {
		case take("UNDEFINED"):
			h.declaration.Algorithm = "undefined"
		case take("MERGE"):
			h.declaration.Algorithm = "merge"
		case take("TEMPTABLE"):
			h.declaration.Algorithm = "temptable"
		default:
			return nil, false
		}
		h.declaration.AlgorithmExplicit = true
	}
	if i < len(tokens) && tokens[i].wordIs(sql, "DEFINER") {
		h.declaration.DefinerOffset = tokens[i].start
	}
	if take("DEFINER") {
		if !punct("=") || i >= len(tokens) {
			return nil, false
		}
		if take("CURRENT_USER") {
			h.declaration.DefinerKind = "current_user"
			if punct("(") && !punct(")") {
				return nil, false
			}
		} else {
			start := tokens[i].start
			startIndex := i
			for i < len(tokens) {
				clause := tokens[i].wordIs(sql, "VIEW") ||
					tokens[i].wordIs(sql, "SQL") && i+1 < len(tokens) && tokens[i+1].wordIs(sql, "SECURITY")
				if clause && i > startIndex && parseViewDefiner(sql[start:tokens[i-1].end]) {
					break
				}
				i++
			}
			if i >= len(tokens) || i == startIndex || tokens[i-1].end <= start {
				return nil, false
			}
			end := tokens[i-1].end
			if !parseViewDefiner(sql[start:end]) {
				return nil, false
			}
			h.declaration.DefinerKind = "named"
			h.declaration.DefinerPrincipal = sql[start:end]
		}
	}
	if i < len(tokens) && tokens[i].wordIs(sql, "SQL") {
		h.declaration.SecurityOffset = tokens[i].start
	}
	if take("SQL") {
		if !take("SECURITY") {
			return nil, false
		}
		switch {
		case take("DEFINER"):
			h.declaration.Security = "definer"
		case take("INVOKER"):
			h.declaration.Security = "invoker"
		default:
			return nil, false
		}
		h.declaration.SecurityExplicit = true
	}
	if !take("VIEW") {
		return nil, false
	}
	first, valid := identifierToken()
	if !valid {
		return nil, false
	}
	if punct(".") {
		second, ok := identifierToken()
		if !ok {
			return nil, false
		}
		h.target.Schema, h.target.Name, valid = parseViewIdentifier(sql[first.start:second.end], 2)
	} else {
		_, h.target.Name, valid = parseViewIdentifier(first.text(sql), 1)
	}
	if !valid {
		return nil, false
	}
	if punct("(") {
		for {
			token, ok := identifierToken()
			if !ok {
				return nil, false
			}
			_, name, ok := parseViewIdentifier(token.text(sql), 1)
			if !ok {
				return nil, false
			}
			h.declaration.Columns = append(h.declaration.Columns, name)
			if punct(")") {
				break
			}
			if !punct(",") {
				return nil, false
			}
		}
	}
	if !take("AS") || i >= len(tokens) {
		return nil, false
	}
	h.selectStart = tokens[i].start
	end := len(tokens)
	if end > i && tokens[end-1].text(sql) == ";" {
		end--
	}
	for j := i; j < end; j++ {
		if tokens[j].kind == tokPunct && tokens[j].text(sql) == ";" {
			return nil, false
		}
	}
	if end <= i {
		return nil, false
	}
	h.selectEnd = tokens[end-1].end
	checkStart := end - 3
	if end-i >= 3 && tokens[end-1].wordIs(sql, "OPTION") && tokens[end-2].wordIs(sql, "CHECK") {
		if checkStart > i && (tokens[checkStart].wordIs(sql, "LOCAL") || tokens[checkStart].wordIs(sql, "CASCADED")) {
			h.declaration.CheckOption = strings.ToLower(tokens[checkStart].text(sql))
			checkStart--
		} else {
			h.declaration.CheckOption = "cascaded"
			h.declaration.CheckBare = true
		}
		if checkStart <= i || !tokens[checkStart].wordIs(sql, "WITH") {
			return nil, false
		}
		h.declaration.CheckExplicit = true
		h.declaration.CheckOffset = tokens[checkStart].start
		h.selectEnd = tokens[checkStart].start
	}
	return h, true
}

func parseViewQuery(ctx context.Context, sql string) (ast.StmtNode, []error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	nodes, warnings, err := parser.New().Parse(sql, "", "")
	if err != nil {
		return nil, nil, err
	}
	if len(nodes) != 1 {
		return nil, nil, fmt.Errorf("view definition must contain one query")
	}
	if _, ok := nodes[0].(ast.ResultSetNode); !ok {
		return nil, nil, fmt.Errorf("view definition is not a query")
	}
	if selectNode, ok := nodes[0].(*ast.SelectStmt); ok && selectNode.SelectIntoOpt != nil {
		return nil, nil, fmt.Errorf("view definition is not a query")
	}
	return nodes[0], warnings, nil
}

func viewDefinition(node ast.StmtNode, sql string, selectStart, offsetBase int, locations bool) *spec.ViewDefinition {
	query, ok := node.(ast.ResultSetNode)
	if !ok {
		return nil
	}
	if needsViewQuery(query) {
		builder := &viewQueryBuilder{
			sql: sql, selectStart: selectStart, offsetBase: offsetBase, locations: locations,
			usedOffsets: make(map[int]bool),
		}
		root := builder.build(query, nil, 0)
		definition := &spec.ViewDefinition{
			Query: root, HasCTE: builder.hasCTE, SourcesKnown: true, ShapeKnown: !root.Unsupported,
			Bindings:         append([]spec.ViewColumnBinding(nil), root.Bindings...),
			References:       append([]spec.ViewColumnBinding(nil), root.References...),
			UnlabeledComplex: root.Unlabeled, NonMergeable: root.NonMergeable, NonUpdatable: root.NonUpdatable,
		}
		var restored strings.Builder
		if err := node.Restore(format.NewRestoreCtx(format.DefaultRestoreFlags, &restored)); err == nil {
			definition.SQL = restored.String()
		}
		return spec.SelectViewDefinition(definition, spec.DialectMySQL)
	}
	facts := analyzeQueryNode(query, newScopeStack(""))
	ctes := make(map[string]bool)
	for _, relation := range facts.relations {
		if relation.Kind == "cte" {
			ctes[strings.ToLower(relation.Name)] = true
		}
	}
	definition := &spec.ViewDefinition{SourcesKnown: len(ctes) == 0}
	for _, source := range selectSourceTables(query) {
		if source.Schema == "" && ctes[strings.ToLower(source.Name)] {
			continue
		}
		definition.Sources = append(definition.Sources, source)
	}
	definition.SourceAliases = make([]string, len(definition.Sources))
	if locations {
		definition.SourceOffsets = make([]int, len(definition.Sources))
		for i := range definition.SourceOffsets {
			definition.SourceOffsets[i] = -1
		}
	}
	var restored strings.Builder
	if err := node.Restore(format.NewRestoreCtx(format.DefaultRestoreFlags, &restored)); err == nil {
		definition.SQL = restored.String()
	}
	selectNode, ok := node.(*ast.SelectStmt)
	if !ok || selectNode.Fields == nil {
		return definition
	}
	if selectNode.From != nil {
		if selectNode.From.TableRefs == nil || selectNode.From.TableRefs.Right != nil {
			return definition
		}
		source, ok := selectNode.From.TableRefs.Left.(*ast.TableSource)
		if !ok {
			return definition
		}
		if _, ok := source.Source.(*ast.TableName); !ok {
			return definition
		}
		if len(definition.Sources) == 1 {
			definition.SourceAliases[0] = source.AsName.L
		}
		if locations && len(definition.Sources) == 1 {
			tokens := tokenizeSQL(sql)
			depth := 0
			for i, token := range tokens {
				if token.start < selectStart {
					continue
				}
				switch token.text(sql) {
				case "(":
					depth++
				case ")":
					depth--
				}
				if depth == 0 && token.wordIs(sql, "FROM") && i+1 < len(tokens) {
					definition.SourceOffsets[0] = tokens[i+1].start
					break
				}
			}
		}
	} else if len(definition.Sources) != 0 {
		return definition
	}
	if selectNode.From == nil {
		definition.NonMergeable = true
		definition.NonUpdatable = true
	}
	if selectNode.With != nil || selectNode.GroupBy != nil ||
		selectNode.Having != nil || selectNode.LockInfo != nil || selectNode.SelectIntoOpt != nil ||
		selectNode.Limit != nil || selectNode.Distinct || len(selectNode.WindowSpecs) != 0 {
		return definition
	}
	definition.ShapeKnown = true
	for _, field := range selectNode.Fields.Fields {
		if field.WildCard != nil {
			offset := -1
			if locations && field.Offset >= 0 && offsetBase+field.Offset < len(sql) {
				offset = offsetBase + field.Offset
			}
			definition.Bindings = append(definition.Bindings, spec.ViewColumnBinding{
				Schema: field.WildCard.Schema.L, Table: field.WildCard.Table.L, Offset: offset, Star: true,
			})
			continue
		}
		offset := -1
		if locations && field.Offset >= 0 && offsetBase+field.Offset < len(sql) {
			offset = offsetBase + field.Offset
		}
		column, isColumn := field.Expr.(*ast.ColumnNameExpr)
		if isColumn && column.Name != nil {
			output := field.AsName.L
			if output == "" {
				output = column.Name.Name.L
			}
			definition.Bindings = append(definition.Bindings, spec.ViewColumnBinding{
				Schema: column.Name.Schema.L, Table: column.Name.Table.L,
				Column: column.Name.Name.L, Output: output, Offset: offset,
			})
			continue
		}
		if field.Expr == nil {
			definition.ShapeKnown = false
			break
		}
		collector := &viewReferenceCollector{}
		field.Expr.Accept(collector)
		if collector.unsupported {
			definition.ShapeKnown = false
			break
		}
		if collector.aggregate {
			definition.NonMergeable = true
			definition.NonUpdatable = true
		}
		for i := range collector.references {
			collector.references[i].Offset = offset
		}
		definition.References = append(definition.References, collector.references...)
		if field.AsName.L == "" {
			if _, literal := field.Expr.(ast.ValueExpr); !literal {
				definition.UnlabeledComplex = true
			}
		}
		definition.Bindings = append(definition.Bindings, spec.ViewColumnBinding{
			Output: field.AsName.L, Offset: offset, Expression: true,
		})
	}
	if definition.ShapeKnown && selectNode.Where != nil {
		collector := &viewReferenceCollector{sql: sql, start: selectStart, locations: locations}
		selectNode.Where.Accept(collector)
		if collector.unsupported {
			definition.ShapeKnown = false
		} else {
			definition.References = append(definition.References, collector.references...)
		}
	}
	return definition
}

type viewReferenceCollector struct {
	sql         string
	start       int
	locations   bool
	unsupported bool
	aggregate   bool
	references  []spec.ViewColumnBinding
}

func (c *viewReferenceCollector) Enter(node ast.Node) (ast.Node, bool) {
	switch node := node.(type) {
	case *ast.AggregateFuncExpr:
		c.aggregate = true
	case *ast.SubqueryExpr, *ast.WindowFuncExpr, *ast.PositionExpr:
		c.unsupported = true
		return node, true
	case *ast.ColumnNameExpr:
		if node.Name != nil {
			offset := -1
			if c.locations {
				offset = viewReferenceOffset(c.sql, c.start, node.Name, "WHERE")
			}
			c.references = append(c.references, spec.ViewColumnBinding{
				Schema: node.Name.Schema.L, Table: node.Name.Table.L, Column: node.Name.Name.L, Offset: offset,
			})
		}
		return node, true
	}
	return node, false
}

func (c *viewReferenceCollector) Leave(node ast.Node) (ast.Node, bool) { return node, true }

func viewReferenceOffset(sql string, start int, name *ast.ColumnName, clause string) int {
	var parts []string
	if name.Schema.L != "" {
		parts = append(parts, name.Schema.L)
	}
	if name.Table.L != "" {
		parts = append(parts, name.Table.L)
	}
	parts = append(parts, name.Name.L)
	tokens := tokenizeSQL(sql)
	current := ""
	offset := -1
	for i, token := range tokens {
		if token.start < start {
			continue
		}
		switch {
		case token.wordIs(sql, "ON"):
			current = "ON"
		case token.wordIs(sql, "WHERE"):
			current = "WHERE"
		case token.wordIs(sql, "HAVING"):
			current = "HAVING"
		case token.wordIs(sql, "GROUP") && i+1 < len(tokens) && tokens[i+1].wordIs(sql, "BY"):
			current = "GROUP"
		case token.wordIs(sql, "ORDER") && i+1 < len(tokens) && tokens[i+1].wordIs(sql, "BY"):
			current = "ORDER"
		case token.wordIs(sql, "FROM"), token.wordIs(sql, "JOIN"), token.wordIs(sql, "LIMIT"),
			token.wordIs(sql, "UNION"):
			current = ""
		}
		if current != clause || i > 0 && tokens[i-1].text(sql) == "." {
			continue
		}
		j := i
		matched := true
		for partIndex, part := range parts {
			if partIndex > 0 {
				if j >= len(tokens) || tokens[j].text(sql) != "." {
					matched = false
					break
				}
				j++
			}
			if j >= len(tokens) || tokens[j].kind != tokWord && tokens[j].kind != tokQIdent ||
				tokens[j].identText(sql) != part {
				matched = false
				break
			}
			j++
		}
		if matched {
			if offset >= 0 {
				return -1
			}
			offset = token.start
		}
	}
	return offset
}

func ParseViewDefinition(ctx context.Context, sqlText string) (*spec.ViewDefinition, error) {
	node, _, err := parseViewQuery(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	return viewDefinition(node, sqlText, 0, 0, false), nil
}

func parseViewAdapter(ctx context.Context, sql string) (*viewStatement, []error, bool) {
	header, ok := parseViewHeader(sql)
	if !ok {
		return nil, nil, false
	}
	query, warnings, err := parseViewQuery(ctx, sql[header.selectStart:header.selectEnd])
	if err != nil {
		return nil, nil, false
	}
	header.declaration.Definition = viewDefinition(query, sql, header.selectStart, header.selectStart, true)
	query.SetText(charset.FindEncoding(""), strings.TrimSpace(sql))
	query.SetOriginTextPosition(0)
	return &viewStatement{StmtNode: query, operation: header.operation, target: header.target, declaration: header.declaration}, warnings, true
}
