//go:build postgresql

package cli

func supportedDialects() []string {
	return []string{"mysql", "tidb", "postgresql"}
}

func rootCommandShort() string {
	return "Offline SQL review for MySQL, TiDB, and PostgreSQL"
}

func dialectFlagDescription() string {
	return "SQL dialect: mysql, tidb, or postgresql (requires PG-capable build)"
}
