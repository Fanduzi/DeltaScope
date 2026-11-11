package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// t26SentinelProvider fails every object read with an error whose text
// carries both a "parse" keyword — the historical echo branch in the
// HTTP error mapper — and a protected sentinel. Facts are never
// fabricated.
type t26SentinelProvider struct {
	err error
}

func (p *t26SentinelProvider) LoadInstanceFacts(context.Context, deltascope.Dialect, string) (*deltascope.InstanceFacts, error) {
	return &deltascope.InstanceFacts{}, nil
}

func (p *t26SentinelProvider) LoadTableSnapshot(context.Context, deltascope.Dialect, string, string) (*deltascope.TableSnapshot, error) {
	return nil, errors.New("table snapshot reads are not part of this boundary check")
}

func (p *t26SentinelProvider) ResolveObject(context.Context, spec.Dialect, spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return nil, p.err
}

// The HTTP public-error path must never echo provider-carried
// protected content — even when the provider error contains "parse",
// the keyword that historically let raw error text through the
// message mapper. A resolver failure crossing the real audit entry
// reaches the real response writer bounded: the sentinel never appears
// in the public message, and the request still fails (T26/#105).
func TestHandlerT26ProviderErrorBoundedMessage(t *testing.T) {
	sentinelErr := errors.New("parse catalog sentinel credmarker-m5q1")
	provider := &t26SentinelProvider{err: sentinelErr}
	auditFn := func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		request.MetadataProvider = provider
		return deltascope.Audit(ctx, request)
	}
	handler, err := NewHandler("", "test-build", WithAuditFunc(auditFn))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"sql": "DROP SERVER s1", "dialect": "mysql", "schema": "golden",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("resolver failure must still fail the request, got 200: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "credmarker-m5q1") ||
		strings.Contains(rec.Body.String(), sentinelErr.Error()) {
		t.Fatalf("http response echoes provider-carried protected content: %s", rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%q", err, rec.Body.String())
	}
	if payload["error"] == nil {
		t.Fatalf("want a bounded error envelope, got %s", rec.Body.String())
	}
}
