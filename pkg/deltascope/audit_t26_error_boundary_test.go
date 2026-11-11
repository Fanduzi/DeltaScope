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
// and fails every object read with a synthetic marker. Facts are never
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

// The public error exit is the real SDK contract: a provider failure
// propagates verbatim through deltascope.Audit while authored statement
// payloads never enter the returned error text. The authored SQL below
// carries credential-shaped sentinels — the server identity itself, an
// OPTIONS USER value, and an OPTIONS PASSWORD value — none of which may
// appear in the public error. This is a propagation-plus-non-echo check
// on the shared upstream error the CLI, HTTP, and MCP adapters render:
// it registers what the error contract actually is (provider text
// passes through, our layers add nothing authored), not a claim that
// object identities are globally redacted — real server error text may
// legitimately echo object names the engine was asked about.
func TestT26PublicErrorBoundaryMarker(t *testing.T) {
	const providerMarker = "t26-provider-sentinel-8f2d"
	provider := &t26BoundaryProvider{err: errors.New(providerMarker)}
	_, err := Audit(context.Background(), Request{
		SQL: "CREATE SERVER s1_cred_sentinel FOREIGN DATA WRAPPER mysql " +
			"OPTIONS (USER 'u_sentinel_q7', PASSWORD 'p_sentinel_x9')",
		Dialect:          DialectMySQL,
		Schema:           "golden",
		MetadataProvider: provider,
	})
	if err == nil || !strings.Contains(err.Error(), providerMarker) {
		t.Fatalf("public error = %v, want provider failure text propagated", err)
	}
	for _, authored := range []string{
		"s1_cred_sentinel", // the provable server identity — our layers never add it to error text
		"u_sentinel_q7",    // OPTIONS USER payload
		"p_sentinel_x9",    // OPTIONS PASSWORD payload
		"CREATE SERVER",    // statement keywords
	} {
		if strings.Contains(err.Error(), authored) {
			t.Fatalf("public error echoes authored payload %q: %v", authored, err)
		}
	}
}
