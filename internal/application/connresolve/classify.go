// Package connresolve turns caller connection input into a ready-to-open configuration.
// input: connection, TLS, timeout, and authentication errors from later openers
// output: a Connection Failure Class with no host, DSN, driver text, or credential
// pos: shared classification for Transport Connection Resolution; surfaces map class to their own words
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"crypto/x509"
	"errors"
	"strings"
	"syscall"
)

// FailureClass is a Connection Failure Class. It is not a user-facing sentence.
type FailureClass string

const (
	ClassNone             FailureClass = ""
	ClassValidation       FailureClass = "validation"
	ClassAuthentication   FailureClass = "authentication"
	ClassRefused          FailureClass = "refused"
	ClassTimeout          FailureClass = "timeout"
	ClassTLSHostname      FailureClass = "tls_hostname"
	ClassTLSUnknownCA     FailureClass = "tls_unknown_ca"
	ClassTLSNotOffered    FailureClass = "tls_not_offered"
	ClassTLSCertificate   FailureClass = "tls_certificate"
	ClassTLSHandshake     FailureClass = "tls_handshake"
	ClassConnectionFailed FailureClass = "connection_failed"
)

// Classify maps an opener or network error to a Connection Failure Class.
func Classify(err error) FailureClass {
	if err == nil {
		return ClassNone
	}

	msg := connectionErrorText(err)
	var hostnameErr x509.HostnameError
	var hostnameErrPtr *x509.HostnameError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var unknownAuthorityErrPtr *x509.UnknownAuthorityError
	switch {
	case isAuthenticationFailure(err):
		return ClassAuthentication
	case errors.Is(err, syscall.ECONNREFUSED):
		return ClassRefused
	case errors.As(err, &hostnameErr), errors.As(err, &hostnameErrPtr),
		strings.Contains(msg, "certificate is valid for"),
		strings.Contains(msg, "cannot validate certificate"),
		strings.Contains(msg, "certificate relies on legacy common name"),
		strings.Contains(msg, "certificate is not valid for"):
		return ClassTLSHostname
	case errors.As(err, &unknownAuthorityErr), errors.As(err, &unknownAuthorityErrPtr),
		strings.Contains(msg, "unknown authority"),
		strings.Contains(msg, "unknown ca"),
		strings.Contains(msg, "untrusted root"):
		return ClassTLSUnknownCA
	case strings.Contains(msg, "server refused tls connection"),
		strings.Contains(msg, "server refused ssl connection"),
		strings.Contains(msg, "server does not support tls"),
		strings.Contains(msg, "server does not support ssl"),
		strings.Contains(msg, "server did not offer tls"),
		strings.Contains(msg, "server does not offer tls"):
		return ClassTLSNotOffered
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "x509"):
		return ClassTLSCertificate
	case strings.Contains(msg, "tls"):
		return ClassTLSHandshake
	case strings.Contains(msg, "timeout"):
		return ClassTimeout
	default:
		return ClassConnectionFailed
	}
}

func isAuthenticationFailure(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "access denied") ||
		strings.Contains(message, "authentication failed") ||
		strings.Contains(message, "password authentication") ||
		strings.Contains(message, "invalid authorization")
}

func connectionErrorText(err error) string {
	if err == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.ToLower(err.Error()))
	if inner := errors.Unwrap(err); inner != nil {
		b.WriteByte('\n')
		b.WriteString(strings.ToLower(inner.Error()))
	}
	return b.String()
}
