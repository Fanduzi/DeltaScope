//go:build postgresql

// Package postgresqlmeta verifies the *sql.Conn-backed query access resolver.
// input: fake and real PostgreSQL connections
// output: regression coverage for conn-backed resolver construction and schema resolution
// pos: test coverage for QueryAccessConnResolver
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"reflect"
	"testing"
)

func TestConnResolver_NoDBField(t *testing.T) {
	typ := reflect.TypeOf(QueryAccessConnResolver{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type == reflect.TypeOf((*interface{})(nil)).Elem() {
			continue
		}
		// Check no field is *sql.DB.
		if field.Type.String() == "*sql.DB" {
			t.Errorf("QueryAccessConnResolver must not have *sql.DB field, found: %s", field.Name)
		}
	}
}

func TestConnResolver_NilConn(t *testing.T) {
	resolver, err := NewQueryAccessConnResolver(nil)
	if err == nil {
		t.Fatal("expected error for nil conn")
	}
	if resolver != nil {
		t.Fatal("expected nil resolver on error")
	}
}
