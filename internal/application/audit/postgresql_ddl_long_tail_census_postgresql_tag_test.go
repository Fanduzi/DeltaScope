//go:build postgresql

package audit

import "testing"

// PostgreSQL DDL Long-Tail Census (v0.100.0 Task 1)
//
// Characterizes remaining PostgreSQL DDL long-tail object families.
// Each case is classified as one of:
//   - finding_covered:      audit already emits findings
//   - normalized_silent:    parser/extractor normalizes the form but no rule fires
//   - unsupported_boundary: parser succeeds but extractor marks the statement unsupported
//   - parser_error:         PG parser cannot parse the statement
//
// Observed baseline: 0 parser_error, 0 finding_covered,
// 11 normalized_silent (all ALTER ... SET SCHEMA variants),
// 46 unsupported_boundary (everything else).
//
// This file must not modify production code. It only observes current behavior.

var pgDDLLongTailCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// ===== 1. Text search configuration / dictionary / parser / template =====

	{Name: "CREATE TEXT SEARCH CONFIGURATION",
		SQL:      "CREATE TEXT SEARCH CONFIGURATION english_copy ( COPY = pg_catalog.english )",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH CONFIGURATION RENAME TO",
		SQL:      "ALTER TEXT SEARCH CONFIGURATION english_copy RENAME TO english_copy_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH CONFIGURATION OWNER TO",
		SQL:      "ALTER TEXT SEARCH CONFIGURATION english_copy OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH CONFIGURATION SET SCHEMA",
		SQL:      "ALTER TEXT SEARCH CONFIGURATION english_copy SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TEXT SEARCH CONFIGURATION",
		SQL:      "DROP TEXT SEARCH CONFIGURATION english_copy",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE TEXT SEARCH DICTIONARY",
		SQL:      "CREATE TEXT SEARCH DICTIONARY simple_dict (TEMPLATE = simple)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH DICTIONARY RENAME TO",
		SQL:      "ALTER TEXT SEARCH DICTIONARY simple_dict RENAME TO simple_dict_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH DICTIONARY OWNER TO",
		SQL:      "ALTER TEXT SEARCH DICTIONARY simple_dict OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH DICTIONARY SET SCHEMA",
		SQL:      "ALTER TEXT SEARCH DICTIONARY simple_dict SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TEXT SEARCH DICTIONARY",
		SQL:      "DROP TEXT SEARCH DICTIONARY simple_dict",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE TEXT SEARCH PARSER",
		SQL:      "CREATE TEXT SEARCH PARSER parser_name (START = start_func, GETTOKEN = token_func, END = end_func, LEXTYPES = lextype_func)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH PARSER RENAME TO",
		SQL:      "ALTER TEXT SEARCH PARSER parser_name RENAME TO parser_name_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH PARSER SET SCHEMA",
		SQL:      "ALTER TEXT SEARCH PARSER parser_name SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TEXT SEARCH PARSER",
		SQL:      "DROP TEXT SEARCH PARSER parser_name",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE TEXT SEARCH TEMPLATE",
		SQL:      "CREATE TEXT SEARCH TEMPLATE template_name (LEXIZE = lexize_func)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH TEMPLATE RENAME TO",
		SQL:      "ALTER TEXT SEARCH TEMPLATE template_name RENAME TO template_name_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TEXT SEARCH TEMPLATE SET SCHEMA",
		SQL:      "ALTER TEXT SEARCH TEMPLATE template_name SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TEXT SEARCH TEMPLATE",
		SQL:      "DROP TEXT SEARCH TEMPLATE template_name",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== 2. Collation lifecycle =====

	{Name: "CREATE COLLATION",
		SQL:      "CREATE COLLATION app_collation (provider = libc, locale = 'C')",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER COLLATION RENAME TO",
		SQL:      "ALTER COLLATION app_collation RENAME TO app_collation_v2",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER COLLATION OWNER TO",
		SQL:      "ALTER COLLATION app_collation OWNER TO app_owner",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER COLLATION SET SCHEMA",
		SQL:      "ALTER COLLATION app_collation SET SCHEMA app",
		Expected: ddlCoverageFindingCovered},
	{Name: "DROP COLLATION",
		SQL:      "DROP COLLATION app_collation",
		Expected: ddlCoverageFindingCovered},

	// ===== 3. Extended statistics lifecycle =====

	{Name: "CREATE STATISTICS",
		SQL:      "CREATE STATISTICS users_stats ON email, status FROM users",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER STATISTICS RENAME TO",
		SQL:      "ALTER STATISTICS users_stats RENAME TO users_stats_v2",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER STATISTICS OWNER TO",
		SQL:      "ALTER STATISTICS users_stats OWNER TO app_owner",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER STATISTICS SET SCHEMA",
		SQL:      "ALTER STATISTICS users_stats SET SCHEMA app",
		Expected: ddlCoverageFindingCovered},
	{Name: "DROP STATISTICS",
		SQL:      "DROP STATISTICS users_stats",
		Expected: ddlCoverageFindingCovered},

	// ===== 4. Aggregate / operator / conversion lifecycle =====

	{Name: "CREATE AGGREGATE",
		SQL:      "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER AGGREGATE RENAME TO",
		SQL:      "ALTER AGGREGATE sum2(integer) RENAME TO sum2_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER AGGREGATE OWNER TO",
		SQL:      "ALTER AGGREGATE sum2(integer) OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER AGGREGATE SET SCHEMA",
		SQL:      "ALTER AGGREGATE sum2(integer) SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP AGGREGATE",
		SQL:      "DROP AGGREGATE sum2(integer)",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE OPERATOR",
		SQL:      "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR OWNER TO",
		SQL:      "ALTER OPERATOR === (integer, integer) OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR SET SCHEMA",
		SQL:      "ALTER OPERATOR === (integer, integer) SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP OPERATOR",
		SQL:      "DROP OPERATOR === (integer, integer)",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE CONVERSION",
		SQL:      "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER CONVERSION RENAME TO",
		SQL:      "ALTER CONVERSION conv RENAME TO conv_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER CONVERSION OWNER TO",
		SQL:      "ALTER CONVERSION conv OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER CONVERSION SET SCHEMA",
		SQL:      "ALTER CONVERSION conv SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP CONVERSION",
		SQL:      "DROP CONVERSION conv",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== 5. Operator class / operator family lifecycle =====

	{Name: "CREATE OPERATOR FAMILY",
		SQL:      "CREATE OPERATOR FAMILY int4_ops_family USING btree",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR FAMILY RENAME TO",
		SQL:      "ALTER OPERATOR FAMILY int4_ops_family USING btree RENAME TO int4_ops_family_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR FAMILY OWNER TO",
		SQL:      "ALTER OPERATOR FAMILY int4_ops_family USING btree OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR FAMILY SET SCHEMA",
		SQL:      "ALTER OPERATOR FAMILY int4_ops_family USING btree SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP OPERATOR FAMILY",
		SQL:      "DROP OPERATOR FAMILY int4_ops_family USING btree",
		Expected: ddlCoverageUnsupportedBoundary},

	{Name: "CREATE OPERATOR CLASS",
		SQL:      "CREATE OPERATOR CLASS int4_ops_class DEFAULT FOR TYPE int4 USING btree FAMILY int4_ops_family AS OPERATOR 1 < (int4, int4)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR CLASS RENAME TO",
		SQL:      "ALTER OPERATOR CLASS int4_ops_class USING btree RENAME TO int4_ops_class_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR CLASS OWNER TO",
		SQL:      "ALTER OPERATOR CLASS int4_ops_class USING btree OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER OPERATOR CLASS SET SCHEMA",
		SQL:      "ALTER OPERATOR CLASS int4_ops_class USING btree SET SCHEMA app",
		Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP OPERATOR CLASS",
		SQL:      "DROP OPERATOR CLASS int4_ops_class USING btree",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== 6. Boundary candidates =====

	{Name: "CREATE TRANSFORM",
		SQL:      "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP TRANSFORM",
		SQL:      "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE ACCESS METHOD",
		SQL:      "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP ACCESS METHOD",
		SQL:      "DROP ACCESS METHOD heap2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER LARGE OBJECT OWNER TO",
		SQL:      "ALTER LARGE OBJECT 12345 OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
}

func TestPostgreSQLDDLLongTailCensus(t *testing.T) {
	t.Parallel()
	runDDLCensus(t, "PostgreSQL", pgDDLLongTailCensusCases)
}
