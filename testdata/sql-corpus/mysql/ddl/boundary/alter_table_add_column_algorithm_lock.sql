ALTER TABLE order_events
  ADD COLUMN source_system VARCHAR(32) NOT NULL DEFAULT 'api' COMMENT 'ingest source',
  ALGORITHM=INPLACE,
  LOCK=NONE;
