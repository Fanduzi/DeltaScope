// Package deltascope verifies the public PostgreSQL session API.
// input: various connection states and reflection checks
// output: regression coverage for session construction, lifecycle, and leak prevention
// pos: public API test coverage for PostgreSQLQueryAccessSession
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNewSessionFromConn_NilConn(t *testing.T) {
	session, err := NewPostgreSQLQueryAccessSessionFromConn(nil)
	if err == nil {
		t.Fatal("expected error for nil connection")
	}
	if session != nil {
		t.Fatal("expected nil session on error")
	}

	errText := err.Error()
	for _, forbidden := range []string{"pgx", "pq", "dsn", "password", "host=", "user="} {
		if strings.Contains(strings.ToLower(errText), forbidden) {
			t.Errorf("error text must not contain %q, got: %s", forbidden, errText)
		}
	}
}

func TestSession_NoJSONLeak(t *testing.T) {
	// Zero-value session (not usable, but tests JSON shape).
	session := &PostgreSQLQueryAccessSession{}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	forbiddenFields := []string{
		"conn", "session", "binding", "oid", "database", "role",
		"dsn", "password", "user", "host", "port", "catalog", "sql",
		"manifest", "trusted", "severity", "path_epoch",
	}
	for _, field := range forbiddenFields {
		if _, ok := raw[field]; ok {
			t.Errorf("forbidden field %q found in JSON output: %s", field, string(data))
		}
	}
}

func TestSession_NoReflectionLeak(t *testing.T) {
	typ := reflect.TypeOf(PostgreSQLQueryAccessSession{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			t.Errorf("PostgreSQLQueryAccessSession must not have exported fields, found: %s", field.Name)
		}
		tag := field.Tag.Get("json")
		if tag != "" && tag != "-" {
			t.Errorf("field %s must not have json tag (except '-'), found: %s", field.Name, tag)
		}
	}
}

func TestSession_DefaultAnalyzeUnchanged(t *testing.T) {
	// Verify that AnalyzeQueryAccess still works normally and does not
	// create a trusted service or promote PostgreSQL admission.
	result, err := AnalyzeQueryAccess(t.Context(), QueryAccessRequest{
		SQL:     "SELECT id, name FROM users WHERE id = 1",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly {
		t.Errorf("expected read_only, got %s", result.ReadClassification)
	}
	if result.Admission != QueryAccessAdmissible {
		t.Errorf("expected admissible, got %s", result.Admission)
	}
}
