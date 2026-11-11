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

// A failed read propagates — the observation is never degraded into an
// unproven grant or a partial answer.
func TestLoadMySQLServerDropPrivilegePropagatesErrors(t *testing.T) {
	marker := errors.New("grant-channel-down-4f2")
	db := openTestDB(t, map[string]testQueryResult{
		"select current_user()": {
			columns: []string{"current_user()"},
			rows:    [][]driver.Value{{"audited@%"}},
		},
		"show grants for current_user()": {
			columns:  []string{"Grants"},
			queryErr: marker,
		},
	})
	provider := NewProvider(db)
	observation, err := provider.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
	if err == nil || observation != nil {
		t.Fatalf("failed grant read must propagate: obs=%+v err=%v", observation, err)
	}
	if !errors.Is(err, marker) {
		t.Fatalf("error chain lost the loader cause: %v", err)
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

// A zero-row CURRENT_USER() binds an empty account but the grant read
// still runs — the unbound principal marks the observation unprovable,
// never a read failure.
func TestLoadMySQLServerDropPrivilegeUnboundAccount(t *testing.T) {
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if observation == nil || observation.Account != "" || len(observation.GrantLines) != 1 {
		t.Fatalf("observation = %+v, want empty account plus the grant lines", observation)
	}
}
