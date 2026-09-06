// Package connresolve verifies Connection Failure Class mapping.
// input: wrapped network, TLS, and authentication errors
// output: bounded failure classes without driver text
// pos: application tests at the Classify seam
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	certificate := &x509.Certificate{
		Subject:  pkix.Name{CommonName: "secret-cert.example"},
		DNSNames: []string{"secret-cert.example"},
	}

	tests := []struct {
		name string
		err  error
		want FailureClass
	}{
		{name: "nil", err: nil, want: ClassNone},
		{
			name: "refused",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			},
			want: ClassRefused,
		},
		{name: "authentication", err: errors.New("Access denied for user"), want: ClassAuthentication},
		{
			name: "hostname",
			err:  fmt.Errorf("tls: failed to verify certificate: %w", x509.HostnameError{Certificate: certificate, Host: "secret-target.example"}),
			want: ClassTLSHostname,
		},
		{
			name: "unknown ca",
			err:  fmt.Errorf("tls: failed to verify certificate: %w", x509.UnknownAuthorityError{Cert: certificate}),
			want: ClassTLSUnknownCA,
		},
		{name: "mysql no tls", err: gomysql.ErrNoTLS, want: ClassTLSNotOffered},
		{name: "pg refused tls", err: errors.New("server refused TLS connection"), want: ClassTLSNotOffered},
		{name: "cert fallback", err: errors.New("tls: failed to verify certificate: certificate policy rejected"), want: ClassTLSCertificate},
		{name: "handshake fallback", err: errors.New("tls: handshake failure"), want: ClassTLSHandshake},
		{name: "timeout", err: errors.New("dial tcp: i/o timeout"), want: ClassTimeout},
		{name: "generic", err: errors.New("dial tcp 10.0.0.1:3306: no route to host"), want: ClassConnectionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Classify(tt.err); got != tt.want {
				t.Fatalf("Classify() = %q, want %q", got, tt.want)
			}
		})
	}
}
