// Package audit: T26/#105 provider round — the MySQL SERVER and spatial
// reference system fact bindings. Each family resolves its one authored
// identity through the shared object-resolver seam (the once-per-request
// objectCache lookup), then binds the provider's fact projection onto
// statement metadata. There is no ordered registry and no lifecycle
// transition this round — the families keep their named boundaries and
// these facts are provider evidence only, never post-state.
//
// Knownness stays per-fact: a statement naming a family always binds a
// state (the fact was bound), while Read/UnreadReason and the nil
// sub-members keep "not loaded", "identity unproven", "version
// inapplicable", and "read ran, answer unknown" distinct. Resolver
// failures propagate — a failed read is never laundered into an
// unvouched state.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// objectResolverFor returns the family resolver — the once-per-request
// cached seam when the enrich loop wired it, else the provider's own
// ObjectResolver — or nil when no resolver exists.
func (s *batchState) objectResolverFor(cached func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)) func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if cached != nil {
		return cached
	}
	provider, ok := s.provider.(ObjectResolver)
	if !ok {
		return nil
	}
	return func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
		return provider.ResolveObject(ctx, s.dialect, lookup)
	}
}

// bindMySQLServer resolves the one server identity a SERVER-family
// statement names and binds the provider's existence fact. The catalog
// read fires only when the authored name is a provable identifier
// (decl.NameIdent) — a string-literal spelling stays an unissued read
// with the identity-unproven reason, never a normalized-identity query
// and never a cached absence. A contaminated batch or a non-MySQL
// dialect binds the state unvouched; resolver failures propagate.
func (s *batchState) bindMySQLServer(ctx context.Context, statement spec.Statement) (*spec.ServerState, error) {
	decl := statement.DDL.Server
	bound := &spec.ServerState{Name: strings.TrimSpace(decl.Name)}
	if s.dialect != spec.DialectMySQL || s.contaminated {
		return bound, nil
	}
	resolver := s.objectResolverFor(s.mysqlServerResolver)
	if resolver == nil {
		return bound, nil
	}
	lookup := spec.ObjectLookupRequest{Type: "mysql_server", Name: decl.Name}
	if decl.NameIdent {
		lookup.Qualifiers = map[string]string{
			spec.ObjectLookupQualifierNameForm: spec.ObjectLookupNameFormIdentifier,
		}
	}
	snapshot, err := resolver(ctx, lookup)
	if err != nil {
		return nil, err
	}
	if snapshot != nil && snapshot.Server != nil {
		return snapshot.Server, nil
	}
	return bound, nil
}

// bindMySQLSRS resolves the one SRID identity a spatial reference
// system statement names and binds the provider's fact set — existence
// plus the NAME/ORGANIZATION conflict witnesses and column dependencies
// the request qualifies. The authored NAME/ORGANIZATION members are
// string-literal spellings this round — never provable catalog
// identities — so the statement path carries no conflict qualifiers and
// those reads stay unissued; the SRID keeps the existence and
// dependency reads independently provable. Resolver failures propagate.
func (s *batchState) bindMySQLSRS(ctx context.Context, statement spec.Statement) (*spec.SpatialReferenceSystemState, error) {
	decl := statement.DDL.SpatialReference
	bound := &spec.SpatialReferenceSystemState{SRID: decl.SRID}
	if s.dialect != spec.DialectMySQL || s.contaminated {
		return bound, nil
	}
	resolver := s.objectResolverFor(s.mysqlSRSResolver)
	if resolver == nil {
		return bound, nil
	}
	snapshot, err := resolver(ctx, spec.ObjectLookupRequest{
		Type: "mysql_spatial_reference_system",
		Name: decl.SRIDName(),
	})
	if err != nil {
		return nil, err
	}
	if snapshot != nil && snapshot.SpatialReference != nil {
		return snapshot.SpatialReference, nil
	}
	return bound, nil
}
