package tidbparser

import "github.com/pingcap/tidb/pkg/parser/ast"

func (c *effectCandidateCollector) appendFunctionCall(call *ast.FuncCallExpr, scope *scopeStack) {
	namePath, originalPath, explicitSchema := candidateNamePath(call.Schema, call.FnName)
	class := "generic"
	if call.Tp == ast.FuncCallExprTypeKeyword {
		class = "keyword"
	}
	factClass := class
	if class == "keyword" && nativeScalarKeyword(namePath) {
		factClass = "native_scalar_keyword"
	}
	quoted, canonical, ambiguous := candidateCallFacts(c.callSourceText(call), originalPath, explicitSchema, factClass)
	if c.quotedCall(call) {
		quoted = true
		canonical = false
		ambiguous = true
	}
	c.append(EffectCandidate{
		Kind:                 EffectCandidateFunction,
		NamePath:             namePath,
		OriginalNamePath:     originalPath,
		ExplicitSchema:       explicitSchema,
		IsQuoted:             quoted,
		Canonical:            canonical,
		Ambiguous:            ambiguous,
		ParserClassification: class,
		Arity:                functionArity(call.Args),
		OperandKinds:         functionOperandKinds(call.Args),
		OperandColumnRefs:    functionColumnRefs(call.Args, scope),
	})
}

func nativeScalarKeyword(namePath []string) bool {
	if len(namePath) != 1 {
		return false
	}
	switch namePath[0] {
	case "lower", "upper", "length", "char_length", "abs", "ceil", "ceiling", "floor", "coalesce", "nullif", "ifnull":
		return true
	default:
		return false
	}
}

func (c *effectCandidateCollector) appendAggregate(aggregate *ast.AggregateFuncExpr, scope *scopeStack) {
	name := ast.NewCIStr(aggregate.F)
	namePath, originalPath, explicitSchema := candidateNamePath(ast.CIStr{}, name)
	operandKinds := functionOperandKinds(aggregate.Args)
	arity := functionArity(aggregate.Args)
	if len(aggregate.Args) == 1 && aggregate.Args[0].OriginTextPosition() == 0 {
		operandKinds[0] = OperandKindStar
		arity = 0
	}
	quoted, canonical, ambiguous := candidateCallFacts(c.callSourceText(aggregate), originalPath, explicitSchema, "aggregate")
	c.append(EffectCandidate{
		Kind:                 EffectCandidateFunction,
		NamePath:             namePath,
		OriginalNamePath:     originalPath,
		ExplicitSchema:       explicitSchema,
		IsQuoted:             quoted,
		Canonical:            canonical,
		Ambiguous:            ambiguous,
		ParserClassification: "aggregate",
		Arity:                arity,
		OperandKinds:         operandKinds,
		OperandColumnRefs:    functionColumnRefs(aggregate.Args, scope),
		IsAggregate:          true,
		HasDistinct:          aggregate.Distinct,
		HasAggOrder:          aggregate.Order != nil,
	})
}

func (c *effectCandidateCollector) appendWindow(window *ast.WindowFuncExpr, scope *scopeStack) {
	name := ast.NewCIStr(window.Name)
	namePath, originalPath, explicitSchema := candidateNamePath(ast.CIStr{}, name)
	partitionKinds, partitionRefs := partitionClauseFacts(window.Spec.PartitionBy, scope)
	orderKinds, orderRefs := windowClauseFacts(window.Spec.OrderBy, scope)
	frameKinds := windowFrameKinds(window.Spec.Frame)
	quoted, canonical, ambiguous := candidateCallFacts(c.callSourceText(window), originalPath, explicitSchema, "window")
	if c.quotedCall(window) {
		quoted = true
		canonical = false
		ambiguous = true
	}
	c.append(EffectCandidate{
		Kind:                      EffectCandidateFunction,
		NamePath:                  namePath,
		OriginalNamePath:          originalPath,
		ExplicitSchema:            explicitSchema,
		IsQuoted:                  quoted,
		Canonical:                 canonical,
		Ambiguous:                 ambiguous,
		ParserClassification:      "window",
		Arity:                     functionArity(window.Args),
		OperandKinds:              functionOperandKinds(window.Args),
		OperandColumnRefs:         functionColumnRefs(window.Args, scope),
		HasWindow:                 true,
		HasDistinct:               window.Distinct,
		HasFrame:                  window.Spec.Frame != nil,
		HasNamedWindow:            window.Spec.Name.L != "" || window.Spec.Ref.L != "" || window.Spec.OnlyAlias,
		HasWindowPartition:        window.Spec.PartitionBy != nil,
		HasWindowOrder:            window.Spec.OrderBy != nil,
		WindowPartitionKinds:      partitionKinds,
		WindowOrderKinds:          orderKinds,
		WindowFrameKinds:          frameKinds,
		WindowPartitionColumnRefs: partitionRefs,
		WindowOrderColumnRefs:     orderRefs,
	})
}

func (c *effectCandidateCollector) appendCast(cast *ast.FuncCastExpr, scope *scopeStack) {
	path := []string(nil)
	if cast.Tp != nil {
		path = []string{cast.Tp.String()}
	}
	c.append(EffectCandidate{
		Kind:                 EffectCandidateCast,
		Canonical:            false,
		Ambiguous:            true,
		ParserClassification: "cast",
		Arity:                1,
		OperandKinds:         []OperandKindHint{candidateOperandKind(cast.Expr)},
		OperandColumnRefs:    functionColumnRefs([]ast.ExprNode{cast.Expr}, scope),
		TargetTypePath:       path,
	})
}

func (c *effectCandidateCollector) appendSpecialFunction(name string, expr ast.ExprNode, scope *scopeStack, originalText string) {
	if originalText == "" {
		originalText = name + "("
	}
	quoted, canonical, ambiguous := candidateCallFacts(originalText, []string{name}, false, "special")
	c.append(EffectCandidate{
		Kind:                 EffectCandidateFunction,
		NamePath:             []string{name},
		OriginalNamePath:     []string{name},
		IsQuoted:             quoted,
		Canonical:            canonical,
		Ambiguous:            ambiguous,
		ParserClassification: "special",
		Arity:                1,
		OperandKinds:         []OperandKindHint{candidateOperandKind(expr)},
		OperandColumnRefs:    functionColumnRefs([]ast.ExprNode{expr}, scope),
	})
}

func functionArity(args []ast.ExprNode) int {
	if len(args) == 1 && candidateOperandKind(args[0]) == OperandKindStar {
		return 0
	}
	return len(args)
}

func functionOperandKinds(args []ast.ExprNode) []OperandKindHint {
	result := make([]OperandKindHint, 0, len(args))
	for _, arg := range args {
		result = append(result, candidateOperandKind(arg))
	}
	return result
}

func functionColumnRefs(args []ast.ExprNode, scope *scopeStack) []OperandColumnRef {
	refs := make([]OperandColumnRef, 0, len(args))
	for _, arg := range args {
		if ref, ok := candidateColumnRef(arg, scope); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func windowClauseFacts(clause *ast.OrderByClause, scope *scopeStack) ([]OperandKindHint, []OperandColumnRef) {
	if clause == nil {
		return nil, nil
	}
	kinds := make([]OperandKindHint, 0, len(clause.Items))
	refs := make([]OperandColumnRef, 0, len(clause.Items))
	for _, item := range clause.Items {
		if item == nil {
			continue
		}
		kinds = append(kinds, candidateOperandKind(item.Expr))
		if ref, ok := candidateColumnRef(item.Expr, scope); ok {
			refs = append(refs, ref)
		}
	}
	return kinds, refs
}

func partitionClauseFacts(clause *ast.PartitionByClause, scope *scopeStack) ([]OperandKindHint, []OperandColumnRef) {
	if clause == nil {
		return nil, nil
	}
	kinds := make([]OperandKindHint, 0, len(clause.Items))
	refs := make([]OperandColumnRef, 0, len(clause.Items))
	for _, item := range clause.Items {
		if item == nil {
			continue
		}
		kinds = append(kinds, candidateOperandKind(item.Expr))
		if ref, ok := candidateColumnRef(item.Expr, scope); ok {
			refs = append(refs, ref)
		}
	}
	return kinds, refs
}

func windowFrameKinds(frame *ast.FrameClause) []OperandKindHint {
	if frame == nil {
		return nil
	}
	return []OperandKindHint{candidateOperandKind(frame.Extent.Start.Expr), candidateOperandKind(frame.Extent.End.Expr)}
}

func candidateColumnRef(expr ast.ExprNode, scope *scopeStack) (OperandColumnRef, bool) {
	column, ok := expr.(*ast.ColumnNameExpr)
	if !ok || column.Name == nil || column.Name.Name.L == "*" || scope == nil {
		return OperandColumnRef{}, false
	}
	name := column.Name
	if name.Table.L != "" {
		if sources, found := scope.lookupLineage(name.Table.L, name.Name.L); found && len(sources) > 0 {
			schema, table := parseSourceTable(sources[0])
			return OperandColumnRef{Schema: schema, Table: table, Column: name.Name.L}, table != ""
		}
	}
	schema, table := "", ""
	if name.Table.L != "" {
		schema, table = scope.resolveColumn(name.Table.L, name.Schema.L)
	}
	if table == "" {
		schema, table = scope.findColumnInRelations()
	}
	if table == "" {
		return OperandColumnRef{}, false
	}
	return OperandColumnRef{Schema: schema, Table: table, Column: name.Name.L}, true
}

func cloneScope(source *scopeStack) *scopeStack {
	if source == nil {
		return newScopeStack("")
	}
	copy := newScopeStack(source.defaultSchema)
	for name, table := range source.aliases {
		copy.aliases[name] = table
	}
	for name := range source.ctes {
		copy.ctes[name] = true
	}
	copy.relations = append(copy.relations, source.relations...)
	for relation, entry := range source.lineage {
		copyEntry := make(lineageEntry)
		for column, sources := range entry {
			copyEntry[column] = append([]string(nil), sources...)
		}
		copy.lineage[relation] = copyEntry
	}
	return copy
}
