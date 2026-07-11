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
	}
	for _, rel := range relations {
		ref := resolvedRef{schema: rel.Schema, name: rel.Name}
		if rel.Alias != "" {
			s.aliasMap[strings.ToLower(rel.Alias)] = ref
		}
		s.nameMap[strings.ToLower(rel.Name)] = append(s.nameMap[strings.ToLower(rel.Name)], ref)
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
		return refs[0].schema, refs[0].name
	}

	return "", tableRef
}

// resolveMetadata enriches a domain Result with metadata-backed resolution.
func resolveMetadata(ctx context.Context, resolver SchemaResolver, dialect, defaultSchema string, result domain.Result) domain.Result {
	if resolver == nil {
		return result
	}

	state := newResolutionState(ctx, resolver, dialect, defaultSchema, result.Relations)

	result.Relations = resolveRelations(state, result.Relations)
	result.ReferencedColumns = resolveColumns(state, result.ReferencedColumns)
	result.Outputs = resolveOutputs(state, result.Outputs)
	result.Unresolved = filterResolvedUnresolved(result.Unresolved, state)

	return result
}

// resolveRelations enriches relation references with metadata: resolves schemas, detects views.
func resolveRelations(state *resolutionState, relations []domain.RelationReference) []domain.RelationReference {
	out := make([]domain.RelationReference, 0, len(relations))
	for _, rel := range relations {
		if rel.Kind == domain.RelationCTE || rel.Kind == domain.RelationDerived {
			out = append(out, rel)
			continue
		}

		resolvedSchema := rel.Schema
		if resolvedSchema == "" {
			resolvedSchema = state.defaultSchema
		}

		rs, ok := state.resolveSchema(resolvedSchema, rel.Name)
		if !ok {
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
	return out
}

// resolveColumns enriches column references: resolves schemas, table qualifiers, and unqualified columns.
func resolveColumns(state *resolutionState, columns []domain.ColumnReference) []domain.ColumnReference {
	out := make([]domain.ColumnReference, 0, len(columns))
	for _, col := range columns {
		resolved := resolveOneColumn(state, col)
		out = append(out, resolved...)
	}
	return out
}

func resolveOneColumn(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	if col.Column == "*" {
		return expandStarColumn(state, col)
	}

	if col.Table != "" {
		return resolveQualifiedColumn(state, col)
	}

	return resolveUnqualifiedColumn(state, col)
}

// expandStarColumn expands a wildcard column into individual column references.
func expandStarColumn(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	if col.Table == "" {
		return expandGlobalStar(state, col)
	}
	return expandTableStar(state, col)
}

func expandGlobalStar(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	var expanded []domain.ColumnReference
	for _, rel := range state.nameMap {
		for _, ref := range rel {
			if ref.schema == "" && state.defaultSchema != "" {
				ref.schema = state.defaultSchema
			}
			rs, ok := state.resolveSchema(ref.schema, ref.name)
			if !ok {
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
		}}
	}
	return expanded
}

func expandTableStar(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	schema, name := state.resolveRelationRef(col.Table)
	rs, ok := state.resolveSchema(schema, name)
	if !ok {
		return []domain.ColumnReference{col}
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
	return expanded
}

// resolveQualifiedColumn resolves a table.column reference to schema.table.column.
func resolveQualifiedColumn(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	schema, name := state.resolveRelationRef(col.Table)
	rs, ok := state.resolveSchema(schema, name)
	if !ok {
		return []domain.ColumnReference{col}
	}

	found := false
	for _, c := range rs.Columns {
		if strings.EqualFold(c.Name, col.Column) {
			found = true
			break
		}
	}
	if !found {
		return []domain.ColumnReference{col}
	}

	resolved := col
	if resolved.Schema == "" {
		resolved.Schema = rs.Schema
	}
	resolved.Table = rs.Name
	return []domain.ColumnReference{resolved}
}

// resolveUnqualifiedColumn resolves an unqualified column when exactly ONE source matches.
func resolveUnqualifiedColumn(state *resolutionState, col domain.ColumnReference) []domain.ColumnReference {
	var matchSchema, matchName string
	matchCount := 0

	for _, refs := range state.nameMap {
		for _, ref := range refs {
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
		return []domain.ColumnReference{resolved}
	}

	return []domain.ColumnReference{col}
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
		default:
			out = append(out, src)
		}
	}
	return out
}

// filterResolvedUnresolved removes unresolved entries that were successfully resolved.
func filterResolvedUnresolved(unresolved []domain.Unresolved, state *resolutionState) []domain.Unresolved {
	if len(unresolved) == 0 {
		return nil
	}
	out := make([]domain.Unresolved, 0, len(unresolved))
	for _, u := range unresolved {
		if isWildcardReason(u.Reason) && hasMetadata(state) {
			continue
		}
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isWildcardReason(r domain.ReasonCode) bool {
	return r == ReasonUnresolvedWildcard || r == domain.ReasonSchemaUnavailable
}

func hasMetadata(state *resolutionState) bool {
	return len(state.schemaCache) > 0 || state.resolver != nil
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
