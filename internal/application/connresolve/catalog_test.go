// Package connresolve verifies Transport Connection Resolution catalog binding.
// input: dialect and MySQL/TiDB database, connection schema, and request default hints
// output: catalog and qualifier pairs plus conflict failures
// pos: application tests at the BindMySQLTiDBCatalog seam
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"errors"
	"testing"
)

func TestBindMySQLTiDBCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		dialect          string
		database         string
		connectionSchema string
		requestedSchema  string
		wantCatalog      string
		wantQualifier    string
		wantErr          bool
	}{
		{name: "mysql database only", dialect: "mysql", database: "app", wantCatalog: "app", wantQualifier: "app"},
		{name: "mysql schema only", dialect: "mysql", connectionSchema: "app", wantCatalog: "app", wantQualifier: "app"},
		{name: "mysql matching values", dialect: "mysql", database: "app", connectionSchema: "app", wantCatalog: "app", wantQualifier: "app"},
		{name: "mysql request default fills qualifier", dialect: "mysql", database: "app", requestedSchema: "app", wantCatalog: "app", wantQualifier: "app"},
		{name: "tidb database only", dialect: "tidb", database: "app", wantCatalog: "app", wantQualifier: "app"},
		{name: "postgresql leaves values independent", dialect: "postgresql", database: "appdb", requestedSchema: "public", wantCatalog: "appdb", wantQualifier: "public"},
		{name: "mysql database schema conflict", dialect: "mysql", database: "app", connectionSchema: "other", wantErr: true},
		{name: "mysql catalog request conflict", dialect: "mysql", database: "app", requestedSchema: "other", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			catalog, qualifier, err := BindMySQLTiDBCatalog(tt.dialect, tt.database, tt.connectionSchema, tt.requestedSchema)
			if tt.wantErr {
				if !errors.Is(err, ErrCatalogConflict) {
					t.Fatalf("expected ErrCatalogConflict, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("bind catalog: %v", err)
			}
			if catalog != tt.wantCatalog || qualifier != tt.wantQualifier {
				t.Fatalf("got catalog=%q qualifier=%q, want %q %q", catalog, qualifier, tt.wantCatalog, tt.wantQualifier)
			}
		})
	}
}
