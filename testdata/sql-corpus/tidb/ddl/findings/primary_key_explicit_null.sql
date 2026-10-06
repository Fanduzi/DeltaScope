CREATE TABLE pk_explicit_null_probe (
  id INT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB COMMENT='explicit NULL primary-key member conflict';
