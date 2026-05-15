CREATE OPERATOR CLASS int4_ops_class DEFAULT FOR TYPE int4 USING btree FAMILY int4_ops_family AS OPERATOR 1 < (int4, int4)
