# T06-A17-R1 proof command

A bounded collector for the existing 24-column native catalog/provider proof.
It is not a product surface, a new type policy, or a replacement Golden runner.
The production extractor/provider and the 496-case/728-check contracts stay
unchanged.

## Files

| File | Responsibility |
|---|---|
| `main.go` | Records native session/catalog/provider observations, propagates query/scan/iteration/encoding/write errors, and cleans up only a table this invocation successfully created |
| `facts.go` | Keeps the original CREATE bytes, four fixed anchors, exact ordered 24-column identity, independent DECIMAL/member facts, and complete 21-target/3-control pairing |
| `main_test.go` | Uses a controlled `database/sql` driver and failing writers to reject COUNT errors, incomplete/duplicate rows, provider omissions, and evidence failures without live fault injection; its `information_schema.columns` fixture mirrors the nine-column provider row shape including `generation_expression`, and its strict query ledger counts four provider calls per table load since T10 added the `STATISTICS.IS_VISIBLE` capability probe |
| `sensitivity.py` | Runs original and strict assertions with test-file-only Go overlays; missing/empty/duplicate columns and missing/malformed ALTER definitions must pass the old insufficient assertions and fail the new assertions explicitly |

## Exports and invocation

No Go API is exported. Build and run from the repository root:

```sh
CGO_ENABLED=0 go build -o "$proof_root/bin/t06a17proof" ./scripts/t06a17proof
"$proof_root/bin/t06a17proof" "$proof_root/anchors" mysql57
"$proof_root/bin/t06a17proof" "$proof_root/anchors" mysql80
"$proof_root/bin/t06a17proof" "$proof_root/anchors" mysql84
"$proof_root/bin/t06a17proof" "$proof_root/anchors" tidb85
```

Supply `A17_USER` and `A17_PASS` only through the process environment. They are
not accepted in argv or written into the output. The four anchors are bounded
to loopback ports 23357/23380/23384/24000 and exact observed versions
5.7.44/8.0.46/8.4.10/8.0.11-TiDB-v8.5.0. Reuse the unchanged pinned
`docker/ddl-golden-compose.yaml`; do not change images, server variables, or
SQL to make a run pass.

Each anchor requires a new output subdirectory and a successful pre-create
COUNT=0 before CREATE. A pre-existing table is never dropped. Every successful
CREATE installs deferred cleanup on both normal and error exits, using a fresh
bounded cleanup context. Successful completion requires a successful DROP,
real final COUNT=0, complete facts, and all evidence writes. An unsuccessful
COUNT is recorded as unknown/null, never a default integer zero. Primary and
cleanup errors are retained together in the result and stderr.

The CREATE is byte-identical to the historical probe's fixed definition: no
INSERT, ALTER, primary key, or index. All 24 names and ordinals must match in
catalog and provider output; each pair is present once. The 21 target columns
preserve actual catalog Type text, while INT/VARCHAR/BIGINT keep the provider's
existing bare-keyword contract. Independent checks pin d2/d3/d0 effective
precision/scale and the eight member-bearing definitions' exact member bytes
and order. The locked parser decodes only those stored member definitions for
this bounded check; it never executes them. Catalog normalization is retained,
not reconstructed from CREATE (for example TiDB may return bare float/double
for explicit zero-scale declarations).

## Verification and evidence identity

```sh
go test -count=1 ./scripts/t06a17proof -run T06A17R1 -v
python3 scripts/t06a17proof/sensitivity.py --out "$proof_root/sensitivity"
```

Sensitivity faults are constructed assertion inputs, not database observations.
The runner saves overlay source/hashes, old committed source identity, exact
argv/cwd/times/HEAD, stdout/stderr, and direct child exit codes. A strict failure
must be an expected test assertion, not a build failure, panic, or empty match.
`--strict-worktree` and the optional `--kind` filter are development-only;
final evidence uses committed HEAD and all cases, with an exact expected
record count so an empty or partial run cannot count as complete.

Finalize source, tests, headers, module docs, and historical provenance
corrections before making the R1 code commit. Run the final gates and the
unchanged T06 Golden once at that fixed commit, then run these four native
proofs serially after Golden has completed and released its fixtures. They do
not add to the 496 Golden cases. Never stop or remove pre-existing services.

Retain exact command records, raw facts, all case/policy files, source hashes,
and three distinct binary identities: work CLI, Golden-built CLI, and this
probe. Each binary needs its own path, SHA-256, actual `go version -m`, and build
command. A `+dirty` marker is an observation, not proof of which file caused it.
If an archive build has no VCS fields, report their absence; do not synthesize
them. Final evidence belongs under `docs/verification/t06-a17/<R1-code-sha>/`
in a separate evidence-only commit. Historical A17 originals remain at their
original path; see that directory's `PROVENANCE-CORRECTION.md` for attribution.

## Dependencies

- Upstream: the T06-A17-R1 verification workflow, not the product audit path.
- Downstream: existing Go modules (`database/sql`, MySQL driver, domain spec,
  real MySQL-compatible provider, locked TiDB parser adapter), Python stdlib,
  Go/Git commands, and the unchanged Docker anchors.

## Update rule

Change headers and this file together if the collector, checks, or members
change. The root architecture does not change: this is a task-local proof
command under the existing scripts module. The existing A17 ADR records the
verification-completeness and run-identity boundary; its representation,
product contracts, and architecture decisions remain unchanged.
