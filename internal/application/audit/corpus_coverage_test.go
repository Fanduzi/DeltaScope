package audit

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLCorpusCoversSupportedRuleDialects(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")
	files, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}

	covered, _, err := corpusCoveredRuleDialects(files)
	if err != nil {
		t.Fatal(err)
	}
	ruleIDs := corpusDefaultRuleIDs()

	var missing []string
	for _, ruleID := range ruleIDs {
		for _, dialect := range corpusRuleDialectTargets(ruleID) {
			if covered[ruleID][dialect] == "" {
				missing = append(missing, ruleID+"@"+dialect)
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("sql corpus missing supported rule coverage (%d):\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

func corpusRuleDialectTargets(ruleID string) []string {
	if isDeferredCorpusCoverageRule(ruleID) {
		return nil
	}
	switch ruleID {
	case "ddl.alter.merge.mysql.require":
		return []string{"mysql"}
	case "ddl.alter.merge.tidb.require":
		return []string{"tidb"}
	}
	if strings.HasPrefix(ruleID, "ddl.pg.") || isPostgreSQLOnlyRule(ruleID) {
		return []string{"postgresql"}
	}
	if isMySQLFamilyOnlyRule(ruleID) {
		return []string{"mysql", "tidb"}
	}
	return []string{"mysql", "tidb", "postgresql"}
}

func isPostgreSQLOnlyRule(ruleID string) bool {
	switch ruleID {
	case
		"ddl.alter.set_data_type.forbid",
		"ddl.alter.set_default.forbid",
		"ddl.alter.drop_default.forbid",
		"ddl.alter.set_not_null.forbid",
		"ddl.alter.drop_not_null.forbid",
		"ddl.alter.drop_expression.forbid",
		"ddl.alter.set_generated.forbid",
		"ddl.alter.drop_identity.forbid",
		"ddl.alter.set_default.explicit_default_change.forbid",
		"ddl.alter.drop_default.explicit_default_change.forbid",
		"ddl.alter.set_not_null.explicit_nullability_change.forbid",
		"ddl.alter.drop_not_null.explicit_nullability_change.forbid":
		return true
	default:
		return false
	}
}

func isMySQLFamilyOnlyRule(ruleID string) bool {
	switch ruleID {
	case
		"ddl.table.primary_key.not_null.require",
		"ddl.table.primary_key.unsigned.require",
		"ddl.table.primary_key.auto_increment.require",
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment.init_value.require",
		"ddl.table.row_size.max_bytes.require",
		"ddl.table.partition.forbid",
		"ddl.table.drop.adaptive_hash.warn",
		"ddl.table.truncate.adaptive_hash.warn",
		"ddl.table.comment.max_length",
		"ddl.table.create_as.forbid",
		"ddl.table.create_like.forbid",
		"ddl.table.name.max_length",
		"ddl.column.charset.allowlist",
		"ddl.column.collation.allowlist",
		"ddl.column.charset_collation.match.require",
		"ddl.column.bit.forbid",
		"ddl.column.char.max_length",
		"ddl.column.float_double.forbid",
		"ddl.column.json.forbid",
		"ddl.column.name.max_length",
		"ddl.column.timestamp.forbid",
		"ddl.column.varchar.max_length",
		"ddl.constraint.primary_key.name.contains.require",
		"ddl.constraint.primary_key.name.prefix.require",
		"ddl.constraint.primary_key.name.suffix.require",
		"ddl.index.duplicate.forbid",
		"ddl.index.name.keyword.forbid",
		"ddl.index.redundant_left_prefix.forbid",
		"ddl.index.redundant_unique_overlap.forbid",
		"ddl.index.total.max_count",
		"ddl.index.secondary.suffix.require",
		"ddl.index.secondary.contains.require",
		"ddl.index.fulltext.prefix.require",
		"ddl.index.fulltext.suffix.require",
		"ddl.index.fulltext.contains.require",
		"ddl.index.key_length.max_bytes.require",
		"ddl.alter.change_column.forbid",
		"ddl.alter.modify_column.forbid",
		"ddl.alter.add_column.exists.forbid",
		"ddl.alter.drop_column.forbid",
		"ddl.alter.drop_column.exists.require",
		"ddl.alter.drop_index.forbid",
		"ddl.alter.drop_index.exists.require",
		"ddl.alter.drop_primary_key.forbid",
		"ddl.alter.drop_primary_key.exists.require",
		"ddl.alter.rename_index.forbid",
		"ddl.alter.rename_index.exists.require",
		"ddl.alter.modify_column.target_type_family.allowlist",
		"ddl.alter.change_column.target_type_family.allowlist",
		"ddl.alter.modify_column.compatibility.require",
		"ddl.alter.change_column.compatibility.require",
		"ddl.alter.modify_column.exists.require",
		"ddl.alter.change_column.exists.require",
		"ddl.alter.table_option.compatibility.require",
		"ddl.alter.modify_column.explicit_nullability_change.forbid",
		"ddl.alter.change_column.explicit_nullability_change.forbid",
		"ddl.alter.modify_column.explicit_default_change.forbid",
		"ddl.alter.change_column.explicit_default_change.forbid",
		"ddl.alter.modify_column.explicit_auto_increment_change.forbid",
		"ddl.alter.change_column.explicit_auto_increment_change.forbid",
		"ddl.alter.add_index.secondary.prefix.require",
		"ddl.alter.add_index.secondary.suffix.require",
		"ddl.alter.add_index.secondary.contains.require",
		"ddl.alter.add_index.fulltext.prefix.require",
		"ddl.alter.add_index.fulltext.suffix.require",
		"ddl.alter.add_index.fulltext.contains.require",
		"ddl.alter.add_index.columns.max_count",
		"ddl.alter.add_index.duplicate.forbid",
		"ddl.alter.add_index.redundant_left_prefix.forbid",
		"ddl.alter.add_index.unique.suffix.require",
		"ddl.alter.add_index.unique.contains.require",
		"dml.limit.forbid",
		"dml.order_by.forbid",
		"dml.join.on.require",
		"dml.insert.rows.max_count",
		"dml.replace.forbid",
		"dml.insert.on_duplicate.forbid",
		"dml.subquery.forbid":
		return true
	default:
		return false
	}
}

func isDeferredCorpusCoverageRule(ruleID string) bool {
	switch ruleID {
	case
		"ddl.alter.add_index.redundant_unique_overlap.forbid",
		"ddl.pg.alter.add_column.non_null_default.rewrite.warn",
			// Task 4 removes these after corpus fixtures are added.
			"ddl.pg.alter.replica_identity_full.warn",
			"ddl.pg.alter.replica_identity_nothing.warn",
			"ddl.pg.alter.replica_identity_using_index.notice":
		return true
	default:
		return false
	}
}
