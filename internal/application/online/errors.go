// Package online provides the shared online session factory for SDK, CLI, and HTTP.
// input: errors from online operations (session open, identity, authorization)
// output: bounded error taxonomy and status mapping that never leaks secrets, endpoints, or driver text
// pos: shared error boundary for all online surfaces (HTTP, MCP, CLI)
// note: if this file changes, update this header and module README.md.
package online

import (
	"context"
	"errors"
	"net/http"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
)

// Sentinel errors for online operations.
// Messages are bounded — they never contain secrets, endpoints, versions, or driver text.
var (
	ErrConnectionNotFound  = errors.New("connection not found")
	ErrPurposeNotAllowed   = errors.New("purpose not allowed for this connection")
	ErrPrincipalNotAllowed = errors.New("principal not authorized for this connection")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrSchemaRequired      = errors.New("schema is required")
	ErrSchemaLookupFailed  = errors.New("schema lookup failed")
	ErrTimeout             = errors.New("operation timed out")
	ErrCanceled            = errors.New("operation canceled")
	ErrInternal            = errors.New("internal error")
)

// MapOnlineError maps an error from online operations to a bounded (code, message, status) tuple.
// The returned message never contains sensitive information such as DSNs, credentials,
// hostnames, ports, driver text, or version strings.
func MapOnlineError(err error) (code string, message string, status int) {
	if err == nil {
		return "", "", 0
	}

	// Connection failures — check before context errors because a connection
	// failure may wrap a context.DeadlineExceeded (e.g., when the remote end
	// accepts then immediately closes, causing the driver to see unexpected
	// EOF and the context to expire). The root cause is the connection
	// failure, not the timeout.
	if errors.Is(err, ErrConnectionFailed) {
		return "connection_failed", "connection failed", http.StatusBadGateway
	}

	// Context errors — check first, including wrapped inside auditmeta.Error.
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "operation timed out", http.StatusGatewayTimeout
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", "request canceled", 499
	}

	// Check for context errors wrapped inside auditmeta.Error.
	var prepErr *auditmeta.Error
	if errors.As(err, &prepErr) && prepErr.Err != nil {
		if errors.Is(prepErr.Err, context.DeadlineExceeded) {
			return "timeout", "operation timed out", http.StatusGatewayTimeout
		}
		if errors.Is(prepErr.Err, context.Canceled) {
			return "canceled", "request canceled", 499
		}
	}

	// Online sentinel errors.
	if errors.Is(err, ErrConnectionNotFound) {
		return "connection_not_found", "connection not found", http.StatusNotFound
	}
	if errors.Is(err, ErrPurposeNotAllowed) {
		return "purpose_not_allowed", "purpose not allowed", http.StatusForbidden
	}
	if errors.Is(err, ErrPrincipalNotAllowed) {
		return "not_authorized", "not authorized for this connection", http.StatusForbidden
	}

	// Identity sentinel errors (defined in identity.go).
	if errors.Is(err, ErrIdentityUnavailable) ||
		errors.Is(err, ErrIdentityUnknown) ||
		errors.Is(err, ErrIdentityMalformed) ||
		errors.Is(err, ErrIdentityUnsupported) {
		return "identity_error", "server identity error", http.StatusBadGateway
	}
	if errors.Is(err, ErrDialectMismatch) {
		return "dialect_mismatch", "dialect disagrees with server", http.StatusBadRequest
	}

	if errors.Is(err, ErrSchemaRequired) {
		return "schema_required", "schema is required", http.StatusBadRequest
	}
	if errors.Is(err, ErrSchemaLookupFailed) {
		return "schema_lookup_failed", "schema lookup failed", http.StatusBadGateway
	}
	if errors.Is(err, ErrTimeout) {
		return "timeout", "operation timed out", http.StatusGatewayTimeout
	}
	if errors.Is(err, ErrCanceled) {
		return "canceled", "request canceled", 499
	}
	if errors.Is(err, ErrInternal) {
		return "internal_error", "internal server error", http.StatusInternalServerError
	}

	// Auditmeta typed errors.
	if errors.As(err, &prepErr) {
		return mapAuditmetaError(prepErr)
	}

	// Default.
	return "internal_error", "internal server error", http.StatusInternalServerError
}

// mapAuditmetaError maps an auditmeta.Error to a bounded (code, message, status) tuple.
func mapAuditmetaError(prepErr *auditmeta.Error) (code string, message string, status int) {
	switch prepErr.Kind {
	case auditmeta.ErrorConnectionOpen:
		return "connection_failed", "connection failed", http.StatusBadGateway
	case auditmeta.ErrorDialectDetect:
		return "connection_failed", "connection failed", http.StatusBadGateway
	case auditmeta.ErrorDialectMismatch:
		return "dialect_mismatch", "dialect disagrees with server", http.StatusBadRequest
	case auditmeta.ErrorSchemaHintRequired:
		return "schema_required", "schema is required", http.StatusBadRequest
	case auditmeta.ErrorSchemaLookupFailed:
		return "schema_lookup_failed", "schema lookup failed", http.StatusBadGateway
	case auditmeta.ErrorInvalidSQL:
		return "invalid_sql", "invalid SQL", http.StatusBadRequest
	default:
		return "internal_error", "internal server error", http.StatusInternalServerError
	}
}
