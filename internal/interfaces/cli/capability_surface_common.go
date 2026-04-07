package cli

func productDialects() []string {
	return []string{"mysql", "tidb", "postgresql"}
}

func unifiedRootCommandShort() string {
	return "Offline SQL review for MySQL, TiDB, and PostgreSQL"
}

func dialectFlagPrefix() string {
	return "SQL dialect: mysql, tidb, or postgresql"
}
