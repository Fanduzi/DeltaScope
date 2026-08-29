CREATE TABLE regional_accounts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'account identifier',
  region_code VARCHAR(16) NOT NULL DEFAULT 'us' COMMENT 'account region',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',
  PRIMARY KEY (id, region_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='accounts by region'
PARTITION BY LIST COLUMNS (region_code) (
  PARTITION p_us VALUES IN ('us'),
  PARTITION p_eu VALUES IN ('de', 'fr'),
  PARTITION p_apac VALUES IN ('cn', 'jp')
);
