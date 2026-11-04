SET SESSION tidb_placement_mode='STRICT';
SELECT VERSION(),TIDB_VERSION(),@@tidb_placement_mode;