// Package deltascope verifies the closed query access analysis-profile contract.
// input: query access requests with bounded analysis profiles
// output: profile validation, offline fail-closed behavior, and no-leak guarantees
// pos: public API contract tests for query access analysis profiles
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestQueryAccessAnalysisProfileConstants(t *testing.T) {
	tests := []struct {
		name string
		got  QueryAccessAnalysisProfile
		want string
	}{
		{name: "empty", got: QueryAccessAnalysisProfileEmpty, want: ""},
		{name: "mysql 5.7", got: QueryAccessAnalysisProfileMySQL57, want: "mysql-5.7"},
		{name: "mysql 8.0", got: QueryAccessAnalysisProfileMySQL80, want: "mysql-8.0"},
		{name: "mysql 8.4", got: QueryAccessAnalysisProfileMySQL84, want: "mysql-8.4"},
		{name: "tidb 8.5", got: QueryAccessAnalysisProfileTiDB85, want: "tidb-8.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.want {
				t.Fatalf("profile = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestAnalyzeQueryAccessDefaultProfileRemainsOffline(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT COUNT(*) FROM app.users",
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate {
		t.Fatalf("default profile classification = %q, want indeterminate", result.ReadClassification)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("default profile admission = %q, want indeterminate", result.Admission)
	}
}

func TestAnalyzeQueryAccessValidProfilesRemainOfflineAndFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		profile QueryAccessAnalysisProfile
	}{
		{name: "mysql 5.7", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL57},
		{name: "mysql 8.0", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL80},
		{name: "mysql 8.4", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL84},
		{name: "tidb 8.5", dialect: DialectTiDB, profile: QueryAccessAnalysisProfileTiDB85},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:             "SELECT COUNT(*) FROM app.users",
				Dialect:         tt.dialect,
				AnalysisProfile: tt.profile,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
				t.Fatalf("profiled offline function query promoted: classification=%q admission=%q", result.ReadClassification, result.Admission)
			}
		})
	}
}

func TestAnalyzeQueryAccessRejectsUnknownProfileWithoutEcho(t *testing.T) {
	maliciousProfile := QueryAccessAnalysisProfile("mysql-8.4\nSELECT password FROM secrets")
	_, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:             "SELECT 1",
		Dialect:         DialectMySQL,
		AnalysisProfile: maliciousProfile,
	})
	if !errors.Is(err, ErrInvalidQueryAccessAnalysisProfile) {
		t.Fatalf("error = %v, want ErrInvalidQueryAccessAnalysisProfile", err)
	}
	if strings.Contains(err.Error(), string(maliciousProfile)) || strings.Contains(err.Error(), "password") {
		t.Fatalf("profile validation error leaked input: %q", err)
	}
}

func TestAnalyzeQueryAccessRejectsProfileDialectMismatchWithoutEcho(t *testing.T) {
	_, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:             "SELECT 1",
		Dialect:         DialectTiDB,
		AnalysisProfile: QueryAccessAnalysisProfileMySQL84,
	})
	if !errors.Is(err, ErrQueryAccessAnalysisProfileDialectMismatch) {
		t.Fatalf("error = %v, want ErrQueryAccessAnalysisProfileDialectMismatch", err)
	}
	if strings.Contains(err.Error(), "mysql-8.4") || strings.Contains(err.Error(), "tidb") {
		t.Fatalf("profile mismatch error leaked profile/dialect: %q", err)
	}
}

func TestQueryAccessResultJSONOmitsAnalysisProfile(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         DialectMySQL,
		AnalysisProfile: QueryAccessAnalysisProfileMySQL84,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "profile") {
		t.Fatalf("result JSON leaked analysis profile: %s", data)
	}
}

func TestMySQLTiDBQueryAccessSessionBoundaryValidation(t *testing.T) {
	validProfile := QueryAccessAnalysisProfileMySQL84
	tests := []struct {
		name      string
		req       QueryAccessRequest
		session   *MySQLTiDBQueryAccessSession
		want      error
		forbidden []string
	}{
		{
			name:      "nil session",
			req:       QueryAccessRequest{Dialect: DialectMySQL, AnalysisProfile: validProfile},
			want:      ErrMySQLTiDBQueryAccessSessionUnavailable,
			forbidden: []string{"mysql-8.4", "dsn", "password"},
		},
		{
			name:    "wrong dialect",
			req:     QueryAccessRequest{Dialect: DialectPostgreSQL, AnalysisProfile: QueryAccessAnalysisProfileEmpty},
			session: &MySQLTiDBQueryAccessSession{},
			want:    ErrMySQLTiDBQueryAccessDialectRequired,
		},
		{
			name:    "profile mismatch",
			req:     QueryAccessRequest{Dialect: DialectTiDB, AnalysisProfile: validProfile},
			session: &MySQLTiDBQueryAccessSession{},
			want:    ErrQueryAccessAnalysisProfileDialectMismatch,
		},
		{
			name: "external resolver",
			req: QueryAccessRequest{
				Dialect:         DialectMySQL,
				AnalysisProfile: validProfile,
				SchemaResolver:  testQueryAccessSchemaResolver{},
			},
			session: &MySQLTiDBQueryAccessSession{},
			want:    ErrMySQLTiDBQueryAccessSchemaResolverNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AnalyzeMySQLTiDBQueryAccessWithSession(context.Background(), tt.session, tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(forbidden)) {
					t.Fatalf("error leaked %q: %q", forbidden, err)
				}
			}
		})
	}
}

func TestMySQLTiDBQueryAccessSessionHasNoPublicState(t *testing.T) {
	data, err := json.Marshal(&MySQLTiDBQueryAccessSession{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("session JSON = %s, want empty object", data)
	}
}

type testQueryAccessSchemaResolver struct{}

func (testQueryAccessSchemaResolver) ResolveRelation(context.Context, string, string, string) (QueryAccessRelationSchema, error) {
	return QueryAccessRelationSchema{}, nil
}
