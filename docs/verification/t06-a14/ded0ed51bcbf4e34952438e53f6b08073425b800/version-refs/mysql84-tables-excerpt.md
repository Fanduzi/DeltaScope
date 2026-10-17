# 28.3.38 The INFORMATION_SCHEMA TABLES Table

Source: https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/ (see VERSION-REFS.md)
Retrieved: 2026-10-17. Full HTML page kept outside the repo; this is a text-only excerpt.

---

28.3.38 The INFORMATION_SCHEMA TABLES Table 

 Skip to Main Content MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Previous The INFORMATION_SCHEMA ST_UNITS_OF_MEASURE Table Home MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Up INFORMATION_SCHEMA General Tables Next The INFORMATION_SCHEMA TABLES_EXTENSIONS Table 28.3.38 The INFORMATION_SCHEMA TABLES Table 
 The TABLES table provides information
 about tables in databases.

 Columns in TABLES that represent
 table statistics hold cached values. The
 information_schema_stats_expiry 
 system variable defines the period of time before cached table
 statistics expire. The default is 86400 seconds (24 hours). If
 there are no cached statistics or statistics have expired,
 statistics are retrieved from storage engines when querying table
 statistics columns. To update cached values at any time for a
 given table, use ANALYZE TABLE . To
 always retrieve the latest statistics directly from storage
 engines, set
 information_schema_stats_expiry 
 to 0 . For more information, see
 Section 10.2.3, “Optimizing INFORMATION_SCHEMA Queries” .
 Note 
 If the innodb_read_only system
 variable is enabled, ANALYZE
 TABLE may fail because it cannot update statistics
 tables in the data dictionary, which use
 InnoDB . For ANALYZE
 TABLE operations that update the key distribution,
 failure may occur even if the operation updates the table itself
 (for example, if it is a MyISAM table). To
 obtain the updated distribution statistics, set
 information_schema_stats_expiry=0 .

 The TABLES table has these columns:

 TABLE_CATALOG 

 The name of the catalog to which the table belongs. This value
 is always def .

 TABLE_SCHEMA 

 The name of the schema (database) to which the table belongs.

 TABLE_NAME 

 The name of the table.

 TABLE_TYPE 

 BASE TABLE for a table,
 VIEW for a view, or SYSTEM
 VIEW for an INFORMATION_SCHEMA 
 table.

 The TABLES table does not list
 TEMPORARY tables.

 ENGINE 

 The storage engine for the table. See
 Chapter 17, The InnoDB Storage Engine , and
 Chapter 18, Alternative Storage Engines .

 For partitioned tables, ENGINE shows the
 name of the storage engine used by all partitions.

 VERSION 

 This column is unused. With the removal of
 .frm files in MySQL 8.0, this column now
 reports a hardcoded value of 10 , which is
 the last .frm file version used in MySQL
 5.7.

 ROW_FORMAT 

 The row-storage format ( Fixed ,
 Dynamic , Compressed ,
 Redundant , Compact ). For
 MyISAM tables, Dynamic 
 corresponds to what myisamchk -dvv reports
 as Packed .

 TABLE_ROWS 

 The number of rows. Some storage engines, such as
 MyISAM , store the exact count. For other
 storage engines, such as InnoDB , this value
 is an approximation, and may vary from the actual value by as
 much as 40% to 50%. In such cases, use SELECT
 COUNT(*) to obtain an accurate count.

 TABLE_ROWS is NULL for
 INFORMATION_SCHEMA tables.

 For InnoDB tables, the row count
 is only a rough estimate used in SQL optimization. (This is
 also true if the InnoDB table is
 partitioned.)

 AVG_ROW_LENGTH 

 The average row length.

 DATA_LENGTH 

 For MyISAM , DATA_LENGTH 
 is the length of the data file, in bytes.

 For InnoDB , DATA_LENGTH 
 is the approximate amount of space allocated for the clustered
 index, in bytes. Specifically, it is the clustered index size,
 in pages, multiplied by the InnoDB page
 size.

 Refer to the notes at the end of this section for information
 regarding other storage engines.

 MAX_DATA_LENGTH 

 For MyISAM ,
 MAX_DATA_LENGTH is maximum length of the
 data file. This is the total number of bytes of data that can
 be stored in the table, given the data pointer size used.

 Unused for InnoDB .

 Refer to the notes at the end of this section for information
 regarding other storage engines.

 INDEX_LENGTH 

 For MyISAM , INDEX_LENGTH 
 is the length of the index file, in bytes.

 For InnoDB , INDEX_LENGTH 
 is the approximate amount of space allocated for non-clustered
 indexes, in bytes. Specifically, it is the sum of
 non-clustered index sizes, in pages, multiplied by the
 InnoDB page size.

 Refer to the notes at the end of this section for information
 regarding other storage engines.

 DATA_FREE 

 The number of allocated but unused bytes.

 InnoDB tables report the free space of the
 tablespace to which the table belongs. For a table located in
 the shared tablespace, this is the free space of the shared
 tablespace. If you are using multiple tablespaces and the
 table has its own tablespace, the free space is for only that
 table. Free space means the number of bytes in completely free
 extents minus a safety margin. Even if free space displays as
 0, it may be possible to insert rows as long as new extents
 need not be allocated.

 For NDB Cluster, DATA_FREE shows the space
 allocated on disk for, but not used by, a Disk Data table or
 fragment on disk. (In-memory data resource usage is reported
 by the DATA_LENGTH column.)

 For partitioned tables, this value is only an estimate and may
 not be absolutely correct. A more accurate method of obtaining
 this information in such cases is to query the
 INFORMATION_SCHEMA 
 PARTITIONS table, as shown in
 this example:
 SELECT SUM(DATA_FREE)
 FROM INFORMATION_SCHEMA.PARTITIONS
 WHERE TABLE_SCHEMA = 'mydb'
 AND TABLE_NAME = 'mytable';

 For more information, see
 Section 28.3.21, “The INFORMATION_SCHEMA PARTITIONS Table” .

 AUTO_INCREMENT 

 The next AUTO_INCREMENT value.

 CREATE_TIME 

 When the table was created.

 UPDATE_TIME 

 When the table was last updated. For some storage engines,
 this value is NULL . Even with
 file-per-table mode
 with each InnoDB table in a separate
 .ibd file,
 change buffering 
 can delay the write to the data file, so the file modification
 time is different from the time of the last insert, update, or
 delete. For MyISAM , the data file timestamp
 is used; however, on Windows the timestamp is not updated by
 updates, so the value is inaccurate.

 UPDATE_TIME displays a timestamp value for
 the last UPDATE ,
 INSERT , or
 DELETE performed on
 InnoDB tables that are not partitioned. For
 MVCC, the timestamp value reflects the
 COMMIT time, which is
 considered the last update time. Timestamps are not persisted
 when the server is restarted or when the table is evicted from
 the InnoDB data dictionary cache.

 CHECK_TIME 

 When the table was last checked. Not all storage engines
 update this time, in which case, the value is always
 NULL .

 For partitioned InnoDB tables,
 CHECK_TIME is always
 NULL .

 TABLE_COLLATION 

 The table default collation. The output does not explicitly
 list the table default character set, but the collation name
 begins with the character set name.

 CHECKSUM 

 The live checksum value, if any.

 CREATE_OPTIONS 

 Extra options used with CREATE
 TABLE .

 CREATE_OPTIONS shows
 partitioned for a partitioned table.

 CREATE_OPTIONS shows the
 ENCRYPTION clause specified for tables
 created in file-per-table tablespaces. It shows the encryption
 clause for file-per-table tablespaces if the table is
 encrypted or if the specified encryption differs from the
 schema encryption. The encryption clause is not shown for
 tables created in general tablespaces. To identify encrypted
 file-per-table and general tablespaces, query the
 INNODB_TABLESPACES 
 ENCRYPTION column.

 When creating a table with
 strict mode disabled,
 the storage engine's default row format is used if the
 specified row format is not supported. The actual row format
 of the table is reported in the ROW_FORMAT 
 column. CREATE_OPTIONS shows the row format
 that was specified in the CREATE
 TABLE statement.

 When altering the storage engine of a table, table options
 that are not applicable to the new storage engine are retained
 in the table definition to enable reverting the table with its
 previously defined options to the original storage engine, if
 necessary. The CREATE_OPTIONS column may
 show retained options.

 TABLE_COMMENT 

 The comment used when creating the table (or information as to
 why MySQL could not access the table information).
 Notes 
 For NDB tables, the output of
 this statement shows appropriate values for the
 AVG_ROW_LENGTH and
 DATA_LENGTH columns, with the exception
 that BLOB columns are not taken
 into account.

 For NDB tables,
 DATA_LENGTH includes data stored in main
 memory only; the MAX_DATA_LENGTH and
 DATA_FREE columns apply to Disk Data.

 For NDB Cluster Disk Data tables,
 MAX_DATA_LENGTH shows the space allocated
 for the disk part of a Disk Data table or fragment. (In-memory
 data resource usage is reported by the
 DATA_LENGTH column.)

 For MEMORY tables, the
 DATA_LENGTH ,
 MAX_DATA_LENGTH , and
 INDEX_LENGTH values approximate the actual
 amount of allocated memory. The allocation algorithm reserves
 memory in large amounts to reduce the number of allocation
 operations.

 For views, most TABLES columns
 are 0 or NULL except that
 TABLE_NAME indicates the view name,
 CREATE_TIME indicates the creation time,
 and TABLE_COMMENT says
 VIEW .

 Table information is also available from the
 SHOW TABLE STATUS and
 SHOW TABLES statements. See
 Section 15.7.7.38, “SHOW TABLE STATUS Statement” , and
 Section 15.7.7.39, “SHOW TABLES Statement” . The following statements are
 equivalent:
 SELECT
 TABLE_NAME, ENGINE, VERSION, ROW_FORMAT, TABLE_ROWS, AVG_ROW_LENGTH,
 DATA_LENGTH, MAX_DATA_LENGTH, INDEX_LENGTH, DATA_FREE, AUTO_INCREMENT,
 CREATE_TIME, UPDATE_TIME, CHECK_TIME, TABLE_COLLATION, CHECKSUM,
 CREATE_OPTIONS, TABLE_COMMENT
 FROM INFORMATION_SCHEMA.TABLES
 WHERE table_schema = ' db_name '
 [AND table_name LIKE ' wild ']

SHOW TABLE STATUS
 FROM db_name 
 [LIKE ' wild ']

 The following statements are equivalent:
 SELECT
 TABLE_NAME, TABLE_TYPE
 FROM INFORMATION_SCHEMA.TABLES
 WHERE table_schema = ' db_name '
 [AND table_name LIKE ' wild ']

SHOW FULL TABLES
 FROM db_name 
 [LIKE ' wild ']
 Previous The INFORMATION_SCHEMA ST_UNITS_OF_MEASURE Table Home MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Up INFORMATION_SCHEMA General Tables Next The INFORMATION_SCHEMA TABLES_EXTENSIONS Table 
 MySQL Documentation Library
