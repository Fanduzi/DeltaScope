#!/usr/bin/env python3
# input: release docs, landing release cards, and versioned release facts
# output: release-surface consistency errors or a labeled fact summary
# pos: static release contract checker for cross-surface facts
# note: if this file changes, update this header and scripts/README.md.
"""Release surface consistency checker.

Validates that release-domain facts are consistent across all release surfaces:
landing page, release notes EN/ZH, changelog, roadmap, rules reference,
capability matrix.
"""

import os
import re
import sys
from pathlib import Path


SQL_CORPUS_METRIC_LABEL = "supported rule-and-dialect fixture coverage"
SQL_CORPUS_METRIC_LABEL_ZH = "支持的 rule-and-dialect fixture coverage"


RELEASE_FACTS = {
    "v0.510.2": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 586,
            "covered_rule_dialect_targets": 586,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 286,
            "metric_label": SQL_CORPUS_METRIC_LABEL,
            "metric_label_zh": SQL_CORPUS_METRIC_LABEL_ZH,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 407,
            "mysql_entries": 62,
            "tidb_entries": 55,
            "postgresql_entries": 290,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 373,
            "level_blocker": 73,
            "level_warning": 142,
            "level_notice": 158,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 362,
            "kind_dml": 11,
        },
    },
    "v0.510.1": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 586,
            "covered_rule_dialect_targets": 586,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 286,
            "metric_label": SQL_CORPUS_METRIC_LABEL,
            "metric_label_zh": SQL_CORPUS_METRIC_LABEL_ZH,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 407,
            "mysql_entries": 62,
            "tidb_entries": 55,
            "postgresql_entries": 290,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 373,
            "level_blocker": 73,
            "level_warning": 142,
            "level_notice": 158,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 362,
            "kind_dml": 11,
        },
    },
    "v0.510.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 586,
            "covered_rule_dialect_targets": 586,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 286,
            "metric_label": SQL_CORPUS_METRIC_LABEL,
            "metric_label_zh": SQL_CORPUS_METRIC_LABEL_ZH,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 407,
            "mysql_entries": 62,
            "tidb_entries": 55,
            "postgresql_entries": 290,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 373,
            "level_blocker": 73,
            "level_warning": 142,
            "level_notice": 158,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 362,
            "kind_dml": 11,
        },
    },
    "v0.500.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 586,
            "covered_rule_dialect_targets": 586,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 286,
            "metric_label": SQL_CORPUS_METRIC_LABEL,
            "metric_label_zh": SQL_CORPUS_METRIC_LABEL_ZH,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 407,
            "mysql_entries": 62,
            "tidb_entries": 55,
            "postgresql_entries": 290,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 373,
            "level_blocker": 73,
            "level_warning": 142,
            "level_notice": 158,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 362,
            "kind_dml": 11,
        },
    },
    "v0.490.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
            "metric_label": SQL_CORPUS_METRIC_LABEL,
            "metric_label_zh": SQL_CORPUS_METRIC_LABEL_ZH,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.480.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.470.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.460.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.450.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.440.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.430.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.420.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.410.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.400.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.390.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.380.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.370.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 247,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.360.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.340.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.330.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.320.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.310.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.300.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.290.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
        "rule_catalog": {
            "total_rules": 371,
            "level_blocker": 72,
            "level_warning": 142,
            "level_notice": 157,
            "dialect_common": 177,
            "dialect_postgresql": 191,
            "dialect_mysql": 1,
            "dialect_tidb": 2,
            "kind_ddl": 361,
            "kind_dml": 10,
        },
    },
    "v0.280.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
    },
    "v0.270.0": {
        "pg_alter_table_config_entries": 53,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
        "ddl_coverage_catalog": {
            "total_entries": 400,
            "mysql_entries": 61,
            "tidb_entries": 54,
            "postgresql_entries": 285,
            "parser_upgrade_candidate_count": 18,
        },
    },
    "v0.260.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.251.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.250.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.242.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.241.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.240.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.230.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.220.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.210.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.200.0": {
        "pg_alter_table_rule_count": 32,
        "sql_corpus": {
            "supported_rule_dialect_targets": 582,
            "covered_rule_dialect_targets": 582,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 245,
        },
        "required_rule_ids": [],
    },
    "v0.190.0": {
        "pg_alter_table_rule_count": 32,
        "residual_census": {
            "total": 66,
            "finding_covered": 60,
            "normalized_silent": 2,
            "unsupported_boundary": 0,
            "parser_error": 4,
            "unclassified": 0,
        },
        "sql_corpus": {
            "supported_rule_dialect_targets": 535,
            "covered_rule_dialect_targets": 535,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 243,
        },
        "required_rule_ids": [],
    },
    "v0.180.0": {
        "pg_alter_table_rule_count": 32,
        "residual_census": {
            "total": 66,
            "finding_covered": 60,
            "normalized_silent": 2,
            "unsupported_boundary": 0,
            "parser_error": 4,
            "unclassified": 0,
        },
        "sql_corpus": {
            "supported_rule_dialect_targets": 535,
            "covered_rule_dialect_targets": 535,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 243,
        },
        "required_rule_ids": [],
    },
    "v0.170.0": {
        "pg_alter_table_rule_count": 32,
        "residual_census": {
            "total": 66,
            "finding_covered": 60,
            "normalized_silent": 2,
            "unsupported_boundary": 0,
            "parser_error": 4,
            "unclassified": 0,
        },
        "sql_corpus": {
            "supported_rule_dialect_targets": 535,
            "covered_rule_dialect_targets": 535,
            "coverage_percent": "100.0",
            "expected_yaml_files_total": 243,
        },
        "required_rule_ids": [
            "ddl.pg.alter.set_expression.notice",
            "ddl.pg.alter.add_identity.notice",
            "ddl.pg.alter.add_exclusion_constraint.notice",
            "ddl.pg.alter.move_all_tablespace.notice",
        ],
    },
}

STALE_CENSUS_CLAIMS = [
    "64 of 66",
    "finding_covered=64",
    "finding_covered 64",
    "60→64",
    "60 -> 64",
    "60 to 64",
    "60 到 64",
    "unsupported_boundary 0→0",
    "unsupported_boundary 0 -> 0",
    "unchanged at 0",
    "0 (unchanged)",
]

OVERCLAIM_PATTERNS = [
    "Full PostgreSQL ALTER TABLE support",
    "Complete PostgreSQL ALTER TABLE support",
    "PostgreSQL 18 parser support",
    "Runtime validation",
    "Live validation",
    "Rewrite duration estimate",
]

OVERCLAIM_NEGATIVE_MARKERS = [
    "Not", "No", "non-goal", "Non-goal", "deferred",
    "不", "不会", "未", "非目标", "不支持",
]

FORBIDDEN_PAYLOAD_TERMS = [
    "raw_sql", "first_name", "last_name", "||",
    "sequence_options", "exclusions", "operator_class",
    "predicate", "where_clause", "room_id", "during", "&&",
]

NO_LEAK_MARKERS = [
    "No-leak", "No leak", "no-leak",
    "not emitted", "not exposed", "never emitted",
    "不泄漏", "不会输出", "不输出", "非目标",
]

RELEASE_SURFACE_TEMPLATES = [
    "docs/releases/release-notes-{version}.md",
    "docs/releases/release-notes-{version}.zh-CN.md",
    "CHANGELOG.md",
    "docs/roadmap.md",
    "docs/landing/index.html",
    "docs/reference/rules.md",
    "docs/reference/rules.zh-CN.md",
    "docs/reference/audit-capability-matrix.md",
    "docs/reference/audit-capability-matrix.zh-CN.md",
]

# Templates for files scanned by overclaim and no-leak checks.
# Scoped to version-specific content; reference docs and landing are excluded.
_SCOPED_SCAN_TEMPLATES = [
    "docs/releases/release-notes-{version}.md",
    "docs/releases/release-notes-{version}.zh-CN.md",
    "CHANGELOG.md",
    "docs/roadmap.md",
]


class ReleaseConsistencyError(Exception):
    """Raised when a release surface consistency check fails."""


def _read_file(root, rel):
    p = root / rel
    if p.exists():
        return p.read_text(encoding="utf-8")
    return ""


def _extract_release_versions(text):
    versions = []
    seen = set()
    for m in re.finditer(r"\[v(\d+\.\d+\.\d+)\]", text):
        v = "v" + m.group(1)
        if v not in seen:
            versions.append(v)
            seen.add(v)
    return versions


def _extract_landing_recent_cards(text):
    return re.findall(
        r'<div class="release-card-version">(v\d+\.\d+\.\d+)</div>', text
    )


def _validate_landing_release_card_fields(text, errors):
    for card in re.findall(
        r'<article class="release-card">(.*?)</article>', text, re.DOTALL
    ):
        version_match = re.search(
            r'<div class="release-card-version">(v\d+\.\d+\.\d+)</div>',
            card,
        )
        if version_match is None:
            errors.append("docs/landing/index.html release card has no version")
            continue

        version = version_match.group(1)
        key = version.replace(".", "")
        expected = (
            f'data-i18n="releases.labels.{key}"',
            f'data-i18n="releases.items.{key}"',
            f'data-i18n="releases.links.{key}"',
            rf'href="[^"]*release-notes-{re.escape(version)}\.md"',
            rf'data-i18n-href-en="[^"]*release-notes-{re.escape(version)}\.md"',
            rf'data-i18n-href-zh="[^"]*release-notes-{re.escape(version)}\.zh-CN\.md"',
        )
        for field in expected:
            if not re.search(field, card):
                errors.append(
                    f"docs/landing/index.html release card {version} missing {field}"
                )


def _surface_files(version):
    return [t.format(version=version) for t in RELEASE_SURFACE_TEMPLATES]


def _extract_version_section(content, version):
    """Extract the section for a specific version from CHANGELOG-style content."""
    lines = content.split("\n")
    start = None
    for i, line in enumerate(lines):
        if re.match(
            rf"^## \[?{re.escape(version)}\]?(?:\s|$)", line
        ):
            start = i
            break

    if start is None:
        return content

    end = len(lines)
    for i in range(start + 1, len(lines)):
        if re.match(r"^## [^#]", lines[i]):
            end = i
            break

    return "\n".join(lines[start:end])


def _extract_first_section(content):
    """Extract the first ## section from roadmap-style content."""
    lines = content.split("\n")
    start = None
    for i, line in enumerate(lines):
        if re.match(r"^## [^#]", line):
            start = i
            break

    if start is None:
        return content

    end = len(lines)
    for i in range(start + 1, len(lines)):
        if re.match(r"^## [^#]", lines[i]):
            end = i
            break

    return "\n".join(lines[start:end])


def _scoped_scan_content(root, version, template):
    """Get version-scoped content for a template, or None to skip."""
    rel = template.format(version=version)
    content = _read_file(root, rel)
    if not content:
        return None, rel

    if rel == "CHANGELOG.md":
        return _extract_version_section(content, version), rel
    if rel == "docs/roadmap.md":
        return _extract_first_section(content), rel

    return content, rel


def _validate_release_sequence(root, version, errors):
    readme = _read_file(root, "docs/releases/README.md")
    versions = _extract_release_versions(readme)

    if not versions:
        errors.append("docs/releases/README.md contains no version entries")
        return

    if versions[0] != version:
        errors.append(
            f"docs/releases/README.md first version is "
            f"{versions[0]}, expected {version}"
        )

    if len(versions) < 4:
        errors.append(
            f"docs/releases/README.md has {len(versions)} versions, "
            f"need at least 4 (current + 3 historical)"
        )
        return

    landing = _read_file(root, "docs/landing/index.html")
    recent_cards = _extract_landing_recent_cards(landing)
    _validate_landing_release_card_fields(landing, errors)

    if len(recent_cards) != 3:
        errors.append(
            f"docs/landing/index.html has {len(recent_cards)} "
            f"recent release cards, expected 3"
        )

    expected_recent = versions[1:4]
    if recent_cards != expected_recent:
        errors.append(
            f"docs/landing/index.html recent cards {recent_cards} "
            f"!= expected {expected_recent} from release index"
        )


def _validate_residual_census(root, version, facts, errors):
    census = facts.get("residual_census")
    if census is None:
        return

    total_parts = (
        census["finding_covered"]
        + census["normalized_silent"]
        + census["unsupported_boundary"]
        + census["parser_error"]
        + census["unclassified"]
    )
    if total_parts != census["total"]:
        errors.append(
            f"residual census arithmetic error: "
            f"{census['finding_covered']}+{census['normalized_silent']}+"
            f"{census['unsupported_boundary']}+{census['parser_error']}+"
            f"{census['unclassified']}={total_parts} "
            f"!= total={census['total']}"
        )

    census_tuple = (
        f"{census['total']}/{census['finding_covered']}"
        f"/{census['normalized_silent']}/{census['unsupported_boundary']}"
        f"/{census['parser_error']}/{census['unclassified']}"
    )

    census_fields = [
        ("total", str(census["total"])),
        ("finding_covered", str(census["finding_covered"])),
        ("normalized_silent", str(census["normalized_silent"])),
        ("unsupported_boundary", str(census["unsupported_boundary"])),
        ("parser_error", str(census["parser_error"])),
        ("unclassified", str(census["unclassified"])),
    ]

    en_notes = _read_file(root, f"docs/releases/release-notes-{version}.md")
    zh_notes = _read_file(
        root, f"docs/releases/release-notes-{version}.zh-CN.md"
    )

    for label, content in [
        (f"docs/releases/release-notes-{version}.md", en_notes),
        (f"docs/releases/release-notes-{version}.zh-CN.md", zh_notes),
    ]:
        if census_tuple in content:
            continue

        missing = []
        for key, value in census_fields:
            if not re.search(rf"{key}.*{value}", content):
                missing.append(key)

        if missing:
            errors.append(
                f"{label} missing census values for: "
                f"{', '.join(missing)}"
            )

    for rel in _surface_files(version):
        content = _read_file(root, rel)
        if not content:
            continue
        for stale in STALE_CENSUS_CLAIMS:
            if stale in content:
                errors.append(
                    f'{rel} contains stale census claim "{stale}"'
                )


def _validate_sql_corpus(root, version, facts, errors):
    corpus = facts.get("sql_corpus")
    if corpus is None:
        return

    en_notes = _read_file(root, f"docs/releases/release-notes-{version}.md")
    zh_notes = _read_file(
        root, f"docs/releases/release-notes-{version}.zh-CN.md"
    )
    metric_labels = {
        f"docs/releases/release-notes-{version}.md": corpus.get("metric_label"),
        f"docs/releases/release-notes-{version}.zh-CN.md": corpus.get(
            "metric_label_zh", corpus.get("metric_label")
        ),
    }

    covered_str = (
        f"{corpus['supported_rule_dialect_targets']}"
        f"/{corpus['covered_rule_dialect_targets']}"
    )

    for label, content in [
        (f"docs/releases/release-notes-{version}.md", en_notes),
        (f"docs/releases/release-notes-{version}.zh-CN.md", zh_notes),
    ]:
        metric_label = metric_labels.get(label)
        if metric_label and metric_label.lower() not in content.lower():
            errors.append(
                f"{label} missing SQL corpus metric label "
                f"{metric_label}"
            )

        coverage_ok = (
            covered_str in content
            or re.search(
                rf"supported_rule_dialect_targets"
                rf".*{corpus['supported_rule_dialect_targets']}",
                content,
            )
        )
        if not coverage_ok:
            errors.append(
                f"{label} missing SQL corpus coverage {covered_str}"
            )

        percent_ok = (
            corpus["coverage_percent"] in content
            or "100%" in content
            or re.search(
                rf"coverage_percent.*{re.escape(corpus['coverage_percent'])}",
                content,
            )
        )
        if not percent_ok:
            errors.append(
                f"{label} missing SQL corpus coverage percent "
                f"{corpus['coverage_percent']}"
            )

        yaml_ok = (
            str(corpus["expected_yaml_files_total"]) in content
            or re.search(
                rf"expected_yaml_files_total"
                rf".*{corpus['expected_yaml_files_total']}",
                content,
            )
        )
        if not yaml_ok:
            errors.append(
                f"{label} missing SQL corpus YAML file count "
                f"{corpus['expected_yaml_files_total']}"
            )


def _validate_pg_alter_table_rule_count(root, version, facts, errors):
    count = facts.get("pg_alter_table_rule_count")
    if count is None:
        return

    count_str = str(count)
    files_to_check = [
        f"docs/releases/release-notes-{version}.md",
        f"docs/releases/release-notes-{version}.zh-CN.md",
    ]

    for rel in files_to_check:
        content = _read_file(root, rel)
        if not content:
            continue

        found = False
        # Try same-line match first
        for line in content.split("\n"):
            if count_str in line and "alter table" in line.lower():
                found = True
                break

        # Fallback: wider window within ALTER TABLE section
        if not found:
            lower = content.lower()
            idx = lower.find("alter table")
            while idx != -1:
                window = content[max(0, idx - 50):idx + 1000]
                if count_str in window:
                    found = True
                    break
                idx = lower.find("alter table", idx + 1)

        if not found:
            errors.append(
                f"{rel} missing PG ALTER TABLE rule count {count_str} "
                f"with ALTER TABLE context"
            )


def _validate_required_rule_ids(root, version, facts, errors):
    rule_ids = facts.get("required_rule_ids")
    if rule_ids is None:
        return

    files_to_check = [
        f"docs/releases/release-notes-{version}.md",
        f"docs/releases/release-notes-{version}.zh-CN.md",
        "docs/reference/rules.md",
        "docs/reference/rules.zh-CN.md",
        "docs/reference/audit-capability-matrix.md",
        "docs/reference/audit-capability-matrix.zh-CN.md",
    ]

    for rel in files_to_check:
        content = _read_file(root, rel)
        if not content:
            continue
        for rule_id in rule_ids:
            if rule_id not in content:
                errors.append(f"{rel} missing required rule ID {rule_id}")


def _validate_no_overclaim(root, version, errors):
    for template in _SCOPED_SCAN_TEMPLATES:
        content, rel = _scoped_scan_content(root, version, template)
        if not content:
            continue

        for line in content.split("\n"):
            for pattern in OVERCLAIM_PATTERNS:
                if pattern.lower() in line.lower():
                    if not any(m in line for m in OVERCLAIM_NEGATIVE_MARKERS):
                        errors.append(
                            f'{rel} contains overclaim "{pattern}" '
                            f"without negative marker"
                        )


def _validate_no_leak(root, version, errors):
    for template in _SCOPED_SCAN_TEMPLATES:
        content, rel = _scoped_scan_content(root, version, template)
        if not content:
            continue

        for line in content.split("\n"):
            for term in FORBIDDEN_PAYLOAD_TERMS:
                if term in line:
                    if not any(m in line for m in NO_LEAK_MARKERS):
                        errors.append(
                            f'{rel} contains forbidden term "{term}" '
                            f"outside no-leak context"
                        )


def _validate_ddl_coverage_catalog(root, version, facts, errors):
    catalog_facts = facts.get("ddl_coverage_catalog")
    if catalog_facts is None:
        return

    import json as _json

    catalog_path = root / "docs/reference/ddl-coverage-catalog.json"
    if not catalog_path.exists():
        errors.append(
            "docs/reference/ddl-coverage-catalog.json does not exist"
        )
        return

    try:
        catalog = _json.loads(catalog_path.read_text(encoding="utf-8"))
    except _json.JSONDecodeError as exc:
        errors.append(
            f"docs/reference/ddl-coverage-catalog.json is not valid JSON: {exc}"
        )
        return

    entries = catalog.get("entries", [])
    if len(entries) != catalog_facts["total_entries"]:
        errors.append(
            f"catalog has {len(entries)} entries, "
            f"expected {catalog_facts['total_entries']}"
        )

    summary = catalog.get("summary", {})
    for dialect, expected_key in [
        ("mysql", "mysql_entries"),
        ("tidb", "tidb_entries"),
        ("postgresql", "postgresql_entries"),
    ]:
        dialect_summary = summary.get(dialect, {})
        count = dialect_summary.get("total", 0)
        if count != catalog_facts[expected_key]:
            errors.append(
                f"catalog summary {dialect} total is {count}, "
                f"expected {catalog_facts[expected_key]}"
            )

    puc_count = sum(
        1 for e in entries
        if e.get("guidance_code") == "parser_upgrade_candidate"
    )
    if puc_count != catalog_facts["parser_upgrade_candidate_count"]:
        errors.append(
            f"catalog has {puc_count} parser_upgrade_candidate entries, "
            f"expected {catalog_facts['parser_upgrade_candidate_count']}"
        )

    en_doc = root / "docs/reference/ddl-coverage.md"
    zh_doc = root / "docs/reference/ddl-coverage.zh-CN.md"
    if not en_doc.exists():
        errors.append("docs/reference/ddl-coverage.md does not exist")
    if not zh_doc.exists():
        errors.append("docs/reference/ddl-coverage.zh-CN.md does not exist")


def _validate_rule_catalog(root, version, facts, errors):
    """Validate rule catalog facts in release notes EN/ZH."""
    rule_catalog = facts.get("rule_catalog")
    if rule_catalog is None:
        return

    total = rule_catalog["total_rules"]
    total_str = str(total)

    en_notes = _read_file(root, f"docs/releases/release-notes-{version}.md")
    zh_notes = _read_file(
        root, f"docs/releases/release-notes-{version}.zh-CN.md"
    )

    for label, content in [
        (f"docs/releases/release-notes-{version}.md", en_notes),
        (f"docs/releases/release-notes-{version}.zh-CN.md", zh_notes),
    ]:
        if total_str not in content:
            errors.append(
                f"{label} missing rule catalog total {total_str}"
            )

        # Verify no severity field claim
        if "severity" in content.lower():
            # Check if it's used in a negative context (no severity field)
            for line in content.split("\n"):
                if "severity" in line.lower():
                    negative_markers = [
                        "no severity", "not a severity", "not severity",
                        "不存在", "非 severity", "不是 severity",
                        "而非", "不引入", "不将",
                        "rename", "no `severity`",
                    ]
                    if not any(m in line.lower() for m in negative_markers):
                        errors.append(
                            f'{label} contains "severity" outside '
                            f"negative context"
                        )

    # Verify level distribution appears in EN notes
    level_facts = {
        "blocker": rule_catalog["level_blocker"],
        "warning": rule_catalog["level_warning"],
        "notice": rule_catalog["level_notice"],
    }
    for level_name, count in level_facts.items():
        count_str = str(count)
        if count_str not in en_notes:
            errors.append(
                f"docs/releases/release-notes-{version}.md "
                f"missing {level_name} count {count_str}"
            )


def _validate_pg_alter_table_config_entries(root, version, facts, errors):
    count = facts.get("pg_alter_table_config_entries")
    if count is None:
        return

    count_str = str(count)

    config_path = root / "configs/deltascope.example.yaml"
    if config_path.exists():
        content = config_path.read_text(encoding="utf-8")
        actual = sum(
            1 for line in content.split("\n")
            if re.match(r"^  ddl\.pg\.alter\.", line)
        )
        if actual != count:
            errors.append(
                f"configs/deltascope.example.yaml has {actual} "
                f"ddl.pg.alter.* entries, expected {count}"
            )

    for rel in [
        f"docs/releases/release-notes-{version}.md",
        f"docs/releases/release-notes-{version}.zh-CN.md",
    ]:
        content = _read_file(root, rel)
        if not content:
            continue
        found = False
        for line in content.split("\n"):
            if count_str in line and "alter table" in line.lower():
                found = True
                break
        if not found:
            errors.append(
                f"{rel} missing PG ALTER TABLE config entries count "
                f"{count_str} with ALTER TABLE context"
            )


def validate_all(root, version):
    """Run all release surface consistency gates."""
    root = Path(root)
    facts = RELEASE_FACTS.get(version)
    if facts is None:
        return

    errors = []

    _validate_release_sequence(root, version, errors)
    _validate_residual_census(root, version, facts, errors)
    _validate_sql_corpus(root, version, facts, errors)
    _validate_pg_alter_table_rule_count(root, version, facts, errors)
    _validate_pg_alter_table_config_entries(root, version, facts, errors)
    _validate_required_rule_ids(root, version, facts, errors)
    _validate_no_overclaim(root, version, errors)
    _validate_no_leak(root, version, errors)
    _validate_ddl_coverage_catalog(root, version, facts, errors)
    _validate_rule_catalog(root, version, facts, errors)

    if errors:
        raise ReleaseConsistencyError("\n".join(errors))


def _format_sql_corpus_fact(corpus):
    label = corpus.get("metric_label", "sql corpus")
    return (
        f"release-consistency: {label} "
        f"{corpus['supported_rule_dialect_targets']}"
        f"/{corpus['covered_rule_dialect_targets']}, "
        f"{corpus['coverage_percent']}%, "
        f"{corpus['expected_yaml_files_total']} YAML"
    )


def main():
    version = os.environ.get("VERSION", "").strip()
    if not re.match(r"^v\d+\.\d+\.\d+$", version):
        print(
            "[release-consistency][FAIL] "
            "VERSION is required and must look like vX.Y.Z"
        )
        sys.exit(1)

    root = Path(__file__).resolve().parent.parent

    try:
        validate_all(root, version)
    except ReleaseConsistencyError as e:
        for line in str(e).split("\n"):
            print(f"[release-consistency][FAIL] {line}")
        sys.exit(1)

    facts = RELEASE_FACTS.get(version, {})

    print(f"release-consistency: VERSION={version}")

    readme = _read_file(root, "docs/releases/README.md")
    versions = _extract_release_versions(readme)
    if len(versions) >= 4:
        recent = ", ".join(versions[1:4])
        print(f"release-consistency: recent releases {recent}")

    census = facts.get("residual_census")
    if census:
        ct = (
            f"{census['total']}/{census['finding_covered']}"
            f"/{census['normalized_silent']}/{census['unsupported_boundary']}"
            f"/{census['parser_error']}/{census['unclassified']}"
        )
        print(f"release-consistency: residual census {ct}")

    corpus = facts.get("sql_corpus")
    if corpus:
        print(_format_sql_corpus_fact(corpus))

    if "pg_alter_table_rule_count" in facts:
        print(
            f"release-consistency: pg alter table rule count "
            f"{facts['pg_alter_table_rule_count']}"
        )

    if "pg_alter_table_config_entries" in facts:
        print(
            f"release-consistency: pg alter table config entries "
            f"{facts['pg_alter_table_config_entries']}"
        )

    rule_catalog = facts.get("rule_catalog")
    if rule_catalog:
        print(
            f"release-consistency: rule catalog "
            f"{rule_catalog['total_rules']} rules "
            f"(blocker {rule_catalog['level_blocker']}, "
            f"warning {rule_catalog['level_warning']}, "
            f"notice {rule_catalog['level_notice']}), "
            f"dialects (common {rule_catalog['dialect_common']}, "
            f"postgresql {rule_catalog['dialect_postgresql']}, "
            f"mysql {rule_catalog['dialect_mysql']}, "
            f"tidb {rule_catalog['dialect_tidb']}), "
            f"kinds (ddl {rule_catalog['kind_ddl']}, "
            f"dml {rule_catalog['kind_dml']})"
        )

    catalog = facts.get("ddl_coverage_catalog")
    if catalog:
        print(
            f"release-consistency: ddl coverage catalog "
            f"{catalog['total_entries']} entries "
            f"(mysql {catalog['mysql_entries']}, "
            f"tidb {catalog['tidb_entries']}, "
            f"postgresql {catalog['postgresql_entries']}), "
            f"{catalog['parser_upgrade_candidate_count']} "
            f"parser_upgrade_candidate"
        )

    print("release-consistency: PASS")


if __name__ == "__main__":
    main()
