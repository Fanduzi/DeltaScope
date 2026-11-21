# PostgreSQL 17 text search configuration and dictionary changes

Status Accepted

## Decision

Enable `ddl.pg.text_search.change.target.valid` for exact qualified text search configurations and dictionaries. The rule checks RENAME destination occupancy, SET SCHEMA eligibility and OWNER TO role identity. Mapping changes require the verified built-in default parser for authored token names. Its token directory comes from PostgreSQL's built-in `ts_token_type` function. The auditor matches dictionary identity and mapping occupancy using catalog facts. ADD refuses occupied mappings. ALTER can add or replace a mapping. DROP IF EXISTS skips absent mappings after validating the token names.

For the verified built-in simple dictionary template, options retain source order and the final occurrence controls each name. A removed option has no value. The rule checks unsupported names and bounded ACCEPT Boolean forms. Stop-word files, other dictionary templates, custom parsers, unqualified name resolution and execution privileges require separate evidence. Findings expose the authored object and a reason code. Resolved catalog identities and option values stay internal.

## Verification

- [T45 text search controls](../../testdata/ddl-golden/T45.json)
- [Unknown dictionary option corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_text_search_dictionary_unknown_option.expected.yaml)
- [PostgreSQL 17.7 mapping and dictionary changes](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tsearchcmds.c)
- [PostgreSQL 17.7 simple dictionary](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/tsearch/dict_simple.c)

T45 preview passed 59 cases and 1,435 assertions. Twenty-six references bind the four PostgreSQL 17 text search cells.
