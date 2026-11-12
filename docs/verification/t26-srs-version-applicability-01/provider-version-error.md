# Provider version-observation error paths — SRS family

## Taxonomy (source review)

| Failure | Point | Result |
|---|---|---|
| `select version()` errors / empty / unparseable | O2 `mysqlSRSCatalogSupport` | `supported=false, proven=false` → reads issue; downstream query errors propagate |
| Banner parses to non-MySQL product (e.g. TiDB marker) | O2 | `proven=false` → same: reads issue on a MySQL-catalog loader that is only invoked on the MySQL dialect path |
| Banner parses MySQL but unvalidated series (9.x) | O2 | `proven=false` → reads issue |
| Banner parses MySQL 5.7.x | O2 | `proven=true, supported=false` → `version_inapplicable`, zero reads |
| Banner parses MySQL ≥8 proven | O2 | `proven=true, supported=true` → reads issue; all errors propagate |
| Existence/conflict/deps query, scan, iteration, close errors | loader | propagate as errors — never converted to empty/negative facts |
| `select version()` at O1 (request level) | `metadata.go` | failure → `resolvedVersion=nil` → all SRS statements boundary; reads still attempted via O2 if the provider itself can query |

## Distinctness requirements (satisfied today)

1. Proven-inapplicable (5.7) → dedicated `version_inapplicable` unread reason — distinct from unknown.
2. Version-observation error (`!proven`) → not collapsed into inapplicable: reads still issue; errors propagate rather than minting a false applicability fact.
3. Read errors under any version arm propagate — they never become "absent"/"no conflict" facts.

## Residual asymmetry (contract item V10)

The two observation points degrade differently: O1-unparseable ⇒ boundary + attempted reads; O2-unparseable ⇒ attempted reads. There is no surfaced "version observation failed" reason distinct from the silent `!proven` fallthrough — a caller cannot tell "banner unparseable" from "banner said 9.x" without inspecting the statement-level marker plus provider errors. Phase 0 records this; no implementation authorized.
