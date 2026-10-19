#!/bin/bash
source /tmp/ds-t07r1/native_gen.sh
for ak in mysql57 mysql80 mysql84 tidb85; do
  run_gen G_VALID    "$ak" t_r1g_valid    "CREATE TABLE t_r1g_valid (a INT, b INT AS (a + 1) STORED)"
  run_gen G_SELF     "$ak" t_r1g_self     "CREATE TABLE t_r1g_self (a INT, b INT AS (b + 1) STORED)"
  run_gen G_MISSING  "$ak" t_r1g_missing  "CREATE TABLE t_r1g_missing (a INT, b INT AS (missing_c + 1) STORED)"
  run_gen G_BASE_FWD "$ak" t_r1g_basefwd  "CREATE TABLE t_r1g_basefwd (b INT AS (a + 1) STORED, a INT)"
  run_gen G_GEN_FWD  "$ak" t_r1g_genfwd   "CREATE TABLE t_r1g_genfwd (a INT, b INT AS (c + 1) STORED, c INT AS (a + 1) STORED)"
  run_gen G_CYCLE    "$ak" t_r1g_cycle    "CREATE TABLE t_r1g_cycle (a INT, b INT AS (c + 1) STORED, c INT AS (b + 1) STORED)"
  run_gen G_LITERAL  "$ak" t_r1g_literal  "CREATE TABLE t_r1g_literal (a INT, b VARCHAR(32) AS ('missing_c') VIRTUAL)"
done
