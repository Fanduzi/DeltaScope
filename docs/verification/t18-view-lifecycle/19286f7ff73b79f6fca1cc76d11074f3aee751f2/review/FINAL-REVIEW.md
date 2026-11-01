# T18-R1 independent-review handoff

## Scope

This handoff is limited to the four findings reported by the independent source
review of product `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb`:

1. CREATE VIEW must consume the target schema's ordered absent premise.
2. CHECK OPTION must consume static structural updatability.
3. ALTER VIEW must apply fallback after inheriting an old MERGE algorithm.
4. Correlated subquery resolution must retain layered scope frames.

The result is a candidate for independent T18-R1 acceptance. It is not a merge,
release, CI-green, or T19 claim.

## Fixed identity

- Reviewed baseline: `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb`
- Product: `19286f7ff73b79f6fca1cc76d11074f3aee751f2`
- Evidence commit: recorded by the commit that adds this archive
- Remote target: `origin/workfree`, fast-forward only

## Red-to-green mapping

- F1: public Audit accepted `CREATE VIEW g.v ...` after `DROP DATABASE g`; the
  compatible internal replay shows no target refusal. On the fixed product both
  attempts on `g.v` are refused and no present view is published; untouched
  schemas remain unknown rather than guessed absent.
- F2: computed-only and LEFT JOIN CHECK OPTION definitions passed as complete.
  On the fixed product they produce `ddl.view.check_option.valid` blockers;
  direct, mixed-projection, and inner-join controls pass.
- F3: ALTER inheriting old MERGE did not warn or fall back. On the fixed
  product authored, inherited/effective, and final algorithms are separated;
  an aggregate definition warns and publishes `undefined` to subsequent views.
- F4: the innermost unqualified `id` was flattened into a false ambiguity.
  On the fixed product it binds at the nearest outer scope, while true
  same-scope ambiguity still rejects and deeper fallback remains bounded.

## Evidence map

- `red/`: baseline replay and actual semantic failures.
- `green/`: focused internal companion and public Audit runs at the product SHA.
- `native/`: MySQL 8.0.46 disposable-container observations and cleanup.
- `golden/`: final passing T16/T17/T18 artifacts and identity summary.
- `gates/`: final command/stdout/stderr/rc/identity receipts.
- `attempts/`: environment-injection diagnostics only.

## Verification totals

- Internal/public focused R1 suites: pass.
- `go test ./... -count=1`: pass.
- `go vet ./...`: pass.
- `env CGO_ENABLED=1 go test -race ./internal/... -count=1`: pass.
- `make pg-confidence-gates`: pass.
- SQL corpus, Query Access corpus, DDL inventory, DDL catalog, Golden
  validator, parser/unsupported contracts, HTTP/MCP T18, docs/examples,
  decision-record and committed-range L2 gates: pass.
- Golden: T16 60/60 and 832 assertions; T17 52/52 and 834 assertions; T18
  149/149 and 3,755 assertions. All required case IDs were executed exactly
  once and cleanup left no task containers.
- Lint: scoped and full-repository commands retain rc=1, but the diagnostic
  identities match the accepted 22/39 baseline exactly with no added or
  removed diagnostics.

## Boundaries retained

- No provider-resolver schema lookup was added for unproven namespaces; CREATE
  consumes only ordered schema-absent facts.
- CHECK OPTION uses structural updatability only; it does not execute SELECTs,
  evaluate expressions, or infer final DML authorization.
- Unknown old algorithm remains unknown rather than being guessed; CREATE and
  OR REPLACE retain their own creation defaults.
- Failed Golden attempts are environment diagnostics and not passing evidence.
- Historical evidence and foreign untracked paths remain unchanged.
