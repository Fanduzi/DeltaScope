# T17-R1-tail final gate index

Final product: `c6eebc09b64a4c9ffc8d1dc73689f505da06d61c`.
Implementation commit: `84762d9d6c9085dbe91307516e14a984efa76375`.
Task baseline: `9da9f8e02221b1e51bf275cf012ffc2ac88b594a`.

Every gate has actual command/stdout/stderr/rc files, plus execution
identity receipts. This is a review candidate; no independent acceptance
or CI/release result is asserted.

| Gate | Outcome |
|---|---|
| `go test ./... -count=1` | rc0 |
| `go vet ./...` | rc0 |
| `env CGO_ENABLED=1 go test -race ./internal/...` | rc0 |
| `make pg-unit-test-gates` | rc0 |
| `make sql-corpus-gates` | rc0 |
| `make ddl-inventory-gate` | rc0 |
| `make ddl-coverage-catalog-test` | rc0 |
| `make ddl-golden-validator-test` | rc0 |
| `make parser-error-unsupported-contract-test` | rc0 |
| `make unsupported-diagnostics-evidence-test` | rc0 |
| `make docs-example-gates` | rc0 |
| `make decision-record-gate` | rc0 |
| committed-range L2, `9da9f8e0..c6eebc09` | rc0 |
| `make ddl-golden TASK=T17 ARTIFACT_DIR=/tmp/ddl-golden` | rc0, 52/52, 834 CLI/database assertions |
| original eight-tree scoped lint | rc1, 22 inherited findings |
| `golangci-lint run ./...` | rc1, 39 inherited findings |
| lead-authored lint identity comparison | rc0, no added or removed findings |

## Proof layers

- `red/`: real unfixed product `83125653`, permanent tracer tests through
  parse/extract/enrich. Both CREATE and ALTER fail from 0900 versus
  general_ci. Public Audit controls pass and are explicitly not internal
  collation assertions.
- `green/`: focused development receipts, including intermediate steps.
- `gates/`: final-product broad checks and the canonical Golden output;
  the Make target runs the new projection controls separately from the
  unchanged 52-case CLI/database artifact.
- `gates-pre-lint/`: initial-product receipts, including the actual new
  gocritic stringXbytes finding (23/40, not accepted as a new baseline).
- `lint-fix/`: narrow test-only correction and verification.
- `ddl-golden-auth-failed.*`: actual rc2 from worker-supplied redaction
  markers used as fixture passwords, with 1045 receipts and preserved
  failed artifact; this is not product red. The final canonical receipt
  uses the disposable Compose credential without changing the oracle.
- `native/`: frozen additional MySQL 8.0.46 business-only probe authored,
  executed, and checked by the lead. The original MySQL 5.7.44 general_ci
  evidence remains in the historical committed p17 log, not rerun.

The 8.0.46/8.4.10 projection controls provide an observed configurable
default; they do not infer one from those version numbers. TiDB 8.5.0 and
MySQL 5.7.44 use bounded validated identities. Unknown/future identities
without sufficient observations withhold only affected attributes.

## Regression scope

B1/B2/B3, the PostgreSQL shared error contract, and touched-unknown and
known-present table behavior remain unchanged. The full audit package and
canonical PG tagged unit gate cover these existing controls.

The PG gate exercises parser, audit, auditmeta, CLI, HTTP, MCP, and SDK
shared types. The additional observations are MySQL-only empty strings
on PG; PG queries/error behavior do not change. Docker PG confidence
and historical T09-T16 Golden suites are not rerun solely for docs or
unchanged features; their historical evidence remains untouched.

## Baseline and safety

The diagnostic comparison reads the current R1 baseline from committed
`9da9f8e0` evidence, keyed by file and full message/linter multiplicities,
ignoring only shifted line/column coordinates. It does not equate matching
totals with matching identities. Scoped and full lint are not green claims.

Formatting checks cover changed Go files only. Separate receipts preserve
the T07 baseline; no retired L3 file headers are introduced. Module READMEs
are synchronized over the effective committed product range.

The original 52-case manifest is byte-identical to baseline. The user
explicitly permitted original Golden system-schema cases only in their
disposable fixture containers. No historical p17 script or additional
system-schema experiment is run. Preservation hashes cover the previous
R1 artifact before canonical-path reuse; cleanup is task-owned only.
