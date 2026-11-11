# T26-SERVER-DROP-01 — archive index

Local docs-only archive for the accepted minimal audit slice
(issue #105 T26-SERVER-DROP-01). PASS ruling registered against
product/test candidate `2ee906d55cdc3916e5169f090eaf701d17a3bb9d`
(tree `7c6ea98c999f54bb60cda72495860d8ee26d2cb0`, parent
`5de2da5e9fb458355b090159698f4c54cfe816e9`), under the approved
semantics contract `docs/verification/t26-semantics-01`
(`b8a67e7b5f7e62a825adace06113a233240d28df`).

This archive does not replace the product/test candidate identity and
does not re-run anything. Worker executed all runs; the reviewer
audited delivered material only.

## Files

| file | content |
| --- | --- |
| acceptance.md | Acceptance record: PASS scope, identity chain, R1/R2 closure basis, run-record attribution, still-open scope, role split |
| build-invocation-record.md | Retained build command, binary path, and `make_policies`/`execute_cli_case` invocation fragment (reconstructed, marked as such); un-retained items listed |
| three-pkg-test.log | `go test -count=1 ./internal/application/audit ./internal/domain/rule/ddl ./internal/infrastructure/parser/tidb` output bound to `5de2da5e` |
| manifest-cli-results.json | Full `execute_cli_case` records for `mysql-drop-server`, `mysql-drop-server-if-exists`, `mysql-server-mixed-batch` (raw stdout/stderr/rc/assertions) bound to the `5de2da5e`-content binary |
| verdict-test-run.log | `go test -count=1 -run '^TestT26DropServerBarrierPublicShape$' -v ./internal/application/audit` PASS record bound to `2ee906d5` |
| golden-policy-server-drop.yaml | Generated isolated profile enabling only `ddl.mysql.server.drop.notice` (as consumed) |
| golden-policy-all-off.yaml | Generated all-rules-disabled profile (as consumed by mixed-batch) |
| e9d1b5ae-to-5de2da5e.diff | R1 repair increment (7 product paths) |
| 5de2da5e-to-2ee906d5.diff | R2 test increment (1 file, +3 lines verdict assertion) |
| SHA256SUMS.txt | Checksums of the archived evidence files |

Original runner assertion labels are preserved verbatim in
`manifest-cli-results.json`; where label text does not match the
non-zero expectation shape (`zero findings`, `no diagnostics`), the
archive note applies: verification compared `actual` values against
manifest `expected`, and actual values matched.
