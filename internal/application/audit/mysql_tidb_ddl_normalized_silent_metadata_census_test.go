package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type normalizedSilentCase struct {
	Name string
	SQL  string
	// Sensitive marks cases that need forbidden-payload assertions.
	Sensitive bool
	// ForbiddenSubstrings are strings that must not appear in any
	// string-valued field of the extracted statement metadata.
	ForbiddenSubstrings []string
}

var mysqlNormalizedSilentCases = []normalizedSilentCase{
	{Name: "RENAME TABLE", SQL: "RENAME TABLE users TO users_old"},
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)"},
	{Name: "ALTER TABLE DROP COLUMN", SQL: "ALTER TABLE users DROP COLUMN email"},
	{Name: "ALTER TABLE MODIFY COLUMN", SQL: "ALTER TABLE users MODIFY COLUMN email varchar(500)"},
	{Name: "ALTER TABLE ADD PRIMARY KEY", SQL: "ALTER TABLE users ADD PRIMARY KEY (id)"},
	{Name: "ALTER TABLE ADD INDEX", SQL: "ALTER TABLE users ADD INDEX idx_email (email)"},
	{Name: "ALTER TABLE DROP INDEX", SQL: "ALTER TABLE users DROP INDEX idx_email"},
	{Name: "ALTER TABLE ADD FOREIGN KEY", SQL: "ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id)"},
	{Name: "ALTER TABLE DROP FOREIGN KEY", SQL: "ALTER TABLE orders DROP FOREIGN KEY fk_user"},
	{Name: "CREATE INDEX", SQL: "CREATE INDEX idx_email ON users (email)"},
	{Name: "CREATE UNIQUE INDEX", SQL: "CREATE UNIQUE INDEX idx_email ON users (email)"},
	{Name: "CREATE FULLTEXT INDEX", SQL: "CREATE FULLTEXT INDEX idx_content ON posts (content)"},
	{Name: "CREATE SPATIAL INDEX", SQL: "CREATE SPATIAL INDEX idx_location ON places (location)"},
	{Name: "DROP INDEX", SQL: "DROP INDEX idx_email ON users"},
	{Name: "ALTER DATABASE", SQL: "ALTER DATABASE app CHARACTER SET utf8mb4"},
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Sensitive: true,
		ForbiddenSubstrings: []string{"SELECT 1"}},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup"},
	{Name: "CREATE USER", SQL: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", Sensitive: true,
		ForbiddenSubstrings: []string{"secret", "IDENTIFIED BY"}},
	{Name: "ALTER USER", SQL: "ALTER USER 'admin'@'%' IDENTIFIED BY 'new_secret'", Sensitive: true,
		ForbiddenSubstrings: []string{"new_secret", "IDENTIFIED BY"}},
	{Name: "DROP USER", SQL: "DROP USER 'admin'@'%'"},
	{Name: "CREATE ROLE", SQL: "CREATE ROLE manager"},
	{Name: "DROP ROLE", SQL: "DROP ROLE manager"},
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Sensitive: true,
		ForbiddenSubstrings: []string{"app.users", "reader"}},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Sensitive: true,
		ForbiddenSubstrings: []string{"app.users", "reader"}},
	{Name: "DROP RESOURCE GROUP", SQL: "DROP RESOURCE GROUP rg1"},
}

var tidbNormalizedSilentCases = []normalizedSilentCase{
	{Name: "RENAME TABLE", SQL: "RENAME TABLE users TO users_old"},
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)"},
	{Name: "ALTER TABLE DROP COLUMN", SQL: "ALTER TABLE users DROP COLUMN email"},
	{Name: "ALTER TABLE MODIFY COLUMN", SQL: "ALTER TABLE users MODIFY COLUMN email varchar(500)"},
	{Name: "ALTER TABLE ADD PRIMARY KEY", SQL: "ALTER TABLE users ADD PRIMARY KEY (id)"},
	{Name: "ALTER TABLE ADD INDEX", SQL: "ALTER TABLE users ADD INDEX idx_email (email)"},
	{Name: "ALTER TABLE DROP INDEX", SQL: "ALTER TABLE users DROP INDEX idx_email"},
	{Name: "ALTER TABLE ADD FOREIGN KEY", SQL: "ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id)"},
	{Name: "ALTER TABLE DROP FOREIGN KEY", SQL: "ALTER TABLE orders DROP FOREIGN KEY fk_user"},
	{Name: "CREATE INDEX", SQL: "CREATE INDEX idx_email ON users (email)"},
	{Name: "CREATE UNIQUE INDEX", SQL: "CREATE UNIQUE INDEX idx_email ON users (email)"},
	{Name: "DROP INDEX", SQL: "DROP INDEX idx_email ON users"},
	{Name: "ALTER DATABASE", SQL: "ALTER DATABASE app CHARACTER SET utf8mb4"},
	{Name: "CREATE PLACEMENT POLICY", SQL: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1' REGIONS='us-east-1'", Sensitive: true,
		ForbiddenSubstrings: []string{"us-east-1"}},
	{Name: "ALTER PLACEMENT POLICY", SQL: "ALTER PLACEMENT POLICY p1 PRIMARY_REGION='us-west-1' REGIONS='us-west-1'", Sensitive: true,
		ForbiddenSubstrings: []string{"us-west-1"}},
	{Name: "DROP PLACEMENT POLICY", SQL: "DROP PLACEMENT POLICY p1"},
	{Name: "CREATE SEQUENCE", SQL: "CREATE SEQUENCE seq1 START WITH 1 INCREMENT BY 1", Sensitive: true,
		ForbiddenSubstrings: []string{"START WITH 1", "INCREMENT BY 1"}},
	{Name: "ALTER SEQUENCE", SQL: "ALTER SEQUENCE seq1 START WITH 100", Sensitive: true,
		ForbiddenSubstrings: []string{"START WITH 100"}},
	{Name: "DROP SEQUENCE", SQL: "DROP SEQUENCE seq1"},
	{Name: "ALTER TABLE PLACEMENT POLICY", SQL: "ALTER TABLE users PLACEMENT POLICY p1"},
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Sensitive: true,
		ForbiddenSubstrings: []string{"SELECT 1"}},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup"},
	{Name: "CREATE USER", SQL: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", Sensitive: true,
		ForbiddenSubstrings: []string{"secret", "IDENTIFIED BY"}},
	{Name: "ALTER USER", SQL: "ALTER USER 'admin'@'%' IDENTIFIED BY 'new_secret'", Sensitive: true,
		ForbiddenSubstrings: []string{"new_secret", "IDENTIFIED BY"}},
	{Name: "DROP USER", SQL: "DROP USER 'admin'@'%'"},
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Sensitive: true,
		ForbiddenSubstrings: []string{"app.users", "reader"}},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Sensitive: true,
		ForbiddenSubstrings: []string{"app.users", "reader"}},
}

func TestMySQLTiDBNormalizedSilentDDLMetadataCensus(t *testing.T) {
	t.Parallel()

	t.Run("MySQL", func(t *testing.T) {
		t.Parallel()
		runNormalizedSilentMetadataCensus(t, "MySQL", spec.DialectMySQL, mysqlNormalizedSilentCases)
	})
	t.Run("TiDB", func(t *testing.T) {
		t.Parallel()
		runNormalizedSilentMetadataCensus(t, "TiDB", spec.DialectTiDB, tidbNormalizedSilentCases)
	})
}

func runNormalizedSilentMetadataCensus(t *testing.T, dialectName string, dialect spec.Dialect, cases []normalizedSilentCase) {
	t.Helper()

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			parsed, parseErr := Parse(context.Background(), tc.SQL, dialect)
			if parseErr != nil {
				t.Fatalf("%s %s: parse failed (expected normalized_silent): %v", dialectName, tc.Name, parseErr)
			}

			statements, extractErr := Extract(context.Background(), parsed)
			if extractErr != nil {
				t.Fatalf("%s %s: extract failed (expected normalized_silent): %v", dialectName, tc.Name, extractErr)
			}

			var supported *spec.Statement
			for i := range statements {
				if statements[i].Unsupported == nil {
					s := statements[i]
					supported = &s
					break
				}
			}
			if supported == nil {
				t.Fatalf("%s %s: no supported statement (expected at least one)", dialectName, tc.Name)
			}

			result, auditErr := AuditSQL(context.Background(), Request{
				SQL:     tc.SQL,
				Dialect: dialect,
			})
			if auditErr != nil {
				t.Fatalf("%s %s: audit error: %v", dialectName, tc.Name, auditErr)
			}

			findingCount := 0
			if result.Statements != nil {
				for _, s := range result.Statements {
					findingCount += len(s.Findings)
				}
			}
			findingCount += len(result.GlobalFindings)

			if findingCount > 0 {
				t.Errorf("%s %s: expected normalized_silent (0 findings), got %d", dialectName, tc.Name, findingCount)
			}

			logMetadataShape(t, dialectName, tc.Name, supported)

			if tc.Sensitive {
				assertNoForbiddenPayload(t, dialectName, tc.Name, supported, tc.ForbiddenSubstrings)
			}
		})
	}
}

func logMetadataShape(t *testing.T, dialectName, caseName string, stmt *spec.Statement) {
	t.Helper()

	t.Logf("metadata-shape: %s %-40s kind=%s", dialectName, caseName, stmt.Kind)

	if stmt.DDL == nil {
		t.Logf("metadata-shape: %s %-40s ddl=nil", dialectName, caseName)
		return
	}

	ddl := stmt.DDL
	t.Logf("metadata-shape: %s %-40s operation=%s object_type=%s object_name=%s",
		dialectName, caseName, ddl.Operation, ddl.ObjectType, ddl.ObjectName)

	if ddl.Table != nil {
		t.Logf("metadata-shape: %s %-40s table=%s", dialectName, caseName, ddl.Table.Name)
	}
	for _, col := range ddl.Columns {
		t.Logf("metadata-shape: %s %-40s column=%s", dialectName, caseName, col.Name)
	}
	for _, idx := range ddl.Indexes {
		t.Logf("metadata-shape: %s %-40s index=%s kind=%s", dialectName, caseName, idx.Name, idx.Kind)
	}
	for _, c := range ddl.Constraints {
		t.Logf("metadata-shape: %s %-40s constraint=%s type=%s", dialectName, caseName, c.Name, c.Type)
	}
	for _, a := range ddl.Alter {
		parts := []string{"action=" + a.Action}
		if a.Name != "" {
			parts = append(parts, "name="+a.Name)
		}
		if a.Column != nil {
			parts = append(parts, "column.old_name="+a.Column.OldName)
		}
		if a.Index != nil {
			parts = append(parts, "index.old_name="+a.Index.OldName)
		}
		t.Logf("metadata-shape: %s %-40s alter=[%s]", dialectName, caseName, strings.Join(parts, " "))
	}
	var optKeys []string
	for k := range ddl.Options {
		optKeys = append(optKeys, k)
	}
	if len(optKeys) > 0 {
		t.Logf("metadata-shape: %s %-40s options_keys=%v", dialectName, caseName, optKeys)
	}
}

func assertNoForbiddenPayload(t *testing.T, dialectName, caseName string, stmt *spec.Statement, forbidden []string) {
	t.Helper()

	if stmt.DDL == nil {
		return
	}

	allStrings := collectDDLStringValues(stmt.DDL)
	for _, f := range forbidden {
		for _, s := range allStrings {
			if strings.Contains(strings.ToLower(s), strings.ToLower(f)) {
				t.Errorf("%s %s: forbidden payload leak: %q contains %q", dialectName, caseName, s, f)
			}
		}
	}
}

func collectDDLStringValues(ddl *spec.DDL) []string {
	var vals []string

	vals = append(vals, ddl.ObjectName)
	vals = append(vals, ddl.ObjectType)

	if ddl.Table != nil {
		vals = append(vals, ddl.Table.Name, ddl.Table.Schema, ddl.Table.Comment)
	}
	for _, col := range ddl.Columns {
		vals = append(vals, col.Name, col.Type, col.Comment)
	}
	for _, idx := range ddl.Indexes {
		vals = append(vals, idx.Name, string(idx.Kind))
	}
	for _, c := range ddl.Constraints {
		vals = append(vals, c.Name, c.Type)
	}
	for _, a := range ddl.Alter {
		vals = append(vals, a.Name, a.Action)
		if a.Column != nil {
			vals = append(vals, a.Column.OldName)
		}
		if a.Index != nil {
			vals = append(vals, a.Index.OldName)
		}
		for _, v := range a.Options {
			vals = append(vals, v)
		}
	}
	for _, v := range ddl.Options {
		vals = append(vals, v)
	}

	return vals
}
