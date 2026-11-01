# Red receipts

The R1 test files from product commit
`19286f7ff73b79f6fca1cc76d11074f3aee751f2` were replayed in an isolated
worktree at unfixed product `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb`.

- `red-public.*` is the public Audit seam and fails semantically for F1–F4.
- `red-internal.*` is the literal internal companion replay. It is a build
  failure because the unfixed code lacks `ViewMetadata.TargetSchemaAbsent`; it
  is retained as provenance, not as the semantic proof.
- `batch_state_t18r1_test.compat.go` is the internal replay source adjusted
  only to use APIs already present on the unfixed baseline.
- `red-internal-compat.*` is the resulting semantic red run. It proves the
  schema premise was not consumed, non-updatable CHECK OPTION was accepted,
  inherited MERGE fallback was not reviewed, and the nearest correlated outer
  scope was incorrectly rejected.
- `red-identity.txt` records the baseline SHA and the SHA that supplied the
  tests.
