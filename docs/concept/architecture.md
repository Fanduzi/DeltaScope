# Product Architecture

DeltaScope keeps one audit engine and exposes it through library, CLI, and HTTP interfaces. The same rule evaluation path runs offline by default and accepts optional metadata enrichment when a live MySQL or TiDB-compatible instance is available.

## Shared Flow

ASCII diagram:

```text
SQL text / file / stdin / HTTP body
                |
                v
      +----------------------+
      | CLI / HTTP / Library |
      +----------------------+
                |
                v
      +----------------------+
      | Policy / Config Load |
      +----------------------+
                |
                v
      +----------------------+
      | Parser + Extractor   |
      | normalized statements|
      +----------------------+
                |
      +---------+----------+
      |                    |
      v                    v
+----------------+   +----------------------+
| Offline facts  |   | Optional metadata    |
| only           |   | enrichment           |
+----------------+   | instance + schema    |
      |              +----------------------+
      +---------+----------+
                |
                v
      +----------------------+
      | Rule Evaluation      |
      | blocker/warning/...  |
      +----------------------+
                |
                v
      +----------------------+
      | Verdict + Findings   |
      +----------------------+
                |
      +---------+---------+----------------+
      |                   |                |
      v                   v                v
  CLI markdown        CLI JSON        HTTP / library
  or exit code        for agents      structured result
```

## Design Points

- Offline-first is the default contract, not a reduced compatibility mode.
- Metadata-aware mode enriches the same statements instead of switching to a separate ruleset.
- Product surfaces share one severity model: `blocker`, `warning`, and `notice`.
- Output shape changes by interface, but verdict semantics stay aligned.
