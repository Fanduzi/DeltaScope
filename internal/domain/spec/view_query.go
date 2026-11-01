package spec

import "strings"

type viewQueryResult struct {
	outputs    []string
	countKnown bool
}

type viewQueryAssessment struct {
	outputs    []string
	countKnown bool
	failures   []ViewSourceFailure
	required   []string
	limited    bool
}

type viewQueryRelation struct {
	identity       Table
	alias          string
	columns        []Column
	columnsKnown   bool
	columnsUnknown bool
	missing        bool
	root           int
}

type viewQueryEvaluator struct {
	sources []TargetBinding
	dialect Dialect
	result  viewQueryAssessment
	active  map[*ViewQuery]bool
	visits  int
	freeze  bool
}

func assessViewQuery(query *ViewQuery, sources []TargetBinding, dialect Dialect) viewQueryAssessment {
	evaluator := &viewQueryEvaluator{sources: sources, dialect: dialect, active: make(map[*ViewQuery]bool)}
	result := evaluator.evaluate(query, nil, 0)
	evaluator.result.outputs = result.outputs
	evaluator.result.countKnown = result.countKnown
	return evaluator.result
}

func FreezeViewDefinition(in *ViewDefinition, sources []TargetBinding, dialect Dialect) *ViewDefinition {
	out := CloneViewDefinition(in)
	if out == nil || out.Query == nil {
		return out
	}
	evaluator := &viewQueryEvaluator{
		sources: sources, dialect: dialect, active: make(map[*ViewQuery]bool), freeze: true,
	}
	evaluator.evaluate(out.Query, nil, 0)
	return out
}

func (e *viewQueryEvaluator) invalid(root, offset int) {
	for _, failure := range e.result.failures {
		if failure.Root == root {
			return
		}
	}
	e.result.failures = append(e.result.failures, ViewSourceFailure{Root: root, Offset: offset})
}

func (e *viewQueryEvaluator) require(fact string) {
	e.result.required = appendViewFact(e.result.required, fact)
}

func (e *viewQueryEvaluator) evaluate(query *ViewQuery, outer []viewQueryRelation, depth int) viewQueryResult {
	if query == nil {
		return viewQueryResult{}
	}
	e.visits++
	if depth > 32 || e.visits > 1024 || query.Limited || e.active[query] {
		e.result.limited = true
		return viewQueryResult{}
	}
	e.active[query] = true
	defer delete(e.active, query)
	if e.dialect == DialectTiDB {
		for _, body := range query.With {
			e.evaluate(body, nil, depth+1)
		}
	}
	if len(query.SetOperands) > 0 {
		var result viewQueryResult
		for i, operand := range query.SetOperands {
			next := e.evaluate(operand, outer, depth+1)
			if i == 0 {
				result = next
			} else if result.countKnown && next.countKnown && len(result.outputs) != len(next.outputs) {
				e.invalid(viewQueryFirstRoot(operand), -1)
			} else if !next.countKnown {
				result.countKnown = false
			}
		}
		return e.renameOutputs(query, result)
	}
	relations := make([]viewQueryRelation, 0, len(query.Sources))
	for _, source := range query.Sources {
		relation := viewQueryRelation{alias: source.Alias, root: source.Index}
		if source.Table != nil {
			relation.identity = *source.Table
			if source.Index < 0 || source.Index >= len(e.sources) {
				relations = append(relations, relation)
				continue
			}
			binding := e.sources[source.Index]
			relation.identity = binding.Table
			snapshot := binding.Snapshot
			if snapshot == nil {
				relations = append(relations, relation)
				continue
			}
			if !snapshot.Exists || snapshot.Temporary {
				relation.missing = true
			} else if snapshot.Columns == nil || snapshot.ColumnsTailUnknown {
				relation.columnsUnknown = true
			} else {
				relation.columnsKnown = true
				relation.columns = snapshot.Columns
			}
		} else if source.Query != nil {
			child := e.evaluate(source.Query, nil, depth+1)
			relation.root = viewQueryFirstRoot(source.Query)
			if child.countKnown {
				relation.columnsKnown = true
				for _, name := range child.outputs {
					if name == "" {
						relation.columnsKnown = false
						relation.columnsUnknown = true
					}
					relation.columns = append(relation.columns, Column{Name: name})
				}
			}
		}
		relations = append(relations, relation)
	}
	for _, binding := range query.References {
		e.resolveColumn(binding, relations, outer, query.Bindings)
	}
	result := viewQueryResult{countKnown: true}
	var frozen []ViewColumnBinding
	for _, binding := range query.Bindings {
		switch {
		case binding.Star:
			columns, expanded, known := e.expandStar(binding, relations)
			result.outputs = append(result.outputs, columns...)
			result.countKnown = result.countKnown && known
			if known {
				frozen = append(frozen, expanded...)
			} else {
				frozen = append(frozen, binding)
			}
		case binding.Expression:
			result.outputs = append(result.outputs, binding.Output)
			frozen = append(frozen, binding)
		default:
			e.resolveColumn(binding, relations, outer, nil)
			result.outputs = append(result.outputs, binding.Output)
			frozen = append(frozen, binding)
		}
	}
	if e.freeze {
		query.Bindings = frozen
	}
	correlated := append(append([]viewQueryRelation(nil), relations...), outer...)
	for _, subquery := range query.Subqueries {
		e.evaluate(subquery, correlated, depth+1)
	}
	return e.renameOutputs(query, result)
}

func (e *viewQueryEvaluator) renameOutputs(query *ViewQuery, result viewQueryResult) viewQueryResult {
	if result.countKnown && len(query.ColumnAliases) > 0 {
		if len(query.ColumnAliases) != len(result.outputs) {
			e.invalid(viewQueryFirstRoot(query), -1)
			return result
		}
		result.outputs = append([]string(nil), query.ColumnAliases...)
	}
	return result
}

func viewQueryFirstRoot(query *ViewQuery) int {
	visited := make(map[*ViewQuery]bool)
	var firstRoot func(*ViewQuery) int
	firstRoot = func(current *ViewQuery) int {
		if current == nil || visited[current] {
			return -1
		}
		visited[current] = true
		for _, source := range current.Sources {
			if source.Table != nil {
				return source.Index
			}
			if root := firstRoot(source.Query); root >= 0 {
				return root
			}
		}
		for _, operand := range current.SetOperands {
			if root := firstRoot(operand); root >= 0 {
				return root
			}
		}
		return -1
	}
	return firstRoot(query)
}

func viewQueryMatches(binding ViewColumnBinding, relation viewQueryRelation) bool {
	qualifier := relation.identity.Name
	if relation.alias != "" {
		qualifier = relation.alias
	}
	if binding.Table != "" && !strings.EqualFold(binding.Table, qualifier) {
		return false
	}
	if binding.Schema != "" && !strings.EqualFold(binding.Schema, relation.identity.Schema) {
		return false
	}
	return true
}

func (e *viewQueryEvaluator) resolveColumn(binding ViewColumnBinding, local, outer []viewQueryRelation, outputs []ViewColumnBinding) {
	if binding.Expression || binding.Star {
		return
	}
	matched := 0
	firstRoot := -1
	unknown, unknownColumns, missing, qualified := false, false, false, false
	for _, relation := range local {
		if !viewQueryMatches(binding, relation) {
			continue
		}
		qualified = true
		if firstRoot < 0 {
			firstRoot = relation.root
		}
		switch {
		case relation.missing:
			missing = true
		case !relation.columnsKnown:
			unknown = true
			unknownColumns = unknownColumns || relation.columnsUnknown
		default:
			for _, column := range relation.columns {
				if strings.EqualFold(column.Name, binding.Column) {
					matched++
					break
				}
			}
		}
	}
	if matched > 1 {
		e.invalid(firstRoot, binding.Offset)
		return
	}
	if unknownColumns {
		e.require("view.columns")
	}
	if matched == 1 || unknown || missing {
		return
	}
	if binding.OutputReference && binding.Table == "" && binding.Schema == "" {
		for _, output := range outputs {
			if output.Output != "" && strings.EqualFold(output.Output, binding.Column) {
				return
			}
		}
	}
	if (!qualified || binding.Table == "" && binding.Schema == "") && len(outer) > 0 {
		e.resolveColumn(binding, outer, nil, outputs)
		return
	}
	if firstRoot < 0 && len(local) > 0 {
		firstRoot = local[0].root
	}
	e.invalid(firstRoot, binding.Offset)
}

func (e *viewQueryEvaluator) expandStar(binding ViewColumnBinding, relations []viewQueryRelation) ([]string, []ViewColumnBinding, bool) {
	var names []string
	var expanded []ViewColumnBinding
	countKnown := true
	matched := 0
	firstRoot := -1
	for _, relation := range relations {
		if !viewQueryMatches(binding, relation) {
			continue
		}
		matched++
		if firstRoot < 0 {
			firstRoot = relation.root
		}
		if !relation.columnsKnown {
			countKnown = false
			if relation.columnsUnknown {
				e.require("view.columns")
			}
			continue
		}
		for _, column := range relation.columns {
			if !strings.EqualFold(column.Visibility, "invisible") {
				names = append(names, column.Name)
				qualifier := relation.identity.Name
				if relation.alias != "" {
					qualifier = relation.alias
				}
				expanded = append(expanded, ViewColumnBinding{Table: qualifier, Column: column.Name, Output: column.Name, Offset: binding.Offset})
			}
		}
	}
	if matched == 0 || binding.Table != "" && matched > 1 {
		if firstRoot < 0 && len(relations) > 0 {
			firstRoot = relations[0].root
		}
		e.invalid(firstRoot, binding.Offset)
		countKnown = false
	}
	return names, expanded, countKnown
}
