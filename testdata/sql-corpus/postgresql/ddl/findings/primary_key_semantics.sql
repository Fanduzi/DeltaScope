CREATE TABLE bad_pk_type (
  id integer PRIMARY KEY,
  name text
);

CREATE TABLE composite_pk (
  tenant_id bigint,
  user_id bigint,
  PRIMARY KEY (tenant_id, user_id)
);
