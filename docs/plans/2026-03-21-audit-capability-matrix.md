# DeltaScope Audit Capability Matrix

## Purpose

This matrix is the acceptance baseline for the `Audit Completion` milestone.

Status meanings:

- `covered`: implemented and aligned with the current product contract
- `enhanced`: implemented with stronger or cleaner behavior than the legacy baseline
- `gap`: still missing or not yet deep enough
- `deferred`: intentionally left out of this milestone

## DDL: Create Table

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
table name length                                   covered     ddl.table.name.max_length
table name pattern                                  covered     ddl.table.name.pattern.require
table name reserved-keyword forbid                  covered     ddl.table.name.keyword.forbid
table comment required                              covered     ddl.table.comment.require
table comment length                                covered     ddl.table.comment.max_length
table engine allowlist                              covered     ddl.table.engine.allowlist
table charset allowlist                             covered     ddl.table.charset.allowlist
table row_format allowlist                          covered     ddl.table.row_format.allowlist
table auto_increment init value                     covered     ddl.table.auto_increment.init_value.require
table must have at least one column                 covered     ddl.table.columns.min_count
table must have primary key                         covered     ddl.table.primary_key.require
primary key max column count                        covered     ddl.table.primary_key.columns.max_count
primary key must be bigint                          covered     ddl.table.primary_key.bigint.require
primary key must be unsigned                        covered     ddl.table.primary_key.unsigned.require
primary key must be auto_increment                  covered     ddl.table.primary_key.auto_increment.require
primary key must be not null                        covered     ddl.table.primary_key.not_null.require
audit timestamp columns required                    covered     ddl.table.audit_columns.require
forbid create table as select                       covered     ddl.table.create_as.forbid
forbid create table like                            covered     ddl.table.create_like.forbid
forbid foreign key                                  covered     ddl.table.foreign_key.forbid
forbid partition table                              covered     ddl.table.partition.forbid
column name length                                  covered     ddl.column.name.max_length
column name pattern                                 covered     ddl.column.name.pattern.require
column name reserved-keyword forbid                 covered     ddl.column.name.keyword.forbid
column comment required                             covered     ddl.column.comment.require
column default required                             covered     ddl.column.default.require
column not null required                            covered     ddl.column.not_null.require
varchar max length                                  covered     ddl.column.varchar.max_length
char max length guidance                            covered     ddl.column.char.max_length
float/double forbid                                 covered     ddl.column.float_double.forbid
blob/text type governance                           covered     ddl.column.blob_text.forbid
json type governance                                covered     ddl.column.json.forbid
bit type governance                                 covered     ddl.column.bit.forbid
timestamp type governance                           covered     ddl.column.timestamp.forbid
column charset allowlist                            covered     ddl.column.charset.allowlist
column collation allowlist                          covered     ddl.column.collation.allowlist
column charset/collation match                      covered     ddl.column.charset_collation.match.require
index max count                                     covered     ddl.index.total.max_count
index max columns                                   covered     ddl.index.columns.max_count
unique index prefix                                 covered     ddl.index.unique.prefix.require
secondary index prefix                              covered     ddl.index.secondary.prefix.require
fulltext index prefix                               covered     ddl.index.fulltext.prefix.require
duplicate index forbid                              covered     ddl.index.duplicate.forbid
redundant left-prefix index forbid                  covered     ddl.index.redundant_left_prefix.forbid
redundant unique-overlap index forbid               covered     ddl.index.redundant_unique_overlap.forbid
row-size / index-size checks with instance facts    gap         needs metadata-backed instance facts and honest sizing rules
create view governance                              gap         legacy baseline had explicit view switch; DeltaScope has no create-view rule yet
ddl disabled-table lists                            gap         legacy baseline had DB/table blocklists; DeltaScope has no object-scope denylist yet
```

## DDL: Alter Table

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
forbid drop column                                  covered     ddl.alter.drop_column.forbid
forbid drop index                                   covered     ddl.alter.drop_index.forbid
forbid drop primary key                             covered     ddl.alter.drop_primary_key.forbid
forbid rename table                                 covered     ddl.alter.rename_table.forbid
forbid rename column                                covered     ddl.alter.rename_column.forbid
forbid rename index                                 covered     ddl.alter.rename_index.forbid
forbid generic change column                        covered     ddl.alter.change_column.forbid
forbid generic modify column                        covered     ddl.alter.modify_column.forbid
target type-family allowlist                        covered     conservative alter target-family guards already shipped
explicit nullability/default/autoinc change checks  covered     shipped for change/modify column
alter-added index prefix checks                     covered     unique/secondary/fulltext add-index checks shipped
alter-added duplicate index checks                  enhanced    create-table duplicate logic reused in alter path
alter-added redundant index checks                  gap         no deeper redundant-index lifecycle checks for added indexes yet
source-to-target type compatibility                 gap         target-family only; no source-to-target compatibility engine yet
column existence checks                             gap         needs table snapshot + metadata-aware mode
index existence checks                              gap         needs table snapshot + metadata-aware mode
primary-key existence/state checks                  gap         needs table snapshot + metadata-aware mode
rename/change/modify against current schema state   gap         needs metadata-aware table snapshot
merge alter governance                              gap         legacy baseline had MySQL/TiDB merge-alter switches
table option compatibility vs current schema        gap         no metadata-backed alter option compatibility yet
```

## DDL: Drop / Truncate / Object Existence

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
drop table governance                               gap         legacy baseline had enable switch and row-limit risk handling
truncate table governance                           gap         legacy baseline had enable switch and row-limit risk handling
drop/truncate row-count risk                        gap         requires online metadata and row-count-aware checks
drop/truncate adaptive-hash warning                 gap         requires version + innodb_adaptive_hash_index instance facts
table exists / not exists checks                    gap         requires metadata provider
show-create based current schema recovery           gap         requires metadata provider and snapshot model
```

## DML

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
require where for update/delete                      covered     dml.where.require
forbid subquery                                      covered     dml.subquery.forbid
forbid order by                                      covered     dml.order_by.forbid
forbid limit                                         covered     dml.limit.forbid
require join on                                      covered     dml.join.on.require
forbid replace                                       covered     dml.replace.forbid
forbid insert into select                            covered     dml.insert.select.forbid
forbid on duplicate                                  covered     dml.insert.on_duplicate.forbid
insert rows max count                                covered     dml.insert.rows.max_count
affected-row threshold                               gap         needs explain/metadata/runtime estimation path
dml disabled-table lists                             gap         legacy baseline had DB/table blocklists; DeltaScope has no object-scope denylist yet
```

## Instance Facts And Metadata

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
instance version fact                                gap         needed for alter sizing, compatibility, and engine-specific behavior
instance default charset fact                        gap         needed for parser/context-sensitive checks
instance innodb_large_prefix fact                    gap         needed for honest index-length checks
instance innodb_default_row_format fact              gap         needed for row-format-aware checks
instance innodb_adaptive_hash_index fact             gap         needed for drop/truncate risk checks
target table snapshot                                gap         needed for existence and compatibility rules
column snapshot                                      gap         needed for alter compatibility and object existence
index snapshot                                       gap         needed for alter lifecycle rules
primary-key snapshot                                 gap         needed for primary-key state checks
optional metadata-aware audit mode                   gap         this is the central enabling capability for the milestone
```

## Public Product Surface

```text
Capability                                          Status      Notes
-----------------------------------------------------------------------------------------------
stable Go package API                                covered     pkg/deltascope
CLI for audit/config/version                         enhanced    now has --version and version-logo split
HTTP API service                                     covered     health/version/audit endpoints shipped
English README matured for release                   gap         planned in Task 7
Chinese README                                       gap         planned in Task 7
CHANGELOG                                            gap         planned in Task 7
SECURITY                                             gap         planned in Task 7
formal capability matrix                             covered     this document is the baseline artifact
```

## Task Targets

The current blocking gaps for `Audit Completion` are:

- metadata-aware audit mode
- instance facts loading
- table snapshot / object existence facts
- deeper alter source-to-target compatibility
- drop/truncate and object-lifecycle online checks
- disabled-table governance
- public release-surface docs
