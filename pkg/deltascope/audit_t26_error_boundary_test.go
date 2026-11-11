package deltascope

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t26BoundaryProvider is the public-contract stub for the T26/#105
// error-boundary verification: it implements the public
// MetadataProvider seam plus the optional object-resolver extension,
// and fails every object read with a synthetic error. Facts are never
// fabricated — a failed read is the only thing this stub produces.
type t26BoundaryProvider struct {
	err error
}

func (p *t26BoundaryProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{}, nil
}

func (p *t26BoundaryProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*TableSnapshot, error) {
	return nil, errors.New("table snapshot reads are not part of this boundary check")
}

func (p *t26BoundaryProvider) ResolveObject(context.Context, spec.Dialect, spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return nil, p.err
}

// The public error boundary on the real SDK exit: when the provider's
// own failure carries protected content, the public error text isolates
// it while the cause chain keeps the original error reachable. The
// provider error below carries a sensitive sentinel; the authored SQL
// carries more sentinels — the server identity, an OPTIONS USER value,
// and an OPTIONS PASSWORD value. The public err.Error() must contain
// none of them, errors.Is must still recognize the original cause, and
// the call must still return an error — the read fails, it is never
// laundered into an unknown answer. This registers the actual boundary
// contract: bounded public text outside, full cause inside — not a
// claim that object identities are globally redacted.
func TestT26PublicErrorBoundaryMarker(t *testing.T) {
	sensitiveErr := errors.New("sensitive-sentinel-p4k")
	provider := &t26BoundaryProvider{err: sensitiveErr}
	_, err := Audit(context.Background(), Request{
		SQL: "CREATE SERVER s1_cred_sentinel FOREIGN DATA WRAPPER mysql " +
			"OPTIONS (USER 'u_sentinel_q7', PASSWORD 'p_sentinel_x9')",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("resolver failure must still fail at the public boundary")
	}
	if !errors.Is(err, sensitiveErr) {
		t.Fatalf("public error chain lost the provider cause: %v", err)
	}
	for _, protected := range []string{
		"sensitive-sentinel-p4k", // provider-carried sensitive content
		"s1_cred_sentinel",       // the provable server identity
		"u_sentinel_q7",          // OPTIONS USER payload
		"p_sentinel_x9",          // OPTIONS PASSWORD payload
		"CREATE SERVER",          // statement keywords
	} {
		if strings.Contains(err.Error(), protected) {
			t.Fatalf("public error text echoes protected content %q: %v", protected, err)
		}
	}
}
