package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type parserErrorUnsupportedContractCase struct {
	Dialect   spec.Dialect
	Name      string
	SQL       string
	Forbidden []string
}

func assertParserErrorUnsupportedContract(t *testing.T, tc parserErrorUnsupportedContractCase) {
	t.Helper()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     tc.SQL,
		Dialect: tc.Dialect,
	})
	if err == nil {
		t.Fatalf("%s: expected parser-error diagnostic, got nil error and result=%#v", tc.Name, result)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("%s: parser-error SQL must not produce findings: %#v", tc.Name, result.GlobalFindings)
	}
	for i, sr := range result.Statements {
		if len(sr.Findings) != 0 {
			t.Fatalf("%s: parser-error SQL must not produce statement findings [stmt %d]: %#v", tc.Name, i, sr.Findings)
		}
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("%s: parser-error SQL must not fabricate unsupported details: %#v", tc.Name, result.Unsupported)
	}

	msg := strings.ToLower(err.Error())
	for _, required := range []string{"parse", "audit"} {
		if !strings.Contains(msg, required) {
			t.Fatalf("%s: expected error to mention %q, got %q", tc.Name, required, err.Error())
		}
	}
	for _, forbidden := range tc.Forbidden {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("%s: parser-error diagnostic leaked forbidden payload %q in %q", tc.Name, forbidden, err.Error())
		}
	}
}

func TestDDLParserErrorUnsupportedContractMySQLTiDB(t *testing.T) {
	t.Parallel()

	cases := []parserErrorUnsupportedContractCase{
		{
			Dialect:   spec.DialectMySQL,
			Name:      "mysql_create_function_body_not_audited",
			SQL:       "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'",
			Forbidden: []string{"secret_body_value"},
		},
		{
			Dialect:   spec.DialectMySQL,
			Name:      "mysql_create_event_schedule_not_audited",
			SQL:       "CREATE EVENT e_cleanup ON SCHEDULE EVERY 1 DAY DO CALL p_cleanup()",
			Forbidden: []string{"EVERY 1 DAY", "p_cleanup"},
		},
		{
			Dialect:   spec.DialectTiDB,
			Name:      "tidb_alter_table_ttl_not_audited",
			SQL:       "ALTER TABLE users TTL = 7 DAY",
			Forbidden: []string{"7 DAY"},
		},
		{
			Dialect:   spec.DialectTiDB,
			Name:      "tidb_alter_table_locality_not_audited",
			SQL:       "ALTER TABLE users LOCALITY = 'region=us-east-1'",
			Forbidden: []string{"us-east-1"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			assertParserErrorUnsupportedContract(t, tc)
		})
	}
}
