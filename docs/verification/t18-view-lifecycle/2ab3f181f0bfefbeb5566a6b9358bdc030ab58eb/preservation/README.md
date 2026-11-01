# Preservation and security

Original and final task path lists, source hashes, worktree and protected
resource receipts, approved cleanup records and the early publication
preflight. `verify-preserved-foreign.py` is the original protected-resource
checker. The initial and final foreign-root listings match; no historical
foreign-content byte baseline is asserted.

`redaction-ledger.json` records the original and sanitized hashes and categories
for fixture-credential removals. The private originals remain at their
recorded source paths. Redacted command receipts and native source copies are
historical records, not executable credentials.
The scope and limitations of the credential scan are in `SECRET-SCAN.md`.
The [redaction checker](../analysis/check_archive_redactions.py) and
`lead-redaction.*` record the lead's verification.

The archive-local [`.gitattributes`](../.gitattributes) preserves the exact
whitespace of listed original receipts only, without directory wildcards or
exempting authored Markdown or product code. See
[`whitespace-exact.json`](whitespace-exact.json) and its
[generation recipe](../analysis/archive_whitespace_policy.py).
