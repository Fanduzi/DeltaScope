# T06-A17 EXPECTED-UPGRADES — same-root Type-text expectation changes

Every change below is a Type-text expectation on the five A17 target types
(DECIMAL / FLOAT / DOUBLE / ENUM / SET). No rule_id, level, count, coverage,
gap, SQL, profile, or denominator changed.

## Production expectations (2 sites)

| file | before | after |
|---|---|---|
| `internal/infrastructure/parser/tidb/extractor.go` `extractColumn` | `Type = strings.ToLower(col.Tp.String())` — substituted defaults for unspecified sentinels, elided explicit `(M,0)` on float/double, case-folded member literals | `Type = declaredColumnType(col.Tp)` — sentinel-faithful Restore for the five types, existing text for all others |
| `internal/infrastructure/metadata/mysql/provider.go` `parseColumnType`/`loadColumns` | `Type` = base keyword truncated at `(`; `unsigned` read inside member literals possible | `Type` = base + verbatim parameter span for the five bases; quote-aware attribute boundary |

## Test expectation upgrades (1 site)

| file:line | field | before | after | why |
|---|---|---|---|---|
| `internal/infrastructure/metadata/mysql/provider_test.go:400` (`TestParseColumnType`) | first return of `parseColumnType("decimal(10,2)")` | `decimal` | `decimal(10,2)` | same-root upgrade: the five target bases keep their parameter span |

## Behavior changes visible in finding text (not frozen expectations)

- `source_type`/`target_type` metadata in `ddl.alter.*_column.compatibility.require`
  findings now carries faithful text: `decimal(12,2)` (was `decimal` from a
  provider source), `enum('Ready','Closed')` (was `enum('ready','closed')`
  from a declared source). Rule applicability (family comparison via
  `baseType`) is unchanged.
- `Column.Type` values in `{table,column,type}` finding metadata (e.g.
  `ddl.column.not_null.require`) carry the same upgraded text for the five
  types; message shape and attribution are unchanged.

## Explicitly unchanged

- All other parseColumnType table rows (`varchar`, `bigint`, `int`, `text`,
  `tinyint`, `varchar(64) unsigned`) — non-target bases keep the bare-keyword
  contract.
- `extractor_t06a13_test.go` / `batch_state_t06a13_test.go` typeText
  expectations (`float`, `double`, `int(11)`, `blob binary`, `bit(1)`,
  `varchar(8)`) — bare declarations unaffected.
- T06.json / golden oracles — no frozen finding embeds five-type parameter
  text (checked: golden T06 columns are bare FLOAT/DOUBLE/INT/etc.).
