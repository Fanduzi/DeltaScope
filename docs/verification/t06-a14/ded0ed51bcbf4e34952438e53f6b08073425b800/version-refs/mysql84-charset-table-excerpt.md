# Character Set-Related Table Definitions

Source: https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/ (see VERSION-REFS.md)
Retrieved: 2026-10-17. Full HTML page kept outside the repo; this is a text-only excerpt.

---

12.3.4 Table Character Set and Collation 

 Skip to Main Content MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Previous Database Character Set and Collation Home MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Up Specifying Character Sets and Collations Next Column Character Set and Collation 12.3.4 Table Character Set and Collation 
 Every table has a table character set and a table collation. The
 CREATE TABLE and
 ALTER TABLE statements have
 optional clauses for specifying the table character set and
 collation:
 CREATE TABLE tbl_name ( column_list )
 [[DEFAULT] CHARACTER SET charset_name ]
 [COLLATE collation_name ]]

ALTER TABLE tbl_name 
 [[DEFAULT] CHARACTER SET charset_name ]
 [COLLATE collation_name ]

 Example:
 CREATE TABLE t1 ( ... )
CHARACTER SET latin1 COLLATE latin1_danish_ci;

 MySQL chooses the table character set and collation in the
 following manner:

 If both CHARACTER SET
 charset_name and
 COLLATE
 collation_name are
 specified, character set
 charset_name and collation
 collation_name are used.

 If CHARACTER SET
 charset_name is
 specified without COLLATE , character set
 charset_name and its default
 collation are used. To see the default collation for each
 character set, use the SHOW CHARACTER
 SET statement or query the
 INFORMATION_SCHEMA 
 CHARACTER_SETS table.

 If COLLATE
 collation_name is
 specified without CHARACTER SET , the
 character set associated with
 collation_name and collation
 collation_name are used.

 Otherwise (neither CHARACTER SET nor
 COLLATE is specified), the database
 character set and collation are used.

 The table character set and collation are used as default values
 for column definitions if the column character set and collation
 are not specified in individual column definitions. The table
 character set and collation are MySQL extensions; there are no
 such things in standard SQL.
 Previous Database Character Set and Collation Home MySQL 8.4 Reference Manual Including MySQL NDB Cluster 8.4 Up Specifying Character Sets and Collations Next Column Character Set and Collation 
 MySQL Documentation Library
