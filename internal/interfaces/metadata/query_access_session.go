// Package metadata exposes shared helpers for metadata-aware and offline interface adapters.
// input: an opened online.Session and the transport's injectable from-conn constructor
// output: a unified OnlineQueryAccessSession that reuses Observed Server Identity when present
// pos: shared CLI/HTTP attach seam above the public identified-conn constructor
// note: if this file changes, update this header and module README.md.
package metadata

import (
	"context"
	"database/sql"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// OnlineSessionFromConn constructs a unified session from a caller-owned connection.
type OnlineSessionFromConn func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error)

// AttachOnlineQueryAccessSession wraps an opened online session for unified Query Access.
// A nil session or a session without identity uses fromConn. A session that already
// observed server identity uses NewOnlineQueryAccessSessionFromIdentifiedConn so
// transports do not probe VERSION again.
func AttachOnlineQueryAccessSession(ctx context.Context, session *online.Session, fromConn OnlineSessionFromConn) (*deltascope.OnlineQueryAccessSession, error) {
	if session == nil {
		return fromConn(ctx, nil)
	}
	if session.Identity != nil {
		return deltascope.NewOnlineQueryAccessSessionFromIdentifiedConn(session.Conn, session.Identity)
	}
	return fromConn(ctx, session.Conn)
}
