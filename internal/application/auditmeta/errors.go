// Package auditmeta prepares metadata-aware audit requests for multiple adapters.
// input: metadata-preparation failures encountered while opening connections, validating dialects, and resolving schemas
// output: typed shared errors, including PostgreSQL schema/database validation, that let adapters classify metadata-prep failures without brittle string matching
// pos: shared error taxonomy for CLI and MCP metadata-aware preparation flows
// note: if this file changes, update this header and module README.md.
package auditmeta

import "fmt"

// ErrorKind classifies one metadata-preparation failure.
type ErrorKind string

const (
	ErrorInvalidSQL                 ErrorKind = "invalid_sql"
	ErrorDialectMismatch            ErrorKind = "dialect_mismatch"
	ErrorSchemaHintRequired         ErrorKind = "schema_hint_required"
	ErrorSchemaLookupFailed         ErrorKind = "schema_lookup_failed"
	ErrorConnectionOpen             ErrorKind = "connection_open_failed"
	ErrorDialectDetect              ErrorKind = "dialect_detect_failed"
	ErrorPostgreSQLDatabaseRequired ErrorKind = "postgresql_database_required"
)

// Error is a typed metadata-preparation failure.
type Error struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newError(kind ErrorKind, message string, err error) *Error {
	return &Error{Kind: kind, Message: message, Err: err}
}

func newInvalidSQLError(err error) *Error {
	return newError(ErrorInvalidSQL, fmt.Sprintf("resolve schema targets: %v", err), err)
}

func newDialectMismatchError(detected, requested string) *Error {
	return newError(ErrorDialectMismatch, fmt.Sprintf("detected dialect %q does not match requested dialect %q", detected, requested), nil)
}

func newSchemaHintRequiredError(message string) *Error {
	return newError(ErrorSchemaHintRequired, message, nil)
}

func newSchemaLookupFailedError(table string, err error) *Error {
	return newError(ErrorSchemaLookupFailed, fmt.Sprintf("resolve schema for table %q: %v", table, err), err)
}

func newConnectionOpenError(err error) *Error {
	return newError(ErrorConnectionOpen, fmt.Sprintf("open metadata connection: %v", err), err)
}

func newDialectDetectError(err error) *Error {
	return newError(ErrorDialectDetect, fmt.Sprintf("detect dialect: %v", err), err)
}

func newPostgreSQLDatabaseRequiredError() *Error {
	return newError(ErrorPostgreSQLDatabaseRequired, "PostgreSQL schema and database are distinct; pass --database when --schema is explicitly set", nil)
}
