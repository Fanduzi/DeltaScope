// Package queryaccess implements metadata-backed resolution for query access analysis.
// input: domain Result with extracted facts, SchemaResolver for metadata lookup
// output: enriched Result with resolved wildcards, columns, aliases, and lineage
// pos: application resolution layer bridging extracted facts to metadata-resolved results
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"strings"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// UnresolvedReason constants for bounded unresolved tracking.
const (
	ReasonMissingMetadata    domain.ReasonCode = "missing_metadata"
	ReasonRelationNotFound   domain.ReasonCode = "relation_not_found"
	ReasonColumnNotFound     domain.ReasonCode = "column_not_found"
	ReasonAmbiguousColumn    domain.ReasonCode = "ambiguous_column"
	ReasonRelationAmbiguous  domain.ReasonCode = "relation_ambiguous"
	ReasonUnresolvedWildcard domain.ReasonCode = "unresolved_wildcard"
	ReasonUnresolvedAlias    domain.ReasonCode = "unresolved_alias"
)

// resolutionState holds request-scoped caches and resolver context.
type resolutionState struct {
	ctx           context.Context
	resolver      SchemaResolver
	dialect       string
	defaultSchema string
	schemaCache   map[string]RelationSchema // key: canonical schema.name
	aliasMap      map[string]resolvedRef    // key: alias → relation
	nameMap       map[string][]resolvedRef  // key: relation name → matching relations
	relationOrder []resolvedRef             // preserves SQL FROM/JOIN order for wildcard expansion
	unboundMap    map[string]bool           // key: alias or name → unbound relation
}

type resolvedRef struct {
	schema string
	name   string
}

func newResolutionState(ctx context.Context, resolver SchemaResolver, dialect, defaultSchema string, relations []domain.RelationReference) *resolutionState {
	s := &resolutionState{
		ctx:           ctx,
		resolver:      resolver,
		dialect:       dialect,
		defaultSchema: defaultSchema,
		schemaCache:   make(map[string]RelationSchema),
		aliasMap:      make(map[string]resolvedRef),
		nameMap:       make(map[string][]resolvedRef),
		relationOrder: make([]resolvedRef, 0, len(relations)),
		unboundMap:    make(map[string]bool),
	}
	for _, rel := range relations {
		if rel.Unbound {
			if rel.Alias != "" {
				s.unboundMap[strings.ToLower(rel.Alias)] = true
			}
			s.unboundMap[strings.ToLower(rel.Name)] = true
			continue
		}
		ref := resolvedRef{schema: rel.Schema, name: rel.Name}
		if rel.Alias != "" {
			s.aliasMap[strings.ToLower(rel.Alias)] = ref
		}
		s.nameMap[strings.ToLower(rel.Name)] = append(s.nameMap[strings.ToLower(rel.Name)], ref)
		if rel.Kind != domain.RelationCTE && rel.Kind != domain.RelationDerived {
			s.relationOrder = append(s.relationOrder, ref)
		}
	}
	return s
}

// resolveSchema fetches and caches the relation schema from the resolver.
func (s *resolutionState) resolveSchema(schema, name string) (RelationSchema, bool) {
	if err := s.ctx.Err(); err != nil {
		return RelationSchema{}, false
	}

	resolvedSchema := schema
	if resolvedSchema == "" {
		resolvedSchema = s.defaultSchema
	}

	cacheKey := formatCacheKey(resolvedSchema, name)
	if cached, ok := s.schemaCache[cacheKey]; ok {
		return cached, true
	}

	rs, err := s.resolver.ResolveRelation(s.ctx, s.dialect, resolvedSchema, name)
	if err != nil {
		return RelationSchema{}, false
	}

	s.schemaCache[cacheKey] = rs
	return rs, true
}

func formatCacheKey(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

// resolveRelationRef finds the schema for a relation by alias or name.
func (s *resolutionState) resolveRelationRef(tableRef string) (schema, name string) {
	lower := strings.ToLower(tableRef)

	if ref, ok := s.aliasMap[lower]; ok {
		return ref.schema, ref.name
	}

	if refs, ok := s.nameMap[lower]; ok && len(refs) == 1 {
		return refs[0].schema, refs[0].name
	}

	if refs, ok := s.nameMap[lower]; ok && len(refs) > 1 {
		if s.defaultSchema != "" {
			for _, ref := range refs {
				if strings.EqualFold(ref.schema, s.defaultSchema) {
					return ref.schema, ref.name
				}
			}
		}
		// Multiple relations with same name, no schema match — ambiguous.
		return "", ""
	}

	return "", tableRef
}

// resolveMetadata enriches a domain Result with metadata-backed resolution.
func resolveMetadata(ctx context.Context, resolver SchemaResolver, dialect, defaultSchema string, result domain.Result) domain.Result {
	if resolver == nil {
		return result
	}

	state := newResolutionState(ctx, resolver, dialect, defaultSchema, result.Relations)

	var newUnresolved []domain.Unresolved //nolint:prealloc // overwritten by resolveRelations return
	result.Relations, newUnresolved = resolveRelations(state, result.Relations)
	resolvedCols, expandedWildcards, colUnresolved := resolveColumns(state, result.ReferencedColumns)
	result.ReferencedColumns = resolvedCols
	newUnresolved = append(newUnresolved, colUnresolved...)
	result.Outputs = resolveOutputs(state, result.Outputs)

	wcCols, wcExpanded, wcOutputs, wcUnresolved := expandUnresolvedWildcards(state, result.Unresolved, result.Outputs)
	result.ReferencedColumns = append(result.ReferencedColumns, wcCols...)
	for k, v := range wcExpanded {
		expandedWildcards[k] = v
	}
	result.Outputs = wcOutputs
	newUnresolved = append(newUnresolved, wcUnresolved...)

	result.Unresolved = filterResolvedUnresolved(result.Unresolved, expandedWildcards)
	result.Unresolved = append(result.Unresolved, newUnresolved...)
	result.Unresolved = domain.SortUnresolved(result.Unresolved)

	return result
}

// isUnbound reports whether a table reference belongs to a relation marked unbound.
func (s *resolutionState) isUnbound(tableRef string) bool {
	if tableRef == "" {
		return false
	}
	return s.unboundMap[strings.ToLower(tableRef)]
}

// resolveRelations enriches relation references with metadata: resolves schemas, detects views.
func resolveRelations(state *resolutionState, relations []domain.RelationReference) ([]domain.RelationReference, []domain.Unresolved) {
	out := make([]domain.RelationReference, 0, len(relations))
	var unresolved []domain.Unresolved
	for _, rel := range relations {
		if rel.Unbound || rel.Kind == domain.RelationCTE || rel.Kind == domain.RelationDerived {
			out = append(out, rel)
			continue
		}

		resolvedSchema := rel.Schema
		if resolvedSchema == "" {
			resolvedSchema = state.defaultSchema
		}

		if err := state.ctx.Err(); err != nil {
			unresolved = append(unresolved, domain.Unresolved{
				Reference: domain.FormatRelationKey(rel.Schema, rel.Name),
				Reason:    ReasonMissingMetadata,
			})
			out = append(out, rel)
			continue
		}

		rs, ok := state.resolveSchema(resolvedSchema, rel.Name)
		if !ok {
			ref := domain.FormatRelationKey(rel.Schema, rel.Name)
			unresolved = append(unresolved, domain.Unresolved{
				Reference: ref,
				Reason:    ReasonRelationNotFound,
			})
			out = append(out, rel)
			continue
		}

		enriched := rel
		if enriched.Schema == "" {
			enriched.Schema = rs.Schema
		}
		if rs.IsView {
			enriched.Kind = domain.RelationView
		}
		out = append(out, enriched)
	}
	return out, unresolved
}

// resolveColumns enriches column references: resolves schemas, table qualifiers, and unqualified columns.
func resolveColumns(state *resolutionState, columns []domain.ColumnReference) ([]domain.ColumnReference, map[string]bool, []domain.Unresolved) {
	out := make([]domain.ColumnReference, 0, len(columns))
	expandedWildcards := make(map[string]bool)
	var unresolved []domain.Unresolved
	for _, col := range columns {
		if col.Unbound {
			out = append(out, col)
			continue
		}
		if col.Column == "*" {
			expanded, originalRef, wcUnresolved := expandStarColumn(state, col)
			if len(expanded) > 0 && (len(expanded) != 1 || expanded[0].Column != "*") {
				expandedWildcards[originalRef] = true
			}
			out = append(out, expanded...)
			unresolved = append(unresolved, wcUnresolved...)
		} else if col.Table != "" {
			resolved, colUnresolved := resolveQualifiedColumn(state, col)
			out = append(out, resolved...)
			unresolved = append(unresolved, colUnresolved...)
		} else {
			resolved, colUnresolved := resolveUnqualifiedColumn(state, col)
			out = append(out, resolved...)
			unresolved = append(unresolved, colUnresolved...)
		}
	}
	return out, expandedWildcards, unresolved
}

// expandStarColumn expands a wildcard column into individual column references.
func expandStarColumn(state *resolutionState, col domain.ColumnReference) ([]domain.ColumnReference, string, []domain.Unresolved) {
	if col.Table == "" {
		return expandGlobalStar(state, col)
	}
	return expandTableStar(state, col)
}

func expandGlobalStar(state *resolutionState, col domain.ColumnReference) ([]domain.ColumnReference, string, []domain.Unresolved) {
	var expanded []domain.ColumnReference
	var unresolved []domain.Unresolved
	originalRef := "*"
	for _, rel := range state.nameMap {
		for _, ref := range rel {
			if ref.schema == "" && state.defaultSchema != "" {
				ref.schema = state.defaultSchema
			}
			rs, ok := state.resolveSchema(ref.schema, ref.name)
			if !ok {
				unresolved = append(unresolved, domain.Unresolved{
					Reference: domain.FormatRelationKey(ref.schema, ref.name) + ".*",
					Reason:    ReasonUnresolvedWildcard,
				})
				continue
			}
			for _, c := range rs.Columns {
				expanded = append(expanded, domain.ColumnReference{
					Schema: rs.Schema,
					Table:  rs.Name,
					Column: c.Name,
					Usages: col.Usages,
				})
			}
		}
	}
	if len(expanded) == 0 {
		return []domain.ColumnReference{{
			Table:  col.Table,
			Column: col.Column,
			Usages: col.Usages,
		}}, originalRef, unresolved
	}
	return expanded, originalRef, unresolved
}

func expandTableStar(state *resolutionState, col domain.ColumnReference) ([]domain.ColumnReference, string, []domain.Unresolved) {
	schema, name := state.resolveRelationRef(col.Table)
	originalRef := domain.FormatRelationKey(col.Table, "*")
	if schema != "" && schema != col.Table {
		originalRef = domain.FormatRelationKey(schema, name) + ".*"
	}
	if schema == "" && state.isUnbound(col.Table) {
		return []domain.ColumnReference{col}, originalRef, nil
	}
	rs, ok := state.resolveSchema(schema, name)
	if !ok {
		return []domain.ColumnReference{col}, originalRef, []domain.Unresolved{{
			Reference: originalRef,
			Reason:    ReasonUnresolvedWildcard,
		}}
	}

	expanded := make([]domain.ColumnReference, 0, len(rs.Columns))
	for _, c := range rs.Columns {
		expanded = append(expanded, domain.ColumnReference{
			Schema: rs.Schema,
			Table:  rs.Name,
			Column: c.Name,
			Usages: col.Usages,
		})
	}
	return expanded, originalRef, nil
}

// resolveQualifiedColumn resolves a table.column reference to schema.table.column.
func resolveQualifiedColumn(state *resolutionState, col domain.ColumnReference) ([]domain.ColumnReference, []domain.Unresolved) {
	schema, name := state.resolveRelationRef(col.Table)
	if schema == "" && state.isUnbound(col.Table) {
		return []domain.ColumnReference{col}, nil
	}
	rs, ok := state.resolveSchema(schema, name)
	if !ok {
		return []domain.ColumnReference{col}, []domain.Unresolved{{
			Reference: domain.FormatColumnKey(schema, name, col.Column),
			Reason:    ReasonColumnNotFound,
		}}
	}

	found := false
	for _, c := range rs.Columns {
		if strings.EqualFold(c.Name, col.Column) {
			found = true
			break
		}
	}
	if !found {
		return []domain.ColumnReference{col}, []domain.Unresolved{{
			Reference: domain.FormatColumnKey(rs.Schema, rs.Name, col.Column),
			Reason:    ReasonColumnNotFound,
		}}
	}

	resolved := col
	if resolved.Schema == "" {
		resolved.Schema = rs.Schema
	}
	resolved.Table = rs.Name
	return []domain.ColumnReference{resolved}, nil
}

// resolveUnqualifiedColumn resolves an unqualified column when exactly ONE source matches.
func resolveUnqualifiedColumn(state *resolutionState, col domain.ColumnReference) ([]domain.ColumnReference, []domain.Unresolved) {
	var matchSchema, matchName string
	matchCount := 0

	for _, refs := range state.nameMap {
		for _, ref := range refs {
			if ref.schema == "" && ref.name == "" {
				continue
			}
			rs, ok := state.resolveSchema(ref.schema, ref.name)
			if !ok {
				continue
			}
			for _, c := range rs.Columns {
				if strings.EqualFold(c.Name, col.Column) {
					matchCount++
					matchSchema = rs.Schema
					matchName = rs.Name
					break
				}
			}
		}
	}

	if matchCount == 1 {
		resolved := col
		resolved.Schema = matchSchema
		resolved.Table = matchName
		return []domain.ColumnReference{resolved}, nil
	}

	reason := ReasonAmbiguousColumn
	if matchCount == 0 {
		reason = ReasonColumnNotFound
	}
	return []domain.ColumnReference{col}, []domain.Unresolved{{
		Reference: col.Column,
		Reason:    reason,
	}}
}

// resolveOutputs enriches output columns by propagating lineage through resolved columns.
func resolveOutputs(state *resolutionState, outputs []domain.OutputColumn) []domain.OutputColumn {
	out := make([]domain.OutputColumn, 0, len(outputs))
	for _, o := range outputs {
		resolved := o
		if len(resolved.Sources) > 0 {
			resolved.Sources = resolveSourceKeys(state, resolved.Sources)
		}
		out = append(out, resolved)
	}
	return out
}

func resolveSourceKeys(state *resolutionState, sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, src := range sources {
		parts := strings.SplitN(src, ".", 3)
		var tablePart string
		var hasSchema bool
		switch len(parts) {
		case 3:
			schema, table, _ := parts[0], parts[1], parts[2]
			tablePart = table
			hasSchema = schema != ""
		case 2:
			tablePart, _ = parts[0], parts[1]
		default:
			out = append(out, src)
			continue
		}
		if !hasSchema && state.isUnbound(tablePart) {
			continue
		}
		switch len(parts) {
		case 3:
			schema, table, col := parts[0], parts[1], parts[2]
			rsSchema, rsName := state.resolveRelationRef(table)
			if rs, ok := state.resolveSchema(rsSchema, rsName); ok {
				schema = rs.Schema
				table = rs.Name
			} else if schema == "" {
				schema = state.defaultSchema
			}
			out = append(out, domain.FormatColumnKey(schema, table, col))
		case 2:
			table, col := parts[0], parts[1]
			rsSchema, rsName := state.resolveRelationRef(table)
			if rs, ok := state.resolveSchema(rsSchema, rsName); ok {
				out = append(out, domain.FormatColumnKey(rs.Schema, rs.Name, col))
			} else {
				out = append(out, domain.FormatColumnKey(state.defaultSchema, table, col))
			}
		}
	}
	return out
}

// filterResolvedUnresolved removes unresolved entries for wildcards that were successfully expanded.
func filterResolvedUnresolved(unresolved []domain.Unresolved, expandedWildcards map[string]bool) []domain.Unresolved {
	if len(unresolved) == 0 {
		return nil
	}
	out := make([]domain.Unresolved, 0, len(unresolved))
	for _, u := range unresolved {
		if isWildcardReason(u.Reason) && expandedWildcards[u.Reference] {
			continue
		}
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// expandUnresolvedWildcards scans Unresolved for wildcard entries (schema_unavailable reason)
// and tries to expand them using the resolver. Returns expanded columns, which references
// were expanded, updated outputs with sources, and new unresolved entries for failures.
func expandUnresolvedWildcards(state *resolutionState, unresolved []domain.Unresolved, outputs []domain.OutputColumn) ([]domain.ColumnReference, map[string]bool, []domain.OutputColumn, []domain.Unresolved) {
	var wildcardRefs []string
	for _, u := range unresolved {
		if u.Reason == domain.ReasonSchemaUnavailable {
			wildcardRefs = append(wildcardRefs, u.Reference)
		}
	}
	if len(wildcardRefs) == 0 {
		return nil, nil, outputs, nil
	}

	var expandedCols []domain.ColumnReference
	expandedWildcards := make(map[string]bool)
	var newUnresolved []domain.Unresolved
	outputLineage := make(map[string][]string)

	for _, ref := range wildcardRefs {
		if state.isUnboundWildcardRef(ref) {
			continue
		}
		cols, sources, ok := expandWildcardRef(state, ref)
		if ok {
			expandedCols = append(expandedCols, cols...)
			expandedWildcards[ref] = true
			outputLineage[ref] = sources
		} else {
			newUnresolved = append(newUnresolved, domain.Unresolved{
				Reference: ref,
				Reason:    ReasonUnresolvedWildcard,
			})
		}
	}

	updatedOutputs := make([]domain.OutputColumn, 0, len(outputs))
	for _, out := range outputs {
		o := out
		if sources, ok := outputLineage[out.Name]; ok && len(o.Sources) == 0 {
			o.Sources = sources
		}
		updatedOutputs = append(updatedOutputs, o)
	}

	return expandedCols, expandedWildcards, updatedOutputs, newUnresolved
}

func (s *resolutionState) isUnboundWildcardRef(ref string) bool {
	if ref == "*" {
		return false
	}
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) == 2 && parts[1] == "*" && parts[0] != "" {
		// Schema-qualified wildcard like "public.users.*" — not unbound.
		return false
	}
	return s.isUnbound(parts[0])
}

// expandWildcardRef expands a single wildcard reference into physical columns.
func expandWildcardRef(state *resolutionState, ref string) ([]domain.ColumnReference, []string, bool) {
	if ref == "*" {
		return expandGlobalWildcard(state)
	}
	return expandTableWildcard(state, ref)
}

// expandGlobalWildcard expands SELECT * across all relations in scope.
func expandGlobalWildcard(state *resolutionState) ([]domain.ColumnReference, []string, bool) {
	var expanded []domain.ColumnReference
	var sources []string
	anyExpanded := false
	seen := make(map[string]bool)

	for _, ref := range state.relationOrder {
		if ref.schema == "" && state.defaultSchema != "" {
			ref.schema = state.defaultSchema
		}
		key := formatCacheKey(ref.schema, ref.name)
		if seen[key] {
			continue
		}
		seen[key] = true
		rs, ok := state.resolveSchema(ref.schema, ref.name)
		if !ok {
			continue
		}
		anyExpanded = true
		for _, c := range rs.Columns {
			expanded = append(expanded, domain.ColumnReference{
				Schema: rs.Schema,
				Table:  rs.Name,
				Column: c.Name,
				Usages: []domain.UsageContext{domain.UsageProjection},
			})
			sources = append(sources, domain.FormatColumnKey(rs.Schema, rs.Name, c.Name))
		}
	}
	if !anyExpanded {
		return nil, nil, false
	}
	return expanded, sources, true
}

// expandTableWildcard expands table.* wildcard.
func expandTableWildcard(state *resolutionState, ref string) ([]domain.ColumnReference, []string, bool) {
	tableName := strings.TrimSuffix(ref, ".*")
	if tableName == ref {
		return nil, nil, false
	}

	schema, name := state.resolveRelationRef(tableName)
	if schema == "" && state.isUnbound(tableName) {
		return nil, nil, false
	}
	if schema == "" && state.defaultSchema != "" {
		schema = state.defaultSchema
	}
	rs, ok := state.resolveSchema(schema, name)
	if !ok {
		return nil, nil, false
	}

	expanded := make([]domain.ColumnReference, 0, len(rs.Columns))
	sources := make([]string, 0, len(rs.Columns))
	for _, c := range rs.Columns {
		expanded = append(expanded, domain.ColumnReference{
			Schema: rs.Schema,
			Table:  rs.Name,
			Column: c.Name,
			Usages: []domain.UsageContext{domain.UsageProjection},
		})
		sources = append(sources, domain.FormatColumnKey(rs.Schema, rs.Name, c.Name))
	}
	return expanded, sources, true
}

func isWildcardReason(r domain.ReasonCode) bool {
	return r == ReasonUnresolvedWildcard || r == domain.ReasonSchemaUnavailable
}

// FormatRelationSchemaKey returns a cache key for a relation schema lookup.
func FormatRelationSchemaKey(schema, name string) string {
	return formatCacheKey(schema, name)
}

// NewResolutionState creates a resolution state for testing.
func NewResolutionState(ctx context.Context, resolver SchemaResolver, dialect, defaultSchema string, relations []domain.RelationReference) *resolutionState {
	return newResolutionState(ctx, resolver, dialect, defaultSchema, relations)
}

// ResolveMetadata exposes the metadata resolution for testing.
func ResolveMetadata(ctx context.Context, resolver SchemaResolver, dialect, defaultSchema string, result domain.Result) domain.Result {
	return resolveMetadata(ctx, resolver, dialect, defaultSchema, result)
}
