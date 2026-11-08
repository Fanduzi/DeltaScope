// Package mysqlmeta reads TiDB sharding catalog facts.
package mysqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// loadTiDBShardingInfo reads information_schema.TABLES.TIDB_ROW_ID_SHARDING_INFO —
// the authoritative catalog projection of the table's sharding layout that
// TiDB v8.5 exposes:
//
//	NOT_SHARDED                                   — no row-id sharding stored
//	NOT_SHARDED(PK_IS_HANDLE)                     — primary key is the row id
//	PK_AUTO_RANDOM_BITS=n[, RANGE BITS=m]         — AUTO_RANDOM primary key
//	SHARD_BITS=n                                  — SHARD_ROW_ID_BITS stored
//
// A nil cell (views, memory/system tables) or an unrecognized shape leaves
// the facts unobserved rather than guessed; the read itself is only issued
// for TiDB base tables.
func (p *Provider) loadTiDBShardingInfo(ctx context.Context, snapshot *spec.TableSnapshot) error {
	var sharding sql.NullString
	if err := p.db.QueryRowContext(ctx, `
		select tidb_row_id_sharding_info
		from information_schema.tables
		where table_schema = ? and table_name = ?
	`, snapshot.Schema, snapshot.Table.Name).Scan(&sharding); err != nil {
		return fmt.Errorf("query table sharding info: %w", err)
	}
	info := strings.TrimSpace(sharding.String)
	switch {
	case !sharding.Valid || info == "":
		// Unsuitable catalog target — the layout facts stay unobserved.
	case strings.HasPrefix(info, "PK_AUTO_RANDOM_BITS="):
		shard, rangeBits, ok := parseShardingAutoRandom(info)
		if !ok {
			return fmt.Errorf("unrecognized sharding info %q", info)
		}
		markAutoRandomColumn(snapshot, shard, rangeBits)
	case strings.HasPrefix(info, "SHARD_BITS="):
		bits, err := strconv.ParseUint(strings.TrimPrefix(info, "SHARD_BITS="), 10, 64)
		if err != nil {
			return fmt.Errorf("unrecognized sharding info %q", info)
		}
		snapshot.Options["shard_row_id_bits"] = strconv.FormatUint(bits, 10)
	case info == "NOT_SHARDED" || info == "NOT_SHARDED(PK_IS_HANDLE)":
		// ShardRowIDBits == 0 is the proven stored value; PK_IS_HANDLE
		// corroborates the handle answer the primary-key probe owns.
		snapshot.Options["shard_row_id_bits"] = "0"
	default:
		return fmt.Errorf("unrecognized sharding info %q", info)
	}
	return nil
}

// parseShardingAutoRandom splits "PK_AUTO_RANDOM_BITS=n[, RANGE BITS=m]"
// into the stored shard/range values. RANGE BITS is omitted upstream for
// the default 64.
func parseShardingAutoRandom(info string) (shardBits, rangeBits int64, ok bool) {
	body := strings.TrimPrefix(info, "PK_AUTO_RANDOM_BITS=")
	parts := strings.SplitN(body, ",", 2)
	shard, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || shard <= 0 {
		return 0, 0, false
	}
	rangeBits = spec.AutoRandomDefaultRangeBits
	if len(parts) == 2 {
		rest := strings.TrimSpace(parts[1])
		if !strings.HasPrefix(rest, "RANGE BITS=") {
			return 0, 0, false
		}
		rangeBits, err = strconv.ParseInt(strings.TrimPrefix(rest, "RANGE BITS="), 10, 64)
		if err != nil || rangeBits <= 0 {
			return 0, 0, false
		}
	}
	return shard, rangeBits, true
}

// markAutoRandomColumn flags the single-member primary key column — the
// only column upstream can carry AUTO_RANDOM (PKIsHandle) — with the
// stored shard/range values as the authored declaration equivalent.
func markAutoRandomColumn(snapshot *spec.TableSnapshot, shardBits, rangeBits int64) {
	if snapshot.PrimaryKey == nil || len(snapshot.PrimaryKey.Columns) != 1 {
		return
	}
	for i := range snapshot.Columns {
		if strings.EqualFold(snapshot.Columns[i].Name, snapshot.PrimaryKey.Columns[0]) {
			shard, rng := shardBits, rangeBits
			snapshot.Columns[i].AutoRandom = true
			snapshot.Columns[i].AutoRandomShardBits = &shard
			snapshot.Columns[i].AutoRandomRangeBits = &rng
			return
		}
	}
}
