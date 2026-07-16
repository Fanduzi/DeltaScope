// Package queryaccess_test freezes MySQL/TiDB operator-bearing SELECT admission
// so pure-read PostgreSQL work cannot regress cross-dialect behavior (T3).
// input: operator-bearing SELECT SQL for mysql and tidb
// output: read_only + admissible assertions for comparison operators
// pos: application regression characterization (no production changes)
// note: COUNT remains indeterminate on these dialects; only operator SELECT is locked admissible.
package queryaccess_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestMySQLTiDB_OperatorBearingSelectRemainsAdmissible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
	}{
		{name: "eq", sql: "SELECT id FROM users WHERE id = 1"},
		{name: "ne", sql: "SELECT id FROM users WHERE id <> 1"},
		{name: "lt", sql: "SELECT id FROM users WHERE id < 1"},
		{name: "gt", sql: "SELECT id FROM users WHERE id > 1"},
		{name: "le", sql: "SELECT id FROM users WHERE id <= 1"},
		{name: "ge", sql: "SELECT id FROM users WHERE id >= 1"},
		{name: "and_or", sql: "SELECT id FROM users WHERE id = 1 AND name = 'x' OR id > 0"},
	}

	svc := &appqa.Service{}
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		for _, tc := range cases {
			tc := tc
			t.Run(dialect+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				res, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
					SQL:     tc.sql,
					Dialect: dialect,
					Mode:    "strict",
				})
				if err != nil {
					t.Fatalf("analyze: %v", err)
				}
				dr := res.DomainResult
				if dr.ReadClassification != domain.ReadOnly {
					t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
				}
				if dr.Admission != domain.Admissible {
					t.Errorf("admission: got %q, want %q", dr.Admission, domain.Admissible)
				}
				blob := fmt.Sprintf("%+v", dr)
				if strings.Contains(blob, "severity") {
					t.Error("result must not contain severity")
				}
				// Structured result dump should not echo full SQL text.
				if strings.Contains(blob, tc.sql) {
					t.Error("result must not embed raw SQL text")
				}
			})
		}
	}
}

// TestMySQL_SelectWithWhereCorpusContract mirrors the existing corpus fixture
// contract for the canonical operator-bearing SELECT.
func TestMySQL_SelectWithWhereCorpusContract(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}
	res, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: "mysql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	dr := res.DomainResult
	if dr.ReadClassification != domain.ReadOnly || dr.Admission != domain.Admissible {
		t.Fatalf("mysql WHERE equality regression: class=%q admission=%q",
			dr.ReadClassification, dr.Admission)
	}
}
