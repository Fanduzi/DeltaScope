CREATE TABLE events (
  id bigint,
  created_at timestamptz NOT NULL
) PARTITION BY RANGE (created_at);
