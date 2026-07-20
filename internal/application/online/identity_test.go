// Package online provides tests for the identity parser.
// input: version strings and expected dialects
// output: validated or rejected ServerIdentity
// pos: unit tests for ParseServerIdentity
// note: if this file changes, update this header and module README.md.
package online

import (
	"errors"
	"testing"
)

func TestParseServerIdentity_SupportedVersions(t *testing.T) {
	tests := []struct {
		name        string
		rawVersion  string
		dialect     string
		wantProduct ProductFamily
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantSeries  VersionSeries
		wantTarget  CapabilityTarget
	}{
		{
			name:        "MySQL 5.7.44-log",
			rawVersion:  "5.7.44-log",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   5,
			wantMinor:   7,
			wantPatch:   44,
			wantSeries:  SeriesMySQL57,
			wantTarget:  TargetMySQL57,
		},
		{
			name:        "MySQL 5.7.38",
			rawVersion:  "5.7.38",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   5,
			wantMinor:   7,
			wantPatch:   38,
			wantSeries:  SeriesMySQL57,
			wantTarget:  TargetMySQL57,
		},
		{
			name:        "MySQL 8.0.46",
			rawVersion:  "8.0.46",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   8,
			wantMinor:   0,
			wantPatch:   46,
			wantSeries:  SeriesMySQL80,
			wantTarget:  TargetMySQL80,
		},
		{
			name:        "MySQL 8.0.39",
			rawVersion:  "8.0.39",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   8,
			wantMinor:   0,
			wantPatch:   39,
			wantSeries:  SeriesMySQL80,
			wantTarget:  TargetMySQL80,
		},
		{
			name:        "MySQL 8.4.10",
			rawVersion:  "8.4.10",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   8,
			wantMinor:   4,
			wantPatch:   10,
			wantSeries:  SeriesMySQL84,
			wantTarget:  TargetMySQL84,
		},
		{
			name:        "TiDB 8.0.11-TiDB-v8.5.7",
			rawVersion:  "8.0.11-TiDB-v8.5.7",
			dialect:     "tidb",
			wantProduct: ProductTiDB,
			wantMajor:   8,
			wantMinor:   5,
			wantPatch:   7,
			wantSeries:  SeriesTiDB85,
			wantTarget:  TargetTiDB85,
		},
		{
			name:        "PostgreSQL 17.4",
			rawVersion:  "PostgreSQL 17.4",
			dialect:     "postgresql",
			wantProduct: ProductPostgreSQL,
			wantMajor:   17,
			wantMinor:   4,
			wantPatch:   0,
			wantSeries:  SeriesPG17,
			wantTarget:  TargetPG17,
		},
		{
			name:        "PostgreSQL 17.0",
			rawVersion:  "PostgreSQL 17.0",
			dialect:     "postgresql",
			wantProduct: ProductPostgreSQL,
			wantMajor:   17,
			wantMinor:   0,
			wantPatch:   0,
			wantSeries:  SeriesPG17,
			wantTarget:  TargetPG17,
		},
		{
			name:        "PostgreSQL 17.4 on ubuntu",
			rawVersion:  "PostgreSQL 17.4 on x86_64-pc-linux-gnu",
			dialect:     "postgresql",
			wantProduct: ProductPostgreSQL,
			wantMajor:   17,
			wantMinor:   4,
			wantPatch:   0,
			wantSeries:  SeriesPG17,
			wantTarget:  TargetPG17,
		},
		{
			name:        "MySQL 8.0.46-commercial",
			rawVersion:  "8.0.46-commercial",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   8,
			wantMinor:   0,
			wantPatch:   46,
			wantSeries:  SeriesMySQL80,
			wantTarget:  TargetMySQL80,
		},
		{
			name:        "MySQL 8.4.10 with suffix",
			rawVersion:  "8.4.10-MySQL Community Server - GPL",
			dialect:     "mysql",
			wantProduct: ProductMySQL,
			wantMajor:   8,
			wantMinor:   4,
			wantPatch:   10,
			wantSeries:  SeriesMySQL84,
			wantTarget:  TargetMySQL84,
		},
		{
			name:        "TiDB with different format",
			rawVersion:  "5.7.25-TiDB-v8.5.1",
			dialect:     "tidb",
			wantProduct: ProductTiDB,
			wantMajor:   8,
			wantMinor:   5,
			wantPatch:   1,
			wantSeries:  SeriesTiDB85,
			wantTarget:  TargetTiDB85,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseServerIdentity(tt.rawVersion, tt.dialect)
			if err != nil {
				t.Fatalf("ParseServerIdentity(%q, %q) unexpected error: %v", tt.rawVersion, tt.dialect, err)
			}
			if id.Product != tt.wantProduct {
				t.Errorf("Product = %q, want %q", id.Product, tt.wantProduct)
			}
			if id.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", id.Major, tt.wantMajor)
			}
			if id.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", id.Minor, tt.wantMinor)
			}
			if id.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", id.Patch, tt.wantPatch)
			}
			if id.Series != tt.wantSeries {
				t.Errorf("Series = %q, want %q", id.Series, tt.wantSeries)
			}
			if id.RawVersion != tt.rawVersion {
				t.Errorf("RawVersion = %q, want %q", id.RawVersion, tt.rawVersion)
			}
			target := DeriveCapabilityTarget(id)
			if target != tt.wantTarget {
				t.Errorf("DeriveCapabilityTarget() = %q, want %q", target, tt.wantTarget)
			}
		})
	}
}

func TestParseServerIdentity_RejectedVersions(t *testing.T) {
	tests := []struct {
		name       string
		rawVersion string
		dialect    string
		wantErr    error
	}{
		{
			name:       "MariaDB 10.6.7",
			rawVersion: "10.6.7-MariaDB-1:10.6.7+maria~focal",
			dialect:    "mysql",
			wantErr:    ErrIdentityUnknown,
		},
		{
			name:       "MySQL 5.6.51 unsupported",
			rawVersion: "5.6.51",
			dialect:    "mysql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "MySQL 8.1.0 unsupported",
			rawVersion: "8.1.0",
			dialect:    "mysql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "MySQL 8.5.0 unsupported",
			rawVersion: "8.5.0",
			dialect:    "mysql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "MySQL 9.0.0 unsupported",
			rawVersion: "9.0.0",
			dialect:    "mysql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "empty version",
			rawVersion: "",
			dialect:    "mysql",
			wantErr:    ErrIdentityMalformed,
		},
		{
			name:       "whitespace only",
			rawVersion: "   ",
			dialect:    "mysql",
			wantErr:    ErrIdentityMalformed,
		},
		{
			name:       "not a version",
			rawVersion: "not-a-version",
			dialect:    "mysql",
			wantErr:    ErrIdentityMalformed,
		},
		{
			name:       "PostgreSQL 16.4 unsupported",
			rawVersion: "PostgreSQL 16.4",
			dialect:    "postgresql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "PostgreSQL 18.0 unsupported",
			rawVersion: "PostgreSQL 18.0",
			dialect:    "postgresql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "PostgreSQL 15.2 unsupported",
			rawVersion: "PostgreSQL 15.2",
			dialect:    "postgresql",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "TiDB 7.5 unsupported",
			rawVersion: "5.7.25-TiDB-v7.5.0",
			dialect:    "tidb",
			wantErr:    ErrIdentityUnsupported,
		},
		{
			name:       "TiDB 8.1 unsupported",
			rawVersion: "8.0.11-TiDB-v8.1.0",
			dialect:    "tidb",
			wantErr:    ErrIdentityUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseServerIdentity(tt.rawVersion, tt.dialect)
			if err == nil {
				t.Fatalf("ParseServerIdentity(%q, %q) expected error, got nil", tt.rawVersion, tt.dialect)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseServerIdentity(%q, %q) error = %v, want %v", tt.rawVersion, tt.dialect, err, tt.wantErr)
			}
		})
	}
}

func TestParseServerIdentity_DialectMismatch(t *testing.T) {
	tests := []struct {
		name            string
		rawVersion      string
		expectedDialect string
	}{
		{
			name:            "MySQL version with postgresql dialect",
			rawVersion:      "8.0.46",
			expectedDialect: "postgresql",
		},
		{
			name:            "PostgreSQL version with mysql dialect",
			rawVersion:      "PostgreSQL 17.4",
			expectedDialect: "mysql",
		},
		{
			name:            "TiDB version with postgresql dialect",
			rawVersion:      "8.0.11-TiDB-v8.5.7",
			expectedDialect: "postgresql",
		},
		{
			name:            "PostgreSQL version with tidb dialect",
			rawVersion:      "PostgreSQL 17.4",
			expectedDialect: "tidb",
		},
		{
			name:            "MySQL version with tidb dialect",
			rawVersion:      "5.7.44-log",
			expectedDialect: "tidb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseServerIdentity(tt.rawVersion, tt.expectedDialect)
			if err == nil {
				t.Fatalf("ParseServerIdentity(%q, %q) expected error, got nil", tt.rawVersion, tt.expectedDialect)
			}
			if !errors.Is(err, ErrDialectMismatch) {
				t.Errorf("ParseServerIdentity(%q, %q) error = %v, want %v", tt.rawVersion, tt.expectedDialect, err, ErrDialectMismatch)
			}
		})
	}
}

func TestParseServerIdentity_EmptyDialectAcceptsAny(t *testing.T) {
	// When expectedDialect is empty, any supported product should be accepted.
	tests := []struct {
		name       string
		rawVersion string
		wantProd   ProductFamily
	}{
		{"mysql", "8.0.46", ProductMySQL},
		{"tidb", "8.0.11-TiDB-v8.5.7", ProductTiDB},
		{"postgresql", "PostgreSQL 17.4", ProductPostgreSQL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseServerIdentity(tt.rawVersion, "")
			if err != nil {
				t.Fatalf("ParseServerIdentity(%q, \"\") unexpected error: %v", tt.rawVersion, err)
			}
			if id.Product != tt.wantProd {
				t.Errorf("Product = %q, want %q", id.Product, tt.wantProd)
			}
		})
	}
}

func TestDeriveCapabilityTarget_NilIdentity(t *testing.T) {
	target := DeriveCapabilityTarget(nil)
	if target != "" {
		t.Errorf("DeriveCapabilityTarget(nil) = %q, want empty", target)
	}
}
