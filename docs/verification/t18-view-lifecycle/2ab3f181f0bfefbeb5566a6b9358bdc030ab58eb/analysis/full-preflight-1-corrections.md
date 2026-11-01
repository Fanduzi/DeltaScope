# Lead Disposition Of First Full T18 Preflight

The first complete WIP run failed: 146/149 cases passed, with 3755 recorded
CLI/native assertions. This is not final-product or acceptance evidence.
The artifact, original manifest and original author source are retained:

- /tmp/ddl-golden-t18-full-preflight-1/T18/artifact.json
- T18-full-preflight-1.manifest.json, SHA256
  22aadc8cbc1c6c19725192bb06019927a4aed6cc34b911b6041639791da34cdc
- golden-full-author.preflight-1.py, SHA256
  46ed4fddf7c9656b41c53c55ec713828cadc15541876517700a87b16fcf7a843

## Authoring Corrections, Not Relaxed Product Expectations

The lead wrote two incorrect vendor-feature identifiers in the planned
manifest. The typed adapter's existing family identity is view.syntax,
not the DDL operation identifier. The verified pause snapshot predating
this full run already records that exact identity at
internal/infrastructure/parser/tidb/extractor.go:90-93. The operation and
original SQL are separate preserved facts. The two CLI cases now assert
view.syntax exactly. Coverage=incomplete, verdict=review, exit=1,
one vendor-boundary unsupported detail, no findings and no gaps are
unchanged. TiDB ALTER and bare CHECK remain vendor refusals, not support.

The lead also used *hidden* as a denylist fixture parameter. That is not a
supported wildcard: docs/reference/config.md:954-977 specifies exact
unqualified names, schema names and schema.table pairs; the DDL rule's
matches method performs exact normalized set lookup. The private leaf is
t18_private_hidden_e8. Replace only that fixture policy entry with the
exact name. Preserve the expected denylist blocker, security warning,
reject verdict, exit=1, complete coverage and every no-leak sentinel.
Do not implement glob support or weaken this positive privacy assertion.
The shared profile's other protected_v entry remains unchanged.

All 149 IDs, SQL, native queries, locations and remaining expected values
are preserved. No product behavior changes for these corrections. They
repair lead-owned setup/identifier errors using existing documented
semantics, not inferred authorization or output-fitted semantic oracles.

The runner's legacy assertion label "zero findings" compares the configured
finding count even when positive. The condition is correct; the label is
misleading in this failed positive case. Its expected/actual count remains
visible in the manifest and artifact. No historical artifact is rewritten.
