//go:build postgresql

package audit

var defaultPolicyDialectHygienePostgreSQLForbiddenRuleIDs = []string{
	"ddl.table.primary_key.unsigned.require",
	"ddl.table.primary_key.auto_increment.require",
	"ddl.table.charset.allowlist",
	"ddl.column.charset.allowlist",
	"ddl.column.collation.allowlist",
	"ddl.column.charset_collation.match.require",
	"ddl.table.engine.allowlist",
	"ddl.table.row_format.allowlist",
	"ddl.table.primary_key.bigint.require",
}

var defaultPolicyDialectHygienePostgreSQLForbiddenTokens = []string{
	"UNSIGNED",
	"AUTO_INCREMENT",
	"auto_increment",
	"CHARSET",
	"charset",
	"COLLATE",
	"collation",
	"ENGINE",
	"ROW_FORMAT",
	"ON UPDATE CURRENT_TIMESTAMP",
	"adaptive hash",
	"large prefix",
}

func serviceMetadataValueEqual(a, b any) bool {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	return a == b
}
