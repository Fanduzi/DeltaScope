package metadata

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestAttachOnlineQueryAccessSessionNilUsesFromConn(t *testing.T) {
	t.Parallel()

	var got *sql.Conn
	fromConn := func(_ context.Context, conn *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		got = conn
		return nil, errors.New("from-conn stub")
	}

	_, err := AttachOnlineQueryAccessSession(context.Background(), nil, fromConn)
	if err == nil || err.Error() != "from-conn stub" {
		t.Fatalf("expected from-conn stub error, got %v", err)
	}
	if got != nil {
		t.Fatal("nil session must pass a nil connection")
	}
}

func TestAttachOnlineQueryAccessSessionWithoutIdentityUsesFromConn(t *testing.T) {
	t.Parallel()

	conn := new(sql.Conn)
	session := &online.Session{Conn: conn}
	var got *sql.Conn
	fromConn := func(_ context.Context, c *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		got = c
		return nil, errors.New("from-conn stub")
	}

	_, err := AttachOnlineQueryAccessSession(context.Background(), session, fromConn)
	if err == nil || err.Error() != "from-conn stub" {
		t.Fatalf("expected from-conn stub error, got %v", err)
	}
	if got != conn {
		t.Fatal("session without identity must pass session.Conn")
	}
}

func TestAttachOnlineQueryAccessSessionReusesObservedIdentity(t *testing.T) {
	t.Parallel()

	fromConn := func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		t.Fatal("fromConn must not run when identity is already observed")
		return nil, errors.New("from-conn must not run")
	}
	session := &online.Session{
		Conn: new(sql.Conn),
		Identity: &online.ServerIdentity{
			Product: online.ProductMySQL,
			Series:  online.SeriesMySQL80,
		},
	}

	got, err := AttachOnlineQueryAccessSession(context.Background(), session, fromConn)
	if err != nil {
		t.Fatalf("identified attach: %v", err)
	}
	if got == nil {
		t.Fatal("expected a unified session")
	}
}
