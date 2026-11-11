// T26/#105 provider round: the SERVER/SRS fact bindings reach the real
// object-resolver seam from the enrich loop — once per (type, name,
// qualifiers) per request, lazily, only for the families the batch
// names, and only under the MySQL dialect. The stub provider records
// every lookup and answers through the same contract the live provider
// implements, so the tests assert which request actually arrived — no
// fake resolver is substituted inside the binding.
package audit

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t26Call struct {
	typ        string
	name       string
	qualifiers map[string]string
}

type t26Provider struct {
	servers map[string]bool // proven-present server identities
	srs     map[uint32]bool // proven-present SRIDs
	calls   []t26Call
	err     error
}

func (p *t26Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t26Provider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

// ResolveObject mirrors the live provider's read contract: server
// identities answer only under the name_form=identifier proof; a
// spelling request still arrives at the seam but binds unproven.
func (p *t26Provider) ResolveObject(_ context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.calls = append(p.calls, t26Call{typ: req.Type, name: req.Name, qualifiers: req.Qualifiers})
	if p.err != nil {
		return nil, p.err
	}
	snapshot := &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}
	switch req.Type {
	case "mysql_server":
		if dialect != spec.DialectMySQL {
			return snapshot, nil
		}
		state := &spec.ServerState{Name: req.Name}
		snapshot.Server = state
		if req.Qualifiers[spec.ObjectLookupQualifierNameForm] != spec.ObjectLookupNameFormIdentifier {
			state.UnreadReason = spec.MetadataUnreadIdentityUnproven
			return snapshot, nil
		}
		state.Read = true
		if p.servers[req.Name] {
			state.Exists, state.Status = true, spec.MetadataStatusConfirmed
			snapshot.Status, snapshot.Exists = spec.MetadataStatusConfirmed, true
		}
	case "mysql_spatial_reference_system":
		if dialect != spec.DialectMySQL {
			return snapshot, nil
		}
		state := &spec.SpatialReferenceSystemState{}
		snapshot.SpatialReference = state
		parsed, scanErr := strconv.ParseUint(req.Name, 10, 32)
		if scanErr != nil {
			state.UnreadReason = spec.MetadataUnreadIdentityUnproven
			return snapshot, nil
		}
		state.SRID = uint32(parsed)
		state.Read = true
		if p.srs[state.SRID] {
			state.Exists, state.Status = true, spec.MetadataStatusConfirmed
			snapshot.Status, snapshot.Exists = spec.MetadataStatusConfirmed, true
		}
		state.Dependencies = &spec.SRSDependencyFact{}
	}
	return snapshot, nil
}

// enrichT26 runs the shared parse→extract→enrich seam directly so tests
// observe the bound metadata members the public result does not carry.
func enrichT26(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider) []spec.Statement {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, dialect)
	if parseErr != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse %q: err=%v statements=%d", sql, parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), dialect,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich %q: %v", sql, err)
	}
	return enriched
}

// An identifier-named server reaches the seam with the name_form proof —
// the provider's confirmed read binds onto metadata.
func TestT26ServerIdentBindsProvenFact(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1", spec.DialectMySQL, provider)
	state := enriched[0].Metadata.Server
	if state == nil {
		t.Fatalf("server fact not bound: %+v", enriched[0].Metadata)
	}
	if !state.Read || !state.Exists || state.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("server state = %+v, want read+confirmed", state)
	}
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_server" || provider.calls[0].name != "s1" {
		t.Fatalf("resolver calls = %+v, want one mysql_server:s1", provider.calls)
	}
	if provider.calls[0].qualifiers[spec.ObjectLookupQualifierNameForm] != spec.ObjectLookupNameFormIdentifier {
		t.Fatalf("qualifiers = %+v, want name_form=identifier proof", provider.calls[0].qualifiers)
	}
}

// A string-literal name still arrives at the seam but carries no
// provable-identity qualifier — the binding stays unissued with the
// identity-unproven reason, never a fabricated catalog answer.
func TestT26ServerSpellingBindsUnproven(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s 2": true}}
	enriched := enrichT26(t, "DROP SERVER 's 2'", spec.DialectMySQL, provider)
	state := enriched[0].Metadata.Server
	if state == nil || state.Read || state.Exists || state.UnreadReason != spec.MetadataUnreadIdentityUnproven {
		t.Fatalf("server state = %+v, want bound + identity_unproven", state)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one", provider.calls)
	}
	if provider.calls[0].qualifiers[spec.ObjectLookupQualifierNameForm] == spec.ObjectLookupNameFormIdentifier {
		t.Fatalf("string-literal name must never carry the identifier proof: %+v", provider.calls[0].qualifiers)
	}
}

// An SRID statement binds existence + the dependency read through the
// same provable numeric identity — no conflict qualifiers ride along
// from authored spellings.
func TestT26SRSBindsBySRID(t *testing.T) {
	provider := &t26Provider{srs: map[uint32]bool{4326: true}}
	enriched := enrichT26(t, "CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'WGS 84' DEFINITION 'x'", spec.DialectMySQL, provider)
	state := enriched[0].Metadata.SpatialReference
	if state == nil || !state.Read || !state.Exists || state.SRID != 4326 {
		t.Fatalf("srs state = %+v, want read+confirmed 4326", state)
	}
	if state.NameConflict != nil || state.OrgConflict != nil {
		t.Fatalf("conflict reads must stay unissued for authored spellings: %+v / %+v", state.NameConflict, state.OrgConflict)
	}
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_spatial_reference_system" || provider.calls[0].name != "4326" {
		t.Fatalf("resolver calls = %+v, want one srs:4326", provider.calls)
	}
	if provider.calls[0].qualifiers[spec.ObjectLookupQualifierSRSName] != "" || provider.calls[0].qualifiers[spec.ObjectLookupQualifierSRSOrg] != "" {
		t.Fatalf("authored spellings must never populate provable qualifiers: %+v", provider.calls[0].qualifiers)
	}
}

// The once-per-request object cache dedups an identical identity — the
// same server named twice resolves through the provider exactly once.
func TestT26ServerOncePerKeyCache(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1; DROP SERVER s1", spec.DialectMySQL, provider)
	if len(enriched) != 2 {
		t.Fatalf("statements = %d", len(enriched))
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want deduped single lookup", provider.calls)
	}
	// A distinct name is a distinct identity — two lookups.
	provider = &t26Provider{servers: map[string]bool{}}
	enrichT26(t, "DROP SERVER s1; DROP SERVER s2", spec.DialectMySQL, provider)
	if len(provider.calls) != 2 {
		t.Fatalf("resolver calls = %+v, want per-identity lookups", provider.calls)
	}
}

// The decoded identifier is the cache identity verbatim: a backquoted
// name carrying interior space is a different catalog identity from its
// trimmed twin, so each resolves through the provider and binds its own
// answer — the decoded name, the lookup, the key, and the bound state
// name all agree (T26 counterexample registration).
func TestT26ServerDecodedNameCacheIdentity(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER ` s1`; DROP SERVER `s1`", spec.DialectMySQL, provider)
	if len(provider.calls) != 2 {
		t.Fatalf("resolver calls = %+v, want two distinct identities", provider.calls)
	}
	first, second := enriched[0].Metadata.Server, enriched[1].Metadata.Server
	if first == nil || second == nil || first == second {
		t.Fatalf("server states must bind independently: %+v / %+v", first, second)
	}
	if first.Name != " s1" || first.Exists {
		t.Fatalf("` s1` state = %+v, want own read answering unknown", first)
	}
	if second.Name != "s1" || !second.Exists {
		t.Fatalf("`s1` state = %+v, want own read answering confirmed", second)
	}
}

// Qualifier identity is encoded unambiguously: a qualifier value may
// itself carry '=' or ';', so the key cannot be raw name=value;
// concatenation. The registered collision pair must never share one
// cache identity, while the same pair set re-entered resolves to the
// same key regardless of map iteration order.
func TestT26ObjectLookupQualifierKeyUnambiguous(t *testing.T) {
	collideA := &spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1",
		Qualifiers: map[string]string{"srs_name": "a;srs_org=b", "srs_org_id": "1"}}
	collideB := &spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1",
		Qualifiers: map[string]string{"srs_name": "a", "srs_org": "b", "srs_org_id": "1"}}
	if newObjectLookupKey(collideA) == newObjectLookupKey(collideB) {
		t.Fatalf("distinct qualifier sets share one cache identity")
	}
	same := &spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1",
		Qualifiers: map[string]string{"srs_name": "a;srs_org=b", "srs_org_id": "1"}}
	if newObjectLookupKey(collideA) != newObjectLookupKey(same) {
		t.Fatalf("identical qualifier sets must share one cache identity")
	}
	// A lookup carrying the identifier proof and one without stay
	// distinct even when the name matches.
	proved := &spec.ObjectLookupRequest{Type: "mysql_server", Name: "s1",
		Qualifiers: map[string]string{spec.ObjectLookupQualifierNameForm: spec.ObjectLookupNameFormIdentifier}}
	bare := &spec.ObjectLookupRequest{Type: "mysql_server", Name: "s1"}
	if newObjectLookupKey(proved) == newObjectLookupKey(bare) {
		t.Fatalf("identifier-proof qualifier must partition the cache identity")
	}
}

// Statements that never name the families issue no lookups — laziness
// holds across ordinary DDL and even under a provider that would answer.
func TestT26LazinessNonTargetStatements(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enrichT26(t, "CREATE TABLE t (id int); DROP TABLE t", spec.DialectMySQL, provider)
	for _, call := range provider.calls {
		if call.typ == "mysql_server" || call.typ == "mysql_spatial_reference_system" {
			t.Fatalf("non-T26 statement issued %s lookup — families must not read unrelated catalogs", call.typ)
		}
	}
	// A mixed batch resolves only the family it names.
	provider = &t26Provider{servers: map[string]bool{}}
	enriched := enrichT26(t, "CREATE TABLE t (id int); DROP SERVER s1", spec.DialectMySQL, provider)
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_server" {
		t.Fatalf("resolver calls = %+v, want only the server lookup", provider.calls)
	}
	if enriched[0].Metadata != nil && enriched[0].Metadata.Server != nil {
		t.Fatalf("ordinary statement must not bind the server fact: %+v", enriched[0].Metadata)
	}
}

// Under TiDB the families are vendor-boundary — no lookup is ever issued
// and no fact member is bound.
func TestT26FamiliesDialectGated(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, srs: map[uint32]bool{4326: true}}
	enriched := enrichT26(t, "DROP SERVER s1; CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'x' DEFINITION 'y'", spec.DialectTiDB, provider)
	for _, call := range provider.calls {
		if call.typ == "mysql_server" || call.typ == "mysql_spatial_reference_system" {
			t.Fatalf("tidb issued %s lookup", call.typ)
		}
	}
	for _, stmt := range enriched {
		if stmt.Metadata != nil && (stmt.Metadata.Server != nil || stmt.Metadata.SpatialReference != nil) {
			t.Fatalf("tidb bound a MySQL fact: %+v", stmt.Metadata)
		}
	}
}

// A resolver failure propagates as an error — it is never laundered
// into an unvouched binding.
func TestT26ResolverErrorPropagates(t *testing.T) {
	provider := &t26Provider{err: errors.New("catalog read failed")}
	parsed, parseErr := parseSQL(context.Background(), "DROP SERVER s1", spec.DialectMySQL)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	_, err = enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err == nil {
		t.Fatalf("resolver failure must propagate — never a silent unvouched binding")
	}
}

// The public error boundary keeps the resolver's cause reachable
// through the error chain while the public text stays bounded —
// provider text and authored statement payloads never reach
// err.Error(), and the read still fails. This registers the boundary
// contract on the application seam: bounded text outside, original
// cause inside.
func TestT26PublicErrorPathMarker(t *testing.T) {
	markerErr := errors.New("t26-provider-marker-7f3a")
	provider := &t26Provider{err: markerErr}
	_, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("resolver failure must still fail at the public boundary")
	}
	if !errors.Is(err, markerErr) {
		t.Fatalf("public error chain lost the resolver cause: %v", err)
	}
	if strings.Contains(err.Error(), markerErr.Error()) {
		t.Fatalf("public error text echoes provider payload: %v", err)
	}
	if strings.Contains(err.Error(), "DROP SERVER") || strings.Contains(err.Error(), "s1") {
		t.Fatalf("public error echoes authored statement payload: %v", err)
	}
}
