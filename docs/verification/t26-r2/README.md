# T26-R2 evidence package

Scope: #105 / T26 — R2 work order: R1 verbatim supplementation +
semantic/output-contract/native-plan convergence. OFFLINE ONLY: no
product implementation, no native execution, no resource ops.

R1 originals are **preserved unmodified** under
`docs/verification/t26-r1/` (commit `17247147`). This directory holds
only new/revised documents; no R1 file was edited.

| file | answers |
|---|---|
| `r2-official-sources.md` | official source table — pinned URLs, per-version columns, SRS 8.0.x intro/8.0.13 tightening split, 5.7-vs-TiDB boundary separation, DOC/RN/WL/SRC/PEND evidence types |
| `r2-alter-instance-ext.md` | DDL-ALTER-INSTANCE-EXT local tracking — surplus subactions registry, ownership read-back of the two inventory-owned subactions, assignment path |
| `r2-server-semantics.md` | SERVER responsibility→fact→parse→safe-internal-rep→impact mapping; wrapper/name/duplicate/ALTER/post-state answers |
| `r2-srs-decision-table.md` | SRS layered complete-scope, internal representation (Raw serializer-excluded), verification capability split, full 33-row decision table, PENDING ORACLE rows |
| `r2-output-redaction.md` | T26 output redaction contract — internal/external split, 10-exit inventory, field contract, per-path behavior, paired path+leak test design, T26-bounded change surface |
| `r2-metadata-facts.md` | per-fact zero-row/error/completeness scheme for `server.existence`, `srs.existence`, `srs.name_org_conflict`, `srs.column_dependencies`; ordered-state channel separation; registered prohibitions |
| `r2-native-plan.md` | revised per-case matrix 29 cases (11 SERVER + 15 SRS + 3 ALTER-INSTANCE-inventory-owned), counting rule, leak-oracle marker separation, resource ownership, PENDING identities, blocked-egress design, one-shot discipline |

## Receipt-format index (A–J)

- A identity+diff → receipt message + `git show 17247147` output
- B verbatim attachments → the six R1 files + this README, full contents
  sent in-thread with per-file SHA256
- C contract originals → T04-A/T04-B decision doc quotes, T25 ADR
  boundary text, provider zero-row comments, source slices
- D → `r2-alter-instance-ext.md`
- E → `r2-server-semantics.md`
- F → `r2-srs-decision-table.md`
- G → `r2-output-redaction.md`
- H → `r2-metadata-facts.md`
- I → `r2-native-plan.md`
- J → receipt tail (open rulings, unproven boundaries, declaration)
