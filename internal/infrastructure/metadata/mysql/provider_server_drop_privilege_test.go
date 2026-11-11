package mysqlmeta

// T26-SERVER-DROP-EXECUTION-01: the builtin DROP SERVER privilege
// observation — CURRENT_USER() and SHOW GRANTS FOR CURRENT_USER() read
// verbatim on one dedicated connection, every failure propagating
// under the same contract as the other catalog loaders.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// The observation binds the verbatim account and grant lines read on
// the one connection — exact fragments, in order, no synthesis.
func TestLoadMySQLServerDropPrivilegeBindsVerbatim(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select current_user()": {
			columns: []string{"current_user()"},
			rows:    [][]driver.Value{{"audited@172.%"}},
		},
		"show grants for current_user()": {
			columns: []string{"Grants for audited@172.%"},
			rows: [][]driver.Value{
				{"GRANT SELECT, SUPER ON *.* TO 'audited'@'172.%'"},
				{"GRANT 'app_role'@'%' TO 'audited'@'172.%'"},
			},
		},
	})
	provider := NewProvider(db)
	observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if observation == nil || observation.Account != "audited@172.%" {
		t.Fatalf("observation = %+v, want the verbatim account bound", observation)
	}
	if len(observation.GrantLines) != 2 ||
		observation.GrantLines[0] != "GRANT SELECT, SUPER ON *.* TO 'audited'@'172.%'" ||
		observation.GrantLines[1] != "GRANT 'app_role'@'%' TO 'audited'@'172.%'" {
		t.Fatalf("grant lines = %+v, want the verbatim rows in order", observation.GrantLines)
	}
	calls := testQueryCallsByName(testStubName(t))
	if len(calls) != 2 ||
		!strings.Contains(calls[0].query, "current_user()") ||
		!strings.Contains(calls[1].query, "show grants for current_user()") {
		t.Fatalf("queries = %+v, want current_user then show grants on one session", calls)
	}
}

// The compact failure table (R1-C): every read stage — CURRENT_USER
// query/scan/rowset-iteration and the grant query/scan/iteration —
// propagates as an error with the cause preserved, never degrading
// into an unbound or partial observation.
func TestLoadMySQLServerDropPrivilegePropagatesErrors(t *testing.T) {
	accountRow := testQueryResult{
		columns: []string{"current_user()"},
		rows:    [][]driver.Value{{"audited@%"}},
	}
	grantRows := testQueryResult{
		columns: []string{"Grants"},
		rows:    [][]driver.Value{{"GRANT USAGE ON *.* TO 'audited'@'%'"}},
	}
	cases := []struct {
		name    string
		account testQueryResult
		grants  testQueryResult
	}{
		{"current_user query failure", testQueryResult{columns: []string{"current_user()"}, queryErr: errors.New("cu-query-down")}, grantRows},
		{"current_user scan failure", testQueryResult{columns: []string{"current_user()", "extra"}, rows: [][]driver.Value{{"audited@%", "x"}}}, grantRows},
		{"current_user rowset failure", testQueryResult{columns: []string{"current_user()"}, rows: [][]driver.Value{{"audited@%"}}, rowErrAt: 1, rowErr: errors.New("cu-rows-lost")}, grantRows},
		{"grants query failure", accountRow, testQueryResult{columns: []string{"Grants"}, queryErr: errors.New("grants-down")}},
		{"grants scan failure", accountRow, testQueryResult{columns: []string{"Grants", "extra"}, rows: [][]driver.Value{{"GRANT USAGE ON *.* TO 'audited'@'%'", "x"}}}},
		{"grants iteration failure", accountRow, testQueryResult{columns: []string{"Grants"}, rows: [][]driver.Value{{"GRANT USAGE ON *.* TO 'audited'@'%'"}}, rowErrAt: 1, rowErr: errors.New("grants-rows-lost")}},
	}
	for _, tc := range cases {
		db := openTestDB(t, map[string]testQueryResult{
			"select current_user()":          tc.account,
			"show grants for current_user()": tc.grants,
		})
		provider := NewProvider(db)
		observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
		if err == nil || observation != nil {
			t.Fatalf("%s: must propagate, got obs=%+v err=%v", tc.name, observation, err)
		}
	}
	// A returned CURRENT_USER query failure — including the
	// ErrNoRows sentinel a QueryRow-only caller would misread as a
	// successful empty answer — is an error, never an unbound success
	// (R1-C).
	db := openTestDB(t, map[string]testQueryResult{
		"select current_user()": {
			columns:  []string{"current_user()"},
			queryErr: sql.ErrNoRows,
		},
		"show grants for current_user()": {
			columns: []string{"Grants"},
			rows:    [][]driver.Value{{"GRANT USAGE ON *.* TO 'x'@'%'"}},
		},
	})
	provider := NewProvider(db)
	observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
	if err == nil || observation != nil {
		t.Fatalf("deferred query failure must propagate, got obs=%+v err=%v", observation, err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the deferred cause stays reachable: %v", err)
	}
}

// A non-MySQL dialect never touches the connection — the observation
// stays unloaded and no query is issued.
func TestLoadMySQLServerDropPrivilegeDialectGate(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select current_user()": {
			columns: []string{"current_user()"},
			rows:    [][]driver.Value{{"audited@%"}},
		},
	})
	provider := NewProvider(db)
	observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectTiDB)
	if err != nil || observation != nil {
		t.Fatalf("non-mysql dialect must answer nil: obs=%+v err=%v", observation, err)
	}
	if calls := testQueryCallsByName(testStubName(t)); len(calls) != 0 {
		t.Fatalf("non-mysql dialect must issue no queries: %+v", calls)
	}
}

// A genuinely successful-but-empty CURRENT_USER() rowset — zero rows,
// no error — binds an empty account while the grant read still runs.
// The unbound principal stays unprovable, never a read failure; the
// distinction from a deferred query error is the R1-C fix.
func TestLoadMySQLServerDropPrivilegeUnboundAccount(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select current_user()": {
			columns: []string{"current_user()"},
			rows:    [][]driver.Value{},
		},
		"show grants for current_user()": {
			columns: []string{"Grants"},
			rows:    [][]driver.Value{{"GRANT USAGE ON *.* TO 'x'@'%'"}},
		},
	})
	provider := NewProvider(db)
	observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if observation == nil || observation.Account != "" || len(observation.GrantLines) != 1 {
		t.Fatalf("observation = %+v, want empty account plus the grant lines", observation)
	}
}
