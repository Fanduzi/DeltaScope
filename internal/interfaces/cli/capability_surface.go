//go:build !postgresql

package cli

func supportedDialects() []string {
	return []string{"mysql", "tidb"}
}

func rootCommandShort() string {
	return "Offline SQL review for MySQL and TiDB"
}

func dialectFlagDescription() string {
	return "SQL dialect: mysql or tidb"
}
